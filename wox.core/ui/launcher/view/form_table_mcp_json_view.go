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
	Theme       woxcomponent.Theme
	OnCancel    func()
	OnImport    func()
}

// FormTableMCPJSONImportDialog builds the modal dialog that imports MCP servers from JSON.
func FormTableMCPJSONImportDialog(props FormTableMCPJSONImportDialogProps) woxwidget.Widget {
	panelWidth := min(float32(720), max(float32(0), props.Width-56))
	innerWidth := max(float32(0), panelWidth-48)
	statusHeight := float32(0)
	if props.Error != "" {
		statusHeight = 28
	}
	hintHeight := float32(20)
	actionsHeight := SettingsDialogActionsHeight
	childCount := 4
	if statusHeight > 0 {
		childCount++
	}
	contentHeight := 28 + hintHeight + props.FieldHeight + statusHeight + actionsHeight + float32(childCount-1)*12
	panelHeight := max(float32(360), min(float32(640), contentHeight+48))
	panelHeight = max(float32(0), min(panelHeight, props.Height-56))

	title := woxwidget.Container{Width: innerWidth, Height: 28, Child: woxwidget.Text{
		Value: props.Title, Style: woxui.TextStyle{Size: 18, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText,
	}}
	hintText := woxwidget.TextBlock{Value: props.Hint, Width: innerWidth, Height: hintHeight, MaxLines: 1, LineHeight: 20,
		Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultSubtitle}

	children := []woxwidget.Widget{title, hintText, props.Field}
	if statusHeight > 0 {
		children = append(children, woxwidget.TextBlock{Value: props.Error, Width: innerWidth, Height: 22, MaxLines: 2,
			Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ErrorText})
	}
	children = append(children, settingsDialogActions(innerWidth, props.Theme,
		settingsDialogAction{ID: "form-table-mcp-json-cancel", Label: props.CancelLabel, OnTap: props.OnCancel},
		settingsDialogAction{ID: "form-table-mcp-json-confirm", Label: props.ImportLabel, OnTap: props.OnImport},
	))

	body := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: children}
	border := formTableAlpha(props.Theme.ResultSubtitle, 104)
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "form-table-mcp-json-dialog", Label: props.Title, Width: panelWidth, Height: panelHeight,
		OverlayWidth: props.Width, OverlayHeight: props.Height, BackdropID: "form-table-mcp-json-backdrop", BackdropAlpha: 210,
		Radius: 20, Padding: woxwidget.UniformInsets(24), BorderColor: border, BorderWidth: 1,
		InitialFocus: woxwidget.Key("form-table-row-field-0"), OnEscape: props.OnCancel, Theme: props.Theme,
		Child: body,
	})
}
