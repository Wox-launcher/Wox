# {
#   "Id": "{{.PluginID}}",
#   "Name": "{{.Name}}",
#   "Author": "{{.Author}}",
#   "Version": "1.0.0",
#   "MinWoxVersion": "{{.MinWoxVersion}}",
#   "Runtime": "{{.Runtime}}",
#   "Description": "{{.Description}}",
#   "Icon": "emoji:🐍",
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


class MyPlugin:
    async def init(self, ctx: Context, params: PluginInitParams) -> None:
        self.api = params.api

    async def query(self, ctx: Context, query: Query) -> QueryResponse:
        return QueryResponse(
            results=[
                Result(
                    title="{{.Name}}",
                    sub_title=query.search or "Single-file Python SDK plugin",
                    icon=WoxImage.new_emoji("🐍"),
                )
            ]
        )


plugin = MyPlugin()
