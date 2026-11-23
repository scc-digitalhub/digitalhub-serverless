/*
SPDX-FileCopyrightText: © 2026 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package tvmrelax

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestCUDAUnavailable(t *testing.T) {
	if cudaEnabled && os.Getenv("TVM_TEST_CUDA_UNAVAILABLE") != "1" {
		t.Skip("set TVM_TEST_CUDA_UNAVAILABLE=1 on a CUDA build without a visible GPU")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	_, err := LoadWithBackend("missing-model.so", "main", "cuda")
	if err == nil || !strings.Contains(err.Error(), "CUDA") {
		t.Fatalf("expected a clear CUDA availability error before model loading, got %v", err)
	}
	if cudaEnabled && !strings.Contains(err.Error(), "device 0 is unavailable") {
		t.Fatalf("expected a registered CUDA backend with no available device, got %v", err)
	}
}

func TestUnsupportedBackend(t *testing.T) {
	_, err := LoadWithBackend("missing-model.so", "main", "vulkan")
	if err == nil || !strings.Contains(err.Error(), "unsupported TVM backend") {
		t.Fatalf("expected unsupported backend, got %v", err)
	}
}

// Optional integration fixture: a compiled FP32 [4] model returning x + 1.
// Run once with LLVM and once with CUDA on real NVIDIA hardware. Repetition
// exercises device copies, synchronization and input/output handle lifetimes.
func TestFP32Model(t *testing.T) {
	path := os.Getenv("TVM_TEST_MODEL")
	if path == "" {
		t.Skip("TVM_TEST_MODEL is unset; real model inference NOT_RUN")
	}
	backend := os.Getenv("TVM_TEST_BACKEND")
	if backend == "" {
		backend = "llvm"
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	m, err := LoadWithBackend(path, "main", backend)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if reference := os.Getenv("TVM_TEST_REFERENCE"); reference != "" {
		checkFP32Reference(t, m, reference)
		return
	}
	for run := 0; run < 20; run++ {
		data := make([]byte, 16)
		for i := 0; i < 4; i++ {
			binary.LittleEndian.PutUint32(data[i*4:], math.Float32bits(float32(run+i)))
		}
		outputs, err := m.Infer([]Tensor{{Shape: []int64{4}, Code: 2, Bits: 32, Data: data}})
		if err != nil {
			t.Fatal(err)
		}
		if len(outputs) != 1 || len(outputs[0].Shape) != 1 || outputs[0].Shape[0] != 4 || outputs[0].Code != 2 || outputs[0].Bits != 32 || len(outputs[0].Data) != 16 {
			t.Fatalf("unexpected FP32 output: %+v", outputs)
		}
		for i := 0; i < 4; i++ {
			got := math.Float32frombits(binary.LittleEndian.Uint32(outputs[0].Data[i*4:]))
			if got != float32(run+i+1) {
				t.Fatalf("run %d output[%d]: got %v, want %v", run, i, got, run+i+1)
			}
		}
	}
}

// References contain FP32 tensors as {shape:[...], data:[...]} arrays, in the
// model's positional input/output order. They can be generated from existing
// compiler fixtures without committing architecture-specific shared libraries.
func checkFP32Reference(t *testing.T, m *Model, path string) {
	t.Helper()
	type referenceTensor struct {
		Shape []int64   `json:"shape"`
		Data  []float32 `json:"data"`
	}
	var reference struct {
		Inputs  []referenceTensor `json:"inputs"`
		Outputs []referenceTensor `json:"outputs"`
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &reference); err != nil {
		t.Fatal(err)
	}
	inputs := make([]Tensor, len(reference.Inputs))
	for i, input := range reference.Inputs {
		data := make([]byte, 4*len(input.Data))
		for k, value := range input.Data {
			binary.LittleEndian.PutUint32(data[4*k:], math.Float32bits(value))
		}
		inputs[i] = Tensor{Shape: input.Shape, Code: 2, Bits: 32, Data: data}
	}
	for run := 0; run < 20; run++ {
		outputs, err := m.Infer(inputs)
		if err != nil {
			t.Fatal(err)
		}
		if len(outputs) != len(reference.Outputs) {
			t.Fatalf("got %d outputs, want %d", len(outputs), len(reference.Outputs))
		}
		for i, expected := range reference.Outputs {
			actual := outputs[i]
			if actual.Code != 2 || actual.Bits != 32 || len(actual.Shape) != len(expected.Shape) || len(actual.Data) != 4*len(expected.Data) {
				t.Fatalf("output[%d] has unexpected dtype, shape or length", i)
			}
			for k, dimension := range expected.Shape {
				if actual.Shape[k] != dimension {
					t.Fatalf("output[%d] shape %v, want %v", i, actual.Shape, expected.Shape)
				}
			}
			for k, want := range expected.Data {
				got := math.Float32frombits(binary.LittleEndian.Uint32(actual.Data[4*k:]))
				if math.IsNaN(float64(got)) || math.Abs(float64(got-want)) > 1e-4+1e-4*math.Abs(float64(want)) {
					t.Fatalf("run %d output[%d][%d]: got %v, want %v", run, i, k, got, want)
				}
			}
		}
	}
}
