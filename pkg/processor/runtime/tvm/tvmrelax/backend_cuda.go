//go:build cuda

/*
SPDX-FileCopyrightText: © 2026 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package tvmrelax

/*
// Retain the backend even though registration uses static constructors.
#cgo LDFLAGS: -Wl,--no-as-needed -ltvm_runtime_cuda -Wl,--as-needed
*/
import "C"

const cudaEnabled = true
