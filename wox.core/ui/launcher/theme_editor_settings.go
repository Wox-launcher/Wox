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
	"runtime"
	"strconv"
	"strings"
	"time"
	"wox/common"
	"wox/common/icons"

	"github.com/disintegration/imaging"
	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
	"wox/util/wallpaper"
)

const (
	demoWallpaperCacheVersion  = "v4-rounded-stage-1440x672-702x344"
	demoWallpaperWidth         = 1440
	demoWallpaperHeight        = 672
	demoWallpaperBlurredWidth  = 702
	demoWallpaperBlurredHeight = 344
	demoWallpaperCornerRadius  = 8
)

// buildThemeEditorSettingsSurface adapts the authored draft to the preview and property inspector.
func (c *themeSettingsController) buildThemeEditorSettingsSurface(state *themeEditorPreviewSnapshot, palette woxcomponent.ControlTheme, width, height, imageScale float32) woxwidget.Widget {
	groups := make([]launcherview.ThemeEditorColorGroup, 0, len(themeEditorColorGroups))
	formError := state.error
	var resolvedColors map[string]any
	if isV2Theme(state.raw) {
		resolvedColors = themeEditorResolvedColors(state.raw, state.values)
	}
	for _, group := range themeEditorGroups(state.raw) {
		label := c.language.translate(group.label)
		tokens := make([]launcherview.ThemeEditorColorToken, 0, len(group.tokens))
		for _, token := range group.tokens {
			value := state.values[token.key]
			if resolvedColors != nil && value == "" {
				value = themeMapString(resolvedColors, token.key)
			} else {
				value = themeEditorColorValue(state.raw, state.values, token.key)
			}
			if token.key == "AppBorderColor" && value == "" {
				value = themeEditorColorValue(state.raw, state.values, token.key)
			}
			numeric := isV2Theme(state.raw) && themeEditorNumericToken(token.key)
			color, ok := decodeThemeColor(value)
			if !ok {
				color = palette.Error
			}
			tokenLabel := c.language.translate(token.label)
			fieldError := ""
			if numeric && strings.TrimSpace(state.values[token.key]) != "" {
				if !validThemeEditorNumber(token.key, strings.TrimSpace(state.values[token.key])) {
					fieldError = c.language.translate("i18n:ui_theme_editor_invalid_geometry")
					if _, material := common.ThemeMaterialFieldLimit(token.key); material {
						fieldError = c.language.translate("i18n:ui_theme_editor_invalid_material")
					}
					// Numeric validation belongs to the field; keep save/service failures at page level.
					if formError == fieldError {
						formError = ""
					}
				}
			}
			step, upper := float64(0), float64(0)
			if limit, material := common.ThemeMaterialFieldLimit(token.key); material {
				step, upper = 0.1, limit
				if token.key == "ToolbarBlurSigma" {
					step = 0.5
				}
			}
			tokens = append(tokens, launcherview.ThemeEditorColorToken{Key: token.key, Label: tokenLabel, Color: color,
				Error:    fieldError,
				Subgroup: c.language.translate(themeEditorTokenSection(token.key)),
				Numeric:  numeric, NumericStep: step, NumericMax: upper, Value: state.values[token.key], Effective: value,
				Optional: isV2Theme(state.raw) && !strings.HasPrefix(token.key, "Base"),
			})
		}
		groups = append(groups, launcherview.ThemeEditorColorGroup{Label: label, Tokens: tokens})
	}

	foreground := palette.Text
	primaryForeground := palette.AccentText
	locateIcon := c.imageForTint(settingControlIconSource("locate"), &foreground, physicalImageSize(18, imageScale))
	discardIcon := c.imageForTint(settingControlIconSource("undo"), &foreground, physicalImageSize(18, imageScale))
	overwriteIcon := c.imageForTint(settingControlIconSource("overwrite"), &foreground, physicalImageSize(18, imageScale))
	saveAsIcon := c.imageForTint(settingControlIconSource("save"), &primaryForeground, physicalImageSize(18, imageScale))
	wallpaperImage := c.ThemeWallpaperImage()
	wallpaperBlurred := c.ThemeWallpaperBlurred()
	draftPalette := themeEditorDraftPalette(state.raw, state.values, state.surfaces)
	var geometry *woxcomponent.LauncherDemoGeometry
	if isV2Theme(state.raw) {
		geometry = &woxcomponent.LauncherDemoGeometry{AppPadding: draftPalette.appPadding, ResultPadding: draftPalette.resultContainerPadding, ItemPadding: draftPalette.resultItemPadding, ActionPadding: draftPalette.actionPadding, ToolbarPadding: draftPalette.toolbarPadding, ActionQueryRadius: draftPalette.actionQueryRadius}
	}

	assistantWidth, assistantHeight := launcherview.ThemeEditorInspectorSize(width, height, formError != "")
	dirty := themeEditorSnapshotDirty(state)
	return launcherview.ThemeEditorSettingsView(launcherview.ThemeEditorSettingsProps{
		Width: width, Height: height, Theme: palette, DraftTheme: draftPalette.componentTheme(),
		Geometry:  geometry,
		ModeLabel: c.language.translate("i18n:ui_theme_editor_properties"), AILabel: c.language.translate("i18n:ui_theme_editor_ai"),
		AIIcon: c.imageForTint(fromCoreImage(icons.Get(icons.ControlSparkles)), &palette.Text, physicalImageSize(16, imageScale)),
		OnSelectAI: func(open bool) {
			if current := c.ThemeEditor(); current != nil && current.ai.open != open {
				c.toggleThemeEditorAI()
			}
		},
		AIAssistant: c.buildThemeEditorAI(state, palette, assistantWidth, assistantHeight, imageScale),
		AIExpanded:  state.ai.open,
		Key:         state.key, Title: state.sourceName,
		DefaultLabel: c.language.translate("i18n:ui_theme_inherited"), ResetLabel: c.language.translate("i18n:ui_theme_restore_default"),
		LinkPaddingLabel: c.language.translate("i18n:ui_theme_editor_link_padding"), PaddingLabel: c.language.translate("i18n:ui_theme_editor_padding"),
		NoPropertiesLabel: c.language.translate("i18n:ui_theme_editor_no_properties"),
		ChromeHelp:        c.themeEditorChromeHelp(), ShowChromeHelp: isV2Theme(state.raw),
		OpacityLabel:   c.language.translate("i18n:ui_theme_editor_background_opacity"),
		OnChangeToken:  c.changeThemeEditorToken,
		OnChangeTokens: c.changeThemeEditorTokens,
		Groups:         groups, ActiveGroup: state.activeGroup, Dirty: dirty, Saving: state.saving, AIBusy: state.ai.busy, CanOverwrite: !state.isSystem && !state.isAuto && state.sourceID != "", Error: formError,
		Wallpaper: wallpaperImage, WallpaperBlurred: wallpaperBlurred,
		FlashToken: state.flashToken, DialogToken: state.dialogToken,
		LocateIcon: locateIcon, DiscardIcon: discardIcon, OverwriteIcon: overwriteIcon, SaveAsIcon: saveAsIcon,
		LocateLabel:  c.language.translate("i18n:ui_theme_editor_locate_token"),
		DiscardLabel: c.language.translate("i18n:ui_theme_editor_discard"), OverwriteLabel: c.language.translate("i18n:ui_save"), SaveAsLabel: c.language.translate("i18n:ui_theme_editor_save_as"), SavingLabel: c.language.translate("i18n:ui_theme_editor_saving"),
		PreviewResultTitle: c.language.translate("i18n:ui_theme_editor_preview_result_theme"), PreviewResultState: c.language.translate("i18n:ui_theme_editor_preview_result_current"),
		PropertyLabel: c.language.translate("i18n:ui_file_preview_property_size"),
		Window:        c.themeEditorNativeWindow(),
		QueryBoxLabel: c.language.translate("i18n:ui_theme_editor_preview_result_query"), ResultsLabel: c.language.translate("i18n:ui_theme_editor_group_results"),
		ToolbarCopyLabel: c.language.translate("i18n:ui_theme_editor_toolbar_copy"), ToolbarMoreLabel: c.language.translate("i18n:ui_theme_editor_toolbar_more"),
		Dialog:        c.buildThemeEditorSettingsDialog(state, palette, width, height),
		OnSelectGroup: c.selectThemeEditorGroup, OnEditToken: c.openThemeEditorTokenDialog, OnLocateToken: c.locateThemeEditorToken,
		OnDiscard: c.discardThemeEditorDraft, OnOverwrite: c.overwriteThemeEditorDraft, OnSaveAs: c.openThemeEditorSaveAsDialog,
	})
}

// changeThemeEditorToken shares draft validation and persistence with the color dialog.
func (c *themeSettingsController) changeThemeEditorToken(key, value string) {
	c.changeThemeEditorTokens(map[string]string{key: value})
}

// changeThemeEditorTokens commits linked padding as one edit so the preview never sees partial sides.
func (c *themeSettingsController) changeThemeEditorTokens(values map[string]string) {
	state := c.ThemeEditor()
	if state == nil || state.saving {
		return
	}
	for key, value := range values {
		index := themeEditorDefinitionIndex(state.definitions, key)
		if index < 0 {
			continue
		}
		setFormFieldsTextLocked(&state.formFieldsState, index, value)
		state.activeGroup = themeEditorGroupForToken(state.raw, key)
	}
	state.error = ""
	if _, err := themeEditorDraftTheme(state.raw, state.values); err != nil {
		state.error = c.language.translate("i18n:ui_theme_editor_invalid_geometry")
	} else {
		c.applySettingsThemeEditorDraft()
	}
	c.invalidateThemeEditorWindow()
}

// preloadDemoWallpaper loads the shared desktop image only while a preview-owning window is open.
func (c *themeSettingsController) preloadDemoWallpaper(includeBlurred bool) {
	if (c.ThemeWallpaperImage() != nil && (!includeBlurred || c.ThemeWallpaperBlurred() != nil)) || c.ThemeWallpaperLoading() {
		return
	}
	c.SetThemeWallpaperLoading(true)
	c.SetThemeWallpaperLoadID(c.ThemeWallpaperLoadID() + 1)
	loadID := c.ThemeWallpaperLoadID()
	path := c.ThemeWallpaperPath()
	util.Go(c.lifecycleCtx, "load theme editor wallpaper", func() {
		c.loadDemoWallpaper(loadID, path, includeBlurred)
	})
}

// loadDemoWallpaper resolves and decodes the desktop image without blocking UI rendering.
func (c *themeSettingsController) loadDemoWallpaper(loadID uint64, path string, includeBlurred bool) {
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
	_ = c.runOnUI("apply demo wallpaper", func() {
		if c.ThemeWallpaperLoadID() != loadID {
			return
		}
		c.SetThemeWallpaperLoading(false)
		settingsOpen = c.settings.open
		onboardingOpen = c.onboarding.isOpen()
		if err == nil && (settingsOpen || onboardingOpen) {
			c.SetThemeWallpaperPath(path)
			c.SetThemeWallpaperImage(wallpaperImage)
			c.SetThemeWallpaperBlurred(wallpaperBlurred)
		}
		if err == nil && settingsOpen {
			c.settings.invalidateSettingsWindow()
		}
		if err == nil && onboardingOpen {
			c.onboarding.invalidateOnboardingWindow()
		}
	})
	if err != nil {
		util.GetLogger().Error(c.lifecycleCtx, "load theme editor wallpaper: "+err.Error())
		return
	}
}

// releaseDemoWallpaperLocked prevents an in-flight load from restoring image memory after its window closes.
func (c *themeSettingsController) releaseDemoWallpaperLocked() {
	c.SetThemeWallpaperLoadID(c.ThemeWallpaperLoadID() + 1)
	c.SetThemeWallpaperPath("")
	c.SetThemeWallpaperImage(nil)
	c.SetThemeWallpaperBlurred(nil)
	c.SetThemeWallpaperLoading(false)
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
	maskDemoWallpaperRoundedCorners(stage, demoWallpaperCornerRadius)
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
	maskDemoWallpaperRoundedCorners(blurredWindow, demoWallpaperCornerRadius)
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

// themeEditorGroupForToken uses the same schema-aware groups as the visible editor.
func themeEditorGroupForToken(raw map[string]any, key string) int {
	for groupIndex, group := range themeEditorGroups(raw) {
		for _, token := range group.tokens {
			if token.key == key {
				return groupIndex
			}
		}
	}
	return -1
}

func (c *themeSettingsController) selectThemeEditorGroup(index int) {
	state := c.ThemeEditor()
	if state != nil && index >= 0 && index < len(themeEditorGroups(state.raw)) {
		state.activeGroup = index
		state.error = ""
	}
	c.invalidateThemeEditorWindow()
}

func (c *themeSettingsController) locateThemeEditorToken(key string) {
	state := c.ThemeEditor()
	if state == nil {
		return
	}
	group := themeEditorGroupForToken(state.raw, key)
	if group < 0 {
		return
	}
	state.activeGroup = group
	state.flashToken = key
	state.flashRevision++
	revision := state.flashRevision
	stateKey := state.key
	state.error = ""
	c.invalidateThemeEditorWindow()
	util.Go(c.lifecycleCtx, "clear theme editor token flash", func() {
		timer := time.NewTimer(780 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-c.lifecycleCtx.Done():
			return
		case <-timer.C:
		}
		_ = c.runOnUI("clear theme editor token flash", func() {
			current := c.ThemeEditor()
			if current != nil && current.key == stateKey && current.flashRevision == revision {
				current.flashToken = ""
			}
			c.invalidateThemeEditorWindow()
		})
	})
}

func (c *themeSettingsController) openThemeEditorTokenDialog(key string) {
	state := c.ThemeEditor()
	if state == nil || state.saving {
		return
	}
	index := themeEditorDefinitionIndex(state.definitions, key)
	if index < 0 {
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	group := themeEditorGroupForToken(state.raw, key)
	if group < 0 {
		return
	}
	state.activeGroup = group
	state.dialogMode = "token"
	state.dialogToken = key
	state.dialogOriginal = state.values[key]
	if color, ok := decodeThemeColor(state.dialogOriginal); ok {
		setFormFieldsTextLocked(&state.formFieldsState, index, encodeThemeColor(color))
	}
	state.error = ""
	setFormFieldsFocusLocked(&state.formFieldsState, index)
	textInput := state.editor != nil
	c.updateThemeEditorTextInput(textInput)
	c.invalidateThemeEditorWindow()
}

func (c *themeSettingsController) openThemeEditorSaveAsDialog() {
	state := c.ThemeEditor()
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
	defaultName := c.language.translate("i18n:ui_theme_editor_default_theme_name")
	baseName := strings.TrimSpace(state.sourceName)
	if baseName == "" {
		baseName = c.language.translate("i18n:ui_theme_editor_default_theme")
	}
	state.values["ThemeName"] = strings.ReplaceAll(defaultName, "{name}", baseName)
	setFormFieldsFocusLocked(&state.formFieldsState, index)
	if state.editor != nil {
		state.editor.SelectAll()
	}
	state.error = ""
	textInput := state.editor != nil
	c.updateThemeEditorTextInput(textInput)
	c.invalidateThemeEditorWindow()
}

func (c *themeSettingsController) buildThemeEditorSettingsDialog(state *themeEditorPreviewSnapshot, palette woxcomponent.ControlTheme, width, height float32) woxwidget.Widget {
	if state == nil || state.dialogMode == "" {
		return nil
	}
	index := themeEditorDefinitionIndex(state.definitions, state.dialogToken)
	if index < 0 {
		return nil
	}
	panelWidth := min(float32(420), max(float32(320), width-40))
	panelHeight := float32(176)
	callbacks := formFieldCallbacks{idPrefix: "theme-editor-dialog", focus: c.focusThemeEditorField, setText: c.setThemeEditorText, onKey: c.onThemeEditorPreviewKey}
	field := c.buildFormField(state.formFieldsSnapshot, callbacks, palette, index, state.definitions[index], panelWidth-32, 0)
	title := c.language.translate("i18n:ui_theme_editor_save_as_title")
	confirmLabel := c.language.translate("i18n:ui_theme_editor_save_as")
	if state.dialogMode == "token" {
		panelHeight = 456
		title = c.language.translate(state.definitions[index].Value.Label)
		confirmLabel = c.language.translate("i18n:ui_ok")
		selectedColor, ok := decodeThemeColor(themeEditorColorValue(state.raw, state.values, state.dialogToken))
		if !ok {
			selectedColor = woxui.Color{A: 255}
		}
		selectedHSV := themeColorToHSV(selectedColor)
		field = woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
			ID: "theme-editor-dialog-field-" + strconv.Itoa(index), Label: title, Width: 132, Height: 36, Radius: 4,
			Padding: woxwidget.Insets{Left: 8, Top: 8, Right: 8, Bottom: 7}, Transparent: true,
			BorderColor: palette.Text, BorderWidth: 1, Style: woxui.TextStyle{Size: 13},
			Value: state.values[state.dialogToken], Focused: state.active && state.focused == index,
			Window: c.themeEditorNativeWindow(), Theme: palette,
			OnFocusChange: func(focused bool) {
				if focused {
					c.focusThemeEditorField(index)
				}
			},
			OnChanged: func(value string) { c.setThemeEditorText(index, value) },
			OnKey:     c.onThemeEditorPreviewKey,
		})
		field = launcherview.ThemeEditorColorPicker(launcherview.ThemeEditorColorPickerProps{
			Color: selectedColor, Hue: selectedHSV.hue, Saturation: selectedHSV.saturation, Brightness: selectedHSV.value, Opacity: selectedHSV.alpha,
			BrightnessLabel: c.language.translate("i18n:ui_theme_editor_brightness"), OpacityLabel: c.language.translate("i18n:ui_theme_editor_opacity"),
			ColorField: field, Theme: palette,
			OnHueSaturation: c.setThemeEditorDialogHueSaturation, OnBrightnessChange: c.setThemeEditorDialogBrightness, OnOpacityChange: c.setThemeEditorDialogOpacity,
		})
	}
	buttons := []woxwidget.Widget{
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "theme-editor-dialog-cancel", Label: c.language.translate("i18n:ui_cancel"), Variant: woxcomponent.ButtonSecondary, OnTap: c.cancelThemeEditorDialog, Theme: palette}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "theme-editor-dialog-confirm", Label: confirmLabel, Variant: woxcomponent.ButtonPrimary, OnTap: c.confirmThemeEditorDialog, Theme: palette}),
	}
	if state.dialogMode == "token" && isV2Theme(state.raw) && !strings.HasPrefix(state.dialogToken, "Base") {
		buttons = append([]woxwidget.Widget{woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "theme-editor-dialog-reset", Label: c.language.translate("i18n:ui_theme_restore_default"), Variant: woxcomponent.ButtonSecondary, OnTap: c.resetThemeEditorToken, Theme: palette})}, buttons...)
	}
	footer := woxwidget.Align{Width: panelWidth - 32, Height: 46, Horizontal: 1, Child: woxwidget.Container{Height: 46, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: buttons}}}
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "theme-editor-dialog", Label: title, Width: panelWidth, Height: panelHeight, OverlayWidth: width, OverlayHeight: height,
		BackdropID: "theme-editor-dialog-backdrop", BackdropAlpha: 190, Padding: woxwidget.UniformInsets(16), Theme: palette, OnEscape: c.cancelThemeEditorDialog,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
			woxwidget.Container{Width: panelWidth - 32, Height: 28, Child: woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: palette.Text}},
			field,
			footer,
		}},
	})
}

// updateThemeEditorDialogColor applies every picker path through the same CSS value and live preview update.
func (c *themeSettingsController) updateThemeEditorDialogColor(update func(themeColorHSV) themeColorHSV) {
	state := c.ThemeEditor()
	if state == nil || state.dialogMode != "token" {
		return
	}
	color, ok := decodeThemeColor(themeEditorColorValue(state.raw, state.values, state.dialogToken))
	if !ok {
		return
	}
	next := encodeThemeColor(themeColorFromHSV(update(themeColorToHSV(color))))
	index := themeEditorDefinitionIndex(state.definitions, state.dialogToken)
	if setFormFieldsTextLocked(&state.formFieldsState, index, next) {
		state.error = ""
		c.applySettingsThemeEditorDraft()
		c.invalidateThemeEditorWindow()
	}
}

func (c *themeSettingsController) setThemeEditorDialogHueSaturation(hue, saturation float64) {
	c.updateThemeEditorDialogColor(func(color themeColorHSV) themeColorHSV {
		color.hue = hue
		color.saturation = saturation
		return color
	})
}

func (c *themeSettingsController) setThemeEditorDialogBrightness(value float64) {
	c.updateThemeEditorDialogColor(func(color themeColorHSV) themeColorHSV {
		color.value = value
		return color
	})
}

func (c *themeSettingsController) setThemeEditorDialogOpacity(value float64) {
	c.updateThemeEditorDialogColor(func(color themeColorHSV) themeColorHSV {
		color.alpha = value
		return color
	})
}

func (c *themeSettingsController) cancelThemeEditorDialog() {
	state := c.ThemeEditor()
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
	c.applySettingsThemeEditorDraft()
	c.restoreThemeEditorTextInput()
	c.invalidateThemeEditorWindow()
}

func (c *themeSettingsController) confirmThemeEditorDialog() {
	state := c.ThemeEditor()
	if state == nil || state.dialogMode == "" {
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	mode := state.dialogMode
	value := strings.TrimSpace(state.values[state.dialogToken])
	if mode == "token" {
		_, valid := decodeThemeColor(themeEditorColorValue(state.raw, state.values, state.dialogToken))
		if isV2Theme(state.raw) && strings.HasPrefix(state.dialogToken, "Base") && value == "" {
			valid = false
		}
		if !valid {
			state.error = c.language.translate("i18n:ui_theme_editor_invalid_color")
			c.invalidateThemeEditorWindow()
			return
		}
	} else if value == "" {
		state.error = c.language.translate("i18n:ui_theme_editor_name_required")
		c.invalidateThemeEditorWindow()
		return
	}
	state.values[state.dialogToken] = value
	state.dialogMode = ""
	state.dialogToken = ""
	state.dialogOriginal = ""
	state.active = false
	state.error = ""
	c.restoreThemeEditorTextInput()
	c.invalidateThemeEditorWindow()
	if mode == "save-as" {
		c.saveThemeEditorDraft(value, false)
	}
}

func (c *themeSettingsController) discardThemeEditorDraft() {
	state := c.ThemeEditor()
	if state == nil || state.saving {
		return
	}
	definitions := append([]formDefinition(nil), state.definitions...)
	state.formFieldsState = newFormFieldsState(definitions, state.initial, false)
	state.ai.undo = nil
	state.ai.history = nil
	state.dialogMode = ""
	state.dialogToken = ""
	state.dialogOriginal = ""
	state.error = ""
	c.applySettingsThemeEditorDraft()
	c.restoreThemeEditorTextInput()
	c.invalidateThemeEditorWindow()
}

func (c *themeSettingsController) overwriteThemeEditorDraft() {
	state := c.ThemeEditor()
	if state == nil || state.saving || state.isSystem || state.isAuto {
		return
	}
	name := state.sourceName
	c.saveThemeEditorDraft(name, true)
}

// themeEditorChromeHelp follows the native platform's custom-outline material policy.
func (c *themeSettingsController) themeEditorChromeHelp() string {
	if runtime.GOOS == "linux" {
		return c.language.translate("i18n:ui_theme_editor_chrome_help_linux")
	}
	return c.language.translate("i18n:ui_theme_editor_chrome_help")
}
