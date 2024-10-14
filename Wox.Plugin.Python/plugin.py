from typing import List, Dict, Any, Optional, Callable, Union
from abc import ABC, abstractmethod
from enum import Enum

class Platform(Enum):
    WINDOWS = "windows"
    DARWIN = "darwin"
    LINUX = "linux"

class SelectionType(Enum):
    TEXT = "text"
    FILE = "file"

class Selection:
    def __init__(self, type: SelectionType, text: Optional[str] = None, file_paths: Optional[List[str]] = None):
        self.Type = type
        self.Text = text
        self.FilePaths = file_paths

class QueryEnv:
    def __init__(self, active_window_title: str):
        self.ActiveWindowTitle = active_window_title

class Query:
    def __init__(self, type: str, raw_query: str, trigger_keyword: Optional[str], command: Optional[str], search: str, selection: Optional[Selection], env: QueryEnv):
        self.Type = type
        self.RawQuery = raw_query
        self.TriggerKeyword = trigger_keyword
        self.Command = command
        self.Search = search
        self.Selection = selection
        self.Env = env

    def is_global_query(self) -> bool:
        return self.TriggerKeyword is None or self.TriggerKeyword == ""

class WoxImageType(Enum):
    ABSOLUTE = "absolute"
    RELATIVE = "relative"
    BASE64 = "base64"
    SVG = "svg"
    URL = "url"
    EMOJI = "emoji"
    LOTTIE = "lottie"

class WoxImage:
    def __init__(self, image_type: WoxImageType, image_data: str):
        self.ImageType = image_type
        self.ImageData = image_data

class WoxPreviewType(Enum):
    MARKDOWN = "markdown"
    TEXT = "text"
    IMAGE = "image"
    URL = "url"
    FILE = "file"

class WoxPreview:
    def __init__(self, preview_type: WoxPreviewType, preview_data: str, preview_properties: Dict[str, str]):
        self.PreviewType = preview_type
        self.PreviewData = preview_data
        self.PreviewProperties = preview_properties

class ResultTail:
    def __init__(self, type: str, text: Optional[str] = None, image: Optional[WoxImage] = None):
        self.Type = type
        self.Text = text
        self.Image = image

class ActionContext:
    def __init__(self, context_data: str):
        self.ContextData = context_data

class ResultAction:
    def __init__(self, id: Optional[str], name: str, icon: Optional[WoxImage], is_default: bool, prevent_hide_after_action: bool, action: Callable[[ActionContext], None], hotkey: Optional[str]):
        self.Id = id
        self.Name = name
        self.Icon = icon
        self.IsDefault = is_default
        self.PreventHideAfterAction = prevent_hide_after_action
        self.Action = action
        self.Hotkey = hotkey

class Result:
    def __init__(self, id: Optional[str], title: str, sub_title: Optional[str], icon: WoxImage, preview: Optional[WoxPreview], score: Optional[float], group: Optional[str], group_score: Optional[float], tails: Optional[List[ResultTail]], context_data: Optional[str], actions: Optional[List[ResultAction]], refresh_interval: Optional[int], on_refresh: Optional[Callable[['RefreshableResult'], 'RefreshableResult']]):
        self.Id = id
        self.Title = title
        self.SubTitle = sub_title
        self.Icon = icon
        self.Preview = preview
        self.Score = score
        self.Group = group
        self.GroupScore = group_score
        self.Tails = tails
        self.ContextData = context_data
        self.Actions = actions
        self.RefreshInterval = refresh_interval
        self.OnRefresh = on_refresh

class RefreshableResult:
    def __init__(self, title: str, sub_title: str, icon: WoxImage, preview: WoxPreview, context_data: str, refresh_interval: int):
        self.Title = title
        self.SubTitle = sub_title
        self.Icon = icon
        self.Preview = preview
        self.ContextData = context_data
        self.RefreshInterval = refresh_interval

class ChangeQueryParam:
    def __init__(self, query_type: str, query_text: Optional[str] = None, query_selection: Optional[Selection] = None):
        self.QueryType = query_type
        self.QueryText = query_text
        self.QuerySelection = query_selection

class Context:
    def __init__(self):
        self.Values = {}

    def get(self, key: str) -> Optional[str]:
        return self.Values.get(key)

    def set(self, key: str, value: str):
        self.Values[key] = value

    def exists(self, key: str) -> bool:
        return key in self.Values

class PublicAPI:
    @staticmethod
    async def change_query(ctx: Context, query: ChangeQueryParam):
        pass

    @staticmethod
    async def hide_app(ctx: Context):
        pass

    @staticmethod
    async def show_app(ctx: Context):
        pass

    @staticmethod
    async def notify(ctx: Context, title: str, description: Optional[str] = None):
        pass

    @staticmethod
    async def log(ctx: Context, level: str, msg: str):
        pass

    @staticmethod
    async def get_translation(ctx: Context, key: str) -> str:
        pass

    @staticmethod
    async def get_setting(ctx: Context, key: str) -> str:
        pass

    @staticmethod
    async def save_setting(ctx: Context, key: str, value: str, is_platform_specific: bool):
        pass

    @staticmethod
    async def on_setting_changed(ctx: Context, callback: Callable[[str, str], None]):
        pass

class PluginInitParams:
    def __init__(self, api: PublicAPI, plugin_directory: str):
        self.API = api
        self.PluginDirectory = plugin_directory

class WoxPlugin(ABC):
    @abstractmethod
    async def init(self, ctx: Context, init_params: PluginInitParams):
        pass

    @abstractmethod
    async def query(self, ctx: Context, query: Query) -> List[Result]:
        pass

def new_context() -> Context:
    return Context()

def new_context_with_value(key: str, value: str) -> Context:
    ctx = Context()
    ctx.set(key, value)
    return ctx

def new_base64_wox_image(image_data: str) -> WoxImage:
    return WoxImage(WoxImageType.BASE64, image_data)
