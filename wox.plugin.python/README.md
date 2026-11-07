# Wox Plugin Python

This package provides type definitions for developing Wox plugins in Python.

Plugin-owned windows are available through `api.create_window(ctx, option)`.
The returned instance has `show`, `hide`, `close`, `set_options`, `content.set`,
`content.update` and `content.command`, all with named Option/Result models.
See the [WoxView window API guide](../wox.core/ui/pluginwindow/README.md).

## WoxView protocol types

Import `WoxViewTree`, `WoxViewNode`, `WoxViewHandler` and `WoxViewEventData` from
`wox_plugin`. Component props and other types live in `wox_plugin.models.woxview`.
They describe the same JSON wire format as the Node.js SDK, using protocol keys
such as `onPress` and `$handler`. See the [shared SDK guide](../wox.core/ui/woxview/README.md#sdk-wire-types)
for examples, event dispatch, single-file plugins and type generation.

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

Query debounce can be declared in `plugin.json` at plugin, trigger and command
levels. Command `DebounceMs` overrides `TriggerDebounceMs[keyword]`, which overrides
the `debounce` feature's `IntervalMs`. Omit a value to inherit, or use `0` to run
immediately. For runtime registration, pass `debounce_ms` to
`RegisterTriggerKeywordOption` or `MetadataCommand`; `None` inherits the next level.
Trigger/command overrides apply to input queries only.
See the [metadata example and precedence rules](../wox.core/plugin/README.md#query-scheduling).

Use `WoxPreview(..., default_hidden=True)` to keep a result's preview collapsed initially.
Users can open it with the row icon or toggle it with Ctrl/Cmd+P. The manual choice
applies to all results in the current query, including live updates. A new query restores
each result's default; before manual input, each result uses its own default.
The default is `False`. Global search does not expose plugin previews, including manually opened previews.

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

## Notifications

For transient notifications on Wox 2.5.0 or newer, use `show_toast_msg()` with a required message and an optional title and action. The action receives its context data and runs at most once. The older `notify()` method remains available and preserves its original routing.

```python
from wox_plugin import ShowToastMsgOption, ToastMsgAction

await self.api.show_toast_msg(ctx, ShowToastMsgOption(
    message="The file is ready",
    title="Transfer complete",
    action=ToastMsgAction(name="Open", action=open_file, context_data={"path": path}),
))
```

## Saving Settings

`set_setting()` requires Wox >= 2.4.0 and accepts a `SetSettingOption`. Set
`is_local=True` when a value must stay on the current device and remain outside
Cloud Sync. The older `save_setting()` method remains available for plugins
targeting Wox releases before 2.4.0, but is deprecated for new integrations.

## Plugin Tools

Plugin Tools are schema-checked operations that plugins register at runtime. They require Wox >= 2.4.5.

```python
from wox_plugin import (
    PluginToolAnnotations,
    PluginToolDescriptor,
    RegisterPluginToolOption,
    InvokePluginToolHandlerResult,
    InvokePluginToolOption,
)

await self.api.register_plugin_tool(ctx, RegisterPluginToolOption(
    tool=PluginToolDescriptor(
        name="echo_text",
        description="Echo the supplied text",
        input_schema={"type": "object", "properties": {"text": {"type": "string"}}, "required": ["text"]},
        output_schema={"type": "object", "properties": {"text": {"type": "string"}}, "required": ["text"]},
        annotations=PluginToolAnnotations(read_only=True, idempotent=True),
    ),
    handler=lambda _ctx, option: InvokePluginToolHandlerResult(output={"text": option.arguments["text"]}),
))

created = await self.api.invoke_plugin_tool(ctx, InvokePluginToolOption(
    plugin_id=notes_plugin_id,
    name="create_note",
    arguments={"title": "Roadmap", "text": "Ship it"},
))
if created.error:
    await self.api.notify(ctx, created.error.message)
    return
opened = await self.api.invoke_plugin_tool(ctx, InvokePluginToolOption(
    plugin_id=notes_plugin_id,
    name="open_note",
    arguments={"noteId": created.output.get("noteId")},
))
```

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

## Action toolbar visibility

Only the default Enter action appears in the toolbar automatically. Set
`show_in_toolbar=True` on an execute or form `ResultAction` to also show it in
the toolbar, with or without a `hotkey`. The default is `False`. All actions
remain available in the Action Panel and their keyboard shortcuts continue to work.

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

### Result icon container

Set `icon_show_container=True` on a result to show its icon inside a rounded background
and border that adapt to the theme and selection state. The default is `false`;
the image keeps its aspect ratio.

### Updating an inline HTML preview without navigation

`UpdateResult` also accepts a one-shot WebView instruction:
`{"updateMode":"script","script":"window.updateProgress({count: 1})"}`.
The method signature and preview type are unchanged. Do not combine this instruction
with `html` or `url`, or return it from `query()`. It can target only the current
result's plugin-owned inline HTML document; URL previews and noncurrent results
return `false`. Scope the call with the query context that produced the result.

`true` means accepted, not successfully executed. Wox queues accepted instructions
in order until that document is ready. Switching results, replacing the document,
or destroying the preview discards its pending instructions. JavaScript exceptions
are logged without replacing the page. Wox does not return script values or await
Promises. Instructions are neither cached nor replayed by redraws; other fields in
the same update, such as Actions, retain their normal behavior.

Return complete HTML from each query, using the latest task state. Define an updater
in that HTML and change existing DOM nodes inside it so scrolling, selection, and
animation survive. Serialize state as JSON rather than concatenating raw output into
JavaScript. When embedding initial JSON in a `<script>` element, also escape `<`
as `\u003c` to prevent a literal `</script>` from ending the element.

This capability requires a Wox host containing script-mode preview updates; older
hosts do not support it. No SDK publication is required to transport these JSON keys.

```python
from wox_plugin import UpdatableResult

# query() returns HTML defining window.updateProgress, initialized with current state.
# Later, update only the current document. Keep ctx from that query.
state = {"count": 1, "message": "Scanning…"}
accepted = await api.update_result(ctx, UpdatableResult(
    id=result_id,
    preview=WoxPreview(
        preview_type=WoxPreviewType.WEBVIEW,
        preview_data=json.dumps({
            "updateMode": "script",
            "script": "window.updateProgress(" + json.dumps(state) + ");",
        }),
    ),
))
```

## Declarative global hotkeys (development)

Declare `Type: "hotkey"`, `Scope: "global"` in static plugin settings, then subscribe
with `OnHotkey` (`on_hotkey` in Python) during init. Wox manages registration, conflicts
and cleanup. See the [contract and examples](../wox.core/plugin/hotkey.md) for gesture
kinds, paired hold events, stable table IDs and asynchronous callback handling.

### Removing a result

```python
from wox_plugin import RemoveResultOption

removed = await api.remove_result(ctx, RemoveResultOption(
    result_id=action_context.result_id, selected_result_id=next_result_id
))
```

Use the original query context and a result owned by your plugin. `selected_result_id` is optional; if it is absent or unavailable, Wox preserves the current selection or selects the nearest remaining row. `removed.success` indicates whether the result was visible and removed.

## Interactive WoxView windows

Create a view once and mount it through the existing window API. Each `as_window()` call creates an independent session; keep business state in the plugin and call `invalidate()` when it changes. Wox retains native interaction state such as focus and text editing. Supply stable `key` values for nodes that can move in a list.

```python
from wox_plugin import Button, Column, Text, WindowOptions, WindowPresentation, create_wox_view

state = {"count": 0}
errors = []

def render(session):
    def add():
        state["count"] += 1
        session.invalidate()
    return Column(children=[
        Text(text=f"Count: {state['count']}"),
        Button(key="add", label="Add", on_press=add),
    ])

view = create_wox_view(render, errors.append)
result = await api.create_window(ctx, view.as_window(WindowOptions(
    title="Counter", presentation=WindowPresentation.WINDOW, width=360, height=240,
)))
```

Single-file Python plugins use the same `wox_plugin` imports supplied by their host. `wox_view_node(type, props, children, key)` supports all schema-defined components, while `Column`, `Text`, `Button`, and `Loading` are convenience builders. Props passed to the generic builder use WoxView JSON names such as `onChange`; callback arguments follow the WoxView event contract.

Line charts accept optional `chart` props for axes, grid, area fill, and a floating hover tooltip; see the [line chart options](../wox.core/ui/woxview/README.md#line-charts). Omitting them preserves the original chart appearance. WoxView also supports [bar, scatter, and pie charts](../wox.core/ui/woxview/README.md#other-chart-types).

Import `Loading` from `wox_plugin` and use `Loading()` for an animated ring, or customize it with `Loading(style={"width": 24, "color": "$accent"})`. Render it while work is pending and invalidate the session when the work completes. The default diameter is 16 logical units and the default color is `$text`.

The adapter owns the returned `content` and `on_event`. It serializes updates, coalesces invalidations while an update is pending, and releases handlers when the window closes or the plugin unloads. Errors go to `on_error`. This requires matching SDK and host versions. For an interactive result preview, use `view.as_preview()` as the result's `preview`; the host binds the session to that result, forwards events, and publishes `session.invalidate()` through `update_result`. Call `as_preview()` separately for each result. The session is released when its result is replaced or its query is released. Set `ResultPreviewWidthRatio` to 0 in the query layout to show the preview at full width.

Use preview lifecycle hooks to stop work when the preview is hidden:

```python
task = None

def show(hidden):
    global task
    task = asyncio.create_task(refresh_quotes_until_hidden(hidden))

def hide():
    global task
    if task is not None:
        task.cancel()
        task = None

preview = view.as_preview(on_show=show, on_hide=hide, on_dispose=hide)
```

`on_show` receives a new `asyncio.Event` each time the preview becomes visible;
it is set before `on_hide`. Disposal runs `on_hide` first if still visible, then
`on_dispose` once. Hooks should return promptly and arrange cancellation of
their own tasks; Wox cannot stop arbitrary plugin work. A delayed async hook
should check the event before starting work and after awaiting. Results removed,
query caches released, and plugin unloads dispose the session.
