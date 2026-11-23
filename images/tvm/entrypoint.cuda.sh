#!/usr/bin/env bash
# SPDX-FileCopyrightText: © 2026 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

if [ ! -f /opt/tvm/lib/libtvm_runtime_cuda.so ]; then
    echo "[tvm-go-serve] CUDA backend is missing: libtvm_runtime_cuda.so must be packaged in this CUDA image" >&2
    exit 1
fi
libraries="$(LD_TRACE_LOADED_OBJECTS=1 /opt/nuclio/processor 2>&1)" || {
    echo "[tvm-go-serve] cannot load CUDA processor dependencies: $libraries" >&2
    exit 1
}
if [[ "$libraries" == *"libcuda.so.1 => not found"* ]]; then
    echo "[tvm-go-serve] NVIDIA driver library libcuda.so.1 is unavailable; expose an NVIDIA GPU and its host driver using NVIDIA Container Toolkit" >&2
    exit 1
fi
if [[ "$libraries" == *"not found"* ]]; then
    echo "[tvm-go-serve] missing CUDA processor dependencies: $libraries" >&2
    exit 1
fi
exec /entrypoint.sh "$@"
