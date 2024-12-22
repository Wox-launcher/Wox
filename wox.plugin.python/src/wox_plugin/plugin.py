from typing import Protocol, List
from dataclasses import dataclass

from .models.context import Context
from .models.query import Query
from .models.result import Result
from .api import PublicAPI


@dataclass
class PluginInitParams:
    """Parameters for plugin initialization"""

    api: PublicAPI
    plugin_directory: str


class Plugin(Protocol):
    """Plugin interface that all Wox plugins must implement"""

    async def init(self, ctx: Context, init_params: PluginInitParams) -> None:
        """Initialize the plugin"""
        ...

    async def query(self, ctx: Context, query: Query) -> List[Result]:
        """Handle user query"""
        ...
