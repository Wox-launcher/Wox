"""Plugin-owned windows. All geometry uses logical units, never physical pixels."""

from __future__ import annotations

from dataclasses import dataclass, field, fields
from enum import Enum
from typing import Any, Awaitable, Callable, Optional, Protocol, TypedDict

from .context import Context


class WindowContentType(str, Enum):
    WOXVIEW = "woxview"


class WindowPresentation(str, Enum):
    OVERLAY = "overlay"
    WINDOW = "window"


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
    ERROR = "error"
    CLOSED = "closed"
    WOXVIEW = "woxview_event"
    THEME_CHANGED = "theme_changed"


class WindowThemeChangedData(TypedDict):
    """Opaque event.data payload; PascalCase matches the Core JSON contract."""

    Dark: bool


@dataclass
class WindowContent:
    """Serialized WoxView tree (JSON)."""

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


@dataclass
class WindowEvent:
    type: WindowEventType
    window_id: str
    document_id: str = ""
    # WoxView events contain Handler, Args, Key; theme_changed contains WindowThemeChangedData.
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
class WindowContentUpdateOption:
    """Replace a WoxView tree, optionally pinning the document and ordering updates."""

    data: str
    document_id: Optional[str] = None
    sequence: int = 0


class WindowContentCommand(str, Enum):
    FOCUS = "focus"
    SCROLL_TO = "scrollTo"
    SELECT_ALL = "selectAll"


@dataclass
class WindowContentCommandOption:
    """Queue an operation on a unique node key; deferred failures emit an error event."""

    command: WindowContentCommand
    key: str
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
class WindowContentUpdateResult(WindowResult):
    pass


@dataclass
class WindowContentCommandResult(WindowResult):
    pass


class PluginWindowContent(Protocol):
    async def set(self, ctx: Context, option: WindowContentSetOption) -> WindowContentSetResult: ...
    async def update(self, ctx: Context, option: WindowContentUpdateOption) -> WindowContentUpdateResult: ...
    async def command(self, ctx: Context, option: WindowContentCommandOption) -> WindowContentCommandResult: ...


class PluginWindow(Protocol):
    id: str
    content: PluginWindowContent

    async def show(self, ctx: Context, option: WindowShowOption) -> WindowShowResult: ...
    async def hide(self, ctx: Context, option: WindowHideOption) -> WindowHideResult: ...
    async def close(self, ctx: Context, option: WindowCloseOption) -> WindowCloseResult: ...
    async def set_options(self, ctx: Context, option: WindowSetOptionsOption) -> WindowSetOptionsResult: ...


def window_option_payload(option: Any) -> dict[str, Any]:
    """Serialize fixed option fields, preserving explicit false/zero."""
    result = {}
    for item in fields(option):
        value = getattr(option, item.name)
        if item.name == "on_event" or value is None:
            continue
        key = "DocumentID" if item.name == "document_id" else "".join(part.capitalize() for part in item.name.split("_"))
        if item.name in ("window", "content"):
            value = window_option_payload(value)
        result[key] = value
    return result
