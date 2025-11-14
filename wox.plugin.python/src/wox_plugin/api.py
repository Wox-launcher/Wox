from typing import Callable, Dict, List, Optional, Protocol

from .models.ai import AIModel, ChatStreamCallback, Conversation
from .models.context import Context
from .models.mru import MRUData
from .models.query import ChangeQueryParam, MetadataCommand, RefreshQueryParam
from .models.result import Result, UpdatableResult, UpdatableResultAction  # noqa: F401
from .models.setting import PluginSettingDefinitionItem


class PublicAPI(Protocol):
    """Public API interface for Wox plugins"""

    async def change_query(self, ctx: Context, query: ChangeQueryParam) -> None:
        """Change the current query in Wox"""
        ...

    async def hide_app(self, ctx: Context) -> None:
        """Hide the Wox window"""
        ...

    async def show_app(self, ctx: Context) -> None:
        """Show the Wox window"""
        ...

    async def is_visible(self, ctx: Context) -> bool:
        """Check if Wox window is currently visible"""
        ...

    async def notify(self, ctx: Context, message: str) -> None:
        """Show a notification message"""
        ...

    async def log(self, ctx: Context, level: str, msg: str) -> None:
        """Write log message"""
        ...

    async def get_translation(self, ctx: Context, key: str) -> str:
        """Get translation for a key"""
        ...

    async def get_setting(self, ctx: Context, key: str) -> str:
        """Get setting value"""
        ...

    async def save_setting(self, ctx: Context, key: str, value: str, is_platform_specific: bool) -> None:
        """Save setting value"""
        ...

    async def on_setting_changed(self, ctx: Context, callback: Callable[[str, str], None]) -> None:
        """Register setting change callback"""
        ...

    async def on_get_dynamic_setting(self, ctx: Context, callback: Callable[[str], PluginSettingDefinitionItem]) -> None:
        """Register dynamic setting callback"""
        ...

    async def on_deep_link(self, ctx: Context, callback: Callable[[Dict[str, str]], None]) -> None:
        """Register deep link callback"""
        ...

    async def on_unload(self, ctx: Context, callback: Callable[[], None]) -> None:
        """Register unload callback"""
        ...

    async def register_query_commands(self, ctx: Context, commands: List[MetadataCommand]) -> None:
        """Register query commands"""
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

        Args:
            ctx: Context
            model: AI model to use
            conversations: Conversation history
            callback: Stream callback function to receive AI responses
                     The callback takes two parameters:
                     - stream_type: ChatStreamDataType, indicates the stream status
                     - data: str, the stream content
        """
        ...

    async def on_mru_restore(self, ctx: Context, callback: Callable[[MRUData], Optional[Result]]) -> None:
        """Register MRU restore callback

        Args:
            ctx: Context
            callback: Callback function that takes MRUData and returns Result or None
                     Return None if the MRU data is no longer valid
        """
        ...

    async def get_updatable_result(self, ctx: Context, result_id: str) -> Optional[UpdatableResult]:
        """
        Get the current state of a result that is displayed in the UI.

        Returns UpdatableResult with current values if the result is still visible.
        Returns None if the result is no longer visible.

        Note: System actions and tails (like favorite icon) are automatically filtered out.
        They will be re-added by the system when you call update_result().

        Example:
            # In an action handler
            async def toggle_favorite(action_context: ActionContext):
                # Get current result state
                updatable_result = await api.get_updatable_result(ctx, action_context.result_id)
                if updatable_result is None:
                    return  # Result no longer visible

                # Modify the result
                updatable_result.title = "Updated title"
                updatable_result.tails.append(ResultTail(type=ResultTailType.TEXT, text="New tail"))

                # Update the result
                await api.update_result(ctx, updatable_result)

        Args:
            ctx: Context
            result_id: ID of the result to get

        Returns:
            Optional[UpdatableResult]: Current result state, or None if not visible
        """
        ...

    async def update_result(self, ctx: Context, result: UpdatableResult) -> bool:
        """
        Update a query result that is currently displayed in the UI.

        Returns True if the result was successfully updated (still visible in UI).
        Returns False if the result is no longer visible.

        This method is designed for long-running operations within Action handlers.
        Best practices:
        - Set prevent_hide_after_action=True in your action
        - Only use during action execution or in background tasks spawned by actions
        - For periodic updates, start a timer in init() and track result IDs

        Example:
            # In an action handler
            async def my_action(action_context: ActionContext):
                # Update only the title
                success = await api.update_result(ctx, UpdatableResult(
                    id=action_context.result_id,
                    title="Downloading... 50%"
                ))

                # Update title and tails
                success = await api.update_result(ctx, UpdatableResult(
                    id=action_context.result_id,
                    title="Processing...",
                    tails=[ResultTail(type=ResultTailType.TEXT, text="Step 1/3")]
                ))

        Args:
            ctx: Context
            result: UpdatableResult with id (required) and optional fields to update

        Returns:
            bool: True if updated successfully, False if result no longer visible
        """
        ...

    async def update_result_action(self, ctx: Context, action: UpdatableResultAction) -> bool:
        """
        Update a single action within a query result that is currently displayed in the UI.

        Returns True if the action was successfully updated (result still visible in UI).
        Returns False if the result is no longer visible.

        This method is designed for updating action UI after execution, such as toggling
        between "Add to favorite" and "Remove from favorite" states.

        Best practices:
        - Set prevent_hide_after_action=True in your action
        - Use action_context.result_action_id to identify which action to update
        - Only update fields that have changed (use None for fields you don't want to update)

        Example:
            # In an action handler
            async def toggle_favorite(action_context: ActionContext):
                if is_favorite:
                    remove_favorite()
                    success = await api.update_result_action(ctx, UpdatableResultAction(
                        result_id=action_context.result_id,
                        action_id=action_context.result_action_id,
                        name="Add to favorite",
                        icon=WoxImage(image_type="emoji", image_data="⭐")
                    ))
                else:
                    add_favorite()
                    success = await api.update_result_action(ctx, UpdatableResultAction(
                        result_id=action_context.result_id,
                        action_id=action_context.result_action_id,
                        name="Remove from favorite",
                        icon=WoxImage(image_type="emoji", image_data="❌")
                    ))

        Args:
            ctx: Context
            action: UpdatableResultAction with result_id, action_id (required) and optional fields to update

        Returns:
            bool: True if updated successfully, False if result no longer visible
        """
        ...

    async def refresh_query(self, ctx: Context, param: RefreshQueryParam) -> None:
        """
        Re-execute the current query with the existing query text.
        This is useful when plugin data changes and you want to update the displayed results.

        Args:
            ctx: Context
            param: RefreshQueryParam to control refresh behavior

        Example - Refresh after marking item as favorite:
            async def mark_favorite(action_context: ActionContext):
                mark_as_favorite(item)
                # Refresh query and preserve user's current selection
                await api.refresh_query(ctx, RefreshQueryParam(preserve_selected_index=True))

        Example - Refresh after deleting item:
            async def delete_item(action_context: ActionContext):
                delete(item)
                # Refresh query and reset to first item
                await api.refresh_query(ctx, RefreshQueryParam(preserve_selected_index=False))
        """
        ...
