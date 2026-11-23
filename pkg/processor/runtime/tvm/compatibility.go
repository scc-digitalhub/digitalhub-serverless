/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package tvm

import (
	"debug/elf"
	"encoding/json"
	"fmt"
	"regexp"
	"runtime"
	"strings"
)

var (
	runtimeTVMVersion   = "unknown"
	runtimeTVMGitCommit = "unknown"
)

type modelTarget struct {
	Kind    string       `json:"kind"`
	MTriple string       `json:"mtriple"`
	Arch    string       `json:"arch"`
	Host    *modelTarget `json:"host"`
}

var cudaArchitecture = regexp.MustCompile(`^sm_[0-9]+[a-z]?$`)

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
	if _, err := validateModelTarget(metadata.Target, runtime.GOARCH); err != nil {
		return err
	}
	if err := validateTargetTriple(metadata.TargetTriple, runtime.GOARCH); err != nil {
		return err
	}
	return validateELFArchitecture(modelPath, runtime.GOARCH)
}

func validateLLVMTarget(targetText, runtimeArch string) error {
	target, err := parseModelTarget(targetText)
	if err != nil {
		return err
	}
	if target.Kind != "llvm" {
		return fmt.Errorf("expected an LLVM CPU target, got target kind %q", target.Kind)
	}
	return validateTargetTriple(target.MTriple, runtimeArch)
}

func parseModelTarget(targetText string) (modelTarget, error) {
	targetText = strings.TrimSpace(targetText)
	if targetText == "" {
		return modelTarget{}, fmt.Errorf("model metadata has no target")
	}
	target := modelTarget{}
	if strings.HasPrefix(targetText, "{") {
		if err := json.Unmarshal([]byte(targetText), &target); err != nil {
			return modelTarget{}, fmt.Errorf("invalid TVM target %q: %w", targetText, err)
		}
	} else {
		fields := strings.Fields(targetText)
		target.Kind = fields[0]
		for _, field := range fields[1:] {
			if value, found := strings.CutPrefix(field, "-mtriple="); found {
				target.MTriple = value
			}
			if value, found := strings.CutPrefix(field, "-arch="); found {
				target.Arch = value
			}
		}
	}
	return target, nil
}

// CUDA describes the device code. Only its nested LLVM host and the ELF library
// describe the architecture of the processor that loads model.so.
func validateModelTarget(targetText, runtimeArch string) (string, error) {
	target, err := parseModelTarget(targetText)
	if err != nil {
		return "", err
	}
	switch target.Kind {
	case "llvm":
		return "llvm", validateTargetTriple(target.MTriple, runtimeArch)
	case "cuda":
		if runtimeArch != "amd64" && runtimeArch != "arm64" {
			return "", fmt.Errorf("CUDA serve images support amd64 and arm64 hosts, got %q", runtimeArch)
		}
		if !cudaArchitecture.MatchString(target.Arch) {
			return "", fmt.Errorf("CUDA model requires an explicit GPU architecture such as sm_87, got %q", target.Arch)
		}
		if target.Host == nil || target.Host.Kind != "llvm" || target.Host.MTriple == "" {
			return "", fmt.Errorf("CUDA model requires a nested LLVM host target with an explicit mtriple")
		}
		if err := validateTargetTriple(target.Host.MTriple, runtimeArch); err != nil {
			return "", fmt.Errorf("CUDA host target: %w", err)
		}
		return "cuda", nil
	default:
		return "", fmt.Errorf("serve image supports LLVM CPU or CUDA models, got target kind %q", target.Kind)
	}
}

func validateTargetTriple(triple, runtimeArch string) error {
	if triple == "" {
		return nil
	}
	_, abi, linux := strings.Cut(triple, "-linux-")
	if !linux {
		return fmt.Errorf("serve image requires a Linux target, got %q", triple)
	}
	if !strings.HasPrefix(abi, "gnu") {
		return fmt.Errorf("serve image requires the GNU/glibc ABI, got %q", triple)
	}
	targetArch := architectureFromTriple(triple)
	if targetArch == "" {
		return fmt.Errorf("unsupported LLVM target triple %q", triple)
	}
	if targetArch != runtimeArch {
		return fmt.Errorf(
			"model target triple %q is incompatible with runtime architecture %q",
			triple,
			runtimeArch,
		)
	}
	if targetArch == "arm" && abi != "gnueabihf" {
		return fmt.Errorf("ARM32 serve image requires the hard-float ABI, got %q", triple)
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
	case arch == "arm" || strings.HasPrefix(arch, "armv") || strings.HasPrefix(arch, "thumb"):
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
	class, order := elf.ELFCLASS64, elf.ELFDATA2LSB
	if runtimeArch == "arm" {
		class = elf.ELFCLASS32
	}
	if runtimeArch == "s390x" {
		order = elf.ELFDATA2MSB
	}
	if file.Class != class || file.Data != order {
		return fmt.Errorf("model ELF class/byte order is incompatible with runtime architecture %q", runtimeArch)
	}
	return nil
}
