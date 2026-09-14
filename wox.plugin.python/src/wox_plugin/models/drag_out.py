"""Post-drag notifications delivered by the Public API."""

from dataclasses import dataclass, field
from typing import Awaitable, Callable, Dict, List

from .context import Context


@dataclass
class DragOutEvent:
    """
    Terminal status of a result file drag started from this plugin.

    QueryResultDragData is what makes a result draggable. This event cannot
    block or cancel that drag. Native result drags stay copy-only, so a plugin
    that wants take-out to empty its own source should remove those files when
    status is ``success``.
    """

    result_id: str = ""
    files: List[str] = field(default_factory=list)
    status: str = ""

    @classmethod
    def from_dict(cls, data: Dict[str, object] | None) -> "DragOutEvent":
        payload = data or {}
        files = payload.get("Files", payload.get("files", []))
        if not isinstance(files, list):
            files = []
        return cls(
            result_id=str(payload.get("ResultId") or payload.get("result_id") or ""),
            files=[str(item) for item in files if item],
            status=str(payload.get("Status") or payload.get("status") or ""),
        )


@dataclass
class DragOutListenOption:
    callback: Callable[[Context, DragOutEvent], Awaitable[None] | None] | None = None


@dataclass
class DragOutListenResult:
    success: bool = False
