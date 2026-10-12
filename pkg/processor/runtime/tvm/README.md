<!--
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
-->

# DigitalHub TVM Runtime Go

The image **`ghcr.io/scc-digitalhub/tvm-runtime-go`** serves a model compiled by
[Apache TVM](https://tvm.apache.org/) with the **Open Inference Protocol v2**: REST on
port `8080` and gRPC on port `9000`. Inside it runs the Nuclio processor of this repository
with the `tvm` runtime of this folder, which calls TVM through cgo, with no Python.

It is the default serve image of the DigitalHub CORE **`tvm+serve`** task. **DigitalHub TVM
Runtime Rust** behaves the same and can be used instead.

```
 TVM_MODEL_DIR                     Nuclio processor
 ├── model.so        ──load──►   worker 1 ─ tvm runtime ─ model copy ─┐
 └── metadata.json               worker 2 ─ tvm runtime ─ model copy ─┤◄── openinference trigger
                                 ...                                  │    REST :8080 · gRPC :9000
                                 worker N ─ tvm runtime ─ model copy ─┘
```

Nothing model-specific is baked into the image. At startup the image writes the Nuclio
configuration from the environment; each worker checks the model, loads its own copy and
runs the inferences in-process.

## Quick start

Download a model compiled by `tvm+compile` for your machine and start the image:

```bash
dhcli download model -p my-project -n my-model-x86 -d ./my-model   # model.so + metadata.json

docker run --rm -p 8080:8080 -p 9000:9000 \
  -v "$PWD/my-model:/shared/model" \
  -e TVM_MODEL_NAME=my-model \
  ghcr.io/scc-digitalhub/tvm-runtime-go:0.26.0

curl http://localhost:8080/v2/models/my-model
```

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
curl -X POST http://localhost:8080/v2/models/my-model/infer \
  -H 'Content-Type: application/json' \
  -d '{"inputs":[{"name":"images","datatype":"FP32","shape":[1,3,640,640],"data":[...]}]}'
```

- **Inputs** are matched by name when every input has one and the names match the model,
  otherwise by position. A missing `datatype` means `FP32`.
- **Data types**: `FP32`, `FP64`, `INT8`, `INT16`, `INT32`, `INT64`, `UINT8`, `UINT16`,
  `UINT32`, `UINT64`.
- **Size limits**: 512 MB for a REST request and for a gRPC message.
- **Quantized models** (`int8` / `uint8` tensors): the REST model metadata adds `scale`,
  `zero_point` and `quantized_dimension` under `parameters`, so the client can convert the
  values (`real = (q - zero_point) * scale`).
- **gRPC clients** use the proto `pkg/proto/inference/v2/grpc_service.proto`.

## Which models it serves

At startup the runtime refuses the model, with a clear error, unless:

- `metadata.json` has the same `tvm_version` and `tvm_git_commit` as the TVM built into the
  image: **compile with the DigitalHub TVM Toolkit of the same release**;
- the model was compiled for a CPU (LLVM target) of the image architecture, and `model.so`
  is a library for that architecture;
- every input and output uses a supported data type.

The image exists for `linux/amd64`, `linux/arm64` and `linux/arm/v7`, so the same command
runs on a Raspberry Pi.

## Use from DigitalHub CORE

It is the default serve image (`RUNTIME_TVM_SERVE`); a single `tvm+serve` run can choose
another one with `image`. CORE:

- downloads the `tvm-so` Model into `TVM_MODEL_DIR` with an init container;
- sets `TVM_MODEL_NAME`, `TVM_SERVE_WORKERS` and `TVM_NUM_THREADS` (the run CPUs divided by
  the workers);
- starts the pod on a node with the architecture of the model, where Kubernetes pulls the
  matching variant of the image.

## How it works

| File                       | Role                                                                                 |
| -------------------------- | ------------------------------------------------------------------------------------ |
| `factory.go`               | Registers the runtime kind `tvm` in Nuclio.                                          |
| `runtime.go`               | Reads `metadata.json`, loads the model and turns v2 requests into inferences.        |
| `compatibility.go`         | The startup checks: TVM version and commit, target, architecture of `model.so`.      |
| `dtype.go`, `types.go`     | Data type conversions and the `metadata.json` structure.                             |
| `tvmrelax/tvmrelax.go`     | cgo binding: loads `model.so` and runs the Relax VM through the `tvm-ffi` C API.     |
| `images/tvm/Dockerfile`    | Builds the processor against the TVM runtime and packages it with the TVM libraries. |
| `images/tvm/entrypoint.sh` | Writes the processor configuration from the environment and starts the processor.    |

TVM handles are not thread-safe, so every worker keeps its model on one dedicated OS thread
and runs one inference at a time; parallelism comes from the number of workers.

## Versions and release

**The image tag names the Apache TVM version**: git tag `tvm-0.26.0` builds
`tvm-runtime-go:0.26.0` on Apache TVM `0.26.0`. Each architecture also gets its own tag:
`0.26.0-amd64`, `0.26.0-arm64` and `0.26.0-armv7`. Do not create a GitHub Release for these
tags: the releases of this repository publish its other runtimes.

Pushing a tag `tvm-X.Y.Z` (or `tvm-X.Y`) starts `.github/workflows/tvm-runtime-go-image.yml`.
For each architecture it:

1. compiles only the TVM runtime (`libtvm_runtime.so`, `libtvm_ffi.so`) of Apache TVM
   `vX.Y.Z`: natively on amd64 and arm64, cross-compiled for armv7;
2. builds the processor with cgo, embedding the TVM version and commit;
3. builds the image and checks the libraries inside it (`ldd` and SHA-256);
4. pushes `<version>-<arch>`.

A last job publishes the multi-architecture tag.

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
- One model per processor; no batching.
- `FP16` tensors are not supported yet.

## Security Policy

The current release is the supported version. Security fixes are released together with all other fixes in each new release.

If you discover a security vulnerability in this project, please do not open a public issue.

Instead, report it privately by emailing us at digitalhub@fbk.eu. Include as much detail as possible to help us understand and address the issue quickly and responsibly.

## Contributing

To report a bug or request a feature, please first check the existing issues to avoid duplicates. If none exist, open a new issue with a clear title and a detailed description, including any steps to reproduce if it's a bug.

To contribute code, start by forking the repository. Clone your fork locally and create a new branch for your changes. Make sure your commits follow the [Conventional Commits v1.0](https://www.conventionalcommits.org/en/v1.0.0/) specification to keep history readable and consistent.

Once your changes are ready, push your branch to your fork and open a pull request against the main branch. Be sure to include a summary of what you changed and why. If your pull request addresses an issue, mention it in the description (e.g., “Closes #123”).

Please note that new contributors may be asked to sign a Contributor License Agreement (CLA) before their pull requests can be merged. This helps us ensure compliance with open source licensing standards.

We appreciate contributions and help in improving the project!

## Authors

This project is developed and maintained by **DSLab – Fondazione Bruno Kessler**, with contributions from the open source community. A complete list of contributors is available in the project’s commit history and pull requests.

For questions or inquiries, please contact: [digitalhub@fbk.eu](mailto:digitalhub@fbk.eu)

## Copyright and license

Copyright © 2025 DSLab – Fondazione Bruno Kessler and individual contributors.

This project is licensed under the Apache License, Version 2.0.
You may not use this file except in compliance with the License. Ownership of contributions remains with the original authors and is governed by the terms of the Apache 2.0 License, including the requirement to grant a license to the project.
