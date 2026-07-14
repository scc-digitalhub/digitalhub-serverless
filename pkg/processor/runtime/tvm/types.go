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
}

type modelMetadata struct {
	Entry   string       `json:"entry"`
	Inputs  []tensorSpec `json:"inputs"`
	Outputs []tensorSpec `json:"outputs"`
}
