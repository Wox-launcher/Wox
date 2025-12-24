from typing import Dict, Any, Callable, Optional, Awaitable
from dataclasses import dataclass
import asyncio
from wox_plugin import ActionContext, Context, FormActionContext, Plugin, PublicAPI


@dataclass
class PluginInstance:
    plugin: Plugin
    api: Optional[PublicAPI]
    plugin_dir: str
    module_name: str
    actions: Dict[str, Callable[[Context, ActionContext], Awaitable[None]]]
    form_actions: Dict[str, Callable[[Context, FormActionContext], Awaitable[None]]]


# Global state with strong typing
plugin_instances: Dict[str, PluginInstance] = {}
waiting_for_response: Dict[str, asyncio.Future[Any]] = {}
