# {{.Name}} - Python Plugin Scaffold

## plugin.json

```json
{
  "Id": "{{.PluginID}}",
  "Name": "{{.Name}}",
  "Description": "{{.Description}}",
  "Version": "1.0.0",
  "MinWoxVersion": "2.0.0",
  "Runtime": "PYTHON",
  "Entry": "main.py",
  "Icon": "emoji:🚀",
  "TriggerKeywords": {{.TriggerKeywordsJSON}},
  "SupportedOS": ["Windows", "Linux", "Macos"]
}
```

## main.py

```python
from wox_plugin import Plugin, Query, Result, Context, PluginInitParams, PublicAPI
from wox_plugin.models.image import WoxImage
from wox_plugin.models.result import ResultAction

class {{.PascalName}}Plugin(Plugin):
    api: PublicAPI

    async def init(self, ctx: Context, params: PluginInitParams) -> None:
        self.api = params.api

    async def query(self, ctx: Context, query: Query) -> list[Result]:
        async def execute_action(ctx: Context, action_context) -> None:
            await self.api.notify(ctx, "Action executed!")

        results = []

        # Example result with action
        results.append(Result(
            title=f"Echo: {query.search}" if query.search else "{{.Name}} Ready",
            sub_title="Type keywords to search or select this item",
            icon=WoxImage.emoji("🚀"),
            actions=[
                ResultAction(
                    id="action_echo",
                    name="Show Notification",
                    is_default=True,
                    action=execute_action,
                )
            ],
        ))

        return results

plugin = {{.PascalName}}Plugin()
```

## pyproject.toml

```toml
[project]
name = "{{.KebabName}}"
version = "1.0.0"
dependencies = ["wox-plugin"]

[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"
```

## Setup Steps

1. `uv venv`
2. `uv pip install -e .`
