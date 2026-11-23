<!--
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
-->

# DigitalHub TVM Runtime Go

The image **`ghcr.io/scc-digitalhub/tvm-runtime-go`** serves a model compiled by
[Apache TVM](https://tvm.apache.org/) with the **Open Inference Protocol v2**: REST on
port `8080` and gRPC on port `9000`. Inside it runs the Nuclio processor of this repository
with the `tvm` runtime of this folder, which calls TVM through cgo, with no Python.

TVM is registered only when CGO is enabled. Generic processor builds with
`CGO_ENABLED=0` keep the other runtimes and do not require the TVM libraries.

It is the default serve image for a DigitalHub CORE **`tvm+serve`** run. **DigitalHub TVM
Runtime Rust** implements the same inference protocol and can be used instead.

Each worker loads its own `model.so` and `metadata.json`, then executes one
inference at a time. Workers provide parallel requests; `TVM_NUM_THREADS` controls
threads within each inference. No model-specific image is needed.
CPU remains the default, including ARM32, with no NVIDIA dependencies. Optional
[CUDA FP32 builds](#optional-cuda-fp32) use separate AMD64/ARM64 runtime images.

## Quick start

Download a model compiled by `tvm+compile` for your machine and start the image:

```bash
dhcli download model -p my-project -n my-model-amd64 -d ./my-model   # model.so + metadata.json

docker run --rm -p 8080:8080 -p 9000:9000 \
  -v "$PWD/my-model:/shared/model" \
  -e TVM_MODEL_NAME=my-model \
  ghcr.io/scc-digitalhub/tvm-runtime-go:0.26.0-r3

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

Size workers, CPU and memory for the expected concurrency. In `0.26.0-r3`, a
request waiting for a free Nuclio worker returns HTTP `503` / gRPC
`RESOURCE_EXHAUSTED` when worker allocation times out. The older `0.26.0-r2`
image can return HTTP `500` for this condition. This capacity error is
separate from invalid input. Native TVM execution failures retain their existing
HTTP `500` / gRPC `UNKNOWN` behavior.

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

Read the metadata first and match its input names, datatypes, shapes and value
counts. For a model with `x: FP32 [1,4]`, a complete request is:

```bash
curl -X POST http://localhost:8080/v2/models/my-model/infer \
  -H 'Content-Type: application/json' \
  -d '{"inputs":[{"name":"x","datatype":"FP32","shape":[1,4],"data":[1.0,2.0,3.0,4.0]}]}'
```

This example must be adapted for other signatures, including image models. Images
must be preprocessed by the client; the server consumes tensors. The versioned
inference route serves the same loaded model, not a model-version repository.

- **Inputs** are matched by name when every input has one and the names match the model,
  otherwise by position. A missing REST `datatype` means `FP32`.
- **Data types**: `FP32`, `FP64`, `INT8`, `INT16`, `INT32`, `INT64`, `UINT8`, `UINT16`,
  `UINT32`, `UINT64`.
  Integer JSON values retain full `INT64`/`UINT64` precision through REST and gRPC.
- **Size limits**: 512 MiB for a REST request and for a gRPC message.
- **Quantized models** (`int8` / `uint8` tensors): the REST model metadata adds `scale`,
  `zero_point` and `quantized_dimension` under `parameters`, so the client can convert the
  values (`real = (q - zero_point) * scale`).
- **gRPC clients** use the proto `pkg/proto/inference/v2/grpc_service.proto`.

### Input validation

The runtime validates the model's datatype, rank and static dimensions before
calling TVM. Negative metadata dimensions are dynamic; request dimensions must
be nonnegative. Input names still use the positional fallback described above.
Invalid input returns HTTP `400` / gRPC `INVALID_ARGUMENT`, including wrong
element counts, unsupported datatypes, nonfinite values and numeric overflow. Integer values must
be integral and within the declared datatype's range; `1.0` and `1e1` are accepted
without truncating fractional values or wrapping overflow.

gRPC accepts either typed `contents` or little-endian `raw_input_contents`,
never both. Typed contents must use the field matching the declared datatype.
Raw encoding requires exactly one buffer per input; malformed buffers are
rejected. gRPC inference and metadata requests with unknown or empty model names
return `NOT_FOUND`; `ModelReady` returns `ready: false`.

These validation fixes are included in `0.26.0-r3`; the older `0.26.0-r2`
image does not include them. No model recompilation is needed
when the TVM version and revision remain unchanged.

## Which models it serves

At startup the runtime refuses the model, with a clear error, unless:

- `metadata.json` has the same `tvm_version` and `tvm_git_commit` as the TVM built into the
  image: **compile with a DigitalHub TVM Toolkit containing the same TVM version and commit**;
- the model's Linux GNU/glibc host matches the image architecture: an LLVM CPU
  target, or a CUDA target with an explicit nested LLVM host in a CUDA-enabled build;
- `target_triple`, when present, agrees with the runtime, and the actual ELF library
  has the matching machine, 32/64-bit class and byte order;
- every input and output uses a supported data type.

Runtime image builds cover `linux/amd64` (Intel/AMD 64-bit), `linux/arm64`
(ARM 64-bit/AArch64) and `linux/arm/v7` (ARM 32-bit). CORE's `arm32` profile produces
an ARMv7 hard-float model requiring NEON, for the `linux/arm/v7` runtime.

Compile profiles live in CORE's `runtime-tvm.yaml`, not in this runtime. Go reads
the effective target from the model, so adding a profile for an already compatible
CPU does not require a server change. Older metadata can omit `target_triple`;
the existing `target`, TVM version and revision checks still apply. ARM32 requires
the hard-float ABI.

Architecture checks do not verify every instruction selected by `mcpu`/`mattr`;
choose a compatible CPU. Adding a compile profile does not create a runtime image
for another platform. CUDA requires the separate build described below.

## Optional CUDA FP32

The CUDA build loads the standard CUDA TVM runtime alongside the CPU host runtime.
Artifact metadata selects the backend; no new serving environment or device field
is needed. CUDA targets require an explicit GPU architecture and nested LLVM host,
for example the Jetson Orin target:

```json
{"kind":"cuda","arch":"sm_87","host":{"kind":"llvm","mtriple":"aarch64-linux-gnu"}}
```

The host triple, actual ELF architecture and TVM version/commit checks still apply.
A CPU build rejects CUDA artifacts. A CUDA build uses visible CUDA device 0 plus
the CPU VM host and fails if its backend, driver or device is unavailable; it does
not fall back to CPU. CUDA tensors are copied and GPU work is synchronized before
the response is returned. Requests and supported protocol datatypes are unchanged;
the initial GPU validation scope is FP32. FP16/BFLOAT16 protocol inputs and outputs remain unsupported;
internal mixed precision kernels and accuracy require separate hardware qualification.

Build from the repository root with a matching, attested CUDA TVM build:

```bash
TVM_BACKEND=cuda TVM_HOME=/path/to/tvm-0.26.0 TVM_BUILD=/path/to/cuda-build \
  TAG=local/tvm-runtime-go:0.26-cuda13.2.1 images/tvm/build.sh
```

The build uses `images/tvm/Dockerfile.cuda` and the Go `cuda` build tag. Provenance
must attest CPU/CUDA backends and matching hashes for `libtvm_runtime.so`,
`libtvm_ffi.so` and `libtvm_runtime_cuda.so`. The final image has no Python, `nvcc`
or driver stubs. CUDA images support Linux AMD64/ARM64; CPU builds retain ARMv7.
The `local/...` tag is a development example, not a published GHCR CUDA release.
The builder needs `CUDA_HOME=/usr/local/cuda` by default, or explicit
`CUDA_CUDART_LIBRARY` and `CUDA_DRIVER_STUB_LIBRARY` paths to CUDA 13.2.1's
`libcudart.so.13` and build-only `libcuda.so` stub. Those files support linking and
tests in the builder; the driver stub is excluded from the final runtime image.

On a configured AMD64 NVIDIA host, start a CUDA artifact with real device access:

```bash
docker run --rm --gpus all -p 8080:8080 -p 9000:9000 \
  -v "$PWD/my-cuda-model:/shared/model" -e TVM_MODEL_NAME=my-model \
  local/tvm-runtime-go:0.26-cuda13.2.1
```

For Jetson, use the ARM64 image and GPU container configuration compatible with its
installed JetPack stack. Start with one worker; every worker has a separate model
copy and GPU memory allocation. CORE uses the existing `image`, GPU `profile`,
`resources.gpu` and `envs` settings. Set `so_model_path` to the versioned Model key
returned by the selected Compile Run. Selecting a profile does not choose or
recompile a Model.

The intended Jetson baseline is JetPack 7.2 / L4T 39.2 / CUDA 13.2.1 / Ubuntu 24.04.
Actual Jetson and AMD64 NVIDIA execution, accuracy, tuning and performance remain
**NOT_RUN**. Offline compilation and unavailable-device tests are not GPU inference
validation. See the [CUDA guide](https://scc-digitalhub.github.io/docs/0.16/runtimes/tvm/#cuda-fp32-optional).

## Use from DigitalHub CORE

Go is CORE's default `RUNTIME_TVM_SERVE` image. To select this release, set
`RUNTIME_TVM_SERVE=ghcr.io/scc-digitalhub/tvm-runtime-go:0.26.0-r3` on CORE or set
`image` on one serve Run. TVM-specific settings belong to the **Run**, while the
Task supplies common Kubernetes settings.

The Function keeps the reference to the IR produced by build. Each Compile Run
records its own compiled Model key in `status.outputs.compiled_so`. The serve Run
requires that versioned key in `so_model_path`; CORE does not take it from the
Function. Another compile does not change the Model selected by an existing
serve Run. For example:

```yaml
kind: tvm+serve:run
spec:
  task: tvm+serve://my-project/<task-id>
  so_model_path: store://my-project/model/tvm-so/my-model-amd64:<compiled-model-id>
  image: ghcr.io/scc-digitalhub/tvm-runtime-go:0.26.0-r3
  served_name: my-model
  workers: 1
  resources: {cpu: "4", mem: 2Gi}
```

| Run option | Default | Effect |
| --- | --- | --- |
| `so_model_path` | required | Versioned `tvm-so` Model key from the selected Compile Run's `status.outputs.compiled_so`. |
| `served_name` | Function name | Model name in `/v2/models/<name>` and `TVM_MODEL_NAME`. |
| `image` | `RUNTIME_TVM_SERVE` | Runtime image for this Run. |
| `replicas` | `1` | Number of pods. |
| `workers` | `1` | Model copies and parallel requests per pod. |
| `service_type` | `ClusterIP` | Kubernetes Service type. |
| `service_name` | unset | Additional Service name, `<function>-<service_name>`; does not rename the model. |

CORE downloads the Model with an init container, sets the model/worker environment
and derives TVM threads from the Run's CPUs divided by workers (at least one).
`envs` can explicitly set `TVM_NUM_THREADS`. The Run `profile` selects scheduling
settings, not a compile target. Its node selector must match the Model architecture
and CPU requirements. The platform reports the endpoint in `status.inference_v2`.

Runtime builds cover AMD64, ARM64 and ARMv7. CORE's default init image covers only
AMD64/ARM64, so CORE serving on ARM32 also needs an ARMv7 init image. Standalone
Docker serving with a mounted Model does not use it. See the
[CORE guide](https://github.com/scc-digitalhub/digitalhub-core/tree/main/runtimes/runtime-tvm)
for the Function/Task/Run relationship and target configuration.

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

The current published runtime image is `0.26.0-r3`, built with Apache TVM `0.26.0`.
The workflow `.github/workflows/tvm-runtime-go-image.yml` can publish the current
`main` with separate TVM and image versions. Choose a new, unused image tag for
the next release, then run it manually from GitHub Actions, or replace
`<new-image-tag>` below:

```bash
gh workflow run tvm-runtime-go-image.yml --repo scc-digitalhub/digitalhub-serverless --ref main \
  -f tvm_version=0.26.0 -f image_tag='<new-image-tag>'
```

`tvm_version` selects Apache TVM and its bundled FFI. `image_tag` names this runtime
release and can change independently of TVM `0.26.0`. Use a new image
tag for each release; image tags do not determine model compatibility. Manual runs
accept only `main`, and all architecture jobs check out the exact commit selected
when the run starts. Pushing to `main` alone does not publish an image.

The legacy flow remains: pushing `tvm-X.Y.Z` (or `tvm-X.Y`) builds that Git tag and
publishes `:X.Y.Z` (or `:X.Y`, using TVM `X.Y.0`). **Do not create a GitHub Release
for these tags**: releases of this repository publish its other runtimes.

For each of `amd64`, `arm64` and `armv7`, the workflow builds the TVM runtime and
processor, verifies the platform and packaged library hashes, and pushes
`<image_tag>-<arch>`. On native `amd64` and `arm64` jobs, it also runs the TVM runtime
and Open Inference transport tests. The final `<image_tag>` is published only after
all builds pass. It uses their immutable digests and checks that all three images
contain the same source commit and TVM/FFI versions and revisions. Failed architecture jobs can be
retried within the same workflow run.

`/opt/tvm/provenance.json` records the repository, source commit, image tag, TVM/FFI
versions and commits, library hashes and workflow run. The image label
`org.digitalhub.source.revision` identifies the Go source commit; the existing TVM
revision label continues to identify Apache TVM.

CUDA has a separate manual workflow, `tvm-runtime-go-cuda-image.yml`, covering
AMD64/ARM64 and a distinct CUDA image package/tag. It accepts publication from
`main` only. These development instructions publish no CUDA release and retain
the existing CPU tags.

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

Manual CUDA builds and tests add `-tags cuda` and use matching CUDA-enabled TVM
libraries. On real NVIDIA hardware, `TVM_TEST_MODEL` can select a compiled FP32
`[4]` fixture returning `x + 1`, and `TVM_TEST_BACKEND=cuda` selects its CUDA backend:

```bash
TVM_TEST_MODEL=/path/to/cuda-model.so TVM_TEST_BACKEND=cuda \
  go test -tags cuda ./pkg/processor/runtime/tvm/tvmrelax -run TestFP32Model -v
```

The test checks repeated outputs and handle lifetimes. Run the corresponding LLVM
fixture with `TVM_TEST_BACKEND=llvm` for CPU comparison. Missing fixtures are skipped,
not GPU passes. `TVM_TEST_CUDA_UNAVAILABLE=1` enables the missing-device check in a
CUDA build; an isolated driver stub may support that test process only. Never use
stubs for real inference or copy them into the runtime image.

## Limitations

- CPU is the default; optional CUDA FP32 requires a separate build and real NVIDIA device access.
- One model per processor; no automatic request batching.
- At most 64 output tensors per inference; larger results are rejected, never truncated.
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

For Orin with the current ARM64 base, the host needs NVIDIA Container Toolkit
1.19.1 or newer (`nvidia-ctk --version`); 1.19.0 fails before the entrypoint in the
CUDA compatibility hook. See the [CUDA deployment guide](https://scc-digitalhub.github.io/docs/0.16/runtimes/tvm/#cuda-fp32-optional).

Local image builds retain an isolated context; `TVM_BUILD_CONTEXT` selects a new
directory. CPU publication rejects CUDA tags, CUDA tags require a `cuda13.2.1`
component, and publication checks both architecture tags and the final manifest.
Existing registry tags and failed lookups stop publication. Rebuilt releases need
new tags. These guards do not publish any image by themselves.
