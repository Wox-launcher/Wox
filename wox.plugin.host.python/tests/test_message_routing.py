"""Host messages are routed by their envelope type, not by text they carry."""

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

from wox_plugin_host.constants import PLUGIN_JSONRPC_TYPE_REQUEST, PLUGIN_JSONRPC_TYPE_RESPONSE  # noqa: E402
from wox_plugin_host.host import handle_message  # noqa: E402


class MessageRoutingTest(unittest.IsolatedAsyncioTestCase):
    async def test_request_mentioning_response_type_is_handled_as_request(self):
        ws = AsyncMock()
        request = {
            "TraceId": "trace",
            "Id": "request-1",
            "Method": "query",
            "Type": PLUGIN_JSONRPC_TYPE_REQUEST,
            "Params": {"Search": PLUGIN_JSONRPC_TYPE_RESPONSE},
        }
        with patch("wox_plugin_host.host.handle_request_from_wox", new=AsyncMock(return_value="ok")) as handle_request:
            await handle_message(ws, json.dumps(request))

        handle_request.assert_awaited_once()
        response = json.loads(ws.send.await_args.args[0])
        self.assertEqual(response["Id"], "request-1")
        self.assertEqual(response["Type"], PLUGIN_JSONRPC_TYPE_RESPONSE)
        self.assertEqual(response["Result"], "ok")


if __name__ == "__main__":
    unittest.main()
