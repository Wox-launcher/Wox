"""Check native pointer passthrough in a live Hyprland session.

This briefly displays a small Wox overlay and moves the pointer without clicking.
The fixture and cursor position are restored on success or failure.
Requires the normal Linux Wox build dependencies and hyprctl.
"""

import json
import os
from pathlib import Path
import selectors
import subprocess
import tempfile
import time


def hyprctl(*arguments):
    return subprocess.check_output(["hyprctl", *arguments], text=True, timeout=3)


def check(fixture):
    """Exercise repeated input restoration after a pass-through frame is painted."""
    original = json.loads(hyprctl("-j", "cursorpos"))
    process = subprocess.Popen(
        [str(fixture)], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
        text=True, env={**os.environ, "GDK_BACKEND": "wayland"},
    )
    selector = selectors.DefaultSelector()
    selector.register(process.stdout, selectors.EVENT_READ)

    def receive():
        if not selector.select(5):
            raise RuntimeError("native fixture timed out")
        line = process.stdout.readline()
        if not line:
            raise RuntimeError("native fixture exited")
        result = json.loads(line)
        if result.get("error"):
            raise RuntimeError(result["error"])
        return result

    def send(command):
        process.stdin.write(command + "\n")
        process.stdin.flush()
        return receive()

    def move(x, y):
        hyprctl("eval", f"hl.dispatch(hl.dsp.cursor.move({{x={x}, y={y}}}))")
        time.sleep(0.08)

    try:
        receive()
        layers = json.loads(hyprctl("-j", "layers"))
        layer = next(
            layer for monitor in layers.values() for entries in monitor["levels"].values()
            for layer in entries if layer.get("pid") == process.pid and layer["w"] == 240 and layer["h"] == 160
        )
        inside = (layer["x"] + 40, layer["y"] + 40)
        outside = (layer["x"] + layer["w"] + 40, layer["y"] + 40)
        # Commit an empty input region first; otherwise in-process picking alone
        # can make the test pass while the compositor still targets this window.
        send("pass")
        send("draw")
        time.sleep(0.15)
        move(*outside)
        before = send("status")
        move(*inside)
        assert send("status")["enters"] == before["enters"], "initial pass-through intercepted input"
        # Hyprland's exclusive keyboard layer also owns pointer input outside
        # its bounds, so a ready border cannot receive input until it is released.
        move(*outside)
        send("capture")
        time.sleep(0.15)
        before = send("status")
        move(*inside)
        assert send("status")["enters"] == before["enters"], "expected the exclusive toolbar to own input"
        send("release-keyboard")
        time.sleep(0.15)
        for cycle in range(3):
            move(*outside)
            send("capture")
            time.sleep(0.15)
            before = send("status")
            move(*inside)
            after = send("status")
            assert after["enters"] > before["enters"], "restoring input did not deliver a native pointer enter"
            send("pass")
            time.sleep(0.15)
            move(*outside)
            before = send("status")
            move(*inside)
            after = send("status")
            assert after["enters"] == before["enters"], "pass-through still intercepted native input"
            print(f"Pointer passthrough cycle {cycle + 1} passed.", flush=True)
        send("quit")
        assert process.wait(timeout=5) == 0
    finally:
        if process.poll() is None:
            process.terminate()
            try:
                process.wait(timeout=3)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
        selector.close()
        process.stdin.close()
        process.stdout.close()
        move(original["x"], original["y"])


def main():
    directory = Path(__file__).resolve().parent
    with tempfile.TemporaryDirectory(prefix="wox-pointer-regression-") as temporary:
        fixture = Path(temporary) / "fixture"
        subprocess.run(
            ["go", "build", "-tags", "sqlite_fts5", "-o", str(fixture), str(directory / "fixture_linux_pointer_passthrough.go")],
            cwd=directory.parents[4], check=True,
        )
        check(fixture)


if __name__ == "__main__":
    main()
