# Query Model

Wox normalizes every user interaction into a `Query` object that is sent to plugins. Understanding how it is split helps you write predictable plugins and validation.

## Query types

- `input` – standard text input such as `wpm install wox`.
- `selection` – selection/drag-drop payloads (text/files/images). Only delivered when the plugin declares the `querySelection` feature.

## Query shape

| Field | Notes |
| --- | --- |
| `RawQuery` | Original text, or the plain-text projection of `QueryHint`, including the trigger keyword. |
| `QueryHint` | Optional ordered elements for structured `input` queries; see below. |
| `TriggerKeyword` | One of the keywords declared in `plugin.json`. `"*"` means global trigger. Empty means a global query for plugins that registered `*`. |
| `Command` | Optional command segment following the trigger keyword. Comes from `Commands` in `plugin.json`. |
| `Search` | Remainder of the query after trigger keyword + command. |
| `Selection` | When `Type=selection`, includes `Type`, `Text`, `FilePaths`. Available only with `querySelection`. |
| `Env` | Optional environment data such as active window info or browser URL. Available only with the `queryEnv` feature. |

Example split for `wpm install wox`:

- `TriggerKeyword`: `wpm`
- `Command`: `install`
- `Search`: `wox`
- `RawQuery`: `wpm install wox`

## Environment context (`queryEnv` feature)

When `Features` includes `queryEnv`, Wox will attach:

- `ActiveWindowTitle`
- `ActiveWindowPid`
- `ActiveWindowIcon` (as `WoxImage`)
- `ActiveBrowserUrl` (when the Wox Chrome extension is installed and the browser is active)

Use feature params to only request the fields you need (see [Specification](./specification.md)).

## Special query variables

Wox expands the following placeholders in user queries before sending them to plugins:

- `{wox:selected_text}`
- `{wox:active_browser_url}`
- `{wox:file_explorer_path}`

These are useful for plugins that want to seed searches from the current selection or file manager context.

## Structured queries

Use `QueryHint` for inline argument slots or atomic query blocks. It is an
optional field on an `input` query, not a new query type or markup inside text.
This feature is implemented in the current development build; do not assume a
published Wox/SDK version supports it. For distribution, verify the first release
containing it and set `MinWoxVersion` and SDK requirements accordingly. The
single-file runtime's existing version floor alone does not guarantee slot support.

`QueryHint` is semantic background guidance, not a mandatory input form. It can
carry actual argument values as well as display hints. Commands and arguments
remain one continuous editor; ordinary selection, deletion, clipboard and IME
behavior take priority. Tab is optional. If editing invalidates a semantic boundary,
keep the user's text and discard unreliable metadata rather than blocking input.
Only explicit `block` elements are atomic; a highlighted argument stays editable.

### Trigger-level templates

Hints can belong to a trigger keyword as well as a command. Static plugin metadata
can declare `"TriggerQueryHints": {"g": {"Elements": [...]}}` alongside
`"TriggerKeywords": ["g"]`. Templates contain only suffix elements; Wox supplies
the keyword and trailing space. The element ID `command` is reserved for that prefix.

At runtime, use `RegisterTriggerKeyword(ctx, {Keyword: "g", QueryHint: hint})`
in Node.js, or `register_trigger_keyword(ctx, RegisterTriggerKeywordOption("g", hint))`
in Python. Read `Success` / `success`: invalid declarations or keywords occupied
by another enabled plugin fail without changing the current registration.
Registering the same keyword again updates its hint; omitting the hint clears the
runtime template. Existing input instances retain their values.

`UnregisterTriggerKeyword(ctx, {Keyword: "g"})` and Python's
`unregister_trigger_keyword(ctx, UnregisterTriggerKeywordOption("g"))` remove
the runtime keyword and hint. Both return a result with a success flag; repeating
unregistration succeeds. Static or user-configured keywords remain unchanged.
Unloading the plugin clears all runtime registrations.

Complete command matches take precedence over trigger templates. Ambiguous
keyword ownership never supplies a hint. These APIs are development-build
capabilities; verify release support before distributing a plugin.

### Declare a suffix template

Each `Commands` entry can have `Aliases` and `QueryHint`. The template contains
only elements **after** the command. Do not include a command text element or use
the reserved ID `command` in the template; Wox inserts the matched command prefix.
For a plugin with `TriggerKeywords: ["*"]`:

```json
{
  "Command": "set-volume",
  "Description": "Set volume",
  "Aliases": ["set volume", "volume"],
  "QueryHint": {
    "Elements": [
      {
        "Id": "volume",
        "Kind": "argument",
        "Placeholder": "Volume (0–100)",
        "Required": true
      }
    ]
  }
}
```

For a plugin with trigger `gh` and command `issues`, the entry matches `gh issues`.
Aliases use the same trigger; they are complete command aliases, not argument
parsing rules. Static metadata and `RegisterQueryCommands` /
`register_query_commands` use the same template model.

| Kind | Content | Behavior |
| --- | --- | --- |
| `text` | `Text` | Editable text, including explicit separators between values. |
| `argument` | `Value`; optional `Placeholder`, `Required` | Inline editable range; clearing retains its placeholder. |
| `block` | `Value` | Atomic selection/deletion; no internal text editing. |

IDs are nonempty and unique within a structure. The list is flat; nested elements,
dropdowns, custom rendering, and `<woxblock>` markup are not supported. Placeholder
strings support the normal `i18n:` mechanism. `Required` is descriptive metadata:
the plugin must validate empty values and business constraints before exposing or
executing an action. Do not change volume or perform another action while querying.

### Read values without reparsing

Node.js, inside the plugin's query handler:

```typescript
const volume = query.QueryHint?.Elements.find(
  element => element.Id === "volume" && element.Kind === "argument"
)
const value = volume?.Kind === "argument" ? volume.Value ?? "" : ""
// Use value for structured input; keep your existing query.Search parser when
// query.QueryHint is absent. An empty slot is not a legacy query.
```

Python, inside the query handler:

```python
if query.query_hint is not None:
    value = next(
        (element.value for element in query.query_hint.elements
         if element.id == "volume" and element.kind == "argument"),
        "",
    )
else:
    value = query.search  # Feed this through the existing legacy parser.
```

Wox derives `RawQuery`, `Command`, and `Search` from the explicit query text through
the ordinary query parser. Hint content must match that text, excluding placeholders.
Text alone does not preserve parameter boundaries; never use it to reconstruct hints. Hints never carry plugin identity, force routing or create a scope. Routing follows
the ordinary query text and explicit `QueryScope`. Include the trigger keyword in
a complete instance when needed, for example `gh issues `; Wox does not infer it
from the plugin that called `ChangeQuery`.

### Open a complete instance

`ChangeQuery` always requires `QueryType` and complete `QueryText` (input) or
`QuerySelection` (selection), even when a hint is supplied. `QueryHint` is only an
optional visual enhancement, never the source of query content. Input hint elements
must match the complete `QueryText`, including trigger keyword and command prefix;
invalid or inconsistent hints are ignored and `QueryText` is used unchanged. Removing the hint must leave the same query text
and routing. Use an explicit empty `QueryText` to clear the input.
Node.js, inside a result action with `api` and `ctx` available:

```typescript
await api.ChangeQuery(ctx, {
  QueryType: "input",
  QueryText: "set volume 50",
  QueryHint: {
    Elements: [
      { Id: "command", Kind: "text", Text: "set volume " },
      { Id: "volume", Kind: "argument", Value: "50", Placeholder: "Volume (0–100)", Required: true }
    ]
  }
})
```

Python, inside a result action:

```python
from wox_plugin import ChangeQueryParam, QueryElement, QueryHint, QueryType

await self.api.change_query(ctx, ChangeQueryParam(
    query_type=QueryType.INPUT,
    query_text="set volume 50",
    query_hint=QueryHint(elements=[
        QueryElement(id="command", kind="text", text="set volume "),
        QueryElement(id="volume", kind="argument", value="50",
                     placeholder="Volume (0–100)", required=True),
    ]),
))
```

Use these APIs in SDK or single-file SDK plugins. Do not assume the limited script
plugin `change-query` action supports the same structured payload.

### Interaction and compatibility

- Matching a complete, unambiguous command previews hints; space or Tab activates
  the template. Tab / Shift+Tab select element ranges, skipping text separators.
- Command and arguments share one continuous editor. Empty placeholders always
  become ghost chips, including a single variable, so the slot reads as a hole
  to fill. A single filled argument stays unmarked. Multiple filled arguments
  use the same quiet backgrounds as blocks, and chips stay separate so values
  or names that contain spaces remain distinct. Placeholders are decorative and
  never appear in copied text or argument values.
- Character and word deletion retain ordinary text behavior. Cross-element selection
  is allowed. Edits across boundaries keep the text and discard unreliable metadata.
- Reopening with select-all selects the entire query; typing replaces the command
  and arguments. Undo and history retain structure. Blocks remain atomic.
- Pasting a full command with arguments into ordinary text does not parse slots.
  Paste within an argument updates its value.
- Without `QueryHint`, existing text queries continue unchanged. If both
  a hint and text are passed to `ChangeQuery`, the text is authoritative and the hint must match it.

Validate a new integration with blank/valid/invalid values, multiple-slot focus,
whole-query replacement after reopen, undo, and the legacy text path. Set Volume
is Wox's first built-in example; reuse the interface, not its volume-specific rules.
