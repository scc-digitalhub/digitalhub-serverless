#!/usr/bin/env bash
# Build the native Go TVM serving image (tvm-runtime-go): a Nuclio processor with
# the compiled-in `tvm` runtime (cgo -> libtvm_ffi/libtvm_runtime). Serves
# OpenInference v2; the .so Model is downloaded into TVM_MODEL_DIR by an init
# container. No python wrapper.
#
# Usage: ./build.sh [--load] [--push]
#   TAG=<repo:tag>         image name         (default tvm-runtime-go:0.25)
#   REGISTRY=<host[/org]>  push target prefix (the pushed ref is $REGISTRY/$TAG)
#   --load  -> minikube image load (local dev)     --push -> push to the registry
# Remote cluster: build, --push to your registry, then point CORE at the exact
# pushed ref via RUNTIME_TVM_SERVE (e.g. RUNTIME_TVM_SERVE=ghcr.io/acme/tvm-runtime-go:0.25).
# Requires a locally-built TVM (override with TVM_HOME / TVM_BUILD).
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)" # digitalhub-serverless root
TVM_HOME="${TVM_HOME:-$HOME/tvm/src/tvm-current}"
TVM_BUILD="${TVM_BUILD:-$TVM_HOME/build}"
FFI_INC="$TVM_HOME/3rdparty/tvm-ffi/include"
DLPACK_INC="$TVM_HOME/3rdparty/tvm-ffi/3rdparty/dlpack/include"
# Default tag = major.minor of the TVM actually packaged (resolved from the
# TVM_HOME dir name, e.g. tvm-0.25.0 -> 0.25), so the image tag always matches
# its contents. Override with TVM_TAG or a full TAG.
if [ -z "${TVM_TAG:-}" ]; then
    _v="$(basename "$(readlink -f "$TVM_HOME")")"; _v="${_v#tvm-}"
    if [[ "$_v" =~ ^[0-9]+\.[0-9]+ ]]; then TVM_TAG="${_v%.*}"; else TVM_TAG="0.25"; fi
fi
TAG="${TAG:-tvm-runtime-go:${TVM_TAG}}"
REGISTRY="${REGISTRY:-}" # empty = push bare $TAG; set host[/org] to push for a remote cluster

LOAD=0; PUSH=0
for a in "$@"; do case "$a" in --load) LOAD=1 ;; --push) PUSH=1 ;; esac; done

[ -f "$TVM_BUILD/lib/libtvm_ffi.so" ] || { echo "no TVM build at $TVM_BUILD/lib"; exit 1; }
[ -f "$FFI_INC/tvm/ffi/c_api.h" ] || { echo "no tvm-ffi headers at $FFI_INC"; exit 1; }

ctx="$(mktemp -d)"
trap 'rm -rf "$ctx"' EXIT
echo "== staging build context =="
mkdir -p "$ctx/tvm-include/tvm-ffi" "$ctx/tvm-include/dlpack" "$ctx/tvm-lib"
cp -r "$FFI_INC/." "$ctx/tvm-include/tvm-ffi/"
cp -r "$DLPACK_INC/." "$ctx/tvm-include/dlpack/"
cp "$TVM_BUILD/lib/libtvm_ffi.so" "$TVM_BUILD/lib/libtvm_runtime.so" "$ctx/tvm-lib/"
rsync -a --exclude .git --exclude test --exclude images "$REPO/" "$ctx/src/"
cp "$HERE/Dockerfile" "$HERE/entrypoint.sh" "$ctx/"

echo "== docker build $TAG =="
docker build -t "$TAG" "$ctx"

[ "$LOAD" = 1 ] && { echo ">> minikube image load $TAG"; minikube image load "$TAG"; }
if [ "$PUSH" = 1 ]; then
    ref="${REGISTRY:+$REGISTRY/}$TAG"
    docker tag "$TAG" "$ref"; docker push "$ref"
    echo ">> pushed $ref  (point CORE at it: RUNTIME_TVM_SERVE=$ref)"
fi
echo "DONE $TAG"
