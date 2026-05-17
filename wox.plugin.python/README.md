# Wox Plugin Python

This package provides type definitions for developing Wox plugins in Python.

## Requirements

- Python >= 3.8 (defined in `pyproject.toml`)
- Python 3.12 recommended for development (defined in `.python-version`)

## Installation

```bash
# Using pip
pip install wox-plugin

# Using uv (recommended)
uv add wox-plugin
```

## Usage

This example returns `QueryResponse`, so the plugin's `plugin.json` should set
`MinWoxVersion` to `2.0.4` or newer. Return `list[Result]` directly if you need
the same plugin build to run on older Wox releases.

```python
from wox_plugin import Query, QueryResponse, Result, Context, PluginInitParams, WoxImage

class MyPlugin:
    async def init(self, ctx: Context, params: PluginInitParams) -> None:
        self.api = params.API
        
    async def query(self, ctx: Context, query: Query) -> QueryResponse:
        # Your plugin logic here
        results = []
        results.append(
            Result(
                title="Hello Wox",
                sub_title="This is a sample result",
                icon=WoxImage.new_emoji("🔍"),
                score=100
            )
        )
        return QueryResponse(results=results)

# MUST HAVE! The plugin class will be automatically loaded by Wox
plugin = MyPlugin()
```

Returning `list[Result]` directly is deprecated. The Python host still accepts
it for compatibility with older Wox releases. Use `QueryResponse` only when
`plugin.json` declares `MinWoxVersion` >= `2.0.4` so results, refinements, and
layout hints are carried together.

When a plugin needs to control the preview width or grid layout, set
`QueryResponse.layout.result_preview_width_ratio` or
`QueryResponse.layout.grid_layout`. The older `resultPreviewWidthRatio` and
`gridLayout` metadata features are deprecated because they can only describe
static plugin or command defaults.

## Query Requirements

Plugins can declare settings that must be configured before Wox calls `query()`:

```json
{
  "QueryRequirements": {
    "AnyQuery": [
      {
        "SettingKey": "apiKey",
        "Validators": [{ "Type": "not_empty" }],
        "Message": "i18n:my_plugin_api_key_required"
      }
    ],
    "QueryWithoutCommand": [],
    "QueryWithCommand": {
      "download": [
        {
          "SettingKey": "downloadPath",
          "Validators": [{ "Type": "not_empty" }]
        }
      ]
    }
  }
}
```

## License

MIT 
