# Plugin Tools

Named, schema-checked operations that SDK and single-file SDK plugins register at runtime. Wox owns the catalog, validation, routing, and lifecycle. The handler owns business logic.

Requires **Wox >= 2.4.5**. Set `MinWoxVersion` to `"2.4.5"` or newer before calling these APIs. Script plugins cannot register or invoke tools. Search-box subcommands (`Commands` / `RegisterQueryCommands`) are independent.

Do not declare tools in `plugin.json`. Registration is the only source.

## When to register

Register tools in `init()` at the same time as query and action features. Other plugins can then invoke the same capabilities without parsing this plugin's query syntax.

Do this by default for work with a clear input and output: create, update, search, import, control. Keep query results and actions for the user; extract the operation into a tool and have the action call it or share the handler.

Split data tools from UI tools. Data work must not open windows, change the query, steal focus, or notify. UI work (open a window, change the query, show Chat) is a separate tool with `RequiresUI: true`.

Skip a tool only when the feature is a launcher-only result row with no reusable operation.

List and invoke only see tools from plugins that finished init, are enabled, and still have the tool registered.

In AI Chat, `@` is a typed mention picker. Plugins are one kind. Selecting a plugin inserts `{plugin:PluginId}` and displays its localized name; that plugin's tools become callable for the rest of the conversation. Tools are not dumped into every chat; `@` is the consent gate. Do not declare tools in `plugin.json`.

## Identity

Unique key is `(PluginId, Name)`. `PluginId` is the registering instance; a plugin cannot register tools for another plugin.

`Name` is lowercase `snake_case`: ASCII letters, digits, underscores, starting with a letter, at most 64 characters. No version field. Breaking changes register a new name such as `create_note_v2`.

## API

Node.js uses PascalCase. Python uses snake_case. Same option/result shapes.

| Node.js | Python | Purpose |
| --- | --- | --- |
| `RegisterPluginTool` | `register_plugin_tool` | Publish a tool owned by this plugin |
| `UnregisterPluginTool` | `unregister_plugin_tool` | Remove one of this plugin's tools. Repeating succeeds |
| `ListPluginTools` | `list_plugin_tools` | Snapshot of callable tools, optional `PluginId` filter |
| `InvokePluginTool` | `invoke_plugin_tool` | Validate arguments, run the handler, validate output |

Duplicate names return `TOOL_ALREADY_REGISTERED`. They do not replace the handler.

## Descriptor

| Field | Required | Meaning |
| --- | --- | --- |
| `Name` | yes | Stable name inside the plugin |
| `Description` | yes | Purpose, limits, side effects. Use `i18n:` keys for user-visible text; they resolve when listing |
| `InputSchema` | yes | JSON Schema 2020-12 for an object |
| `OutputSchema` | yes | JSON Schema 2020-12 for a success object |
| `Annotations.ReadOnly` | yes | Does not change business or external state |
| `Annotations.Destructive` | yes | May make a destructive change |
| `Annotations.Idempotent` | yes | Repeating the same input has no extra effect |
| `Annotations.RequiresUI` | yes | Opens, changes, or requires UI |

Annotations are hints, not permissions. Do not put functions, callback IDs, or instance pointers in the descriptor.

## JSON Schema

- Draft 2020-12, compiled at registration and reused on invoke.
- Supported: `object`, `array`, `string`, `number`, `integer`, `boolean`, `null`, plus `properties`, `required`, `additionalProperties`, `items`, `enum`, length/numeric bounds, `anyOf`, in-document `$defs` / `$ref`.
- `$ref` must stay in the document. Network and filesystem refs fail registration.
- Top-level input and output must be objects. Omitted `Arguments` become `{}`.
- `default` is documentation only. Handlers apply defaults themselves.
- Input rejects unknown fields unless the schema allows them. Success output may include extra fields.
- Schema max 64 KiB, depth 16. Arguments max 1 MiB.
- Schema checks do not replace business checks. Unsupported keywords fail registration.

## Success and errors

`Output` and `Error` are mutually exclusive. No `Success` or `Handled` flag. Branch on `Code`. Use `Message` only for logs and notifications.

| Code | Meaning |
| --- | --- |
| `INVALID_REGISTRATION` | Name, description, schema, or handler is invalid |
| `TOOL_ALREADY_REGISTERED` | Same plugin already registered this name |
| `TOOL_NOT_FOUND` | Missing or unregistered |
| `PLUGIN_UNAVAILABLE` | Disabled, failed init, unloading, or host down |
| `INVALID_ARGUMENTS` | Input does not match `InputSchema` |
| `PERMISSION_DENIED` | Call context refused the invoke, including recursive re-entry of the same tool |
| `CANCELLED` | Caller cancelled |
| `TIMEOUT` | Deadline reached. Default 30s, or the caller's earlier deadline |
| `EXECUTION_FAILED` | Business failure, host error, or panic |
| `INVALID_OUTPUT` | Handler claimed success but output failed `OutputSchema` |

Document plugin-specific codes such as `NOTE_NOT_FOUND`. Host disconnects still throw, matching other RPC methods. Timeout, transport failure, and `INVALID_OUTPUT` can happen after side effects. Wox does not retry.

Go handlers must check `ctx`. Cancelling a wait does not force-stop Python or Node.js work already running.

Go handlers run synchronously and must cooperate with cancellation; Wox waits for them to exit before releasing plugin resources. Both hosts also drain active tool handlers before unload callbacks run. Nested tool calls must reuse the supplied context so Wox can preserve the parent deadline and reject recursive re-entry across hosts.

## UI convention

Data tools return data only. UI tools encapsulate window and query details. If create succeeds and open fails, keep the id. Do not create again.

```text
create_note({title, text, path}) → {noteId}
open_note({noteId}) → {}
```

## Built-in tools

| Plugin | Name | UI |
| --- | --- | --- |
| Notes | `create_note` | no |
| Notes | `open_note` | yes |
| Folder | `browse_path` | yes |
| Shell | `open_at_directory` | yes |
| Chat | `open_chat_with_attachments` | yes |
| File Search | `search` | no |
| Media Player | `get_status`, `play`, `pause`, `toggle`, `next`, `previous` | no |

`get_status` returns `{status}`: `none`, `playing`, `paused`, `stopped`, or `unknown`. Control tools only send commands; callers that must restore playback should read status first.

## Node.js

```typescript
await this.api.RegisterPluginTool(ctx, {
  Tool: {
    Name: "echo_text",
    Description: "Echo the supplied text",
    InputSchema: {
      type: "object",
      properties: { text: { type: "string" } },
      required: ["text"]
    },
    OutputSchema: {
      type: "object",
      properties: { text: { type: "string" } },
      required: ["text"]
    },
    Annotations: { ReadOnly: true, Destructive: false, Idempotent: true, RequiresUI: false }
  },
  Handler: async (_ctx, option) => ({ Output: { text: option.Arguments.text } })
})

const created = await this.api.InvokePluginTool(ctx, {
  PluginId: notesPluginId,
  Name: "create_note",
  Arguments: { title: "Roadmap", text: "Ship it" }
})
if (created.Error) {
  await this.api.Notify(ctx, created.Error.Message)
  return
}
const opened = await this.api.InvokePluginTool(ctx, {
  PluginId: notesPluginId,
  Name: "open_note",
  Arguments: { noteId: created.Output?.noteId }
})
```

## Python

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
