"""Verify window operations and callbacks stay on the host RPC path."""

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

from wox_plugin import (  # noqa: E402
    Context,
    CreateWindowOption,
    WindowCloseOption,
    WindowContent,
    WindowContentSetOption,
    WindowContentUpdateOption,
    WindowContentUpdateResult,
    WindowContentCommand,
    WindowContentCommandOption,
    WindowContentCommandResult,
    WindowContentType,
    WindowEventType,
    WindowThemeChangedData,
    WindowHideOption,
    WindowMessageOption,
    WindowOptions,
    WindowPresentation,
    WindowSetOptionsOption,
    WindowShowOption,
)

from wox_plugin_host import jsonrpc  # noqa: E402
from wox_plugin_host.plugin_api import PluginAPI  # noqa: E402
from wox_plugin_host.plugin_manager import PluginInstance, plugin_instances  # noqa: E402


class WindowHostTest(unittest.IsolatedAsyncioTestCase):
    async def test_window_rpc_and_early_events(self):
        api = PluginAPI(AsyncMock(), "window", "Window")
        plugin_instances["window"] = PluginInstance(
            plugin=object(), api=api, plugin_dir="", module_name="window", toolbar_msg_actions={}, sys_paths=[]
        )
        self.addCleanup(lambda: plugin_instances.pop("window", None))
        calls, fired = [], []
        callback_id = ""

        async def event(payload):
            request = {"PluginId": "window", "Method": "windowEvent", "Params": {"CallbackID": callback_id, **payload}}
            await jsonrpc.handle_request_from_wox(Context.new(), request, AsyncMock())

        async def invoke(ctx, method, params):
            nonlocal callback_id
            calls.append((method, params))
            if method == "CreateWindow":
                callback_id = params["option"]["CallbackID"]
                await event({"Event": json.dumps({"Type": "ready", "WindowID": "native-1", "DocumentID": "doc-1"})})
                return {"Success": True, "WindowID": "native-1"}
            return {"Success": True}

        api.invoke_method = invoke
        ctx = Context.new()
        created = await api.create_window(
            ctx,
            CreateWindowOption(
                content=WindowContent(WindowContentType.HTML, "<p>timer</p>"),
                window=WindowOptions(
                    presentation=WindowPresentation.WINDOW, topmost=False, offset_x=0, movable=True, drag_content=True, hide_chrome=True
                ),
                on_event=lambda ctx, e: fired.append(e),
            ),
        )
        self.assertEqual(fired[0].type, WindowEventType.READY)
        self.assertEqual(created.window.id, "native-1")
        self.assertEqual(json.loads(json.dumps(calls[0][1]))["option"]["Window"]["Presentation"], "window")
        self.assertIs(calls[0][1]["option"]["Window"]["Topmost"], False)
        self.assertIs(calls[0][1]["option"]["Window"]["DragContent"], True)
        self.assertIs(calls[0][1]["option"]["Window"]["HideChrome"], True)
        self.assertNotIn("OnEvent", calls[0][1]["option"])
        await created.window.show(ctx, WindowShowOption())
        await created.window.hide(ctx, WindowHideOption())
        await created.window.set_options(ctx, WindowSetOptionsOption(offset_y=0, topmost=False))
        await created.window.content.set(ctx, WindowContentSetOption(WindowContent(WindowContentType.TEXT, "done")))
        tree = '{"version":1,"root":{"type":"text","props":{"text":"hello"}}}'
        await created.window.content.set(ctx, WindowContentSetOption(WindowContent(WindowContentType.WOXVIEW, tree)))
        self.assertEqual(calls[-1][1]["option"]["Content"]["Type"], "woxview")
        updated = await created.window.content.update(ctx, WindowContentUpdateOption(tree, "doc-1", 7))
        self.assertIsInstance(updated, WindowContentUpdateResult)
        self.assertTrue(updated.success)
        self.assertEqual(
            calls[-1], ("WindowContentUpdate", {"windowID": "native-1", "option": {"Data": tree, "DocumentID": "doc-1", "Sequence": 7}})
        )
        for command in WindowContentCommand:
            result = await created.window.content.command(ctx, WindowContentCommandOption(command, "draft/世界", "doc-1"))
            self.assertIsInstance(result, WindowContentCommandResult)
            self.assertTrue(result.success)
            wire = json.loads(json.dumps(calls[-1]))
            self.assertEqual(
                wire,
                [
                    "WindowContentCommand",
                    {"windowID": "native-1", "option": {"DocumentID": "doc-1", "Command": command.value, "Key": "draft/世界"}},
                ],
            )
        payload = {"Handler": "save", "Args": [{"name": "hello"}]}
        await event({"Event": json.dumps({"Type": "woxview_event", "WindowID": "native-1", "DocumentID": "doc-1", "Data": payload})})
        self.assertEqual(fired[-1].type, WindowEventType.WOXVIEW)
        self.assertEqual(fired[-1].data, payload)
        for dark in (False, True):
            theme: WindowThemeChangedData = {"Dark": dark}
            await event({"Event": json.dumps({"Type": "theme_changed", "WindowID": "native-1", "DocumentID": "doc-1", "Data": theme})})
            self.assertEqual(fired[-1].type, WindowEventType.THEME_CHANGED)
            self.assertEqual(fired[-1].document_id, "doc-1")
            self.assertEqual(fired[-1].data, theme)
        data = {"snake_case": False, "__proto__": {"x": 1}, "Unicode": "你好"}
        await created.window.content.post_message(ctx, WindowMessageOption(data))
        self.assertEqual(calls[-1][1], {"windowID": "native-1", "option": {"Data": data}})
        await created.window.content.post_message(ctx, WindowMessageOption(None))
        self.assertEqual(calls[-1][1]["option"], {"Data": None})
        await event({"Event": json.dumps({"Type": "message", "WindowID": "native-1", "Data": data})})
        self.assertEqual(fired[-1].data, data)
        await event({"Release": "true"})
        self.assertFalse(api.window_callbacks)
        delivered = len(fired)
        await event({"Event": json.dumps({"Type": "theme_changed", "WindowID": "native-1", "Data": {"Dark": False}})})
        self.assertEqual(len(fired), delivered)
        count = len(calls)
        self.assertFalse((await created.window.show(ctx, WindowShowOption())).success)
        self.assertFalse((await created.window.content.update(ctx, WindowContentUpdateOption(tree))).success)
        self.assertFalse(
            (await created.window.content.command(ctx, WindowContentCommandOption(WindowContentCommand.FOCUS, "draft"))).success
        )
        self.assertTrue((await created.window.close(ctx, WindowCloseOption())).success)
        self.assertEqual(len(calls), count)

    async def test_failure_and_unload_cleanup(self):
        api = PluginAPI(AsyncMock(), "window", "Window")
        option = CreateWindowOption(WindowContent(WindowContentType.TEXT, ""))
        api.invoke_method = AsyncMock(side_effect=RuntimeError("disconnected"))
        with self.assertRaisesRegex(RuntimeError, "disconnected"):
            await api.create_window(Context.new(), option)
        self.assertFalse(api.window_callbacks)
        api.invoke_method = AsyncMock(return_value={"Success": False, "ErrMsg": "unsupported"})
        self.assertFalse((await api.create_window(Context.new(), option)).success)
        self.assertFalse(api.window_callbacks)
        api.invoke_method = AsyncMock(return_value={"Success": True, "WindowID": "native-2"})
        created = await api.create_window(Context.new(), option)
        api.release_windows()
        self.assertFalse(api.window_callbacks)
        self.assertFalse((await created.window.hide(Context.new(), WindowHideOption())).success)
