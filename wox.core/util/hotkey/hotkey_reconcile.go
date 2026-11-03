package hotkey

import (
	"context"
	"fmt"
	"wox/util"
)

// ReconcileIndependent changes only replaced registrations on platforms with independent native hotkeys.
// The caller supplies unchanged physical chords and the exact previously active specs for rollback.
func (g *Group) ReconcileIndependent(ctx context.Context, previous, next []Spec, keep map[string]bool) error {
	if util.IsLinux() || g.registration != nil {
		return fmt.Errorf("independent hotkeys unavailable")
	}
	retained := make([]*Hotkey, 0, len(g.hotkeys))
	for _, hk := range g.hotkeys {
		key, _ := BindingKey(hk.combineKey)
		if keep[key] {
			retained = append(retained, hk)
		} else {
			hk.Unregister(ctx)
		}
	}
	selectSpecs := func(specs []Spec) []Spec {
		selected := make([]Spec, 0, len(specs))
		for _, spec := range specs {
			key, _ := BindingKey(spec.CombineKey)
			if !keep[key] {
				selected = append(selected, spec)
			}
		}
		return selected
	}
	wanted := selectSpecs(next)
	added, err := RegisterGroup(ctx, wanted)
	if err == nil && len(added.RegisteredCombineKeys()) != len(wanted) {
		err = fmt.Errorf("one or more hotkeys were unavailable")
	}
	if err != nil {
		if added != nil {
			added.Unregister(ctx)
		}
		removed := selectSpecs(previous)
		restored, restoreErr := RegisterGroup(ctx, removed)
		g.hotkeys = retained
		if restored != nil {
			g.hotkeys = append(g.hotkeys, restored.hotkeys...)
		}
		if restoreErr != nil || restored == nil || len(restored.RegisteredCombineKeys()) != len(removed) {
			return fmt.Errorf("%w; previous hotkeys could not be restored", err)
		}
		return err
	}
	g.hotkeys = append(retained, added.hotkeys...)
	return nil
}
