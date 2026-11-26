package setting

import (
	"maps"
	"strings"
	"unicode"
	"wox/common"
)

// ResultBinding is a user-defined hotkey or alias for one restorable MRU result.
// Hash is assigned at create time with the plugin's MRU HashBy rules and is the
// durable identity for later restore, edit, and MRU updates.
type ResultBinding struct {
	Hash        string
	PluginID    string
	Title       string
	SubTitle    string
	Icon        common.WoxImage
	ContextData common.ContextData
	Hotkey      string
	Alias       string
}

// NormalizeResultAlias trims and lowercases a result alias for comparison.
func NormalizeResultAlias(alias string) string {
	return strings.ToLower(strings.TrimSpace(alias))
}

// IsSingleWordResultAlias reports whether alias is a non-empty token without whitespace.
func IsSingleWordResultAlias(alias string) bool {
	trimmed := strings.TrimSpace(alias)
	if trimmed == "" {
		return false
	}
	return strings.IndexFunc(trimmed, unicode.IsSpace) < 0
}

// MergeResultHotkeysAndAliases combines each binding by its durable result hash.
func MergeResultHotkeysAndAliases(hotkeys, aliases []ResultBinding) []ResultBinding {
	bindings := make([]ResultBinding, 0, len(hotkeys))
	byHash := make(map[string]int, len(hotkeys))
	for _, binding := range CloneResultBindings(hotkeys) {
		if binding.Hotkey != "" {
			binding.Alias = ""
			byHash[binding.Hash] = len(bindings)
			bindings = append(bindings, binding)
		}
	}
	for _, binding := range CloneResultBindings(aliases) {
		if binding.Alias == "" {
			continue
		}
		if index, ok := byHash[binding.Hash]; ok {
			bindings[index].Alias = binding.Alias
		} else {
			binding.Hotkey = ""
			bindings = append(bindings, binding)
		}
	}
	return bindings
}

// CloneResultBindings copies a binding slice so callers can edit without sharing storage.
func CloneResultBindings(bindings []ResultBinding) []ResultBinding {
	if len(bindings) == 0 {
		return nil
	}
	cloned := make([]ResultBinding, len(bindings))
	copy(cloned, bindings)
	for i := range cloned {
		cloned[i].ContextData = maps.Clone(cloned[i].ContextData)
	}
	return cloned
}

// FindResultBinding returns the binding with the given hash.
func FindResultBinding(bindings []ResultBinding, hash string) (ResultBinding, bool) {
	for _, binding := range bindings {
		if binding.Hash == hash {
			return binding, true
		}
	}
	return ResultBinding{}, false
}
