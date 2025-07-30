from typing import Protocol, Callable, Dict, List, Optional

from .models.query import MetadataCommand
from .models.context import Context
from .models.query import ChangeQueryParam
from .models.ai import AIModel, Conversation, ChatStreamCallback
from .models.mru import MRUData
from .models.result import Result
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
