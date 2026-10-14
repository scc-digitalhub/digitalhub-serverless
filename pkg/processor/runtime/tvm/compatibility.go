/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package tvm

import (
	"debug/elf"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
)

var (
	runtimeTVMVersion   = "unknown"
	runtimeTVMGitCommit = "unknown"
)

type llvmTarget struct {
	Kind    string `json:"kind"`
	MTriple string `json:"mtriple"`
}

func validateModelCompatibility(metadata modelMetadata, modelPath string) error {
	if runtimeTVMVersion == "" || runtimeTVMVersion == "unknown" {
		return fmt.Errorf("serve image has no embedded TVM version")
	}
	if runtimeTVMGitCommit == "" || runtimeTVMGitCommit == "unknown" {
		return fmt.Errorf("serve image has no embedded TVM source revision")
	}
	if metadata.TVMVersion == "" {
		return fmt.Errorf("model metadata has no tvm_version")
	}
	if metadata.TVMVersion != runtimeTVMVersion {
		return fmt.Errorf(
			"model TVM version %q is incompatible with serve image version %q",
			metadata.TVMVersion,
			runtimeTVMVersion,
		)
	}
	if metadata.TVMGitCommit == "" {
		return fmt.Errorf("model metadata has no tvm_git_commit; recompile it with an attested toolkit")
	}
	if metadata.TVMGitCommit != runtimeTVMGitCommit {
		return fmt.Errorf(
			"model TVM revision %q is incompatible with serve image revision %q",
			metadata.TVMGitCommit,
			runtimeTVMGitCommit,
		)
	}
	if err := validateLLVMTarget(metadata.Target, runtime.GOARCH); err != nil {
		return err
	}
	if err := validateELFArchitecture(modelPath, runtime.GOARCH); err != nil {
		return err
	}
	return nil
}

func validateLLVMTarget(targetText, runtimeArch string) error {
	targetText = strings.TrimSpace(targetText)
	if targetText == "" {
		return fmt.Errorf("model metadata has no target")
	}
	target := llvmTarget{}
	if strings.HasPrefix(targetText, "{") {
		if err := json.Unmarshal([]byte(targetText), &target); err != nil {
			return fmt.Errorf("invalid TVM target %q: %w", targetText, err)
		}
	} else {
		fields := strings.Fields(targetText)
		target.Kind = fields[0]
		for _, field := range fields[1:] {
			if value, found := strings.CutPrefix(field, "-mtriple="); found {
				target.MTriple = value
			}
		}
	}
	if target.Kind != "llvm" {
		return fmt.Errorf("serve image supports LLVM CPU models, got target kind %q", target.Kind)
	}
	if target.MTriple == "" {
		return nil
	}
	targetArch := architectureFromTriple(target.MTriple)
	if targetArch == "" {
		return fmt.Errorf("unsupported LLVM target triple %q", target.MTriple)
	}
	if targetArch != runtimeArch {
		return fmt.Errorf(
			"model target triple %q is incompatible with runtime architecture %q",
			target.MTriple,
			runtimeArch,
		)
	}
	return nil
}

func architectureFromTriple(triple string) string {
	arch := strings.ToLower(strings.SplitN(triple, "-", 2)[0])
	switch {
	case arch == "x86_64" || arch == "amd64":
		return "amd64"
	case arch == "aarch64" || arch == "arm64":
		return "arm64"
	case strings.HasPrefix(arch, "arm"):
		return "arm"
	case arch == "riscv64":
		return "riscv64"
	case arch == "powerpc64le" || arch == "ppc64le":
		return "ppc64le"
	case arch == "s390x":
		return "s390x"
	default:
		return ""
	}
}

func validateELFArchitecture(path, runtimeArch string) error {
	file, err := elf.Open(path)
	if err != nil {
		return fmt.Errorf("invalid TVM model library %q: %w", path, err)
	}
	defer file.Close()
	expected, found := map[string]elf.Machine{
		"amd64":   elf.EM_X86_64,
		"arm64":   elf.EM_AARCH64,
		"arm":     elf.EM_ARM,
		"riscv64": elf.EM_RISCV,
		"ppc64le": elf.EM_PPC64,
		"s390x":   elf.EM_S390,
	}[runtimeArch]
	if !found {
		return fmt.Errorf("unsupported serve image architecture %q", runtimeArch)
	}
	if file.Machine != expected {
		return fmt.Errorf(
			"model ELF machine %q is incompatible with runtime architecture %q",
			file.Machine,
			runtimeArch,
		)
	}
	return nil
}
