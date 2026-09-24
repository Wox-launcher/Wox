"""Plugin API calls follow the host connection that is current at send time."""

import asyncio
import importlib.util
import json
import sys
import unittest
from pathlib import Path
from unittest.mock import AsyncMock, patch

package_path = Path(__file__).resolve().parents[1] / "src" / "wox_plugin_host"
spec = importlib.util.spec_from_loader("wox_plugin_host", loader=None, is_package=True)
package = importlib.util.module_from_spec(spec)
package.__path__ = [str(package_path)]
sys.modules.setdefault("wox_plugin_host", package)

from wox_plugin import Context  # noqa: E402
from wox_plugin_host.plugin_api import PluginAPI  # noqa: E402
from wox_plugin_host.plugin_manager import close_current_connection, set_current_connection, waiting_for_response  # noqa: E402


class CurrentConnectionTest(unittest.IsolatedAsyncioTestCase):
    async def asyncTearDown(self):
        waiting_for_response.clear()
        set_current_connection(None)

    async def test_invoke_uses_the_replacement_connection(self):
        sent = []

        async def send(raw):
            sent.append(json.loads(raw))

        original = AsyncMock()
        replacement = AsyncMock()
        replacement.send.side_effect = send
        api = PluginAPI(original, "tasknotes", "TaskNotes")
        saved = api
        set_current_connection(replacement)

        invoke = asyncio.create_task(api.invoke_method(Context.new(), "HideApp", {}))
        await asyncio.sleep(0)
        self.assertFalse(invoke.done())
        self.assertEqual(close_current_connection(original), False)
        waiting_for_response.pop(sent[0]["Id"]).set_result(None)
        await invoke

        self.assertIs(api, saved)
        self.assertEqual(original.send.await_count, 0)
        self.assertEqual(sent[0]["Method"], "HideApp")

    async def test_close_fails_a_call_waiting_on_the_current_connection(self):
        ws = AsyncMock()
        api = PluginAPI(ws, "tasknotes", "TaskNotes")
        with patch("wox_plugin_host.plugin_api.logger.error", new=AsyncMock()):
            invoke = asyncio.create_task(api.invoke_method(Context.new(), "HideApp", {}))
            await asyncio.sleep(0)
            self.assertEqual(close_current_connection(ws), True)
            with self.assertRaisesRegex(RuntimeError, "host websocket disconnected"):
                await invoke
        self.assertFalse(waiting_for_response)


if __name__ == "__main__":
    unittest.main()
