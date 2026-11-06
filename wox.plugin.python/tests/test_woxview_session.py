"""Exercise WoxView sessions without starting a native window."""

import asyncio
import json
import unittest
from unittest.mock import AsyncMock

from wox_plugin import Button, Column, Context, Text, create_wox_view, wox_view_node
from wox_plugin.models.window import WindowContentUpdateResult, WindowEvent, WindowEventType


class WoxViewSessionTest(unittest.IsolatedAsyncioTestCase):
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
