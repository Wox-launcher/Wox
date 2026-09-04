package launcher

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"time"

	"wox/util"
)

const formTableIgnoreRuleAppIconLimit = 6

type formTablePatternPreviewState struct {
	unchecked     map[string]bool
	savedApps     []ignoredHotkeyApp
	seedFromSaved bool
	apps          []formTablePatternPreviewApp
	matches       []ignoredHotkeyApp
	pattern       string
	requested     bool
	loading       bool
	errorText     string
	request       uint64
}

func snapshotFormTablePatternPreviewLocked(state *formTablePatternPreviewState) *formTablePatternPreviewSnapshot {
	if state == nil {
		return nil
	}
	return &formTablePatternPreviewSnapshot{Apps: append([]formTablePatternPreviewApp(nil), state.apps...), Loading: state.loading, ErrorText: state.errorText}
}

type formTablePatternPreviewSnapshot struct {
	Apps      []formTablePatternPreviewApp
	Loading   bool
	ErrorText string
}

type formTablePatternPreviewApp struct {
	Key     string
	Name    string
	Path    string
	Icon    woxImage
	Checked bool
}

func formTableHasPatternPreview(definition formDefinition) bool {
	for _, column := range definition.Value.Columns {
		if column.PreviewMatchedApps {
			return true
		}
	}
	return false
}

func formTableAppMatchKey(appPath, identity string) string {
	if key := formTableAppPathMatchKey(appPath); key != "" {
		return "path:" + key
	}
	if identity = strings.ToLower(strings.TrimSpace(identity)); identity != "" {
		return "identity:" + identity
	}
	return ""
}

func formTableAppPathMatchKey(appPath string) string {
	appPath = strings.TrimSpace(appPath)
	if appPath == "" {
		return ""
	}
	cleaned := filepath.Clean(appPath)
	if util.IsWindows() {
		return strings.ToLower(cleaned)
	}
	return cleaned
}

// formTableIgnoreRuleIncludeFuture reports whether the row hides every current and future match.
func formTableIgnoreRuleIncludeFuture(row map[string]any) bool {
	if row == nil {
		return false
	}
	switch value := row["IncludeFuture"].(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(strings.TrimSpace(value), "true")
	default:
		return false
	}
}

// formTableIgnoreRuleApps reads the fixed app list stored on an IgnoreRules row.
func formTableIgnoreRuleApps(row map[string]any) []ignoredHotkeyApp {
	if row == nil {
		return nil
	}
	raw, ok := row["Apps"]
	if !ok || raw == nil {
		return nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var apps []ignoredHotkeyApp
	if err := json.Unmarshal(encoded, &apps); err != nil {
		return nil
	}
	filtered := make([]ignoredHotkeyApp, 0, len(apps))
	for _, app := range apps {
		if strings.TrimSpace(app.Name) == "" && strings.TrimSpace(app.Identity) == "" && strings.TrimSpace(app.Path) == "" {
			continue
		}
		filtered = append(filtered, app)
	}
	return filtered
}

func formTableIgnoreRuleAppIconOverflow(total, limit int) int {
	if limit <= 0 || total <= limit {
		return 0
	}
	return total - limit
}

// formTableIgnoreRuleAppPathTooltip is the hover text for one Apps-column icon.
func formTableIgnoreRuleAppPathTooltip(app ignoredHotkeyApp) string {
	if path := strings.TrimSpace(app.Path); path != "" {
		return path
	}
	if identity := strings.TrimSpace(app.Identity); identity != "" {
		return identity
	}
	return strings.TrimSpace(app.Name)
}

// initFormTablePatternPreview opens the live match list and defaults new rules to IncludeFuture.
func (a *App) initFormTablePatternPreview(state *formTableEditorState) {
	if state == nil || state.rowForm == nil {
		return
	}
	if state.rowIndex < 0 && len(state.rowBase) == 0 {
		state.rowForm.values["IncludeFuture"] = "true"
	}
	preview := &formTablePatternPreviewState{unchecked: map[string]bool{}}
	if state.rowForm.values["IncludeFuture"] != "true" {
		preview.savedApps = formTableIgnoreRuleApps(state.rowBase)
		preview.seedFromSaved = len(preview.savedApps) > 0
	}
	state.patternPreview = preview
	a.refreshFormTablePatternPreview(state)
}

func formTablePatternPreviewIncludeFuture(state *formTableEditorState) bool {
	return state != nil && state.rowForm != nil && state.rowForm.values["IncludeFuture"] == "true"
}

func (a *App) refreshFormTablePatternPreview(state *formTableEditorState) {
	if state == nil || state.patternPreview == nil || state.rowForm == nil {
		return
	}
	pattern := strings.TrimSpace(state.rowForm.values["Pattern"])
	includeFuture := formTablePatternPreviewIncludeFuture(state)
	preview := state.patternPreview
	if !preview.requested || preview.pattern != pattern {
		a.loadFormTablePatternPreview(state, pattern)
		return
	}
	matches := preview.matches
	if state.patternPreview.seedFromSaved && !preview.loading && preview.errorText == "" {
		saved := map[string]bool{}
		for _, app := range state.patternPreview.savedApps {
			if key := formTableAppMatchKey(app.Path, app.Identity); key != "" {
				saved[key] = true
			}
		}
		for _, app := range matches {
			key := formTableAppMatchKey(app.Path, app.Identity)
			if key != "" && !saved[key] {
				state.patternPreview.unchecked[key] = true
			}
		}
		state.patternPreview.seedFromSaved = false
	}
	apps := make([]formTablePatternPreviewApp, 0, len(matches))
	for _, app := range matches {
		key := formTableAppMatchKey(app.Path, app.Identity)
		apps = append(apps, formTablePatternPreviewApp{
			Key: key, Name: app.Name, Path: app.Path, Icon: app.Icon,
			Checked: includeFuture || !state.patternPreview.unchecked[key],
		})
	}
	state.patternPreview.apps = apps
}

func (a *App) toggleFormTablePatternPreviewApp(key string, checked bool) {
	state := a.activeFormTableEditor()
	if !formTablePatternPreviewToggle(state, key, checked) {
		return
	}
	a.refreshFormTablePatternPreview(state)
	clearFormTableRowValidationLocked(state)
	a.invalidateFormTableWindow()
}

// formTablePatternPreviewToggle updates one preview checkbox. Unchecking any app
// turns IncludeFuture off so later saves persist the remaining selection.
func formTablePatternPreviewToggle(state *formTableEditorState, key string, checked bool) bool {
	if state == nil || state.patternPreview == nil || state.rowForm == nil || strings.TrimSpace(key) == "" {
		return false
	}
	if state.patternPreview.unchecked == nil {
		state.patternPreview.unchecked = map[string]bool{}
	}
	if checked {
		delete(state.patternPreview.unchecked, key)
	} else {
		state.rowForm.values["IncludeFuture"] = "false"
		state.patternPreview.unchecked[key] = true
	}
	return true
}

// loadFormTablePatternPreview asks core to match its complete search candidates.
// Each editor owns its request; late results cannot overwrite a newer pattern or editor.
func (a *App) loadFormTablePatternPreview(state *formTableEditorState, pattern string) {
	preview := state.patternPreview
	preview.request++
	request := preview.request
	preview.pattern, preview.requested = pattern, true
	preview.matches, preview.apps = nil, nil
	preview.errorText = ""
	preview.loading = pattern != "" && a.services != nil
	if !preview.loading {
		return
	}
	util.Go(a.lifecycleCtx, "load ignore rule preview", func() {
		ctx, cancel := context.WithTimeout(a.lifecycleCtx, 10*time.Second)
		defer cancel()
		loaded, err := a.services.IndexedApps(ctx, a.sessionID, pattern)
		_ = a.runOnUI("apply ignore rule preview", func() {
			if a.activeFormTableEditor() != state || state.patternPreview != preview || preview.request != request {
				return
			}
			preview.loading = false
			if err != nil {
				preview.errorText = a.translate("i18n:plugin_app_ignore_rule_preview_failed")
				util.GetLogger().Warn(ctx, "failed to load ignore rule preview: "+err.Error())
			} else {
				for _, app := range loaded {
					preview.matches = append(preview.matches, ignoredHotkeyApp{Name: app.Name, Identity: app.Identity, Path: app.Path, Icon: woxImage{ImageType: app.Icon.ImageType, ImageData: app.Icon.ImageData}})
				}
				a.refreshFormTablePatternPreview(state)
			}
			a.invalidateFormTableWindow()
		})
	})
}

func (a *App) applyFormTableIgnoreRuleSave(state *formTableEditorState) bool {
	if state == nil || state.rowForm == nil {
		return false
	}
	if !formTableHasPatternPreview(state.definition) {
		a.replaceFormTableEditorRows(state, []map[string]any{formTableRowFromFields(state.definition, state.rowForm, state.rowBase)})
		return true
	}

	pattern := strings.TrimSpace(state.rowForm.values["Pattern"])
	includeFuture := formTablePatternPreviewIncludeFuture(state)
	preview := state.patternPreview
	if preview == nil {
		return false
	}
	if !includeFuture && (preview.loading || preview.errorText != "") {
		state.status = preview.errorText
		if preview.loading {
			state.status = a.translate("i18n:ui_hotkey_ignore_apps_loading")
		}
		a.invalidateFormTableWindow()
		return false
	}
	matches := preview.matches
	row, errKey := formTableIgnoreRuleSaveRow(pattern, includeFuture, state.patternPreview, matches)
	if errKey != "" {
		state.status = a.translate(errKey)
		a.invalidateFormTableWindow()
		return false
	}
	base := formTableRowFromFields(state.definition, state.rowForm, state.rowBase)
	for key, value := range row {
		base[key] = value
	}
	if includeFuture {
		delete(base, "Apps")
	}
	a.replaceFormTableEditorRows(state, []map[string]any{base})
	return true
}

func (a *App) replaceFormTableEditorRows(state *formTableEditorState, rows []map[string]any) {
	if state.rowIndex >= 0 && state.rowIndex < len(state.rows) {
		before := append([]map[string]any{}, state.rows[:state.rowIndex]...)
		after := append([]map[string]any{}, state.rows[state.rowIndex+1:]...)
		state.rows = append(append(before, rows...), after...)
		state.selected = state.rowIndex + len(rows) - 1
		return
	}
	state.rows = append(state.rows, rows...)
	state.selected = len(state.rows) - 1
}

// formTableIgnoreRuleSaveRow writes one IgnoreRules row: a dynamic pattern or the checked apps.
func formTableIgnoreRuleSaveRow(pattern string, includeFuture bool, preview *formTablePatternPreviewState, matches []ignoredHotkeyApp) (map[string]any, string) {
	pattern = strings.TrimSpace(pattern)
	if includeFuture {
		return map[string]any{"Pattern": pattern, "IncludeFuture": true}, ""
	}

	seen := map[string]bool{}
	checked := make([]ignoredHotkeyApp, 0, len(matches))
	for _, app := range matches {
		key := formTableAppMatchKey(app.Path, app.Identity)
		if preview != nil && preview.unchecked[key] {
			continue
		}
		if key != "" {
			seen[key] = true
		}
		checked = append(checked, app)
	}
	if preview != nil {
		for _, app := range preview.savedApps {
			key := formTableAppMatchKey(app.Path, app.Identity)
			// Preserve unavailable apps, but never restore an explicit deselection.
			if key == "" || seen[key] || preview.unchecked[key] {
				continue
			}
			checked = append(checked, app)
			seen[key] = true
		}
	}
	if len(checked) == 0 {
		return nil, "i18n:plugin_app_ignore_rule_preview_select_one"
	}

	apps := make([]map[string]any, 0, len(checked))
	for _, app := range checked {
		apps = append(apps, map[string]any{
			"Name": app.Name, "Identity": app.Identity, "Path": app.Path, "Icon": app.Icon,
		})
	}
	return map[string]any{"Pattern": pattern, "IncludeFuture": false, "Apps": apps}, ""
}
