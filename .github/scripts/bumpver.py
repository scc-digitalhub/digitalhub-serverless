# SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0
"""Update a package version in the Dockerfiles and retry concurrent pushes."""

from __future__ import annotations

import logging
import os
import random
import re
import subprocess
import time
from pathlib import Path

from packaging.version import InvalidVersion, Version

LOGGER = logging.getLogger("bumpver")
DOCKERFILES = (Path("Dockerfile.handler"), Path("Dockerfile.layer"))
ALLOWED_LIBRARIES = {
    "sdk",
    "python",
    "container",
    "modelserve",
    "dbt",
    "hera",
    "flower",
    "servicegraph",
    "tvm",
    "hydra",
    "ray",
}
MAX_ATTEMPTS = 5
MIN_RETRY_DELAY = 1
MAX_RETRY_DELAY = 15


def run_git(*args: str, check: bool = True) -> subprocess.CompletedProcess:
    """Run a git command in the checked-out repository."""
    return subprocess.run(["git", *args], check=check, text=True)


def normalize_branch(branch: str) -> str:
    """Accept either a branch name or a refs/heads/<name> ref."""
    prefix = "refs/heads/"
    if branch.startswith(prefix):
        branch = branch.removeprefix(prefix)
    if not branch or branch.startswith("refs/"):
        raise RuntimeError(f"Invalid branch name: {branch}")
    run_git("check-ref-format", "--branch", branch)
    return branch


def dockerfile_with_version(
    content: str, library: str, version: str
) -> tuple[str, str]:
    """Replace one ARG value and return the updated content and current version."""
    argument = f"DH_{library.upper()}_VERSION"
    pattern = re.compile(rf"^ARG {re.escape(argument)}=(.+)$", re.MULTILINE)
    matches = list(pattern.finditer(content))
    if len(matches) != 1:
        raise RuntimeError(
            f"Expected one Dockerfile argument named {argument}, found {len(matches)}"
        )

    current_version = matches[0].group(1).strip()
    updated = pattern.sub(f"ARG {argument}={version}", content, count=1)
    return updated, current_version


def bump_dockerfile(dockerfile: Path, library: str, requested_version: str) -> bool:
    """Bump the version in one Dockerfile and return whether it changed."""
    content = dockerfile.read_text(encoding="utf-8")
    updated, current_version_text = dockerfile_with_version(
        content, library, requested_version
    )
    try:
        current = Version(current_version_text)
    except InvalidVersion as error:
        raise RuntimeError(
            f"Invalid current version for {library} in {dockerfile}: "
            f"{current_version_text}"
        ) from error

    if Version(requested_version) <= current:
        LOGGER.info(
            "%s is already at %s or newer in %s; skipping", library, current, dockerfile
        )
        return False

    dockerfile.write_text(updated, encoding="utf-8")
    return True


def update_and_push(branch: str, library: str, requested_version: str) -> None:
    """Apply the requested bump on the latest branch state and retry rejected pushes."""
    if library not in ALLOWED_LIBRARIES:
        raise RuntimeError(f"Unsupported library name: {library}")

    Version(requested_version)
    branch = normalize_branch(branch)
    run_git("config", "user.name", os.environ["GITHUB_ACTOR"])
    run_git(
        "config", "user.email", f"{os.environ['GITHUB_ACTOR']}@users.noreply.github.com"
    )

    for attempt in range(1, MAX_ATTEMPTS + 1):
        LOGGER.info("Push attempt %s/%s", attempt, MAX_ATTEMPTS)
        run_git("fetch", "origin", f"refs/heads/{branch}")
        run_git("reset", "--hard", "FETCH_HEAD")

        changed = [
            str(dockerfile)
            for dockerfile in DOCKERFILES
            if bump_dockerfile(dockerfile, library, requested_version)
        ]
        if not changed:
            return

        run_git("add", *changed)
        staged_diff = run_git("diff", "--cached", "--quiet", check=False)
        if staged_diff.returncode == 0:
            LOGGER.info("No Dockerfile changes to commit")
            return
        if staged_diff.returncode != 1:
            raise RuntimeError("Unable to inspect staged Dockerfile changes")

        run_git("commit", "-m", f"bump: bumpver {library} to {requested_version}")
        push = run_git("push", "origin", f"HEAD:refs/heads/{branch}", check=False)
        if push.returncode == 0:
            LOGGER.info("Pushed %s version %s", library, requested_version)
            return

        if attempt < MAX_ATTEMPTS:
            delay = random.randint(MIN_RETRY_DELAY, MAX_RETRY_DELAY)
            LOGGER.warning("Push rejected; retrying in %ss", delay)
            time.sleep(delay)

    raise RuntimeError(f"Unable to push {library} after {MAX_ATTEMPTS} attempts")


def main() -> int:
    logging.basicConfig(level=logging.INFO, format="%(levelname)s: %(message)s")
    try:
        update_and_push(
            branch=os.environ["BRANCH"],
            library=os.environ["LIB_NAME"],
            requested_version=os.environ["LIB_VERSION"],
        )
    except (
        KeyError,
        InvalidVersion,
        RuntimeError,
        subprocess.CalledProcessError,
    ) as error:
        LOGGER.error("%s", error)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
