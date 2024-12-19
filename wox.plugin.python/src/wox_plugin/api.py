from typing import Protocol, List, Callable

from .types import MapString, ChatStreamFunc
from .models.context import Context
from .models.query import ChangeQueryParam
from .models.settings import MetadataCommand, PluginSettingDefinitionItem
from .models.result import Conversation


class PublicAPI(Protocol):
    """Public API interface for Wox plugins"""

    async def change_query(self, ctx: Context, query: ChangeQueryParam) -> None:
        """Change the current query"""
        ...

    async def hide_app(self, ctx: Context) -> None:
        """Hide the Wox window"""
        ...

    async def show_app(self, ctx: Context) -> None:
        """Show the Wox window"""
        ...

    async def notify(self, ctx: Context, message: str) -> None:
        """Show a notification"""
        ...

    async def log(self, ctx: Context, level: str, msg: str) -> None:
        """Log a message"""
        ...

    async def get_translation(self, ctx: Context, key: str) -> str:
        """Get translation for a key"""
        ...

    async def get_setting(self, ctx: Context, key: str) -> str:
        """Get setting value"""
        ...

    async def save_setting(
        self, ctx: Context, key: str, value: str, is_platform_specific: bool
    ) -> None:
        """Save setting value"""
        ...

    async def on_setting_changed(
        self, ctx: Context, callback: Callable[[str, str], None]
    ) -> None:
        """Register setting change callback"""
        ...

    async def on_get_dynamic_setting(
        self, ctx: Context, callback: Callable[[str], PluginSettingDefinitionItem]
    ) -> None:
        """Register dynamic setting callback"""
        ...

    async def on_deep_link(
        self, ctx: Context, callback: Callable[[MapString], None]
    ) -> None:
        """Register deep link callback"""
        ...

    async def on_unload(self, ctx: Context, callback: Callable[[], None]) -> None:
        """Register unload callback"""
        ...

    async def register_query_commands(
        self, ctx: Context, commands: List[MetadataCommand]
    ) -> None:
        """Register query commands"""
        ...

    async def llm_stream(
        self, ctx: Context, conversations: List[Conversation], callback: ChatStreamFunc
    ) -> None:
        """Stream LLM responses"""
        ...
