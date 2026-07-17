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
)

type themeData struct {
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
	ActionContainerHeaderFontColor       string
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
	ToolbarPaddingLeft                   int
	ToolbarPaddingRight                  int
}

type uiPalette struct {
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

// componentTheme exposes launcher colors through the stable component package boundary.
func (palette uiPalette) componentTheme() woxcomponent.Theme {
	return woxcomponent.Theme{
		Background:             palette.background,
		QueryBackground:        palette.queryBackground,
		QueryText:              palette.queryText,
		Cursor:                 palette.cursor,
		SelectionBackground:    palette.selectionBackground,
		SelectionText:          palette.selectionText,
		ResultTitle:            palette.resultTitle,
		ResultSubtitle:         palette.resultSubtitle,
		ErrorText:              woxui.Color{R: 232, G: 95, B: 95, A: 255},
		SelectedBackground:     palette.selectedBackground,
		SelectedTitle:          palette.selectedTitle,
		SelectedSubtitle:       palette.selectedSubtitle,
		ActionBackground:       palette.actionBackground,
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

// appSurfaceRadius leaves compositor-backed platforms to clip the native window.
func appSurfaceRadius() float32 {
	if runtime.GOOS == "linux" {
		return 14
	}
	return 0
}

func defaultPalette() uiPalette {
	return uiPalette{
		background:             woxui.Color{R: 24, G: 29, B: 38, A: 242},
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
	var theme themeData
	if err := a.client.Post(ctx, "/theme", map[string]any{}, &theme); err != nil {
		return fmt.Errorf("load current theme: %w", err)
	}
	a.applyTheme(theme)
	return nil
}

func (a *App) applyTheme(theme themeData) {
	palette := paletteForTheme(theme)
	a.mu.Lock()
	a.palette = palette
	a.mu.Unlock()
	if a.window != nil {
		_ = a.applyWindowBounds()
		_ = a.window.Invalidate()
	}
	a.invalidateSettingsWindow()
}

// paletteForTheme resolves a complete portable palette without mutating the active UI.
func paletteForTheme(theme themeData) uiPalette {
	fallback := defaultPalette()
	actionQueryRadius := float32(theme.ActionQueryBoxBorderRadius)
	if actionQueryRadius < 0 {
		actionQueryRadius = fallback.actionQueryRadius
	}
	return uiPalette{
		background:             parseThemeColor(theme.AppBackgroundColor, fallback.background),
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
					alpha *= 255
				}
			}
			return woxui.Color{R: colorByte(channels[0]), G: colorByte(channels[1]), B: colorByte(channels[2]), A: colorByte(alpha)}, true
		}
	}
	return woxui.Color{}, false
}

func colorByte(value float64) uint8 {
	return uint8(math.Round(max(float64(0), min(float64(255), value))))
}
