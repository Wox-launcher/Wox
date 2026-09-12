package launcher

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"runtime"
	"strconv"
	"strings"
	"time"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
	"wox/util/overlay"
)

type themeData struct {
	PreviewBackgroundColor                     string
	PreviewBorderColor                         string
	PreviewBorderRadius                        *int
	ResultItemActiveIndicatorWidth             *int
	ResultItemActiveIndicatorInsetLeft         *int
	ResultItemActiveIndicatorInsetTop          *int
	ResultItemActiveIndicatorInsetBottom       *int
	ResultItemActiveIndicatorBorderRadius      *int
	QueryBoxBorderBottomWidth                  *int
	ResultItemActiveIndicatorColor             string
	QueryBoxBorderBottomColor                  string
	PreviewTagBorderRadius                     *int
	PreviewTagFontColor                        string
	PreviewTagBackgroundColor                  string
	PreviewTagBorderColor                      string
	GlanceFontColor                            string
	GlanceIconColor                            string
	GlanceBackgroundColor                      string
	RefinementButtonFontColor                  string
	RefinementButtonIconColor                  string
	RefinementButtonBackgroundColor            string
	RefinementButtonBorderColor                string
	RefinementButtonHoverBackgroundColor       string
	RefinementButtonActiveFontColor            string
	RefinementButtonActiveIconColor            string
	RefinementButtonActiveBackgroundColor      string
	RefinementButtonActiveBorderColor          string
	RefinementButtonActiveHoverBackgroundColor string
	RefinementBackgroundColor                  string
	RefinementBorderColor                      string
	RefinementTitleColor                       string
	RefinementDividerColor                     string
	RefinementHotkeyColor                      string
	RefinementItemFontColor                    string
	RefinementItemBackgroundColor              string
	RefinementItemHoverBackgroundColor         string
	RefinementItemActiveFontColor              string
	RefinementItemActiveBackgroundColor        string
	RefinementItemActiveHoverBackgroundColor   string
	ScrollbarThumbColor                        string
	ScrollbarThumbHoverColor                   string
	ScrollbarThumbActiveColor                  string
	ScrollbarWidth                             *int
	ScrollbarHoverWidth                        *int
	ScrollbarBorderRadius                      *int
	AppWindowChrome                            bool
	AppBorderColor                             string
	AppBorderWidth                             *int
	AppBorderRadius                            *int
	GlanceHoverBackgroundColor                 string

	ActionContainerDividerColor           string
	ToolbarHotkeyFontColor                string
	ActionItemHotkeyFontColor             string
	ActionItemActiveHotkeyFontColor       string
	ToolbarHotkeyBackgroundColor          string
	ActionItemHotkeyBackgroundColor       string
	ActionItemActiveHotkeyBackgroundColor string
	ToolbarHotkeyBorderColor              string
	ActionItemHotkeyBorderColor           string
	ActionItemActiveHotkeyBorderColor     string
	ResultItemHoverBackgroundColor        string

	AppBackgroundColor                   string
	AppPaddingLeft                       int
	AppPaddingTop                        int
	AppPaddingRight                      int
	AppPaddingBottom                     int
	ResultContainerPaddingLeft           int
	ResultContainerPaddingTop            int
	ResultContainerPaddingRight          int
	ResultContainerPaddingBottom         int
	ResultItemBorderRadius               int
	ResultItemPaddingLeft                int
	ResultItemPaddingTop                 int
	ResultItemPaddingRight               int
	ResultItemPaddingBottom              int
	ResultItemTitleColor                 string
	ResultItemSubTitleColor              string
	ResultItemTailTextColor              string
	ResultItemActiveBackgroundColor      string
	ResultItemActiveBorderLeftWidth      int
	ResultItemActiveBorderLeftColor      string
	ResultItemActiveTitleColor           string
	ResultItemActiveSubTitleColor        string
	ResultItemActiveTailTextColor        string
	QueryBoxFontColor                    string
	QueryBoxBackgroundColor              string
	QueryBoxBorderRadius                 int
	QueryBoxCursorColor                  string
	QueryBoxTextSelectionBackgroundColor string
	QueryBoxTextSelectionColor           string
	ActionContainerBackgroundColor       string
	ActionContainerBorderColor           string
	ActionContainerBorderWidth           *int
	ActionContainerHeaderFontColor       string
	ActionContainerBorderRadius          *int
	ActionItemBorderRadius               *int
	ActionContainerPaddingLeft           int
	ActionContainerPaddingTop            int
	ActionContainerPaddingRight          int
	ActionContainerPaddingBottom         int
	ActionItemActiveBackgroundColor      string
	ActionItemActiveFontColor            string
	ActionItemFontColor                  string
	ActionQueryBoxFontColor              string
	ActionQueryBoxBackgroundColor        string
	ActionQueryBoxBorderRadius           int
	PreviewFontColor                     string
	PreviewSplitLineColor                string
	PreviewPropertyTitleColor            string
	PreviewPropertyContentColor          string
	ToolbarFontColor                     string
	ToolbarBackgroundColor               string
	ToolbarBorderColor                   string
	ToolbarBorderWidth                   *int
	ToolbarPaddingLeft                   int
	ToolbarPaddingRight                  int
}

type uiPalette struct {
	PreviewBackgroundColor                     *woxui.Color
	PreviewBorderColor                         *woxui.Color
	PreviewBorderRadius                        *int
	ResultItemActiveIndicatorWidth             *int
	ResultItemActiveIndicatorInsetLeft         *int
	ResultItemActiveIndicatorInsetTop          *int
	ResultItemActiveIndicatorInsetBottom       *int
	ResultItemActiveIndicatorBorderRadius      *int
	QueryBoxBorderBottomWidth                  *int
	ResultItemActiveIndicatorColor             *woxui.Color
	QueryBoxBorderBottomColor                  *woxui.Color
	PreviewTagBorderRadius                     *int
	PreviewTagFontColor                        *woxui.Color
	PreviewTagBackgroundColor                  *woxui.Color
	PreviewTagBorderColor                      *woxui.Color
	GlanceFontColor                            *woxui.Color
	GlanceIconColor                            *woxui.Color
	GlanceBackgroundColor                      *woxui.Color
	RefinementButtonFontColor                  *woxui.Color
	RefinementButtonIconColor                  *woxui.Color
	RefinementButtonBackgroundColor            *woxui.Color
	RefinementButtonBorderColor                *woxui.Color
	RefinementButtonHoverBackgroundColor       *woxui.Color
	RefinementButtonActiveFontColor            *woxui.Color
	RefinementButtonActiveIconColor            *woxui.Color
	RefinementButtonActiveBackgroundColor      *woxui.Color
	RefinementButtonActiveBorderColor          *woxui.Color
	RefinementButtonActiveHoverBackgroundColor *woxui.Color
	RefinementBackgroundColor                  *woxui.Color
	RefinementBorderColor                      *woxui.Color
	RefinementTitleColor                       *woxui.Color
	RefinementDividerColor                     *woxui.Color
	RefinementHotkeyColor                      *woxui.Color
	RefinementItemFontColor                    *woxui.Color
	RefinementItemBackgroundColor              *woxui.Color
	RefinementItemHoverBackgroundColor         *woxui.Color
	RefinementItemActiveFontColor              *woxui.Color
	RefinementItemActiveBackgroundColor        *woxui.Color
	RefinementItemActiveHoverBackgroundColor   *woxui.Color
	ScrollbarThumbColor                        *woxui.Color
	ScrollbarThumbHoverColor                   *woxui.Color
	ScrollbarThumbActiveColor                  *woxui.Color
	ScrollbarWidth                             *int
	ScrollbarHoverWidth                        *int
	ScrollbarBorderRadius                      *int
	AppWindowChrome                            bool
	AppBorderColor                             *woxui.Color
	AppBorderWidth                             *int
	AppBorderRadius                            *int
	GlanceHoverBackgroundColor                 *woxui.Color

	ActionContainerDividerColor           *woxui.Color
	ToolbarHotkeyFontColor                *woxui.Color
	ActionItemHotkeyFontColor             *woxui.Color
	ActionItemActiveHotkeyFontColor       *woxui.Color
	ToolbarHotkeyBackgroundColor          *woxui.Color
	ActionItemHotkeyBackgroundColor       *woxui.Color
	ActionItemActiveHotkeyBackgroundColor *woxui.Color
	ToolbarHotkeyBorderColor              *woxui.Color
	ActionItemHotkeyBorderColor           *woxui.Color
	ActionItemActiveHotkeyBorderColor     *woxui.Color
	ResultItemHoverBackgroundColor        *woxui.Color

	selectedBorderLeftWidth float32
	actionBorderWidth       float32
	actionContainerRadius   float32
	actionItemRadius        float32
	toolbarBorderWidth      float32
	selectedBorderLeftColor woxui.Color
	toolbarBorder           woxui.Color

	background             woxui.Color
	appPadding             woxwidget.Insets
	queryBackground        woxui.Color
	queryRadius            float32
	queryText              woxui.Color
	cursor                 woxui.Color
	selectionBackground    woxui.Color
	selectionText          woxui.Color
	resultTitle            woxui.Color
	resultSubtitle         woxui.Color
	resultTail             woxui.Color
	resultContainerPadding woxwidget.Insets
	resultItemPadding      woxwidget.Insets
	resultItemRadius       float32
	selectedBackground     woxui.Color
	selectedTitle          woxui.Color
	selectedSubtitle       woxui.Color
	selectedTail           woxui.Color
	actionBackground       woxui.Color
	actionBorder           woxui.Color
	actionHeader           woxui.Color
	actionPadding          woxwidget.Insets
	actionSelected         woxui.Color
	actionSelectedText     woxui.Color
	actionText             woxui.Color
	actionQueryBackground  woxui.Color
	actionQueryText        woxui.Color
	actionQueryRadius      float32
	previewText            woxui.Color
	previewSplit           woxui.Color
	previewPropertyTitle   woxui.Color
	previewPropertyContent woxui.Color
	toolbarBackground      woxui.Color
	toolbarText            woxui.Color
	toolbarPadding         woxwidget.Insets
}

// componentTheme exposes launcher appearance through the stable component package boundary.
func (palette uiPalette) componentTheme() woxcomponent.Theme {
	return woxcomponent.Theme{
		PreviewBackgroundColor:                     palette.PreviewBackgroundColor,
		PreviewBorderColor:                         palette.PreviewBorderColor,
		PreviewBorderRadius:                        palette.PreviewBorderRadius,
		ResultItemActiveIndicatorWidth:             palette.ResultItemActiveIndicatorWidth,
		ResultItemActiveIndicatorInsetLeft:         palette.ResultItemActiveIndicatorInsetLeft,
		ResultItemActiveIndicatorInsetTop:          palette.ResultItemActiveIndicatorInsetTop,
		ResultItemActiveIndicatorInsetBottom:       palette.ResultItemActiveIndicatorInsetBottom,
		ResultItemActiveIndicatorBorderRadius:      palette.ResultItemActiveIndicatorBorderRadius,
		QueryBoxBorderBottomWidth:                  palette.QueryBoxBorderBottomWidth,
		ResultItemActiveIndicatorColor:             palette.ResultItemActiveIndicatorColor,
		QueryBoxBorderBottomColor:                  palette.QueryBoxBorderBottomColor,
		PreviewTagBorderRadius:                     palette.PreviewTagBorderRadius,
		PreviewTagFontColor:                        palette.PreviewTagFontColor,
		PreviewTagBackgroundColor:                  palette.PreviewTagBackgroundColor,
		PreviewTagBorderColor:                      palette.PreviewTagBorderColor,
		GlanceFontColor:                            palette.GlanceFontColor,
		GlanceIconColor:                            palette.GlanceIconColor,
		GlanceBackgroundColor:                      palette.GlanceBackgroundColor,
		RefinementButtonFontColor:                  palette.RefinementButtonFontColor,
		RefinementButtonIconColor:                  palette.RefinementButtonIconColor,
		RefinementButtonBackgroundColor:            palette.RefinementButtonBackgroundColor,
		RefinementButtonBorderColor:                palette.RefinementButtonBorderColor,
		RefinementButtonHoverBackgroundColor:       palette.RefinementButtonHoverBackgroundColor,
		RefinementButtonActiveFontColor:            palette.RefinementButtonActiveFontColor,
		RefinementButtonActiveIconColor:            palette.RefinementButtonActiveIconColor,
		RefinementButtonActiveBackgroundColor:      palette.RefinementButtonActiveBackgroundColor,
		RefinementButtonActiveBorderColor:          palette.RefinementButtonActiveBorderColor,
		RefinementButtonActiveHoverBackgroundColor: palette.RefinementButtonActiveHoverBackgroundColor,
		RefinementBackgroundColor:                  palette.RefinementBackgroundColor,
		RefinementBorderColor:                      palette.RefinementBorderColor,
		RefinementTitleColor:                       palette.RefinementTitleColor,
		RefinementDividerColor:                     palette.RefinementDividerColor,
		RefinementHotkeyColor:                      palette.RefinementHotkeyColor,
		RefinementItemFontColor:                    palette.RefinementItemFontColor,
		RefinementItemBackgroundColor:              palette.RefinementItemBackgroundColor,
		RefinementItemHoverBackgroundColor:         palette.RefinementItemHoverBackgroundColor,
		RefinementItemActiveFontColor:              palette.RefinementItemActiveFontColor,
		RefinementItemActiveBackgroundColor:        palette.RefinementItemActiveBackgroundColor,
		RefinementItemActiveHoverBackgroundColor:   palette.RefinementItemActiveHoverBackgroundColor,
		ScrollbarThumbColor:                        palette.ScrollbarThumbColor,
		ScrollbarThumbHoverColor:                   palette.ScrollbarThumbHoverColor,
		ScrollbarThumbActiveColor:                  palette.ScrollbarThumbActiveColor,
		ScrollbarWidth:                             palette.ScrollbarWidth,
		ScrollbarHoverWidth:                        palette.ScrollbarHoverWidth,
		ScrollbarBorderRadius:                      palette.ScrollbarBorderRadius,
		AppWindowChrome:                            palette.AppWindowChrome,
		AppBorderColor:                             palette.AppBorderColor,
		AppBorderWidth:                             palette.AppBorderWidth,
		AppBorderRadius:                            palette.AppBorderRadius,
		GlanceHoverBackgroundColor:                 palette.GlanceHoverBackgroundColor,

		ActionContainerDividerColor:           palette.ActionContainerDividerColor,
		ToolbarHotkeyFontColor:                palette.ToolbarHotkeyFontColor,
		ActionItemHotkeyFontColor:             palette.ActionItemHotkeyFontColor,
		ActionItemActiveHotkeyFontColor:       palette.ActionItemActiveHotkeyFontColor,
		ToolbarHotkeyBackgroundColor:          palette.ToolbarHotkeyBackgroundColor,
		ActionItemHotkeyBackgroundColor:       palette.ActionItemHotkeyBackgroundColor,
		ActionItemActiveHotkeyBackgroundColor: palette.ActionItemActiveHotkeyBackgroundColor,
		ToolbarHotkeyBorderColor:              palette.ToolbarHotkeyBorderColor,
		ActionItemHotkeyBorderColor:           palette.ActionItemHotkeyBorderColor,
		ActionItemActiveHotkeyBorderColor:     palette.ActionItemActiveHotkeyBorderColor,
		ResultItemHoverBackgroundColor:        palette.ResultItemHoverBackgroundColor,

		SelectedBorderLeftWidth: palette.selectedBorderLeftWidth,
		ActionBorderWidth:       palette.actionBorderWidth,
		ActionContainerRadius:   palette.actionContainerRadius,
		ActionItemRadius:        palette.actionItemRadius,
		ToolbarBorderWidth:      palette.toolbarBorderWidth,
		SelectedBorderLeftColor: palette.selectedBorderLeftColor,
		ToolbarBorder:           palette.toolbarBorder,

		Background:             palette.background,
		QueryBackground:        palette.queryBackground,
		QueryText:              palette.queryText,
		Cursor:                 palette.cursor,
		SelectionBackground:    palette.selectionBackground,
		SelectionText:          palette.selectionText,
		ResultTitle:            palette.resultTitle,
		ResultSubtitle:         palette.resultSubtitle,
		ResultTail:             palette.resultTail,
		ErrorText:              woxui.Color{R: 232, G: 95, B: 95, A: 255},
		SelectedBackground:     palette.selectedBackground,
		SelectedTitle:          palette.selectedTitle,
		SelectedSubtitle:       palette.selectedSubtitle,
		SelectedTail:           palette.selectedTail,
		ActionBackground:       palette.actionBackground,
		ActionBorder:           palette.actionBorder,
		ActionHeader:           palette.actionHeader,
		ActionText:             palette.actionText,
		ActionSelected:         palette.actionSelected,
		ActionSelectedText:     palette.actionSelectedText,
		PreviewText:            palette.previewText,
		PreviewSplit:           palette.previewSplit,
		PreviewPropertyTitle:   palette.previewPropertyTitle,
		PreviewPropertyContent: palette.previewPropertyContent,
		ToolbarBackground:      palette.toolbarBackground,
		ToolbarText:            palette.toolbarText,
	}
}

// usesCustomWindowChrome follows authored outline fields, not resolved color defaults.
func (theme themeData) usesCustomWindowChrome() bool {
	return theme.AppWindowChrome || theme.AppBorderWidth != nil || theme.AppBorderRadius != nil
}

// opaqueWindowBackground disables unsupported desktop translucency without changing component blending.
func opaqueWindowBackground(color woxui.Color, customChrome bool) woxui.Color {
	if runtime.GOOS == "linux" && (customChrome || !woxui.HasNativeWindowMaterial()) {
		color.A = 255
	}
	return color
}

func defaultPalette() uiPalette {
	return uiPalette{
		actionBorderWidth:       1,
		actionContainerRadius:   8,
		actionItemRadius:        8,
		toolbarBorderWidth:      1,
		selectedBorderLeftColor: woxui.Color{R: 57, G: 204, B: 183, A: 255},
		toolbarBorder:           woxui.Color{R: 166, G: 176, B: 190, A: 26},
		actionBorder:            woxui.Color{R: 85, G: 96, B: 112, A: 150},

		background:             opaqueWindowBackground(woxui.Color{R: 24, G: 29, B: 38, A: 242}, false),
		appPadding:             woxwidget.UniformInsets(10),
		queryBackground:        woxui.Color{R: 56, G: 67, B: 82, A: 230},
		queryRadius:            8,
		queryText:              woxui.Color{R: 244, G: 247, B: 250, A: 255},
		cursor:                 woxui.Color{R: 57, G: 204, B: 183, A: 255},
		selectionBackground:    woxui.Color{R: 57, G: 204, B: 183, A: 120},
		selectionText:          woxui.Color{R: 244, G: 247, B: 250, A: 255},
		resultTitle:            woxui.Color{R: 244, G: 247, B: 250, A: 255},
		resultSubtitle:         woxui.Color{R: 166, G: 176, B: 190, A: 255},
		resultTail:             woxui.Color{R: 184, G: 184, B: 194, A: 255},
		resultContainerPadding: woxwidget.Insets{Top: 8},
		resultItemPadding:      woxwidget.Insets{Left: 8, Top: 3, Right: 8, Bottom: 3},
		resultItemRadius:       8,
		selectedBackground:     woxui.Color{R: 43, G: 181, B: 168, A: 210},
		selectedTitle:          woxui.Color{R: 244, G: 247, B: 250, A: 255},
		selectedSubtitle:       woxui.Color{R: 225, G: 251, B: 248, A: 255},
		selectedTail:           woxui.Color{R: 209, G: 209, B: 216, A: 255},
		actionBackground:       woxui.Color{R: 31, G: 36, B: 46, A: 250},
		actionHeader:           woxui.Color{R: 166, G: 176, B: 190, A: 255},
		actionPadding:          woxwidget.Insets{Left: 14, Top: 10, Right: 14, Bottom: 10},
		actionSelected:         woxui.Color{R: 43, G: 181, B: 168, A: 210},
		actionSelectedText:     woxui.Color{R: 244, G: 247, B: 250, A: 255},
		actionText:             woxui.Color{R: 244, G: 247, B: 250, A: 255},
		actionQueryBackground:  woxui.Color{R: 20, G: 24, B: 31, A: 210},
		actionQueryText:        woxui.Color{R: 244, G: 247, B: 250, A: 255},
		actionQueryRadius:      8,
		previewText:            woxui.Color{R: 244, G: 247, B: 250, A: 255},
		previewSplit:           woxui.Color{R: 85, G: 96, B: 112, A: 150},
		previewPropertyTitle:   woxui.Color{R: 166, G: 176, B: 190, A: 255},
		previewPropertyContent: woxui.Color{R: 224, G: 224, B: 230, A: 255},
		toolbarBackground:      woxui.Color{R: 20, G: 24, B: 31, A: 180},
		toolbarText:            woxui.Color{R: 166, G: 176, B: 190, A: 255},
		toolbarPadding:         woxwidget.Insets{Left: 10, Right: 10},
	}
}

// reloadTheme pulls the platform-resolved theme from core before the next frame.
func (a *App) reloadTheme() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	loaded, err := a.services.CurrentTheme(ctx, a.sessionID)
	if err != nil {
		return fmt.Errorf("load current theme: %w", err)
	}
	return a.runOnUI("apply current theme", func() {
		a.applyTheme(fromCoreTheme(loaded))
	})
}

func (a *App) applyTheme(theme themeData) {
	palette := paletteForTheme(theme)
	isDark := themeColorIsDark(palette.background)
	woxui.SetDefaultAppearance(isDark)
	a.palette = palette
	settingsView := a.settingsView
	onboardingView := a.onboardingView
	if a.window != nil {
		_ = a.window.SetAppearance(isDark)
		if err := a.window.SetWindowChrome(a.palette.AppWindowChrome, a.palette.AppBorderRadius); err != nil {
			util.GetLogger().Error(context.Background(), fmt.Sprintf("apply theme window chrome: %v", err))
		}
		_ = a.applyWindowBounds()
		_ = a.window.Invalidate()
	}
	if settingsView != nil {
		_ = settingsView.Window().SetAppearance(isDark)
	}
	if onboardingView != nil {
		onboardingDark := isDark
		if a.onboardingOpen {
			onboardingDark = true
		}
		_ = onboardingView.Window().SetAppearance(onboardingDark)
	}
	if chatWindow := a.chatNativeWindow(); chatWindow != nil {
		_ = chatWindow.SetAppearance(isDark)
		_ = chatWindow.Invalidate()
	}
	for _, controller := range a.noteWindows {
		if controller.managed != nil {
			_ = controller.managed.Window().SetAppearance(isDark)
			_ = controller.managed.Window().Invalidate()
		}
	}
	a.invalidateSettingsWindow()
	a.invalidateOnboardingWindow()
	overlay.NotifyThemeChanged(isDark)
}

func themeColorIsDark(color woxui.Color) bool {
	linear := func(value uint8) float64 {
		channel := float64(value) / 255
		if channel <= 0.03928 {
			return channel / 12.92
		}
		return math.Pow((channel+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(color.R)+0.7152*linear(color.G)+0.0722*linear(color.B) < 0.5
}

// paletteForTheme resolves a complete portable palette without mutating the active UI.
func paletteForTheme(theme themeData) uiPalette {
	fallback := defaultPalette()
	actionBorderWidth := float32(1)
	if theme.ActionContainerBorderWidth != nil {
		actionBorderWidth = max(float32(0), float32(*theme.ActionContainerBorderWidth))
	}
	actionQueryRadius := float32(theme.ActionQueryBoxBorderRadius)
	if actionQueryRadius < 0 {
		actionQueryRadius = fallback.actionQueryRadius
	}
	// Optional geometry keeps old themes unchanged while allowing explicit square corners and no divider.
	actionContainerRadius := actionQueryRadius
	if theme.ActionContainerBorderRadius != nil {
		actionContainerRadius = max(float32(0), float32(*theme.ActionContainerBorderRadius))
	}
	actionItemRadius := max(float32(0), float32(theme.ResultItemBorderRadius))
	if theme.ActionItemBorderRadius != nil {
		actionItemRadius = max(float32(0), float32(*theme.ActionItemBorderRadius))
	}
	toolbarBorderWidth := float32(1)
	if theme.ToolbarBorderWidth != nil {
		toolbarBorderWidth = max(float32(0), float32(*theme.ToolbarBorderWidth))
	}
	toolbarBorder := parseThemeColor(theme.ToolbarFontColor, fallback.toolbarText)
	toolbarBorder.A = min(toolbarBorder.A, uint8(26))
	return uiPalette{
		ActionContainerDividerColor:                optionalThemeColor(theme.ActionContainerDividerColor),
		PreviewBackgroundColor:                     optionalThemeColor(theme.PreviewBackgroundColor),
		PreviewBorderColor:                         optionalThemeColor(theme.PreviewBorderColor),
		PreviewBorderRadius:                        theme.PreviewBorderRadius,
		ResultItemActiveIndicatorWidth:             theme.ResultItemActiveIndicatorWidth,
		ResultItemActiveIndicatorInsetLeft:         theme.ResultItemActiveIndicatorInsetLeft,
		ResultItemActiveIndicatorInsetTop:          theme.ResultItemActiveIndicatorInsetTop,
		ResultItemActiveIndicatorInsetBottom:       theme.ResultItemActiveIndicatorInsetBottom,
		ResultItemActiveIndicatorBorderRadius:      theme.ResultItemActiveIndicatorBorderRadius,
		QueryBoxBorderBottomWidth:                  theme.QueryBoxBorderBottomWidth,
		ResultItemActiveIndicatorColor:             optionalThemeColor(theme.ResultItemActiveIndicatorColor),
		QueryBoxBorderBottomColor:                  optionalThemeColor(theme.QueryBoxBorderBottomColor),
		PreviewTagBorderRadius:                     theme.PreviewTagBorderRadius,
		PreviewTagFontColor:                        optionalThemeColor(theme.PreviewTagFontColor),
		PreviewTagBackgroundColor:                  optionalThemeColor(theme.PreviewTagBackgroundColor),
		PreviewTagBorderColor:                      optionalThemeColor(theme.PreviewTagBorderColor),
		GlanceFontColor:                            optionalThemeColor(theme.GlanceFontColor),
		GlanceIconColor:                            optionalThemeColor(theme.GlanceIconColor),
		GlanceBackgroundColor:                      optionalThemeColor(theme.GlanceBackgroundColor),
		RefinementButtonFontColor:                  optionalThemeColor(theme.RefinementButtonFontColor),
		RefinementButtonIconColor:                  optionalThemeColor(theme.RefinementButtonIconColor),
		RefinementButtonBackgroundColor:            optionalThemeColor(theme.RefinementButtonBackgroundColor),
		RefinementButtonBorderColor:                optionalThemeColor(theme.RefinementButtonBorderColor),
		RefinementButtonHoverBackgroundColor:       optionalThemeColor(theme.RefinementButtonHoverBackgroundColor),
		RefinementButtonActiveFontColor:            optionalThemeColor(theme.RefinementButtonActiveFontColor),
		RefinementButtonActiveIconColor:            optionalThemeColor(theme.RefinementButtonActiveIconColor),
		RefinementButtonActiveBackgroundColor:      optionalThemeColor(theme.RefinementButtonActiveBackgroundColor),
		RefinementButtonActiveBorderColor:          optionalThemeColor(theme.RefinementButtonActiveBorderColor),
		RefinementButtonActiveHoverBackgroundColor: optionalThemeColor(theme.RefinementButtonActiveHoverBackgroundColor),
		RefinementBackgroundColor:                  optionalThemeColor(theme.RefinementBackgroundColor),
		RefinementBorderColor:                      optionalThemeColor(theme.RefinementBorderColor),
		RefinementTitleColor:                       optionalThemeColor(theme.RefinementTitleColor),
		RefinementDividerColor:                     optionalThemeColor(theme.RefinementDividerColor),
		RefinementHotkeyColor:                      optionalThemeColor(theme.RefinementHotkeyColor),
		RefinementItemFontColor:                    optionalThemeColor(theme.RefinementItemFontColor),
		RefinementItemBackgroundColor:              optionalThemeColor(theme.RefinementItemBackgroundColor),
		RefinementItemHoverBackgroundColor:         optionalThemeColor(theme.RefinementItemHoverBackgroundColor),
		RefinementItemActiveFontColor:              optionalThemeColor(theme.RefinementItemActiveFontColor),
		RefinementItemActiveBackgroundColor:        optionalThemeColor(theme.RefinementItemActiveBackgroundColor),
		RefinementItemActiveHoverBackgroundColor:   optionalThemeColor(theme.RefinementItemActiveHoverBackgroundColor),
		ScrollbarThumbColor:                        optionalThemeColor(theme.ScrollbarThumbColor),
		ScrollbarThumbHoverColor:                   optionalThemeColor(theme.ScrollbarThumbHoverColor),
		ScrollbarThumbActiveColor:                  optionalThemeColor(theme.ScrollbarThumbActiveColor),
		ScrollbarWidth:                             theme.ScrollbarWidth,
		ScrollbarHoverWidth:                        theme.ScrollbarHoverWidth,
		ScrollbarBorderRadius:                      theme.ScrollbarBorderRadius,
		AppWindowChrome:                            theme.usesCustomWindowChrome(),
		AppBorderColor:                             optionalThemeColor(theme.AppBorderColor),
		AppBorderWidth:                             theme.AppBorderWidth,
		AppBorderRadius:                            theme.AppBorderRadius,
		GlanceHoverBackgroundColor:                 optionalThemeColor(theme.GlanceHoverBackgroundColor),

		ToolbarHotkeyFontColor:                optionalThemeColor(theme.ToolbarHotkeyFontColor),
		ActionItemHotkeyFontColor:             optionalThemeColor(theme.ActionItemHotkeyFontColor),
		ActionItemActiveHotkeyFontColor:       optionalThemeColor(theme.ActionItemActiveHotkeyFontColor),
		ToolbarHotkeyBackgroundColor:          optionalThemeColor(theme.ToolbarHotkeyBackgroundColor),
		ActionItemHotkeyBackgroundColor:       optionalThemeColor(theme.ActionItemHotkeyBackgroundColor),
		ActionItemActiveHotkeyBackgroundColor: optionalThemeColor(theme.ActionItemActiveHotkeyBackgroundColor),
		ToolbarHotkeyBorderColor:              optionalThemeColor(theme.ToolbarHotkeyBorderColor),
		ActionItemHotkeyBorderColor:           optionalThemeColor(theme.ActionItemHotkeyBorderColor),
		ActionItemActiveHotkeyBorderColor:     optionalThemeColor(theme.ActionItemActiveHotkeyBorderColor),
		ResultItemHoverBackgroundColor:        optionalThemeColor(theme.ResultItemHoverBackgroundColor),

		selectedBorderLeftWidth: max(float32(0), float32(theme.ResultItemActiveBorderLeftWidth)),
		actionBorderWidth:       actionBorderWidth,
		actionContainerRadius:   actionContainerRadius,
		actionItemRadius:        actionItemRadius,
		toolbarBorderWidth:      toolbarBorderWidth,
		selectedBorderLeftColor: parseThemeColor(theme.ResultItemActiveBorderLeftColor, parseThemeColor(theme.QueryBoxCursorColor, fallback.cursor)),
		toolbarBorder:           parseThemeColor(theme.ToolbarBorderColor, toolbarBorder),

		background:             opaqueWindowBackground(parseThemeColor(theme.AppBackgroundColor, fallback.background), theme.usesCustomWindowChrome()),
		appPadding:             themeInsets(theme.AppPaddingLeft, theme.AppPaddingTop, theme.AppPaddingRight, theme.AppPaddingBottom),
		queryBackground:        parseThemeColor(theme.QueryBoxBackgroundColor, fallback.queryBackground),
		queryRadius:            max(float32(0), float32(theme.QueryBoxBorderRadius)),
		queryText:              parseThemeColor(theme.QueryBoxFontColor, fallback.queryText),
		cursor:                 parseThemeColor(theme.QueryBoxCursorColor, fallback.cursor),
		selectionBackground:    parseThemeColor(theme.QueryBoxTextSelectionBackgroundColor, fallback.selectionBackground),
		selectionText:          parseThemeColor(theme.QueryBoxTextSelectionColor, fallback.selectionText),
		resultTitle:            parseThemeColor(theme.ResultItemTitleColor, fallback.resultTitle),
		resultSubtitle:         parseThemeColor(theme.ResultItemSubTitleColor, fallback.resultSubtitle),
		resultTail:             parseThemeColor(theme.ResultItemTailTextColor, fallback.resultTail),
		resultContainerPadding: themeInsets(theme.ResultContainerPaddingLeft, theme.ResultContainerPaddingTop, theme.ResultContainerPaddingRight, theme.ResultContainerPaddingBottom),
		resultItemPadding:      themeInsets(theme.ResultItemPaddingLeft, theme.ResultItemPaddingTop, theme.ResultItemPaddingRight, theme.ResultItemPaddingBottom),
		resultItemRadius:       max(float32(0), float32(theme.ResultItemBorderRadius)),
		selectedBackground:     parseThemeColor(theme.ResultItemActiveBackgroundColor, fallback.selectedBackground),
		selectedTitle:          parseThemeColor(theme.ResultItemActiveTitleColor, fallback.selectedTitle),
		selectedSubtitle:       parseThemeColor(theme.ResultItemActiveSubTitleColor, fallback.selectedSubtitle),
		selectedTail:           parseThemeColor(theme.ResultItemActiveTailTextColor, fallback.selectedTail),
		actionBackground:       parseThemeColor(theme.ActionContainerBackgroundColor, fallback.actionBackground),
		actionBorder:           parseThemeColor(theme.ActionContainerBorderColor, parseThemeColor(theme.PreviewSplitLineColor, fallback.previewSplit)),
		actionHeader:           parseThemeColor(theme.ActionContainerHeaderFontColor, fallback.actionHeader),
		actionPadding:          themeInsets(theme.ActionContainerPaddingLeft, theme.ActionContainerPaddingTop, theme.ActionContainerPaddingRight, theme.ActionContainerPaddingBottom),
		actionSelected:         parseThemeColor(theme.ActionItemActiveBackgroundColor, fallback.actionSelected),
		actionSelectedText:     parseThemeColor(theme.ActionItemActiveFontColor, fallback.actionSelectedText),
		actionText:             parseThemeColor(theme.ActionItemFontColor, fallback.actionText),
		actionQueryBackground:  parseThemeColor(theme.ActionQueryBoxBackgroundColor, fallback.actionQueryBackground),
		actionQueryText:        parseThemeColor(theme.ActionQueryBoxFontColor, fallback.actionQueryText),
		actionQueryRadius:      actionQueryRadius,
		previewText:            parseThemeColor(theme.PreviewFontColor, fallback.previewText),
		previewSplit:           parseThemeColor(theme.PreviewSplitLineColor, fallback.previewSplit),
		previewPropertyTitle:   parseThemeColor(theme.PreviewPropertyTitleColor, fallback.previewPropertyTitle),
		previewPropertyContent: parseThemeColor(theme.PreviewPropertyContentColor, fallback.previewPropertyContent),
		toolbarBackground:      parseThemeColor(theme.ToolbarBackgroundColor, fallback.toolbarBackground),
		toolbarText:            parseThemeColor(theme.ToolbarFontColor, fallback.toolbarText),
		toolbarPadding:         themeInsets(theme.ToolbarPaddingLeft, 0, theme.ToolbarPaddingRight, 0),
	}
}

func themeInsets(left, top, right, bottom int) woxwidget.Insets {
	return woxwidget.Insets{Left: float32(max(0, left)), Top: float32(max(0, top)), Right: float32(max(0, right)), Bottom: float32(max(0, bottom))}
}

func parseThemeColor(value string, fallback woxui.Color) woxui.Color {
	if color, ok := decodeThemeColor(value); ok {
		return color
	}
	return fallback
}

func decodeThemeColor(value string) (woxui.Color, bool) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "#") {
		raw := strings.TrimPrefix(value, "#")
		if len(raw) == 6 || len(raw) == 8 {
			decoded, err := hex.DecodeString(raw)
			if err == nil {
				color := woxui.Color{R: decoded[0], G: decoded[1], B: decoded[2], A: 255}
				if len(decoded) == 4 {
					color.A = decoded[3]
				}
				return color, true
			}
		}
	}
	lower := strings.ToLower(value)
	if (strings.HasPrefix(lower, "rgb(") || strings.HasPrefix(lower, "rgba(")) && strings.HasSuffix(value, ")") {
		start := strings.IndexByte(value, '(')
		parts := strings.Split(value[start+1:len(value)-1], ",")
		if len(parts) == 3 || len(parts) == 4 {
			channels := make([]float64, len(parts))
			for index, part := range parts {
				channel, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
				if err != nil {
					return woxui.Color{}, false
				}
				channels[index] = channel
			}
			alpha := float64(255)
			if len(channels) == 4 {
				alpha = channels[3]
				if alpha <= 1 {
					// from_css_color quantizes fractional CSS alpha before Flutter exposes the channel.
					alpha = math.Floor(alpha * 255)
				}
			}
			return woxui.Color{R: colorByte(channels[0]), G: colorByte(channels[1]), B: colorByte(channels[2]), A: colorByte(alpha)}, true
		}
	}
	return woxui.Color{}, false
}

type themeColorHSV struct {
	hue        float64
	saturation float64
	value      float64
	alpha      float64
}

// themeColorToHSV preserves Flutter's HSV controls while theme values remain CSS colors.
func themeColorToHSV(color woxui.Color) themeColorHSV {
	red := float64(color.R) / 255
	green := float64(color.G) / 255
	blue := float64(color.B) / 255
	high := max(red, green, blue)
	low := min(red, green, blue)
	delta := high - low
	hue := float64(0)
	switch {
	case delta == 0:
	case high == red:
		hue = 60 * math.Mod((green-blue)/delta, 6)
	case high == green:
		hue = 60 * ((blue-red)/delta + 2)
	default:
		hue = 60 * ((red-green)/delta + 4)
	}
	if hue < 0 {
		hue += 360
	}
	saturation := float64(0)
	if high > 0 {
		saturation = delta / high
	}
	return themeColorHSV{hue: hue, saturation: saturation, value: high, alpha: float64(color.A) / 255}
}

// themeColorFromHSV converts normalized picker state back to the renderer's byte color.
func themeColorFromHSV(color themeColorHSV) woxui.Color {
	hue := math.Mod(max(float64(0), color.hue), 360)
	saturation := min(float64(1), max(float64(0), color.saturation))
	value := min(float64(1), max(float64(0), color.value))
	chroma := value * saturation
	section := hue / 60
	offset := chroma * (1 - math.Abs(math.Mod(section, 2)-1))
	var red, green, blue float64
	switch int(section) {
	case 0:
		red, green = chroma, offset
	case 1:
		red, green = offset, chroma
	case 2:
		green, blue = chroma, offset
	case 3:
		green, blue = offset, chroma
	case 4:
		red, blue = offset, chroma
	default:
		red, blue = chroma, offset
	}
	match := value - chroma
	return woxui.Color{
		R: colorByte((red + match) * 255),
		G: colorByte((green + match) * 255),
		B: colorByte((blue + match) * 255),
		A: colorByte(min(float64(1), max(float64(0), color.alpha)) * 255),
	}
}

func encodeThemeColor(color woxui.Color) string {
	if float64(color.A)/255 >= 0.995 {
		return fmt.Sprintf("#%02X%02X%02X", color.R, color.G, color.B)
	}
	return fmt.Sprintf("#%02X%02X%02X%02X", color.R, color.G, color.B, color.A)
}

func colorByte(value float64) uint8 {
	return uint8(math.Round(max(float64(0), min(float64(255), value))))
}

// optionalThemeColor preserves absence independently from an explicitly transparent color.
func optionalThemeColor(value string) *woxui.Color {
	if value == "" {
		return nil
	}
	parsed := parseThemeColor(value, woxui.Color{})
	return &parsed
}
