"""Exercise the real host handshake and connection lifecycle."""

import asyncio
import contextlib
import importlib.util
import json
import os
import socket
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import AsyncMock, patch

import websockets
from websockets.exceptions import InvalidStatus

# Import the host without executing the entry point's command-line parsing.
package_path = Path(__file__).resolve().parents[1] / "src" / "wox_plugin_host"
spec = importlib.util.spec_from_loader("wox_plugin_host", loader=None, is_package=True)
package = importlib.util.module_from_spec(spec)
package.__path__ = [str(package_path)]
sys.modules.setdefault("wox_plugin_host", package)

from wox_plugin_host import host, plugin_manager  # noqa: E402


class HostSecurityTest(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.log_patches = [patch.object(host.logger, name, new=AsyncMock()) for name in ("info", "error")]
        for log_patch in self.log_patches:
            log_patch.start()
        with socket.socket() as sock:
            sock.bind(("127.0.0.1", 0))
            self.port = sock.getsockname()[1]
        self.url = f"ws://127.0.0.1:{self.port}"
        self.secret = "test-secret"
        self.serve_patch = patch.object(host.websockets, "serve", wraps=host.websockets.serve)
        self.serve = self.serve_patch.start()
        self.server = asyncio.create_task(host.start_websocket(self.port, self.secret))
        for _ in range(100):
            try:
                await websockets.connect(self.url)
            except InvalidStatus:
                break
            except OSError:
                await asyncio.sleep(0.02)
        else:
            self.fail("host did not start")
        self.assertEqual(self.serve.call_args.args[1], "127.0.0.1")

    async def asyncTearDown(self):
        self.server.cancel()
        with contextlib.suppress(asyncio.CancelledError):
            await self.server
        plugin_manager.plugin_instances.clear()
        plugin_manager.set_current_connection(None)
        self.serve_patch.stop()
        for log_patch in self.log_patches:
            log_patch.stop()

    async def test_authentication_single_connection_and_reconnect(self):
        for headers, origin in [
            ({}, None),
            ({"X-Wox-Host-Secret": "wrong"}, None),
            ({"X-Wox-Host-Secret": self.secret}, "https://example.com"),
        ]:
            with self.assertRaises(InvalidStatus):
                await websockets.connect(self.url, additional_headers=headers, origin=origin)

        headers = {"X-Wox-Host-Secret": self.secret}
        async with websockets.connect(self.url, additional_headers=headers) as active:
            with self.assertRaises(InvalidStatus):
                await websockets.connect(self.url, additional_headers=headers)

            # An authenticated request can still load a single-file development plugin.
            with tempfile.TemporaryDirectory() as directory:
                Path(directory, "plugin.py").write_text("plugin = object()\n", encoding="utf-8")
                await active.send(
                    json.dumps(
                        {
                            "Type": "WOX_JSONRPC_REQUEST",
                            "Id": "test",
                            "Method": "loadPlugin",
                            "PluginId": "test",
                            "PluginName": "test",
                            "Params": {"PluginDirectory": directory, "Entry": "plugin.py", "EntryMode": "singleFile"},
                        }
                    )
                )
                response = json.loads(await asyncio.wait_for(active.recv(), 3))
                self.assertNotIn("Error", response)
                self.assertIn("test", plugin_manager.plugin_instances)

        for _ in range(100):
            if plugin_manager.current_connection is None:
                break
            await asyncio.sleep(0.01)
        async with websockets.connect(self.url, additional_headers=headers):
            self.assertIsNotNone(plugin_manager.current_connection)


class HostStartupSecurityTest(unittest.TestCase):
    def test_missing_secret_fails_startup(self):
        with tempfile.TemporaryDirectory() as directory:
            env = os.environ.copy()
            env.pop("WOX_HOST_SECRET", None)
            result = subprocess.run(
                [sys.executable, "-c", "from wox_plugin_host.__main__ import run; run()", "0", directory, str(os.getpid())],
                env=env,
                capture_output=True,
                timeout=10,
            )
            self.assertNotEqual(result.returncode, 0)
