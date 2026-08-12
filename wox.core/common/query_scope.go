package common

import "strings"

// QueryScope is a routing allowlist orthogonal to QueryType.
// It is a core-internal feature and is not part of the public plugin SDK.
// Empty Plugins means no scope (default keyword / selection-feature routing).
type QueryScope struct {
	Plugins []QueryScopePlugin `json:"Plugins,omitempty"`
}

// QueryScopePlugin pins one plugin, optionally locking a command.
// Core-internal only; not exposed through Node/Python plugin SDKs.
type QueryScopePlugin struct {
	PluginID string `json:"PluginId"`
	Command  string `json:"Command,omitempty"`
}

// IsEmpty reports whether the scope carries no plugins.
func (s QueryScope) IsEmpty() bool {
	return len(s.Plugins) == 0
}

// Clone returns a deep copy of the scope plugins slice.
func (s QueryScope) Clone() QueryScope {
	if len(s.Plugins) == 0 {
		return QueryScope{}
	}
	cloned := make([]QueryScopePlugin, len(s.Plugins))
	copy(cloned, s.Plugins)
	return QueryScope{Plugins: cloned}
}

// Deduplicate keeps the first entry for each PluginID and drops empty IDs.
func (s QueryScope) Deduplicate() QueryScope {
	if len(s.Plugins) == 0 {
		return QueryScope{}
	}
	seen := make(map[string]struct{}, len(s.Plugins))
	out := make([]QueryScopePlugin, 0, len(s.Plugins))
	for _, item := range s.Plugins {
		id := strings.TrimSpace(item.PluginID)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, QueryScopePlugin{PluginID: id, Command: item.Command})
	}
	return QueryScope{Plugins: out}
}

// NormalizeForRouting deduplicates valid entries without erasing an explicit invalid allowlist.
func (s QueryScope) NormalizeForRouting() QueryScope {
	normalized := s.Deduplicate()
	if !s.IsEmpty() && normalized.IsEmpty() {
		// Keeping the invalid entry makes the query non-global while scheduling no
		// plugins; silently returning an empty scope would broaden its routing.
		return s.Clone()
	}
	return normalized
}

// Identity is a stable string for UI state (refinements, layout, history).
func (s QueryScope) Identity() string {
	deduped := s.Deduplicate()
	if deduped.IsEmpty() {
		return ""
	}
	parts := make([]string, 0, len(deduped.Plugins))
	for _, item := range deduped.Plugins {
		if item.Command == "" {
			parts = append(parts, item.PluginID)
			continue
		}
		parts = append(parts, item.PluginID+"|"+item.Command)
	}
	return strings.Join(parts, ",")
}

// Find returns the first scoped entry for pluginID, if any.
func (s QueryScope) Find(pluginID string) (QueryScopePlugin, bool) {
	for _, item := range s.Deduplicate().Plugins {
		if item.PluginID == pluginID {
			return item, true
		}
	}
	return QueryScopePlugin{}, false
}

// Equal reports whether two scopes have the same ordered PluginID+Command list.
func (s QueryScope) Equal(other QueryScope) bool {
	return s.Identity() == other.Identity()
}
