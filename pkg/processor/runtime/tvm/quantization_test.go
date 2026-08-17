/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package tvm

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// metadata.json states TVM dtype names and, for quantized models, the affine params.
// Both must survive the trip into the runtime: Go silently drops unknown JSON fields,
// so a missing struct member loses the data without any error.
func TestMetadataParsesQuantizationParams(t *testing.T) {
	raw := []byte(`{
	  "entry": "main",
	  "inputs":  [{"name":"images","shape":[1,224,224,3],"dtype":"int8",
	               "scale":[0.003921568859368563],"zero_point":[-128]}],
	  "outputs": [{"name":"out","shape":[1,56,1029],"dtype":"int8",
	               "scale":[0.0172492116689682],"zero_point":[-35]}]
	}`)

	var meta modelMetadata
	assert.NoError(t, json.Unmarshal(raw, &meta))
	assert.Equal(t, []float64{0.003921568859368563}, meta.Inputs[0].Scale)
	assert.Equal(t, []int64{-128}, meta.Inputs[0].ZeroPoint)
	assert.Equal(t, []int64{-35}, meta.Outputs[0].ZeroPoint)
}

// A float model has no params at all: the fields stay empty rather than zero-valued,
// which is what lets the trigger tell "not quantized" from "quantized with scale 0".
func TestMetadataFloatModelHasNoQuantizationParams(t *testing.T) {
	raw := []byte(`{"entry":"main",
	  "inputs":[{"name":"images","shape":[1,3,640,640],"dtype":"float32"}],
	  "outputs":[{"name":"out","shape":[1,84,8400],"dtype":"float32"}]}`)

	var meta modelMetadata
	assert.NoError(t, json.Unmarshal(raw, &meta))
	assert.Empty(t, meta.Inputs[0].Scale)
	assert.Empty(t, meta.Outputs[0].Scale)
}

// toTensorDefs is what the trigger advertises: TVM dtype names become v2 names, and
// the quantization params ride along unchanged.
func TestToTensorDefsMapsDtypeAndCarriesParams(t *testing.T) {
	defs := toTensorDefs([]tensorSpec{
		{Name: "images", Dtype: "int8", Shape: []int64{1, 224, 224, 3},
			Scale: []float64{0.00392}, ZeroPoint: []int64{-128}},
		{Name: "plain", Dtype: "float32", Shape: []int64{1, 3}},
	})

	assert.Equal(t, "INT8", defs[0].DataType)
	assert.Equal(t, []float64{0.00392}, defs[0].Scale)
	assert.Equal(t, []int64{-128}, defs[0].ZeroPoint)

	assert.Equal(t, "FP32", defs[1].DataType)
	assert.Empty(t, defs[1].Scale, "un tensore float non deve acquisire parametri")
}
