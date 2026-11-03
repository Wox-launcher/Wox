"""Verify hotkey delivery remains responsive while a start handler awaits work."""

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

from wox_plugin import Context, OnHotkeyOption, HotkeyPhase  # noqa: E402
from wox_plugin_host import jsonrpc  # noqa: E402
from wox_plugin_host.plugin_api import PluginAPI  # noqa: E402
from wox_plugin_host.plugin_manager import PluginInstance, plugin_instances  # noqa: E402


class HotkeyHostTest(unittest.IsolatedAsyncioTestCase):
    async def test_release_does_not_wait_for_start_and_unload_cancels_work(self):
        api = PluginAPI(AsyncMock(), "hotkeys", "Hotkeys")
        plugin_instances["hotkeys"] = PluginInstance(
            plugin=object(), api=api, plugin_dir="", module_name="hotkeys", toolbar_msg_actions={}, sys_paths=[]
        )
        self.addCleanup(lambda: plugin_instances.pop("hotkeys", None))
        api.invoke_method = AsyncMock(return_value={"Success": True})
        pending = asyncio.Event()
        fired = []

        async def callback(ctx, event):
            fired.append(event.phase)
            if event.phase == HotkeyPhase.STARTED:
                await pending.wait()

        self.assertTrue((await api.on_hotkey(Context.new(), OnHotkeyOption(callback))).success)
        callback_id = json.loads(api.invoke_method.call_args.args[2]["option"])["CallbackID"]

        async def send(owner, phase):
            event = {
                "SettingKey": "record",
                "Kind": "holdModifier",
                "Phase": phase,
                "Hotkey": "hold:right_alt",
                "ActivationID": "a",
                "Timestamp": 1,
            }
            await jsonrpc.handle_request_from_wox(
                Context.new(),
                {"PluginId": owner, "Method": "hotkeyEvent", "Params": {"CallbackID": callback_id, "Event": json.dumps(event)}},
                AsyncMock(),
            )
            await asyncio.sleep(0)

        await send("hotkeys", "started")
        await send("missing-owner", "ended")
        self.assertEqual(fired, [HotkeyPhase.STARTED])
        await send("hotkeys", "ended")
        self.assertEqual(fired, [HotkeyPhase.STARTED, HotkeyPhase.ENDED])
        tasks = list(api.hotkey_tasks)
        api.release_hotkeys()
        await asyncio.sleep(0)
        self.assertTrue(all(task.done() for task in tasks))
        await send("hotkeys", "triggered")
        self.assertEqual(len(fired), 2)

    async def test_failed_subscription_releases_callback(self):
        api = PluginAPI(AsyncMock(), "hotkeys", "Hotkeys")
        api.invoke_method = AsyncMock(return_value={"Success": False, "ErrMsg": "duplicate"})
        self.assertFalse((await api.on_hotkey(Context.new(), OnHotkeyOption(lambda ctx, event: None))).success)
        self.assertFalse(api.hotkey_callbacks)
