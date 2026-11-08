"""Exercise result callback ownership through the host RPC and API paths."""

import asyncio
import importlib.util
import json
import sys
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import AsyncMock, patch

package_path = Path(__file__).resolve().parents[1] / "src" / "wox_plugin_host"
spec = importlib.util.spec_from_loader("wox_plugin_host", loader=None, is_package=True)
package = importlib.util.module_from_spec(spec)
package.__path__ = [str(package_path)]
sys.modules.setdefault("wox_plugin_host", package)

from wox_plugin import Context, Query, Result, ResultAction, ResultActionType, ToolbarMsg, ToolbarMsgAction, UpdatableResult, WoxImage  # noqa: E402
from wox_plugin_host import jsonrpc  # noqa: E402
from wox_plugin_host.plugin_api import PluginAPI  # noqa: E402
from wox_plugin_host.plugin_manager import PluginInstance, plugin_instances  # noqa: E402
from wox_plugin_host.result_actions import ResultActions  # noqa: E402


def context(session="", query=""):
    ctx = Context.new()
    ctx.values.update(SessionId=session, QueryId=query)
    return ctx


def result(result_id, callback, form=False):
    action = ResultAction(id="open", name="Open")
    if form:
        action.type = ResultActionType.FORM
        action.on_submit = callback
    else:
        action.action = callback
    return Result(id=result_id, title=result_id, sub_title="", icon=WoxImage(), actions=[action])


class ResultActionsTest(unittest.IsolatedAsyncioTestCase):
    async def test_plugin_preview_survives_query_and_is_released(self):
        from wox_plugin import Button, QueryResponse, create_wox_view

        clicks, disposed = [], []
        def render(session):
            def press():
                clicks.append("click")
                session.invalidate()
            return Button(label=str(len(clicks)), on_press=press)

        preview = create_wox_view(render, lambda error: self.fail(str(error))).as_preview(on_dispose=lambda: disposed.append(True))
        self.api.invoke_method.return_value = {"Success": True}
        self.plugin.query = AsyncMock(return_value=QueryResponse(plugin_preview=preview))
        await self.call("query", "a", "first")
        result_id = "plugin-preview:owner"
        params = {"ResultId": result_id, "DocumentID": preview.wox_view_id}
        await self.call("woxviewPreviewEvent", "a", "first", **params, Event=json.dumps({"Type": "show"}))
        self.callbacks.release("a", "first")
        await self.call("query", "a", "second")
        self.callbacks.release("a", "second")
        handler = json.loads(preview.preview_data)["root"]["props"]["onPress"]["$handler"]
        event = json.dumps({"Handler": handler, "Args": [], "Key": ""})
        await self.call("woxviewPreviewEvent", "a", "first", **params, Event=event)
        await asyncio.sleep(0.05)
        self.assertEqual(clicks, ["click"])
        self.assertEqual(self.api.invoke_method.await_args.args[1], "UpdatePluginPreview")
        await self.call("releasePluginPreview", "a", "first", **params)
        await self.call("woxviewPreviewEvent", "a", "first", **params, Event=event)
        self.assertEqual(clicks, ["click"])
        self.assertEqual(disposed, [True])

    async def test_preview_returned_after_query_release_is_disposed(self):
        from wox_plugin import Text, create_wox_view

        disposed = []
        pending = asyncio.get_running_loop().create_future()

        async def delayed_query(*_args):
            return await pending

        self.plugin.query = delayed_query
        request = asyncio.create_task(self.call("query", "a", "one"))
        await asyncio.sleep(0)
        self.callbacks.release("a", "one")
        preview = create_wox_view(lambda _session: Text(text="test"), lambda error: self.fail(str(error))).as_preview(
            on_dispose=lambda: disposed.append(True)
        )
        pending.set_result([Result(id="row", title="row", sub_title="", icon=WoxImage(), preview=preview)])
        await request
        self.assertEqual(disposed, [True])

    async def test_rejected_preview_replacement_preserves_old_session(self):
        from wox_plugin import Text, create_wox_view

        disposed = []
        view = create_wox_view(lambda _session: Text(text="test"), lambda error: self.fail(str(error)))
        old = view.as_preview(on_dispose=lambda: disposed.append("old"))
        new = view.as_preview(on_dispose=lambda: disposed.append("new"))
        self.plugin.query = AsyncMock(return_value=[Result(id="row", title="row", sub_title="", icon=WoxImage(), preview=old)])
        await self.call("query", "a", "one")
        self.api.invoke_method.return_value = False
        self.assertFalse(await self.api.update_result(context("a", "one"), UpdatableResult(id="row", preview=new)))
        self.assertEqual(self.callbacks.find("a", "one", "row").previews["row"][0], old.wox_view_id)
        self.assertEqual(disposed, ["new"])
        self.callbacks.release("a", "one")
        self.assertEqual(disposed, ["new", "old"])

    async def test_preview_event_scope_and_release(self):
        from wox_plugin import Button, create_wox_view

        calls, transitions = [], []
        preview = create_wox_view(
            lambda _session: Button(on_press=lambda value: calls.append(value)), lambda error: self.fail(str(error))
        ).as_preview(
            on_show=lambda _hidden: transitions.append("show"),
            on_hide=lambda: transitions.append("hide"),
            on_dispose=lambda: transitions.append("dispose"),
        )
        self.plugin.query = AsyncMock(return_value=[Result(id="row", title="row", sub_title="", icon=WoxImage(), preview=preview)])
        await self.call("query", "a", "one")
        await self.call(
            "woxviewPreviewEvent", "a", "one", ResultId="row", DocumentID=preview.wox_view_id, Event=json.dumps({"Type": "show"})
        )
        handler = json.loads(preview.preview_data)["root"]["props"]["onPress"]["$handler"]
        event = json.dumps({"Handler": handler, "Args": [5], "Key": ""})
        await self.call("woxviewPreviewEvent", "a", "one", ResultId="row", DocumentID="old", Event=event)
        await self.call("woxviewPreviewEvent", "a", "one", ResultId="row", DocumentID=preview.wox_view_id, Event=event)
        self.callbacks.release("a", "one")
        await self.call("woxviewPreviewEvent", "a", "one", ResultId="row", DocumentID=preview.wox_view_id, Event=event)
        self.assertEqual(calls, [5])
        self.assertEqual(transitions, ["show", "hide", "dispose"])

    async def asyncSetUp(self):
        self.calls = []
        self.callbacks = ResultActions()
        self.api = PluginAPI(AsyncMock(), "owner", "Owner", self.callbacks)
        self.api.invoke_method = AsyncMock(return_value=True)
        self.plugin = SimpleNamespace(query=self.query)
        plugin_instances["owner"] = PluginInstance(
            plugin=self.plugin,
            api=self.api,
            plugin_dir="",
            module_name="unused",
            toolbar_msg_actions={},
            sys_paths=[],
            result_actions=self.callbacks,
        )
        for level in ("info", "error", "warning"):
            patcher = patch.object(jsonrpc.logger, level, new=AsyncMock())
            patcher.start()
            self.addCleanup(patcher.stop)

    async def asyncTearDown(self):
        plugin_instances.pop("owner", None)

    async def query(self, ctx, query):
        return [result(row, lambda _ctx, _action, row=row: self.calls.append((query.session_id, query.id, row))) for row in ("one", "two")]

    async def call(self, method, session, query, **params):
        if method == "query":
            params.update(Id=query, SessionId=session, Type="input", RawQuery="", TriggerKeyword="", Command="", Search="")
        return await jsonrpc.handle_request_from_wox(
            Context.new(),
            {
                "PluginId": "owner",
                "PluginName": "Owner",
                "Method": method,
                "SessionId": session,
                "QueryId": query,
                "ManagedQuery": True,
                "Params": params,
            },
            AsyncMock(),
        )

    async def test_windows_queries_and_rows_can_reuse_action_ids(self):
        await self.call("query", "a", "old")
        await self.call("query", "b", "other")
        await self.call("query", "a", "new")
        for session, query, row in (("a", "old", "one"), ("b", "other", "one"), ("a", "new", "two")):
            await self.call("action", session, query, ResultId=row, ActionId="open")
        self.assertEqual(self.calls, [("a", "old", "one"), ("b", "other", "one"), ("a", "new", "two")])
        await self.call("action", "a", "missing", ResultId="one", ActionId="open")
        await self.call("action", "", "", ResultId="one", ActionId="open")
        self.assertEqual(len(self.calls), 3)

    async def test_toolbar_callbacks_are_isolated_and_retired_by_snapshot(self):
        for session in ("one", "two"):
            msg = ToolbarMsg(
                id="status",
                title="status",
                actions=[
                    ToolbarMsgAction(
                        id="open",
                        name="Open",
                        action=lambda _ctx, action, session=session: self.calls.append((session, action.toolbar_msg_action_id)),
                    )
                ],
            )
            await self.api.show_toolbar_msg(context(session), msg)
        first, second = (call.args[2]["msg"] for call in self.api.invoke_method.await_args_list)
        self.assertEqual(first["Actions"][0]["Id"], "open")
        self.assertEqual(second["Actions"][0]["Id"], "open")
        self.assertNotEqual(first["HostSnapshotId"], second["HostSnapshotId"])
        for session, action in (("one", first), ("two", second)):
            await self.call(
                "toolbarMsgAction",
                session,
                "",
                ActionId="open",
                HostSnapshotId=action["HostSnapshotId"],
                ToolbarMsgId="status",
                ToolbarMsgActionId="open",
            )
        self.assertEqual(self.calls, [("one", "open"), ("two", "open")])
        await self.call("retireToolbarSnapshot", "one", "", HostSnapshotId=first["HostSnapshotId"])
        callbacks = plugin_instances["owner"].toolbar_msg_actions
        self.assertNotIn(first["HostSnapshotId"], callbacks)
        self.assertIn(second["HostSnapshotId"], callbacks)

    async def test_failed_toolbar_request_releases_callbacks(self):
        self.api.invoke_method.side_effect = RuntimeError("disconnected")
        with self.assertRaises(RuntimeError):
            await self.api.show_toolbar_msg(
                context("one"),
                ToolbarMsg(id="status", title="status", actions=[ToolbarMsgAction(id="run", name="Run", action=lambda *_: None)]),
            )
        self.assertFalse(plugin_instances["owner"].toolbar_msg_actions)

    async def test_toolbar_without_callback_cannot_invoke_older_snapshot(self):
        for action in (lambda *_: self.calls.append("old"), None):
            await self.api.show_toolbar_msg(
                context("one"), ToolbarMsg(id="status", title="status", actions=[ToolbarMsgAction(id="run", name="Run", action=action)])
            )
        snapshot = self.api.invoke_method.await_args_list[-1].args[2]["msg"]["HostSnapshotId"]
        await self.call("toolbarMsgAction", "one", "", ActionId="run", HostSnapshotId=snapshot)
        self.assertFalse(self.calls)

    async def test_out_of_order_and_released_late_responses(self):
        pending = {}

        async def query(ctx, query):
            event = pending.setdefault(query.id, asyncio.Event())
            await event.wait()
            return [result("row", lambda _ctx, _action: self.calls.append(query.id))]

        self.plugin.query = query
        old = asyncio.create_task(self.call("query", "a", "old"))
        current = asyncio.create_task(self.call("query", "a", "current"))
        await asyncio.sleep(0)
        pending["current"].set()
        await current
        pending["old"].set()
        await old
        for query_id in ("old", "current"):
            await self.call("action", "a", query_id, ResultId="row", ActionId="open")
        self.assertEqual(self.calls, ["old", "current"])
        late = asyncio.create_task(self.call("query", "a", "late"))
        await asyncio.sleep(0)
        await self.call("releaseQueryResults", "a", "late")
        pending["late"].set()
        await late
        self.assertIsNone(self.callbacks.find("a", "late", "row"))

    async def test_push_update_forms_mru_and_unload(self):
        await self.call("query", "a", "q")
        await self.call("query", "b", "q")
        query = Query.from_json(json.dumps({"Id": "q", "SessionId": "a", "Type": "input", "RawQuery": ""}))
        await self.api.push_results(context("b", "q"), query, [result("row", lambda _ctx, _action: self.calls.append("form"), form=True)])
        await self.call("formAction", "a", "q", ResultId="row", ActionId="open", Values="{}")
        self.api.invoke_method.return_value = json.dumps({"Actions": [{"Id": "open", "Name": "Open", "Type": "form"}]})
        restored = await self.api.get_updatable_result(context("a", "q"), "row")
        self.assertTrue(callable(restored.actions[0].on_submit))
        self.api.invoke_method.return_value = True
        await self.api.update_result(
            context("a", "q"), UpdatableResult(id="row", actions=[ResultAction(id="open", name="Rename", type=ResultActionType.FORM)])
        )
        self.assertIs(self.callbacks.find("a", "q", "row").results["row"]["open"].on_submit, restored.actions[0].on_submit)
        await self.api.update_result(
            context("a", "q"), UpdatableResult(id="row", actions=result("row", lambda _ctx, _action: self.calls.append("updated")).actions)
        )
        await self.call("formAction", "a", "q", ResultId="row", ActionId="open")
        await self.call("action", "a", "q", ResultId="row", ActionId="open")
        await self.api.update_result(context("a", "q"), UpdatableResult(id="row", actions=[]))
        await self.call("action", "a", "q", ResultId="row", ActionId="open")
        self.api.mru_restore_callbacks["mru"] = lambda _ctx, _data: result("row", lambda _ctx, _action: self.calls.append("mru"))
        restored = await self.call("onMRURestore", "a", "home", CallbackId="mru")
        self.assertEqual(restored["Id"], "row")
        await self.call("query", "a", "next")
        await self.call("action", "a", "home", ResultId="row", ActionId="open")
        self.assertEqual(self.calls, ["form", "updated", "mru"])
        await self.call("releaseQueryResults", "a", "home")
        await self.api.update_result(context("a", "home"), UpdatableResult(id="row", actions=[]))
        self.assertIsNone(self.callbacks.find("a", "home", "row"))
        self.assertIsNotNone(self.callbacks.find("b", "q", "one"))
        await self.call("unloadPlugin", "", "")
        self.assertFalse(self.callbacks.begin("a", "late", True).active)

    def test_legacy_bound_and_ambiguous_lookup(self):
        for i in range(33):
            self.callbacks.begin("a", str(i), False).cache(result(str(i), lambda *_: None))
        self.assertIsNone(self.callbacks.find("a", "0", "0"))
        self.assertIsNotNone(self.callbacks.find("", "", "1"))
        self.callbacks.begin("b", "1", False).cache(result("1", lambda *_: None))
        self.assertIsNone(self.callbacks.find("", "", "1"))

    async def test_ambiguous_updates_warn_without_replacing_callbacks(self):
        original = lambda *_: None
        replacement = lambda *_: None
        for query in ("old", "new"):
            self.callbacks.begin("a", query, True).cache(result("row", original))
        update = UpdatableResult(id="row", actions=result("row", replacement).actions)
        self.assertTrue(await self.api.update_result(context(), update))
        jsonrpc.logger.warning.assert_awaited_once()
        message = jsonrpc.logger.warning.call_args.args[1]
        self.assertRegex(message, r"Owner.*row.*ambiguous.*SessionId/QueryId")
        for query in ("old", "new"):
            self.assertIs(self.callbacks.find("a", query, "row").results["row"]["open"].action, original)
        await self.api.update_result(context("a", "new"), update)
        self.assertIs(self.callbacks.find("a", "new", "row").results["row"]["open"].action, replacement)
        jsonrpc.logger.warning.assert_awaited_once()

    async def test_remove_result_releases_only_accepted_query_callbacks(self):
        from wox_plugin import RemoveResultOption

        for query in ("old", "current"):
            self.callbacks.begin("a", query, True).cache(result("row", lambda *_: None))
        ctx = context("a", "current")
        option = RemoveResultOption(result_id="row", selected_result_id="next")
        self.api.invoke_method.return_value = {"Success": False}
        self.assertFalse((await self.api.remove_result(ctx, option)).success)
        self.assertIn("row", self.callbacks.find("a", "current", "row").results)
        payload = self.api.invoke_method.call_args.args[2]
        self.assertEqual(json.loads(payload["option"]), {"ResultId": "row", "SelectedResultId": "next"})
        self.api.invoke_method.return_value = {"Success": True}
        self.assertTrue((await self.api.remove_result(ctx, option)).success)
        self.assertNotIn("row", self.callbacks.find("a", "current", "row").results)
        self.assertIn("row", self.callbacks.find("a", "old", "row").results)
