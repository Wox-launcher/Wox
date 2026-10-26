"""The Python host must call core AIChatStream with model and options."""

import asyncio
import importlib.util
import json
import sys
import unittest
from pathlib import Path
from unittest.mock import AsyncMock

package_path = Path(__file__).resolve().parents[1] / "src" / "wox_plugin_host"
spec = importlib.util.spec_from_loader("wox_plugin_host", loader=None, is_package=True)
package = importlib.util.module_from_spec(spec)
package.__path__ = [str(package_path)]
sys.modules.setdefault("wox_plugin_host", package)

from wox_plugin import AIModel, ChatLoopPolicy, ChatOptions, Context, Conversation  # noqa: E402
from wox_plugin_host import jsonrpc  # noqa: E402
from wox_plugin_host.plugin_api import PluginAPI  # noqa: E402
from wox_plugin_host.plugin_manager import PluginInstance, plugin_instances  # noqa: E402


def drive(coro):
    """Run a coroutine that only awaits other coroutines, without an event loop."""
    result = None
    while True:
        try:
            inner = coro.send(result)
        except StopIteration as done:
            return done.value
        result = drive(inner)


class AIChatStreamHostTest(unittest.TestCase):
    def test_stream_request_matches_core_contract(self):
        api = PluginAPI(AsyncMock(), "notes", "Notes")
        sent = {}

        async def invoke(_ctx, method, params):
            sent["method"] = method
            sent["params"] = params

        api.invoke_method = invoke
        conversation = Conversation.new_user_message("Hello")
        conversation.timestamp = 1
        drive(
            api.ai_chat_stream(
                Context.new(),
                AIModel(name="deepseek-chat", provider="deepseek", provider_alias="work"),
                [conversation],
                lambda _stream: None,
                ChatOptions(thinking_mode="non_thinking", loop_policy=ChatLoopPolicy(max_iterations=2)),
            )
        )

        self.assertEqual(sent["method"], "AIChatStream")
        params = sent["params"]
        self.assertEqual(params["model"]["Name"], "deepseek-chat")
        self.assertEqual(params["model"]["Provider"], "deepseek")
        self.assertEqual(params["model"]["ProviderAlias"], "work")
        self.assertEqual(params["conversations"][0]["Role"], "user")
        self.assertEqual(params["conversations"][0]["Text"], "Hello")
        self.assertNotIn("Images", params["conversations"][0])
        self.assertEqual(params["options"]["ThinkingMode"], "non_thinking")
        self.assertEqual(params["options"]["LoopPolicy"]["MaxIterations"], 2)
        self.assertTrue(params["callbackId"])
        json.dumps(params)

    def test_omitted_options_are_empty_and_model_is_required(self):
        api = PluginAPI(AsyncMock(), "notes", "Notes")
        sent = {}

        async def invoke(_ctx, method, params):
            sent["method"] = method
            sent["options"] = params["options"]

        api.invoke_method = invoke
        drive(
            api.ai_chat_stream(
                Context.new(),
                AIModel(name="gpt-4", provider="openai"),
                [Conversation.new_user_message("Hello")],
                lambda _stream: None,
            )
        )
        self.assertEqual(sent["method"], "AIChatStream")
        self.assertEqual(sent["options"], {})
        with self.assertRaises(ValueError):
            drive(api.ai_chat_stream(Context.new(), AIModel(name="", provider="openai"), [], lambda _stream: None))


class AIChatStreamLifecycleTest(unittest.IsolatedAsyncioTestCase):
    async def test_terminal_callbacks_are_released_even_when_they_raise(self):
        """Intermediate events retain callbacks; terminal events release only their own callback."""
        for terminal_status in ("finished", "error"):
            for raises in (False, True):
                with self.subTest(terminal_status=terminal_status, raises=raises):
                    api = PluginAPI(AsyncMock(), "chat", "Chat")
                    plugin_instances["chat"] = PluginInstance(
                        plugin=object(),
                        api=api,
                        plugin_dir="",
                        module_name="chat",
                        toolbar_msg_actions={},
                        sys_paths=[],
                    )
                    try:
                        statuses = []
                        failure = RuntimeError("plugin callback failed")

                        def callback(data):
                            """Record each event and optionally fail on the terminal event."""
                            statuses.append(data.status.value)
                            if raises and data.status.value == terminal_status:
                                raise failure

                        api.invoke_method = AsyncMock()
                        await api.ai_chat_stream(Context.new(), AIModel(name="test", provider="openai"), [], callback)
                        callback_id = api.invoke_method.call_args.args[2]["callbackId"]

                        def other_callback(_data):
                            pass

                        api.llm_stream_callbacks["other"] = other_callback
                        for status in ("streaming", "streamed", "running_tool_call", terminal_status):
                            invocation = jsonrpc.on_llm_stream(
                                Context.new(),
                                {"PluginId": "chat", "Params": {"CallbackId": callback_id, "StreamType": status, "Data": "response"}},
                            )
                            if raises and status == terminal_status:
                                with self.assertRaises(RuntimeError) as caught:
                                    await invocation
                                self.assertIs(caught.exception, failure)
                            else:
                                await invocation
                            self.assertEqual(callback_id in api.llm_stream_callbacks, status != terminal_status)
                            self.assertIs(api.llm_stream_callbacks["other"], other_callback)
                        self.assertEqual(statuses, ["streaming", "streamed", "running_tool_call", terminal_status])
                    finally:
                        plugin_instances.pop("chat", None)

    async def test_failed_or_cancelled_requests_release_callbacks(self):
        """Request failures preserve the original exception without retaining the callback."""
        for failure in (RuntimeError("host websocket disconnected"), asyncio.CancelledError()):
            with self.subTest(failure=type(failure).__name__):
                api = PluginAPI(AsyncMock(), "chat", "Chat")
                api.invoke_method = AsyncMock(side_effect=failure)
                with self.assertRaises(type(failure)) as caught:
                    await api.ai_chat_stream(Context.new(), AIModel(name="test", provider="openai"), [], lambda _data: None)
                self.assertIs(caught.exception, failure)
                self.assertFalse(api.llm_stream_callbacks)

    async def test_serialization_failure_releases_callback(self):
        """Payload construction can fail before the request reaches core."""
        api = PluginAPI(AsyncMock(), "chat", "Chat")
        api.invoke_method = AsyncMock()
        conversation = Conversation.new_user_message("Hello")
        conversation.timestamp = object()
        with self.assertRaises(TypeError):
            await api.ai_chat_stream(Context.new(), AIModel(name="test", provider="openai"), [conversation], lambda _data: None)
        api.invoke_method.assert_not_awaited()
        self.assertFalse(api.llm_stream_callbacks)
