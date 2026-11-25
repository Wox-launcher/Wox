package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// FormTableMCPJSONImportDialogProps contains the MCP JSON editor dialog.
type FormTableMCPJSONImportDialogProps struct {
	Width       float32
	Height      float32
	Title       string
	Hint        string
	Error       string
	CancelLabel string
	ImportLabel string
	Field       woxwidget.Widget
	FieldHeight float32
	Theme       woxcomponent.ControlTheme
	OnCancel    func()
	OnImport    func()
}

// FormTableMCPJSONImportDialog builds the modal dialog that imports MCP servers from JSON.
func FormTableMCPJSONImportDialog(props FormTableMCPJSONImportDialogProps) woxwidget.Widget {
	panelWidth := min(float32(720), max(float32(0), props.Width-56))
	innerWidth := max(float32(0), panelWidth-48)
	title := woxwidget.TextBlock{Value: props.Title, MinLineHeight: props.Theme.Scaled(28), MaxLines: 1,
		Style: woxui.TextStyle{Size: props.Theme.Scaled(18), Weight: woxui.FontWeightSemibold}, Color: props.Theme.Text}
	hintText := woxwidget.TextBlock{Value: props.Hint, MinLineHeight: props.Theme.Scaled(20), MaxLines: 2,
		Style: woxui.TextStyle{Size: props.Theme.Scaled(13)}, Color: props.Theme.TextSecondary}
	errorText := woxwidget.TextBlock{Value: props.Error, MinLineHeight: props.Theme.Scaled(20), MaxLines: 2,
		Style: woxui.TextStyle{Size: props.Theme.Scaled(13)}, Color: props.Theme.Error}
	children := []woxwidget.Widget{title, hintText, props.Field}
	if props.Error != "" {
		children = append(children, errorText)
	}
	actions := settingsDialogActions(innerWidth, props.Theme,
		settingsDialogAction{ID: "form-table-mcp-json-cancel", Label: props.CancelLabel, OnTap: props.OnCancel},
		settingsDialogAction{ID: "form-table-mcp-json-confirm", Label: props.ImportLabel, OnTap: props.OnImport},
	)

	body := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: children}
	return woxwidget.TextLayoutBuilder{Build: func(_ woxui.Size, measure woxwidget.TextMeasurer) woxwidget.Widget {
		bodyHeight := props.FieldHeight + float32(len(children)-1)*body.Gap
		bodyHeight += woxwidget.MeasureStateless(measure, title, innerWidth).Height + woxwidget.MeasureStateless(measure, hintText, innerWidth).Height
		if props.Error != "" {
			bodyHeight += woxwidget.MeasureStateless(measure, errorText, innerWidth).Height
		}
		return settingsScrollableDialog(woxcomponent.DialogProps{
			ID: "form-table-mcp-json-dialog", Label: props.Title, Width: panelWidth,
			OverlayWidth: props.Width, OverlayHeight: props.Height, BackdropID: "form-table-mcp-json-backdrop", BackdropAlpha: 210,
			Radius: 20, Padding: woxwidget.UniformInsets(24), BorderColor: formTableAlpha(props.Theme.TextSecondary, 104), BorderWidth: 1,
			InitialFocus: "form-table-row-field-0", OnEscape: props.OnCancel, Theme: props.Theme,
		}, body, actions, bodyHeight, 360, 640)
	}}
}
