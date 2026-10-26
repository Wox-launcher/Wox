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

from wox_plugin import Context, Query, Result, ResultAction, ResultActionType, UpdatableResult, WoxImage  # noqa: E402
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
