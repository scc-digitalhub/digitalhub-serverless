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
}

func TestValidateELFArchitecture(t *testing.T) {
	executable, err := os.Executable()
	require.NoError(t, err)
	assert.NoError(t, validateELFArchitecture(executable, runtime.GOARCH))
	assert.Error(t, validateELFArchitecture(executable, "arm64"))
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

	metadata.TVMVersion = "0.25.0"
	assert.ErrorContains(t, validateModelCompatibility(metadata, executable), "version")
	metadata.TVMVersion = "0.26.0"
	metadata.TVMGitCommit = "different"
	assert.ErrorContains(t, validateModelCompatibility(metadata, executable), "revision")
	metadata.TVMGitCommit = ""
	assert.ErrorContains(t, validateModelCompatibility(metadata, executable), "recompile")
}
