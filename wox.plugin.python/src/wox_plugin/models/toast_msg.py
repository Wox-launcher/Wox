"""Transient notification options, available since Wox 2.5.0."""

from dataclasses import dataclass, field
from typing import Awaitable, Callable, Dict, Optional

from .context import Context


@dataclass
class ToastMsgActionContext:
    context_data: Dict[str, str] = field(default_factory=dict)


@dataclass
class ToastMsgAction:
    name: str
    action: Callable[[Context, ToastMsgActionContext], Awaitable[None] | None]
    context_data: Dict[str, str] = field(default_factory=dict)


@dataclass
class ShowToastMsgOption:
    message: str
    title: str = ""
    action: Optional[ToastMsgAction] = None


@dataclass
class ShowToastMsgResult:
    success: bool = False
    err_msg: str = ""
