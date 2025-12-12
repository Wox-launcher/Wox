# Wox Python Plugin SDK

## Installation
uv add wox-plugin

## Key Classes

### Plugin Base Class
from wox_plugin import Plugin, Query, Result, Context, PluginInitParams
from wox_plugin.models.image import WoxImage

class MyPlugin(Plugin):
    async def init(self, ctx: Context, params: PluginInitParams) -> None:
        self.api = params.api

    async def query(self, ctx: Context, query: Query) -> list[Result]:
        return []

### Query Model
class Query:
    type: str  # "input" or "selection"
    raw_query: str
    trigger_keyword: str
    command: str
    search: str

### Result Model
class Result:
    title: str
    icon: WoxImage
    sub_title: str = ""
    actions: List[ResultAction] = []
    score: float = 0.0

### WoxImage
class WoxImage:
    image_type: str  # "emoji", "url", "base64", etc.
    image_data: str

    @classmethod
    def emoji(cls, emoji: str) -> "WoxImage":
        return cls(image_type="emoji", image_data=emoji)

### API Methods
- change_query(ctx, query): Change the query
- hide_app(ctx): Hide Wox
- show_app(ctx): Show Wox
- notify(ctx, message): Show notification
- log(ctx, level, msg): Write log
- get_setting(ctx, key): Get plugin setting
- save_setting(ctx, key, value): Save plugin setting
- llm_stream(ctx, conversations, callback): Chat with LLM

## Example
from wox_plugin import Plugin, Query, Result, Context, PluginInitParams
from wox_plugin.models.image import WoxImage
from wox_plugin.models.result import ResultAction

class HelloPlugin(Plugin):
    async def init(self, ctx, params): self.api = params.api
    async def query(self, ctx, query):
        return [Result(
            title=f"Hello {query.search}",
            icon=WoxImage.emoji("👋"),
            actions=[ResultAction(id="copy", name="Copy", action=lambda c,a: None)]
        )]

plugin = HelloPlugin()
