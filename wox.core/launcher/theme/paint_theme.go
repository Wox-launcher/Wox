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

type ResultListPaintTheme struct {
	TitleColor            string
	SubtitleColor         string
	ActiveBackgroundColor string
	ActiveTitleColor      string
	ActiveSubtitleColor   string
	BorderRadius          int
}

type PreviewPaintTheme struct {
	FontColor            string
	SplitLineColor       string
	PropertyTitleColor   string
	PropertyContentColor string
}

type LayoutPaintTheme struct {
	AppPaddingTop                int
	AppPaddingBottom             int
	ResultContainerPaddingTop    int
	ResultContainerPaddingBottom int
	ResultItemPaddingTop         int
	ResultItemPaddingBottom      int
}

type PaintTheme struct {
	ThemeID  string
	Window   WindowPaintTheme
	QueryBox QueryBoxPaintTheme
	Results  ResultListPaintTheme
	Preview  PreviewPaintTheme
	Layout   LayoutPaintTheme
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
		Results: ResultListPaintTheme{
			TitleColor:            "#E2E8F0",
			SubtitleColor:         "#9CA3AF",
			ActiveBackgroundColor: "rgba(0, 168, 142, 0.7)",
			ActiveTitleColor:      "#FFFFFF",
			ActiveSubtitleColor:   "#E2E8F0",
			BorderRadius:          10,
		},
		Preview: PreviewPaintTheme{
			FontColor:            "#E2E8F0",
			SplitLineColor:       "#4A5568",
			PropertyTitleColor:   "#9CA3AF",
			PropertyContentColor: "#E2E8F0",
		},
		Layout: LayoutPaintTheme{
			AppPaddingTop:                10,
			AppPaddingBottom:             10,
			ResultContainerPaddingTop:    10,
			ResultContainerPaddingBottom: 0,
			ResultItemPaddingTop:         4,
			ResultItemPaddingBottom:      4,
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
	if source.ResultItemTitleColor != "" {
		mapped.Results.TitleColor = source.ResultItemTitleColor
	}
	if source.ResultItemSubTitleColor != "" {
		mapped.Results.SubtitleColor = source.ResultItemSubTitleColor
	}
	if source.ResultItemActiveBackgroundColor != "" {
		mapped.Results.ActiveBackgroundColor = source.ResultItemActiveBackgroundColor
	}
	if source.ResultItemActiveTitleColor != "" {
		mapped.Results.ActiveTitleColor = source.ResultItemActiveTitleColor
	}
	if source.ResultItemActiveSubTitleColor != "" {
		mapped.Results.ActiveSubtitleColor = source.ResultItemActiveSubTitleColor
	}
	if source.ResultItemBorderRadius > 0 {
		mapped.Results.BorderRadius = source.ResultItemBorderRadius
	}
	if source.PreviewFontColor != "" {
		mapped.Preview.FontColor = source.PreviewFontColor
	}
	if source.PreviewSplitLineColor != "" {
		mapped.Preview.SplitLineColor = source.PreviewSplitLineColor
	}
	if source.PreviewPropertyTitleColor != "" {
		mapped.Preview.PropertyTitleColor = source.PreviewPropertyTitleColor
	}
	if source.PreviewPropertyContentColor != "" {
		mapped.Preview.PropertyContentColor = source.PreviewPropertyContentColor
	}
	if source.AppPaddingTop > 0 {
		mapped.Layout.AppPaddingTop = source.AppPaddingTop
	}
	if source.AppPaddingBottom > 0 {
		mapped.Layout.AppPaddingBottom = source.AppPaddingBottom
	}
	if source.ResultContainerPaddingTop > 0 {
		mapped.Layout.ResultContainerPaddingTop = source.ResultContainerPaddingTop
	}
	if source.ResultContainerPaddingBottom > 0 {
		mapped.Layout.ResultContainerPaddingBottom = source.ResultContainerPaddingBottom
	}
	if source.ResultItemPaddingTop > 0 {
		mapped.Layout.ResultItemPaddingTop = source.ResultItemPaddingTop
	}
	if source.ResultItemPaddingBottom > 0 {
		mapped.Layout.ResultItemPaddingBottom = source.ResultItemPaddingBottom
	}

	return mapped
}
