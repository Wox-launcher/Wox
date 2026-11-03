"""Plugin-owned windows. All geometry uses logical units, never physical pixels."""

from __future__ import annotations

from dataclasses import dataclass, field, fields
from enum import Enum
from typing import Any, Awaitable, Callable, Optional, Protocol

from .context import Context


class WindowContentType(str, Enum):
    TEXT = "text"
    MARKDOWN = "markdown"
    HTML = "html"


class WindowPresentation(str, Enum):
    OVERLAY = "overlay"


class WindowAnchor(str, Enum):
    TOP_LEFT = "top-left"
    TOP_CENTER = "top-center"
    TOP_RIGHT = "top-right"
    LEFT_CENTER = "left-center"
    CENTER = "center"
    RIGHT_CENTER = "right-center"
    BOTTOM_LEFT = "bottom-left"
    BOTTOM_CENTER = "bottom-center"
    BOTTOM_RIGHT = "bottom-right"


class WindowEventType(str, Enum):
    READY = "ready"
    MESSAGE = "message"
    ERROR = "error"
    CLOSED = "closed"


@dataclass
class WindowContent:
    type: WindowContentType
    data: str


@dataclass
class WindowOptions:
    title: str = ""
    presentation: WindowPresentation = WindowPresentation.OVERLAY
    width: Optional[float] = None
    height: Optional[float] = None
    anchor: Optional[WindowAnchor] = None
    offset_x: Optional[float] = None
    offset_y: Optional[float] = None
    topmost: Optional[bool] = None
    focusable: Optional[bool] = None
    resizable: Optional[bool] = None
    movable: Optional[bool] = None
    closable: Optional[bool] = None
    hide_chrome: Optional[bool] = None
    drag_content: Optional[bool] = None


@dataclass
class WindowEvent:
    type: WindowEventType
    window_id: str
    document_id: str = ""
    data: Any = None
    err_msg: str = ""


@dataclass
class WindowResult:
    success: bool = False
    err_msg: str = ""


@dataclass
class CreateWindowOption:
    content: WindowContent
    window: WindowOptions = field(default_factory=WindowOptions)
    on_event: Optional[Callable[[Context, WindowEvent], Awaitable[None] | None]] = None


@dataclass
class CreateWindowResult(WindowResult):
    window: Optional[PluginWindow] = None
    window_id: str = ""


@dataclass
class WindowShowOption:
    activate: bool = False


@dataclass
class WindowHideOption:
    pass


@dataclass
class WindowCloseOption:
    pass


@dataclass
class WindowSetOptionsOption:
    width: Optional[float] = None
    height: Optional[float] = None
    anchor: Optional[WindowAnchor] = None
    offset_x: Optional[float] = None
    offset_y: Optional[float] = None
    topmost: Optional[bool] = None


@dataclass
class WindowContentSetOption:
    content: WindowContent


@dataclass
class WindowMessageOption:
    data: Any
    document_id: Optional[str] = None


@dataclass
class WindowShowResult(WindowResult):
    pass


@dataclass
class WindowHideResult(WindowResult):
    pass


@dataclass
class WindowCloseResult(WindowResult):
    pass


@dataclass
class WindowSetOptionsResult(WindowResult):
    pass


@dataclass
class WindowContentSetResult(WindowResult):
    pass


@dataclass
class WindowMessageResult(WindowResult):
    pass


class PluginWindowContent(Protocol):
    async def set(self, ctx: Context, option: WindowContentSetOption) -> WindowContentSetResult: ...
    async def post_message(self, ctx: Context, option: WindowMessageOption) -> WindowMessageResult: ...


class PluginWindow(Protocol):
    id: str
    content: PluginWindowContent

    async def show(self, ctx: Context, option: WindowShowOption) -> WindowShowResult: ...
    async def hide(self, ctx: Context, option: WindowHideOption) -> WindowHideResult: ...
    async def close(self, ctx: Context, option: WindowCloseOption) -> WindowCloseResult: ...
    async def set_options(self, ctx: Context, option: WindowSetOptionsOption) -> WindowSetOptionsResult: ...


def window_option_payload(option: Any) -> dict[str, Any]:
    """Serialize fixed option fields, preserving explicit false/zero and opaque JSON message data."""
    result = {}
    for item in fields(option):
        value = getattr(option, item.name)
        if item.name == "on_event" or (value is None and not (isinstance(option, WindowMessageOption) and item.name == "data")):
            continue
        key = "DocumentID" if item.name == "document_id" else "".join(part.capitalize() for part in item.name.split("_"))
        if item.name in ("window", "content"):
            value = window_option_payload(value)
        result[key] = value
    return result
