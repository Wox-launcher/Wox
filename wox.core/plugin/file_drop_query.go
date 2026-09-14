package plugin

import (
	"context"

	"wox/common"
	"wox/util/selection"

	"github.com/google/uuid"
)

// NewGlobalFileDropQuery builds an unscoped file selection query.
func NewGlobalFileDropQuery(paths []string) common.PlainQuery {
	return common.PlainQuery{
		QueryId:   uuid.NewString(),
		QueryType: QueryTypeSelection,
		QuerySelection: selection.Selection{
			Type:      selection.SelectionTypeFile,
			FilePaths: append([]string(nil), paths...),
		},
		QueryRefinements: map[string]string{},
		ContextData:      common.ContextData{},
	}
}

// BuildFileDropQuery routes dropped files using the current explicit plugin target.
// UI must not infer a target from the highlighted result list.
//
// A plugin that already owns the query and declares querySelection receives a
// selection query while the visible QueryText is left as typed. Only an
// unsupported target falls back to a global selection that replaces the query box.
func (m *Manager) BuildFileDropQuery(_ context.Context, current common.PlainQuery, paths []string) common.PlainQuery {
	cleaned := append([]string(nil), paths...)
	scope := current.QueryScope.NormalizeForRouting()
	if !scope.IsEmpty() {
		if len(scope.Deduplicate().Plugins) == 1 {
			owner := m.GetPluginInstanceById(scope.Deduplicate().Plugins[0].PluginID)
			if shouldFallbackUnsupportedSelection(owner) {
				return NewGlobalFileDropQuery(cleaned)
			}
		}
		return newTargetedFileDropQuery(m.retainValidScopeCommands(scope), current.QueryText, cleaned)
	}

	parsed, owner := newQueryInputWithPlugins(current.QueryText, m.GetPluginInstances())
	if owner == nil || parsed.TriggerKeyword == "" {
		return NewGlobalFileDropQuery(cleaned)
	}
	if shouldFallbackUnsupportedSelection(owner) {
		return NewGlobalFileDropQuery(cleaned)
	}
	if pluginInstanceDisabled(owner) {
		scoped := common.QueryScope{Plugins: []common.QueryScopePlugin{{
			PluginID: owner.Metadata.Id,
			Command:  retainValidPluginCommand(owner, parsed.Command),
		}}}
		return newTargetedFileDropQuery(scoped, parsed.Search, cleaned)
	}
	return newTargetedFileDropQuery(common.QueryScope{}, current.QueryText, cleaned)
}

func shouldFallbackUnsupportedSelection(instance *Instance) bool {
	// Missing, unloaded, or disabled plugins keep existing protection and must
	// not be rewritten as a global selection fallback.
	return instance != nil && !pluginInstanceDisabled(instance) && !instance.Metadata.IsSupportFeature(MetadataFeatureQuerySelection)
}

func newTargetedFileDropQuery(scope common.QueryScope, text string, paths []string) common.PlainQuery {
	query := NewGlobalFileDropQuery(paths)
	query.QueryText = text
	query.QueryScope = scope.Clone()
	return query
}

func (m *Manager) retainValidScopeCommands(scope common.QueryScope) common.QueryScope {
	cloned := scope.Clone()
	for index, item := range cloned.Plugins {
		cloned.Plugins[index].Command = retainValidPluginCommand(m.GetPluginInstanceById(item.PluginID), item.Command)
	}
	return cloned
}

func retainValidPluginCommand(instance *Instance, command string) string {
	if instance == nil || command == "" {
		return ""
	}
	for _, item := range instance.GetQueryCommands() {
		if item.Command == command {
			return command
		}
	}
	return ""
}
