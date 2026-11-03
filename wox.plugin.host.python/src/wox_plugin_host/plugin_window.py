"""Host-side window capabilities backed by Core RPC."""

import inspect

from wox_plugin.models.window import (
    WindowCloseResult,
    WindowContentSetResult,
    WindowEvent,
    WindowEventType,
    WindowHideResult,
    WindowMessageResult,
    WindowSetOptionsResult,
    WindowShowResult,
    window_option_payload,
)


class WindowContentProxy:
    def __init__(self, window):
        self.window = window

    async def set(self, ctx, option):
        return await self.window.call(ctx, "WindowContentSet", option, WindowContentSetResult)

    async def post_message(self, ctx, option):
        return await self.window.call(ctx, "WindowPostMessage", option, WindowMessageResult)


class WindowProxy:
    def __init__(self, invoke, callback):
        self.id = ""
        self.content = WindowContentProxy(self)
        self.closed = False
        self.invoke = invoke
        self.callback = callback

    def release(self):
        self.closed = True
        self.callback = None

    async def event(self, ctx, payload):
        """Handle callbacks that may arrive before the creation reply."""
        if self.closed:
            return
        self.id = payload["WindowID"]
        event = WindowEvent(
            type=WindowEventType(payload["Type"]),
            window_id=self.id,
            document_id=payload.get("DocumentID", ""),
            data=payload.get("Data"),
            err_msg=payload.get("ErrMsg", ""),
        )
        if event.type == WindowEventType.CLOSED:
            self.closed = True
        if self.callback:
            result = self.callback(ctx, event)
            if inspect.isawaitable(result):
                await result

    async def call(self, ctx, method, option, result_type):
        """Keep closed proxies inert when plugin code retains a reference."""
        if self.closed:
            return result_type(success=method == "WindowClose", err_msg="" if method == "WindowClose" else "window closed")
        result = await self.invoke(ctx, method, {"windowID": self.id, "option": window_option_payload(option)})
        return result_type(success=result.get("Success", False), err_msg=result.get("ErrMsg", ""))

    async def show(self, ctx, option):
        return await self.call(ctx, "WindowShow", option, WindowShowResult)

    async def hide(self, ctx, option):
        return await self.call(ctx, "WindowHide", option, WindowHideResult)

    async def close(self, ctx, option):
        return await self.call(ctx, "WindowClose", option, WindowCloseResult)

    async def set_options(self, ctx, option):
        return await self.call(ctx, "WindowSetOptions", option, WindowSetOptionsResult)
