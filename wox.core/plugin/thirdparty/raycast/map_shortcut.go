package raycast

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"wox/common"
	"wox/database"
	"wox/plugin"
	"wox/setting"
	"wox/util"
)

// logRaycastImport writes import diagnostics only after user data is initialized.
// Unit tests have no data directory, so they stay silent and do not create a log folder.
func logRaycastImport(format string, args ...any) {
	if util.GetLocation().GetWoxDataDirectory() == "" {
		return
	}
	util.GetLogger().Info(context.Background(), fmt.Sprintf(format, args...))
}

// mapShortcutRows turns Raycast shortcut commands into one result binding each.
// Clipboard history and emoji stay query hotkeys.
func mapShortcutRows(payload Payload, quicklinks []mappedQuicklink) []mappedShortcut {
	apps := raycastAppIndex()
	settings := settingsApplications()
	logRaycastImport(
		"raycast import: commands=%d indexedAppShortcuts=%d controlPanelApps=%d pluginsQueryable=%v",
		len(payload.Settings.Commands), len(apps), len(settings), pluginsQueryable(),
	)
	var rows []mappedShortcut
	for index, command := range payload.Settings.Commands {
		if controlPanelPreferenceGUID(command) != "" || strings.Contains(strings.ToLower(command.ExtensionID), "preference") {
			alias, hotkey, bound := shortcutBinding(command)
			logRaycastImport(
				"raycast import: preference command index=%d extension=%q id=%q alias=%q hotkey=%q bound=%v guid=%q",
				index, command.ExtensionID, command.ID, alias, hotkey, bound, controlPanelPreferenceGUID(command),
			)
		}
		row, ok := shortcutForCommand(index, command, quicklinks, apps, settings)
		if !ok {
			continue
		}
		resolveShortcut(&row)
		if controlPanelPreferenceGUID(command) != "" {
			logRaycastImport(
				"raycast import: preference row title=%q selectable=%v status=%q search=%q path=%q",
				row.title, row.selectable, row.status, row.search, row.path,
			)
		}
		rows = append(rows, row)
	}
	return rows
}

func shortcutIndexes(rows []mappedShortcut) map[int]struct{} {
	claimed := map[int]struct{}{}
	for _, row := range rows {
		claimed[row.commandIndex] = struct{}{}
	}
	return claimed
}

func shortcutForCommand(index int, command Command, quicklinks []mappedQuicklink, apps map[string]plugin.DesktopApp, settings []plugin.DesktopApp) (mappedShortcut, bool) {
	alias, hotkey, ok := shortcutBinding(command)
	if !ok {
		return mappedShortcut{}, false
	}
	row := mappedShortcut{
		id: "setting:result_binding:" + strconv.Itoa(index), commandIndex: index,
		title: commandTitle(command), alias: alias, hotkey: hotkey, selectable: true,
		detail: shortcutDetail(alias, hotkey),
	}
	switch {
	case command.ExtensionID == "e:r:applications":
		path := appPathFromCommand(command.ID)
		if app, found := matchIndexedRaycastApp(path, apps); found {
			if !namedCommand(command) {
				row.title = app.Name
			}
			row.pluginID = common.AppPluginID
			row.search = row.title
			row.path = app.Path
			row.identityKey = app.Path
			break
		}
		name, found := resolveApp(path)
		if !found {
			return mappedShortcut{}, false
		}
		if strings.TrimSpace(row.title) == "" {
			row.title = name
		}
		row.pluginID = common.AppPluginID
		row.search = row.title
		row.path = path
		row.identityKey = path
	case isWoxSettingsCommand(command):
		row.pluginID = common.SysPluginID
		row.command = "open_wox_settings"
		row.search = row.command
		row.identityKey = row.command
		if !namedCommand(command) {
			row.title = "Settings"
		}
	case isMediaToggleCommand(command):
		row.pluginID = common.MediaPlayerPluginID
		row.search = "playpause"
		row.identityKey = common.MediaPlayerToggleIdentity
		if !namedCommand(command) {
			row.title = "Play / Pause"
		}
	case isSystemExtension(command.ExtensionID):
		commandID, found := sysCommandID(command)
		if !found {
			return mappedShortcut{}, false
		}
		row.pluginID = common.SysPluginID
		row.command = commandID
		row.search = commandID
		row.identityKey = commandID
		if !namedCommand(command) {
			row.title = splitCamel(commandToken(command.ID))
		}
	case isControlPanelPreference(command):
		guid := controlPanelPreferenceGUID(command)
		app, found := matchSettingsAppByGUID(guid, settings)
		logRaycastImport(
			"raycast import: match control panel guid=%q settingsApps=%d found=%v name=%q",
			guid, len(settings), found, app.Name,
		)
		if !found {
			return mappedShortcut{}, false
		}
		if !namedCommand(command) {
			row.title = app.Name
		}
		row.pluginID = common.AppPluginID
		row.search = app.Name
		row.path = app.Path
		row.identityKey = app.Path
	case windowsSettingsURI(command) != "" || (isWindowsSettingsExtension(command.ExtensionID) && pluginsQueryable()):
		uri := windowsSettingsURI(command)
		row.pluginID = common.AppPluginID
		row.search = row.title
		row.path = uri
		row.identityKey = uri
		if uri == "" {
			row.matchTitle = row.title
			row.requireSettings = true
		}
	default:
		link, found := matchingQuicklink(command, quicklinks)
		if !found {
			return mappedShortcut{}, false
		}
		row.prerequisiteID = link.id
		row.prerequisiteName = link.title
		if !namedCommand(command) {
			row.title = link.title
		}
		if link.favorite {
			row.pluginID = common.FolderPluginID
			row.search = link.title
			row.path = link.path
			row.identityKey = link.path
			row.prerequisiteFavorite = true
			row.prerequisitePath = link.path
		} else {
			row.pluginID = common.WebSearchPluginID
			row.trigger = link.keyword
			row.keyword = link.keyword
			row.prerequisiteKeyword = link.keyword
			row.prerequisiteURL = link.url
			row.prerequisiteBrowser = link.browser
		}
	}
	return row, row.pluginID != ""
}

func shortcutBinding(command Command) (alias, hotkey string, ok bool) {
	alias = firstAliasWord(command.Alias)
	if alias != "" && !setting.IsSingleWordResultAlias(alias) {
		alias = ""
	}
	if raw := commandHotkey(command); len(raw) > 0 {
		if value, _, _, mapped := mapHotkey(raw); mapped {
			hotkey = value
		}
	}
	return alias, hotkey, alias != "" || hotkey != ""
}

func shortcutDetail(alias, hotkey string) string {
	var parts []string
	if alias != "" {
		parts = append(parts, alias)
	}
	if hotkey != "" {
		parts = append(parts, hotkeyLabel(hotkey))
	}
	return strings.Join(parts, "  ")
}

func isWoxSettingsCommand(command Command) bool {
	extension := strings.ToLower(strings.TrimSpace(command.ExtensionID))
	if strings.Contains(extension, "windows") {
		return false
	}
	switch extension {
	case "e:r:raycast", "e:r:raycast-core", "e:r:settings", "raycast":
	default:
		return false
	}
	for _, value := range []string{commandToken(command.ID), command.Name, command.Title} {
		switch compactCommandKey(value) {
		case "settings", "opensettings":
			return true
		}
	}
	return false
}

func namedCommand(command Command) bool {
	return strings.TrimSpace(command.Name) != "" || strings.TrimSpace(command.Title) != ""
}

// commandToken is the command key at the end of a Raycast command id.
func commandToken(id string) string {
	const marker = "::-::"
	if index := strings.LastIndex(id, marker); index >= 0 {
		return strings.TrimSpace(id[index+len(marker):])
	}
	return strings.TrimSpace(id)
}

func compactCommandKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.NewReplacer("-", "", "_", "", " ", "").Replace(value)
}

func splitCamel(token string) string {
	var words []string
	var current []rune
	flush := func() {
		if len(current) == 0 {
			return
		}
		word := string(current)
		words = append(words, strings.ToUpper(word[:1])+word[1:])
		current = nil
	}
	for _, r := range token {
		if unicode.IsUpper(r) && len(current) > 0 {
			flush()
		}
		current = append(current, unicode.ToLower(r))
	}
	flush()
	if len(words) == 0 {
		return token
	}
	return strings.Join(words, " ")
}

func isSystemExtension(id string) bool {
	id = strings.ToLower(strings.TrimSpace(id))
	switch id {
	case "e:r:system", "e:r:system-commands", "e:r:builtin-system":
		return true
	}
	return strings.HasPrefix(id, "e:r:") && strings.Contains(id, "system") && !strings.Contains(id, "windows")
}

func isWindowsSettingsExtension(id string) bool {
	id = strings.ToLower(strings.TrimSpace(id))
	return strings.Contains(id, "windows") && (strings.Contains(id, "setting") || strings.HasPrefix(id, "e:r:"))
}

// isControlPanelPreference reports a Raycast Windows Settings row stored as a Control Panel preference.
func isControlPanelPreference(command Command) bool {
	extension := strings.ToLower(strings.TrimSpace(command.ExtensionID))
	if !strings.Contains(extension, "preference") {
		return false
	}
	return controlPanelPreferenceGUID(command) != ""
}

// controlPanelPreferenceGUID reads the shell item GUID from a preference id.
// Raycast stores the full shell path after /ControlPanel/, for example
// /ControlPanel/::{26EE0668-...}\0\::{ED7BA470-...}\{TASK-GUID}. The item identity is the last GUID.
// A short /ControlPanel/{GUID} id uses that only GUID.
func controlPanelPreferenceGUID(command Command) string {
	for _, value := range []string{command.ID, command.Path, command.Link, command.Deeplink} {
		if guid := controlPanelGUID(value); guid != "" {
			return guid
		}
	}
	return ""
}

func controlPanelGUID(value string) string {
	const marker = "/controlpanel/"
	lower := strings.ToLower(value)
	index := strings.Index(lower, marker)
	if index < 0 {
		return ""
	}
	rest := value[index+len(marker):]
	var found string
	for {
		start := strings.IndexByte(rest, '{')
		if start < 0 {
			break
		}
		rest = rest[start+1:]
		end := strings.IndexByte(rest, '}')
		if end < 0 {
			break
		}
		parsed, err := uuid.Parse(rest[:end])
		if err == nil {
			found = parsed.String()
		}
		rest = rest[end+1:]
	}
	return found
}

// matchSettingsAppByGUID finds the indexed Control Panel item whose path contains the preference GUID.
func matchSettingsAppByGUID(guid string, apps []plugin.DesktopApp) (plugin.DesktopApp, bool) {
	guid = strings.ToLower(strings.TrimSpace(guid))
	if guid == "" {
		return plugin.DesktopApp{}, false
	}
	needle := "{" + guid + "}"
	for _, app := range apps {
		if strings.Contains(strings.ToLower(app.Path), needle) {
			return app, true
		}
	}
	return plugin.DesktopApp{}, false
}

// isMediaToggleCommand maps Raycast Play/Pause onto the media plugin's toggle result.
func isMediaToggleCommand(command Command) bool {
	if !isSystemExtension(command.ExtensionID) {
		return false
	}
	for _, value := range []string{commandToken(command.ID), command.Name, command.Title} {
		switch compactMediaKey(value) {
		case "playpause", "toggle":
			return true
		}
	}
	return false
}

func compactMediaKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.NewReplacer("-", "", "_", "", " ", "", "/", "").Replace(value)
}

func sysCommandID(command Command) (string, bool) {
	for _, value := range []string{commandToken(command.ID), command.Name, command.Title} {
		if id, ok := sysCommands[compactCommandKey(value)]; ok {
			return id, true
		}
	}
	return "", false
}

var sysCommands = map[string]string{
	"volumeup": "volume-up", "turnvolumeup": "volume-up",
	"volumedown": "volume-down", "turnvolumedown": "volume-down",
	"togglemute": "toggle-mute", "mute": "toggle-mute",
	"emptytrash":   "empty_trash",
	"lock":         "lock_computer",
	"lockcomputer": "lock_computer",
	"sleep":        "sleep",
	"shutdown":     "shutdown_computer",
	"restart":      "restart_computer",
	"logout":       "log-out",
	"showdesktop":  "show-desktop",
}

func windowsSettingsURI(command Command) string {
	best := ""
	for _, value := range []string{command.ID, command.Name, command.Title, command.Link, command.Path, command.Deeplink} {
		if uri := longestSettingsURI(value); len(uri) > len(best) {
			best = uri
		}
	}
	return best
}

func longestSettingsURI(value string) string {
	lower := strings.ToLower(value)
	best := ""
	for uri := range knownWindowsSettingsURIs {
		index := strings.Index(lower, uri)
		if index < 0 || len(uri) <= len(best) {
			continue
		}
		end := index + len(uri)
		if end < len(lower) {
			next := lower[end]
			if next == '-' || next == ':' || (next >= 'a' && next <= 'z') || (next >= '0' && next <= '9') {
				continue
			}
		}
		best = uri
	}
	return best
}

var knownWindowsSettingsURIs = map[string]struct{}{
	"ms-settings:system": {}, "ms-settings:display": {}, "ms-settings:sound": {},
	"ms-settings:notifications": {}, "ms-settings:quiethours": {}, "ms-settings:powersleep": {},
	"ms-settings:storagesense": {}, "ms-settings:crossdevice": {}, "ms-settings:multitasking": {},
	"ms-settings:developers": {}, "ms-settings:activation": {}, "ms-settings:troubleshoot": {},
	"ms-settings:recovery": {}, "ms-settings:project": {}, "ms-settings:remotedesktop": {},
	"ms-settings:clipboard": {}, "ms-settings:about": {}, "ms-settings:bluetooth": {},
	"ms-settings:network": {}, "ms-settings:personalization": {}, "ms-settings:appsfeatures": {},
	"ms-settings:yourinfo": {}, "ms-settings:time-language": {}, "ms-settings:gaming-gamebar": {},
	"ms-settings:easeofaccess": {}, "ms-settings:privacy": {}, "ms-settings:windowsupdate": {},
}

func matchingQuicklink(command Command, rows []mappedQuicklink) (mappedQuicklink, bool) {
	extension := strings.ToLower(command.ExtensionID)
	name := strings.ToLower(strings.TrimSpace(command.Name))
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(command.Title))
	}
	id := strings.TrimSpace(command.ID)
	for _, row := range rows {
		if !row.selectable {
			continue
		}
		nameMatch := name != "" && strings.EqualFold(name, row.title)
		idMatch := id != "" && strings.EqualFold(id, row.title)
		sourceID := appPathFromCommand(command.ID)
		sourceMatch := sourceID != "" && sourceID == row.sourceID
		if strings.Contains(extension, "quicklink") && (nameMatch || idMatch || sourceMatch) {
			return row, true
		}
		if idMatch && nameMatch {
			return row, true
		}
	}
	return mappedQuicklink{}, false
}

func pluginsQueryable() bool {
	return database.GetDB() != nil && plugin.GetPluginManager() != nil
}

// resolveShortcut hides a row when the live plugin result is missing, already bound, or not executable.
// A quicklink or favorite that has not been written yet stays listed so import can create it first.
func resolveShortcut(row *mappedShortcut) {
	if row == nil || !row.selectable || !pluginsQueryable() {
		return
	}
	manager := plugin.GetPluginManager()
	query := plugin.ResultBindingQuery{Search: row.search, Command: row.command, TriggerKeyword: row.trigger}.PluginQuery()
	results, err := manager.QueryPlugin(context.Background(), row.pluginID, query)
	if err != nil {
		logRaycastImport(
			"raycast import: query failed plugin=%s search=%q err=%v", row.pluginID, row.search, err,
		)
		if row.prerequisiteID == "" {
			row.selectable = false
			row.status = statusUnsupported
		}
		return
	}
	picked, ok := plugin.SelectQueryResult(results, plugin.ResultBindingSelector{
		IdentityKey: row.identityKey, Path: row.path, Title: row.matchTitle, Keyword: row.keyword,
		RequireSettingsPath: row.requireSettings,
	})
	if !ok && row.trigger != "" && len(results) > 0 {
		picked = results[0]
		ok = true
	}
	if !ok {
		logRaycastImport(
			"raycast import: query missed plugin=%s search=%q results=%d identity=%q path=%q",
			row.pluginID, row.search, len(results), row.identityKey, row.path,
		)
		if row.prerequisiteID == "" {
			row.selectable = false
			row.status = statusUnsupported
		}
		return
	}
	status, previewErr := manager.PreviewResultBinding(context.Background(), row.pluginID, query, picked, row.alias, row.hotkey)
	if status != "ready" {
		logRaycastImport(
			"raycast import: preview plugin=%s title=%q status=%s err=%v alias=%q hotkey=%q",
			row.pluginID, picked.Title, status, previewErr, row.alias, row.hotkey,
		)
	}
	switch status {
	case "exists":
		row.selectable = false
		row.status = statusExists
	case "conflict":
		row.selectable = false
		row.status = statusConflict
	case "ready":
	default:
		if row.prerequisiteID == "" {
			row.selectable = false
			row.status = statusUnsupported
		}
	}
}
