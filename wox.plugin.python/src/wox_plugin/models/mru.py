"""
Wox MRU (Most Recently Used) Models

This module provides models for handling Most Recently Used data in Wox plugins.

The MRU system allows plugins to display recently used items to users, providing
quick access to previously selected results. Users can select an MRU item to
restore it as a regular result.
"""

import json
from dataclasses import dataclass
from typing import TYPE_CHECKING, Awaitable, Callable, Dict, Optional

from .context import Context
from .image import WoxImage

if TYPE_CHECKING:
    from .result import Result


@dataclass
class MRUData:
    """
    MRU (Most Recently Used) data structure.

    Represents an item that was recently used by the user. When a user
    executes an action on a result, the plugin can save it as MRU data.
    Later, when the user triggers the MRU restore, the plugin receives
    this data and can reconstruct the original Result.

    Attributes:
        plugin_id: The unique identifier of the plugin that owns this MRU
        title: Display title of the MRU item
        sub_title: Display subtitle (additional description)
        icon: Icon to display for the MRU item
        context_data: Arbitrary data to store with the MRU for later restoration

    Example usage:
        # Save MRU after user action
        async def my_action(action_context: ActionContext):
            # Save to MRU
            mru = MRUData(
                plugin_id="my.plugin",
                title="Document",
                sub_title="/path/to/document.txt",
                icon=WoxImage.new_emoji("📄"),
                context_data={"path": "/path/to/document.txt"}
            )
            await api.save_mru(ctx, mru)

        # Restore MRU later
        async def on_mru_restore(ctx: Context, mru: MRUData) -> Optional[Result]:
            path = mru.context_data.get("path")
            if path and os.path.exists(path):
                return Result(
                    title=os.path.basename(path),
                    sub_title=path,
                    icon=WoxImage.new_absolute(path)
                )
            return None  # Item no longer exists
    """

    plugin_id: str
    """
    The unique identifier of the plugin that owns this MRU.

    This should match your plugin's ID as defined in the plugin manifest.
    Used to route MRU restore requests to the correct plugin.
    """

    title: str
    """
    Display title of the MRU item.

    This is shown to the user in the MRU list and should be a concise,
    human-readable description of what the item is.
    """

    sub_title: str
    """
    Display subtitle of the MRU item.

    Provides additional context or details about the item. This is
    shown below the title in the MRU list.
    """

    icon: WoxImage
    """
    Icon to display for the MRU item.

    Should visually represent the type of item (file, folder, URL, etc.)
    """

    context_data: Dict[str, str]
    """
    Arbitrary data to store with the MRU for later restoration.

    Use this to store any data needed to reconstruct the original Result
    when the user restores the MRU item. Common use cases:
        - File paths
        - URLs
        - Database IDs
        - Serialized state
    """

    @classmethod
    def from_dict(cls, data: dict) -> "MRUData":
        """
        Create MRUData from dictionary with camelCase naming.

        Args:
            data: Dictionary with MRU data (camelCase keys)

        Returns:
            A new MRUData instance
        """
        context_data = data.get("ContextData", {}) or {}
        if isinstance(context_data, str):
            try:
                context_data = json.loads(context_data)
            except Exception:
                context_data = {}

        return cls(
            plugin_id=data.get("PluginID", ""),
            title=data.get("Title", ""),
            sub_title=data.get("SubTitle", ""),
            icon=WoxImage.from_dict(data.get("Icon", {})),
            context_data=context_data if isinstance(context_data, dict) else {},
        )

    def to_dict(self) -> dict:
        """
        Convert MRUData to dictionary with camelCase naming.

        Returns:
            Dictionary representation with camelCase keys
        """
        return {
            "PluginID": self.plugin_id,
            "Title": self.title,
            "SubTitle": self.sub_title,
            "Icon": self.icon.to_dict(),
            "ContextData": self.context_data,
        }


# Type alias for MRU restore callback
#
# This callback is invoked when a user selects an MRU item to restore.
# The plugin receives the MRUData and should return a Result if the
# item can be restored, or None if the item is no longer valid.
#
# Return None to indicate the MRU item should be removed from the list
# (e.g., file was deleted, URL is no longer accessible).
#
# Example:
#     async def restore_mru(ctx: Context, mru: MRUData) -> Optional[Result]:
#         path = mru.context_data.get("path")
#         if path and os.path.exists(path):
#             return Result(
#                 title=os.path.basename(path),
#                 sub_title=path,
#                 icon=WoxImage.new_absolute(path)
#             )
#         return None  # Remove from MRU list
#
# Register the callback in init():
#     await api.on_mru_restore(ctx, restore_mru)
MRURestoreCallback = Callable[[Context, "MRUData"], Optional["Result"] | Awaitable[Optional["Result"]]]
