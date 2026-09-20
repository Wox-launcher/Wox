"""Verify plugin tool callbacks stay on the host RPC path."""

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

from wox_plugin import (  # noqa: E402
    Context,
    InvokePluginToolHandlerResult,
    PluginToolDescriptor,
    RegisterPluginToolOption,
    UnregisterPluginToolOption,
    InvokePluginToolOption,
)
from wox_plugin_host import jsonrpc  # noqa: E402
from wox_plugin_host.plugin_api import PluginAPI  # noqa: E402
from wox_plugin_host.plugin_manager import PluginInstance, plugin_instances, waiting_for_response  # noqa: E402


class PluginToolHostTest(unittest.IsolatedAsyncioTestCase):
    async def test_unload_waits_for_handler_and_rejects_new_calls(self):
        api = PluginAPI(AsyncMock(), "notes", "Notes")
        plugin_instances["notes"] = PluginInstance(
            plugin=object(), api=api, plugin_dir="", module_name="notes", actions={}, form_actions={}, toolbar_msg_actions={}, sys_paths=[]
        )
        started, release = asyncio.Event(), asyncio.Event()

        async def handler(ctx, option):
            started.set()
            await release.wait()
            return InvokePluginToolHandlerResult(output={})

        api.plugin_tool_callbacks["cb"] = handler
        request = {"PluginId": "notes", "Params": {"CallbackId": "cb", "Arguments": "{}"}}
        invocation = asyncio.create_task(jsonrpc.on_invoke_plugin_tool(Context.new(), request))
        await started.wait()
        stopping = asyncio.create_task(api.stop_plugin_tools())
        await asyncio.sleep(0)
        self.assertFalse(stopping.done())
        rejected = await jsonrpc.on_invoke_plugin_tool(Context.new(), request)
        self.assertEqual(rejected["Error"]["Code"], "PLUGIN_UNAVAILABLE")
        release.set()
        await asyncio.wait_for(asyncio.gather(invocation, stopping), timeout=1)
        self.assertFalse(api.plugin_tool_calls)

    async def test_duplicate_failure_preserves_original_callback_until_unregister(self):
        api = PluginAPI(AsyncMock(), "notes", "Notes")
        api.invoke_method = AsyncMock(
            side_effect=[
                {"Error": None},
                {"Error": {"Code": "TOOL_ALREADY_REGISTERED", "Message": "duplicate"}},
                {"Error": {"Code": "EXECUTION_FAILED", "Message": "unregister failed"}},
                {"Error": None},
            ]
        )
        option = RegisterPluginToolOption(
            tool=PluginToolDescriptor(name="ping", description="Ping", input_schema={"type": "object"}, output_schema={"type": "object"}),
            handler=lambda ctx, option: InvokePluginToolHandlerResult(output={}),
        )
        await api.register_plugin_tool(Context.new(), option)
        original = api.plugin_tool_callback_ids["ping"]
        await api.register_plugin_tool(Context.new(), option)
        self.assertEqual(api.plugin_tool_callback_ids["ping"], original)
        self.assertEqual(len(api.plugin_tool_callbacks), 1)
        await api.unregister_plugin_tool(Context.new(), UnregisterPluginToolOption(name="ping"))
        self.assertIn(original, api.plugin_tool_callbacks)
        await api.unregister_plugin_tool(Context.new(), UnregisterPluginToolOption(name="ping"))
        self.assertFalse(api.plugin_tool_callbacks)
        self.assertFalse(api.plugin_tool_callback_ids)

    async def test_nested_invoke_forwards_parent_token(self):
        api = PluginAPI(AsyncMock(), "notes", "Notes")
        api.invoke_method = AsyncMock(return_value={"Output": {}, "Error": None})
        plugin_instances["notes"] = PluginInstance(
            plugin=object(), api=api, plugin_dir="", module_name="notes", actions={}, form_actions={}, toolbar_msg_actions={}, sys_paths=[]
        )

        async def handler(ctx, option):
            await api.invoke_plugin_tool(ctx, InvokePluginToolOption(plugin_id="other", name="ping"))
            return InvokePluginToolHandlerResult(output={})

        api.plugin_tool_callbacks["cb"] = handler
        await jsonrpc.on_invoke_plugin_tool(
            Context.new(),
            {
                "PluginId": "notes",
                "Params": {"CallbackId": "cb", "Arguments": "{}", "PluginToolCallId": "parent-token"},
            },
        )
        self.assertEqual(api.invoke_method.call_args.args[2]["PluginToolCallId"], "parent-token")

    async def asyncTearDown(self):
        plugin_instances.clear()
        waiting_for_response.clear()

    async def test_register_and_invoke_round_trip(self):
        sent = []

        async def send(raw):
            request = json.loads(raw)
            sent.append(request)
            asyncio.get_running_loop().call_soon(lambda: waiting_for_response.pop(request["Id"]).set_result({"Error": None}))

        ws = AsyncMock()
        ws.send.side_effect = send
        api = PluginAPI(ws, "notes", "Notes")
        plugin_instances["notes"] = PluginInstance(
            plugin=object(), api=api, plugin_dir="", module_name="notes", actions={}, form_actions={}, toolbar_msg_actions={}, sys_paths=[]
        )

        async def handler(ctx, option):
            return InvokePluginToolHandlerResult(output={"noteId": option.arguments["text"]})

        with patch.object(jsonrpc.logger, "info", new=AsyncMock()), patch.object(jsonrpc.logger, "error", new=AsyncMock()):
            result = await api.register_plugin_tool(
                Context.new(),
                RegisterPluginToolOption(
                    tool=PluginToolDescriptor(
                        name="echo_text", description="Echo", input_schema={"type": "object"}, output_schema={"type": "object"}
                    ),
                    handler=handler,
                ),
            )
            self.assertIsNone(result.error)
            callback_id = api.plugin_tool_callback_ids["echo_text"]
            invoked = await jsonrpc.handle_request_from_wox(
                Context.new(),
                {
                    "Method": "onInvokePluginTool",
                    "PluginId": "notes",
                    "PluginName": "Notes",
                    "Params": {"CallbackId": callback_id, "Arguments": json.dumps({"text": "hello"})},
                },
                ws,
            )
        self.assertEqual(invoked["Output"]["noteId"], "hello")
        self.assertIsNone(invoked["Error"])
        self.assertEqual(sent[0]["Method"], "RegisterPluginTool")

    async def test_handler_exception_becomes_execution_failed(self):
        api = PluginAPI(AsyncMock(), "notes", "Notes")
        plugin_instances["notes"] = PluginInstance(
            plugin=object(), api=api, plugin_dir="", module_name="notes", actions={}, form_actions={}, toolbar_msg_actions={}, sys_paths=[]
        )

        def handler(_ctx, _option):
            raise RuntimeError("boom")

        api.plugin_tool_callbacks["cb"] = handler
        with patch.object(jsonrpc.logger, "info", new=AsyncMock()), patch.object(jsonrpc.logger, "error", new=AsyncMock()):
            invoked = await jsonrpc.handle_request_from_wox(
                Context.new(),
                {
                    "Method": "onInvokePluginTool",
                    "PluginId": "notes",
                    "PluginName": "Notes",
                    "Params": {"CallbackId": "cb", "Arguments": "{}"},
                },
                AsyncMock(),
            )
        self.assertEqual(invoked["Error"]["Code"], "EXECUTION_FAILED")


if __name__ == "__main__":
    unittest.main()
