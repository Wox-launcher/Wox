"""Events for global hotkeys declared in plugin settings."""

from __future__ import annotations

import json
from dataclasses import dataclass
from enum import Enum
from typing import Awaitable, Callable, Optional
from .context import Context
from .query import QueryEnv


class HotkeyKind(str, Enum):
    NORMAL_COMBO = "normalCombo"
    DOUBLE_MODIFIER = "doubleModifier"
    CAPS_LOCK_COMBO = "capsLockCombo"
    PRESS_MODIFIER = "pressModifier"
    HOLD_MODIFIER = "holdModifier"
    HOLD_KEY = "holdKey"


class HotkeyPhase(str, Enum):
    TRIGGERED = "triggered"
    STARTED = "started"
    ENDED = "ended"


class HotkeyEndReason(str, Enum):
    RELEASED = "released"
    INTERRUPTED = "interrupted"
    DISABLED = "disabled"
    BINDING_CHANGED = "bindingChanged"
    UNAVAILABLE = "unavailable"


@dataclass
class HotkeyEvent:
    setting_key: str
    kind: HotkeyKind
    phase: HotkeyPhase
    hotkey: str
    activation_id: str
    timestamp: int
    item_id: str = ""
    column_key: str = ""
    env: Optional[QueryEnv] = None
    end_reason: Optional[HotkeyEndReason] = None

    @classmethod
    def from_dict(cls, data: dict) -> HotkeyEvent:
        return cls(
            setting_key=data["SettingKey"],
            kind=HotkeyKind(data["Kind"]),
            phase=HotkeyPhase(data["Phase"]),
            hotkey=data["Hotkey"],
            activation_id=data["ActivationID"],
            timestamp=data["Timestamp"],
            item_id=data.get("ItemID", ""),
            column_key=data.get("ColumnKey", ""),
            env=QueryEnv.from_json(json.dumps(data["Env"])) if data.get("Env") else None,
            end_reason=HotkeyEndReason(data["EndReason"]) if data.get("EndReason") else None,
        )


@dataclass
class OnHotkeyOption:
    callback: Callable[[Context, HotkeyEvent], Awaitable[None] | None]


@dataclass
class OnHotkeyResult:
    success: bool = False
    err_msg: str = ""
