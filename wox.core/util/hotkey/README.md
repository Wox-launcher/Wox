# Hotkey ownership

This package owns shortcut syntax, modifier aliases, persisted press/hold
bindings, global registration, special trigger policies, and recording sessions.
`keyboard.KeyName` owns canonical key names independently of native key-code
support. `ParseCombo` and the global registration parser share token parsing.

`ParseCombo` exposes a normal key name and modifier bits for local UI matching.
It preserves local navigation and printable Unicode keys without adding them to
native global registration backends. CapsLock combinations, double modifiers,
hold triggers, and side-specific modifier chords remain native capabilities.
UI adapters translate the parsed names and bits into their own event types;
this package does not import UI or retain its windows.
