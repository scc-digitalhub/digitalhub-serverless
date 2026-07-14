/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package tvm

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

var allV2Dtypes = []string{
	"FP32", "FP64",
	"INT8", "INT16", "INT32", "INT64",
	"UINT8", "UINT16", "UINT32", "UINT64",
}

// jsonNumbers decodes a JSON array literal with UseNumber and flattens it via
// collectNumbers — the exact path ProcessEvent uses for request tensor data.
func jsonNumbers(t *testing.T, jsonData string) []json.Number {
	dec := json.NewDecoder(bytes.NewReader([]byte(jsonData)))
	dec.UseNumber()
	var v any
	err := dec.Decode(&v)
	assert.NoError(t, err)
	nums, err := collectNumbers(v)
	assert.NoError(t, err)
	return nums
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	// encodeInputBytes -> decodeOutputData must recover the original values for
	// every supported dtype, including negative signed ints and the unsigned /
	// signed boundary values that only survive the json.Number (UseNumber) path.
	cases := []struct {
		datatype string
		jsonData string
		want     any // exact concrete slice type json.Marshal sees
	}{
		{"FP32", "[0.5, -1.25, 3.5, 0]", []float32{0.5, -1.25, 3.5, 0}},
		{"FP64", "[0.1, -2.5, 1e10, 0]", []float64{0.1, -2.5, 1e10, 0}},
		{"INT8", "[-128, -1, 0, 127]", []int8{-128, -1, 0, 127}},
		{"INT16", "[-32768, -1, 0, 32767]", []int16{-32768, -1, 0, 32767}},
		{"INT32", "[-2147483648, -1, 0, 2147483647]", []int32{-2147483648, -1, 0, 2147483647}},
		{"INT64", "[-9223372036854775808, -1, 0, 9223372036854775807]",
			[]int64{-9223372036854775808, -1, 0, 9223372036854775807}},
		{"UINT8", "[0, 1, 255]", []int{0, 1, 255}}, // []int, not []uint8 (json would base64 it)
		{"UINT16", "[0, 1, 65535]", []uint16{0, 1, 65535}},
		{"UINT32", "[0, 1, 4294967295]", []uint32{0, 1, 4294967295}},
		{"UINT64", "[0, 1, 18446744073709551615]", []uint64{0, 1, 18446744073709551615}},
	}
	for _, c := range cases {
		t.Run(c.datatype, func(t *testing.T) {
			nums := jsonNumbers(t, c.jsonData)
			raw, err := encodeInputBytes(nums, c.datatype)
			assert.NoError(t, err)

			got, err := decodeOutputData(raw, c.datatype)
			assert.NoError(t, err)
			// assert.Equal checks the concrete type too, pinning the exact slice
			// type ([]float32, []int8, ... []int for UINT8) that json.Marshal sees.
			assert.Equal(t, c.want, got)
		})
	}

	t.Run("NestedArrays", func(t *testing.T) {
		// collectNumbers flattens nested (row-major) arrays the same as flat ones.
		nums := jsonNumbers(t, "[[1.5, -2.5], [0, 4.25]]")
		raw, err := encodeInputBytes(nums, "FP32")
		assert.NoError(t, err)
		got, err := decodeOutputData(raw, "FP32")
		assert.NoError(t, err)
		assert.Equal(t, []float32{1.5, -2.5, 0, 4.25}, got)
	})

	t.Run("UnsupportedDatatype", func(t *testing.T) {
		nums := jsonNumbers(t, "[1]")
		_, err := encodeInputBytes(nums, "FP16")
		assert.Error(t, err)
		_, err = decodeOutputData([]byte{0, 0}, "FP16")
		assert.Error(t, err)
	})
}

func TestV2DLPackMappingInverses(t *testing.T) {
	// v2ToDLPack and dlpackToV2 must be mutual inverses over the 10 dtypes.
	for _, name := range allV2Dtypes {
		t.Run(name, func(t *testing.T) {
			code, bits, err := v2ToDLPack(name)
			assert.NoError(t, err)
			back, err := dlpackToV2(int(code), int(bits))
			assert.NoError(t, err)
			assert.Equal(t, name, back)
		})
	}

	t.Run("Rejected", func(t *testing.T) {
		_, _, err := v2ToDLPack("FP16")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "FP16") // distinct deferred-FP16 error
		_, _, err = v2ToDLPack("BOOL")
		assert.Error(t, err)
		_, _, err = v2ToDLPack("")
		assert.Error(t, err)
		_, err = dlpackToV2(dlFloat, 16) // FP16
		assert.Error(t, err)
		_, err = dlpackToV2(99, 32)
		assert.Error(t, err)
	})
}

func TestSupportedTvmDtype(t *testing.T) {
	supported := []string{
		"float32", "float64",
		"int8", "int16", "int32", "int64",
		"uint8", "uint16", "uint32", "uint64",
	}
	for _, dt := range supported {
		assert.True(t, supportedTvmDtype(dt), dt)
	}
	for _, dt := range []string{"float16", "bool", "bfloat16", "FP32", ""} {
		assert.False(t, supportedTvmDtype(dt), dt)
	}
}
