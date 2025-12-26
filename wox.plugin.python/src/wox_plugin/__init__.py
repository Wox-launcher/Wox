"""
Wox Plugin SDK for Python

This package provides the SDK for developing Wox plugins in Python.

Wox is a powerful launcher application that allows users to quickly
search, access, and manipulate information across their system.
Plugins extend Wox's functionality by providing custom search results
and actions.

## Quick Start

To create a plugin, implement the `Plugin` protocol:

```python
from wox_plugin import Plugin, PluginInitParams, Context, Query, Result
from wox_plugin import WoxImage, QueryType, LogLevel
from typing import List

class MyPlugin:
    async def init(self, ctx: Context, params: PluginInitParams) -> None:
        # Store API for later use
        self.api = params.api

        # Log initialization
        await self.api.log(ctx, LogLevel.INFO, "MyPlugin initialized")

    async def query(self, ctx: Context, query: Query) -> List[Result]:
        # Return results based on query
        results = []

        for item in self.get_items():
            if query.search.lower() in item.name.lower():
                results.append(Result(
                    title=item.name,
                    sub_title=item.description,
                    icon=WoxImage.new_emoji("🔍"),
                    score=100
                ))

        return results
```

## Key Components

### Plugin Interface (`Plugin`, `PluginInitParams`)
- `Plugin`: Protocol defining the plugin interface
- `PluginInitParams`: Parameters passed during initialization
- Required methods: `init()`, `query()`

### Public API (`PublicAPI`)
Methods for interacting with Wox:
- **UI Control**: `show_app()`, `hide_app()`, `is_visible()`, `notify()`
- **Query**: `change_query()`, `refresh_query()`, `push_results()`
- **Settings**: `get_setting()`, `save_setting()`, `on_setting_changed()`
- **Logging**: `log()`
- **i18n**: `get_translation()`
- **Results**: `get_updatable_result()`, `update_result()`
- **AI**: `ai_chat_stream()`
- **MRU**: `on_mru_restore()`
- **Callbacks**: `on_unload()`, `on_deep_link()`
- **Commands**: `register_query_commands()`
- **Clipboard**: `copy()`

### Models

#### Query Models (`models/query.py`)
- `Query`: User query with search text, type, selection, environment
- `QueryType`: INPUT (typing) or SELECTION (selected content)
- `SelectionType`: TEXT or FILE selection
- `Selection`: Selected text or file paths
- `QueryEnv`: Environment context (active window, browser URL)
- `ChangeQueryParam`: Parameters to change the query
- `RefreshQueryParam`: Parameters to refresh the query
- `CopyParams`: Parameters for clipboard operations

#### Result Models (`models/result.py`)
- `Result`: Search result with title, icon, preview, actions
- `ResultAction`: User action on a result
- `ResultActionType`: EXECUTE (immediate) or FORM (show form)
- `ActionContext`: Context passed to action callbacks
- `FormActionContext`: Context for form submissions
- `ResultTail`: Additional visual elements (text or image)
- `UpdatableResult`: Result that can be updated in UI

#### Image Models (`models/image.py`)
- `WoxImage`: Image model with multiple types
- `WoxImageType`: ABSOLUTE, RELATIVE, BASE64, SVG, LOTTIE, EMOJI, URL, THEME, FILE_ICON
- Factory methods: `new_base64()`, `new_svg()`, `new_emoji()`, etc.

#### Preview Models (`models/preview.py`)
- `WoxPreview`: Preview content for results
- `WoxPreviewType`: MARKDOWN, TEXT, IMAGE, URL, FILE, REMOTE
- `WoxPreviewScrollPosition`: Control initial scroll position

#### Setting Models (`models/setting.py`)
- `PluginSettingDefinitionItem`: Setting definition
- `PluginSettingDefinitionType`: HEAD, TEXTBOX, CHECKBOX, SELECT, LABEL, NEWLINE, TABLE, DYNAMIC
- `PluginSettingValueStyle`: Visual styling options
- Helper functions: `create_textbox_setting()`, `create_checkbox_setting()`, `create_label_setting()`

#### AI Models (`models/ai.py`)
- `AIModel`: AI model definition (provider and name)
- `Conversation`: Chat message with role and content
- `ConversationRole`: USER or AI
- `ChatStreamData`: Streaming response data
- `ChatStreamDataType`: STREAMING, FINISHED, ERROR

#### Other Models
- `Context` (`models/context.py`): Request-scoped context with trace ID
- `LogLevel` (`models/log.py`): INFO, ERROR, DEBUG, WARNING
- `MRUData` (`models/mru.py`): Most Recently Used item data

## Plugin Metadata

Plugins must declare metadata in a `plugin.json` file:

```json
{
    "ID": "com.myplugin.example",
    "Name": "My Plugin",
    "Author": "Your Name",
    "Version": "1.0.0",
    "MinWoxVersion": "2.0.0",
    "Runtime": "python",
    "Entry": "main.py",
    "TriggerKeywords": ["my"],
    "Description": "My awesome Wox plugin",
    "Website": "https://github.com/user/myplugin",
    "Icon": "https://example.com/icon.png"
}
```

## Query Flow

1. User triggers Wox and types trigger keyword (e.g., "my query")
2. Wox calls `plugin.query()` with:
   - `query.trigger_keyword = "my"`
   - `query.command = ""`
   - `query.search = "query"`
3. Plugin returns `List[Result]`
4. Wox displays results sorted by score

## Actions

Actions are operations users can perform on results:

```python
ResultAction(
    name="Copy",
    icon=WoxImage.new_emoji("📋"),
    is_default=True,
    action=lambda ctx, ac: self.copy_to_clipboard(ac.context_data)
)
```

## Settings

Define settings in your plugin's JSON or return from `get_setting_definitions()`:

```python
settings = [
    create_textbox_setting(
        key="api_key",
        label="API Key",
        tooltip="Enter your API key"
    ),
    create_checkbox_setting(
        key="enabled",
        label="Enable Feature",
        default_value="true"
    )
]
```

## For More Information

- Wox Documentation: https://github.com/Wox-launcher/Wox
- Plugin Examples: https://github.com/Wox-launcher/Wox.Plugin.Python
"""

from typing import List

from .api import ChatStreamCallback, PublicAPI
from .models.ai import (
    AIModel,
    ChatStreamData,
    ChatStreamDataType,
    Conversation,
    ConversationRole,
    ToolCallInfo,
)
from .models.context import Context
from .models.image import WoxImage, WoxImageType
from .models.log import LogLevel
from .models.mru import MRUData, MRURestoreCallback
from .models.preview import WoxPreview, WoxPreviewScrollPosition, WoxPreviewType
from .models.query import (
    ChangeQueryParam,
    MetadataCommand,
    Query,
    QueryEnv,
    QueryType,
    RefreshQueryParam,
    Selection,
    SelectionType,
    CopyParams,
    CopyType,
)
from .models.result import (
    ActionContext,
    FormActionContext,
    Result,
    ResultAction,
    ResultActionType,
    ResultTail,
    ResultTailType,
    UpdatableResult,
)
from .models.setting import (
    PluginSettingDefinitionItem,
    PluginSettingDefinitionType,
    PluginSettingDefinitionValue,
    PluginSettingValueCheckBox,
    PluginSettingValueLabel,
    PluginSettingValueStyle,
    PluginSettingValueTextBox,
    create_checkbox_setting,
    create_label_setting,
    create_textbox_setting,
)
from .plugin import Plugin, PluginInitParams

__all__: List[str] = [
    # Plugin
    "Plugin",
    "PluginInitParams",
    # API
    "PublicAPI",
    "ChatStreamCallback",
    # Models
    "Context",
    "Query",
    "QueryEnv",
    "Selection",
    "Result",
    "WoxImage",
    "WoxPreview",
    "LogLevel",
    "ResultTail",
    "ResultAction",
    "ActionContext",
    "FormActionContext",
    "ResultActionType",
    "UpdatableResult",
    "MetadataCommand",
    "PluginSettingDefinitionItem",
    "PluginSettingValueStyle",
    # AI
    "AIModel",
    "ChatStreamData",
    "Conversation",
    "ConversationRole",
    "ChatStreamDataType",
    "ToolCallInfo",
    "user_message",
    "assistant_message",
    # Query
    "ChangeQueryParam",
    "RefreshQueryParam",
    "QueryType",
    "Selection",
    "SelectionType",
    "CopyParams",
    "CopyType",
    # Exceptions
    "WoxPluginError",
    "InvalidQueryError",
    "PluginInitError",
    "APIError",
    # Image
    "WoxImage",
    "WoxImageType",
    # Preview
    "WoxPreview",
    "WoxPreviewType",
    "WoxPreviewScrollPosition",
    # Result
    "ResultTailType",
    # MRU
    "MRUData",
    "MRURestoreCallback",
    # Settings
    "PluginSettingDefinitionItem",
    "PluginSettingDefinitionType",
    "PluginSettingDefinitionValue",
    "PluginSettingValueStyle",
    "PluginSettingValueTextBox",
    "PluginSettingValueCheckBox",
    "PluginSettingValueLabel",
    "create_textbox_setting",
    "create_checkbox_setting",
    "create_label_setting",
]


# Convenience functions for creating AI conversation messages
def user_message(text: str, images: List[bytes] | None = None) -> Conversation:
    """
    Create a user message for AI conversations.

    Convenience function to create a Conversation with role=USER.

    Args:
        text: The user's message text
        images: Optional list of PNG image bytes for vision models

    Returns:
        A new Conversation instance with role=USER

    Example:
        msg = user_message("What's in this image?", images=[png_data])
    """
    return Conversation.new_user_message(text, images)


def assistant_message(text: str) -> Conversation:
    """
    Create an AI message for AI conversations.

    Convenience function to create an assistant message.

    Args:
        text: The assistant's response text

    Returns:
        A new Conversation instance with role=ASSISTANT

    Example:
        msg = assistant_message("The image shows a cat sitting on a windowsill.")
    """
    return Conversation.new_assistant_message(text)


# Exception classes (stubs for documentation purposes)
#
# These exception types are mentioned in __all__ but not defined
# in the current codebase. They are documented here for reference
# and may be added in future versions.


class WoxPluginError(Exception):
    """Base exception for Wox plugin errors."""

    pass


class InvalidQueryError(WoxPluginError):
    """Raised when a query is invalid or cannot be processed."""

    pass


class PluginInitError(WoxPluginError):
    """Raised when plugin initialization fails."""

    pass


class APIError(WoxPluginError):
    """Raised when an API call fails."""

    pass
