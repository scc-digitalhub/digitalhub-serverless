# Nuclio processor with the compiled-in `tvm` runtime (cgo -> libtvm_ffi/libtvm_runtime);
# serves OpenInference v2. Build context is assembled by images/tvm/build.sh
# (tvm-include/ headers, tvm-lib/ .so, src/ source).

# build: golang + cgo linked against TVM
FROM golang:1.25 AS build
COPY tvm-include /opt/tvm/include
COPY tvm-lib /opt/tvm/lib
WORKDIR /src
COPY src/ .
ENV CGO_ENABLED=1 \
    CGO_CFLAGS="-I/opt/tvm/include/tvm-ffi -I/opt/tvm/include/dlpack" \
    CGO_LDFLAGS="-L/opt/tvm/lib -Wl,-rpath,/opt/tvm/lib"
RUN go build -o /out/processor ./cmd/processor/main.go

# runtime: ubuntu + TVM runtime libs + processor
FROM ubuntu:24.04
RUN apt-get update \
    && apt-get install -y --no-install-recommends libstdc++6 ca-certificates jq \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /opt/tvm/lib/libtvm_runtime.so /opt/tvm/lib/libtvm_ffi.so /opt/tvm/lib/
COPY --from=build /out/processor /opt/nuclio/processor
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
# LD_LIBRARY_PATH beats the binary's rpath (host build path, absent here).
ENV LD_LIBRARY_PATH=/opt/tvm/lib \
    TVM_MODEL_DIR=/shared/model \
    TVM_MODEL_NAME=model \
    TVM_SERVE_PORT=8080 \
    TVM_SERVE_GRPC_PORT=9000
EXPOSE 8080 9000
ENTRYPOINT ["/entrypoint.sh"]
