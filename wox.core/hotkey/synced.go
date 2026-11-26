package hotkey

import (
	"context"
	"errors"
	"slices"
	utilhotkey "wox/util/hotkey"
)

var ErrHotkeyRegistrationConflict = errors.New("hotkey cannot be registered")

// ApplySyncedConfig saves shared settings before binding them. A nil persist refreshes already saved bindings.
func (s *Service) ApplySyncedConfig(ctx context.Context, config WoxConfig, persist func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if persist != nil {
		if err := persist(); err != nil {
			return err
		}
	}
	previousRegistered := s.registered
	s.collectWoxConfig(ctx, config)
	desired := s.collector.snapshot()
	entries, specs := syncedHotkeySpecs(desired)
	s.unregisterLocked(ctx)
	if err := s.registerSpecsLocked(ctx, entries, specs); err != nil {
		// If a backend rejects the group, retry previously active bindings that are still in the shared config.
		var retainedEntries []Entry
		var retainedSpecs []utilhotkey.Spec
		for i, entry := range entries {
			if hasHotkey(previousRegistered, entry) {
				retainedEntries = append(retainedEntries, entry)
				retainedSpecs = append(retainedSpecs, specs[i])
			}
		}
		if len(retainedSpecs) > 0 {
			err = errors.Join(err, s.registerSpecsLocked(ctx, retainedEntries, retainedSpecs))
		}
		return errors.Join(ErrHotkeyRegistrationConflict, err)
	}
	for _, entry := range desired {
		if entry.Source != SourcePlugin && !hasHotkey(s.registered, entry) {
			return ErrHotkeyRegistrationConflict
		}
	}
	return nil
}

// syncedHotkeySpecs isolates unavailable chords and gives shared Wox settings priority over local plugins.
func syncedHotkeySpecs(desired []Entry) ([]Entry, []utilhotkey.Spec) {
	entries := make([]Entry, 0, len(desired))
	specs := make([]utilhotkey.Spec, 0, len(desired))
	seen := make(map[string]bool, len(desired))
	ordered := slices.Clone(desired)
	slices.SortStableFunc(ordered, func(a, b Entry) int {
		return hotkeySourcePriority(a.Source) - hotkeySourcePriority(b.Source)
	})
	for _, entry := range ordered {
		key, err := utilhotkey.BindingKey(entry.CombineKey)
		if err != nil || key == "" || seen[key] {
			continue
		}
		if _, err := utilhotkey.BindingKind(entry.CombineKey); err != nil {
			continue
		}
		spec, err := buildHotkeySpecs([]Entry{entry})
		if err != nil || len(spec) == 0 {
			continue
		}
		seen[key] = true
		entries = append(entries, entry)
		specs = append(specs, spec[0])
	}
	return entries, specs
}

func hotkeySourcePriority(source EntrySource) int {
	switch source {
	case SourceMain:
		return 0
	case SourceSelection:
		return 1
	case SourceQuery:
		return 2
	case SourceResult:
		return 3
	default:
		return 4
	}
}
