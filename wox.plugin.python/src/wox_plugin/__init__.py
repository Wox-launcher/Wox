"""
Wox Plugin SDK for Python

This package provides the SDK for developing Wox plugins in Python.
"""

from typing import List

from .plugin import Plugin, PluginInitParams
from .api import PublicAPI, ChatStreamCallback
from .models.context import Context
from .models.query import (
    Query,
    QueryEnv,
    Selection,
    ChangeQueryParam,
    QueryType,
    SelectionType,
    MetadataCommand,
)
from .models.result import (
    Result,
    ResultTail,
    ResultAction,
    ActionContext,
    RefreshableResult,
    ResultTailType,
)

from .models.ai import (
    AIModel,
    Conversation,
    ConversationRole,
    ChatStreamDataType,
)
from .models.image import WoxImage, WoxImageType
from .models.preview import WoxPreview, WoxPreviewType, WoxPreviewScrollPosition
from .models.mru import MRUData, MRURestoreCallback
from .models.setting import (
    PluginSettingDefinitionItem,
    PluginSettingDefinitionType,
    PluginSettingDefinitionValue,
    PluginSettingValueStyle,
    PluginSettingValueTextBox,
    PluginSettingValueCheckBox,
    PluginSettingValueLabel,
    create_textbox_setting,
    create_checkbox_setting,
    create_label_setting,
)

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
    "ResultTail",
    "ResultAction",
    "ActionContext",
    "RefreshableResult",
    "MetadataCommand",
    "PluginSettingDefinitionItem",
    "PluginSettingValueStyle",
    # AI
    "AIModel",
    "Conversation",
    "ConversationRole",
    "ChatStreamDataType",
    "user_message",
    "ai_message",
    # Query
    "ChangeQueryParam",
    "QueryType",
    "Selection",
    "SelectionType",
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
