Due to the GO's cyclic reference limitation, some common type definitions are defined in this package

## Theme variants

Theme overrides resolve before schema defaults: root → operating system → matching
OS/desktop variant → available capabilities nested under that variant.
Windows variants are `win10` and `win11`; Linux desktop variants are `hyprland`,
`kde`, and `gnome`. Unknown desktops use the shared `linux` fields.

```json
"linux": {
  "AppBorderRadius": 8,
  "variants": {
    "kde": {
      "backgroundBlur": {
        "AppBackgroundColor": "#16161A84",
        "AppBorderRadius": 8
      }
    },
    "hyprland": {
      "backgroundBlur": {
        "AppBackgroundColor": "#16161AA0",
        "AppBorderRadius": 8
      }
    }
  }
}
```

The capability sits directly under the desktop (`linux.variants.kde.backgroundBlur`),
without another `variants` wrapper. `backgroundBlur` is selected only when the native runtime exposes backdrop material.
On Linux this requires `ext-background-effect-v1` with the blur capability; a theme
cannot force protocol support. Desktop siblings never inherit one another's
capability styles. Omitted fields inherit; JSON `null` clears an inherited field
and restores its schema default. Inactive nested variants are validated and kept
when editing, saving, packaging, and syncing a theme.

Linux blur follows the authored rounded outline in surface-local logical units.
Without blur support, translucent window washes become opaque. Fully transparent
image-theme windows opt out of blur so their clear margins remain clear. Windows
and macOS retain their existing custom-outline material behavior.


## Toolbar backdrop controls (schema v2, Wox 2.4.5+)

`ToolbarBlurSigma` controls the local toolbar blur in logical units (0–64, default
12). Zero removes spatial blur; brightness and saturation can still adjust the
sampled pixels. `ToolbarBlurBrightness` and `ToolbarBlurSaturation` accept 0–2 and
default to 1. Brightness 0 produces black RGB; saturation 0 produces grayscale.
These multipliers adjust the platform's existing material result, preserving its
alpha. `ToolbarBackgroundColor` remains a separate tint and can be fully transparent.
Opaque tint hides the backdrop and skips its processing. Renderer fallbacks without
backdrop sampling retain the existing tint-only behavior.

All three controls accept decimals and the same platform/desktop overrides as
other v2 fields. Omission inherits; `null` restores the native default. Example:

```json
"linux": {
  "variants": {
    "kde": {
      "backgroundBlur": {
        "ToolbarBackgroundColor": "#00000000",
        "ToolbarBlurSigma": 4,
        "ToolbarBlurBrightness": 0.8,
        "ToolbarBlurSaturation": 0.5
      }
    }
  }
}
```

These settings affect Wox's local toolbar material, not the desktop compositor's
whole-window blur. The editor and launcher preview use the same controls. The
internal `Container.Floating` flag remains necessary: ordinary containers and the
content inside an already-blurred rounded toolbar must not add another blur pass.

Capability overrides and toolbar backdrop controls require Wox 2.4.5. The theme
editor raises `MinWoxVersion` on drafts using these features, including inactive
platform overrides, while preserving higher requirements and the loaded source.
