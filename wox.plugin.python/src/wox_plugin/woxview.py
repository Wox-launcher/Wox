"""WoxView definitions, document sessions and the window adapter."""

from __future__ import annotations

import asyncio
import inspect
import json
from collections.abc import Callable
from dataclasses import replace
from typing import Any
from uuid import uuid4

from .models.context import Context
from .models.preview import WoxPreview, WoxPreviewType
from .models.result import UpdatableResult
from .models.window import (
    CreateWindowOption,
    PluginWindow,
    WindowContent,
    WindowContentType,
    WindowContentUpdateOption,
    WindowEvent,
    WindowEventType,
    WindowOptions,
)

Node = dict[str, Any]
Callback = Callable[..., Any]
Render = Callable[["WoxViewSession"], Node]


class WoxViewSession:
    """Own callback identities and render commits independently of a surface."""

    def __init__(self, render: Render, on_error: Callable[[Exception], None]):
        self._render: Render | None = render
        self._on_error: Callable[[Exception], None] | None = on_error
        self._paths: dict[str, str] = {}
        self._handlers: dict[str, Callback] = {}
        self._next_id = 0
        self._disposed = False
        self._dirty = False
        self._publish: Callable[[str], Any] | None = None
        self._task: asyncio.Task | None = None
        self.data, self._handlers, self._paths = self._snapshot()

    def invalidate(self) -> None:
        """Schedule a new rendering, coalescing invalidations during a commit."""
        if not self._disposed:
            self._dirty = True
            self._schedule()

    def connect(self, publish: Callable[[str], Any]) -> None:
        """Connect a surface after it has a live document capability."""
        if not self._disposed:
            self._publish = publish
            self._schedule()

    async def dispatch(self, event: dict[str, Any]) -> None:
        """Dispatch using the handlers for the last accepted document."""
        if self._task is not None:
            await asyncio.shield(self._task)
        if self._disposed:
            return
        handler = event.get("Handler")
        if not isinstance(handler, str):
            return
        callback = self._handlers.get(handler)
        args = event.get("Args")
        if callback is not None and isinstance(args, list):
            result = callback(*args)
            if inspect.isawaitable(result):
                await result

    def dispose(self) -> None:
        """Discard callbacks and prevent later events or invalidations."""
        self._disposed = True
        self._dirty = False
        self._publish = None
        self._render = None
        self._on_error = None
        self._handlers.clear()
        self._paths.clear()

    def _snapshot(self) -> tuple[str, dict[str, Callback], dict[str, str]]:
        paths: dict[str, str] = {}
        handlers: dict[str, Callback] = {}

        def convert(value: Any, path: str) -> Any:
            if callable(value):
                handler = self._paths.get(path)
                if handler is None:
                    self._next_id += 1
                    handler = f"h{self._next_id}"
                paths[path] = handler
                handlers[handler] = value
                return {"$handler": handler}
            if isinstance(value, list):
                counts: dict[str, int] = {}
                result = []
                for index, item in enumerate(value):
                    identity: Any
                    if isinstance(item, dict) and isinstance(item.get("key"), str):
                        identity = [item.get("type", ""), item["key"]]
                    elif path.endswith('/"contextMenu"') and isinstance(item, dict) and isinstance(item.get("label"), str):
                        identity = ["label", item["label"]]
                    else:
                        identity = index
                    token = json.dumps(identity, ensure_ascii=False, separators=(",", ":"))
                    occurrence = counts.get(token, 0)
                    counts[token] = occurrence + 1
                    result.append(convert(item, f"{path}/{token}:{occurrence}"))
                return result
            if isinstance(value, dict):
                if isinstance(value.get("type"), str):
                    path += f"/type:{json.dumps(value['type'], ensure_ascii=False)}"
                return {key: convert(item, f"{path}/{json.dumps(key, ensure_ascii=False)}") for key, item in value.items()}
            return value

        if self._render is None:
            raise RuntimeError("WoxView session is disposed")
        data = json.dumps({"version": 1, "root": convert(self._render(self), "root")}, ensure_ascii=False, separators=(",", ":"))
        return data, handlers, paths

    def _schedule(self) -> None:
        if self._dirty and self._publish is not None and self._task is None and not self._disposed:
            self._task = asyncio.create_task(self._flush())

    async def _flush(self) -> None:
        try:
            while self._dirty and self._publish is not None and not self._disposed:
                self._dirty = False
                data, handlers, paths = self._snapshot()
                if await self._publish(data) and not self._disposed:
                    self.data, self._handlers, self._paths = data, handlers, paths
        except Exception as error:  # noqa: BLE001 - render and transport errors belong to the view's error handler.
            if self._on_error is not None:
                self._on_error(error)
        finally:
            self._task = None
            self._schedule()


class _WindowBinding:
    """Adapt a WoxView session to a window without exposing window policy to it."""

    def __init__(self, session: WoxViewSession, on_error: Callable[[Exception], None]):
        self.session = session
        self.on_error: Callable[[Exception], None] | None = on_error
        self.window: PluginWindow | None = None
        self.ctx: Context | None = None
        self.document = ""
        self.sequence = 0
        self.released = False
        self.overflowed = False
        self.events: list[tuple[Context, WindowEvent]] = []
        self.drain: asyncio.Task | None = None

    def bind(self, ctx: Context, window: PluginWindow) -> None:
        if not self.released:
            self.ctx, self.window = ctx, window
            if self.overflowed:
                self.events.append((ctx, WindowEvent(WindowEventType.ERROR, window.id, err_msg="WoxView early event queue overflow")))
            self._schedule()

    def release(self) -> None:
        self.released = True
        self.events.clear()
        self.window = None
        self.ctx = None
        self.on_error = None
        self.session.dispose()

    async def event(self, ctx: Context, event: WindowEvent) -> None:
        if self.released:
            return
        if self.window is None and len(self.events) >= 128:
            self.overflowed = True
            return
        self.events.append((ctx, event))
        if self.window is not None:
            self._schedule()
            if self.drain is not None:
                await asyncio.shield(self.drain)

    def _schedule(self) -> None:
        if self.events and self.window is not None and self.drain is None and not self.released:
            self.drain = asyncio.create_task(self._drain())

    async def _drain(self) -> None:
        try:
            while self.events and not self.released:
                ctx, event = self.events.pop(0)
                if event.type == WindowEventType.CLOSED:
                    self.release()
                elif event.type == WindowEventType.ERROR:
                    if self.on_error is not None:
                        self.on_error(RuntimeError(event.err_msg or "WoxView window error"))
                elif event.type == WindowEventType.READY:
                    if self.document and self.document != event.document_id:
                        self.release()
                        continue
                    self.document = event.document_id
                    if self.document:
                        self.session.connect(self._publish)
                elif event.type == WindowEventType.WOXVIEW and event.document_id == self.document and isinstance(event.data, dict):
                    self.ctx = ctx
                    try:
                        await self.session.dispatch(event.data)
                    except Exception as error:  # noqa: BLE001 - plugin callbacks may raise arbitrary exceptions.
                        if self.on_error is not None:
                            self.on_error(error)
        finally:
            self.drain = None
            self._schedule()

    async def _publish(self, data: str) -> bool:
        if self.window is None or self.ctx is None:
            return False
        self.sequence += 1
        result = await self.window.content.update(
            self.ctx, WindowContentUpdateOption(data=data, document_id=self.document, sequence=self.sequence)
        )
        if not result.success:
            raise RuntimeError(result.err_msg or "WoxView update failed")
        return True


class WoxView:
    """A reusable view definition that creates an independent session per mount."""

    def __init__(self, render: Render, on_error: Callable[[Exception], None]):
        self._render = render
        self._on_error = on_error

    def as_preview(
        self,
        *,
        on_show: Callable[[asyncio.Event], Any] | None = None,
        on_hide: Callable[[], Any] | None = None,
        on_dispose: Callable[[], Any] | None = None,
    ) -> WoxPreview:
        """Create a result-scoped session, bound and released by the plugin host."""
        session = WoxViewSession(self._render, self._on_error)
        binding = _PreviewBinding(session, self._on_error, on_show, on_hide, on_dispose)
        preview = WoxPreview(
            preview_type=WoxPreviewType.WOXVIEW, preview_data=session.data, wox_view_id=str(uuid4()), on_wox_view_event=binding.event
        )
        binding.preview = preview
        preview._wox_preview_binding = binding  # type: ignore[attr-defined]
        return preview

    def as_window(self, window: WindowOptions | None = None) -> CreateWindowOption:
        """Create options for the existing create_window API."""
        session = WoxViewSession(self._render, self._on_error)
        binding = _WindowBinding(session, self._on_error)
        option = CreateWindowOption(
            content=WindowContent(WindowContentType.WOXVIEW, session.data),
            window=window or WindowOptions(),
            on_event=binding.event,
        )
        option._wox_window_binding = binding  # type: ignore[attr-defined]
        return option


class _PreviewBinding:
    """Connect one preview session to its owning query result."""

    def __init__(
        self,
        session: WoxViewSession,
        on_error: Callable[[Exception], None],
        on_show: Callable[[asyncio.Event], Any] | None,
        on_hide: Callable[[], Any] | None,
        on_dispose: Callable[[], Any] | None,
    ):
        self.session = session
        self.on_error: Callable[[Exception], None] | None = on_error
        self.preview: WoxPreview | None = None
        self.bound = False
        self.released = False
        self.visible = False
        self.hidden: asyncio.Event | None = None
        self.on_show = on_show
        self.on_hide = on_hide
        self.on_dispose = on_dispose

    def _invoke(self, callback: Callable[..., Any] | None, *args: Any) -> None:
        if callback is None:
            return
        report = self.on_error
        try:
            result = callback(*args)
            if inspect.isawaitable(result):

                async def finish() -> None:
                    try:
                        await result
                    except Exception as error:  # noqa: BLE001 - plugin lifecycle callbacks may fail.
                        if report is not None:
                            report(error)

                asyncio.create_task(finish())
        except Exception as error:  # noqa: BLE001 - plugin lifecycle callbacks may fail.
            if report is not None:
                report(error)

    def _hide(self) -> None:
        if not self.visible:
            return
        self.visible = False
        if self.hidden is not None:
            self.hidden.set()
            self.hidden = None
        self._invoke(self.on_hide)

    def bind(self, ctx: Context, api: Any, result_id: str) -> None:
        if self.bound or self.released:
            return
        self.bound = True

        async def publish(data: str) -> bool:
            if self.preview is None:
                return False
            preview = replace(self.preview, preview_data=data)
            if not await api.update_result(ctx, UpdatableResult(id=result_id, preview=preview)):
                raise RuntimeError("WoxView preview update failed")
            return True

        self.session.connect(publish)

    async def event(self, _ctx: Context, event: dict[str, Any]) -> None:
        if not self.released:
            if event.get("Type") == "show":
                if not self.visible:
                    self.visible = True
                    self.hidden = asyncio.Event()
                    self._invoke(self.on_show, self.hidden)
                return
            if event.get("Type") == "hide":
                self._hide()
                return
            try:
                await self.session.dispatch(event)
            except Exception as error:  # noqa: BLE001 - plugin callbacks may raise arbitrary exceptions.
                if self.on_error is not None:
                    self.on_error(error)

    def release(self) -> None:
        if self.released:
            return
        self.released = True
        self._hide()
        self.preview = None
        self.session.dispose()
        self._invoke(self.on_dispose)
        self.on_show = self.on_hide = self.on_dispose = None
        self.on_error = None


def create_wox_view(render: Render, on_error: Callable[[Exception], None]) -> WoxView:
    """Create a view definition without choosing its eventual surface."""
    return WoxView(render, on_error)


def wox_view_node(type: str, props: dict[str, Any] | None = None, children: list[Node] | None = None, key: str | None = None) -> Node:
    """Build a node whose callbacks are bound when its view is mounted."""
    result: Node = {"type": type, "props": props or {}}
    if children is not None:
        result["children"] = children
    if key is not None:
        result["key"] = key
    return result


def Column(*, children: list[Node] | None = None, key: str | None = None, **props: Any) -> Node:
    return wox_view_node("column", props, children, key)


def Text(*, text: str = "", key: str | None = None, **props: Any) -> Node:
    return wox_view_node("text", {"text": text, **props}, key=key)


def Loading(*, key: str | None = None, **props: Any) -> Node:
    return wox_view_node("loading", props, key=key)


def Button(*, label: str = "", on_press: Callback | None = None, key: str | None = None, **props: Any) -> Node:
    return wox_view_node("button", {"label": label, **props, **({"onPress": on_press} if on_press is not None else {})}, key=key)
