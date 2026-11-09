# Query Refinements

Return query-scoped controls such as type filters or sort modes on `QueryResponse.Refinements`. Wox renders the controls and sends the selected values back on the next query as `Query.Refinements` / `query.refinements`. The plugin owns the filtering or sorting semantics.

Wox shows `singleSelect` and `multiSelect` as searchable dropdowns. Options can supply `Keywords` / `keywords` for alternative names. Searching the dropdown is local to Wox and leaves the query text untouched. While the filters bar is visible, a group's shortcut opens its dropdown. With the bar hidden, it cycles single-select values or toggles all multi-select values, as before.

The bookmark at the end of the filters bar saves or removes the current command's default selections. Wox stores the snapshot as a synced plugin setting and supplies it through `Query.Refinements` on re-entry. Temporary edits stay in the current query.

Requires `MinWoxVersion` >= `2.0.4` (or `2.4.2` for single-file SDK plugins). Do not invent command syntax such as `type:file` when a refinement can express the same control.

## Required hotkey

Every refinement must supply a non-empty `Hotkey` (Python `hotkey`). Prefer `Primary+Shift+<key>` to leave common `Primary+<key>` chords available for result actions.

- On macOS, use a chord such as `cmd+shift+t`.
- On Windows and Linux, use a chord such as `ctrl+shift+t`.

Detect the OS at runtime and emit the matching chord.

```typescript
const refinementHotkey = (key: string) =>
  (process.platform === "darwin" ? "cmd+shift+" : "ctrl+shift+") + key;

{ Hotkey: refinementHotkey("t") }
```

```python
import sys

def refinement_hotkey(key: str) -> str:
    modifier = "cmd" if sys.platform == "darwin" else "ctrl"
    return f"{modifier}+shift+{key}"

QueryRefinement(..., hotkey=refinement_hotkey("t"))
```

Pick a mnemonic after checking Wox built-in hotkeys and the actions shown alongside the refinement. If the chord conflicts, choose another one. [Wox built-in hotkeys](hotkeys.md)

### Reserved shortcuts

Avoid window commands (`Primary+,`, `Primary+W`) and launcher defaults such as Action Panel (`Primary+K`, legacy `Primary+J`), filters (`Primary+F`), Attention (`Primary+U`), and preview toggle (`Primary+P`). See [hotkeys.md](hotkeys.md) for scopes, focused-editor and preview bindings, configurable shortcuts, and conflict checking. Enter and primary+Enter belong to action execution/submission, not refinement toggles.

## Read selected values

Selected values are opaque strings keyed by refinement `Id`. Multi-select values are comma-separated.

- Node.js: `query.Refinements?.["item_type"] ?? "all"`
- Python: `query.refinements.get("item_type", "all")`

## Node.js example

```typescript
const refinementHotkey = (key: string) =>
  (process.platform === "darwin" ? "cmd+shift+" : "ctrl+shift+") + key;

async query(ctx, query): Promise<QueryResponse> {
  const selectedType = query.Refinements?.["item_type"] ?? "all";
  return {
    Results: this.filterResults(selectedType),
    Refinements: [
      {
        Id: "item_type",
        Title: "Type",
        Type: "singleSelect",
        Hotkey: refinementHotkey("t"),
        DefaultValue: ["all"],
        Options: [
          { Value: "all", Title: "All" },
          { Value: "file", Title: "Files" },
          { Value: "folder", Title: "Folders" },
        ],
      },
    ],
  };
}
```

## Python example

```python
import sys
from wox_plugin import QueryRefinement, QueryRefinementOption, QueryRefinementType, QueryResponse

def refinement_hotkey(key: str) -> str:
    modifier = "cmd" if sys.platform == "darwin" else "ctrl"
    return f"{modifier}+shift+{key}"

def build_type_refinement() -> QueryRefinement:
    return QueryRefinement(
        id="item_type",
        title="Type",
        type=QueryRefinementType.SINGLE_SELECT,
        hotkey=refinement_hotkey("t"),
        default_value=["all"],
        options=[
            QueryRefinementOption(value="all", title="All"),
            QueryRefinementOption(value="file", title="Files"),
            QueryRefinementOption(value="folder", title="Folders"),
        ],
    )

async def query(self, ctx, query) -> QueryResponse:
    selected_type = query.refinements.get("item_type", "all")
    return QueryResponse(
        results=self.filter_results(selected_type),
        refinements=[build_type_refinement()],
    )
```
