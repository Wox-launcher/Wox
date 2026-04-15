package theme

import "wox/common"

type WindowPaintTheme struct {
	BackgroundColor string
}

type QueryBoxPaintTheme struct {
	BackgroundColor          string
	ForegroundColor          string
	CursorColor              string
	SelectionBackgroundColor string
	SelectionColor           string
	BorderRadius             int
}

type PaintTheme struct {
	ThemeID  string
	Window   WindowPaintTheme
	QueryBox QueryBoxPaintTheme
}

func DefaultPaintTheme() PaintTheme {
	return PaintTheme{
		ThemeID: "launcher-default",
		Window: WindowPaintTheme{
			BackgroundColor: "rgba(35, 41, 51, 0.75)",
		},
		QueryBox: QueryBoxPaintTheme{
			BackgroundColor:          "rgba(49, 56, 68, 0.3)",
			ForegroundColor:          "#E2E8F0",
			CursorColor:              "#00A88E",
			SelectionBackgroundColor: "rgba(0, 168, 142, 0.8)",
			SelectionColor:           "#ffffff",
			BorderRadius:             8,
		},
	}
}

func MapCommonTheme(source common.Theme) PaintTheme {
	mapped := DefaultPaintTheme()

	if source.ThemeId != "" {
		mapped.ThemeID = source.ThemeId
	}
	if source.AppBackgroundColor != "" {
		mapped.Window.BackgroundColor = source.AppBackgroundColor
	}
	if source.QueryBoxBackgroundColor != "" {
		mapped.QueryBox.BackgroundColor = source.QueryBoxBackgroundColor
	}
	if source.QueryBoxFontColor != "" {
		mapped.QueryBox.ForegroundColor = source.QueryBoxFontColor
	}
	if source.QueryBoxCursorColor != "" {
		mapped.QueryBox.CursorColor = source.QueryBoxCursorColor
	}
	if source.QueryBoxTextSelectionBackgroundColor != "" {
		mapped.QueryBox.SelectionBackgroundColor = source.QueryBoxTextSelectionBackgroundColor
	}
	if source.QueryBoxTextSelectionColor != "" {
		mapped.QueryBox.SelectionColor = source.QueryBoxTextSelectionColor
	}
	if source.QueryBoxBorderRadius > 0 {
		mapped.QueryBox.BorderRadius = source.QueryBoxBorderRadius
	}

	return mapped
}
