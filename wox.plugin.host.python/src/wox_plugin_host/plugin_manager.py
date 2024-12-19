from typing import Dict, TypeVar, Callable
from dataclasses import dataclass
import asyncio
from wox_plugin import PublicAPI, Plugin, RefreshableResult, ActionContext

@dataclass
class PluginInstance:
    plugin: Plugin
    api: PublicAPI
    module_path: str
    actions: Dict[str, Callable[[ActionContext], None]]
    refreshes: Dict[str, Callable[[RefreshableResult], RefreshableResult]]

T = TypeVar('T')

# Global state with strong typing
plugin_instances: Dict[str, PluginInstance] = {}
waiting_for_response: Dict[str, asyncio.Future[T]] = {} 