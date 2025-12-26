"""
Wox Public API

This module defines the PublicAPI protocol that provides methods for plugins
to interact with Wox. The API is passed to plugins during initialization
and provides access to UI control, settings, logging, and more.
"""

from typing import Awaitable, Callable, Dict, List, Optional, Protocol

from .models.ai import AIModel, ChatStreamCallback, Conversation
from .models.context import Context
from .models.log import LogLevel
from .models.mru import MRUData
from .models.query import ChangeQueryParam, MetadataCommand, Query, RefreshQueryParam, CopyParams
from .models.result import Result, UpdatableResult  # noqa: F401
from .models.setting import PluginSettingDefinitionItem


class PublicAPI(Protocol):
    """
    Public API interface for Wox plugins.

    This Protocol defines all the methods that plugins can use to
    interact with Wox. The API instance is provided during plugin
    initialization via PluginInitParams.

    Method categories:
        - UI Control: show_app, hide_app, is_visible, notify
        - Query: change_query, refresh_query, push_results
        - Settings: get_setting, save_setting, on_setting_changed, on_get_dynamic_setting
        - Logging: log
        - Internationalization: get_translation
        - Results: get_updatable_result, update_result
        - AI: ai_chat_stream
        - MRU: on_mru_restore
        - Callbacks: on_unload, on_deep_link
        - Commands: register_query_commands
        - Clipboard: copy

    Example:
        class MyPlugin:
            async def init(self, ctx: Context, params: PluginInitParams) -> None:
                self.api = params.api

                # Register callbacks
                await self.api.on_setting_changed(ctx, self._on_setting_changed)
                await self.api.on_unload(ctx, self._on_unload)

            async def query(self, ctx: Context, query: Query) -> List[Result]:
                # Use API methods
                await self.api.log(ctx, LogLevel.INFO, "Processing query")
                return results
    """

    async def change_query(self, ctx: Context, query: ChangeQueryParam) -> None:
        """
        Change the current query in Wox.

        This method allows you to programmatically change what query
        is active in Wox. Useful for:
        - Implementing search suggestions
        - Chain queries together
        - Navigate between query types

        Args:
            ctx: Context
            query: New query parameters (type and content)

        Example:
            # Change to a text input query
            await api.change_query(ctx, ChangeQueryParam(
                query_type=QueryType.INPUT,
                query_text="new search text"
            ))

            # Change to a selection query
            await api.change_query(ctx, ChangeQueryParam(
                query_type=QueryType.SELECTION,
                query_selection=Selection(
                    type=SelectionType.TEXT,
                    text="selected text"
                )
            ))
        """
        ...

    async def hide_app(self, ctx: Context) -> None:
        """
        Hide the Wox window.

        Closes the Wox UI. This is useful after completing an action
        where the user doesn't need to see Wox anymore.

        Args:
            ctx: Context

        Example:
            async def my_action(action_ctx: ActionContext):
                # Do something
                await perform_task()

                # Hide Wox
                await api.hide_app(ctx)
        """
        ...

    async def show_app(self, ctx: Context) -> None:
        """
        Show the Wox window.

        Opens the Wox UI. Useful for bringing Wox to the foreground
        or re-showing it after hiding.

        Args:
            ctx: Context

        Example:
            # Show Wox with all text selected
            await api.show_app(ctx)
        """
        ...

    async def is_visible(self, ctx: Context) -> bool:
        """
        Check if Wox window is currently visible.

        Returns True if the Wox window is shown, False if hidden.

        This is useful for plugins that perform periodic updates
        (e.g., CPU/memory monitoring) to avoid wasting resources
        when the window is hidden.

        Args:
            ctx: Context

        Returns:
            bool: True if visible, False if hidden

        Example:
            async def refresh_data(ctx: Context):
                if not await api.is_visible(ctx):
                    return  # Skip update, window is hidden

                # Update data...
                await update_display()
        """
        ...

    async def notify(self, ctx: Context, message: str) -> None:
        """
        Show a notification message.

        Displays a system notification to the user. Useful for
        async operations where the result is shown later.

        Args:
            ctx: Context
            message: Message text to display (supports i18n keys)

        Example:
            await api.notify(ctx, "Download complete!")
            await api.notify(ctx, "i18n:plugin.download_complete")
        """
        ...

    async def log(self, ctx: Context, level: LogLevel, msg: str) -> None:
        """
        Write log message.

        Logs a message at the specified level. Logs are written
        to the Wox log file and can be viewed for debugging.

        Args:
            ctx: Context
            level: Log level (INFO, ERROR, DEBUG, WARNING)
            msg: Message to log

        Example:
            await api.log(ctx, LogLevel.INFO, "Plugin initialized")
            await api.log(ctx, LogLevel.ERROR, f"Failed to load: {error}")
            await api.log(ctx, LogLevel.DEBUG, f"Processing item {i}")
        """
        ...

    async def get_translation(self, ctx: Context, key: str) -> str:
        """
        Get translation for a key.

        Returns the translated string for the given i18n key.
        Falls back to the key if no translation is found.

        Args:
            ctx: Context
            key: Translation key (e.g., "plugin.title", "plugin.error")

        Returns:
            str: Translated string or the key if not found

        Example:
            title = await api.get_translation(ctx, "plugin.title")
            error = await api.get_translation(ctx, "plugin.error.not_found")
        """
        ...

    async def get_setting(self, ctx: Context, key: str) -> str:
        """
        Get setting value.

        Retrieves the current value for a setting key. Returns the
        default value if the user hasn't set a custom value.

        Args:
            ctx: Context
            key: Setting key defined in your plugin settings

        Returns:
            str: Current setting value (or default)

        Example:
            api_key = await api.get_setting(ctx, "api_key")
            enabled = await api.get_setting(ctx, "enabled")
            is_enabled = enabled.lower() == "true"
        """
        ...

    async def save_setting(self, ctx: Context, key: str, value: str, is_platform_specific: bool) -> None:
        """
        Save setting value.

        Stores a setting value. If is_platform_specific is True,
        the value is stored separately for each platform.

        Args:
            ctx: Context
            key: Setting key
            value: Value to store
            is_platform_specific: Whether to store per-platform

        Example:
            # Save a regular setting
            await api.save_setting(ctx, "username", "john", False)

            # Save platform-specific setting
            await api.save_setting(ctx, "path", "/usr/local/bin", True)
        """
        ...

    async def on_setting_changed(
        self,
        ctx: Context,
        callback: Callable[[Context, str, str], Awaitable[None] | None],
    ) -> None:
        """
        Register setting change callback.

        The callback is invoked whenever a setting value changes.
        Use this to react to setting changes in real-time.

        Args:
            ctx: Context
            callback: Function called when setting changes
                    Receives: (context, key, new_value)

        Example:
            async def _on_setting_changed(ctx: Context, key: str, value: str):
                if key == "api_key":
                    self.api_key = value
                    await self.reload_data()

            await api.on_setting_changed(ctx, self._on_setting_changed)
        """
        ...

    async def on_get_dynamic_setting(
        self,
        ctx: Context,
        callback: Callable[[Context, str], PluginSettingDefinitionItem | Awaitable[PluginSettingDefinitionItem]],
    ) -> None:
        """
        Register dynamic setting callback.

        Dynamic settings are generated at runtime based on current
        state. The callback is invoked when Wox needs to display
        the setting.

        Args:
            ctx: Context
            callback: Function that returns the setting definition
                    Receives: (context, key)
                    Returns: PluginSettingDefinitionItem

        Example:
            async def _on_get_dynamic_setting(ctx: Context, key: str):
                if key == "dynamic_select":
                    options = [
                        {"label": "Option 1", "value": "opt1"},
                        {"label": "Option 2", "value": "opt2"},
                    ]
                    return create_select_setting(key, "Choose", options)

            await api.on_get_dynamic_setting(ctx, self._on_get_dynamic_setting)
        """
        ...

    async def on_deep_link(
        self,
        ctx: Context,
        callback: Callable[[Context, Dict[str, str]], Awaitable[None] | None],
    ) -> None:
        """
        Register deep link callback.

        Deep links allow external apps/websites to invoke your plugin
        with parameters. The callback receives the arguments.

        Args:
            ctx: Context
            callback: Function called when deep link is received
                    Receives: (context, arguments_dict)

        Example:
            # Deep link URL: wox://myplugin?action=open&id=123
            async def _on_deep_link(ctx: Context, args: Dict[str, str]):
                action = args.get("action")
                item_id = args.get("id")
                if action == "open" and item_id:
                    await self.open_item(item_id)

            await api.on_deep_link(ctx, self._on_deep_link)
        """
        ...

    async def on_unload(self, ctx: Context, callback: Callable[[Context], Awaitable[None] | None]) -> None:
        """
        Register unload callback.

        The callback is invoked when the plugin is being unloaded
        (e.g., Wox is shutting down or reloading plugins). Use this
        to clean up resources.

        Args:
            ctx: Context
            callback: Function called on unload

        Example:
            async def _on_unload(ctx: Context):
                # Save state
                await self.save_state()

                # Stop background tasks
                self.stop_tasks()

                # Close connections
                await self.close_connections()

            await api.on_unload(ctx, self._on_unload)
        """
        ...

    async def register_query_commands(self, ctx: Context, commands: List[MetadataCommand]) -> None:
        """
        Register query commands.

        Commands provide structured sub-commands for your plugin.
        For example, ">todo add" and ">todo list" both use the
        "todo" trigger keyword but have different commands.

        Args:
            ctx: Context
            commands: List of commands to register

        Example:
            await api.register_query_commands(ctx, [
                MetadataCommand(
                    command="add",
                    description="Add a new todo"
                ),
                MetadataCommand(
                    command="list",
                    description="List all todos"
                ),
            ])
        """
        ...

    async def ai_chat_stream(
        self,
        ctx: Context,
        model: AIModel,
        conversations: List[Conversation],
        callback: ChatStreamCallback,
    ) -> None:
        """
        Start an AI chat stream.

        Sends a conversation history to an AI model and receives
        streaming responses via the callback.

        Args:
            ctx: Context
            model: AI model to use (provider and name)
            conversations: Conversation history
            callback: Stream callback function
                     Receives ChatStreamData with status and content

        Example:
            def on_stream_data(stream_data: ChatStreamData):
                if stream_data.status == ChatStreamDataType.STREAMING:
                    update_display(stream_data.data)
                elif stream_data.status == ChatStreamDataType.FINISHED:
                    finalize(stream_data.data)

            await api.ai_chat_stream(
                ctx,
                model=AIModel(name="gpt-4", provider="openai"),
                conversations=[
                    Conversation.new_user_message("Hello!"),
                ],
                callback=on_stream_data
            )
        """
        ...

    async def on_mru_restore(
        self,
        ctx: Context,
        callback: Callable[[Context, MRUData], Optional[Result] | Awaitable[Optional[Result]]],
    ) -> None:
        """
        Register MRU (Most Recently Used) restore callback.

        The callback is invoked when a user selects an MRU item.
        Return a Result to restore it, or None if the item is
        no longer valid (it will be removed from MRU).

        Args:
            ctx: Context
            callback: Function that restores MRU item
                    Receives: MRUData
                    Returns: Result or None

        Example:
            async def _on_mru_restore(ctx: Context, mru: MRUData):
                path = mru.context_data.get("path")
                if path and os.path.exists(path):
                    return Result(
                        title=os.path.basename(path),
                        sub_title=path,
                        icon=WoxImage.new_absolute(path)
                    )
                return None  # Remove from MRU

            await api.on_mru_restore(ctx, self._on_mru_restore)
        """
        ...

    async def get_updatable_result(self, ctx: Context, result_id: str) -> Optional[UpdatableResult]:
        """
        Get the current state of a result displayed in the UI.

        Returns UpdatableResult with current values if the result
        is still visible. Returns None if the result is no longer
        visible (e.g., user changed query).

        System actions and tails (like favorite icon) are filtered
        out and re-added automatically by update_result().

        Args:
            ctx: Context
            result_id: ID of the result to get

        Returns:
            Optional[UpdatableResult]: Current state or None

        Example:
            # In an action handler
            updatable_result = await api.get_updatable_result(ctx, result_id)
            if updatable_result is None:
                return  # Result no longer visible

            # Modify fields
            updatable_result.title = "Updated title"
            updatable_result.tails.append(ResultTail(...))

            # Apply updates
            await api.update_result(ctx, updatable_result)
        """
        ...

    async def update_result(self, ctx: Context, result: UpdatableResult) -> bool:
        """
        Update a query result displayed in the UI.

        Returns True if the result was updated (still visible).
        Returns False if the result is no longer visible.

        This is designed for long-running operations in Action handlers.
        Set prevent_hide_after_action=True in your action to keep
        Wox visible during updates.

        Args:
            ctx: Context
            result: UpdatableResult with id (required) and optional fields

        Returns:
            bool: True if updated, False if no longer visible

        Example:
            async def my_action(action_ctx: ActionContext):
                # Update title
                await api.update_result(ctx, UpdatableResult(
                    id=action_ctx.result_id,
                    title="Downloading... 50%"
                ))

                # Update multiple fields
                await api.update_result(ctx, UpdatableResult(
                    id=action_ctx.result_id,
                    title="Processing...",
                    tails=[ResultTail(...)]
                ))
        """
        ...

    async def push_results(self, ctx: Context, query: Query, results: List[Result]) -> bool:
        """
        Push additional results for the current query.

        Returns True if results were accepted (query still active).
        Returns False if query is no longer active (results ignored).

        Useful for:
        - Streaming results as they become available
        - Showing partial results before full completion
        - Adding results from long-running operations

        Args:
            ctx: Context
            query: Current query (must match the active query)
            results: Results to append to the list

        Returns:
            bool: True if accepted, False if query changed

        Example:
            async def query_with_streaming(ctx: Context, query: Query) -> List[Result]:
                # Return initial results immediately
                initial_results = await get_quick_results(query)
                asyncio.create_task(fetch_more_results(ctx, query))
                return initial_results

            async def fetch_more_results(ctx: Context, query: Query):
                # Fetch and push results as they arrive
                for batch in await fetch_in_batches(query):
                    await api.push_results(ctx, query, batch)
        """
        ...

    async def refresh_query(self, ctx: Context, param: RefreshQueryParam) -> None:
        """
        Re-execute the current query with existing text.

        Useful when plugin data changes and you want to update
        results without user retyping.

        Args:
            ctx: Context
            param: RefreshQueryParam controlling selection behavior

        Example:
            # After marking as favorite - keep selection
            await api.refresh_query(ctx, RefreshQueryParam(
                preserve_selected_index=True
            ))

            # After deleting item - reset to first
            await api.refresh_query(ctx, RefreshQueryParam(
                preserve_selected_index=False
            ))
        """
        ...

    async def copy(self, ctx: Context, params: CopyParams) -> None:
        """
        Copy text or image to system clipboard.

        Args:
            ctx: Context
            params: CopyParams with content type and data

        Example:
            # Copy text
            await api.copy(ctx, CopyParams(
                type=CopyType.TEXT,
                text="Hello, World!"
            ))

            # Copy image
            await api.copy(ctx, CopyParams(
                type=CopyType.IMAGE,
                wox_image=WoxImage.new_absolute("/path/to/image.png").to_dict()
            ))
        """
        ...
