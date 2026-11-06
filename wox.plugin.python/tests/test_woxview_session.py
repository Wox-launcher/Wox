"""Exercise WoxView sessions without starting a native window."""

import asyncio
import json
import unittest
from unittest.mock import AsyncMock

from wox_plugin import Button, Column, Context, Loading, Text, create_wox_view, wox_view_node
from wox_plugin.models.window import WindowContentUpdateResult, WindowEvent, WindowEventType
from wox_plugin.woxview import WoxViewSession


class WoxViewSessionTest(unittest.IsolatedAsyncioTestCase):
    async def test_replacing_root_key_revokes_own_and_descendant_callbacks(self):
        for nested in (False, True):
            key = "old"
            calls = []

            def render(_session):
                current = key
                button = Button(key="child" if nested else key, label=key, on_press=lambda: calls.append(current))
                return Column(key=key, children=[button]) if nested else button

            session = WoxViewSession(render, lambda error: self.fail(str(error)))

            def handler():
                root = json.loads(session.data)["root"]
                return (root["children"][0] if nested else root)["props"]["onPress"]["$handler"]

            old = handler()
            session.connect(AsyncMock(return_value=True))
            session.invalidate()
            await session.dispatch({"Handler": old, "Args": [], "Key": "old"})
            self.assertEqual(handler(), old)
            key = "new"
            session.invalidate()
            await session.dispatch({"Handler": old, "Args": [], "Key": "old"})
            self.assertNotEqual(handler(), old)
            self.assertEqual(calls, ["old"])
            await session.dispatch({"Handler": handler(), "Args": [], "Key": "new"})
            self.assertEqual(calls, ["old", "new"])
            session.dispose()

    def test_loading_builder(self):
        self.assertEqual(Loading(), {"type": "loading", "props": {}})
        self.assertEqual(
            Loading(key="pending", style={"width": 24, "color": "$accent"}),
            {"type": "loading", "key": "pending", "props": {"style": {"width": 24, "color": "$accent"}}},
        )

    async def test_hide_does_not_wait_for_async_show(self):
        gate = asyncio.Event()
        seen = []

        async def show(hidden):
            await gate.wait()
            seen.append(hidden.is_set())

        preview = create_wox_view(lambda _session: Text(text="preview"), lambda error: self.fail(str(error))).as_preview(on_show=show)
        ctx = Context.new()
        await preview.on_wox_view_event(ctx, {"Type": "show"})
        await preview.on_wox_view_event(ctx, {"Type": "hide"})
        gate.set()
        await asyncio.sleep(0.01)
        self.assertEqual(seen, [True])
        preview._wox_preview_binding.release()

    async def test_preview_visibility_stops_work_and_disposes_once(self):
        transitions, signals, errors = [], [], []
        timer = None

        def show(hidden):
            nonlocal timer
            transitions.append("show")
            signals.append(hidden)
            timer = asyncio.get_running_loop().call_later(60, lambda: None)

        def hide():
            transitions.append("hide")
            timer.cancel()

        preview = create_wox_view(lambda _session: Text(text="preview"), errors.append).as_preview(
            on_show=show, on_hide=hide, on_dispose=lambda: transitions.append("dispose")
        )
        binding = preview._wox_preview_binding
        ctx = Context.new()
        await preview.on_wox_view_event(ctx, {"Type": "show"})
        await preview.on_wox_view_event(ctx, {"Type": "show"})
        await preview.on_wox_view_event(ctx, {"Type": "hide"})
        await preview.on_wox_view_event(ctx, {"Type": "hide"})
        await preview.on_wox_view_event(ctx, {"Type": "show"})
        binding.release()
        binding.release()
        await preview.on_wox_view_event(ctx, {"Type": "show"})
        self.assertEqual(transitions, ["show", "hide", "show", "hide", "dispose"])
        self.assertTrue(all(signal.is_set() for signal in signals))
        self.assertTrue(timer.cancelled())
        self.assertEqual(errors, [])

    async def test_preview_callback_update_and_release(self):
        count = 0
        errors = []

        def render(session):
            def increment(value):
                nonlocal count
                count += value
                session.invalidate()

            return Button(label=str(count), on_press=increment)

        preview = create_wox_view(render, errors.append).as_preview()
        binding = preview._wox_preview_binding
        api = type("API", (), {"update_result": AsyncMock(return_value=True)})()
        ctx = Context.new()
        binding.bind(ctx, api, "row")
        handler = json.loads(preview.preview_data)["root"]["props"]["onPress"]["$handler"]
        await preview.on_wox_view_event(ctx, {"Handler": handler, "Args": [2], "Key": ""})
        await asyncio.sleep(0.01)
        update = api.update_result.await_args.args[1]
        self.assertEqual(update.id, "row")
        self.assertEqual(update.preview.wox_view_id, preview.wox_view_id)
        self.assertEqual(json.loads(update.preview.preview_data)["root"]["props"]["label"], "2")
        binding.release()
        await preview.on_wox_view_event(ctx, {"Handler": handler, "Args": [2], "Key": ""})
        self.assertEqual(count, 2)
        self.assertEqual(errors, [])

    async def test_early_ready_callbacks_updates_and_release(self):
        count = 0
        errors = []
        updates = []

        def render(session):
            def increment():
                nonlocal count
                count += 1
                session.invalidate()

            return Column(children=[Text(text=str(count), key="label"), Button(label="Add", key="add", on_press=increment)])

        view = create_wox_view(render, errors.append)
        option = view.as_window()
        binding = option._wox_window_binding
        initial = json.loads(option.content.data)
        handler = initial["root"]["children"][1]["props"]["onPress"]["$handler"]
        ctx = Context.new()

        async def update(_, payload):
            updates.append(payload)
            return WindowContentUpdateResult(success=True)

        window = type("Window", (), {"content": type("Content", (), {"update": staticmethod(update)})()})()
        await option.on_event(ctx, WindowEvent(WindowEventType.READY, "win", "doc"))
        binding.bind(ctx, window)
        await asyncio.sleep(0.01)
        await option.on_event(ctx, WindowEvent(WindowEventType.WOXVIEW, "win", "doc", {"Handler": handler, "Args": [], "Key": "add"}))
        await asyncio.sleep(0.01)
        self.assertEqual(count, 1)
        self.assertEqual(len(updates), 1)
        self.assertEqual((updates[0].document_id, updates[0].sequence), ("doc", 1))
        tree = json.loads(updates[0].data)
        self.assertEqual(tree["root"]["children"][0]["props"]["text"], "1")
        self.assertEqual(tree["root"]["children"][1]["props"]["onPress"]["$handler"], handler)
        binding.release()
        await option.on_event(ctx, WindowEvent(WindowEventType.WOXVIEW, "win", "doc", {"Handler": handler, "Args": [], "Key": "add"}))
        self.assertEqual(count, 1)
        self.assertEqual(errors, [])

    async def test_rejected_update_keeps_last_accepted_callbacks(self):
        state = {"label": "old", "session": None}
        calls = []
        errors = []

        def render(session):
            state["session"] = session
            label = state["label"]
            return Button(on_press=lambda: calls.append(label))

        option = create_wox_view(render, errors.append).as_window()
        binding = option._wox_window_binding
        handler = json.loads(option.content.data)["root"]["props"]["onPress"]["$handler"]
        window = type(
            "Window",
            (),
            {"content": type("Content", (), {"update": AsyncMock(return_value=WindowContentUpdateResult(err_msg="rejected"))})()},
        )()
        ctx = Context.new()
        binding.bind(ctx, window)
        await option.on_event(ctx, WindowEvent(WindowEventType.READY, "win", "doc"))
        state["label"] = "new"
        state["session"].invalidate()
        await asyncio.sleep(0.01)
        await option.on_event(ctx, WindowEvent(WindowEventType.WOXVIEW, "win", "doc", {"Handler": handler, "Args": [], "Key": ""}))
        self.assertEqual(calls, ["old"])
        self.assertEqual([str(error) for error in errors], ["rejected"])
        binding.release()

    async def test_node_type_and_menu_reordering_preserve_callback_identity(self):
        state = {"type": "button", "reverse": False}
        updates = []

        def render(session):
            state["session"] = session
            entries = [{"label": label, "onPress": lambda: None} for label in ("one", "two")]
            if state["reverse"]:
                entries.reverse()
            return wox_view_node(state["type"], {"onPress": lambda: None, "contextMenu": entries})

        option = create_wox_view(render, lambda error: self.fail(str(error))).as_window()
        binding = option._wox_window_binding
        initial = json.loads(option.content.data)["root"]["props"]

        async def update(_, payload):
            updates.append(json.loads(payload.data))
            return WindowContentUpdateResult(success=True)

        window = type("Window", (), {"content": type("Content", (), {"update": staticmethod(update)})()})()
        ctx = Context.new()
        binding.bind(ctx, window)
        await option.on_event(ctx, WindowEvent(WindowEventType.READY, "win", "doc"))
        state["reverse"] = True
        state["session"].invalidate()
        await asyncio.sleep(0.01)
        reordered = updates[0]["root"]["props"]
        self.assertEqual(reordered["contextMenu"][0]["onPress"]["$handler"], initial["contextMenu"][1]["onPress"]["$handler"])
        state["type"] = "column"
        state["session"].invalidate()
        await asyncio.sleep(0.01)
        self.assertNotEqual(updates[1]["root"]["props"]["onPress"]["$handler"], initial["onPress"]["$handler"])
        binding.release()

    async def test_events_wait_for_pending_update(self):
        state = {"label": "old"}
        calls = []
        started = asyncio.Event()
        finish = asyncio.Event()

        def render(session):
            state["session"] = session
            label = state["label"]
            return Button(on_press=lambda: calls.append(label))

        async def update(_ctx, _option):
            started.set()
            await finish.wait()
            return WindowContentUpdateResult(success=True)

        option = create_wox_view(render, lambda error: self.fail(str(error))).as_window()
        binding = option._wox_window_binding
        window = type("Window", (), {"content": type("Content", (), {"update": staticmethod(update)})()})()
        ctx = Context.new()
        handler = json.loads(option.content.data)["root"]["props"]["onPress"]["$handler"]
        binding.bind(ctx, window)
        await option.on_event(ctx, WindowEvent(WindowEventType.READY, "win", "doc"))
        state["label"] = "new"
        state["session"].invalidate()
        await started.wait()
        event = asyncio.create_task(
            option.on_event(ctx, WindowEvent(WindowEventType.WOXVIEW, "win", "doc", {"Handler": handler, "Args": []}))
        )
        await asyncio.sleep(0)
        self.assertEqual(calls, [])
        finish.set()
        await event
        self.assertEqual(calls, ["new"])
        binding.release()
