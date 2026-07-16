package launcher

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	woxui "github.com/Wox-launcher/wox.ui.go"
)

type themeData struct {
	AppBackgroundColor                   string
	ResultItemTitleColor                 string
	ResultItemSubTitleColor              string
	ResultItemActiveBackgroundColor      string
	ResultItemActiveTitleColor           string
	ResultItemActiveSubTitleColor        string
	QueryBoxFontColor                    string
	QueryBoxBackgroundColor              string
	QueryBoxCursorColor                  string
	QueryBoxTextSelectionBackgroundColor string
	QueryBoxTextSelectionColor           string
	ActionContainerBackgroundColor       string
	ActionContainerHeaderFontColor       string
	ActionItemActiveBackgroundColor      string
	ActionItemActiveFontColor            string
	ActionItemFontColor                  string
	PreviewFontColor                     string
	PreviewSplitLineColor                string
	ToolbarFontColor                     string
	ToolbarBackgroundColor               string
}

type uiPalette struct {
	background          woxui.Color
	queryBackground     woxui.Color
	queryText           woxui.Color
	cursor              woxui.Color
	selectionBackground woxui.Color
	selectionText       woxui.Color
	resultTitle         woxui.Color
	resultSubtitle      woxui.Color
	selectedBackground  woxui.Color
	selectedTitle       woxui.Color
	selectedSubtitle    woxui.Color
	actionBackground    woxui.Color
	actionHeader        woxui.Color
	actionSelected      woxui.Color
	actionSelectedText  woxui.Color
	actionText          woxui.Color
	previewText         woxui.Color
	previewSplit        woxui.Color
	toolbarBackground   woxui.Color
	toolbarText         woxui.Color
}

func defaultPalette() uiPalette {
	return uiPalette{
		background:          woxui.Color{R: 24, G: 29, B: 38, A: 242},
		queryBackground:     woxui.Color{R: 56, G: 67, B: 82, A: 230},
		queryText:           woxui.Color{R: 244, G: 247, B: 250, A: 255},
		cursor:              woxui.Color{R: 57, G: 204, B: 183, A: 255},
		selectionBackground: woxui.Color{R: 57, G: 204, B: 183, A: 120},
		selectionText:       woxui.Color{R: 244, G: 247, B: 250, A: 255},
		resultTitle:         woxui.Color{R: 244, G: 247, B: 250, A: 255},
		resultSubtitle:      woxui.Color{R: 166, G: 176, B: 190, A: 255},
		selectedBackground:  woxui.Color{R: 43, G: 181, B: 168, A: 210},
		selectedTitle:       woxui.Color{R: 244, G: 247, B: 250, A: 255},
		selectedSubtitle:    woxui.Color{R: 225, G: 251, B: 248, A: 255},
		actionBackground:    woxui.Color{R: 31, G: 36, B: 46, A: 250},
		actionHeader:        woxui.Color{R: 166, G: 176, B: 190, A: 255},
		actionSelected:      woxui.Color{R: 43, G: 181, B: 168, A: 210},
		actionSelectedText:  woxui.Color{R: 244, G: 247, B: 250, A: 255},
		actionText:          woxui.Color{R: 244, G: 247, B: 250, A: 255},
		previewText:         woxui.Color{R: 244, G: 247, B: 250, A: 255},
		previewSplit:        woxui.Color{R: 85, G: 96, B: 112, A: 150},
		toolbarBackground:   woxui.Color{R: 20, G: 24, B: 31, A: 180},
		toolbarText:         woxui.Color{R: 166, G: 176, B: 190, A: 255},
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

func (a *App) changeTheme(raw json.RawMessage) error {
	var theme themeData
	if err := json.Unmarshal(raw, &theme); err != nil {
		return fmt.Errorf("decode changed theme: %w", err)
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
		_ = a.window.Invalidate()
	}
}

// paletteForTheme resolves a complete portable palette without mutating the active UI.
func paletteForTheme(theme themeData) uiPalette {
	fallback := defaultPalette()
	return uiPalette{
		background:          parseThemeColor(theme.AppBackgroundColor, fallback.background),
		queryBackground:     parseThemeColor(theme.QueryBoxBackgroundColor, fallback.queryBackground),
		queryText:           parseThemeColor(theme.QueryBoxFontColor, fallback.queryText),
		cursor:              parseThemeColor(theme.QueryBoxCursorColor, fallback.cursor),
		selectionBackground: parseThemeColor(theme.QueryBoxTextSelectionBackgroundColor, fallback.selectionBackground),
		selectionText:       parseThemeColor(theme.QueryBoxTextSelectionColor, fallback.selectionText),
		resultTitle:         parseThemeColor(theme.ResultItemTitleColor, fallback.resultTitle),
		resultSubtitle:      parseThemeColor(theme.ResultItemSubTitleColor, fallback.resultSubtitle),
		selectedBackground:  parseThemeColor(theme.ResultItemActiveBackgroundColor, fallback.selectedBackground),
		selectedTitle:       parseThemeColor(theme.ResultItemActiveTitleColor, fallback.selectedTitle),
		selectedSubtitle:    parseThemeColor(theme.ResultItemActiveSubTitleColor, fallback.selectedSubtitle),
		actionBackground:    parseThemeColor(theme.ActionContainerBackgroundColor, fallback.actionBackground),
		actionHeader:        parseThemeColor(theme.ActionContainerHeaderFontColor, fallback.actionHeader),
		actionSelected:      parseThemeColor(theme.ActionItemActiveBackgroundColor, fallback.actionSelected),
		actionSelectedText:  parseThemeColor(theme.ActionItemActiveFontColor, fallback.actionSelectedText),
		actionText:          parseThemeColor(theme.ActionItemFontColor, fallback.actionText),
		previewText:         parseThemeColor(theme.PreviewFontColor, fallback.previewText),
		previewSplit:        parseThemeColor(theme.PreviewSplitLineColor, fallback.previewSplit),
		toolbarBackground:   parseThemeColor(theme.ToolbarBackgroundColor, fallback.toolbarBackground),
		toolbarText:         parseThemeColor(theme.ToolbarFontColor, fallback.toolbarText),
	}
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
