"""Verify window operations and callbacks stay on the host RPC path."""

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

from wox_plugin import (  # noqa: E402
    Context,
    CreateWindowOption,
    WindowCloseOption,
    WindowContent,
    WindowContentType,
    WindowContentSetOption,
    WindowContentUpdateOption,
    WindowContentUpdateResult,
    WindowContentCommand,
    WindowContentCommandOption,
    WindowContentCommandResult,
    WindowEventType,
    WindowThemeChangedData,
    WindowHideOption,
    WindowOptions,
    WindowPresentation,
    WindowSetOptionsOption,
    WindowShowOption,
    Button,
    create_wox_view,
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
        tree = '{"version":1,"root":{"type":"text","props":{"text":"hello"}}}'
        created = await api.create_window(
            ctx,
            CreateWindowOption(
                content=WindowContent(WindowContentType.WOXVIEW, tree),
                window=WindowOptions(presentation=WindowPresentation.WINDOW, topmost=False, offset_x=0, movable=True, hide_chrome=True, drag_content=True, transparent=True, pass_through_transparent=True),
                on_event=lambda ctx, e: fired.append(e),
            ),
        )
        self.assertEqual(fired[0].type, WindowEventType.READY)
        self.assertEqual(created.window.id, "native-1")
        self.assertEqual(json.loads(json.dumps(calls[0][1]))["option"]["Window"]["Presentation"], "window")
        self.assertIs(calls[0][1]["option"]["Window"]["Topmost"], False)
        self.assertEqual(calls[0][1]["option"]["Content"], {"Type": "woxview", "Data": tree})
        self.assertIs(calls[0][1]["option"]["Window"]["HideChrome"], True)
        self.assertIs(calls[0][1]["option"]["Window"]["DragContent"], True)
        self.assertIs(calls[0][1]["option"]["Window"]["Transparent"], True)
        self.assertIs(calls[0][1]["option"]["Window"]["PassThroughTransparent"], True)
        self.assertNotIn("OnEvent", calls[0][1]["option"])
        await created.window.show(ctx, WindowShowOption())
        await created.window.hide(ctx, WindowHideOption())
        await created.window.set_options(ctx, WindowSetOptionsOption(offset_y=0, topmost=False))
        await created.window.content.set(ctx, WindowContentSetOption(WindowContent(WindowContentType.WOXVIEW, tree)))
        self.assertEqual(calls[-1][1]["option"]["Content"], {"Type": "woxview", "Data": tree})
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
        option = CreateWindowOption(WindowContent(WindowContentType.WOXVIEW, '{"version":1,"root":{"type":"text","props":{"text":""}}}'))
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

    async def test_window_binding_attaches_and_releases_once(self):
        api = PluginAPI(AsyncMock(), "window", "Window")
        events = []

        class Binding:
            def bind(self, _ctx, window):
                events.append(f"bound:{window.id}")

            def release(self):
                events.append("released")

        option = CreateWindowOption(WindowContent(WindowContentType.WOXVIEW, "{}"))
        option._wox_window_binding = Binding()
        api.invoke_method = AsyncMock(return_value={"Success": True, "WindowID": "native-3"})
        created = await api.create_window(Context.new(), option)
        self.assertTrue(created.success)
        payload = api.invoke_method.await_args.args[2]["option"]
        self.assertNotIn("_wox_window_binding", payload)
        self.assertEqual(events, ["bound:native-3"])
        api.release_windows()
        api.release_windows()
        self.assertEqual(events, ["bound:native-3", "released"])
        failed = CreateWindowOption(WindowContent(WindowContentType.WOXVIEW, "{}"))
        failed._wox_window_binding = Binding()
        api.invoke_method = AsyncMock(return_value={"Success": False, "ErrMsg": "unsupported"})
        self.assertFalse((await api.create_window(Context.new(), failed)).success)
        self.assertEqual(events, ["bound:native-3", "released", "released"])

    async def test_woxview_uses_window_rpc_and_dispatches_events(self):
        api = PluginAPI(AsyncMock(), "view", "View")
        plugin_instances["view"] = PluginInstance(
            plugin=object(), api=api, plugin_dir="", module_name="view", toolbar_msg_actions={}, sys_paths=[]
        )
        self.addCleanup(lambda: plugin_instances.pop("view", None))
        state = {"count": 0}
        calls, errors = [], []
        callback_id = ""

        async def event(payload):
            await jsonrpc.handle_request_from_wox(
                Context.new(), {"PluginId": "view", "Method": "windowEvent", "Params": {"CallbackID": callback_id, **payload}}, AsyncMock()
            )

        async def invoke(_ctx, method, params):
            nonlocal callback_id
            calls.append((method, params))
            if method == "CreateWindow":
                callback_id = params["option"]["CallbackID"]
                await event({"Event": json.dumps({"Type": "ready", "WindowID": "win", "DocumentID": "doc"})})
                return {"Success": True, "WindowID": "win"}
            return {"Success": True}

        def render(session):
            def add():
                state["count"] += 1
                session.invalidate()

            return Button(label=str(state["count"]), on_press=add)

        api.invoke_method = invoke
        option = create_wox_view(render, errors.append).as_window()
        handler = json.loads(option.content.data)["root"]["props"]["onPress"]["$handler"]
        await api.create_window(Context.new(), option)
        await event(
            {
                "Event": json.dumps(
                    {"Type": "woxview_event", "WindowID": "win", "DocumentID": "doc", "Data": {"Handler": handler, "Args": [], "Key": ""}}
                )
            }
        )
        await asyncio.sleep(0.01)
        self.assertEqual(state["count"], 1)
        update = next(params["option"] for method, params in calls if method == "WindowContentUpdate")
        self.assertEqual(json.loads(update["Data"])["root"]["props"]["label"], "1")
        self.assertEqual(update["DocumentID"], "doc")
        self.assertEqual(errors, [])
        api.release_windows()
