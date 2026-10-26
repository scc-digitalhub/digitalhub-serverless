/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package openinference

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A quantized tensor must publish scale/zero_point: a client receiving int8 has no
// other way to map the values back to reals.
func TestTensorMetadataCarriesQuantizationParams(t *testing.T) {
	m := toRESTTensorMetadata(TensorDef{
		Name:      "images",
		DataType:  "INT8",
		Shape:     []int64{1, 224, 224, 3},
		Scale:     []float64{0.003921568859368563},
		ZeroPoint: []int64{-128},
	})

	assert.NotNil(t, m.Parameters)
	assert.Equal(t, []float64{0.003921568859368563}, m.Parameters["scale"])
	assert.Equal(t, []int64{-128}, m.Parameters["zero_point"])
	assert.NotContains(t, m.Parameters, "quantized_dimension", "per-tensor: nessun asse da indicizzare")
}

// Per-axis quantization indexes the entries by an axis, which must travel too:
// without it a client cannot tell which dimension the scales belong to.
func TestTensorMetadataCarriesQuantizedDimension(t *testing.T) {
	m := toRESTTensorMetadata(TensorDef{
		Name:               "weights",
		DataType:           "INT8",
		Shape:              []int64{64, 3, 3, 3},
		Scale:              []float64{0.1, 0.2, 0.3},
		ZeroPoint:          []int64{0, 0, 0},
		QuantizedDimension: 3,
	})

	assert.Equal(t, int64(3), m.Parameters["quantized_dimension"])
}

// Non-regression: a float model must serialize exactly as before, with no
// `parameters` key at all. Quantization is an independent axis — it must not leak
// into the response of models that have none.
func TestTensorMetadataOmitsParametersForFloat(t *testing.T) {
	m := toRESTTensorMetadata(TensorDef{
		Name:     "images",
		DataType: "FP32",
		Shape:    []int64{1, 3, 640, 640},
	})

	assert.Nil(t, m.Parameters)

	raw, err := json.Marshal(m)
	assert.NoError(t, err)
	assert.NotContains(t, string(raw), "parameters",
		"il campo deve sparire dal JSON, non comparire come null")
}

// Runtimes that do not report their own signature keep the previous behaviour:
// the tensors declared in the function configuration.
func TestModelTensorsFallsBackToConfiguration(t *testing.T) {
	oi := &openInference{ // WorkerAllocator nil: nessun runtime da interrogare
		configuration: &Configuration{
			InputTensors:  []TensorDef{{Name: "in", DataType: "FP32", Shape: []int64{1, 3}}},
			OutputTensors: []TensorDef{{Name: "out", DataType: "FP32", Shape: []int64{1, 1}}},
		},
	}

	in, out := oi.modelTensors()
	assert.Equal(t, "in", in[0].Name)
	assert.Equal(t, "out", out[0].Name)
}

// The same, seen from the wire: the metadata endpoint of a float model is unchanged.
func TestModelMetadataResponseHasNoParametersForFloatModel(t *testing.T) {
	oi := createTestOpenInferenceTrigger(t)

	rec := httptest.NewRecorder()
	oi.handleModelMetadata(rec, httptest.NewRequest("GET", "/v2/models/test-model", nil))
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp ModelMetadataResponse
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Inputs)
	for _, tensor := range append(resp.Inputs, resp.Outputs...) {
		assert.Nil(t, tensor.Parameters, "tensore %s", tensor.Name)
	}
	assert.NotContains(t, rec.Body.String(), "parameters")
}
