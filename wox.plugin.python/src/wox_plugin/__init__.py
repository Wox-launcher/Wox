"""
Wox Plugin SDK for Python

This package provides the SDK for developing Wox plugins in Python.
"""

from typing import List

from .api import ChatStreamCallback, PublicAPI
from .models.ai import (
    AIModel,
    ChatStreamDataType,
    Conversation,
    ConversationRole,
)
from .models.context import Context
from .models.image import WoxImage, WoxImageType
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
)
from .models.result import (
    ActionContext,
    Result,
    ResultAction,
    ResultTail,
    ResultTailType,
    UpdatableResult,
    UpdatableResultAction,
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
    "ResultTail",
    "ResultAction",
    "ActionContext",
    "UpdatableResult",
    "UpdatableResultAction",
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
    "RefreshQueryParam",
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
