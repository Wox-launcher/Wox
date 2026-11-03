package plugin

import (
	"context"
	"errors"
	"testing"
	"time"
	corehotkey "wox/hotkey"
	"wox/setting/definition"
	utilhotkey "wox/util/hotkey"
)

type hotkeySettingsPlugin struct {
	Plugin
	values map[string]string
}

func (p *hotkeySettingsPlugin) GetSettingOverride(_ context.Context, key string) (string, bool) {
	return p.values[key], true
}

// TestDeclarativeHotkeyLifecycle covers init admission, ordered hold events, rollback and teardown.
func TestDeclarativeHotkeyLifecycle(t *testing.T) {
	ctx := t.Context()
	p := &hotkeySettingsPlugin{values: map[string]string{"record": "hold:right_alt"}}
	instance := &Instance{Plugin: p, Metadata: Metadata{Id: "test-hotkeys", SettingDefinitions: []definition.PluginSettingDefinitionItem{{Type: definition.PluginSettingDefinitionTypeHotkey, Value: &definition.PluginSettingValueHotkey{Key: "record", Scope: definition.HotkeyScopeGlobal}}}}}
	a := NewAPI(instance).(*APIImpl)
	instance.API = a
	hotkeyCapability.RLock()
	previous := hotkeyCapability.apply
	hotkeyCapability.RUnlock()
	t.Cleanup(func() { SetHotkeyBindingApplier(previous) })
	var entries []corehotkey.Entry
	fail := false
	applications := 0
	SetHotkeyBindingApplier(func(_ context.Context, owner string, next []corehotkey.Entry, persist func() error) error {
		applications++
		if owner != "test-hotkeys" {
			t.Fatalf("owner=%s", owner)
		}
		if fail && len(next) > 0 {
			return errors.New("conflict")
		}
		if persist != nil {
			if err := persist(); err != nil {
				return err
			}
		}
		entries = next
		return nil
	})
	events := make(chan HotkeyEvent, 8)
	option := OnHotkeyOption{Callback: func(_ context.Context, e HotkeyEvent) { events <- e }}
	if r := a.OnHotkey(ctx, option); !r.Success {
		t.Fatal(r)
	}
	if len(entries) != 0 {
		t.Fatal("registered before init finished")
	}
	if a.OnHotkey(ctx, option).Success {
		t.Fatal("duplicate subscription accepted")
	}
	if err := a.ActivateHotkeys(ctx); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("bindings=%d", len(entries))
	}
	old := entries[0]
	old.OnHoldTransition(utilhotkey.HoldTransition{Started: true})
	if err := a.RefreshHotkeys(ctx); err != nil || applications != 1 {
		t.Fatalf("unchanged settings re-registered hotkeys: applications=%d err=%v", applications, err)
	}
	old.OnHoldTransition(utilhotkey.HoldTransition{Reason: utilhotkey.HoldEndReleased})
	receive := func() HotkeyEvent {
		t.Helper()
		select {
		case e := <-events:
			return e
		case <-time.After(time.Second):
			t.Fatal("missing event")
			return HotkeyEvent{}
		}
	}
	start, end := receive(), receive()
	if start.Phase != HotkeyPhaseStarted || end.Phase != HotkeyPhaseEnded || start.ActivationID == "" || start.ActivationID != end.ActivationID || end.EndReason != HotkeyEndReleased {
		t.Fatalf("bad pair: %+v %+v", start, end)
	}
	fail = true
	persisted := false
	if err := a.applyHotkeys(ctx, "record", "ctrl+alt+d", func() error { persisted = true; return nil }); err == nil || persisted {
		t.Fatal("failed change persisted")
	}
	// A synced invalid value must disable stale bindings without losing the subscriber.
	p.values["record"] = "ctrl+alt+d"
	if a.RefreshHotkeys(ctx) == nil || len(entries) != 0 {
		t.Fatal("failed sync kept stale binding")
	}
	fail = false
	if err := a.RefreshHotkeys(ctx); err != nil || len(entries) != 1 {
		t.Fatalf("sync recovery: %v", err)
	}
	entries[0].OnPress()
	if e := receive(); e.Kind != HotkeyKindNormalCombo || e.Phase != HotkeyPhaseTriggered {
		t.Fatal(e)
	}
	a.stopHotkeys()
	if len(entries) != 0 || a.OnHotkey(ctx, option).Success {
		t.Fatal("teardown did not close admission")
	}
	old.OnHoldTransition(utilhotkey.HoldTransition{Started: true})
	select {
	case e := <-events:
		t.Fatalf("late event: %+v", e)
	case <-time.After(10 * time.Millisecond):
	}
}

// TestGlobalHotkeyTableIdentity verifies explicit opt-in and persistent row ownership.
func TestGlobalHotkeyTableIdentity(t *testing.T) {
	p := &hotkeySettingsPlugin{values: map[string]string{"legacy": "invalid", "actions": `[{"id":"a","key":"hold:right_alt"},{"key":"invalid","disabled":true}]`}}
	table := &definition.PluginSettingValueTable{Key: "actions", HotkeyDisabledField: "disabled", Columns: []definition.PluginSettingValueTableColumn{{Key: "key", Type: definition.PluginSettingValueTableColumnTypeHotkey, Scope: definition.HotkeyScopeGlobal}}}
	a := NewAPI(&Instance{Plugin: p, Metadata: Metadata{SettingDefinitions: []definition.PluginSettingDefinitionItem{{Value: &definition.PluginSettingValueHotkey{Key: "legacy"}}, {Value: table}}}}).(*APIImpl)
	bindings, err := a.collectHotkeys(t.Context(), "", "")
	if err != nil || len(bindings) != 1 || bindings[0].event.ItemID != "a" || bindings[0].event.ColumnKey != "key" {
		t.Fatalf("bindings=%+v err=%v", bindings, err)
	}
	for _, raw := range []string{`[{"key":"ctrl+d"}]`, `[{"id":"a","key":"ctrl+d"},{"id":"a","key":"ctrl+e"}]`, `[{"id":"a","key":42}]`} {
		if _, err := a.collectHotkeys(t.Context(), "actions", raw); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	table.Columns[0].AllowedHotkeyKinds = []string{"normalCombo"}
	if _, err := a.collectHotkeys(t.Context(), "", ""); err == nil {
		t.Fatal("allowed kinds ignored")
	}
}

// TestHotkeyChangeKeepsUnchangedBinding proves a second setting does not invalidate an active hold.
func TestHotkeyChangeKeepsUnchangedBinding(t *testing.T) {
	p := &hotkeySettingsPlugin{values: map[string]string{"first": "hold:right_alt", "second": "ctrl+f10"}}
	instance := &Instance{Plugin: p, Metadata: Metadata{Id: "two-hotkeys", SettingDefinitions: []definition.PluginSettingDefinitionItem{
		{Value: &definition.PluginSettingValueHotkey{Key: "first", Scope: definition.HotkeyScopeGlobal}},
		{Value: &definition.PluginSettingValueHotkey{Key: "second", Scope: definition.HotkeyScopeGlobal}},
	}}}
	a := NewAPI(instance).(*APIImpl)
	instance.API = a
	hotkeyCapability.RLock()
	previous := hotkeyCapability.apply
	hotkeyCapability.RUnlock()
	t.Cleanup(func() { SetHotkeyBindingApplier(previous) })
	var entries []corehotkey.Entry
	SetHotkeyBindingApplier(func(_ context.Context, _ string, next []corehotkey.Entry, persist func() error) error {
		entries = next
		if persist != nil {
			return persist()
		}
		return nil
	})
	events := make(chan HotkeyEvent, 4)
	if !a.OnHotkey(t.Context(), OnHotkeyOption{Callback: func(_ context.Context, e HotkeyEvent) { events <- e }}).Success {
		t.Fatal("subscription failed")
	}
	if err := a.ActivateHotkeys(t.Context()); err != nil {
		t.Fatal(err)
	}
	held := entries[0]
	held.OnHoldTransition(utilhotkey.HoldTransition{Started: true})
	start := <-events
	if err := a.applyHotkeys(t.Context(), "second", "ctrl+f11", nil); err != nil {
		t.Fatal(err)
	}
	held.OnHoldTransition(utilhotkey.HoldTransition{Reason: utilhotkey.HoldEndReleased})
	select {
	case end := <-events:
		if end.Phase != HotkeyPhaseEnded || end.ActivationID != start.ActivationID || end.EndReason != HotkeyEndReleased {
			t.Fatalf("end=%+v", end)
		}
	case <-time.After(time.Second):
		t.Fatal("unchanged hold was lost")
	}
	a.stopHotkeys()
}
