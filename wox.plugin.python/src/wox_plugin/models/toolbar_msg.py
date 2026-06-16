"""
Wox toolbar msg models.

Toolbar messages are a first-class channel for long-running plugin work such as
indexing, syncing, and background downloads.
"""

import json
from dataclasses import dataclass, field
from typing import Awaitable, Callable, Dict, List, Optional

from .context import Context
from .image import WoxImage


@dataclass
class ToolbarMsgActionContext:
    """Context passed to a toolbar msg action callback."""

    #: Id of the toolbar msg that owns the action.
    toolbar_msg_id: str = field(default="")
    #: Id of the toolbar msg action that was invoked.
    toolbar_msg_action_id: str = field(default="")
    #: Arbitrary string data attached to the action.
    context_data: Dict[str, str] = field(default_factory=dict)

    @classmethod
    def from_json(cls, json_str: str) -> "ToolbarMsgActionContext":
        """Deserialize action context from the host callback payload."""
        data = json.loads(json_str)
        context_data = data.get("ContextData", {}) or {}
        if not isinstance(context_data, dict):
            context_data = {}
        return cls(
            toolbar_msg_id=data.get("ToolbarMsgId", ""),
            toolbar_msg_action_id=data.get("ToolbarMsgActionId", ""),
            context_data={str(key): str(value) for key, value in context_data.items()},
        )


@dataclass
class ToolbarMsgAction:
    """Execute action shown on the toolbar while a toolbar msg is visible."""

    #: Action label shown in the toolbar.
    name: str
    #: Callback invoked when the user triggers the action.
    action: Optional[Callable[[Context, ToolbarMsgActionContext], Awaitable[None] | None]] = None
    #: Unique action id. Wox will backfill one when omitted.
    id: str = field(default="")
    #: Optional action icon.
    icon: WoxImage = field(default_factory=WoxImage)
    #: Optional hotkey label shown in the toolbar.
    hotkey: str = field(default="")
    #: Whether this action should be treated as the default action.
    is_default: bool = field(default=False)
    #: Whether Wox should stay visible after the action runs.
    prevent_hide_after_action: bool = field(default=False)
    #: Arbitrary string data passed back in ToolbarMsgActionContext.
    context_data: Dict[str, str] = field(default_factory=dict)

    def to_dict(self) -> Dict[str, object]:
        """Convert the action to the JSON-friendly payload sent to the host."""
        return {
            "Id": self.id,
            "Name": self.name,
            "Icon": json.loads(self.icon.to_json()),
            "Hotkey": self.hotkey,
            "IsDefault": self.is_default,
            "PreventHideAfterAction": self.prevent_hide_after_action,
            "ContextData": self.context_data,
        }


@dataclass
class ToolbarMsg:
    """Toolbar msg payload sent through PublicAPI.show_toolbar_msg()."""

    #: Unique toolbar msg id within the current plugin. Reusing it updates the msg in place.
    id: str
    #: Primary text shown in the toolbar.
    title: str
    #: Optional icon shown before the title.
    icon: WoxImage = field(default_factory=WoxImage)
    #: Optional 0-100 progress value for determinate progress.
    progress: Optional[int] = field(default=None)
    #: Show an indeterminate spinner when progress is ongoing but no percentage is available.
    indeterminate: bool = field(default=False)
    #: Optional actions rendered on the right side of the toolbar.
    actions: List[ToolbarMsgAction] = field(default_factory=list)

    def to_json(self) -> str:
        """Serialize the toolbar msg to the host payload format."""
        return json.dumps(
            {
                "Id": self.id,
                "Title": self.title,
                "Icon": json.loads(self.icon.to_json()),
                "Progress": self.progress,
                "Indeterminate": self.indeterminate,
                "Actions": [action.to_dict() for action in self.actions],
            }
        )
