"""
Wox Plugin SDK for Python

This package provides the SDK for developing Wox plugins in Python.
"""

from typing import List

from .plugin import Plugin, BasePlugin, PluginInitParams
from .api import PublicAPI
from .models.context import Context
from .models.query import Query, QueryEnv, Selection
from .models.result import (
    Result,
    WoxImage,
    WoxPreview,
    ResultTail,
    ResultAction,
    ActionContext,
    RefreshableResult,
)
from .models.settings import (
    PluginSettingDefinitionItem,
    PluginSettingDefinitionValue,
    PluginSettingValueStyle,
    MetadataCommand,
)
from .types import (
    Platform,
    SelectionType,
    QueryType,
    WoxImageType,
    WoxPreviewType,
    ResultTailType,
    ConversationRole,
    ChatStreamDataType,
    PluginSettingDefinitionType,
)
from .utils.helpers import new_base64_wox_image
from .exceptions import WoxPluginError, InvalidQueryError, PluginInitError, APIError

__version__: str = "0.1.0"
__all__: List[str] = [
    # Plugin
    "Plugin",
    "BasePlugin",
    "PluginInitParams",
    # API
    "PublicAPI",
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
    "PluginSettingDefinitionItem",
    "PluginSettingDefinitionValue",
    "PluginSettingValueStyle",
    "MetadataCommand",
    # Types
    "Platform",
    "SelectionType",
    "QueryType",
    "WoxImageType",
    "WoxPreviewType",
    "ResultTailType",
    "ConversationRole",
    "ChatStreamDataType",
    "PluginSettingDefinitionType",
    # Utils
    "new_base64_wox_image",
    # Exceptions
    "WoxPluginError",
    "InvalidQueryError",
    "PluginInitError",
    "APIError",
]
