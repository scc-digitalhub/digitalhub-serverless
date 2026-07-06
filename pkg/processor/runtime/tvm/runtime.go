/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

// Package tvm is a native Nuclio Go runtime that serves a compiled TVM Relax
// model.so via the OpenInference v2 protocol. It does the inference IN-PROCESS
// through cgo (package tvmrelax) — no python wrapper, no subprocess — replicating
// what the rust tvm-serve does. The openinference trigger handles the v2 wire
// format and hands each request to ProcessEvent.
package tvm

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	goruntime "runtime"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	nuclio "github.com/nuclio/nuclio-sdk-go"
	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/scc-digitalhub/digitalhub-serverless/pkg/processor/runtime/tvm/tvmrelax"
)

const defaultModelDir = "/shared/model"

// The TVM Relax VM (and every tvm-ffi handle) is not thread-safe, so we can't call
// Infer from arbitrary Nuclio worker goroutines. Instead one dedicated goroutine
// (modelLoop) owns the model and pins itself to a single OS thread; ProcessEvent
// hands it work over jobCh and waits on respCh. This is the classic actor pattern
// and mirrors how the rust worker serializes inference on one thread.
type inferJob struct {
	inputs []tvmrelax.Tensor
	respCh chan inferResult // the caller's private reply channel
}

type inferResult struct {
	outputs []tvmrelax.Tensor
	err     error
}

type tvmRuntime struct {
	*runtime.AbstractRuntime // supplies every Runtime method except ProcessEvent/ProcessBatch
	configuration            *runtime.Configuration
	metadata                 modelMetadata // entry + input/output tensor specs from metadata.json
	jobCh                    chan inferJob // ProcessEvent -> modelLoop
}

// NewRuntime loads the model and starts the single-threaded inference loop.
func NewRuntime(parentLogger logger.Logger,
	configuration *runtime.Configuration) (runtime.Runtime, error) {
	abstract, err := runtime.NewAbstractRuntime(parentLogger, configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract runtime")
	}
	r := &tvmRuntime{
		AbstractRuntime: abstract,
		configuration:   configuration,
		jobCh:           make(chan inferJob),
	}

	modelDir := os.Getenv("TVM_MODEL_DIR")
	if modelDir == "" {
		modelDir = defaultModelDir
	}

	metaRaw, err := os.ReadFile(filepath.Join(modelDir, "metadata.json"))
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to read metadata.json in %s", modelDir)
	}
	if err := json.Unmarshal(metaRaw, &r.metadata); err != nil {
		return nil, errors.Wrap(err, "Failed to parse metadata.json")
	}
	if r.metadata.Entry == "" {
		r.metadata.Entry = "main"
	}
	// FP32-only serving: fail at startup with a clear message instead of erroring
	// on every request for models that declare other dtypes (mirrors the rust worker).
	for _, t := range append(append([]tensorSpec{}, r.metadata.Inputs...), r.metadata.Outputs...) {
		if t.Dtype != "" && t.Dtype != "float32" {
			return nil, errors.Errorf("model tensor %q declares dtype %q: this serve image is FP32-only", t.Name, t.Dtype)
		}
	}

	// load + run the model on ONE dedicated OS thread: the Relax VM and every
	// tvm-ffi handle are not thread-safe (same constraint as the rust worker).
	ready := make(chan error, 1)
	go r.modelLoop(modelDir, ready)
	if err := <-ready; err != nil {
		return nil, errors.Wrap(err, "Failed to load TVM model")
	}

	parentLogger.InfoWith("TVM runtime ready",
		"modelDir", modelDir, "entry", r.metadata.Entry,
		"inputs", len(r.metadata.Inputs), "outputs", len(r.metadata.Outputs))
	r.SetStatus(status.Ready)
	return r, nil
}

// modelLoop is the actor goroutine: it pins itself to one OS thread, loads the
// model there, reports the load result on `ready`, then serves inference jobs one
// at a time from jobCh forever. LockOSThread guarantees every tvm-ffi call for this
// model happens on the same thread (the ABI requires it).
func (r *tvmRuntime) modelLoop(modelDir string, ready chan error) {
	goruntime.LockOSThread()
	defer goruntime.UnlockOSThread()

	model, err := tvmrelax.Load(filepath.Join(modelDir, "model.so"), r.metadata.Entry)
	ready <- err
	if err != nil {
		return
	}
	for job := range r.jobCh {
		outs, inferErr := model.Infer(job.inputs)
		job.respCh <- inferResult{outputs: outs, err: inferErr}
	}
}

// ProcessEvent parses an OpenInference v2 infer request, runs TVM inference, and
// returns the v2 response.
func (r *tvmRuntime) ProcessEvent(event nuclio.Event,
	functionLogger logger.Logger) (interface{}, error) {
	var req v2Request
	if err := json.Unmarshal(event.GetBody(), &req); err != nil {
		return nil, errors.Wrap(err, "invalid inference request")
	}

	inputs, err := r.buildInputs(req.Inputs)
	if err != nil {
		return nil, err
	}

	job := inferJob{inputs: inputs, respCh: make(chan inferResult, 1)}
	r.jobCh <- job
	res := <-job.respCh
	if res.err != nil {
		return nil, errors.Wrap(res.err, "inference failed")
	}

	body, err := json.Marshal(r.buildResponse(req.ID, res.outputs))
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal response")
	}
	return nuclio.Response{
		StatusCode:  200,
		ContentType: "application/json",
		Body:        body,
	}, nil
}

func (r *tvmRuntime) buildInputs(in []v2InputTensor) ([]tvmrelax.Tensor, error) {
	// Enforce the model arity up front (the entry call would fail later with an
	// obscure FFI error otherwise).
	if len(r.metadata.Inputs) > 0 && len(in) != len(r.metadata.Inputs) {
		return nil, errors.Errorf("expected %d input tensor(s), got %d", len(r.metadata.Inputs), len(in))
	}
	// Name-based matching when possible: the v2 spec identifies tensors by name,
	// so if every request input is named and the names are exactly the model's
	// input names, reorder to metadata order; otherwise match positionally.
	if len(r.metadata.Inputs) > 0 {
		byName := make(map[string]int, len(in))
		named := true
		for i := range in {
			if in[i].Name == "" {
				named = false
				break
			}
			byName[in[i].Name] = i
		}
		if named && len(byName) == len(r.metadata.Inputs) {
			reordered := make([]v2InputTensor, 0, len(in))
			ok := true
			for _, spec := range r.metadata.Inputs {
				idx, found := byName[spec.Name]
				if !found {
					ok = false
					break
				}
				reordered = append(reordered, in[idx])
			}
			if ok {
				in = reordered
			}
		}
	}
	out := make([]tvmrelax.Tensor, 0, len(in))
	for i := range in {
		t := &in[i]
		if t.Datatype != "" && t.Datatype != "FP32" && t.Datatype != "float32" {
			return nil, errors.Errorf("input %q datatype %q unsupported (FP32 only)", t.Name, t.Datatype)
		}
		data, err := flattenFloat32(t.Data)
		if err != nil {
			return nil, errors.Wrapf(err, "input %q data", t.Name)
		}
		// Guard the FFI copy: tvm_input_add copies product(shape) floats out of `data`,
		// so a client shape that disagrees with the data length — or overflows int64 —
		// would read past the Go backing array and crash the in-process worker. Mirror
		// the rust worker's validate_shape: reject negative dims, use a checked multiply
		// (a plain product could wrap around to a "plausible" positive value), and
		// require product(shape) == len(data). An empty shape with empty data is also
		// rejected (product 1 != 0), which would otherwise memcpy from a nil ptr;
		// zero-element shapes like [2,0] with no data are valid and pass (product 0).
		n := int64(1)
		for _, d := range t.Shape {
			if d < 0 {
				return nil, errors.Errorf("input %q has a negative dimension in shape %v", t.Name, t.Shape)
			}
			if d != 0 && n > math.MaxInt64/d {
				return nil, errors.Errorf("input %q shape %v is too large", t.Name, t.Shape)
			}
			n *= d
		}
		if n != int64(len(data)) {
			return nil, errors.Errorf("input %q shape %v implies %d elements but %d were provided", t.Name, t.Shape, n, len(data))
		}
		out = append(out, tvmrelax.Tensor{Name: t.Name, Shape: t.Shape, Data: data})
	}
	return out, nil
}

func (r *tvmRuntime) buildResponse(id string, outs []tvmrelax.Tensor) v2Response {
	resp := v2Response{ID: id, Outputs: make([]v2OutputTensor, len(outs))}
	for i, o := range outs {
		name := fmt.Sprintf("output_%d", i)
		if i < len(r.metadata.Outputs) && r.metadata.Outputs[i].Name != "" {
			name = r.metadata.Outputs[i].Name
		}
		resp.Outputs[i] = v2OutputTensor{
			Name:     name,
			Shape:    o.Shape,
			Datatype: "FP32",
			Data:     o.Data,
		}
	}
	return resp
}

// flattenFloat32 converts a JSON-decoded numeric tensor payload (flat or nested
// arrays of numbers) into a flat row-major []float32.
func flattenFloat32(v any) ([]float32, error) {
	var out []float32
	var rec func(any) error
	rec = func(x any) error {
		switch t := x.(type) {
		case []any:
			for _, e := range t {
				if err := rec(e); err != nil {
					return err
				}
			}
		case float64:
			out = append(out, float32(t))
		case float32:
			out = append(out, t)
		case json.Number:
			f, err := t.Float64()
			if err != nil {
				return err
			}
			out = append(out, float32(f))
		case nil:
			return errors.New("nil data")
		default:
			return errors.Errorf("unsupported data element type %T", x)
		}
		return nil
	}
	if err := rec(v); err != nil {
		return nil, err
	}
	return out, nil
}

// ProcessBatch is not supported (single-event openinference serving).
func (r *tvmRuntime) ProcessBatch(batch []nuclio.Event,
	functionLogger logger.Logger) ([]*runtime.ResponseWithErrors, error) {
	return nil, nuclio.ErrNotImplemented
}
