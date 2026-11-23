/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package tvm

import (
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateLLVMTarget(t *testing.T) {
	assert.NoError(t, validateLLVMTarget("llvm", "amd64"))
	assert.NoError(t, validateLLVMTarget(
		`{"kind":"llvm","mtriple":"x86_64-pc-linux-gnu","mcpu":"alderlake"}`,
		"amd64",
	))
	assert.NoError(t, validateLLVMTarget("llvm -mtriple=aarch64-linux-gnu", "arm64"))
	assert.ErrorContains(t, validateLLVMTarget("cuda", "amd64"), "LLVM CPU")
	assert.ErrorContains(t, validateLLVMTarget(
		`{"kind":"llvm","mtriple":"aarch64-linux-gnu"}`,
		"amd64",
	), "incompatible")
	assert.ErrorContains(t, validateLLVMTarget("{", "amd64"), "invalid TVM target")
	for _, triple := range []string{"x86_64-apple-darwin", "x86_64-linux-musl", "aarch64-linux-gnu"} {
		assert.Error(t, validateTargetTriple(triple, "amd64"), triple)
	}
	assert.Error(t, validateTargetTriple("armv7l-unknown-linux-gnueabi", "arm"))
	assert.NoError(t, validateTargetTriple("armv7l-unknown-linux-gnueabihf", "arm"))
	assert.Empty(t, architectureFromTriple("arm64_be-linux-gnu"))
}

func TestValidateModelTargetCUDA(t *testing.T) {
	for _, tc := range []struct{ arch, triple, gpu string }{
		{"amd64", "x86_64-pc-linux-gnu", "sm_90a"},
		{"arm64", "aarch64-linux-gnu", "sm_87"},
		{"arm64", "aarch64-linux-gnu", "sm_110"},
	} {
		target := `{"kind":"cuda","arch":"` + tc.gpu + `","host":{"kind":"llvm","mtriple":"` + tc.triple + `"}}`
		backend, err := validateModelTarget(target, tc.arch)
		require.NoError(t, err)
		assert.Equal(t, "cuda", backend)
	}
	for _, target := range []string{
		`cuda -arch=sm_87`,
		`{"kind":"cuda","host":{"kind":"llvm","mtriple":"x86_64-linux-gnu"}}`,
		`{"kind":"cuda","arch":"compute_87","host":{"kind":"llvm","mtriple":"x86_64-linux-gnu"}}`,
		`{"kind":"cuda","arch":"sm_87","host":{"kind":"llvm"}}`,
		`{"kind":"cuda","arch":"sm_87","host":{"kind":"cuda"}}`,
		`{"kind":"cuda","arch":"sm_87","host":{"kind":"llvm","mtriple":"aarch64-linux-gnu"}}`,
		`{"kind":"cuda","arch":"sm_87","host":{"kind":"llvm","mtriple":"x86_64-linux-musl"}}`,
		`{"kind":"vulkan"}`,
	} {
		_, err := validateModelTarget(target, "amd64")
		assert.Error(t, err, target)
	}
	_, err := validateModelTarget(`{"kind":"cuda","arch":"sm_87","host":{"kind":"llvm","mtriple":"armv7l-linux-gnueabihf"}}`, "arm")
	assert.ErrorContains(t, err, "amd64 and arm64")
	for _, target := range []string{"llvm", "llvm -mtriple=armv7l-linux-gnueabihf"} {
		backend, err := validateModelTarget(target, "arm")
		assert.NoError(t, err)
		assert.Equal(t, "llvm", backend)
	}
}

func TestValidateELFArchitecture(t *testing.T) {
	executable, err := os.Executable()
	require.NoError(t, err)
	assert.NoError(t, validateELFArchitecture(executable, runtime.GOARCH))
	for _, arch := range []string{"amd64", "arm64", "arm"} {
		if arch != runtime.GOARCH {
			assert.Error(t, validateELFArchitecture(executable, arch))
		}
	}
	assert.Error(t, validateELFArchitecture(t.TempDir()+"/missing.so", runtime.GOARCH))
}

func TestValidateModelCompatibilityIdentity(t *testing.T) {
	executable, err := os.Executable()
	require.NoError(t, err)
	previousVersion := runtimeTVMVersion
	previousCommit := runtimeTVMGitCommit
	t.Cleanup(func() {
		runtimeTVMVersion = previousVersion
		runtimeTVMGitCommit = previousCommit
	})
	runtimeTVMVersion = "0.26.0"
	runtimeTVMGitCommit = "c7b458e"

	metadata := modelMetadata{
		TVMVersion:   "0.26.0",
		TVMGitCommit: "c7b458e",
		Target:       "llvm",
	}
	assert.NoError(t, validateModelCompatibility(metadata, executable))
	metadata.TargetTriple = "aarch64-linux-gnu"
	if runtime.GOARCH == "arm64" {
		metadata.TargetTriple = "x86_64-linux-gnu"
	}
	assert.ErrorContains(t, validateModelCompatibility(metadata, executable), "incompatible")
	metadata.TargetTriple = ""

	metadata.TVMVersion = "0.25.0"
	assert.ErrorContains(t, validateModelCompatibility(metadata, executable), "version")
	metadata.TVMVersion = "0.26.0"
	metadata.TVMGitCommit = "different"
	assert.ErrorContains(t, validateModelCompatibility(metadata, executable), "revision")
	metadata.TVMGitCommit = ""
	assert.ErrorContains(t, validateModelCompatibility(metadata, executable), "recompile")
}
