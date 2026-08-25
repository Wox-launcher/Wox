package app

import (
	"context"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"strings"
	"wox/common"
	"wox/plugin"
	"wox/util"
)

const (
	appUninstallActionID          = "uninstall"
	appUninstallActionValue       = "uninstall"
	uninstallPathMatchScore       = 80
	uninstallNameMatchScore       = 40
	uninstallInstallLocationScore = 100
	uninstallDisplayIconScore     = 90
	uninstallSiblingDirScore      = 80
	uninstallExactNameScore       = 40
)

var (
	uninstallVersionSuffixPattern = regexp.MustCompile(`(?i)[\s\-_.]*v?\d+(?:\.\d+){1,3}\b.*$`)
	msiProductCodePattern         = regexp.MustCompile(`(?i)\{[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12}\}`)
	iconIndexSuffixPattern        = regexp.MustCompile(`^-?\d+$`)
)

type windowsUninstallEntry struct {
	DisplayName      string
	UninstallString  string
	InstallLocation  string
	DisplayIcon      string
	KeyName          string
	NoRemove         bool
	WindowsInstaller bool
}

func shouldOfferWindowsUninstall(info appInfo) bool {
	if info.Type == AppTypeWindowsSetting {
		return false
	}
	if info.Type == AppTypeUWP {
		return true
	}
	if info.Type != AppTypeDesktop {
		return false
	}

	extension := strings.ToLower(filepath.Ext(info.Path))
	return extension == ".exe" || extension == ".lnk"
}

func uwpPackageFamilyName(appPath string) string {
	appID := strings.TrimPrefix(appPath, "shell:AppsFolder\\")
	family, _, _ := strings.Cut(appID, "!")
	return strings.TrimSpace(family)
}

func splitUninstallCommand(command string) (file string, parameters string) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", ""
	}

	if command[0] == '"' {
		end := strings.Index(command[1:], "\"")
		if end >= 0 {
			return command[1 : 1+end], strings.TrimSpace(command[2+end:])
		}
	}

	// Unquoted UninstallString values often contain spaces, e.g.
	// C:\Program Files\Foo\unins000.exe. Split on the executable suffix
	// instead of the first space so "C:\Program" is not launched.
	if file, parameters, ok := splitUnquotedUninstallCommand(command); ok {
		return file, parameters
	}

	if index := strings.IndexByte(command, ' '); index >= 0 {
		return command[:index], strings.TrimSpace(command[index+1:])
	}

	return command, ""
}

func splitUnquotedUninstallCommand(command string) (file string, parameters string, ok bool) {
	lower := strings.ToLower(command)
	bestEnd := -1
	for _, ext := range []string{".exe", ".bat", ".cmd", ".com", ".msi"} {
		searchFrom := 0
		for {
			index := strings.Index(lower[searchFrom:], ext)
			if index < 0 {
				break
			}
			end := searchFrom + index + len(ext)
			if end == len(command) || command[end] == ' ' || command[end] == '\t' {
				if end > bestEnd {
					bestEnd = end
				}
			}
			searchFrom = searchFrom + index + 1
		}
	}
	if bestEnd < 0 {
		return "", "", false
	}

	return command[:bestEnd], strings.TrimSpace(command[bestEnd:]), true
}

func rewriteMsiUninstallCommand(file string, parameters string, windowsInstaller bool, keyName string) (string, string) {
	productCode := ""
	if match := msiProductCodePattern.FindString(parameters); match != "" {
		productCode = match
	} else if match := msiProductCodePattern.FindString(keyName); match != "" {
		productCode = match
	}

	isMsiExec := strings.EqualFold(windowsBaseName(file), "msiexec.exe") || strings.EqualFold(windowsBaseName(file), "msiexec")
	if productCode == "" || (!windowsInstaller && !isMsiExec) {
		return file, parameters
	}

	return "msiexec.exe", "/X" + productCode
}

func normalizeUninstallAppName(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = strings.ReplaceAll(normalized, "版本", " ")
	normalized = strings.ReplaceAll(normalized, "version", " ")
	normalized = uninstallVersionSuffixPattern.ReplaceAllString(normalized, "")
	return strings.Join(strings.Fields(normalized), " ")
}

func stripIconIndex(displayIcon string) string {
	displayIcon = strings.TrimSpace(displayIcon)
	displayIcon = strings.Trim(displayIcon, `"`)
	if index := strings.LastIndex(displayIcon, ","); index > 0 {
		suffix := strings.TrimSpace(displayIcon[index+1:])
		if iconIndexSuffixPattern.MatchString(suffix) {
			displayIcon = strings.TrimSpace(displayIcon[:index])
			displayIcon = strings.Trim(displayIcon, `"`)
		}
	}
	return canonicalizeWindowsPath(displayIcon)
}

func canonicalizeWindowsPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.Trim(path, `"`)
	if path == "" {
		return ""
	}
	path = strings.ReplaceAll(path, `\`, "/")
	return strings.TrimSuffix(strings.ToLower(pathpkg.Clean(path)), "/")
}

func windowsBaseName(path string) string {
	return pathpkg.Base(strings.ReplaceAll(strings.TrimSpace(path), `\`, "/"))
}

func windowsParentPath(path string) string {
	clean := canonicalizeWindowsPath(path)
	if clean == "" {
		return ""
	}
	parent := pathpkg.Dir(clean)
	if parent == "." || parent == "/" {
		return ""
	}
	return parent
}

func isPathUnderDirectory(filePath string, directory string) bool {
	cleanFile := canonicalizeWindowsPath(filePath)
	cleanDirectory := canonicalizeWindowsPath(directory)
	if cleanFile == "" || cleanDirectory == "" {
		return false
	}
	if cleanFile == cleanDirectory {
		return true
	}
	return strings.HasPrefix(cleanFile, cleanDirectory+"/")
}

func scoreUninstallMatch(entry windowsUninstallEntry, normalizedAppName string, targetPaths []string) int {
	score := 0

	installLocation := canonicalizeWindowsPath(entry.InstallLocation)
	iconPath := stripIconIndex(entry.DisplayIcon)
	uninstallFile, _ := splitUninstallCommand(entry.UninstallString)
	uninstallDir := windowsParentPath(uninstallFile)

	for _, targetPath := range targetPaths {
		cleanTarget := canonicalizeWindowsPath(targetPath)
		if cleanTarget == "" {
			continue
		}

		if installLocation != "" && isPathUnderDirectory(cleanTarget, installLocation) {
			score += uninstallInstallLocationScore
		}
		if iconPath != "" && iconPath == cleanTarget {
			score += uninstallDisplayIconScore
		}
		if uninstallDir != "" && uninstallDir == windowsParentPath(cleanTarget) {
			score += uninstallSiblingDirScore
		}
	}

	if normalizedAppName != "" && normalizedAppName == normalizeUninstallAppName(entry.DisplayName) {
		score += uninstallExactNameScore
	}

	return score
}

func findWindowsUninstallEntry(info appInfo, targetPaths []string, entries []windowsUninstallEntry) *windowsUninstallEntry {
	normalizedAppName := normalizeUninstallAppName(info.Name)
	var best *windowsUninstallEntry
	bestScore := 0
	bestCount := 0

	for i := range entries {
		entry := &entries[i]
		if entry.NoRemove || strings.TrimSpace(entry.UninstallString) == "" {
			continue
		}

		score := scoreUninstallMatch(*entry, normalizedAppName, targetPaths)
		if score <= 0 {
			continue
		}
		if score > bestScore {
			best = entry
			bestScore = score
			bestCount = 1
			continue
		}
		if score == bestScore {
			bestCount++
		}
	}

	if best == nil {
		return nil
	}
	if bestScore >= uninstallPathMatchScore {
		return best
	}
	if bestCount == 1 && bestScore >= uninstallNameMatchScore {
		return best
	}
	return nil
}

func (a *ApplicationPlugin) buildUninstallAction(info appInfo, displayName string, contextData map[string]string) (plugin.QueryResultAction, bool) {
	if !util.IsWindows() || !shouldOfferWindowsUninstall(info) {
		return plugin.QueryResultAction{}, false
	}

	actionContextData := common.ContextData{}
	for key, value := range contextData {
		actionContextData[key] = value
	}
	actionContextData["action"] = appUninstallActionValue

	return plugin.QueryResultAction{
		Id:          appUninstallActionID,
		Name:        "i18n:plugin_app_uninstall",
		Icon:        common.TrashIcon,
		ContextData: actionContextData,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			a.executeUninstall(ctx, info, displayName)
		},
	}, true
}

func (a *ApplicationPlugin) executeUninstall(ctx context.Context, info appInfo, displayName string) {
	// Hide before ShellExecute so the UAC prompt or vendor uninstaller is not covered by the launcher.
	a.api.HideApp(ctx)
	if err := executeWindowsUninstall(ctx, info); err != nil {
		a.api.Log(ctx, plugin.LogLevelError, "uninstall failed for "+displayName+": "+err.Error())
		if isWindowsUninstallNotFound(err) {
			a.api.Notify(ctx, a.api.GetTranslation(ctx, "plugin_app_uninstall_not_found"))
			return
		}
		if isWindowsUninstallNotAllowed(err) {
			a.api.Notify(ctx, a.api.GetTranslation(ctx, "plugin_app_uninstall_not_allowed"))
			return
		}
		a.api.Notify(ctx, fmtUninstallMessage(a.api.GetTranslation(ctx, "plugin_app_uninstall_failed"), err.Error()))
		return
	}

	a.api.Notify(ctx, a.api.GetTranslation(ctx, "plugin_app_uninstall_started"))
}

func fmtUninstallMessage(template string, value string) string {
	if strings.Contains(template, "%s") {
		return strings.Replace(template, "%s", value, 1)
	}
	if strings.TrimSpace(template) == "" {
		return value
	}
	return template
}
