package launcher

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"wox/common"
	woxui "wox/ui/runtime"
	"wox/util"
)

type themeColorToken struct {
	key   string
	label string
}

type themeColorGroup struct {
	label  string
	tokens []themeColorToken
}

var themeEditorColorGroups = []themeColorGroup{
	{label: "i18n:ui_theme_editor_group_window", tokens: []themeColorToken{{key: "AppBackgroundColor", label: "i18n:ui_theme_editor_token_app_background"}}},
	{label: "i18n:ui_theme_editor_group_query_box", tokens: []themeColorToken{
		{key: "QueryBoxBackgroundColor", label: "i18n:ui_theme_editor_token_query_background"},
		{key: "QueryBoxFontColor", label: "i18n:ui_theme_editor_token_query_text"},
		{key: "QueryBoxCursorColor", label: "i18n:ui_theme_editor_token_query_cursor"},
		{key: "QueryBoxTextSelectionBackgroundColor", label: "i18n:ui_theme_editor_token_query_selection"},
	}},
	{label: "i18n:ui_theme_editor_group_results", tokens: []themeColorToken{
		{key: "ResultItemTitleColor", label: "i18n:ui_theme_editor_token_result_title"},
		{key: "ResultItemSubTitleColor", label: "i18n:ui_theme_editor_token_result_subtitle"},
		{key: "ResultItemTailTextColor", label: "i18n:ui_theme_editor_token_result_tail"},
		{key: "ResultItemActiveBackgroundColor", label: "i18n:ui_theme_editor_token_result_active_background"},
		{key: "ResultItemActiveTitleColor", label: "i18n:ui_theme_editor_token_result_active_title"},
		{key: "ResultItemActiveTailTextColor", label: "i18n:ui_theme_editor_token_result_active_tail"},
	}},
	{label: "i18n:ui_theme_editor_group_preview", tokens: []themeColorToken{
		{key: "PreviewFontColor", label: "i18n:ui_theme_editor_token_preview_text"},
		{key: "PreviewPropertyTitleColor", label: "i18n:ui_theme_editor_token_preview_tag_border"},
		{key: "PreviewPropertyContentColor", label: "i18n:ui_theme_editor_token_preview_tag_text"},
		{key: "PreviewSplitLineColor", label: "i18n:ui_theme_editor_token_preview_split"},
		{key: "PreviewTextSelectionColor", label: "i18n:ui_theme_editor_token_preview_selection"},
	}},
	{label: "i18n:ui_theme_editor_group_action_panel", tokens: []themeColorToken{
		{key: "ActionContainerBackgroundColor", label: "i18n:ui_theme_editor_token_action_background"},
		{key: "ActionContainerHeaderFontColor", label: "i18n:ui_theme_editor_token_action_header"},
		{key: "ActionItemFontColor", label: "i18n:ui_theme_editor_token_action_text"},
		{key: "ActionItemActiveBackgroundColor", label: "i18n:ui_theme_editor_token_action_active_background"},
		{key: "ActionItemActiveFontColor", label: "i18n:ui_theme_editor_token_action_active_text"},
		{key: "ActionQueryBoxBackgroundColor", label: "i18n:ui_theme_editor_token_action_query_background"},
	}},
	{label: "i18n:ui_theme_editor_group_toolbar", tokens: []themeColorToken{
		{key: "ToolbarBackgroundColor", label: "i18n:ui_theme_editor_token_toolbar_background"},
		{key: "ToolbarFontColor", label: "i18n:ui_theme_editor_token_toolbar_text"},
	}},
}

type themeEditorPreviewState struct {
	formFieldsState
	key            string
	raw            map[string]any
	initial        map[string]string
	sourceID       string
	sourceName     string
	isSystem       bool
	isAuto         bool
	activeGroup    int
	dialogMode     string
	dialogToken    string
	dialogOriginal string
	flashToken     string
	flashRevision  uint64
	saving         bool
	error          string
	revision       uint64
}

type themeEditorPreviewSnapshot struct {
	formFieldsSnapshot
	raw         map[string]any
	key         string
	initial     map[string]string
	sourceID    string
	sourceName  string
	isSystem    bool
	isAuto      bool
	activeGroup int
	dialogMode  string
	dialogToken string
	flashToken  string
	saving      bool
	error       string
}

func themeEditorTokens() []themeColorToken {
	count := 0
	for _, group := range themeEditorColorGroups {
		count += len(group.tokens)
	}
	tokens := make([]themeColorToken, 0, count)
	for _, group := range themeEditorColorGroups {
		tokens = append(tokens, group.tokens...)
	}
	return tokens
}

func themeMapString(raw map[string]any, key string) string {
	value, ok := raw[key]
	if !ok || value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func themeEditorForm(raw map[string]any) ([]formDefinition, map[string]string) {
	definitions := []formDefinition{{Type: "textbox", Value: formDefinitionValue{Key: "ThemeName", Label: "Theme name", Tooltip: "Change the name to save a new copy"}}, {Type: "newline"}}
	values := map[string]string{"ThemeName": themeMapString(raw, "ThemeName")}
	for _, group := range themeEditorColorGroups {
		definitions = append(definitions, formDefinition{Type: "head", Value: formDefinitionValue{Content: group.label}})
		for _, token := range group.tokens {
			definitions = append(definitions, formDefinition{Type: "textbox", Value: formDefinitionValue{Key: token.key, Label: token.label, Tooltip: "CSS color: #RRGGBB, #RRGGBBAA, rgb(), or rgba()"}})
			values[token.key] = themeMapString(raw, token.key)
		}
	}
	return definitions, values
}

func copyStringMap(source map[string]string) map[string]string {
	copy := make(map[string]string, len(source))
	for key, value := range source {
		copy[key] = value
	}
	return copy
}

func copyThemeMap(source map[string]any) map[string]any {
	copy := make(map[string]any, len(source))
	for key, value := range source {
		copy[key] = value
	}
	return copy
}

// newThemeEditorState builds one portable draft from the current Settings theme.
func newThemeEditorState(key string, raw map[string]any) *themeEditorPreviewState {
	definitions, values := themeEditorForm(raw)
	fields := newFormFieldsState(definitions, values, false)
	isSystem, _ := raw["IsSystem"].(bool)
	isAuto, _ := raw["IsAutoAppearance"].(bool)
	return &themeEditorPreviewState{
		formFieldsState: fields,
		key:             key,
		raw:             copyThemeMap(raw),
		initial:         copyStringMap(values),
		sourceID:        themeMapString(raw, "ThemeId"),
		sourceName:      themeMapString(raw, "ThemeName"),
		isSystem:        isSystem,
		isAuto:          isAuto,
	}
}

// loadSettingsThemeEditor opens the applied theme as the Settings theme editor draft.
func (a *App) loadSettingsThemeEditor() error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	theme, err := a.services.CurrentTheme(ctx, a.sessionID)
	if err != nil {
		return fmt.Errorf("load active theme: %w", err)
	}
	encoded, err := json.Marshal(theme)
	if err != nil {
		return fmt.Errorf("encode active theme: %w", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(encoded, &raw); err != nil {
		return fmt.Errorf("decode active theme: %w", err)
	}
	if strings.TrimSpace(themeMapString(raw, "AppBackgroundColor")) == "" {
		return fmt.Errorf("active theme has no color data")
	}
	encoded, _ = json.Marshal(raw)
	hash := sha256.Sum256(encoded)
	return a.runOnUI("apply settings theme editor", func() {
		a.themeSettings.SetThemeEditor(newThemeEditorState(fmt.Sprintf("settings-theme|%x", hash[:8]), raw))
		a.preloadDemoWallpaper(true)
		a.invalidateThemeEditorWindow()
	})
}

// snapshotThemeEditorPreviewLocked copies the Settings draft for a render pass.
func snapshotThemeEditorPreviewLocked(state *themeEditorPreviewState) *themeEditorPreviewSnapshot {
	if state == nil {
		return nil
	}
	return &themeEditorPreviewSnapshot{
		formFieldsSnapshot: snapshotFormFieldsLocked(&state.formFieldsState),
		raw:                copyThemeMap(state.raw),
		key:                state.key,
		initial:            copyStringMap(state.initial),
		sourceID:           state.sourceID,
		sourceName:         state.sourceName,
		isSystem:           state.isSystem,
		isAuto:             state.isAuto,
		activeGroup:        state.activeGroup,
		dialogMode:         state.dialogMode,
		dialogToken:        state.dialogToken,
		flashToken:         state.flashToken,
		saving:             state.saving,
		error:              state.error,
	}
}

func themeEditorPalette(values map[string]string) uiPalette {
	theme := themeData{}
	raw, _ := json.Marshal(values)
	_ = json.Unmarshal(raw, &theme)
	return paletteForTheme(theme)
}

// themeEditorDraftPalette preserves non-editable theme geometry while applying the live color draft.
func themeEditorDraftPalette(raw map[string]any, values map[string]string) uiPalette {
	theme, err := themeEditorDraftTheme(raw, values)
	if err != nil {
		return themeEditorPalette(values)
	}
	return paletteForTheme(theme)
}

// themeEditorDraftTheme merges editable values into the complete source theme.
func themeEditorDraftTheme(raw map[string]any, values map[string]string) (themeData, error) {
	for _, token := range themeEditorTokens() {
		if _, ok := decodeThemeColor(values[token.key]); !ok {
			return themeData{}, fmt.Errorf("%s is not a valid CSS color", token.key)
		}
	}
	draft := copyThemeMap(raw)
	for key, value := range values {
		draft[key] = value
	}
	var theme themeData
	encoded, err := json.Marshal(draft)
	if err == nil {
		err = json.Unmarshal(encoded, &theme)
	}
	if err != nil {
		return themeData{}, err
	}
	return theme, nil
}

// applySettingsThemeEditorDraft previews a valid settings draft across every Wox window.
func (a *App) applySettingsThemeEditorDraft() {
	state := a.themeSettings.ThemeEditor()
	if state == nil || !strings.HasPrefix(state.key, "settings-theme|") {
		return
	}
	theme, err := themeEditorDraftTheme(state.raw, state.values)
	if err == nil {
		a.applyTheme(theme)
	}
}

// onThemeEditorPreviewKey gives the draft form keyboard ownership only after a field is focused.
func (a *App) onThemeEditorPreviewKey(event woxui.KeyEvent) bool {
	state := a.themeSettings.ThemeEditor()
	active := state != nil && state.active
	dialogOpen := state != nil && state.dialogMode != ""
	if !active {
		return false
	}
	if dialogOpen && event.Key == woxui.KeyEscape {
		a.cancelThemeEditorDialog()
		return true
	}
	if dialogOpen && event.Key == woxui.KeyEnter && !event.Modifiers.HasPrimary() {
		a.confirmThemeEditorDialog()
		return true
	}
	if event.Key == woxui.KeyEscape {
		a.deactivateThemeEditorPreview()
		return true
	}
	switch event.Key {
	case woxui.KeyTab, woxui.KeyArrowDown:
		delta := 1
		if event.Key == woxui.KeyTab && event.Modifiers&woxui.KeyModifierShift != 0 {
			delta = -1
		}
		a.moveThemeEditorFocus(delta)
	case woxui.KeyArrowUp:
		a.moveThemeEditorFocus(-1)
	case woxui.KeyEnter:
		return true
	default:
		return false
	}
	return true
}

func (a *App) onThemeEditorPreviewTextInput(_ woxui.TextInputEvent) bool {
	state := a.themeSettings.ThemeEditor()
	active := state != nil && state.active
	return active
}

func (a *App) editThemeEditorKey(event woxui.KeyEvent) {
	state := a.themeSettings.ThemeEditor()
	if state != nil && state.active && state.editor != nil && state.focused >= 0 && state.focused < len(state.definitions) {
		_, changed := handleFormEditorKey(state.editor, state.definitions[state.focused], event)
		if changed {
			syncFormFieldsEditorLocked(&state.formFieldsState)
			state.error = ""
			a.applySettingsThemeEditorDraft()
		}
	}
	a.invalidateThemeEditorWindow()
}

func (a *App) moveThemeEditorFocus(delta int) {
	state := a.themeSettings.ThemeEditor()
	if state == nil || len(state.definitions) == 0 {
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	index := state.focused
	for step := 0; step < len(state.definitions); step++ {
		index = (index + delta + len(state.definitions)) % len(state.definitions)
		if formDefinitionFocusable(state.definitions[index]) {
			setFormFieldsFocusLocked(&state.formFieldsState, index)
			break
		}
	}
	textInput := state.editor != nil
	a.updateThemeEditorTextInput(textInput)
	a.invalidateThemeEditorWindow()
}

func (a *App) focusThemeEditorField(index int) {
	state := a.themeSettings.ThemeEditor()
	if state == nil || state.saving || index < 0 || index >= len(state.definitions) || !formDefinitionFocusable(state.definitions[index]) {
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	setFormFieldsFocusLocked(&state.formFieldsState, index)
	state.error = ""
	textInput := state.editor != nil
	a.updateThemeEditorTextInput(textInput)
	a.invalidateThemeEditorWindow()
}

func (a *App) setThemeEditorText(index int, value string) {
	state := a.themeSettings.ThemeEditor()
	changed := state != nil && !state.saving && setFormFieldsTextLocked(&state.formFieldsState, index, value)
	if changed {
		state.error = ""
		a.applySettingsThemeEditorDraft()
		a.invalidateThemeEditorWindow()
	}
}

func (a *App) deactivateThemeEditorPreview() {
	state := a.themeSettings.ThemeEditor()
	wasActive := state != nil && state.active
	if wasActive {
		syncFormFieldsEditorLocked(&state.formFieldsState)
		state.active = false
	}
	if !wasActive {
		return
	}
	a.restoreThemeEditorTextInput()
	a.invalidateThemeEditorWindow()
}

func validateThemeEditorValues(values map[string]string) string {
	if strings.TrimSpace(values["ThemeName"]) == "" {
		return "Theme name cannot be empty."
	}
	for _, token := range themeEditorTokens() {
		if _, ok := decodeThemeColor(values[token.key]); !ok {
			return fmt.Sprintf("%s is not a valid CSS color.", token.key)
		}
	}
	return ""
}

// saveThemeEditorDraft preserves non-color theme fields while saving through the shared theme service.
func (a *App) saveThemeEditorDraft(name string, overwrite bool) {
	state := a.themeSettings.ThemeEditor()
	if state == nil || state.saving {
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	if validationError := validateThemeEditorValues(state.values); validationError != "" {
		state.error = validationError
		a.invalidateThemeEditorWindow()
		return
	}
	values := copyStringMap(state.values)
	draft := copyThemeMap(state.raw)
	name = strings.TrimSpace(name)
	values["ThemeName"] = name
	draft["ThemeName"] = name
	for _, token := range themeEditorTokens() {
		draft[token.key] = strings.TrimSpace(values[token.key])
	}
	if overwrite && (state.isSystem || state.isAuto || state.sourceID == "") {
		state.error = "This theme cannot be overwritten."
		a.invalidateThemeEditorWindow()
		return
	}
	state.saving = true
	state.active = false
	state.dialogMode = ""
	state.dialogToken = ""
	state.dialogOriginal = ""
	state.error = ""
	state.revision++
	revision := state.revision
	key := state.key
	a.restoreThemeEditorTextInput()
	a.invalidateThemeEditorWindow()

	util.Go(a.lifecycleCtx, "save theme editor draft", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		encodedDraft, err := json.Marshal(draft)
		var theme common.Theme
		if err == nil {
			err = json.Unmarshal(encodedDraft, &theme)
		}
		var saved map[string]any
		if err == nil {
			var savedTheme common.Theme
			savedTheme, err = a.services.SaveTheme(ctx, a.sessionID, name, theme, overwrite)
			if err == nil {
				var encodedSaved []byte
				encodedSaved, err = json.Marshal(savedTheme)
				if err == nil {
					err = json.Unmarshal(encodedSaved, &saved)
				}
			}
		}
		var applied themeData
		if err == nil {
			encoded, marshalErr := json.Marshal(saved)
			if marshalErr != nil {
				err = marshalErr
			} else {
				if unmarshalErr := json.Unmarshal(encoded, &applied); unmarshalErr != nil {
					err = unmarshalErr
				}
			}
		}

		_ = a.runOnUI("apply saved theme editor draft", func() {
			if err == nil {
				a.applyTheme(applied)
				a.generalSettings.Update(func(d *settingsData) { d.ThemeID = themeMapString(saved, "ThemeId") })
			}
			current := a.themeSettings.ThemeEditor()
			if current != nil && current.key == key && current.revision == revision {
				current.saving = false
				if err != nil {
					current.error = err.Error()
				} else {
					definitions, savedValues := themeEditorForm(saved)
					current.formFieldsState = newFormFieldsState(definitions, savedValues, false)
					current.raw = saved
					current.initial = copyStringMap(savedValues)
					current.sourceID = themeMapString(saved, "ThemeId")
					current.sourceName = themeMapString(saved, "ThemeName")
					current.isSystem = false
					current.isAuto = false
					current.error = ""
				}
			}
			a.invalidateThemeEditorWindow()
		})
		if err != nil {
			log.Printf("save theme editor draft: %v", err)
		}
	})
}
