package hotkey

import (
	"context"
	"fmt"
	"wox/util"
	utilhotkey "wox/util/hotkey"
)

// ApplyPluginBindings coordinates registration and persistence without exposing native ownership to plugins.
func (s *Service) ApplyPluginBindings(ctx context.Context, owner string, bindings []Entry, persist func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := s.collector.snapshot()
	next := make([]Entry, 0, len(previous)+len(bindings))
	for _, entry := range previous {
		if entry.Source != SourcePlugin || entry.Owner != owner {
			next = append(next, entry)
		}
	}
	for _, entry := range bindings {
		entry.Source = SourcePlugin
		entry.Owner = owner
		entry.ID = owner + "\x00" + entry.ID
		next = append(next, entry)
	}
	if len(bindings) == 0 && persist == nil {
		if len(next) == len(previous) {
			return nil
		}
		if !util.IsLinux() && s.group != nil {
			s.collector.restore(next)
			return s.registerPluginChangesLocked(ctx, owner, next)
		}
		// Teardown must never restore a stopped plugin when another registration fails.
		remaining := make([]Entry, 0, len(s.registered))
		for _, entry := range s.registered {
			if entry.Source != SourcePlugin || entry.Owner != owner {
				remaining = append(remaining, entry)
			}
		}
		specs, err := buildHotkeySpecs(remaining)
		if err != nil {
			return err
		}
		s.registered, s.registeredSpecs = remaining, specs
		s.collector.restore(next)
		return s.registerAllLocked(ctx)
	}
	s.collector.restore(next)
	restore := func() { s.collector.restore(previous) }
	if err := validateHotkeyConflicts(next); err != nil {
		restore()
		return err
	}
	register := func(entries []Entry) error {
		if !util.IsLinux() && s.group != nil {
			return s.registerPluginChangesLocked(ctx, owner, entries)
		}
		return s.registerAllLocked(ctx)
	}
	if err := register(next); err != nil {
		restore()
		return err
	}
	// Linux can skip unsupported special bindings; never report those as successfully applied.
	for _, entry := range next {
		if entry.Source != SourcePlugin || entry.Owner != owner {
			continue
		}
		found := false
		for _, registered := range s.registered {
			if registered.Source == SourcePlugin && registered.ID == entry.ID {
				found = true
				break
			}
		}
		if !found {
			restore()
			if rollbackErr := register(previous); rollbackErr != nil {
				return fmt.Errorf("hotkey unavailable: %s; rollback failed: %w", entry.CombineKey, rollbackErr)
			}
			return fmt.Errorf("hotkey unavailable: %s", entry.CombineKey)
		}
	}
	if persist != nil {
		if err := persist(); err != nil {
			restore()
			if rollbackErr := register(previous); rollbackErr != nil {
				return fmt.Errorf("%w; hotkey rollback failed: %v", err, rollbackErr)
			}
			return err
		}
	}
	return nil
}

// registerPluginChangesLocked keeps other owners' native registrations on Windows and macOS.
func (s *Service) registerPluginChangesLocked(ctx context.Context, owner string, entries []Entry) error {
	desired := make([]Entry, 0, len(s.registered)+len(entries))
	for _, entry := range s.registered {
		if entry.Source != SourcePlugin || entry.Owner != owner {
			desired = append(desired, entry)
		}
	}
	for _, entry := range entries {
		if entry.Source == SourcePlugin && entry.Owner == owner {
			desired = append(desired, entry)
		}
	}
	specs, err := buildHotkeySpecs(desired)
	if err != nil {
		return err
	}
	keep := make(map[string]bool, len(desired))
	for _, next := range desired {
		for _, old := range s.registered {
			if old.Source == next.Source && old.Owner == next.Owner && old.ID == next.ID && old.CombineKey == next.CombineKey {
				key, _ := utilhotkey.BindingKey(next.CombineKey)
				keep[key] = true
				break
			}
		}
	}
	if err := s.group.ReconcileIndependent(ctx, s.registeredSpecs, specs, keep); err != nil {
		s.registeredSpecs, s.registered = registeredHotkeyState(s.registeredSpecs, s.registered, s.group.RegisteredCombineKeys())
		return err
	}
	s.registeredSpecs, s.registered = registeredHotkeyState(specs, desired, s.group.RegisteredCombineKeys())
	return nil
}
