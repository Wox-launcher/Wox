package raycast

import (
	"path/filepath"
	"strings"

	"wox/plugin"
	"wox/setting"
)

func appPathFromCommand(id string) string {
	const marker = "::=::"
	index := strings.Index(id, marker)
	if index < 0 {
		return ""
	}
	return strings.TrimSpace(id[index+len(marker):])
}

// resolveApp reports whether path still points at an application on this machine.
func resolveApp(path string) (name string, ok bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", false
	}
	info, err := statApp(path)
	if err != nil {
		return "", false
	}
	if info.isDir && !strings.HasSuffix(strings.ToLower(path), ".app") {
		return "", false
	}
	name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return name, name != ""
}

func appQuery(name string) string {
	return "app " + strings.TrimSpace(name)
}

func firstAliasWord(alias string) string {
	fields := strings.Fields(alias)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func aliasAllowed(alias, query string, bindings []setting.ResultBinding) bool {
	if strings.TrimSpace(alias) == "" || strings.TrimSpace(query) == "" {
		return false
	}
	return plugin.ValidateQueryAliasAgainstResultAliases(setting.QueryAlias{Alias: alias, Query: query}, bindings) == nil
}
