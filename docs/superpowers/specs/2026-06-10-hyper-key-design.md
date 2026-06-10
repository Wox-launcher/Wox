# Hyper Key Design

## Problem
Wox already supports main hotkey, selection hotkey, query hotkeys, and double-modifier hotkeys, but users still need conflict-free shortcut space for Wox-specific commands. Raycast's Hyper Key shows a useful model: one physical key can act as a dedicated shortcut modifier. Wox should support the useful part without adding broad remapping complexity.

## Approved Approach
Add a Wox Hyper Key mode that treats `Caps Lock + key` as a Wox-only hotkey combination. The physical Hyper Key is fixed to Caps Lock, and the stored hotkey form should preserve user intent as `hyper+key` instead of flattening it into platform modifiers.

At trigger time, Wox matches `hyper+key` through its own raw Caps Lock dispatcher:

- Windows/Linux: `Ctrl + Alt + Win + key`
- macOS: `Ctrl + Option + Command + key`

The UI should display Hyper Key shortcuts with a stable `✦` label. The storage and matching behavior matter more than decorative display for the first implementation.

## Boundaries
- Hyper Key is always Caps Lock.
- No setting for choosing another physical key.
- No setting for including or excluding Shift.
- No single-tap behavior. Pressing and releasing Caps Lock alone does nothing in Wox.
- No system-wide remapping for other applications.
- Only Wox-owned hotkeys are in scope: main hotkey, selection hotkey, and query hotkeys.
- Existing normal hotkeys and double-modifier hotkeys must continue to work.

## Architecture
- Settings: add one boolean setting, such as `EnableHyperKey`.
- Core hotkey layer: add Hyper Key parsing and matching for persisted `hyper+key` values.
- Native keyboard layer: use the existing raw-key listener path to detect Caps Lock state and consume Hyper Key combinations when they match Wox registrations.
- UI recorder: when Hyper Key mode is enabled and the recorder sees Caps Lock plus an allowed key, record `hyper+key`.
- UI display: render persisted `hyper+key` consistently in settings, result tails, and toolbar hotkey labels.
- Availability checks: compare `hyper+key` against other Wox-owned Caps Lock based hotkeys to avoid duplicate registrations.

## Expected Result
Users can enable Hyper Key and assign shortcuts such as `hyper+k` to Wox actions. Pressing Caps Lock by itself has no Wox behavior, while pressing Caps Lock with an allowed key triggers the configured Wox hotkey without exposing a general-purpose keyboard remapper.
