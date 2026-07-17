package preview

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// ThemeEditorPreviewProps contains prepared editor rows and draft colors.
type ThemeEditorPreviewProps struct {
	Width       float32
	Height      float32
	Theme       woxcomponent.Theme
	FatalError  string
	DraftTheme  woxcomponent.Theme
	Error       string
	SaveLabel   string
	Dirty       bool
	Saving      bool
	Rows        []woxwidget.Widget
	RowsHeight  float32
	KeepVisible *woxwidget.ScrollRange
	OnSubmit    func()
}

// ThemeEditorPreviewView builds the live sample and color editor surface.
func ThemeEditorPreviewView(props ThemeEditorPreviewProps) woxwidget.Widget {
	if props.FatalError != "" {
		return previewError(props.FatalError, props.Width, props.Height, props.Theme)
	}
	innerWidth := max(float32(0), props.Width-32)
	innerHeight := max(float32(0), props.Height-24)
	headerHeight := float32(34)
	sampleHeight := min(float32(150), max(float32(96), innerHeight*0.3))
	saveLabel := props.SaveLabel
	if props.Saving {
		saveLabel += "…"
	}
	variant := woxcomponent.ButtonSelected
	if props.Dirty && !props.Saving {
		variant = woxcomponent.ButtonPrimary
	}
	beforeBody := []woxwidget.Widget{
		woxwidget.Container{Width: innerWidth, Height: headerHeight, Child: woxwidget.Text{Value: "Theme editor · edit CSS colors directly", Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: props.Theme.PreviewText}},
		ThemeDraftSample(props.DraftTheme, innerWidth, sampleHeight),
	}
	return editorPreviewShell(editorPreviewShellProps{
		Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: 16, Top: 12, Right: 16, Bottom: 12}, Theme: props.Theme,
		BeforeBody: beforeBody, BeforeBodyHeight: headerHeight + sampleHeight, MinimumBodyHeight: 72,
		Rows: props.Rows, RowsHeight: props.RowsHeight, ScrollID: "theme-editor-scroll", KeepVisible: props.KeepVisible,
		Error: props.Error, ShowError: props.Error != "",
		SaveButton: woxcomponent.ButtonProps{ID: "theme-editor-save", Label: saveLabel, Width: 116, Variant: variant, OnTap: props.OnSubmit, Theme: props.Theme},
	})
}

// ThemeDraftSample builds the reusable live theme preview.
func ThemeDraftSample(theme woxcomponent.Theme, width, height float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-20)
	queryHeight := float32(32)
	toolbarHeight := float32(24)
	rowHeight := max(float32(24), (height-queryHeight-toolbarHeight-20)/2)
	query := woxwidget.Container{Width: innerWidth, Height: queryHeight, Radius: 7, Color: theme.QueryBackground, Padding: woxwidget.Insets{Left: 10, Top: 8}, Child: woxwidget.Text{
		Value: "WOX", Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: theme.QueryText,
	}}
	row := func(selected bool, title, subtitle string) woxwidget.Widget {
		background := woxui.Color{}
		titleColor := theme.ResultTitle
		subtitleColor := theme.ResultSubtitle
		if selected {
			background = theme.SelectedBackground
			titleColor = theme.SelectedTitle
			subtitleColor = theme.SelectedSubtitle
		}
		return woxwidget.Container{Width: innerWidth, Height: rowHeight, Radius: 7, Color: background, Padding: woxwidget.Insets{Left: 10, Top: 6}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: titleColor},
			woxwidget.Text{Value: subtitle, Style: woxui.TextStyle{Size: 9}, Color: subtitleColor},
		}}}
	}
	toolbar := woxwidget.Container{Width: innerWidth, Height: toolbarHeight, Color: theme.ToolbarBackground, Padding: woxwidget.Insets{Left: 10, Top: 6}, Child: woxwidget.Text{
		Value: "Open   ·   Actions", Style: woxui.TextStyle{Size: 9}, Color: theme.ToolbarText,
	}}
	return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: theme.Background, Padding: woxwidget.UniformInsets(10), Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
			query,
			row(false, "Wox Go UI", "Portable GPU-rendered theme preview"),
			row(true, "Selected result", "Colors update as you type"),
			toolbar,
		},
	}}
}
