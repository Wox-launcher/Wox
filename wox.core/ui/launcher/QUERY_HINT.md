# Query Hint

## Design principle: continuous input comes first

Query Hint annotates one continuous native text editor. It does not supply query
text or route queries. `text` elements describe fixed spans, `argument` elements
hold freely editable values, and `block` elements are atomic. Invalid annotations
are discarded without rejecting the user's input. Selection, clipboard, undo and
IME use the actual document, never the painted guidance.

## Suggestions

An argument's optional `Suggestions` list supplies literal, ordered candidate
values. An empty argument displays `created / assigned / search` in the existing
quiet placeholder chip. A nonempty prefix displays the first case-insensitive
match's remaining suffix and the shared Tab glyph. An exact match suppresses
completion even when later candidates extend it. No match leaves input unchanged.

Empty previews contain at most three whole candidates, with ` / …` indicating
more. Reduce the count until the label fits the available logical width. Automatic
command previews sort by Unicode character count, preserving declaration order
for ties; matching always retains the original complete candidate list.

The shared `queryHintSuggestion` decision requires focus, a collapsed selection,
no composition, and the caret at the argument end. The view and key handler use
the same decision. Tab accepts just the suffix as one undoable document edit;
the next Tab navigates arguments normally. Shift+Tab retains backward navigation.

Core-generated command suggestions are the exception: accepting one also appends
a context space and resolves its parameter template, all in one undo step.
The internal `CommandSuggestions` marker survives clones but is not plugin JSON;
ordinary argument suggestions never receive this command-specific behavior.

When there is exactly one command candidate, an empty command value already
offers the entire command as plain ghost text with Tab, without a placeholder
chip. Accepting it still appends a space. Multiple commands and ordinary argument
suggestions require a nonempty prefix before completion is offered.

Paint-only insertion space allows an earlier argument to complete while later
arguments contain text. The view translates pointer positions across that space
before ordinary text hit testing. It never changes the native value, caret or
IME anchor. All measurements are logical units, with the existing viewport clip.
Boundary props contain every paint dependency, including insertion geometry.

## Template resolution

The core first resolves complete commands, then explicit trigger hints. If a
non-global trigger has no explicit hint, its current static and runtime Commands
provide unique primary names in declaration order. Aliases keep their existing
parser behavior. Conflicting trigger owners and global `*` inputs are excluded.

The `disableAutoCommandHint` metadata feature opts a plugin out of generated candidates only.
Explicit trigger and command templates still take precedence. Clipboard opts out;
there is no plugin-specific condition in the shared resolver.

While a trigger hint is active, entering a complete command plus a space can
replace it with a more specific command template. Preserve typed whitespace and
selection through this transition. Existing query generations retain isolated
copies, including suggestion arrays. No persisted-data migration is needed.

See the [plugin query model](../../../www/docs/development/plugins/query-model.md#candidate-suggestions)
for declarations and compatibility, and the focused Query Hint unit and native
smoke tests for editing and paint invariants.
