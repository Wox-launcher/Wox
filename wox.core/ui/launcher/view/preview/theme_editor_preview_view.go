package preview

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// ThemeEditorPreviewProps contains prepared editor rows and draft colors.
type ThemeEditorPreviewProps struct {
	Width          float32
	Height         float32
	Theme          woxcomponent.Theme
	FatalError     string
	DraftTheme     woxcomponent.Theme
	Error          string
	SaveLabel      string
	Dirty          bool
	Saving         bool
	Rows           []woxwidget.Widget
	KeepVisibleKey woxwidget.Key
	OnSubmit       func()
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
		Rows: props.Rows, ScrollID: "theme-editor-scroll", KeepVisibleKey: props.KeepVisibleKey,
		Error: props.Error, ShowError: props.Error != "",
		SaveButton: woxcomponent.ButtonProps{ID: "theme-editor-save", Label: saveLabel, Variant: variant, OnTap: props.OnSubmit, Theme: props.Theme},
	})
}

// ThemeDraftSample builds the reusable live theme preview.
func ThemeDraftSample(theme woxcomponent.Theme, width, height float32) woxwidget.Widget {
	return woxcomponent.WoxLauncherDemo(woxcomponent.LauncherDemoProps{
		Width: width, Height: height, Background: theme.Background, Theme: theme, Opacity: 1, Query: "WOX", ShowQuery: true, ShowToolbar: true,
		Results: []woxcomponent.LauncherDemoResult{
			{Title: "Wox Go UI", Subtitle: "Portable GPU-rendered theme preview", Glyph: "W", GlyphColor: theme.ResultTitle},
			{Title: "Selected result", Subtitle: "Colors update as you type", Glyph: "✓", GlyphColor: theme.SelectedTitle, Selected: true},
		},
		PrimaryAction: "Open", ActionMore: "Actions",
	})
}
