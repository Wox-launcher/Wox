"""Verify callback routing survives the host RPC round trip."""

import asyncio
import importlib.util
import json
import sys
import unittest
from pathlib import Path
from unittest.mock import AsyncMock, patch

# Import host modules without starting the executable's process monitor.
package_path = Path(__file__).resolve().parents[1] / "src" / "wox_plugin_host"
spec = importlib.util.spec_from_loader("wox_plugin_host", loader=None, is_package=True)
package = importlib.util.module_from_spec(spec)
package.__path__ = [str(package_path)]
sys.modules.setdefault("wox_plugin_host", package)

from wox_plugin import Context
from wox_plugin_host import jsonrpc
from wox_plugin_host.plugin_api import PluginAPI
from wox_plugin_host.plugin_manager import waiting_for_response


class DragContextTest(unittest.IsolatedAsyncioTestCase):
    async def test_callback_api_preserves_source_window(self):
        sent = []

        async def send(raw):
            request = json.loads(raw)
            sent.append(request)
            asyncio.get_running_loop().call_soon(lambda: waiting_for_response.pop(request["Id"]).set_result(None))

        ws = AsyncMock()
        ws.send.side_effect = send
        api = PluginAPI(ws, "droppy", "Droppy")

        async def callback(ctx, request):
            await api.invoke_method(ctx, "RefreshQuery", {})

        with patch.object(jsonrpc.logger, "info", new=AsyncMock()), patch.object(jsonrpc, "on_drag_out", side_effect=callback):
            await jsonrpc.handle_request_from_wox(
                Context.new(),
                {"Method": "onDragOut", "PluginName": "Droppy", "SessionId": "secondary", "QueryId": "source-query"},
                ws,
            )
        self.assertEqual(sent[0]["SessionId"], "secondary")
        self.assertEqual(sent[0]["QueryId"], "source-query")
        self.assertEqual(sent[0]["Method"], "RefreshQuery")


if __name__ == "__main__":
    unittest.main()
