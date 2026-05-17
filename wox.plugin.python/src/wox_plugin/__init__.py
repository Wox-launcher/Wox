"""
Wox Plugin SDK for Python

This package provides the SDK for developing Wox plugins in Python.

Wox is a powerful launcher application that allows users to quickly
search, access, and manipulate information across their system.
Plugins extend Wox's functionality by providing custom search results
and actions.

## Quick Start

To create a plugin, implement the `Plugin` protocol:

The example below returns `QueryResponse`, so its `plugin.json` should declare
`MinWoxVersion` >= `2.0.4`. Return `List[Result]` directly when supporting older
Wox releases.

```python
from wox_plugin import Plugin, PluginInitParams, Context, Query, QueryResponse, Result
from wox_plugin import WoxImage, QueryType, LogLevel

class MyPlugin:
    async def init(self, ctx: Context, params: PluginInitParams) -> None:
        # Store API for later use
        self.api = params.api

        # Log initialization
        await self.api.log(ctx, LogLevel.INFO, "MyPlugin initialized")

    async def query(self, ctx: Context, query: Query) -> QueryResponse:
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

        return QueryResponse(results=results)
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
- **Screenshot**: `screenshot()`

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
- `ScreenshotOption`: Options for the screenshot workflow
- `ScreenshotResult`: Result returned by the screenshot workflow

#### Result Models (`models/result.py`)
- `Result`: Search result with title, icon, preview, actions
- `ResultAction`: User action on a result
- `ResultActionType`: EXECUTE (immediate) or FORM (show form)
- `ActionContext`: Context passed to action callbacks
- `FormActionContext`: Context for form submissions
- `ResultTail`: Additional visual elements (text or image)
- `UpdatableResult`: Result that can be updated in UI
- `QueryResponse`: Structured query response with results, refinements, and layout hints (requires Wox >= 2.0.4)

#### Image Models (`models/image.py`)
- `WoxImage`: Image model with multiple types
- `WoxImageType`: ABSOLUTE, RELATIVE, BASE64, SVG, LOTTIE, EMOJI, URL, THEME, FILE_ICON
- Factory methods: `new_base64()`, `new_svg()`, `new_emoji()`, etc.

#### Preview Models (`models/preview.py`)
- `WoxPreview`: Preview content for results
- `WoxPreviewListData`: Structured data for list previews
- `WoxPreviewListItem`: Row data for list previews
- `WoxPreviewType`: MARKDOWN, TEXT, IMAGE, URL, FILE, LIST, REMOTE
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
    "Icon": "https://example.com/icon.png",
    "QueryRequirements": {
        "AnyQuery": [
            {
                "SettingKey": "apiKey",
                "Validators": [{"Type": "not_empty"}],
                "Message": "i18n:my_plugin_api_key_required"
            }
        ],
        "QueryWithoutCommand": [],
        "QueryWithCommand": {
            "download": [
                {
                    "SettingKey": "downloadPath",
                    "Validators": [{"Type": "not_empty"}]
                }
            ]
        }
    }
}
```

## Query Flow

1. User triggers Wox and types trigger keyword (e.g., "my query")
2. Wox calls `plugin.query()` with:
   - `query.trigger_keyword = "my"`
   - `query.command = ""`
   - `query.search = "query"`
3. Plugin returns `QueryResponse` when `MinWoxVersion` is at least `2.0.4`
4. Wox displays results sorted by score

Returning `List[Result]` directly is deprecated. The Python host still accepts it
for compatibility with older Wox releases. Use `QueryResponse` only when
`plugin.json` declares `MinWoxVersion` >= `2.0.4` so results, refinements, and
layout hints are carried together.

For preview width and grid presentation, prefer `QueryResponse.layout` over the
deprecated `resultPreviewWidthRatio` and `gridLayout` metadata features. The
metadata features remain compatible, but they are static defaults rather than
query-scoped layout decisions.

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

from .api import ChatStreamCallback, PublicAPI, ScreenshotOption, ScreenshotResult
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
from .models.preview import WoxPreview, WoxPreviewListData, WoxPreviewListItem, WoxPreviewScrollPosition, WoxPreviewType
from .models.query import (
    ChangeQueryParam,
    CopyParams,
    CopyType,
    MetadataCommand,
    Query,
    QueryEnv,
    QueryType,
    RefreshQueryParam,
    Selection,
    SelectionType,
)
from .models.query_response import (
    QueryGridLayout,
    QueryLayout,
    QueryRefinement,
    QueryRefinementOption,
    QueryRefinementType,
    QueryResponse,
)
from .models.result import (
    ActionContext,
    FormActionContext,
    Result,
    ResultAction,
    ResultActionType,
    ResultTail,
    ResultTailTextCategory,
    ResultTailType,
    UpdatableResult,
)
from .models.setting import (
    PluginQueryRequirement,
    PluginQueryRequirements,
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
from .models.toolbar_msg import (
    ToolbarMsg,
    ToolbarMsgAction,
    ToolbarMsgActionContext,
)
from .plugin import Plugin, PluginInitParams, QueryReturn

__all__: List[str] = [
    # Plugin
    "Plugin",
    "PluginInitParams",
    "QueryReturn",
    # API
    "PublicAPI",
    "ChatStreamCallback",
    "ScreenshotOption",
    "ScreenshotResult",
    # Models
    "Context",
    "Query",
    "QueryResponse",
    "QueryRefinement",
    "QueryRefinementOption",
    "QueryRefinementType",
    "QueryLayout",
    "QueryGridLayout",
    "QueryEnv",
    "Selection",
    "Result",
    "WoxImage",
    "WoxPreview",
    "WoxPreviewListData",
    "WoxPreviewListItem",
    "LogLevel",
    "ResultTail",
    "ResultAction",
    "ActionContext",
    "FormActionContext",
    "ResultActionType",
    "UpdatableResult",
    "ToolbarMsg",
    "ToolbarMsgAction",
    "ToolbarMsgActionContext",
    "MetadataCommand",
    "PluginSettingDefinitionItem",
    "PluginQueryRequirement",
    "PluginQueryRequirements",
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
    "WoxPreviewListData",
    "WoxPreviewListItem",
    "WoxPreviewType",
    "WoxPreviewScrollPosition",
    # Result
    "ResultTailTextCategory",
    "ResultTailType",
    # MRU
    "MRUData",
    "MRURestoreCallback",
    # Settings
    "PluginSettingDefinitionItem",
    "PluginQueryRequirement",
    "PluginQueryRequirements",
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
