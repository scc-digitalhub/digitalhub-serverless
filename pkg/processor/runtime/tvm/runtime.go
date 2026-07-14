/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

// Package tvm is a native Nuclio Go runtime serving a compiled TVM Relax model.so
// over OpenInference v2, running inference in-process via cgo (package tvmrelax).
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

// The Relax VM (and every tvm-ffi handle) is not thread-safe, so one dedicated
// goroutine (modelLoop) owns the model on a single OS thread; ProcessEvent hands it
// work over jobCh and waits on respCh (actor pattern, as the rust worker does).
type inferJob struct {
	inputs []tvmrelax.Tensor
	respCh chan inferResult
}

type inferResult struct {
	outputs []tvmrelax.Tensor
	err     error
}

type tvmRuntime struct {
	*runtime.AbstractRuntime // supplies every Runtime method except ProcessEvent/ProcessBatch
	configuration            *runtime.Configuration
	metadata                 modelMetadata
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
	// fail fast if a model declares a dtype this image can't serve (mirrors rust).
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

	// load + run on ONE dedicated OS thread (VM/ffi handles are not thread-safe).
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

// modelLoop pins itself to one OS thread (the ffi ABI requires all calls on the
// same thread), loads the model, reports on `ready`, then serves jobs one at a time.
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

// ProcessEvent parses a v2 infer request, runs inference, returns the v2 response.
func (r *tvmRuntime) ProcessEvent(event nuclio.Event,
	functionLogger logger.Logger) (interface{}, error) {
	// UseNumber keeps integer precision: plain decoding floats every number, losing
	// bits above 2^53 for INT64.
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
	// enforce arity up front (else the entry call fails later with an obscure FFI error).
	if len(r.metadata.Inputs) > 0 && len(in) != len(r.metadata.Inputs) {
		return nil, errors.Errorf("expected %d input tensor(s), got %d", len(r.metadata.Inputs), len(in))
	}
	// if every request input is named and the names match the model's, reorder to
	// metadata order; otherwise match positionally.
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
		// guard the FFI copy (mirrors rust validate_shape): reject negative dims,
		// multiply with an overflow check, and require product(shape) == len(nums),
		// else tvm_input_add reads past the packed buffer and crashes the worker.
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

// buildResponse decodes each output using its actual dtype (from the tensor, not
// the metadata), so a model returning e.g. INT64 indices reports INT64.
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
