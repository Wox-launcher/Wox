# Portable input

This package owns semantic input events, local shortcut adaptation, Unicode
grapheme editing, selection, composition and undo history. It depends on shared
geometry and `util/hotkey` syntax, not native windows or their event loops.

Native adapters translate platform events into these types. Widgets consume the
same types through the runtime facade. Tests remain here with editing state.
