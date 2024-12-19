from enum import StrEnum
from typing import Dict, Callable

# Basic types
MapString = Dict[str, str]


class Platform(StrEnum):
    WINDOWS = "windows"
    DARWIN = "darwin"
    LINUX = "linux"


class SelectionType(StrEnum):
    TEXT = "text"
    FILE = "file"


class QueryType(StrEnum):
    INPUT = "input"
    SELECTION = "selection"


class WoxImageType(StrEnum):
    ABSOLUTE = "absolute"
    RELATIVE = "relative"
    BASE64 = "base64"
    SVG = "svg"
    URL = "url"
    EMOJI = "emoji"
    LOTTIE = "lottie"


class WoxPreviewType(StrEnum):
    MARKDOWN = "markdown"
    TEXT = "text"
    IMAGE = "image"
    URL = "url"
    FILE = "file"


class ResultTailType(StrEnum):
    TEXT = "text"
    IMAGE = "image"


class ConversationRole(StrEnum):
    USER = "user"
    SYSTEM = "system"


class ChatStreamDataType(StrEnum):
    STREAMING = "streaming"
    FINISHED = "finished"
    ERROR = "error"


class PluginSettingDefinitionType(StrEnum):
    HEAD = "head"
    TEXTBOX = "textbox"
    CHECKBOX = "checkbox"
    SELECT = "select"
    LABEL = "label"
    NEWLINE = "newline"
    TABLE = "table"
    DYNAMIC = "dynamic"


# Type aliases
ChatStreamFunc = Callable[[ChatStreamDataType, str], None]
