#!/usr/bin/env python3
# SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0

import argparse
import hashlib
import json
import re
import subprocess
from pathlib import Path


REQUIRED_LIBRARIES = {
    "libtvm_runtime.so",
    "libtvm_runtime_extra.so",
    "libtvm_ffi.so",
    "libtvm_compiler.so",
}


CUDA_LIBRARIES = {"libtvm_runtime_cuda.so"}
CUDA_REVISION = "c7b458e946bc4266915da582457476bdcd9705ae"


def verify_backend_libraries(provenance: dict, backend: str) -> set[str]:
    # Legacy CPU attestations predate compiled_backends. Keep their exact library
    # check, including rejection of unrequested CUDA/backend libraries.
    backends = provenance.get("compiled_backends", ["cpu"])
    expected_backends = ["cpu", "cuda"] if backend == "cuda" else ["cpu"]
    require(
        backends == expected_backends,
        f"TVM provenance compiled_backends must be {expected_backends!r} for {backend}",
    )
    expected_libraries = REQUIRED_LIBRARIES | (CUDA_LIBRARIES if backend == "cuda" else set())
    require(
        set(provenance.get("libraries", {})) == expected_libraries,
        "TVM provenance does not contain the complete requested backend library set",
    )
    if backend == "cuda":
        version = provenance.get("cuda_build", {}).get("cuda_version", provenance.get("cuda_version"))
        require(version == "13.2.1", f"CUDA serve image requires an attested CUDA 13.2.1 build, got {version!r}")
    return expected_libraries


def verify_cuda_build(provenance: dict, build: Path) -> None:
    require(provenance.get("tvm_git_commit") == CUDA_REVISION,
            "CUDA image requires the pinned TVM 0.26.0 revision")
    proof = provenance.get("cuda_build", {})
    for key in ("tvm_git_commit", "tvm_ffi_git_commit"):
        require(proof.get(key) == provenance.get(key), f"CUDA provenance {key} mismatch")
    require(proof.get("build_target") == "tvm_runtime_cuda",
            "CUDA provenance has no runtime build target")
    require(proof.get("cuda_version") == "13.2.1", "CUDA image requires CUDA 13.2.1")
    require(proof.get("library_sha256") == provenance.get("libraries", {}).get("libtvm_runtime_cuda.so"),
            "CUDA sidecar hash differs from TVM library provenance")
    cache = build / "cuda-CMakeCache.txt"
    if not cache.is_file():
        cache = build / "CMakeCache.txt"
    require(cache.is_file() and digest(cache) == proof.get("cmake_cache_sha256"),
            "CUDA CMake cache does not match provenance")
    settings = re.findall(r"(?m)^USE_CUDA:[^=]+=(.*)$", cache.read_text())
    require(bool(settings), "CUDA CMake cache does not record USE_CUDA")
    setting = settings[-1].strip('"')
    require("$" not in setting and setting.upper() not in
            {"", "0", "OFF", "NO", "FALSE", "N", "IGNORE", "NOTFOUND"},
            f"CUDA CMake cache has no explicit enabled USE_CUDA: {setting!r}")


def git(source: Path, *arguments: str) -> str:
    return subprocess.run(
        ["git", "-C", str(source), *arguments],
        check=True,
        capture_output=True,
        text=True,
    ).stdout.strip()


def digest(path: Path) -> str:
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit(message)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", required=True, type=Path)
    parser.add_argument("--build", required=True, type=Path)
    parser.add_argument("--backend", choices=("cpu", "cuda"), default="cpu")
    parser.add_argument("--exported-artifacts", action="store_true",
                        help="verify a CUDA toolkit library export against this pinned source checkout")
    args = parser.parse_args()
    require(not args.exported_artifacts or args.backend == "cuda",
            "exported-artifact verification is only available for the CUDA variant")

    source = args.source.resolve()
    build = args.build.resolve()
    release_ref = git(source, "describe", "--tags", "--exact-match")
    match = re.fullmatch(r"v(\d+\.\d+\.\d+)", release_ref)
    require(match is not None, f"TVM checkout is not an exact stable release: {release_ref}")
    assert match is not None
    require(
        not git(source, "status", "--porcelain", "--untracked-files=no"),
        f"TVM checkout contains tracked changes: {source}",
    )
    submodules = git(source, "submodule", "status", "--recursive").splitlines()
    invalid_submodules = [line for line in submodules if line[:1] in {"-", "+", "U"}]
    require(
        not invalid_submodules,
        "TVM submodules are not aligned:\n" + "\n".join(invalid_submodules),
    )

    version = match.group(1)
    tvm_commit = git(source, "rev-parse", "HEAD")
    ffi_commit = git(source / "3rdparty/tvm-ffi", "rev-parse", "HEAD")
    provenance_path = build / "digitalhub-build-provenance.json"
    if args.exported_artifacts and not provenance_path.is_file():
        provenance_path = build / "provenance.json"
    try:
        provenance = json.loads(provenance_path.read_text())
    except (OSError, json.JSONDecodeError) as error:
        raise SystemExit(
            f"invalid or missing {provenance_path}; attest the build with "
            "digitalhub-tvm-toolkit/build-tvm.sh"
        ) from error

    expected = {
        "schema_version": 1,
        "tvm_version": version,
        "tvm_git_commit": tvm_commit,
        "tvm_ffi_git_commit": ffi_commit,
    }
    if not args.exported_artifacts:
        expected["source_dir"] = str(source)
    mismatches = [
        f"{key}: {provenance.get(key)!r} != {value!r}"
        for key, value in expected.items()
        if provenance.get(key) != value
    ]
    require(not mismatches, "TVM provenance mismatch: " + "; ".join(mismatches))

    cache_source = next(
        (
            Path(line.partition("=")[2]).resolve()
            for line in ((build / "CMakeCache.txt").read_text().splitlines()
                         if not args.exported_artifacts else [])
            if line.startswith("CMAKE_HOME_DIRECTORY:INTERNAL=")
        ),
        None,
    )
    require(
        args.exported_artifacts or cache_source == source,
        f"TVM build {build} was configured from {cache_source}, not {source}",
    )

    libraries = provenance.get("libraries", {})
    verify_backend_libraries(provenance, args.backend)
    if args.backend == "cuda":
        verify_cuda_build(provenance, build)
    changed_libraries = [
        str(build / "lib" / name)
        for name, expected_hash in libraries.items()
        if not (build / "lib" / name).is_file()
        or digest(build / "lib" / name) != expected_hash
    ]
    require(
        not changed_libraries,
        "TVM libraries do not match provenance: " + ", ".join(changed_libraries),
    )

    tag = ".".join(version.split(".")[:2])
    print(
        "\t".join(
            (
                version,
                tag,
                tvm_commit,
                ffi_commit,
                provenance["tvm_ffi_version"],
            )
        )
    )


if __name__ == "__main__":
    main()
