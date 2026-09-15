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

## Saving Settings

`set_setting()` requires Wox >= 2.4.0 and accepts a `SetSettingOption`. Set
`is_local=True` when a value must stay on the current device and remain outside
Cloud Sync. The older `save_setting()` method remains available for plugins
targeting Wox releases before 2.4.0, but is deprecated for new integrations.

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

## Static HTML preview

Use `WoxPreviewType.WEBVIEW` with a JSON-encoded `html` field. No HTTP server or temporary HTML file is needed; there is no separate `html` preview type.

```python
import json
from wox_plugin import WoxPreview, WoxPreviewType

preview = WoxPreview(
    preview_type=WoxPreviewType.WEBVIEW,
    preview_data=json.dumps({
        "html": '<!doctype html><html><body><h1 style="color:teal">Hello Wox</h1></body></html>'
    }),
)
# Assign preview to Result(preview=preview, ...).
```

Set either `html` or `url`. Optional JSON fields are `injectCss`, `userAgent`, `cacheDisabled`, and `cacheKey` (defaults to the URL or HTML). Inline HTML has no plugin-relative base URL: embed CSS/images or use absolute resource URLs. This is browser content, not sanitized Markdown; use `html.escape` for untrusted text before interpolation.

## Structured queries

`QueryHint` and `QueryElement` are exported by `wox_plugin`. Declare a suffix
template in `MetadataCommand.query_hint`, read
`query.query_hint.elements`, and use `ChangeQueryParam(query_type=QueryType.INPUT,
query_hint=...)` for a complete instance. When structure is absent, retain
the existing `query.search` parsing path.
See the [query model and Python examples](../www/docs/development/plugins/query-model.md#structured-queries).
This is a development-build capability; verify release and SDK support before
setting a distributable plugin's minimum versions.

Arguments can offer ordered suggestions without restricting free text:

```python
hint = QueryHint(elements=[
    QueryElement("filter", "argument", suggestions=["created", "assigned", "search"]),
])
# Use as MetadataCommand("issues", "Issues", query_hint=hint),
# or in RegisterTriggerKeywordOption("gh", hint).
```

An empty slot shows `created / assigned / search`. Typing `cr` shows the `eated`
suffix and a Tab mark. Tab inserts only that suffix, leaving the caret at the end;
another Tab uses ordinary argument navigation. Matching ignores case, picks the
first prefix match, and stops at an exact match. Unmatched input stays editable.
Suggestions are literal input values and are not translated.

For metadata with `TriggerKeywords: ["gh"]` and commands `issues` and `prs`,
Wox automatically shows `issues / prs` after `gh ` when there is no explicit
trigger Query Hint. Runtime commands participate; global `*` triggers do not.
Empty previews show at most three complete candidates and an ellipsis, shrinking
to fit the available width. Automatic command previews prefer shorter names;
completion still searches the full list in declaration order. Tab completion of
an automatic command appends a space and activates that command's own Query Hint.
Ordinary argument suggestions do not append a space.
Existing registration and ChangeQuery APIs carry suggestions; omitted suggestions
preserve ordinary placeholders. The JSON field is `Suggestions`.

To disable automatic command hints, add `{ "Name": "disableAutoCommandHint" }`
to the plugin metadata's `Features` array. Explicit Query Hints remain available.
