from dataclasses import dataclass
from enum import Enum
from typing import Dict, List, Optional, Protocol, Union, Callable, Any, TypedDict, Literal, Awaitable
import uuid

# Basic types
MapString = Dict[str, str]
Platform = Literal["windows", "darwin", "linux"]

# Context
class Context(TypedDict):
    Values: Dict[str, str]

    # get traceId from context
    def get_trace_id(self) -> str:
        return self["Values"]["traceId"]

def new_context() -> Context:
    return {"Values": {"traceId": str(uuid.uuid4())}}

def new_context_with_value(key: str, value: str) -> Context:
    ctx = new_context()
    ctx["Values"][key] = value
    return ctx

# Selection
class SelectionType(str, Enum):
    TEXT = "text"
    FILE = "file"

@dataclass
class Selection:
    Type: SelectionType
    Text: Optional[str] = None
    FilePaths: Optional[List[str]] = None

    def to_dict(self):
        return {
            "Type": self.Type,
            "Text": self.Text,
            "FilePaths": self.FilePaths
        }

    def __dict__(self):
        return self.to_dict()

# Query Environment
@dataclass
class QueryEnv:
    """
    Active window title when user query
    """
    ActiveWindowTitle: str
    
    """
    Active window pid when user query, 0 if not available
    """
    ActiveWindowPid: int

    """
    active browser url when user query
    Only available when active window is browser and https://github.com/Wox-launcher/Wox.Chrome.Extension is installed
    """
    ActiveBrowserUrl: str

    def to_dict(self):
        return {
            "ActiveWindowTitle": self.ActiveWindowTitle,
            "ActiveWindowPid": self.ActiveWindowPid,
            "ActiveBrowserUrl": self.ActiveBrowserUrl
        }

    def __dict__(self):
        return self.to_dict()

# Query
class QueryType(str, Enum):
    INPUT = "input"
    SELECTION = "selection"

@dataclass
class Query:
    Type: QueryType
    RawQuery: str
    TriggerKeyword: Optional[str]
    Command: Optional[str]
    Search: str
    Selection: Selection
    Env: QueryEnv

    def to_dict(self):
        return {
            "Type": self.Type,
            "RawQuery": self.RawQuery,
            "TriggerKeyword": self.TriggerKeyword,
            "Command": self.Command,
            "Search": self.Search,
            "Selection": self.Selection.to_dict(),
            "Env": self.Env.to_dict()
        }

    def __dict__(self):
        return self.to_dict()

    def is_global_query(self) -> bool:
        return self.Type == QueryType.INPUT and not self.TriggerKeyword

# Result
class WoxImageType(str, Enum):
    ABSOLUTE = "absolute"
    RELATIVE = "relative"
    BASE64 = "base64"
    SVG = "svg"
    URL = "url"
    EMOJI = "emoji"
    LOTTIE = "lottie"

@dataclass
class WoxImage:
    ImageType: WoxImageType
    ImageData: str

    def to_dict(self):
        return {
            "ImageType": self.ImageType,
            "ImageData": self.ImageData
        }

    def __dict__(self):
        return self.to_dict()

def new_base64_wox_image(image_data: str) -> WoxImage:
    return WoxImage(ImageType=WoxImageType.BASE64, ImageData=image_data)

class WoxPreviewType(str, Enum):
    MARKDOWN = "markdown"
    TEXT = "text"
    IMAGE = "image"
    URL = "url"
    FILE = "file"

@dataclass
class WoxPreview:
    PreviewType: WoxPreviewType
    PreviewData: str
    PreviewProperties: Dict[str, str]

    def to_dict(self):
        return {
            "PreviewType": self.PreviewType,
            "PreviewData": self.PreviewData,
            "PreviewProperties": self.PreviewProperties
        }

    def __dict__(self):
        return self.to_dict()

class ResultTailType(str, Enum):
    TEXT = "text"
    IMAGE = "image"

@dataclass
class ResultTail:
    Type: ResultTailType
    Text: Optional[str] = None
    Image: Optional[WoxImage] = None

    def to_dict(self):
        return {
            "Type": self.Type,
            "Text": self.Text,
            "Image": self.Image.to_dict() if self.Image else None
        }

    def __dict__(self):
        return self.to_dict()

@dataclass
class ActionContext:
    ContextData: str

@dataclass
class ResultAction:
    Name: str
    Action: Callable[[ActionContext], Awaitable[None]]
    Id: Optional[str] = None
    Icon: Optional[WoxImage] = None
    IsDefault: Optional[bool] = None
    PreventHideAfterAction: Optional[bool] = None
    Hotkey: Optional[str] = None

    def to_dict(self):
        return {
            "Name": self.Name,
            "Id": self.Id,
            "Icon": self.Icon.to_dict() if self.Icon else None,
            "IsDefault": self.IsDefault,
            "PreventHideAfterAction": self.PreventHideAfterAction,
            "Hotkey": self.Hotkey
        }

    def __dict__(self):
        return self.to_dict()

@dataclass
class Result:
    Title: str
    Icon: WoxImage
    Id: Optional[str] = None
    SubTitle: Optional[str] = None
    Preview: Optional[WoxPreview] = None
    Score: Optional[float] = None
    Group: Optional[str] = None
    GroupScore: Optional[float] = None
    Tails: Optional[List[ResultTail]] = None
    ContextData: Optional[str] = None
    Actions: Optional[List[ResultAction]] = None
    RefreshInterval: Optional[int] = None
    OnRefresh: Optional[Callable[["RefreshableResult"], "RefreshableResult"]] = None

    def to_dict(self):
        return {
            "Title": self.Title,
            "Icon": self.Icon.to_dict(),
            "Id": self.Id,
            "SubTitle": self.SubTitle,
            "Preview": self.Preview.to_dict() if self.Preview else None,
            "Score": self.Score,
            "Group": self.Group,
            "GroupScore": self.GroupScore,
            "Tails": [tail.to_dict() for tail in self.Tails] if self.Tails else None,
            "ContextData": self.ContextData,
            "Actions": [action.to_dict() for action in self.Actions] if self.Actions else None,
            "RefreshInterval": self.RefreshInterval
        }
 
    def __dict__(self):
        return self.to_dict()

@dataclass
class RefreshableResult:
    Title: str
    SubTitle: str
    Icon: WoxImage
    Preview: WoxPreview
    Tails: List[ResultTail]
    ContextData: str
    RefreshInterval: int
    Actions: List[ResultAction]

    def to_dict(self):
        return {
            "Title": self.Title,
            "SubTitle": self.SubTitle,
            "Icon": self.Icon.to_dict(),
            "Preview": self.Preview.to_dict(),
            "Tails": [tail.to_dict() for tail in self.Tails],
            "ContextData": self.ContextData,
            "RefreshInterval": self.RefreshInterval,
            "Actions": [action.to_dict() for action in self.Actions]
        }

    def __dict__(self):
        return self.to_dict()

# Plugin API
@dataclass
class ChangeQueryParam:
    QueryType: QueryType
    QueryText: Optional[str] = None
    QuerySelection: Optional[Selection] = None

    def to_dict(self):
        return {
            "QueryType": self.QueryType,
            "QueryText": self.QueryText,
            "QuerySelection": self.QuerySelection.to_dict() if self.QuerySelection else None
        }

    def __dict__(self):
        return self.to_dict()

# AI
class ConversationRole(str, Enum):
    USER = "user"
    SYSTEM = "system"

class ChatStreamDataType(str, Enum):
    STREAMING = "streaming"
    FINISHED = "finished"
    ERROR = "error"

@dataclass
class Conversation:
    Role: ConversationRole
    Text: str
    Timestamp: int

    def to_dict(self):
        return {
            "Role": self.Role,
            "Text": self.Text,
            "Timestamp": self.Timestamp
        }

    def __dict__(self):
        return self.to_dict()

ChatStreamFunc = Callable[[ChatStreamDataType, str], None]

# Settings
class PluginSettingDefinitionType(str, Enum):
    HEAD = "head"
    TEXTBOX = "textbox"
    CHECKBOX = "checkbox"
    SELECT = "select"
    LABEL = "label"
    NEWLINE = "newline"
    TABLE = "table"
    DYNAMIC = "dynamic"

@dataclass
class PluginSettingValueStyle:
    PaddingLeft: int
    PaddingTop: int
    PaddingRight: int
    PaddingBottom: int
    Width: int
    LabelWidth: int

@dataclass
class PluginSettingDefinitionValue:
    def get_key(self) -> str:
        raise NotImplementedError

    def get_default_value(self) -> str:
        raise NotImplementedError

    def translate(self, translator: Callable[[Context, str], str]) -> None:
        raise NotImplementedError

@dataclass
class PluginSettingDefinitionItem:
    Type: PluginSettingDefinitionType
    Value: PluginSettingDefinitionValue
    DisabledInPlatforms: List[Platform]
    IsPlatformSpecific: bool

@dataclass
class MetadataCommand:
    Command: str
    Description: str

# Plugin Interface
class Plugin(Protocol):
    async def init(self, ctx: Context, init_params: "PluginInitParams") -> None:
        ...

    async def query(self, ctx: Context, query: Query) -> List[Result]:
        ...

# Public API Interface
class PublicAPI(Protocol):
    async def change_query(self, ctx: Context, query: ChangeQueryParam) -> None:
        ...

    async def hide_app(self, ctx: Context) -> None:
        ...

    async def show_app(self, ctx: Context) -> None:
        ...

    async def notify(self, ctx: Context, message: str) -> None:
        ...

    async def log(self, ctx: Context, level: str, msg: str) -> None:
        ...

    async def get_translation(self, ctx: Context, key: str) -> str:
        ...

    async def get_setting(self, ctx: Context, key: str) -> str:
        ...

    async def save_setting(self, ctx: Context, key: str, value: str, is_platform_specific: bool) -> None:
        ...

    async def on_setting_changed(self, ctx: Context, callback: Callable[[str, str], None]) -> None:
        ...

    async def on_get_dynamic_setting(self, ctx: Context, callback: Callable[[str], PluginSettingDefinitionItem]) -> None:
        ...

    async def on_deep_link(self, ctx: Context, callback: Callable[[MapString], None]) -> None:
        ...

    async def on_unload(self, ctx: Context, callback: Callable[[], None]) -> None:
        ...

    async def register_query_commands(self, ctx: Context, commands: List[MetadataCommand]) -> None:
        ...

    async def llm_stream(self, ctx: Context, conversations: List[Conversation], callback: ChatStreamFunc) -> None:
        ...

@dataclass
class PluginInitParams:
    API: PublicAPI
    PluginDirectory: str 