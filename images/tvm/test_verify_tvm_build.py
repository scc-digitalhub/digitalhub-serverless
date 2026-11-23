# SPDX-FileCopyrightText: © 2026 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0

import importlib.util
import tempfile
import unittest
from pathlib import Path


spec = importlib.util.spec_from_file_location(
    "verify_tvm_build", Path(__file__).with_name("verify-tvm-build.py")
)
verifier = importlib.util.module_from_spec(spec)
spec.loader.exec_module(verifier)


class BackendProvenanceTests(unittest.TestCase):
    def manifest(self, backend=None):
        libraries = verifier.REQUIRED_LIBRARIES.copy()
        manifest = {"libraries": dict.fromkeys(libraries, "sha256")}
        if backend:
            manifest["compiled_backends"] = ["cpu", "cuda"]
            manifest["cuda_build"] = {"cuda_version": "13.2.1"}
            manifest["libraries"].update(dict.fromkeys(verifier.CUDA_LIBRARIES, "sha256"))
        return manifest

    def test_legacy_cpu_and_explicit_cpu(self):
        manifest = self.manifest()
        verifier.verify_backend_libraries(manifest, "cpu")
        manifest["compiled_backends"] = ["cpu"]
        verifier.verify_backend_libraries(manifest, "cpu")

    def test_cpu_rejects_cuda_and_unknown_libraries(self):
        with self.assertRaises(SystemExit):
            verifier.verify_backend_libraries(self.manifest("cuda"), "cpu")
        manifest = self.manifest()
        manifest["libraries"]["libtvm_runtime_unknown.so"] = "sha256"
        with self.assertRaises(SystemExit):
            verifier.verify_backend_libraries(manifest, "cpu")

    def test_cuda_requires_backend_and_library_attestation(self):
        verifier.verify_backend_libraries(self.manifest("cuda"), "cuda")
        with self.assertRaises(SystemExit):
            verifier.verify_backend_libraries(self.manifest(), "cuda")
        manifest = self.manifest("cuda")
        del manifest["libraries"]["libtvm_runtime_cuda.so"]
        with self.assertRaises(SystemExit):
            verifier.verify_backend_libraries(manifest, "cuda")
        manifest = self.manifest("cuda")
        del manifest["compiled_backends"]
        with self.assertRaises(SystemExit):
            verifier.verify_backend_libraries(manifest, "cuda")

    def test_cuda_rejects_incompatible_runtime_version(self):
        manifest = self.manifest("cuda")
        manifest["cuda_build"]["cuda_version"] = "12.6"
        with self.assertRaises(SystemExit):
            verifier.verify_backend_libraries(manifest, "cuda")

    def test_export_requires_matching_sidecar_and_enabled_cache(self):
        with tempfile.TemporaryDirectory() as directory:
            build = Path(directory)
            cache = build / "cuda-CMakeCache.txt"
            cache.write_text("USE_CUDA:UNINITIALIZED=/usr/local/cuda\n")
            manifest = self.manifest("cuda")
            manifest.update(tvm_git_commit=verifier.CUDA_REVISION, tvm_ffi_git_commit="ffi-revision")
            manifest["cuda_build"].update(
                tvm_git_commit=verifier.CUDA_REVISION,
                tvm_ffi_git_commit="ffi-revision",
                build_target="tvm_runtime_cuda",
                library_sha256=manifest["libraries"]["libtvm_runtime_cuda.so"],
                cmake_cache_sha256=verifier.digest(cache),
            )
            verifier.verify_cuda_build(manifest, build)
            cache.write_text("USE_CUDA:UNINITIALIZED=OFF\n")
            with self.assertRaises(SystemExit):
                verifier.verify_cuda_build(manifest, build)
            manifest["cuda_build"]["cmake_cache_sha256"] = verifier.digest(cache)
            with self.assertRaises(SystemExit):
                verifier.verify_cuda_build(manifest, build)
            cache.write_text("USE_CUDA:UNINITIALIZED=ON\n")
            manifest["cuda_build"]["cmake_cache_sha256"] = verifier.digest(cache)
            manifest["cuda_build"]["library_sha256"] = "wrong-sidecar"
            with self.assertRaises(SystemExit):
                verifier.verify_cuda_build(manifest, build)


if __name__ == "__main__":
    unittest.main()
