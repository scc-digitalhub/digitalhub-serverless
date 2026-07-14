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
	"bytes"
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
	"github.com/scc-digitalhub/digitalhub-serverless/pkg/processor/trigger/openinference"
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
	// Fail fast at startup for a model whose declared dtypes this image can't serve
	// (mirrors the rust worker). Native dtypes are supported; FP16 is deferred.
	checkDtype := func(t tensorSpec) error {
		if t.Dtype != "" && !supportedTvmDtype(t.Dtype) {
			return errors.Errorf("model tensor %q declares dtype %q: not supported by this serve image (FP16/others deferred)", t.Name, t.Dtype)
		}
		return nil
	}
	for _, t := range r.metadata.Inputs {
		if err := checkDtype(t); err != nil {
			return nil, err
		}
	}
	for _, t := range r.metadata.Outputs {
		if err := checkDtype(t); err != nil {
			return nil, err
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
	// Decode with UseNumber so integer inputs keep full precision: default JSON
	// decoding turns every number into a float64, losing bits above 2^53 for INT64.
	dec := json.NewDecoder(bytes.NewReader(event.GetBody()))
	dec.UseNumber()
	var req openinference.RESTInferenceRequest
	if err := dec.Decode(&req); err != nil {
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

	resp, err := r.buildResponse(req.ID, res.outputs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build response")
	}
	body, err := json.Marshal(resp)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal response")
	}
	return nuclio.Response{
		StatusCode:  200,
		ContentType: "application/json",
		Body:        body,
	}, nil
}

func (r *tvmRuntime) buildInputs(in []openinference.RESTInferInputTensor) ([]tvmrelax.Tensor, error) {
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
			reordered := make([]openinference.RESTInferInputTensor, 0, len(in))
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
		datatype := t.Datatype
		if datatype == "" {
			datatype = "FP32" // default when a client omits it
		}
		code, bits, err := v2ToDLPack(datatype)
		if err != nil {
			return nil, errors.Wrapf(err, "input %q", t.Name)
		}
		nums, err := collectNumbers(t.Data)
		if err != nil {
			return nil, errors.Wrapf(err, "input %q data", t.Name)
		}
		// Guard the FFI copy: tvm_input_add copies product(shape) elements, so a
		// shape that disagrees with the data length — or overflows int64 — would read
		// past the packed buffer and crash the worker. Same checks as the rust
		// worker's validate_shape: reject negative dims, multiply with an overflow
		// check (a plain product can wrap to a plausible-looking positive), and
		// require product(shape) == len(nums). Empty shape with empty data is rejected
		// (product 1 != 0); zero-element shapes like [2,0] pass (product 0).
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
		if n != int64(len(nums)) {
			return nil, errors.Errorf("input %q shape %v implies %d elements but %d were provided", t.Name, t.Shape, n, len(nums))
		}
		raw, err := encodeInputBytes(nums, datatype)
		if err != nil {
			return nil, errors.Wrapf(err, "input %q", t.Name)
		}
		out = append(out, tvmrelax.Tensor{Shape: t.Shape, Code: code, Bits: bits, Data: raw})
	}
	return out, nil
}

// buildResponse decodes each output tensor's raw bytes into a typed number array
// carrying its actual dtype (from the tensor, not the metadata), so a model that
// returns e.g. INT64 indices reports INT64.
func (r *tvmRuntime) buildResponse(id string, outs []tvmrelax.Tensor) (openinference.RESTInferenceResponse, error) {
	resp := openinference.RESTInferenceResponse{ID: id, Outputs: make([]openinference.RESTInferOutputTensor, len(outs))}
	for i, o := range outs {
		name := fmt.Sprintf("output_%d", i)
		if i < len(r.metadata.Outputs) && r.metadata.Outputs[i].Name != "" {
			name = r.metadata.Outputs[i].Name
		}
		datatype, err := dlpackToV2(int(o.Code), int(o.Bits))
		if err != nil {
			return openinference.RESTInferenceResponse{}, errors.Wrapf(err, "output %q", name)
		}
		data, err := decodeOutputData(o.Data, datatype)
		if err != nil {
			return openinference.RESTInferenceResponse{}, errors.Wrapf(err, "output %q", name)
		}
		resp.Outputs[i] = openinference.RESTInferOutputTensor{
			Name:     name,
			Shape:    o.Shape,
			Datatype: datatype,
			Data:     data,
		}
	}
	return resp, nil
}

// ProcessBatch is not supported (single-event openinference serving).
func (r *tvmRuntime) ProcessBatch(batch []nuclio.Event,
	functionLogger logger.Logger) ([]*runtime.ResponseWithErrors, error) {
	return nil, nuclio.ErrNotImplemented
}
