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

// OpenInference v2 (KServe) request/response — must match the openinference
// trigger's RESTInferenceRequest/RESTInferenceResponse wire shapes. The trigger
// hands us the request JSON in the event body and expects the response JSON back.
type v2InputTensor struct {
	Name     string  `json:"name"`
	Shape    []int64 `json:"shape"`
	Datatype string  `json:"datatype"`
	Data     any     `json:"data"`
}

type v2OutputTensor struct {
	Name     string  `json:"name"`
	Shape    []int64 `json:"shape,omitempty"`
	Datatype string  `json:"datatype,omitempty"`
	Data     any     `json:"data,omitempty"`
}

type v2Request struct {
	ID     string          `json:"id,omitempty"`
	Inputs []v2InputTensor `json:"inputs"`
}

type v2Response struct {
	ModelName string           `json:"model_name,omitempty"`
	ID        string           `json:"id,omitempty"`
	Outputs   []v2OutputTensor `json:"outputs"`
}
