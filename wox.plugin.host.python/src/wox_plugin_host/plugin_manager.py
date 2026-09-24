from typing import Dict, Any, Callable, Optional, Awaitable
from dataclasses import dataclass
import asyncio
from wox_plugin import ActionContext, Context, FormActionContext, Plugin, ToolbarMsgActionContext, PublicAPI


@dataclass
class PluginInstance:
    plugin: Plugin
    api: Optional[PublicAPI]
    plugin_dir: str
    module_name: str
    actions: Dict[str, Callable[[Context, ActionContext], Awaitable[None]]]
    form_actions: Dict[str, Callable[[Context, FormActionContext], Awaitable[None]]]
    toolbar_msg_actions: Dict[str, Callable[[Context, ToolbarMsgActionContext], Awaitable[None] | None]]
    sys_paths: list[str]


# Global state with strong typing
plugin_instances: Dict[str, PluginInstance] = {}
waiting_for_response: Dict[str, asyncio.Future[Any]] = {}

# Plugins keep the PluginAPI created at init. Sends read this socket so a
# host reconnect reaches Wox without replacing that object.
current_connection: Any = None


def set_current_connection(ws: Any) -> None:
    """Remember the live host connection."""
    global current_connection
    current_connection = ws


def close_current_connection(ws: Any) -> bool:
    """Drop ws when it is still current, and fail calls waiting on it.

    A stale close from the replaced socket must not drop the live connection.
    Returns whether this socket was still current.
    """
    global current_connection
    if current_connection is not ws:
        return False

    current_connection = None
    pending = list(waiting_for_response.items())
    waiting_for_response.clear()
    for _request_id, future in pending:
        if not future.done():
            future.set_exception(RuntimeError("host websocket disconnected"))
    return True
