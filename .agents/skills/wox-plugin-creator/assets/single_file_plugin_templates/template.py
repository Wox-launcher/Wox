# {
#   "Id": "{{.PluginID}}",
#   "Name": "{{.Name}}",
#   "Author": "{{.Author}}",
#   "Version": "1.0.0",
#   "MinWoxVersion": "{{.MinWoxVersion}}",
#   "Runtime": "{{.Runtime}}",
#   "Description": "{{.Description}}",
#   "Icon": "svg:<svg xmlns='http://www.w3.org/2000/svg' width='48' height='48' viewBox='0 0 48 48'><rect width='48' height='48' rx='12' fill='#2563eb'/><path d='M14 11h14l7 7v19H14z' fill='#eff6ff'/><path d='M28 11v7h7' fill='#bfdbfe'/><path d='M19 24h11M19 29h11M19 34h7' fill='none' stroke='#2563eb' stroke-width='2.5' stroke-linecap='round'/></svg>",
#   "TriggerKeywords": {{.TriggerKeywordsJSON}},
#   "SupportedOS": ["Windows", "Linux", "Macos"]
# }

"""
Wox Python Single-file SDK Plugin Template

This file is loaded by the shared Python runtime host. Query and action
calls reuse that host process; they do not start a new interpreter.

Python can import wox_plugin directly. Register OnUnload if you create
timers, watchers, or sockets so reload can clean them up.

Single-file plugins cannot ship extra files, pip dependencies, or
relative image paths. Use a packaged .wox SDK plugin for those.
"""

from wox_plugin import Context, PluginInitParams, Query, QueryResponse, Result, WoxImage

RESULT_ICON = WoxImage.new_svg(
    "<svg xmlns='http://www.w3.org/2000/svg' width='48' height='48' viewBox='0 0 48 48'>"
    "<rect width='48' height='48' rx='12' fill='#2563eb'/>"
    "<path d='M14 11h14l7 7v19H14z' fill='#eff6ff'/>"
    "<path d='M28 11v7h7' fill='#bfdbfe'/>"
    "<path d='M19 24h11M19 29h11M19 34h7' fill='none' stroke='#2563eb' stroke-width='2.5' stroke-linecap='round'/>"
    "</svg>"
)


class MyPlugin:
    async def init(self, ctx: Context, params: PluginInitParams) -> None:
        self.api = params.api

    async def query(self, ctx: Context, query: Query) -> QueryResponse:
        return QueryResponse(
            results=[
                Result(
                    title="{{.Name}}",
                    sub_title=query.search or "Single-file Python SDK plugin",
                    icon=RESULT_ICON,
                )
            ]
        )


plugin = MyPlugin()
