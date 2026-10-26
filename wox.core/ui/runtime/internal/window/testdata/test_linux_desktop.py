"""Run on Linux with GTK/X11 development files and xvfb-run; no desktop required."""

import os
from pathlib import Path
import shlex
import subprocess
import tempfile


root = Path(__file__).resolve().parents[1]
flags = shlex.split(subprocess.check_output(
    ["pkg-config", "--cflags", "--libs", "gtk+-3.0", "x11"], text=True))
with tempfile.TemporaryDirectory(prefix="wox-linux-desktop-") as temp:
    binary = Path(temp) / "test-linux-x11"
    subprocess.run([
        "cc", "-std=c11", "-Wall", "-Wextra", "-Werror", "-I", str(root),
        str(root / "native_linux_x11.c"), str(root / "testdata/test_linux_x11.c"),
        "-o", str(binary), *flags,
    ], check=True)
    for scale in ("1", "2"):
        subprocess.run(["xvfb-run", "-a", str(binary)], check=True, timeout=30,
                       env={**os.environ, "GDK_BACKEND": "x11", "GDK_SCALE": scale})
