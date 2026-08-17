/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package tvm

// metadata.json produced by tvm+compile (read at startup).
type tensorSpec struct {
	Name  string  `json:"name"`
	Shape []int64 `json:"shape"`
	Dtype string  `json:"dtype"`
	// Affine quantization params, present only for quantized models (TFLite int8):
	// without them a client receiving int8 cannot map the values back to reals.
	// Per-axis quantization yields more than one entry, indexed by QuantizedDimension.
	Scale              []float64 `json:"scale,omitempty"`
	ZeroPoint          []int64   `json:"zero_point,omitempty"`
	QuantizedDimension int64     `json:"quantized_dimension,omitempty"`
}

type modelMetadata struct {
	Entry   string       `json:"entry"`
	Inputs  []tensorSpec `json:"inputs"`
	Outputs []tensorSpec `json:"outputs"`
}
