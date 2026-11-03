# Declarative plugin hotkeys

Plugins declare settings and subscribe once during initialization. Wox owns native
registration, conflicts, settings reconciliation and cleanup. This contract is available
in development Core/hosts and SDK sources; it is not implied by an older release number.

```json
{
  "Type": "hotkey",
  "IsPlatformSpecific": true,
  "Value": {
    "Key": "recordHotkey",
    "Label": "Record",
    "DefaultValue": "",
    "Scope": "global",
    "AllowedHotkeyKinds": ["normalCombo", "doubleModifier", "capsLockCombo", "pressModifier", "holdModifier"]
  }
}
```

Empty values are unbound. Empty/omitted `Scope`, or `local`, preserves the existing
recorder-only behavior; it does not install a global listener. Omitted allowed kinds
permits all five kinds for global settings. Platform support is still required.
Global declarations must be in static metadata, not returned by dynamic settings.

```typescript
const subscribed = await api.OnHotkey(ctx, {
  Callback: async (ctx, event) => {
    if (event.SettingKey !== "recordHotkey") return
    // Route started/ended by ActivationID. Keep cleanup in OnUnload as well.
    await recorder.handle(event)
  }
})
if (!subscribed.Success) throw new Error(subscribed.ErrMsg)
```

```python
from wox_plugin import OnHotkeyOption

async def hotkey(ctx, event):
    if event.setting_key == "recordHotkey":
        await recorder.handle(event)

result = await api.on_hotkey(ctx, OnHotkeyOption(callback=hotkey))
if not result.success:
    raise RuntimeError(result.err_msg)
```

Go uses `OnHotkey(ctx context.Context, option OnHotkeyOption) OnHotkeyResult` with
`Callback func(context.Context, HotkeyEvent)`. Go handlers must return promptly; move
long-running work into the plugin's own cancellable session. Node/Python hosts dispatch
async callbacks without awaiting completion, so release can arrive while start awaits
model loading. Plugins must tolerate overlapping asynchronous handlers.

| Kind | Example stored value | Phase |
| --- | --- | --- |
| normalCombo | `ctrl+alt+d`, `win+e` | triggered |
| doubleModifier | `ctrl+ctrl` | triggered |
| capsLockCombo | `capslock+d` | triggered |
| pressModifier | `right_alt` | triggered |
| holdModifier | `hold:right_alt` | started, ended |

System shortcuts such as Win+E use normalCombo; availability and interception depend on
the platform. Wox does not swallow unrelated system shortcuts.

Each activation has an opaque `ActivationID` and Unix millisecond `Timestamp`. A hold
starts after the existing 200 ms threshold; early release produces no activation.
Its ended event shares the same ID. `EndReason` is `released`, `interrupted`,
`bindingChanged`, `unavailable`, or `disabled`. Unload/crash closes delivery, so a final
ended callback is not guaranteed there: always clean up in OnUnload too.
The event includes `SettingKey`, `Kind`, `Phase`, and the effective `Hotkey` string.
`Env` uses existing queryEnv feature filters and is captured before delivering start;
the paired end reuses that snapshot. It is context, not a text insertion capability.

For a table, mark the relevant `hotkey` column with `Scope: "global"` and optional
`AllowedHotkeyKinds`. The table may declare `HotkeyIdentityField` (default `id`) and
`HotkeyDisabledField` (optional boolean field). Bound rows require unique nonempty string
IDs. Events include `ItemID` and `ColumnKey`; array indexes are never identities. Wox
generates IDs when the user creates/edits rows, and generates a new ID when cloning.
Programmatic table writes must supply IDs. Disabled rows are not registered.

One subscription is allowed per init cycle. Registration happens after Init succeeds;
an invalid initial binding fails initialization visibly. `SetSetting` validates/registers
before persistence and returns an error on conflicts; failed changes keep the old
binding. Re-registration may interrupt an in-progress hold. Externally persisted changes
(Cloud Sync) that cannot register disable stale bindings and log the error; a later valid
value can recover without a new subscription. Unload, disable and runtime failure remove
owned bindings and reject late callbacks. No imperative register/unregister API is needed.

All operation signatures use named Option/Result contracts. New metadata and event
fields can be added with backward-compatible defaults without changing those signatures.
