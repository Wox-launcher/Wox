package launcher

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
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

// buildThemeEditorPreview prepares the current editor state for the pure preview view.
func (a *App) buildThemeEditorPreview(result queryResult, preview queryPreview, palette uiPalette, width, height float32) woxwidget.Widget {
	state, err := a.themeEditorPreviewSnapshotFor(result, preview)
	if err != nil {
		return previewview.ThemeEditorPreviewView(previewview.ThemeEditorPreviewProps{Width: width, Height: height, Theme: palette.componentTheme(), FatalError: err.Error()})
	}
	return a.buildThemeEditorSurface(state, palette, width, height)
}

// buildThemeEditorSurface adapts controller-owned form fields to the shared theme editor view.
func (a *App) buildThemeEditorSurface(state *themeEditorPreviewSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-32)
	callbacks := formFieldCallbacks{idPrefix: "theme-editor", focus: a.focusThemeEditorField, setCaret: a.setThemeEditorCaret}
	rows := make([]woxwidget.Widget, 0, len(state.definitions))
	for index, definition := range state.definitions {
		rows = append(rows, a.buildFormField(state.formFieldsSnapshot, callbacks, palette, index, definition, innerWidth, formDefinitionHeight(definition, state.values)))
	}
	dirty := false
	for key, value := range state.values {
		if value != state.initial[key] {
			dirty = true
			break
		}
	}
	saveLabel := a.translate("i18n:ui_save")
	if state.isSystem || strings.TrimSpace(state.values["ThemeName"]) != state.sourceName {
		saveLabel = "Save copy"
	}
	draftPalette := themeEditorDraftPalette(state.raw, state.values)
	return previewview.ThemeEditorPreviewView(previewview.ThemeEditorPreviewProps{
		Width: width, Height: height, Theme: palette.componentTheme(), DraftTheme: draftPalette.componentTheme(),
		Error: state.error, SaveLabel: saveLabel, Dirty: dirty, Saving: state.saving,
		Rows: rows, RowsHeight: formDefinitionsContentHeight(state.definitions, state.values), Scroll: state.scroll,
		OnScroll:      func(delta float32) { a.scrollThemeEditorPreview(state.key, delta) },
		OnSetViewport: func(viewport float32) { a.setThemeEditorViewport(state.key, viewport) }, OnSubmit: a.submitThemeEditorPreview,
	})
}

func (a *App) buildThemeDraftSample(values map[string]string, width, height float32) woxwidget.Widget {
	return previewview.ThemeDraftSample(themeEditorPalette(values).componentTheme(), width, height)
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

// newThemeEditorState builds one portable draft from either a query preview or the settings route.
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

// loadSettingsThemeEditor opens the applied theme through the same draft engine used by query previews.
func (a *App) loadSettingsThemeEditor() error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	var raw map[string]any
	if err := a.client.Post(ctx, "/theme", nil, &raw); err != nil {
		return fmt.Errorf("load active theme: %w", err)
	}
	if strings.TrimSpace(themeMapString(raw, "AppBackgroundColor")) == "" {
		return fmt.Errorf("active theme has no color data")
	}
	encoded, _ := json.Marshal(raw)
	hash := sha256.Sum256(encoded)
	a.mu.Lock()
	a.themeEditor = newThemeEditorState(fmt.Sprintf("settings-theme|%x", hash[:8]), raw)
	a.mu.Unlock()
	a.preloadThemeEditorWallpaper()
	if a.window != nil {
		a.invalidateThemeEditorWindow()
	}
	return nil
}

// themeEditorPreviewDataAndKey validates the draft and derives its stable controller identity.
func themeEditorPreviewDataAndKey(result queryResult, preview queryPreview) (map[string]any, string, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(preview.PreviewData), &raw); err != nil {
		return nil, "", fmt.Errorf("decode theme editor preview: %w", err)
	}
	if strings.TrimSpace(themeMapString(raw, "AppBackgroundColor")) == "" {
		return nil, "", fmt.Errorf("theme editor preview has no theme data")
	}
	hash := sha256.Sum256([]byte(preview.PreviewData))
	return raw, fmt.Sprintf("%s|%s|%x", result.QueryID, result.ID, hash), nil
}

// activateThemeEditorPreview prepares the draft state before rendering.
func (a *App) activateThemeEditorPreview(result queryResult, preview queryPreview) error {
	raw, key, err := themeEditorPreviewDataAndKey(result, preview)
	if err != nil {
		return err
	}
	a.mu.RLock()
	changed := a.themeEditor != nil && a.themeEditor.key != key
	a.mu.RUnlock()
	if changed {
		a.deactivateThemeEditorPreview()
	}

	a.mu.Lock()
	if a.themeEditor == nil || a.themeEditor.key != key {
		a.themeEditor = newThemeEditorState(key, raw)
	}
	a.mu.Unlock()
	return nil
}

// themeEditorPreviewSnapshotFor returns the prepared theme draft.
func (a *App) themeEditorPreviewSnapshotFor(result queryResult, preview queryPreview) (*themeEditorPreviewSnapshot, error) {
	_, key, err := themeEditorPreviewDataAndKey(result, preview)
	if err != nil {
		return nil, err
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.themeEditor == nil || a.themeEditor.key != key {
		return nil, fmt.Errorf("theme editor preview is not ready")
	}
	return snapshotThemeEditorPreviewLocked(a.themeEditor), nil
}

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
		return themeEditorPalette(values)
	}
	return paletteForTheme(theme)
}

// onThemeEditorPreviewKey gives the draft form keyboard ownership only after a field is focused.
func (a *App) onThemeEditorPreviewKey(event woxui.KeyEvent) bool {
	a.mu.RLock()
	state := a.themeEditor
	active := state != nil && state.active
	dialogOpen := state != nil && state.dialogMode != ""
	a.mu.RUnlock()
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
	if event.Key == woxui.KeyEnter && event.Modifiers.HasPrimary() {
		a.submitThemeEditorPreview()
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
	default:
		a.editThemeEditorKey(event)
	}
	return true
}

func (a *App) onThemeEditorPreviewTextInput(event woxui.TextInputEvent) bool {
	a.mu.Lock()
	state := a.themeEditor
	if state == nil || !state.active {
		a.mu.Unlock()
		return false
	}
	if state.editor != nil && state.focused >= 0 && state.focused < len(state.definitions) {
		if state.editor.HandleTextInput(event) {
			syncFormFieldsEditorLocked(&state.formFieldsState)
			state.error = ""
		}
	}
	a.mu.Unlock()
	a.invalidateThemeEditorWindow()
	return true
}

func (a *App) editThemeEditorKey(event woxui.KeyEvent) {
	a.mu.Lock()
	if state := a.themeEditor; state != nil && state.active && state.editor != nil && state.focused >= 0 && state.focused < len(state.definitions) {
		_, changed := handleFormEditorKey(state.editor, state.definitions[state.focused], event)
		if changed {
			syncFormFieldsEditorLocked(&state.formFieldsState)
			state.error = ""
		}
	}
	a.mu.Unlock()
	a.invalidateThemeEditorWindow()
}

func (a *App) moveThemeEditorFocus(delta int) {
	a.mu.Lock()
	state := a.themeEditor
	if state == nil || len(state.definitions) == 0 {
		a.mu.Unlock()
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
	a.mu.Unlock()
	a.updateThemeEditorTextInput(textInput)
	a.invalidateThemeEditorWindow()
}

func (a *App) focusThemeEditorField(index int) {
	a.mu.Lock()
	state := a.themeEditor
	if state == nil || state.saving || index < 0 || index >= len(state.definitions) || !formDefinitionFocusable(state.definitions[index]) {
		a.mu.Unlock()
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	setFormFieldsFocusLocked(&state.formFieldsState, index)
	state.error = ""
	textInput := state.editor != nil
	a.mu.Unlock()
	a.updateThemeEditorTextInput(textInput)
	a.invalidateThemeEditorWindow()
}

func (a *App) setThemeEditorCaret(index, offset int) {
	a.mu.Lock()
	state := a.themeEditor
	if state == nil || !state.active || state.focused != index || state.editor == nil {
		a.mu.Unlock()
		return
	}
	state.editor.SetCaret(offset)
	a.mu.Unlock()
	a.invalidateThemeEditorWindow()
}

func (a *App) deactivateThemeEditorPreview() {
	a.mu.Lock()
	wasActive := a.themeEditor != nil && a.themeEditor.active
	if wasActive {
		syncFormFieldsEditorLocked(&a.themeEditor.formFieldsState)
		a.themeEditor.active = false
	}
	a.mu.Unlock()
	if !wasActive {
		return
	}
	a.restoreThemeEditorTextInput()
	a.invalidateThemeEditorWindow()
}

func (a *App) setThemeEditorViewport(key string, height float32) {
	a.mu.Lock()
	if state := a.themeEditor; state != nil && state.key == key {
		state.viewportHeight = max(float32(1), height)
		state.scroll = min(state.scroll, max(float32(0), formDefinitionsContentHeight(state.definitions, state.values)-state.viewportHeight))
	}
	a.mu.Unlock()
}

func (a *App) scrollThemeEditorPreview(key string, delta float32) {
	a.mu.Lock()
	state := a.themeEditor
	if state == nil || state.key != key {
		a.mu.Unlock()
		return
	}
	maxOffset := max(float32(0), formDefinitionsContentHeight(state.definitions, state.values)-state.viewportHeight)
	state.scroll = min(max(float32(0), state.scroll+delta), maxOffset)
	a.mu.Unlock()
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

// submitThemeEditorPreview keeps the launcher preview's original save-or-copy behavior.
func (a *App) submitThemeEditorPreview() {
	a.mu.Lock()
	state := a.themeEditor
	if state == nil || state.saving {
		a.mu.Unlock()
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	name := strings.TrimSpace(state.values["ThemeName"])
	overwrite := !state.isSystem && !state.isAuto && state.sourceID != "" && name == state.sourceName
	a.mu.Unlock()
	a.saveThemeEditorDraft(name, overwrite)
}

// saveThemeEditorDraft preserves non-color theme fields while saving through the shared core route.
func (a *App) saveThemeEditorDraft(name string, overwrite bool) {
	a.mu.Lock()
	state := a.themeEditor
	if state == nil || state.saving {
		a.mu.Unlock()
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	if validationError := validateThemeEditorValues(state.values); validationError != "" {
		state.error = validationError
		a.mu.Unlock()
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
		a.mu.Unlock()
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
	a.mu.Unlock()
	a.restoreThemeEditorTextInput()
	a.invalidateThemeEditorWindow()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var saved map[string]any
		err := a.client.Post(ctx, "/theme/save", map[string]any{"Name": name, "Theme": draft, "Overwrite": overwrite}, &saved)
		if err == nil {
			encoded, marshalErr := json.Marshal(saved)
			if marshalErr != nil {
				err = marshalErr
			} else {
				var applied themeData
				if unmarshalErr := json.Unmarshal(encoded, &applied); unmarshalErr != nil {
					err = unmarshalErr
				} else {
					a.applyTheme(applied)
					a.mu.Lock()
					a.settings.ThemeID = themeMapString(saved, "ThemeId")
					a.mu.Unlock()
				}
			}
		}

		a.mu.Lock()
		current := a.themeEditor != nil && a.themeEditor.key == key && a.themeEditor.revision == revision
		if current {
			a.themeEditor.saving = false
			if err != nil {
				a.themeEditor.error = err.Error()
			} else {
				definitions, savedValues := themeEditorForm(saved)
				a.themeEditor.formFieldsState = newFormFieldsState(definitions, savedValues, false)
				a.themeEditor.raw = saved
				a.themeEditor.initial = copyStringMap(savedValues)
				a.themeEditor.sourceID = themeMapString(saved, "ThemeId")
				a.themeEditor.sourceName = themeMapString(saved, "ThemeName")
				a.themeEditor.isSystem = false
				a.themeEditor.isAuto = false
				a.themeEditor.error = ""
			}
		}
		a.mu.Unlock()
		if err != nil {
			log.Printf("save theme editor preview: %v", err)
		}
		a.invalidateThemeEditorWindow()
	}()
}
