package launcher

import (
	"image"
	"log"
	"math"
	"os"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/wallpaper"
)

// buildThemeEditorSettingsSurface adapts the shared draft controller to Flutter's settings-only editor layout.
func (a *App) buildThemeEditorSettingsSurface(state *themeEditorPreviewSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	groups := make([]launcherview.ThemeEditorColorGroup, 0, len(themeEditorColorGroups))
	for _, group := range themeEditorColorGroups {
		tokens := make([]launcherview.ThemeEditorColorToken, 0, len(group.tokens))
		for _, token := range group.tokens {
			color, ok := decodeThemeColor(state.values[token.key])
			if !ok {
				color = palette.componentTheme().ErrorText
			}
			tokens = append(tokens, launcherview.ThemeEditorColorToken{Key: token.key, Label: a.translate(token.label), Color: color})
		}
		groups = append(groups, launcherview.ThemeEditorColorGroup{Label: a.translate(group.label), Tokens: tokens})
	}

	foreground := palette.resultTitle
	primaryForeground := palette.actionSelectedText
	locateIcon := a.imageForTint(settingControlIconSource("locate"), &foreground, 18)
	discardIcon := a.imageForTint(settingControlIconSource("undo"), &foreground, 18)
	overwriteIcon := a.imageForTint(settingControlIconSource("save-edit"), &foreground, 18)
	saveAsIcon := a.imageForTint(settingControlIconSource("save"), &primaryForeground, 18)
	a.mu.RLock()
	wallpaperImage := a.themeWallpaperImage
	wallpaperBlurred := a.themeWallpaperBlurred
	a.mu.RUnlock()
	draftPalette := themeEditorDraftPalette(state.raw, state.values)
	previewItemPadding := draftPalette.resultItemPadding
	previewItemPadding.Left += 5
	previewItemPadding.Right += 5

	dirty := themeEditorSnapshotDirty(state)
	return launcherview.ThemeEditorSettingsView(launcherview.ThemeEditorSettingsProps{
		Width: width, Height: height, Theme: palette.componentTheme(), DraftTheme: draftPalette.componentTheme(),
		Groups: groups, ActiveGroup: state.activeGroup, Dirty: dirty, Saving: state.saving, CanOverwrite: !state.isSystem && !state.isAuto && state.sourceID != "", Error: state.error,
		Wallpaper: wallpaperImage, WallpaperBlurred: wallpaperBlurred,
		PreviewGeometry: launcherview.ThemeEditorPreviewGeometry{
			AppPadding: draftPalette.appPadding, QueryRadius: draftPalette.queryRadius, ResultContainerPadding: draftPalette.resultContainerPadding,
			ResultItemPadding: previewItemPadding, ResultItemRadius: draftPalette.resultItemRadius, ToolbarPadding: draftPalette.toolbarPadding,
		},
		FlashToken: state.flashToken,
		LocateIcon: locateIcon, DiscardIcon: discardIcon, OverwriteIcon: overwriteIcon, SaveAsIcon: saveAsIcon,
		DiscardLabel: a.translate("i18n:ui_theme_editor_discard"), OverwriteLabel: a.translate("i18n:ui_theme_editor_overwrite"), SaveAsLabel: a.translate("i18n:ui_theme_editor_save_as"), SavingLabel: a.translate("i18n:ui_theme_editor_saving"),
		PreviewResultTitle: a.translate("i18n:ui_theme_editor_preview_result_theme"), PreviewResultState: a.translate("i18n:ui_theme_editor_preview_result_current"),
		QueryBoxLabel: a.translate("i18n:ui_theme_editor_preview_result_query"), ResultsLabel: a.translate("i18n:ui_theme_editor_group_results"),
		ToolbarCopyLabel: a.translate("i18n:ui_theme_editor_toolbar_copy"), ToolbarMoreLabel: a.translate("i18n:ui_theme_editor_toolbar_more"),
		Dialog:        a.buildThemeEditorSettingsDialog(state, palette, width, height),
		OnSelectGroup: a.selectThemeEditorGroup, OnEditToken: a.openThemeEditorTokenDialog, OnLocateToken: a.locateThemeEditorToken,
		OnDiscard: a.discardThemeEditorDraft, OnOverwrite: a.overwriteThemeEditorDraft, OnSaveAs: a.openThemeEditorSaveAsDialog,
	})
}

// preloadThemeEditorWallpaper starts one settings-owned wallpaper load before the editor needs it.
func (a *App) preloadThemeEditorWallpaper() {
	a.mu.Lock()
	if a.themeWallpaperImage != nil || a.themeWallpaperLoading {
		a.mu.Unlock()
		return
	}
	a.themeWallpaperLoading = true
	a.themeWallpaperLoadID++
	loadID := a.themeWallpaperLoadID
	path := a.themeWallpaperPath
	a.mu.Unlock()
	go a.loadThemeEditorWallpaper(loadID, path)
}

// loadThemeEditorWallpaper resolves and decodes the desktop image without blocking settings rendering.
func (a *App) loadThemeEditorWallpaper(loadID uint64, path string) {
	var err error
	if path == "" {
		path, err = wallpaper.GetSystemWallpaperPath()
	}
	if err == nil {
		if _, statErr := os.Stat(path); statErr != nil {
			err = statErr
		}
	}
	var wallpaperImage *woxui.Image
	var wallpaperBlurred *woxui.Image
	if err == nil {
		wallpaperImage, wallpaperBlurred, err = decodeThemeEditorWallpaper(path)
	}
	a.mu.Lock()
	if a.themeWallpaperLoadID != loadID {
		a.mu.Unlock()
		return
	}
	a.themeWallpaperLoading = false
	settingsOpen := a.settingsOpen
	if err == nil && settingsOpen {
		a.themeWallpaperPath = path
		a.themeWallpaperImage = wallpaperImage
		a.themeWallpaperBlurred = wallpaperBlurred
	}
	a.mu.Unlock()
	if err != nil {
		log.Printf("load theme editor wallpaper: %v", err)
		return
	}
	if !settingsOpen {
		return
	}
	a.invalidateSettingsWindow()
}

// releaseThemeEditorWallpaperLocked prevents an in-flight load from restoring settings-owned image memory after close.
func (a *App) releaseThemeEditorWallpaperLocked() {
	a.themeWallpaperLoadID++
	a.themeWallpaperPath = ""
	a.themeWallpaperImage = nil
	a.themeWallpaperBlurred = nil
	a.themeWallpaperLoading = false
}

// decodeThemeEditorWallpaper prepares a cover-fitted desktop image and the matching blurred center crop used by the simulated window.
func decodeThemeEditorWallpaper(path string) (*woxui.Image, *woxui.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	source, _, err := image.Decode(file)
	if err != nil {
		return nil, nil, err
	}
	bounds := source.Bounds()
	if bounds.Dx() > 2048 {
		source = imaging.Resize(source, 2048, 0, imaging.CatmullRom)
	}
	stage := imaging.Fill(source, 1800, 840, imaging.Center, imaging.Lanczos)
	logicalStage := imaging.Resize(stage, 900, 420, imaging.Lanczos)
	blurredStage := imaging.Blur(logicalStage, 24)
	blurredWindow := imaging.CropCenter(blurredStage, 702, 344)
	maskThemeEditorRoundedCorners(stage, 36)
	maskThemeEditorRoundedCorners(blurredWindow, 12)
	wallpaperImage, err := woxui.NewImage(stage)
	if err != nil {
		return nil, nil, err
	}
	wallpaperBlurred, err := woxui.NewImage(blurredWindow)
	if err != nil {
		return nil, nil, err
	}
	return wallpaperImage, wallpaperBlurred, nil
}

// maskThemeEditorRoundedCorners keeps preprocessed wallpaper layers inside the same rounded bounds as Flutter's clips.
func maskThemeEditorRoundedCorners(source *image.NRGBA, radius int) {
	if source == nil || radius <= 0 {
		return
	}
	bounds := source.Bounds()
	radius = min(radius, min(bounds.Dx(), bounds.Dy())/2)
	center := float64(radius)
	for y := 0; y < radius; y++ {
		for x := 0; x < radius; x++ {
			distance := math.Hypot(float64(x)+0.5-center, float64(y)+0.5-center)
			coverage := min(float64(1), max(float64(0), center+0.5-distance))
			if coverage >= 1 {
				continue
			}
			for _, point := range [][2]int{{x, y}, {bounds.Dx() - 1 - x, y}, {x, bounds.Dy() - 1 - y}, {bounds.Dx() - 1 - x, bounds.Dy() - 1 - y}} {
				offset := source.PixOffset(point[0], point[1])
				source.Pix[offset+3] = uint8(float64(source.Pix[offset+3])*coverage + 0.5)
			}
		}
	}
}

func themeEditorSnapshotDirty(state *themeEditorPreviewSnapshot) bool {
	if state == nil {
		return false
	}
	for key, value := range state.values {
		if value != state.initial[key] {
			return true
		}
	}
	return false
}

func themeEditorDefinitionIndex(definitions []formDefinition, key string) int {
	for index, definition := range definitions {
		if definition.Value.Key == key {
			return index
		}
	}
	return -1
}

func themeEditorGroupForToken(key string) int {
	for groupIndex, group := range themeEditorColorGroups {
		for _, token := range group.tokens {
			if token.key == key {
				return groupIndex
			}
		}
	}
	return 0
}

func (a *App) selectThemeEditorGroup(index int) {
	a.mu.Lock()
	if a.themeEditor != nil && index >= 0 && index < len(themeEditorColorGroups) {
		a.themeEditor.activeGroup = index
		a.themeEditor.error = ""
	}
	a.mu.Unlock()
	a.invalidateThemeEditorWindow()
}

func (a *App) locateThemeEditorToken(key string) {
	a.mu.Lock()
	state := a.themeEditor
	if state == nil {
		a.mu.Unlock()
		return
	}
	state.activeGroup = themeEditorGroupForToken(key)
	state.flashToken = key
	state.flashRevision++
	revision := state.flashRevision
	stateKey := state.key
	state.error = ""
	a.mu.Unlock()
	a.invalidateThemeEditorWindow()
	time.AfterFunc(780*time.Millisecond, func() {
		a.mu.Lock()
		if a.themeEditor != nil && a.themeEditor.key == stateKey && a.themeEditor.flashRevision == revision {
			a.themeEditor.flashToken = ""
		}
		a.mu.Unlock()
		a.invalidateThemeEditorWindow()
	})
}

func (a *App) openThemeEditorTokenDialog(key string) {
	a.mu.Lock()
	state := a.themeEditor
	if state == nil || state.saving {
		a.mu.Unlock()
		return
	}
	index := themeEditorDefinitionIndex(state.definitions, key)
	if index < 0 {
		a.mu.Unlock()
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	state.activeGroup = themeEditorGroupForToken(key)
	state.dialogMode = "token"
	state.dialogToken = key
	state.dialogOriginal = state.values[key]
	state.error = ""
	setFormFieldsFocusLocked(&state.formFieldsState, index)
	textInput := state.editor != nil
	a.mu.Unlock()
	a.updateThemeEditorTextInput(textInput)
	a.invalidateThemeEditorWindow()
}

func (a *App) openThemeEditorSaveAsDialog() {
	a.mu.Lock()
	state := a.themeEditor
	if state == nil || state.saving {
		a.mu.Unlock()
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	index := themeEditorDefinitionIndex(state.definitions, "ThemeName")
	if index < 0 {
		a.mu.Unlock()
		return
	}
	state.dialogMode = "save-as"
	state.dialogToken = "ThemeName"
	state.dialogOriginal = state.values["ThemeName"]
	defaultName := a.translate("i18n:ui_theme_editor_default_theme_name")
	baseName := strings.TrimSpace(state.sourceName)
	if baseName == "" {
		baseName = a.translate("i18n:ui_theme_editor_default_theme")
	}
	state.values["ThemeName"] = strings.ReplaceAll(defaultName, "{name}", baseName)
	setFormFieldsFocusLocked(&state.formFieldsState, index)
	if state.editor != nil {
		state.editor.SelectAll()
	}
	state.error = ""
	textInput := state.editor != nil
	a.mu.Unlock()
	a.updateThemeEditorTextInput(textInput)
	a.invalidateThemeEditorWindow()
}

func (a *App) buildThemeEditorSettingsDialog(state *themeEditorPreviewSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	if state == nil || state.dialogMode == "" {
		return nil
	}
	index := themeEditorDefinitionIndex(state.definitions, state.dialogToken)
	if index < 0 {
		return nil
	}
	panelWidth := min(float32(420), max(float32(320), width-40))
	panelHeight := float32(176)
	callbacks := formFieldCallbacks{idPrefix: "theme-editor-dialog", focus: a.focusThemeEditorField, setText: a.setThemeEditorText, onKey: a.onThemeEditorPreviewKey}
	field := a.buildFormField(state.formFieldsSnapshot, callbacks, palette, index, state.definitions[index], panelWidth-32, 56)
	title := a.translate("i18n:ui_theme_editor_save_as_title")
	confirmLabel := a.translate("i18n:ui_theme_editor_save_as")
	if state.dialogMode == "token" {
		for _, token := range themeEditorTokens() {
			if token.key == state.dialogToken {
				title = a.translate(token.label)
				break
			}
		}
		confirmLabel = a.translate("i18n:ui_ok")
	}
	footer := woxwidget.Container{Width: panelWidth - 32, Height: 46, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		woxwidget.Container{Width: max(float32(0), panelWidth-32-210), Height: 38},
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "theme-editor-dialog-cancel", Label: a.translate("i18n:ui_cancel"), Width: 96, Height: 36, Variant: woxcomponent.ButtonOutline, OnTap: a.cancelThemeEditorDialog, Theme: palette.componentTheme()}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "theme-editor-dialog-confirm", Label: confirmLabel, Width: 104, Height: 36, Variant: woxcomponent.ButtonPrimary, OnTap: a.confirmThemeEditorDialog, Theme: palette.componentTheme()}),
	}}}
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "theme-editor-dialog", Label: title, Width: panelWidth, Height: panelHeight, OverlayWidth: width, OverlayHeight: height,
		BackdropID: "theme-editor-dialog-backdrop", BackdropAlpha: 190, Padding: woxwidget.UniformInsets(16), Theme: palette.componentTheme(), OnDismiss: a.cancelThemeEditorDialog,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
			woxwidget.Container{Width: panelWidth - 32, Height: 28, Child: woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: palette.actionText}},
			field,
			footer,
		}},
	})
}

func (a *App) cancelThemeEditorDialog() {
	a.mu.Lock()
	state := a.themeEditor
	if state == nil || state.dialogMode == "" {
		a.mu.Unlock()
		return
	}
	state.values[state.dialogToken] = state.dialogOriginal
	if state.editor != nil {
		state.editor.SetText(state.dialogOriginal, false)
	}
	state.dialogMode = ""
	state.dialogToken = ""
	state.dialogOriginal = ""
	state.active = false
	state.error = ""
	a.mu.Unlock()
	a.restoreThemeEditorTextInput()
	a.invalidateThemeEditorWindow()
}

func (a *App) confirmThemeEditorDialog() {
	a.mu.Lock()
	state := a.themeEditor
	if state == nil || state.dialogMode == "" {
		a.mu.Unlock()
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	mode := state.dialogMode
	value := strings.TrimSpace(state.values[state.dialogToken])
	if mode == "token" {
		if _, ok := decodeThemeColor(value); !ok {
			state.error = a.translate("i18n:ui_theme_editor_invalid_color")
			a.mu.Unlock()
			a.invalidateThemeEditorWindow()
			return
		}
	} else if value == "" {
		state.error = a.translate("i18n:ui_theme_editor_name_required")
		a.mu.Unlock()
		a.invalidateThemeEditorWindow()
		return
	}
	state.values[state.dialogToken] = value
	state.dialogMode = ""
	state.dialogToken = ""
	state.dialogOriginal = ""
	state.active = false
	state.error = ""
	a.mu.Unlock()
	a.restoreThemeEditorTextInput()
	a.invalidateThemeEditorWindow()
	if mode == "save-as" {
		a.saveThemeEditorDraft(value, false)
	}
}

func (a *App) discardThemeEditorDraft() {
	a.mu.Lock()
	state := a.themeEditor
	if state == nil || state.saving {
		a.mu.Unlock()
		return
	}
	definitions := append([]formDefinition(nil), state.definitions...)
	state.formFieldsState = newFormFieldsState(definitions, state.initial, false)
	state.dialogMode = ""
	state.dialogToken = ""
	state.dialogOriginal = ""
	state.error = ""
	a.mu.Unlock()
	a.restoreThemeEditorTextInput()
	a.invalidateThemeEditorWindow()
}

func (a *App) overwriteThemeEditorDraft() {
	a.mu.RLock()
	state := a.themeEditor
	if state == nil || state.saving || state.isSystem || state.isAuto {
		a.mu.RUnlock()
		return
	}
	name := state.sourceName
	a.mu.RUnlock()
	a.saveThemeEditorDraft(name, true)
}
