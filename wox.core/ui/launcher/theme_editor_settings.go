package launcher

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/png"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
	"wox/util/wallpaper"
)

const (
	demoWallpaperCacheVersion  = "v3-rounded-stage-1440x672-702x344"
	demoWallpaperWidth         = 1440
	demoWallpaperHeight        = 672
	demoWallpaperBlurredWidth  = 702
	demoWallpaperBlurredHeight = 344
)

// buildThemeEditorSettingsSurface adapts the shared draft controller to Flutter's settings-only editor layout.
func (a *App) buildThemeEditorSettingsSurface(state *themeEditorPreviewSnapshot, palette uiPalette, width, height, imageScale float32) woxwidget.Widget {
	groups := make([]launcherview.ThemeEditorColorGroup, 0, len(themeEditorColorGroups))
	for _, group := range themeEditorColorGroups {
		label := a.translate(group.label)
		labelWidth := float32(0)
		if a.window != nil {
			metrics, _ := a.window.MeasureText(label, woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold})
			labelWidth = metrics.Size.Width
		}
		tokens := make([]launcherview.ThemeEditorColorToken, 0, len(group.tokens))
		for _, token := range group.tokens {
			color, ok := decodeThemeColor(state.values[token.key])
			if !ok {
				color = palette.componentTheme().ErrorText
			}
			tokens = append(tokens, launcherview.ThemeEditorColorToken{Key: token.key, Label: a.translate(token.label), Color: color})
		}
		groups = append(groups, launcherview.ThemeEditorColorGroup{Label: label, LabelWidth: labelWidth, Tokens: tokens})
	}

	foreground := palette.resultTitle
	primaryForeground := palette.actionSelectedText
	locateIcon := a.imageForTint(settingControlIconSource("locate"), &foreground, physicalImageSize(18, imageScale))
	discardIcon := a.imageForTint(settingControlIconSource("undo"), &foreground, physicalImageSize(18, imageScale))
	overwriteIcon := a.imageForTint(settingControlIconSource("overwrite"), &foreground, physicalImageSize(18, imageScale))
	saveAsIcon := a.imageForTint(settingControlIconSource("save"), &primaryForeground, physicalImageSize(18, imageScale))
	wallpaperImage := a.themeSettings.ThemeWallpaperImage()
	wallpaperBlurred := a.themeSettings.ThemeWallpaperBlurred()
	draftPalette := themeEditorDraftPalette(state.raw, state.values)
	previewItemPadding := draftPalette.resultItemPadding
	previewItemPadding.Left += 5
	previewItemPadding.Right += 5
	measureTail := func(value string) float32 {
		metrics, _ := a.window.MeasureText(value, woxui.TextStyle{Size: 11})
		return metrics.Size.Width + 16
	}

	dirty := themeEditorSnapshotDirty(state)
	return launcherview.ThemeEditorSettingsView(launcherview.ThemeEditorSettingsProps{
		Width: width, Height: height, Theme: palette.componentTheme(), DraftTheme: draftPalette.componentTheme(),
		ResultTail: draftPalette.resultTail, SelectedTail: draftPalette.selectedTail,
		Groups: groups, ActiveGroup: state.activeGroup, Dirty: dirty, Saving: state.saving, CanOverwrite: !state.isSystem && !state.isAuto && state.sourceID != "", Error: state.error,
		Wallpaper: wallpaperImage, WallpaperBlurred: wallpaperBlurred,
		PreviewGeometry: launcherview.ThemeEditorPreviewGeometry{
			AppPadding: draftPalette.appPadding, QueryRadius: draftPalette.queryRadius, ResultContainerPadding: draftPalette.resultContainerPadding,
			ResultItemPadding: previewItemPadding, ResultItemRadius: draftPalette.resultItemRadius, ToolbarPadding: draftPalette.toolbarPadding,
		},
		FlashToken: state.flashToken,
		LocateIcon: locateIcon, DiscardIcon: discardIcon, OverwriteIcon: overwriteIcon, SaveAsIcon: saveAsIcon,
		LocateLabel:  a.translate("i18n:ui_theme_editor_locate_token"),
		DiscardLabel: a.translate("i18n:ui_theme_editor_discard"), OverwriteLabel: a.translate("i18n:ui_theme_editor_overwrite"), SaveAsLabel: a.translate("i18n:ui_theme_editor_save_as"), SavingLabel: a.translate("i18n:ui_theme_editor_saving"),
		PreviewResultTitle: a.translate("i18n:ui_theme_editor_preview_result_theme"), PreviewResultState: a.translate("i18n:ui_theme_editor_preview_result_current"),
		PreviewTailP1Width: measureTail("P1"), PreviewTail4msWidth: measureTail("4ms"), PreviewTail13msWidth: measureTail("13ms"),
		Window:        a.window,
		QueryBoxLabel: a.translate("i18n:ui_theme_editor_preview_result_query"), ResultsLabel: a.translate("i18n:ui_theme_editor_group_results"),
		ToolbarCopyLabel: a.translate("i18n:ui_theme_editor_toolbar_copy"), ToolbarMoreLabel: a.translate("i18n:ui_theme_editor_toolbar_more"),
		Dialog:        a.buildThemeEditorSettingsDialog(state, palette, width, height),
		OnSelectGroup: a.selectThemeEditorGroup, OnEditToken: a.openThemeEditorTokenDialog, OnLocateToken: a.locateThemeEditorToken,
		OnDiscard: a.discardThemeEditorDraft, OnOverwrite: a.overwriteThemeEditorDraft, OnSaveAs: a.openThemeEditorSaveAsDialog,
	})
}

// preloadDemoWallpaper loads the shared desktop image only while a preview-owning window is open.
func (a *App) preloadDemoWallpaper(includeBlurred bool) {
	if (a.themeSettings.ThemeWallpaperImage() != nil && (!includeBlurred || a.themeSettings.ThemeWallpaperBlurred() != nil)) || a.themeSettings.ThemeWallpaperLoading() {
		return
	}
	a.themeSettings.SetThemeWallpaperLoading(true)
	a.themeSettings.SetThemeWallpaperLoadID(a.themeSettings.ThemeWallpaperLoadID() + 1)
	loadID := a.themeSettings.ThemeWallpaperLoadID()
	path := a.themeSettings.ThemeWallpaperPath()
	util.Go(a.lifecycleCtx, "load theme editor wallpaper", func() {
		a.loadDemoWallpaper(loadID, path, includeBlurred)
	})
}

// loadDemoWallpaper resolves and decodes the desktop image without blocking UI rendering.
func (a *App) loadDemoWallpaper(loadID uint64, path string, includeBlurred bool) {
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
		wallpaperImage, wallpaperBlurred, err = decodeDemoWallpaper(path, includeBlurred)
	}
	settingsOpen := false
	onboardingOpen := false
	_ = a.runOnUI("apply demo wallpaper", func() {
		if a.themeSettings.ThemeWallpaperLoadID() != loadID {
			return
		}
		a.themeSettings.SetThemeWallpaperLoading(false)
		settingsOpen = a.settingsOpen
		onboardingOpen = a.onboardingOpen
		if err == nil && (settingsOpen || onboardingOpen) {
			a.themeSettings.SetThemeWallpaperPath(path)
			a.themeSettings.SetThemeWallpaperImage(wallpaperImage)
			a.themeSettings.SetThemeWallpaperBlurred(wallpaperBlurred)
		}
		if err == nil && settingsOpen {
			a.invalidateSettingsWindow()
		}
		if err == nil && onboardingOpen {
			a.invalidateOnboardingWindow()
		}
	})
	if err != nil {
		util.GetLogger().Error(a.lifecycleCtx, "load theme editor wallpaper: "+err.Error())
		return
	}
}

// releaseDemoWallpaperLocked prevents an in-flight load from restoring image memory after its window closes.
func (a *App) releaseDemoWallpaperLocked() {
	a.themeSettings.SetThemeWallpaperLoadID(a.themeSettings.ThemeWallpaperLoadID() + 1)
	a.themeSettings.SetThemeWallpaperPath("")
	a.themeSettings.SetThemeWallpaperImage(nil)
	a.themeSettings.SetThemeWallpaperBlurred(nil)
	a.themeSettings.SetThemeWallpaperLoading(false)
}

// decodeDemoWallpaper prepares the desktop image and optionally the blurred theme-editor crop.
func decodeDemoWallpaper(path string, includeBlurred bool) (*woxui.Image, *woxui.Image, error) {
	return decodeDemoWallpaperWithCache(path, includeBlurred, util.GetLocation().GetImageCacheDirectory())
}

// decodeDemoWallpaperWithCache loads processed previews first and generates them only on a cache miss.
func decodeDemoWallpaperWithCache(path string, includeBlurred bool, cacheDirectory string) (*woxui.Image, *woxui.Image, error) {
	cacheKey, err := demoWallpaperCacheKey(path)
	if err != nil {
		return nil, nil, err
	}
	wallpaperCachePath := filepath.Join(cacheDirectory, "demo_wallpaper_"+cacheKey+".png")
	blurredCachePath := filepath.Join(cacheDirectory, "demo_wallpaper_blurred_"+cacheKey+".png")
	if wallpaperImage, wallpaperBlurred, ok := loadDemoWallpaperCache(wallpaperCachePath, blurredCachePath, includeBlurred); ok {
		return wallpaperImage, wallpaperBlurred, nil
	}

	path, err = prepareWallpaperDecodePath(path, cacheDirectory, cacheKey)
	if err != nil {
		return nil, nil, err
	}

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
	stage := imaging.Fill(source, demoWallpaperWidth, demoWallpaperHeight, imaging.Center, imaging.Lanczos)
	maskDemoWallpaperRoundedCorners(stage, 29)
	wallpaperImage, err := woxui.NewImage(stage)
	if err != nil {
		return nil, nil, err
	}
	if !includeBlurred {
		persistDemoWallpaperCache(cacheDirectory, wallpaperCachePath, stage)
		return wallpaperImage, nil, nil
	}
	logicalStage := imaging.Resize(stage, 900, 420, imaging.Lanczos)
	blurredStage := imaging.Blur(logicalStage, 24)
	blurredWindow := imaging.CropCenter(blurredStage, demoWallpaperBlurredWidth, demoWallpaperBlurredHeight)
	maskDemoWallpaperRoundedCorners(blurredWindow, 12)
	wallpaperBlurred, err := woxui.NewImage(blurredWindow)
	if err != nil {
		return nil, nil, err
	}
	persistDemoWallpaperCache(cacheDirectory, wallpaperCachePath, stage)
	persistDemoWallpaperCache(cacheDirectory, blurredCachePath, blurredWindow)
	return wallpaperImage, wallpaperBlurred, nil
}

// prepareWallpaperDecodePath converts wallpaper formats that Go cannot decode directly into a cached PNG.
func prepareWallpaperDecodePath(path, cacheDirectory, cacheKey string) (string, error) {
	if !strings.EqualFold(filepath.Ext(path), ".jxl") {
		return path, nil
	}

	transcodedPath := filepath.Join(cacheDirectory, "demo_wallpaper_source_"+cacheKey+".png")
	if _, err := os.Stat(transcodedPath); err == nil {
		return transcodedPath, nil
	}
	if err := os.MkdirAll(cacheDirectory, 0755); err != nil {
		return "", err
	}

	if err := transcodeWallpaperToPNG(path, transcodedPath); err != nil {
		return "", err
	}
	return transcodedPath, nil
}

// transcodeWallpaperToPNG uses a local JXL decoder when the wallpaper format is not supported by image.Decode.
func transcodeWallpaperToPNG(sourcePath, targetPath string) error {
	if err := runWallpaperTranscoder("djxl", sourcePath, targetPath); err == nil {
		return nil
	}
	_ = os.Remove(targetPath)
	if err := runWallpaperTranscoder("magick", sourcePath, targetPath); err == nil {
		return nil
	}
	_ = os.Remove(targetPath)
	return errors.New("desktop wallpaper is in a format that could not be transcoded")
}

func runWallpaperTranscoder(command, sourcePath, targetPath string) error {
	if _, err := exec.LookPath(command); err != nil {
		return err
	}
	cmd := exec.Command(command, sourcePath, targetPath)
	return cmd.Run()
}

// demoWallpaperCacheKey invalidates processed previews when the source or rendering contract changes.
func demoWallpaperCacheKey(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	_, _ = io.WriteString(hash, demoWallpaperCacheVersion+"|")
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)[:12]), nil
}

// loadDemoWallpaperCache requires the complete cache pair before replacing the lightweight placeholder.
func loadDemoWallpaperCache(wallpaperPath, blurredPath string, includeBlurred bool) (*woxui.Image, *woxui.Image, bool) {
	wallpaperImage, err := decodeDemoWallpaperCacheFile(wallpaperPath)
	if err != nil {
		return nil, nil, false
	}
	if !includeBlurred {
		return wallpaperImage, nil, true
	}
	wallpaperBlurred, err := decodeDemoWallpaperCacheFile(blurredPath)
	if err != nil {
		return nil, nil, false
	}
	return wallpaperImage, wallpaperBlurred, true
}

// decodeDemoWallpaperCacheFile loads one bounded processed preview instead of the original desktop image.
func decodeDemoWallpaperCacheFile(path string) (*woxui.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoded, err := woxui.DecodeImage(file)
	if err == nil {
		now := time.Now()
		_ = os.Chtimes(path, now, now)
	}
	return decoded, err
}

// persistDemoWallpaperCache keeps disk failures from hiding the already-generated in-memory preview.
func persistDemoWallpaperCache(cacheDirectory, path string, source image.Image) {
	if err := os.MkdirAll(cacheDirectory, 0755); err != nil {
		log.Printf("create demo wallpaper cache directory: %v", err)
		return
	}
	if err := writeDemoWallpaperCache(path, source); err != nil {
		log.Printf("write demo wallpaper cache: %v", err)
	}
}

// writeDemoWallpaperCache publishes a complete PNG without exposing partial files to readers.
func writeDemoWallpaperCache(path string, source image.Image) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := png.Encode(temporary, source); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

// maskDemoWallpaperRoundedCorners keeps preprocessed wallpaper layers inside the same rounded bounds as Flutter's clips.
func maskDemoWallpaperRoundedCorners(source *image.NRGBA, radius int) {
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
	state := a.themeSettings.ThemeEditor()
	if state != nil && index >= 0 && index < len(themeEditorColorGroups) {
		state.activeGroup = index
		state.error = ""
	}
	a.invalidateThemeEditorWindow()
}

func (a *App) locateThemeEditorToken(key string) {
	state := a.themeSettings.ThemeEditor()
	if state == nil {
		return
	}
	state.activeGroup = themeEditorGroupForToken(key)
	state.flashToken = key
	state.flashRevision++
	revision := state.flashRevision
	stateKey := state.key
	state.error = ""
	a.invalidateThemeEditorWindow()
	util.Go(a.lifecycleCtx, "clear theme editor token flash", func() {
		timer := time.NewTimer(780 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-a.lifecycleCtx.Done():
			return
		case <-timer.C:
		}
		_ = a.runOnUI("clear theme editor token flash", func() {
			current := a.themeSettings.ThemeEditor()
			if current != nil && current.key == stateKey && current.flashRevision == revision {
				current.flashToken = ""
			}
			a.invalidateThemeEditorWindow()
		})
	})
}

func (a *App) openThemeEditorTokenDialog(key string) {
	state := a.themeSettings.ThemeEditor()
	if state == nil || state.saving {
		return
	}
	index := themeEditorDefinitionIndex(state.definitions, key)
	if index < 0 {
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	state.activeGroup = themeEditorGroupForToken(key)
	state.dialogMode = "token"
	state.dialogToken = key
	state.dialogOriginal = state.values[key]
	if color, ok := decodeThemeColor(state.dialogOriginal); ok {
		setFormFieldsTextLocked(&state.formFieldsState, index, encodeThemeColor(color))
	}
	state.error = ""
	setFormFieldsFocusLocked(&state.formFieldsState, index)
	textInput := state.editor != nil
	a.updateThemeEditorTextInput(textInput)
	a.invalidateThemeEditorWindow()
}

func (a *App) openThemeEditorSaveAsDialog() {
	state := a.themeSettings.ThemeEditor()
	if state == nil || state.saving {
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	index := themeEditorDefinitionIndex(state.definitions, "ThemeName")
	if index < 0 {
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
	field := a.buildFormField(state.formFieldsSnapshot, callbacks, palette, index, state.definitions[index], panelWidth-32, 0)
	title := a.translate("i18n:ui_theme_editor_save_as_title")
	confirmLabel := a.translate("i18n:ui_theme_editor_save_as")
	if state.dialogMode == "token" {
		panelHeight = 456
		for _, token := range themeEditorTokens() {
			if token.key == state.dialogToken {
				title = a.translate(token.label)
				break
			}
		}
		confirmLabel = a.translate("i18n:ui_ok")
		selectedColor, ok := decodeThemeColor(state.values[state.dialogToken])
		if !ok {
			selectedColor = woxui.Color{A: 255}
		}
		selectedHSV := themeColorToHSV(selectedColor)
		field = woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
			ID: "theme-editor-dialog-field-" + strconv.Itoa(index), Label: title, Width: 132, Height: 36, Radius: 4,
			Padding: woxwidget.Insets{Left: 8, Top: 8, Right: 8, Bottom: 7}, Transparent: true,
			BorderColor: palette.actionText, BorderWidth: 1, Style: woxui.TextStyle{Size: 13},
			Value: state.values[state.dialogToken], Focused: state.active && state.focused == index,
			Window: a.formFieldNativeWindow("theme-editor-dialog"), Theme: palette.componentTheme(),
			OnFocusChange: func(focused bool) {
				if focused {
					a.focusThemeEditorField(index)
				}
			},
			OnChanged: func(value string) { a.setThemeEditorText(index, value) },
			OnKey:     a.onThemeEditorPreviewKey,
		})
		field = launcherview.ThemeEditorColorPicker(launcherview.ThemeEditorColorPickerProps{
			Color: selectedColor, Hue: selectedHSV.hue, Saturation: selectedHSV.saturation, Brightness: selectedHSV.value, Opacity: selectedHSV.alpha,
			BrightnessLabel: a.translate("i18n:ui_theme_editor_brightness"), OpacityLabel: a.translate("i18n:ui_theme_editor_opacity"),
			ColorField: field, Theme: palette.componentTheme(),
			OnHueSaturation: a.setThemeEditorDialogHueSaturation, OnBrightnessChange: a.setThemeEditorDialogBrightness, OnOpacityChange: a.setThemeEditorDialogOpacity,
		})
	}
	footer := woxwidget.Container{Width: panelWidth - 32, Height: 46, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		woxwidget.Container{Width: max(float32(0), panelWidth-32-210), Height: 38},
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "theme-editor-dialog-cancel", Label: a.translate("i18n:ui_cancel"), Width: 96, Height: 36, Variant: woxcomponent.ButtonOutline, OnTap: a.cancelThemeEditorDialog, Theme: palette.componentTheme()}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "theme-editor-dialog-confirm", Label: confirmLabel, Width: 104, Height: 36, Variant: woxcomponent.ButtonPrimary, OnTap: a.confirmThemeEditorDialog, Theme: palette.componentTheme()}),
	}}}
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "theme-editor-dialog", Label: title, Width: panelWidth, Height: panelHeight, OverlayWidth: width, OverlayHeight: height,
		BackdropID: "theme-editor-dialog-backdrop", BackdropAlpha: 190, Padding: woxwidget.UniformInsets(16), Theme: palette.componentTheme(), OnEscape: a.cancelThemeEditorDialog,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
			woxwidget.Container{Width: panelWidth - 32, Height: 28, Child: woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: palette.actionText}},
			field,
			footer,
		}},
	})
}

// updateThemeEditorDialogColor applies every picker path through the same CSS value and live preview update.
func (a *App) updateThemeEditorDialogColor(update func(themeColorHSV) themeColorHSV) {
	state := a.themeSettings.ThemeEditor()
	if state == nil || state.dialogMode != "token" {
		return
	}
	color, ok := decodeThemeColor(state.values[state.dialogToken])
	if !ok {
		return
	}
	next := encodeThemeColor(themeColorFromHSV(update(themeColorToHSV(color))))
	index := themeEditorDefinitionIndex(state.definitions, state.dialogToken)
	if setFormFieldsTextLocked(&state.formFieldsState, index, next) {
		state.error = ""
		a.applySettingsThemeEditorDraft()
		a.invalidateThemeEditorWindow()
	}
}

func (a *App) setThemeEditorDialogHueSaturation(hue, saturation float64) {
	a.updateThemeEditorDialogColor(func(color themeColorHSV) themeColorHSV {
		color.hue = hue
		color.saturation = saturation
		return color
	})
}

func (a *App) setThemeEditorDialogBrightness(value float64) {
	a.updateThemeEditorDialogColor(func(color themeColorHSV) themeColorHSV {
		color.value = value
		return color
	})
}

func (a *App) setThemeEditorDialogOpacity(value float64) {
	a.updateThemeEditorDialogColor(func(color themeColorHSV) themeColorHSV {
		color.alpha = value
		return color
	})
}

func (a *App) cancelThemeEditorDialog() {
	state := a.themeSettings.ThemeEditor()
	if state == nil || state.dialogMode == "" {
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
	a.applySettingsThemeEditorDraft()
	a.restoreThemeEditorTextInput()
	a.invalidateThemeEditorWindow()
}

func (a *App) confirmThemeEditorDialog() {
	state := a.themeSettings.ThemeEditor()
	if state == nil || state.dialogMode == "" {
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	mode := state.dialogMode
	value := strings.TrimSpace(state.values[state.dialogToken])
	if mode == "token" {
		if _, ok := decodeThemeColor(value); !ok {
			state.error = a.translate("i18n:ui_theme_editor_invalid_color")
			a.invalidateThemeEditorWindow()
			return
		}
	} else if value == "" {
		state.error = a.translate("i18n:ui_theme_editor_name_required")
		a.invalidateThemeEditorWindow()
		return
	}
	state.values[state.dialogToken] = value
	state.dialogMode = ""
	state.dialogToken = ""
	state.dialogOriginal = ""
	state.active = false
	state.error = ""
	a.restoreThemeEditorTextInput()
	a.invalidateThemeEditorWindow()
	if mode == "save-as" {
		a.saveThemeEditorDraft(value, false)
	}
}

func (a *App) discardThemeEditorDraft() {
	state := a.themeSettings.ThemeEditor()
	if state == nil || state.saving {
		return
	}
	definitions := append([]formDefinition(nil), state.definitions...)
	state.formFieldsState = newFormFieldsState(definitions, state.initial, false)
	state.dialogMode = ""
	state.dialogToken = ""
	state.dialogOriginal = ""
	state.error = ""
	a.applySettingsThemeEditorDraft()
	a.restoreThemeEditorTextInput()
	a.invalidateThemeEditorWindow()
}

func (a *App) overwriteThemeEditorDraft() {
	state := a.themeSettings.ThemeEditor()
	if state == nil || state.saving || state.isSystem || state.isAuto {
		return
	}
	name := state.sourceName
	a.saveThemeEditorDraft(name, true)
}
