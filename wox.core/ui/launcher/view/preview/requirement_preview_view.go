package preview

import (
	"fmt"
	"strings"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// RequirementPreviewProps contains the prepared form rows and actions for a requirement preview.
type RequirementPreviewProps struct {
	Width         float32
	Height        float32
	Theme         woxcomponent.Theme
	FatalError    string
	Title         string
	Message       string
	PluginName    string
	Error         string
	SaveLabel     string
	Saving        bool
	Rows          []woxwidget.Widget
	RowsHeight    float32
	Scroll        float32
	OnScroll      func(float32)
	OnSetViewport func(float32)
	OnSubmit      func()
}

// RequirementPreviewView builds the compact plugin configuration surface.
func RequirementPreviewView(props RequirementPreviewProps) woxwidget.Widget {
	if props.FatalError != "" {
		return previewError(props.FatalError, props.Width, props.Height, props.Theme)
	}
	innerWidth := max(float32(0), props.Width-36)
	titleHeight := float32(28)
	messageHeight := float32(42)
	saveLabel := props.SaveLabel
	variant := woxcomponent.ButtonPrimary
	if props.Saving {
		saveLabel += "…"
		variant = woxcomponent.ButtonSelected
	}
	beforeBody := []woxwidget.Widget{
		woxwidget.Container{Width: innerWidth, Height: titleHeight, Child: woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 18, Weight: woxui.FontWeightSemibold}, Color: props.Theme.PreviewText}},
		woxwidget.Container{Width: innerWidth, Height: messageHeight, Child: woxwidget.TextBlock{Value: props.Message, Width: innerWidth, Height: messageHeight, Style: woxui.TextStyle{Size: 12}, LineHeight: 17, Color: props.Theme.ResultSubtitle}},
	}
	return editorPreviewShell(editorPreviewShellProps{
		Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: 18, Top: 14, Right: 18, Bottom: 14}, Theme: props.Theme,
		BeforeBody: beforeBody, BeforeBodyHeight: titleHeight + messageHeight, MinimumBodyHeight: 48,
		Rows: props.Rows, RowsHeight: props.RowsHeight, EmptyMessage: fmt.Sprintf("No editable settings were provided for %s.", props.PluginName),
		ScrollID: "requirement-form-scroll", Scroll: props.Scroll, OnScroll: props.OnScroll, OnSetViewport: props.OnSetViewport,
		Error: props.Error, ShowError: strings.TrimSpace(props.Error) != "",
		SaveButton: woxcomponent.ButtonProps{ID: "requirement-form-save", Label: saveLabel, Width: 104, Variant: variant, OnTap: props.OnSubmit, Theme: props.Theme},
	})
}
