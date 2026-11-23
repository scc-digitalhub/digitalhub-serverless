#!/usr/bin/env bash
# SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0

# Build the native Go TVM serving image (tvm-runtime-go): a Nuclio processor with
# the compiled-in `tvm` runtime (cgo -> libtvm_ffi/libtvm_runtime). Serves
# OpenInference v2; the .so Model is downloaded into TVM_MODEL_DIR by an init
# container. No python wrapper.
#
# Usage: ./build.sh [--load] [--push]
#   TAG=<repo:tag>         image name         (default tvm-runtime-go:0.26)
#   REGISTRY=<host[/org]>  push target prefix (the pushed ref is $REGISTRY/$TAG)
#   --load  -> minikube image load (local dev)     --push -> push to the registry
# Remote cluster: build, --push to your registry, then point CORE at the exact
# pushed ref via RUNTIME_TVM_SERVE (e.g. RUNTIME_TVM_SERVE=ghcr.io/acme/tvm-runtime-go:0.26).
# Requires a locally-built TVM (override with TVM_HOME / TVM_BUILD).
# TVM_BACKEND=cuda selects an attested CUDA build and a distinct -cuda13.2.1 tag.
# CUDA builds need CUDA_CUDART_LIBRARY / CUDA_DRIVER_STUB_LIBRARY, defaulting to
# CUDA_HOME=/usr/local/cuda lib64 locations; these are used in the builder only.
# TVM_EXPORTED_ARTIFACTS=1 accepts an exported CUDA toolkit artifact directory.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)" # digitalhub-serverless root
TVM_HOME="${TVM_HOME:-$HOME/tvm/src/tvm-current}"
TVM_BUILD="${TVM_BUILD:-$TVM_HOME/build}"
TVM_BACKEND="${TVM_BACKEND:-cpu}"
case "$TVM_BACKEND" in
    cpu) DOCKERFILE=Dockerfile; TAG_SUFFIX=; LIBRARIES=(libtvm_runtime.so libtvm_ffi.so) ;;
    cuda) DOCKERFILE=Dockerfile.cuda; TAG_SUFFIX=-cuda13.2.1; LIBRARIES=(libtvm_runtime.so libtvm_ffi.so libtvm_runtime_cuda.so) ;;
    *) echo "unsupported TVM_BACKEND: $TVM_BACKEND (expected cpu or cuda)" >&2; exit 1 ;;
esac
FFI_INC="$TVM_HOME/3rdparty/tvm-ffi/include"
DLPACK_INC="$TVM_HOME/3rdparty/tvm-ffi/3rdparty/dlpack/include"
for tool in docker git python3 rsync sha256sum; do
    command -v "$tool" >/dev/null 2>&1 || { echo "missing required tool: $tool" >&2; exit 1; }
done
VERIFY_ARGS=(--source "$TVM_HOME" --build "$TVM_BUILD" --backend "$TVM_BACKEND")
PROVENANCE="$TVM_BUILD/digitalhub-build-provenance.json"
if [ "${TVM_EXPORTED_ARTIFACTS:-0}" = 1 ]; then
    VERIFY_ARGS+=(--exported-artifacts)
    [ -f "$PROVENANCE" ] || PROVENANCE="$TVM_BUILD/provenance.json"
fi
IFS=$'\t' read -r TVM_RELEASE SOURCE_TAG TVM_GIT_COMMIT TVM_FFI_GIT_COMMIT TVM_FFI_VERSION < <(
    python3 "$HERE/verify-tvm-build.py" "${VERIFY_ARGS[@]}"
)
TVM_TAG="${TVM_TAG:-$SOURCE_TAG}"
TAG="${TAG:-tvm-runtime-go:${TVM_TAG}${TAG_SUFFIX}}"
if [ "$TVM_BACKEND" = cuda ] && [[ ! "${TAG##*:}" =~ (^|[-_.])cuda13[.]2[.]1($|[-_]) ]]; then
    echo "CUDA image TAG must include a cuda13.2.1 component to keep CPU tags independent" >&2
    exit 1
fi
REGISTRY="${REGISTRY:-}" # empty = push bare $TAG; set host[/org] to push for a remote cluster

LOAD=0; PUSH=0
for a in "$@"; do case "$a" in --load) LOAD=1 ;; --push) PUSH=1 ;; esac; done

[ -f "$TVM_BUILD/lib/libtvm_ffi.so" ] || { echo "no TVM build at $TVM_BUILD/lib"; exit 1; }
[ -f "$FFI_INC/tvm/ffi/c_api.h" ] || { echo "no tvm-ffi headers at $FFI_INC"; exit 1; }

if [ -n "${TVM_BUILD_CONTEXT:-}" ]; then
    ctx="$TVM_BUILD_CONTEXT"
    mkdir "$ctx"
else
    ctx="$(mktemp -d -t tvm-go-image.XXXXXXXX)"
fi
echo "== build context: $ctx =="
echo "== staging build context =="
mkdir -p "$ctx/tvm-include/tvm-ffi" "$ctx/tvm-include/dlpack" "$ctx/tvm-lib"
cp -r "$FFI_INC/." "$ctx/tvm-include/tvm-ffi/"
cp -r "$DLPACK_INC/." "$ctx/tvm-include/dlpack/"
for library in "${LIBRARIES[@]}"; do cp "$TVM_BUILD/lib/$library" "$ctx/tvm-lib/"; done
if [ "$TVM_BACKEND" = cuda ]; then
    if [ "${TVM_EXPORTED_ARTIFACTS:-0}" = 1 ]; then
        cudart="${CUDA_CUDART_LIBRARY:-$TVM_BUILD/cuda/libcudart.so.13}"
        driver_stub="${CUDA_DRIVER_STUB_LIBRARY:-$TVM_BUILD/cuda-driver-stubs/libcuda.so.1}"
    else
        cudart="${CUDA_CUDART_LIBRARY:-${CUDA_HOME:-/usr/local/cuda}/lib64/libcudart.so.13}"
        driver_stub="${CUDA_DRIVER_STUB_LIBRARY:-${CUDA_HOME:-/usr/local/cuda}/lib64/stubs/libcuda.so}"
    fi
    [ -f "$cudart" ] || { echo "missing CUDA 13.2.1 libcudart: $cudart" >&2; exit 1; }
    [ -f "$driver_stub" ] || { echo "missing build-only CUDA driver stub: $driver_stub" >&2; exit 1; }
    mkdir -p "$ctx/cuda-lib" "$ctx/cuda-driver-stubs"
    cp -L "$cudart" "$ctx/cuda-lib/libcudart.so.13"
    cp -L "$driver_stub" "$ctx/cuda-driver-stubs/libcuda.so.1"
fi
cp "$PROVENANCE" "$ctx/provenance.json"
rsync -a --exclude .git --exclude test --exclude images \
    --exclude /.bin --exclude /.cuda-build --exclude /.cache \
    --exclude venv --exclude .venv --exclude node_modules "$REPO/" "$ctx/src/"
cp "$HERE/$DOCKERFILE" "$ctx/Dockerfile"
cp "$HERE/entrypoint.sh" "$ctx/"
if [ "$TVM_BACKEND" = cuda ]; then
    cp "$HERE/entrypoint.cuda.sh" "$ctx/"
    cuda_cache="$TVM_BUILD/cuda-CMakeCache.txt"
    [ -f "$cuda_cache" ] || cuda_cache="$TVM_BUILD/CMakeCache.txt"
    cp "$cuda_cache" "$ctx/cuda-CMakeCache.txt"
fi
python3 - "$ctx" <<'PY'
import sys
from pathlib import Path

context = Path(sys.argv[1])
size = sum(path.stat().st_size for path in context.rglob("*") if path.is_file())
print(f"== staged build context: {size} bytes ==")
PY

echo "== docker build $TAG =="
docker build \
    --build-arg "TVM_VERSION=$TVM_RELEASE" \
    --build-arg "TVM_GIT_COMMIT=$TVM_GIT_COMMIT" \
    --build-arg "TVM_FFI_VERSION=$TVM_FFI_VERSION" \
    --build-arg "TVM_FFI_GIT_COMMIT=$TVM_FFI_GIT_COMMIT" \
    -t "$TAG" "$ctx"

for library in "${LIBRARIES[@]}"; do
    source_line="$(sha256sum "$TVM_BUILD/lib/$library")"
    image_line="$(docker run --rm --entrypoint /usr/bin/sha256sum "$TAG" "/opt/tvm/lib/$library")"
    source_hash="${source_line%% *}"
    image_hash="${image_line%% *}"
    [ "$image_hash" = "$source_hash" ] || {
        echo "$library in $TAG does not match the verified TVM build" >&2
        exit 1
    }
done

[ "$LOAD" = 1 ] && { echo ">> minikube image load $TAG"; minikube image load "$TAG"; }
if [ "$PUSH" = 1 ]; then
    ref="${REGISTRY:+$REGISTRY/}$TAG"
    image_tag="${ref##*:}"
    if [ "$TVM_BACKEND" = cpu ] && [[ "${image_tag,,}" == *cuda* ]]; then
        echo "CPU publication cannot use a CUDA tag" >&2
        exit 1
    fi
    python3 "$HERE/check-image-tag.py" "$ref"
    docker tag "$TAG" "$ref"; docker push "$ref"
    echo ">> pushed $ref  (point CORE at it: RUNTIME_TVM_SERVE=$ref)"
fi
echo "DONE $TAG"
