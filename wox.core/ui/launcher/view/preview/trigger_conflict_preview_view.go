package preview

import (
	"fmt"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TriggerConflictPreviewProps contains the prepared rows and actions for a trigger conflict.
type TriggerConflictPreviewProps struct {
	Width       float32
	Height      float32
	Theme       woxcomponent.Theme
	FatalError  string
	Keyword     string
	Title       string
	Message     string
	Error       string
	SaveLabel   string
	Dirty       bool
	Saving      bool
	Rows        []woxwidget.Widget
	RowsHeight  float32
	KeepVisible *woxwidget.ScrollRange
	OnSubmit    func()
}

// TriggerConflictPreviewView builds the editable conflict resolver.
func TriggerConflictPreviewView(props TriggerConflictPreviewProps) woxwidget.Widget {
	if props.FatalError != "" {
		return previewError(props.FatalError, props.Width, props.Height, props.Theme)
	}
	innerWidth := max(float32(0), props.Width-36)
	titleHeight := float32(30)
	messageHeight := float32(46)
	title := props.Title
	if title == "" {
		title = fmt.Sprintf("Resolve trigger keyword %q", props.Keyword)
	}
	message := props.Message
	if message == "" {
		message = "Edit one or more comma-separated keyword lists so each concrete trigger has a single owner."
	}
	saveLabel := props.SaveLabel
	if props.Saving {
		saveLabel += "…"
	}
	variant := woxcomponent.ButtonSelected
	if props.Dirty && !props.Saving {
		variant = woxcomponent.ButtonPrimary
	}
	beforeBody := []woxwidget.Widget{
		woxwidget.Container{Width: innerWidth, Height: titleHeight, Child: woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: 17, Weight: woxui.FontWeightSemibold}, Color: props.Theme.PreviewText}},
		woxwidget.Container{Width: innerWidth, Height: messageHeight, Child: woxwidget.TextBlock{Value: message, Width: innerWidth, Height: messageHeight, Style: woxui.TextStyle{Size: 12}, LineHeight: 17, Color: props.Theme.ResultSubtitle}},
	}
	return editorPreviewShell(editorPreviewShellProps{
		Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: 18, Top: 14, Right: 18, Bottom: 14}, Theme: props.Theme,
		BeforeBody: beforeBody, BeforeBodyHeight: titleHeight + messageHeight, MinimumBodyHeight: 56,
		Rows: props.Rows, RowsHeight: props.RowsHeight, ScrollID: "trigger-conflict-scroll", KeepVisible: props.KeepVisible,
		Error: props.Error, ShowError: props.Error != "",
		SaveButton: woxcomponent.ButtonProps{ID: "trigger-conflict-save", Label: saveLabel, Width: 112, Variant: variant, OnTap: props.OnSubmit, Theme: props.Theme},
	})
}
