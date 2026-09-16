<!--
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
-->

# TVM runtime for Nuclio (`tvm`)

A Nuclio runtime, compiled into the processor, that serves a model compiled by TVM over
the **Open Inference Protocol v2**: REST on `8080`, gRPC on `9000`. It is published as the
**`ghcr.io/scc-digitalhub/tvm-runtime-go`** image, the default serve image of DigitalHub
CORE's **`tvm+serve`** task. The Rust image of `digitalhub-tvm-rust` behaves the same and
can be used instead.

```
 TVM_MODEL_DIR                     Nuclio processor
 ├── model.so        ──load──►   worker 1 ─ tvm runtime ─ model copy ─┐
 └── metadata.json               worker 2 ─ tvm runtime ─ model copy ─┤◄── openinference trigger
                                 ...                                  │    REST :8080 · gRPC :9000
                                 worker N ─ tvm runtime ─ model copy ─┘
```

Nothing model-specific is baked into the image. At startup the image entrypoint writes
the Nuclio configuration from the environment, and each worker checks the model, loads its
own copy and runs inferences in-process through cgo, with no Python.

## Configuration

| Variable              | Default         | Description                                                                      |
| --------------------- | --------------- | -------------------------------------------------------------------------------- |
| `TVM_MODEL_DIR`       | `/shared/model` | Folder with `model.so` and `metadata.json`.                                      |
| `TVM_MODEL_NAME`      | `model`         | Model name in the URLs, `/v2/models/<name>`.                                     |
| `TVM_SERVE_WORKERS`   | `1`             | Nuclio workers: inferences run in parallel, each with its own model copy.        |
| `TVM_NUM_THREADS`     | every core      | TVM threads of each worker. Keep `workers × threads` within the CPUs of the pod. |
| `TVM_SERVE_PORT`      | `8080`          | REST port.                                                                       |
| `TVM_SERVE_GRPC_PORT` | `9000`          | gRPC port.                                                                       |

## Endpoints

Served by the `openinference` trigger of this repository.

| What            | REST                                                        | gRPC             |
| --------------- | ----------------------------------------------------------- | ---------------- |
| Server live     | `GET /v2/health/live`                                       | `ServerLive`     |
| Server ready    | `GET /v2/health/ready`                                      | `ServerReady`    |
| Server metadata | `GET /v2`                                                   | `ServerMetadata` |
| Model ready     | `GET /v2/models/<name>/ready`                               | `ModelReady`     |
| Model metadata  | `GET /v2/models/<name>`                                     | `ModelMetadata`  |
| Inference       | `POST /v2/models/<name>/infer` (also `/versions/<v>/infer`) | `ModelInfer`     |

```bash
curl -X POST http://localhost:8080/v2/models/model/infer \
  -H 'Content-Type: application/json' \
  -d '{"inputs":[{"name":"images","datatype":"FP32","shape":[1,3,640,640],"data":[...]}]}'
```

- **Inputs** are matched by name when every input has one and the names match the model,
  otherwise by position. A missing `datatype` means `FP32`.
- **Data types**: `FP32`, `FP64`, `INT8`, `INT16`, `INT32`, `INT64`, `UINT8`, `UINT16`,
  `UINT32`, `UINT64`. `FP16` is not supported yet.
- **Size limit**: 512 MB for a REST request and for a gRPC message.
- **Model metadata** comes from `metadata.json`. For quantized models (`int8` / `uint8`
  tensors) the REST metadata adds `scale`, `zero_point` and, per axis,
  `quantized_dimension` under `parameters`, so the client can convert values
  (`real = (q - zero_point) * scale`). The gRPC metadata has no field for them.

## Which models it serves

At startup the runtime refuses the model, with a clear error, unless:

- `metadata.json` has the same `tvm_version` and `tvm_git_commit` as the TVM built into the
  image: **compile with the `tvm-toolkit` of the same release**;
- the model was compiled for a CPU (LLVM target) of the image architecture, and `model.so`
  is a library for that architecture;
- every input and output uses a supported data type.

The image exists for `linux/amd64`, `linux/arm64` and `linux/arm/v7`.

## Run it

**From CORE**: nothing to do, it is the default (`RUNTIME_TVM_SERVE`). CORE downloads the
model into `TVM_MODEL_DIR` with an init container and sets `TVM_MODEL_NAME`,
`TVM_SERVE_WORKERS` and `TVM_NUM_THREADS` (the task CPUs divided by the workers).

**With Docker**, given a folder with `model.so` and `metadata.json`:

```bash
docker run --rm -p 8080:8080 -p 9000:9000 \
  -v "$PWD/my-model:/shared/model" \
  ghcr.io/scc-digitalhub/tvm-runtime-go:0.26.0
```

## How it works

| File                       | Role                                                                                                                     |
| -------------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| `factory.go`               | Registers the runtime kind `tvm` in Nuclio (blank import in `cmd/processor/app/processor.go`).                           |
| `runtime.go`               | Reads `metadata.json`, loads the model and turns v2 requests into inferences and back.                                   |
| `compatibility.go`         | The startup checks: TVM version and commit, target, architecture of `model.so`.                                          |
| `dtype.go`, `types.go`     | Data type conversions and the `metadata.json` structure.                                                                 |
| `tvmrelax/tvmrelax.go`     | cgo binding: loads `model.so` and runs the Relax VM through the `tvm-ffi` C API.                                         |
| `images/tvm/Dockerfile`    | Builds the processor against the TVM runtime and packages it with the two TVM libraries.                                 |
| `images/tvm/entrypoint.sh` | Writes `/tmp/processor.yaml` (runtime `tvm`, `openinference` trigger, ports, workers, tensors) and starts the processor. |

TVM handles are not thread-safe, so every worker keeps its model on one dedicated OS thread
and runs one inference at a time; parallelism comes from the number of workers.

## Versions and release

**The image tag names the Apache TVM version**: git tag `tvm-0.26.0` builds
`tvm-runtime-go:0.26.0` on Apache TVM `0.26.0`. Do not create a GitHub Release for these
tags: releases publish the other runtimes of this repository.

Pushing a tag `tvm-X.Y.Z` (or `tvm-X.Y`) starts `.github/workflows/tvm-runtime-go-image.yml`.
For each architecture it:

1. compiles only the TVM runtime (`libtvm_runtime.so`, `libtvm_ffi.so`) of Apache TVM
   `vX.Y.Z`: natively on amd64 and arm64, cross-compiled for armv7;
2. builds the processor with cgo, embedding the TVM version and commit;
3. builds the image and checks the libraries inside it (`ldd` and SHA-256);
4. pushes `<version>-<arch>`.

A last job joins the images into the multi-architecture tag.

## Development

Tests and builds need the headers and libraries of a local build of the same Apache TVM
release:

```bash
TVM=~/tvm/src/tvm-0.26.0
export CGO_ENABLED=1
export CGO_CFLAGS="-I$TVM/3rdparty/tvm-ffi/include -I$TVM/3rdparty/tvm-ffi/3rdparty/dlpack/include"
export CGO_LDFLAGS="-L$TVM/build/lib -Wl,-rpath,$TVM/build/lib"

go test ./pkg/processor/runtime/tvm/... ./pkg/processor/trigger/openinference/...
```

A processor built without the TVM version and commit refuses every model; the image sets
them with
`-ldflags "-X .../runtime/tvm.runtimeTVMVersion=<version> -X .../runtime/tvm.runtimeTVMGitCommit=<commit>"`.

## Limitations

- CPU only, no GPU.
- One model per processor; no batching (`ProcessBatch` is not implemented).
- `FP16` tensors are not supported yet.

## Copyright and license

Copyright © 2025 DSLab – Fondazione Bruno Kessler and individual contributors.

This project is licensed under the Apache License, Version 2.0.
You may not use this file except in compliance with the License. Ownership of contributions remains with the original authors and is governed by the terms of the Apache 2.0 License, including the requirement to grant a license to the project.
