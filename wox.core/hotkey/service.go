package hotkey

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"wox/setting"
	"wox/util"
	utilhotkey "wox/util/hotkey"
)

// Callbacks routes Wox-owned hotkey triggers back to their owning subsystem.
type Callbacks struct {
	OnMain                 func(combineKey string)
	OnSelection            func(combineKey string)
	OnQuery                func(combineKey string, queryHotkey setting.QueryHotkey)
	OnDictationHoldPress   func(ctx context.Context, actionID string)
	OnDictationHoldRelease func(ctx context.Context, actionID string)
	OnDictationPressAction func(ctx context.Context, actionID string)
}

// WoxConfig is the hotkey subset of Wox settings used by the service.
type WoxConfig struct {
	MainHotkey      string
	SelectionHotkey string
	QueryHotkeys    []setting.QueryHotkey
}

// DictationBinding is the runtime hotkey binding for one dictation action.
type DictationBinding struct {
	ActionID string
	Hotkey   string
}

// Service is the Wox business-layer registry for global hotkeys.
type Service struct {
	callbacks Callbacks
	collector *collector

	mu              sync.Mutex
	group           *utilhotkey.Group
	registeredSpecs []utilhotkey.Spec
}

// NewService creates a Wox hotkey service with the given trigger callbacks.
func NewService(callbacks Callbacks) *Service {
	return &Service{
		callbacks: callbacks,
		collector: newCollector(),
	}
}

// WoxConfigFromSetting snapshots the hotkey fields from Wox settings.
func WoxConfigFromSetting(woxSetting *setting.WoxSetting) WoxConfig {
	return WoxConfig{
		MainHotkey:      woxSetting.MainHotkey.Get(),
		SelectionHotkey: woxSetting.SelectionHotkey.Get(),
		QueryHotkeys:    cloneQueryHotkeys(woxSetting.QueryHotkeys.Get()),
	}
}

// CollectWoxSettings collects startup Wox-setting hotkeys before plugins load.
func (s *Service) CollectWoxSettings(ctx context.Context, woxSetting *setting.WoxSetting) {
	s.collectWoxConfig(ctx, WoxConfigFromSetting(woxSetting))
}

// RegisterAll binds all collected hotkeys to the platform in one pass.
func (s *Service) RegisterAll(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.registerAllLocked(ctx)
}

// UpdateWoxConfig registers a pending Wox hotkey config and optionally restores
// the collector if registration fails before the caller persists the setting.
func (s *Service) UpdateWoxConfig(ctx context.Context, config WoxConfig, restoreCollectorOnFailure bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	previousEntries := s.collector.snapshot()
	s.collectWoxConfig(ctx, config)
	if err := s.registerAllLocked(ctx); err != nil {
		if restoreCollectorOnFailure {
			s.collector.restore(previousEntries)
		}
		return err
	}
	return nil
}

// UpdateDictationBindings replaces dictation action bindings and optionally
// re-registers the whole Wox hotkey set immediately.
func (s *Service) UpdateDictationBindings(ctx context.Context, bindings []DictationBinding, registerNow bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	previousEntries := s.collector.snapshot()
	if err := s.collectDictationBindings(ctx, bindings); err != nil {
		s.collector.restore(previousEntries)
		return err
	}
	if registerNow {
		if err := s.registerAllLocked(ctx); err != nil {
			s.collector.restore(previousEntries)
			return err
		}
	}
	return nil
}

// UnregisterAll unregisters all hotkeys managed by this service.
func (s *Service) UnregisterAll(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unregisterLocked(ctx)
}

// Snapshot returns a copy of all collected Wox hotkey entries.
func (s *Service) Snapshot() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.collector.snapshot()
}

// EffectiveSelectionHotkeyForRuntime returns the runtime selection hotkey.
func EffectiveSelectionHotkeyForRuntime(selectionHotkey string) string {
	if util.IsLinuxWaylandSession() {
		return ""
	}
	return strings.TrimSpace(selectionHotkey)
}

func (s *Service) registerAllLocked(ctx context.Context) error {
	entries := s.collector.snapshot()
	specs, specsErr := buildHotkeySpecs(entries)
	if specsErr != nil {
		return specsErr
	}

	if len(specs) == 0 {
		s.unregisterLocked(ctx)
		return nil
	}

	return s.registerSpecsLocked(ctx, specs)
}

func buildHotkeySpecs(entries []Entry) ([]utilhotkey.Spec, error) {
	specs := make([]utilhotkey.Spec, 0, len(entries))
	for _, e := range entries {
		combineKey := e.CombineKey
		parsed, parseErr := utilhotkey.ParseBinding(combineKey)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid hotkey binding: source=%s id=%s key=%s: %w", e.Source, e.ID, combineKey, parseErr)
		}
		if parsed.CombineKey == "" {
			continue
		}
		if e.OnPress == nil {
			return nil, fmt.Errorf("hotkey callback is required: source=%s id=%s key=%s", e.Source, e.ID, combineKey)
		}

		spec := utilhotkey.Spec{
			CombineKey: parsed.CombineKey,
			Callback:   e.OnPress,
		}
		if parsed.Trigger == utilhotkey.TriggerHold {
			if !utilhotkey.IsModifierChordHotkeyString(parsed.CombineKey) {
				return nil, fmt.Errorf("hold hotkey requires a modifier-only chord: source=%s id=%s key=%s", e.Source, e.ID, combineKey)
			}
			if e.OnRelease == nil {
				return nil, fmt.Errorf("hold hotkey release callback is required: source=%s id=%s key=%s", e.Source, e.ID, combineKey)
			}
			spec.OnRelease = e.OnRelease
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

func (s *Service) registerSpecsLocked(ctx context.Context, specs []utilhotkey.Spec) error {
	previousSpecs := cloneHotkeySpecs(s.registeredSpecs)
	if s.group != nil {
		s.group.Unregister(ctx)
		s.group = nil
		s.registeredSpecs = nil
	}

	group, err := utilhotkey.RegisterGroup(ctx, specs)
	if err != nil {
		if len(previousSpecs) > 0 {
			restoreGroup, restoreErr := utilhotkey.RegisterGroup(ctx, previousSpecs)
			if restoreErr != nil {
				return fmt.Errorf("failed to register hotkeys: %w; failed to restore previous hotkeys: %v", err, restoreErr)
			}
			s.group = restoreGroup
			s.registeredSpecs = previousSpecs
		}
		return err
	}

	s.group = group
	s.registeredSpecs = cloneHotkeySpecs(specs)
	return nil
}

func (s *Service) unregisterLocked(ctx context.Context) {
	if s.group != nil {
		s.group.Unregister(ctx)
		s.group = nil
	}
	s.registeredSpecs = nil
}

func cloneHotkeySpecs(specs []utilhotkey.Spec) []utilhotkey.Spec {
	if len(specs) == 0 {
		return nil
	}
	return append([]utilhotkey.Spec(nil), specs...)
}

func (s *Service) collectWoxConfig(ctx context.Context, config WoxConfig) {
	mainHotkey := strings.TrimSpace(config.MainHotkey)
	if mainHotkey != "" {
		combineKey := mainHotkey
		s.collector.set(SourceMain, "main", Entry{
			CombineKey: combineKey,
			OnPress: func() {
				s.callbacks.OnMain(combineKey)
			},
		})
	} else {
		s.collector.remove(SourceMain, "main")
	}

	selectionHotkey := EffectiveSelectionHotkeyForRuntime(config.SelectionHotkey)
	if selectionHotkey != "" {
		combineKey := selectionHotkey
		s.collector.set(SourceSelection, "selection", Entry{
			CombineKey: combineKey,
			OnPress: func() {
				s.callbacks.OnSelection(combineKey)
			},
		})
	} else {
		s.collector.remove(SourceSelection, "selection")
	}

	queryEntries := make([]Entry, 0, len(config.QueryHotkeys))
	for _, qh := range config.QueryHotkeys {
		if qh.Disabled || strings.TrimSpace(qh.Hotkey) == "" {
			continue
		}
		queryHotkey := qh
		combineKey := strings.TrimSpace(queryHotkey.Hotkey)
		queryEntries = append(queryEntries, Entry{
			ID:         combineKey,
			CombineKey: combineKey,
			OnPress: func() {
				s.callbacks.OnQuery(combineKey, queryHotkey)
			},
		})
	}
	s.collector.replaceSource(SourceQuery, queryEntries)
}

func (s *Service) collectDictationBindings(ctx context.Context, bindings []DictationBinding) error {
	entries := make([]Entry, 0, len(bindings))
	for _, binding := range bindings {
		actionID := strings.TrimSpace(binding.ActionID)
		hotkeyStr := strings.TrimSpace(binding.Hotkey)
		if actionID == "" || hotkeyStr == "" {
			continue
		}
		parsed, parseErr := utilhotkey.ParseBinding(hotkeyStr)
		if parseErr != nil {
			return fmt.Errorf("invalid dictation hotkey: action=%s key=%s: %w", actionID, hotkeyStr, parseErr)
		}
		if parsed.CombineKey == "" {
			continue
		}

		entry := Entry{
			ID:         actionID,
			CombineKey: hotkeyStr,
			OnPress: func() {
				cbCtx := util.NewTraceContext()
				if parsed.Trigger == utilhotkey.TriggerHold {
					s.callbacks.OnDictationHoldPress(cbCtx, actionID)
				} else {
					s.callbacks.OnDictationPressAction(cbCtx, actionID)
				}
			},
		}
		if parsed.Trigger == utilhotkey.TriggerHold {
			entry.OnRelease = func() {
				cbCtx := util.NewTraceContext()
				s.callbacks.OnDictationHoldRelease(cbCtx, actionID)
			}
		}
		entries = append(entries, entry)
	}
	s.collector.replaceSource(SourceDictation, entries)
	return nil
}

func cloneQueryHotkeys(queryHotkeys []setting.QueryHotkey) []setting.QueryHotkey {
	if len(queryHotkeys) == 0 {
		return nil
	}
	return append([]setting.QueryHotkey(nil), queryHotkeys...)
}
