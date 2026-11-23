# TVM image releases

## Project releases and image tags

A project Git tag such as `0.16.1` identifies this repository's source release.
It is independent of Apache TVM (`0.26.0`) and CUDA (`13.2.1`). The supported
TVM/FFI commits and dependency versions are recorded in `images/tvm/release-images.json`.
Changing CUDA requires updating and validating its pinned build recipes as well.

Pushing a project tag `X.Y.Z` starts `tvm-runtime-go-image.yml`. It builds CPU and CUDA
images from the exact source tag and generates these tags automatically:

```text
ghcr.io/scc-digitalhub/tvm-runtime-go:0.16.1-tvm0.26.0
ghcr.io/scc-digitalhub/tvm-runtime-go-cuda:0.16.1-tvm0.26.0-cuda13.2.1
```

Each tag is a multi-architecture index. Individual images have `-amd64` and
`-arm64` suffixes. Rust/Go CPU images also include `-armv7` (cross-built and checked under QEMU).
CUDA images remain separate from CPU images. GPU execution, accuracy and Jetson
qualification require hardware and are not certified by hosted CI runners.

The image labels and provenance distinguish the project release, its source
commit, the build-recipe commit and the upstream TVM/FFI revisions. Existing
image tags cannot be overwritten. Updating these images does not change CORE's
configured image references automatically.

The same workflow supports manual execution for a source tag containing its
dependency configuration and CUDA support:

```bash
gh workflow run tvm-runtime-go-image.yml --ref main \
  -f release_tag='<new-project-tag>' -f publish=true
```

The workflow recipe is taken from the selected ref; the source is resolved from
`release_tag`. Publication requires that tag to exist and be an ancestor of the
workflow commit. On a validation branch, `publish=false` builds without pushing;
a release tag that does not yet exist resolves to that branch commit for validation
only. No release tag or GitHub Release is created automatically.

To recover a failed manifest, run the updated workflow with the original source
tag and `manifest_run_id=<previous run ID>`. The build jobs are skipped. Only
digest artifacts whose platform, release, source commit and TVM/FFI versions match
are accepted. A matching existing index is reused; a different index is rejected.
This supports recovery without deleting images or rebuilding successful binaries.
CPU and CUDA artifacts have separate names.

Only the TVM workflow responds to project tag pushes. The existing non-TVM image
workflows continue to run when a GitHub Release is published; they are unchanged.
The Go source is checked out separately from the workflow's TVM build recipes.
Native tests and the Docker build use that source checkout. The historical
`0.16.1` source tag predates CUDA support and cannot produce CUDA images. Keep
that tag unchanged and use a new project release containing the CUDA code.
