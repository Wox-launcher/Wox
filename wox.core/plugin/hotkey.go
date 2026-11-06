package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
	corehotkey "wox/hotkey"
	"wox/setting/definition"
	"wox/util"
	utilhotkey "wox/util/hotkey"

	"github.com/google/uuid"
)

type HotkeyKind string

const (
	HotkeyKindNormalCombo    HotkeyKind = "normalCombo"
	HotkeyKindDoubleModifier HotkeyKind = "doubleModifier"
	HotkeyKindCapsLockCombo  HotkeyKind = "capsLockCombo"
	HotkeyKindPressModifier  HotkeyKind = "pressModifier"
	HotkeyKindHoldModifier   HotkeyKind = "holdModifier"
	HotkeyKindHoldKey        HotkeyKind = "holdKey"
)

type HotkeyPhase string

const (
	HotkeyPhaseTriggered HotkeyPhase = "triggered"
	HotkeyPhaseStarted   HotkeyPhase = "started"
	HotkeyPhaseEnded     HotkeyPhase = "ended"
)

type HotkeyEndReason string

const (
	HotkeyEndReleased       HotkeyEndReason = "released"
	HotkeyEndInterrupted    HotkeyEndReason = "interrupted"
	HotkeyEndDisabled       HotkeyEndReason = "disabled"
	HotkeyEndBindingChanged HotkeyEndReason = "bindingChanged"
	HotkeyEndUnavailable    HotkeyEndReason = "unavailable"
)

type HotkeyEvent struct {
	SettingKey   string
	ItemID       string `json:",omitempty"`
	ColumnKey    string `json:",omitempty"`
	Kind         HotkeyKind
	Phase        HotkeyPhase
	Hotkey       string
	ActivationID string
	Timestamp    int64
	Env          *QueryEnv       `json:",omitempty"`
	EndReason    HotkeyEndReason `json:",omitempty"`
}
type OnHotkeyOption struct {
	Callback func(context.Context, HotkeyEvent) `json:"-"`
}
type OnHotkeyResult struct {
	Success bool
	ErrMsg  string
}

type HotkeyBindingApplier func(context.Context, string, []corehotkey.Entry, func() error) error

var hotkeyCapability struct {
	sync.RWMutex
	apply HotkeyBindingApplier
}

// SetHotkeyBindingApplier injects the shared registry without a plugin-to-UI dependency.
func SetHotkeyBindingApplier(apply HotkeyBindingApplier) {
	hotkeyCapability.Lock()
	hotkeyCapability.apply = apply
	hotkeyCapability.Unlock()
}

// hotkeySubscription owns one init cycle, immutable binding definitions and event delivery.
type hotkeySubscription struct {
	mu       sync.Mutex
	callback func(context.Context, HotkeyEvent)
	enabled  bool
	closed   bool
	bindings []*pluginHotkeyBinding
}
type pluginHotkeyBinding struct {
	event        HotkeyEvent
	owner        *APIImpl
	subscription *hotkeySubscription
	mu           sync.Mutex
	activation   string
	endReason    HotkeyEndReason
	closed       bool
	queue        []HotkeyEvent
	wake         chan struct{}
	done         chan struct{}
}

// OnHotkey subscribes once per init cycle; global settings become live only after successful Init.
func (a *APIImpl) OnHotkey(ctx context.Context, option OnHotkeyOption) OnHotkeyResult {
	if a.pluginInstance == nil || option.Callback == nil {
		return OnHotkeyResult{ErrMsg: "hotkey callback is required"}
	}
	a.hotkeyMu.Lock()
	defer a.hotkeyMu.Unlock()
	if a.hotkeysStopped {
		return OnHotkeyResult{ErrMsg: "plugin is stopped"}
	}
	if a.hotkeys != nil {
		return OnHotkeyResult{ErrMsg: "OnHotkey is already subscribed"}
	}
	a.hotkeys = &hotkeySubscription{callback: option.Callback}
	initialized, _ := a.pluginInstance.initStatus()
	if initialized {
		if err := a.activateHotkeysLocked(ctx); err != nil {
			a.hotkeys = nil
			return OnHotkeyResult{ErrMsg: err.Error()}
		}
	}
	return OnHotkeyResult{Success: true}
}

// collectHotkeys reads static metadata only; dynamic settings do not silently acquire global bindings.
func (a *APIImpl) collectHotkeys(ctx context.Context, key, value string) ([]*pluginHotkeyBinding, error) {
	var result []*pluginHotkeyBinding
	add := func(settingKey, itemID, columnKey, hotkey string, allowed []string) error {
		hotkey = strings.TrimSpace(hotkey)
		if hotkey == "" {
			return nil
		}
		kind, err := utilhotkey.BindingKind(hotkey)
		if err != nil {
			return fmt.Errorf("%s: %w", settingKey, err)
		}
		if len(allowed) > 0 && !slices.Contains(allowed, string(kind)) {
			return fmt.Errorf("%s: hotkey kind %s is not allowed", settingKey, kind)
		}
		result = append(result, &pluginHotkeyBinding{event: HotkeyEvent{SettingKey: settingKey, ItemID: itemID, ColumnKey: columnKey, Hotkey: hotkey, Kind: HotkeyKind(kind)}, owner: a, subscription: a.hotkeys, wake: make(chan struct{}, 1), done: make(chan struct{})})
		if len(result) > 128 {
			return errors.New("at most 128 global hotkey bindings are supported per plugin")
		}
		return nil
	}
	for _, item := range a.pluginInstance.Metadata.SettingDefinitions {
		if item.Value == nil || slices.Contains(item.DisabledInPlatforms, util.Platform(util.GetCurrentPlatform())) {
			continue
		}
		settingKey := item.Value.GetKey()
		read := func() string {
			if settingKey == key {
				return value
			}
			return a.GetSetting(ctx, settingKey)
		}
		switch v := item.Value.(type) {
		case *definition.PluginSettingValueHotkey:
			if v.Scope == definition.HotkeyScopeGlobal {
				if err := add(settingKey, "", "", read(), v.AllowedHotkeyKinds); err != nil {
					return nil, err
				}
			}
		case *definition.PluginSettingValueTable:
			global := false
			for _, column := range v.Columns {
				if column.Type == definition.PluginSettingValueTableColumnTypeHotkey && column.Scope == definition.HotkeyScopeGlobal {
					global = true
				}
			}
			if !global {
				continue
			}
			raw := read()
			if raw == "" {
				raw = "[]"
			}
			var rows []map[string]json.RawMessage
			if err := json.Unmarshal([]byte(raw), &rows); err != nil {
				return nil, fmt.Errorf("%s: invalid hotkey table: %w", settingKey, err)
			}
			idField := v.HotkeyIdentityField
			if idField == "" {
				idField = "id"
			}
			seen := map[string]bool{}
			for _, row := range rows {
				var id string
				_ = json.Unmarshal(row[idField], &id)
				var disabled bool
				if v.HotkeyDisabledField != "" {
					if raw, ok := row[v.HotkeyDisabledField]; ok {
						if err := json.Unmarshal(raw, &disabled); err != nil {
							return nil, fmt.Errorf("%s: invalid disabled flag", settingKey)
						}
					}
				}
				if disabled {
					continue
				}
				hasBinding := false
				for _, column := range v.Columns {
					if column.Type == definition.PluginSettingValueTableColumnTypeHotkey && column.Scope == definition.HotkeyScopeGlobal {
						var binding string
						if raw, ok := row[column.Key]; ok {
							if err := json.Unmarshal(raw, &binding); err != nil {
								return nil, fmt.Errorf("%s: invalid hotkey value", settingKey)
							}
						}
						if strings.TrimSpace(binding) == "" {
							continue
						}
						hasBinding = true
						if id == "" {
							return nil, fmt.Errorf("%s: global hotkey rows require stable %s", settingKey, idField)
						}
						if err := add(settingKey, id, column.Key, binding, column.AllowedHotkeyKinds); err != nil {
							return nil, err
						}
					}
				}
				if hasBinding {
					if seen[id] {
						return nil, fmt.Errorf("%s: duplicate row ID %s", settingKey, id)
					}
					seen[id] = true
				}
			}
		}
	}
	return result, nil
}

// applyHotkeys keeps the last working registration when validation or persistence fails.
func (a *APIImpl) applyHotkeys(ctx context.Context, key, value string, persist func() error) error {
	next, err := a.collectHotkeys(ctx, key, value)
	if err != nil {
		return err
	}
	if len(next) == len(a.hotkeys.bindings) {
		unchanged := true
		for i := range next {
			if next[i].event != a.hotkeys.bindings[i].event {
				unchanged = false
				break
			}
		}
		if unchanged {
			if persist != nil {
				return persist()
			}
			return nil
		}
	}
	// Preserve unchanged callback owners so another binding's update cannot end their active hold.
	reused := make(map[*pluginHotkeyBinding]bool, len(next))
	for i, binding := range next {
		for _, old := range a.hotkeys.bindings {
			if !reused[old] && old.event == binding.event {
				next[i] = old
				reused[old] = true
				break
			}
		}
	}
	hotkeyCapability.RLock()
	apply := hotkeyCapability.apply
	hotkeyCapability.RUnlock()
	if apply == nil {
		if len(next) == 0 {
			if persist != nil {
				return persist()
			}
			return nil
		}
		return errors.New("global hotkey service unavailable")
	}
	var entries []corehotkey.Entry
	for _, binding := range next {
		b := binding
		entry := corehotkey.Entry{ID: b.event.SettingKey + "\x00" + b.event.ItemID + "\x00" + b.event.ColumnKey, CombineKey: b.event.Hotkey, OnPress: func() { b.enqueue(HotkeyPhaseTriggered, "") }}
		if b.event.Kind == HotkeyKindHoldModifier || b.event.Kind == HotkeyKindHoldKey {
			entry.OnPress = func() {}
			entry.OnRelease = func() {}
			entry.OnHoldTransition = func(t utilhotkey.HoldTransition) {
				if t.Started {
					b.enqueue(HotkeyPhaseStarted, "")
				} else {
					b.enqueue(HotkeyPhaseEnded, HotkeyEndReason(t.Reason))
				}
			}
		}
		entries = append(entries, entry)
	}
	// Native re-registration ends active holds; report the owning setting change.
	for _, old := range a.hotkeys.bindings {
		old.mu.Lock()
		old.endReason = HotkeyEndBindingChanged
		old.mu.Unlock()
	}
	err = apply(ctx, a.pluginInstance.Metadata.Id, entries, persist)
	for _, old := range a.hotkeys.bindings {
		old.mu.Lock()
		old.endReason = ""
		old.mu.Unlock()
	}
	if err != nil {
		return err
	}
	for _, old := range a.hotkeys.bindings {
		if !reused[old] {
			old.stop(HotkeyEndBindingChanged)
		}
	}
	a.hotkeys.bindings = next
	for _, binding := range next {
		if !reused[binding] {
			util.Go(ctx, "plugin hotkey events", binding.deliver)
		}
	}
	return nil
}

func (a *APIImpl) activateHotkeysLocked(ctx context.Context) error {
	if a.hotkeys == nil {
		return nil
	}
	if err := a.applyHotkeys(ctx, "", "", nil); err != nil {
		return err
	}
	a.hotkeys.mu.Lock()
	a.hotkeys.enabled = true
	a.hotkeys.mu.Unlock()
	return nil
}

// ActivateHotkeys runs after Init; registration errors remain visible as plugin initialization errors.
func (a *APIImpl) ActivateHotkeys(ctx context.Context) error {
	a.hotkeyMu.Lock()
	defer a.hotkeyMu.Unlock()
	return a.activateHotkeysLocked(ctx)
}

// RefreshHotkeys reconciles externally persisted changes, such as Cloud Sync.
func (a *APIImpl) RefreshHotkeys(ctx context.Context) error {
	a.hotkeyMu.Lock()
	defer a.hotkeyMu.Unlock()
	if a.hotkeys == nil || !a.hotkeys.enabled {
		return nil
	}
	if err := a.applyHotkeys(ctx, "", "", nil); err != nil {
		// The synchronized value is already persisted. Disable stale bindings, but
		// retain the subscription so a later corrected value can recover.
		for _, binding := range a.hotkeys.bindings {
			binding.stop(HotkeyEndUnavailable)
		}
		a.hotkeys.bindings = nil
		hotkeyCapability.RLock()
		apply := hotkeyCapability.apply
		hotkeyCapability.RUnlock()
		if apply != nil {
			err = errors.Join(err, apply(ctx, a.pluginInstance.Metadata.Id, nil, nil))
		}
		return err
	}
	return nil
}

// stopHotkeysLocked closes admission before touching the platform registry.
func (a *APIImpl) stopHotkeysLocked(ctx context.Context) {
	a.hotkeysStopped = true
	if a.hotkeys == nil {
		return
	}
	a.hotkeys.mu.Lock()
	a.hotkeys.closed = true
	a.hotkeys.enabled = false
	a.hotkeys.mu.Unlock()
	for _, binding := range a.hotkeys.bindings {
		binding.stop(HotkeyEndDisabled)
	}
	hotkeyCapability.RLock()
	apply := hotkeyCapability.apply
	hotkeyCapability.RUnlock()
	if apply != nil {
		if err := apply(ctx, a.pluginInstance.Metadata.Id, nil, nil); err != nil {
			util.GetLogger().Error(ctx, "remove plugin hotkeys: "+err.Error())
		}
	}
	a.hotkeys = nil
}
func (a *APIImpl) stopHotkeys() {
	a.hotkeyMu.Lock()
	defer a.hotkeyMu.Unlock()
	a.stopHotkeysLocked(util.NewTraceContext())
}

// enqueue is called on the native transition path: it only records state and wakes delivery.
func (b *pluginHotkeyBinding) enqueue(phase HotkeyPhase, reason HotkeyEndReason) {
	b.subscription.mu.Lock()
	enabled := b.subscription.enabled && !b.subscription.closed
	b.subscription.mu.Unlock()
	if !enabled {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	if phase != HotkeyPhaseEnded && len(b.queue) >= 256 {
		return
	}
	e := b.event
	e.Phase = phase
	e.Timestamp = time.Now().UnixMilli()
	if reason == HotkeyEndUnavailable && b.endReason != "" {
		reason = b.endReason
	}
	e.EndReason = reason
	if phase == HotkeyPhaseEnded {
		if b.activation == "" {
			return
		}
		e.ActivationID = b.activation
		b.activation = ""
	} else {
		if phase == HotkeyPhaseStarted && b.activation != "" {
			return
		}
		e.ActivationID = uuid.NewString()
		if phase == HotkeyPhaseStarted {
			b.activation = e.ActivationID
		}
	}
	b.queue = append(b.queue, e)
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

// stop invalidates old native callbacks and wakes the dispatcher so it can finish its queue.
func (b *pluginHotkeyBinding) stop(reason HotkeyEndReason) {
	b.enqueue(HotkeyPhaseEnded, reason)
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

// deliver preserves per-binding event order; runtime hosts acknowledge dispatch without awaiting plugin promises.
func (b *pluginHotkeyBinding) deliver() {
	defer close(b.done)
	var env *QueryEnv
	for range b.wake {
		for {
			b.mu.Lock()
			if len(b.queue) == 0 {
				closed := b.closed
				b.mu.Unlock()
				if closed {
					return
				}
				break
			}
			event := b.queue[0]
			b.queue = b.queue[1:]
			b.mu.Unlock()
			b.subscription.mu.Lock()
			closed := b.subscription.closed
			callback := b.subscription.callback
			b.subscription.mu.Unlock()
			if closed {
				return
			}
			ctx := util.NewTraceContext()
			if event.Phase != HotkeyPhaseEnded {
				query := Query{}
				manager := GetPluginManager()
				// Global activation can happen without showing Wox, so the last query's
				// cached foreground window is not necessarily the current target.
				if b.owner.pluginInstance.Metadata.IsSupportFeature(MetadataFeatureQueryEnv) {
					if ui, ok := manager.GetUI().(interface{ RefreshActiveWindowSnapshot(context.Context) }); ok {
						ui.RefreshActiveWindowSnapshot(ctx)
					}
					manager.fillResultBindingQueryEnv(ctx, &query)
				}
				query = manager.buildPluginQueryEnv(ctx, b.owner.pluginInstance, query)
				snapshot := query.Env
				env = &snapshot
			}
			event.Env = env
			callback(ctx, event)
		}
	}
}

// setHotkeySetting registers the proposed value before committing it to persisted settings.
func (a *APIImpl) setHotkeySetting(ctx context.Context, option SetSettingOption) SetSettingResult {
	a.hotkeyMu.Lock()
	defer a.hotkeyMu.Unlock()
	if a.hotkeys == nil || !a.hotkeys.enabled {
		return a.persistSetting(ctx, option)
	}
	relevant := false
	for _, item := range a.pluginInstance.Metadata.SettingDefinitions {
		if item.Value == nil || item.Value.GetKey() != option.Key {
			continue
		}
		switch v := item.Value.(type) {
		case *definition.PluginSettingValueHotkey:
			relevant = v.Scope == definition.HotkeyScopeGlobal
		case *definition.PluginSettingValueTable:
			for _, c := range v.Columns {
				if c.Scope == definition.HotkeyScopeGlobal && c.Type == definition.PluginSettingValueTableColumnTypeHotkey {
					relevant = true
				}
			}
		}
	}
	if !relevant {
		return a.persistSetting(ctx, option)
	}
	result := SetSettingResult{}
	err := a.applyHotkeys(ctx, option.Key, option.Value, func() error {
		result = a.persistSetting(ctx, option)
		if !result.Success {
			return errors.New(result.ErrMsg)
		}
		return nil
	})
	if err != nil {
		return SetSettingResult{ErrMsg: err.Error()}
	}
	return result
}
