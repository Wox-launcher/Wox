from typing import Protocol, List

from .models.context import Context
from .models.query import Query
from .models.result import Result
from .api import PublicAPI


class PluginInitParams:
    """Parameters for plugin initialization"""

    API: PublicAPI
    PluginDirectory: str


class Plugin(Protocol):
    """Plugin interface that all Wox plugins must implement"""

    async def init(self, ctx: Context, init_params: PluginInitParams) -> None:
        """Initialize the plugin"""
        ...

    async def query(self, ctx: Context, query: Query) -> List[Result]:
        """Handle user query"""
        ...


class BasePlugin:
    """Base implementation of Plugin with common functionality"""

    def __init__(self):
        self.api: PublicAPI = None
        self.plugin_dir: str = None

    async def init(self, ctx: Context, init_params: PluginInitParams) -> None:
        """Initialize the plugin with API and plugin directory"""
        self.api = init_params.API
        self.plugin_dir = init_params.PluginDirectory

    async def query(self, ctx: Context, query: Query) -> List[Result]:
        """Handle user query - must be implemented by subclasses"""
        raise NotImplementedError("Subclasses must implement query method")
