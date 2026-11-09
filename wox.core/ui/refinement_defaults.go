package ui

import (
	"context"
	"maps"

	"wox/plugin"
	"wox/ui/contract"
)

// prepareRefinementDefaults seeds only new entries into a resolved plugin command.
func prepareRefinementDefaults(previous contract.RefinementScope, query *plugin.Query, owner *plugin.Instance) contract.RefinementDefaults {
	if owner == nil || owner.Setting == nil {
		if previous.PluginID != "" {
			query.Refinements = map[string]string{}
		}
		return contract.RefinementDefaults{}
	}
	command := query.Command
	if len(query.Scope.Plugins) == 1 {
		command = query.Scope.Plugins[0].Command
	}
	scope := contract.RefinementScope{PluginID: owner.Metadata.Id, Command: command}
	state := contract.RefinementDefaults{Scope: scope}
	if scope == previous {
		return state
	}
	state.SavedValues, _ = owner.Setting.RefinementDefault(command)
	if previous.PluginID != "" {
		// Launcher refinements from another command have no meaning in this scope.
		query.Refinements = map[string]string{}
		state.AppliedValues = map[string]string{}
	}
	// Plugin navigation may carry explicit selections in ContextData.
	if state.SavedValues != nil && len(query.Refinements) == 0 && len(query.ContextData) == 0 {
		query.Refinements = maps.Clone(state.SavedValues)
		state.AppliedValues = state.SavedValues
		state.Restored = true
	}
	return state
}

func (s *CoreServices) SetRefinementDefault(_ context.Context, scope contract.RefinementScope, values map[string]string) error {
	return plugin.GetStoreManager().SetRefinementDefault(scope.PluginID, scope.Command, values)
}
