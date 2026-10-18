package pathquery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// IsBrowseQuery recognizes existing paths and child-name prefixes inside existing directories.
func IsBrowseQuery(input string) bool {
	path, _, ok := Parse(input)
	if !ok {
		return false
	}
	if _, err := os.Stat(path); err == nil {
		return true
	}
	// The final component may still be a name prefix while its parent is browsable.
	parent, err := os.Stat(filepath.Dir(path))
	return err == nil && parent.IsDir()
}

// Parse expands a typed filesystem path and reports whether it ends in a browsing separator.
func Parse(input string) (path string, shouldListChildren bool, ok bool) {
	trimmedInput := strings.TrimSpace(input)
	if trimmedInput == "" || !isPathLike(trimmedInput) {
		return "", false, false
	}

	shouldListChildren = hasTrailingSeparator(trimmedInput)
	lookupPath := trimTrailingSeparators(trimmedInput)
	expandedPath, expandErr := Expand(lookupPath)
	if expandErr != nil {
		return "", false, false
	}

	return filepath.Clean(expandedPath), shouldListChildren, true
}

// isPathLike filters ordinary search text before any filesystem stat.
func isPathLike(input string) bool {
	if input == "~" || strings.HasPrefix(input, "~/") || strings.HasPrefix(input, `~\`) {
		return true
	}
	if _, rest, ok := splitWindowsEnvPrefix(input); ok {
		return rest == "" || strings.HasPrefix(rest, `\`) || strings.HasPrefix(rest, "/")
	}
	return filepath.IsAbs(input)
}

// hasTrailingSeparator detects the browsing mode requested by the user.
func hasTrailingSeparator(input string) bool {
	return strings.HasSuffix(input, "/") || strings.HasSuffix(input, `\`)
}

// trimTrailingSeparators removes browsing separators without breaking roots.
func trimTrailingSeparators(input string) string {
	for hasTrailingSeparator(input) {
		next := input[:len(input)-1]
		if next == "" || isWindowsVolumeRoot(next) {
			break
		}
		input = next
	}
	return input
}

// isWindowsVolumeRoot keeps inputs like C:\ from becoming C: during trimming.
func isWindowsVolumeRoot(path string) bool {
	volumeName := filepath.VolumeName(path)
	return volumeName != "" && strings.EqualFold(volumeName, path)
}

// Expand resolves Windows %VAR% values and ~ before filesystem checks.
func Expand(path string) (string, error) {
	expandedEnv, expandErr := expandEnvVars(path)
	if expandErr != nil {
		return "", expandErr
	}
	return expandHome(expandedEnv)
}

// expandEnvVars expands %LOCALAPPDATA% and other Windows-style env vars.
func expandEnvVars(path string) (string, error) {
	if !strings.Contains(path, "%") {
		return path, nil
	}

	var builder strings.Builder
	builder.Grow(len(path))
	for i := 0; i < len(path); {
		name, _, ok := splitWindowsEnvPrefix(path[i:])
		if !ok {
			builder.WriteByte(path[i])
			i++
			continue
		}

		value, found := os.LookupEnv(name)
		if !found || strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("environment variable %%%s%% is not set", name)
		}
		builder.WriteString(value)
		i += len(name) + 2
	}
	return builder.String(), nil
}

// splitWindowsEnvPrefix reports a leading %VAR% token used by Windows path input.
func splitWindowsEnvPrefix(input string) (name string, rest string, ok bool) {
	if !strings.HasPrefix(input, "%") {
		return "", "", false
	}

	end := strings.IndexByte(input[1:], '%')
	if end <= 0 {
		return "", "", false
	}

	name = input[1 : 1+end]
	if !isEnvVarName(name) {
		return "", "", false
	}
	return name, input[end+2:], true
}

// isEnvVarName accepts Windows env names such as LOCALAPPDATA and ProgramFiles(x86).
func isEnvVarName(name string) bool {
	if name == "" {
		return false
	}

	for _, r := range name {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '(', r == ')':
			continue
		default:
			return false
		}
	}
	return true
}

// expandHome resolves ~ paths before filesystem checks.
func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") && !strings.HasPrefix(path, `~\`) {
		return path, nil
	}

	homeDir, homeErr := os.UserHomeDir()
	if homeErr != nil {
		return "", homeErr
	}
	if path == "~" {
		return homeDir, nil
	}

	return filepath.Join(homeDir, path[2:]), nil
}

// EnsureTrailingSeparator builds the follow-up query for entering a folder.
func EnsureTrailingSeparator(path string) string {
	if hasTrailingSeparator(path) {
		return path
	}
	return path + string(os.PathSeparator)
}
