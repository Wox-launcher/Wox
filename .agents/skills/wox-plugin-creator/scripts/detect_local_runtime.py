#!/usr/bin/env python3
"""Detect a usable local Wox plugin runtime.

Prints `nodejs`, `python`, or `none`. When both Node.js 20+ and Python 3.10+
are present, Node.js wins.
"""

from __future__ import annotations

import re
import shutil
import subprocess
import sys

MIN_NODE = (20, 0, 0)
MIN_PYTHON = (3, 10, 0)


def parse_version(text: str) -> tuple[int, int, int] | None:
    match = re.search(r"(\d+)\.(\d+)\.(\d+)", text)
    if match:
        return int(match.group(1)), int(match.group(2)), int(match.group(3))
    match = re.search(r"(\d+)\.(\d+)", text)
    if match:
        return int(match.group(1)), int(match.group(2)), 0
    return None


def command_version(args: list[str]) -> tuple[int, int, int] | None:
    try:
        completed = subprocess.run(
            args,
            capture_output=True,
            text=True,
            timeout=5,
            check=False,
        )
    except (OSError, subprocess.TimeoutExpired):
        return None
    return parse_version((completed.stdout or "") + (completed.stderr or ""))


def has_nodejs() -> bool:
    if shutil.which("node") is None:
        return False
    version = command_version(["node", "--version"])
    return version is not None and version >= MIN_NODE


def python_version_commands() -> list[list[str]]:
    commands: list[list[str]] = []
    names = ("python3", "python") if sys.platform != "win32" else ("py", "python", "python3")
    for name in names:
        if shutil.which(name) is None:
            continue
        if name == "py":
            commands.append(["py", "-3", "--version"])
        else:
            commands.append([name, "--version"])
    return commands


def has_python() -> bool:
    for args in python_version_commands():
        version = command_version(args)
        if version is not None and version >= MIN_PYTHON:
            return True
    return False


def choose_from_flags(node_ok: bool, python_ok: bool) -> str:
    if node_ok:
        return "nodejs"
    if python_ok:
        return "python"
    return "none"


def choose_runtime() -> str:
    return choose_from_flags(has_nodejs(), has_python())


def main() -> None:
    print(choose_runtime())


if __name__ == "__main__":
    main()
