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
	Width          float32
	Height         float32
	Theme          woxcomponent.Theme
	FatalError     string
	Title          string
	Message        string
	PluginName     string
	Error          string
	SaveLabel      string
	Saving         bool
	Rows           []woxwidget.Widget
	KeepVisibleKey woxwidget.Key
	OnSubmit       func()
	OnOpenLink     func(string)
	Window         *woxui.Window
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
	messageTheme := props.Theme
	messageTheme.PreviewText = props.Theme.ResultSubtitle
	beforeBody := []woxwidget.Widget{
		woxwidget.Container{Width: innerWidth, Height: titleHeight, Child: woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 18, Weight: woxui.FontWeightSemibold}, Color: props.Theme.PreviewText}},
		woxwidget.Container{Width: innerWidth, Height: messageHeight, Child: requirementPreviewMessage(props.Message, innerWidth, messageTheme, props.Window, props.OnOpenLink)},
	}
	return editorPreviewShell(editorPreviewShellProps{
		Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: 18, Top: 14, Right: 18, Bottom: 14}, Theme: props.Theme,
		BeforeBody: beforeBody, BeforeBodyHeight: titleHeight + messageHeight, MinimumBodyHeight: 48,
		Rows: props.Rows, EmptyMessage: fmt.Sprintf("No editable settings were provided for %s.", props.PluginName),
		ScrollID: "requirement-form-scroll", KeepVisibleKey: props.KeepVisibleKey,
		Error: props.Error, ShowError: strings.TrimSpace(props.Error) != "",
		SaveButton: woxcomponent.ButtonProps{ID: "requirement-form-save", Label: saveLabel, Variant: variant, OnTap: props.OnSubmit, Theme: props.Theme.Controls},
	})
}

// requirementPreviewMessage renders setup guidance as Markdown so plugin tips can use links and emphasis.
func requirementPreviewMessage(message string, width float32, theme woxcomponent.Theme, window *woxui.Window, onOpenLink func(string)) woxwidget.Widget {
	if strings.TrimSpace(message) == "" {
		return woxwidget.Painter{}
	}
	return woxcomponent.WoxMarkdown(woxcomponent.MarkdownProps{
		ID: "requirement-form-message", Document: woxcomponent.ParseMarkdown(message), Width: width,
		FontSize: 12, BlockGap: 4, ExcludeLinkFocus: true, Theme: theme.Controls, Window: window, OnOpenLink: onOpenLink,
	})
}
