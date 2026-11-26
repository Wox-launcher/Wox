# Wox Doctor Plugin

Doctor checks common setup problems: permissions, runtimes, update state, and platform-specific requirements.

![The Doctor plugin](/images/guide/plugin_doctor.jpg)

## Quick Start

```text
doctor
```

Follow the listed items. On Linux Wayland, Doctor also reports `input` and `uinput` permissions needed for double-modifier and CapsLock hotkeys. See the [Wayland FAQ](../../faq.md#wayland-double-modifier-hotkeys).

## Memory diagnostics

Use `doctor memory` to see how much memory Wox and its helper processes use. Open a result to drill down, or type a command directly.

- `doctor memory go` shows retained Go memory by category.
- `doctor memory native` shows native allocations by component.
- `doctor memory processes` lists plugin hosts and helper processes.
- `doctor memory profile` opens the action to save a Go heap profile.

You can select **Wox Memory** as your primary Glance to see the current memory footprint.
