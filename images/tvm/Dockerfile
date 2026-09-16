# SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0

# Nuclio processor with the compiled-in `tvm` runtime (cgo -> libtvm_ffi/libtvm_runtime);
# serves OpenInference v2. Build context is assembled by images/tvm/build.sh
# (tvm-include/ headers, tvm-lib/ .so, src/ source).

# build: golang + cgo linked against TVM
ARG TVM_VERSION=unknown
ARG TVM_GIT_COMMIT=unknown
ARG TVM_FFI_VERSION=unknown
ARG TVM_FFI_GIT_COMMIT=unknown
FROM golang:1.25 AS build
ARG TVM_VERSION
ARG TVM_GIT_COMMIT
COPY tvm-include /opt/tvm/include
COPY tvm-lib /opt/tvm/lib
WORKDIR /src
COPY src/ .
ENV CGO_ENABLED=1 \
    CGO_CFLAGS="-I/opt/tvm/include/tvm-ffi -I/opt/tvm/include/dlpack" \
    CGO_LDFLAGS="-L/opt/tvm/lib -Wl,-rpath,/opt/tvm/lib"
RUN go build \
    -ldflags="-X github.com/scc-digitalhub/digitalhub-serverless/pkg/processor/runtime/tvm.runtimeTVMVersion=${TVM_VERSION} -X github.com/scc-digitalhub/digitalhub-serverless/pkg/processor/runtime/tvm.runtimeTVMGitCommit=${TVM_GIT_COMMIT}" \
    -o /out/processor ./cmd/processor/main.go

# runtime: ubuntu + TVM runtime libs + processor
FROM ubuntu:24.04
ARG TVM_VERSION
ARG TVM_GIT_COMMIT
ARG TVM_FFI_VERSION
ARG TVM_FFI_GIT_COMMIT
LABEL org.opencontainers.image.version="${TVM_VERSION}" \
    org.opencontainers.image.source="https://github.com/apache/tvm" \
    org.opencontainers.image.revision="${TVM_GIT_COMMIT}" \
    org.digitalhub.tvm-ffi.version="${TVM_FFI_VERSION}" \
    org.digitalhub.tvm-ffi.revision="${TVM_FFI_GIT_COMMIT}"
RUN apt-get update \
    && apt-get install -y --no-install-recommends libstdc++6 ca-certificates jq \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /opt/tvm/lib/libtvm_runtime.so /opt/tvm/lib/libtvm_ffi.so /opt/tvm/lib/
COPY provenance.json /opt/tvm/provenance.json
COPY --from=build /out/processor /opt/nuclio/processor
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
# Lists the libraries the processor loads, like ldd. ldd itself crashes under the QEMU
# emulation used to build the arm/v7 image, while the loader's trace mode works there too.
RUN LD_TRACE_LOADED_OBJECTS=1 /opt/nuclio/processor > /tmp/libraries \
    && grep -F '/opt/tvm/lib/libtvm_ffi.so' /tmp/libraries \
    && grep -F '/opt/tvm/lib/libtvm_runtime.so' /tmp/libraries \
    && ! grep -F 'not found' /tmp/libraries \
    && rm /tmp/libraries
# LD_LIBRARY_PATH beats the binary's rpath (host build path, absent here).
ENV LD_LIBRARY_PATH=/opt/tvm/lib \
    TVM_VERSION=${TVM_VERSION} \
    TVM_GIT_COMMIT=${TVM_GIT_COMMIT} \
    TVM_FFI_VERSION=${TVM_FFI_VERSION} \
    TVM_FFI_GIT_COMMIT=${TVM_FFI_GIT_COMMIT} \
    TVM_MODEL_DIR=/shared/model \
    TVM_MODEL_NAME=model \
    TVM_SERVE_PORT=8080 \
    TVM_SERVE_GRPC_PORT=9000
EXPOSE 8080 9000
ENTRYPOINT ["/entrypoint.sh"]
