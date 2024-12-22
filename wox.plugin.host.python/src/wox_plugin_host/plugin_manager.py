from typing import Dict, Any, Callable, Optional, Awaitable
from dataclasses import dataclass
import asyncio
from wox_plugin import PublicAPI, Plugin, RefreshableResult, ActionContext


@dataclass
class PluginInstance:
    plugin: Plugin
    api: Optional[PublicAPI]
    module_path: str
    actions: Dict[str, Callable[[ActionContext], Awaitable[None]]]
    refreshes: Dict[str, Callable[[RefreshableResult], Awaitable[RefreshableResult]]]


# Global state with strong typing
plugin_instances: Dict[str, PluginInstance] = {}
waiting_for_response: Dict[str, asyncio.Future[Any]] = {}
