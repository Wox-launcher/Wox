"""
Wox Plugin Interface

This module defines the plugin interface that all Wox plugins must implement.
The Plugin protocol specifies the required methods for plugin lifecycle and
query handling.
"""

from typing import Protocol, List
from dataclasses import dataclass

from .models.context import Context
from .models.query import Query
from .models.result import Result
from .api import PublicAPI


@dataclass
class PluginInitParams:
    """
    Parameters passed to the plugin during initialization.

    This dataclass contains the initialization parameters provided
    to the plugin's `init()` method.

    Attributes:
        api: The PublicAPI instance for interacting with Wox
        plugin_directory: Absolute path to the plugin's directory

    Example usage:
        async def init(self, ctx: Context, params: PluginInitParams) -> None:
            api = params.api
            plugin_dir = params.plugin_directory

            # Load configuration from plugin directory
            config_path = os.path.join(plugin_dir, "config.json")

            # Register callbacks
            await api.on_setting_changed(ctx, self.on_setting_changed)
    """

    api: PublicAPI
    """
    The PublicAPI instance for interacting with Wox.

    Store this if you need to call API methods later. The API
    provides methods for:
    - Query manipulation (change_query, refresh_query)
    - UI control (show_app, hide_app, notify)
    - Settings (get_setting, save_setting)
    - Logging (log)
    - And more...

    Example:
        # Store API for later use
        self.api = params.api

        # Use in other methods
        await self.api.notify(ctx, "Hello!")
    """

    plugin_directory: str
    """
    Absolute path to the plugin's directory.

    Use this to access plugin-specific files like:
    - Configuration files
    - Resource files (images, templates)
    - Data files
    - Python modules within the plugin

    Example:
        # Load a config file
        config_path = os.path.join(params.plugin_directory, "config.json")

        # Load an image
        icon_path = os.path.join(params.plugin_directory, "icons", "app.png")

        # Load a Python module
        import sys
        sys.path.insert(0, params.plugin_directory)
    """


class Plugin(Protocol):
    """
    Plugin interface that all Wox plugins must implement.

    This Protocol defines the required methods that every Wox plugin
    must provide. Plugins are instantiated once when Wox starts,
    then the lifecycle methods are called.

    Lifecycle:
        1. Plugin class is instantiated
        2. init() is called with initialization parameters
        3. query() is called whenever the user triggers a query

    Example implementation:
        class MyPlugin:
            async def init(self, ctx: Context, params: PluginInitParams) -> None:
                # One-time initialization
                self.api = params.api
                self.data = load_data()

            async def query(self, ctx: Context, query: Query) -> List[Result]:
                # Handle user query
                results = []
                for item in self.data:
                    if query.search.lower() in item.name.lower():
                        results.append(Result(
                            title=item.name,
                            sub_title=item.description,
                            icon=WoxImage.new_emoji("🔍")
                        ))
                return results
    """

    async def init(self, ctx: Context, init_params: PluginInitParams) -> None:
        """
        Initialize the plugin.

        This method is called once when the plugin is first loaded.
        Use this to:
        - Store the API reference for later use
        - Load configuration files
        - Initialize data structures
        - Register callbacks (on_setting_changed, on_unload, etc.)
        - Start background tasks

        Args:
            ctx: Context for this initialization request
            init_params: Initialization parameters including API and directory

        Example:
            async def init(self, ctx: Context, init_params: PluginInitParams) -> None:
                # Store API reference
                self.api = init_params.api

                # Load settings
                self.api_key = await self.api.get_setting(ctx, "api_key")

                # Register callbacks
                await self.api.on_setting_changed(ctx, self._on_setting_changed)
                await self.api.on_unload(ctx, self._on_unload)

                # Start background task
                self._start_refresh_task()

        Note:
            This method should return quickly. For long-running
            initialization, consider using background tasks.
        """
        ...

    async def query(self, ctx: Context, query: Query) -> List[Result]:
        """
        Handle user query and return results.

        This method is called whenever:
        - The user types in the search box with your plugin's trigger keyword
        - The query changes (while still matching your plugin)
        - The query is refreshed (via api.refresh_query())

        The method should return a list of Result objects that match
        the user's query. Results are sorted by their score field
        (higher scores appear first).

        Args:
            ctx: Context for this query request
            query: The query object containing search text and metadata

        Returns:
            List of Result objects matching the query

        Example:
            async def query(self, ctx: Context, query: Query) -> List[Result]:
                results = []

                # Get the search text
                search = query.search.lower()

                # Filter and create results
                for item in self.items:
                    if search in item.name.lower():
                        results.append(Result(
                            title=item.name,
                            sub_title=item.description,
                            icon=WoxImage.new_relative(item.icon_path),
                            score=self._calculate_score(item, search),
                            actions=[
                                ResultAction(
                                    name="Open",
                                    icon=WoxImage.new_emoji("📂"),
                                    is_default=True
                                )
                            ]
                        ))

                return results

        Query structure:
            - query.trigger_keyword: Your plugin's trigger keyword (if set)
            - query.command: Command keyword (if registered)
            - query.search: The actual search text
            - query.raw_query: Full query including trigger keyword
            - query.type: INPUT or SELECTION
            - query.selection: Selected text/files (for SELECTION type)
            - query.env: Environment context (active window, browser URL)

        Tips:
            - Return results sorted by relevance (use the score field)
            - Use async operations efficiently (don't block on I/O)
            - Consider caching expensive operations
            - Return empty list if no results match
            - Use query.is_global_query() to check if it's a global query
        """
        ...
