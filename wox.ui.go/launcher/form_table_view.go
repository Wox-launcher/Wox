package launcher

import (
	"encoding/json"
	"fmt"
	"strings"

	woxui "github.com/Wox-launcher/wox.ui.go"
	woxwidget "github.com/Wox-launcher/wox.ui.go/widget"
)

const formTableListRowHeight = float32(48)

func formTableTitle(definition formDefinition) string {
	if definition.Value.Title != "" {
		return definition.Value.Title
	}
	if definition.Value.Label != "" {
		return definition.Value.Label
	}
	return definition.Value.Key
}

func compactFormTableText(value string, maxRunes int) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:max(0, maxRunes-1)]) + "…"
}

func (a *App) formTableDisplayValue(column formTableColumn, row map[string]any) string {
	value := formTableColumnValue(column, row)
	if column.Type == "checkbox" {
		if value == "true" {
			return "On"
		}
		return "Off"
	}
	if column.Type == "select" {
		for _, option := range column.SelectOptions {
			if option.Value == value {
				return a.translate(option.Label)
			}
		}
	}
	if column.Type == "selectAIModel" && value != "" {
		var model aiModel
		if json.Unmarshal([]byte(value), &model) == nil {
			return aiModelLabel(model)
		}
	}
	if column.Type == "app" {
		var app ignoredHotkeyApp
		if json.Unmarshal([]byte(value), &app) == nil {
			if strings.TrimSpace(app.Name) != "" {
				return app.Name
			}
			return app.Identity
		}
	}
	return value
}

func (a *App) formTableRowSummary(definition formDefinition, row map[string]any) string {
	parts := make([]string, 0, 3)
	for _, column := range definition.Value.Columns {
		if column.HideInTable {
			continue
		}
		label := a.translate(column.Label)
		value := compactFormTableText(a.formTableDisplayValue(column, row), 28)
		if value == "" {
			continue
		}
		if label == "" {
			parts = append(parts, value)
		} else {
			parts = append(parts, label+": "+value)
		}
		if len(parts) == 3 {
			break
		}
	}
	if len(parts) == 0 {
		return "Empty row"
	}
	return strings.Join(parts, "   ·   ")
}

func (a *App) buildFormTableField(fields formFieldsSnapshot, callbacks formFieldCallbacks, palette uiPalette, index int, definition formDefinition, width, height float32) woxwidget.Widget {
	focused := fields.active && fields.focused == index
	background := palette.queryBackground
	if focused {
		background = palette.selectedBackground
	}
	rows, err := decodeFormTableRows(fields.values[definition.Value.Key])
	countLabel := fmt.Sprintf("%d rows · Open editor ›", len(rows))
	if err != nil {
		countLabel = "Invalid table data · Open read-only ›"
		rows = nil
	}
	preview := "No rows yet"
	if len(rows) > 0 {
		preview = a.formTableRowSummary(definition, rows[0])
		if len(rows) > 1 {
			preview += "\n" + a.formTableRowSummary(definition, rows[1])
		}
	}
	fieldWidth := width - 142
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		woxwidget.Container{Width: 132, Height: height - 14, Padding: woxwidget.Insets{Top: 11}, Child: woxwidget.Text{
			Value: a.translate(formTableTitle(definition)), Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: palette.actionText,
		}},
		woxwidget.Gesture{
			ID: fmt.Sprintf("%s-field-%d", callbacks.idPrefix, index),
			OnTap: func() {
				callbacks.focus(index)
				if callbacks.openTable != nil {
					callbacks.openTable(index)
				}
			},
			Child: woxwidget.Container{Width: fieldWidth, Height: height - 14, Radius: 8, Color: background, Padding: woxwidget.Insets{Left: 12, Top: 10, Right: 10, Bottom: 8}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: []woxwidget.Widget{
				woxwidget.Text{Value: countLabel, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: palette.actionText},
				woxwidget.TextBlock{Value: preview, Width: max(float32(0), fieldWidth-22), Height: max(float32(0), height-52), MaxLines: 2, Style: woxui.TextStyle{Size: 10}, Color: palette.actionHeader},
			}}},
		},
	}}}
}

// buildFormTableOverlay renders the same GPU-backed table workflow over launcher forms and the settings page.
func (a *App) buildFormTableOverlay(snapshot *formTableEditorSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	panelWidth := max(float32(0), min(float32(760), width-28))
	panelHeight := max(float32(0), min(float32(640), height-28))
	left := max(float32(14), (width-panelWidth)/2)
	top := max(float32(14), (height-panelHeight)/2)
	panel := a.buildFormTablePanel(snapshot, palette, panelWidth, panelHeight)
	shade := palette.background
	shade.A = 205
	return woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
		{Child: woxwidget.Gesture{ID: "form-table-modal-shade", OnTap: func() {}, OnScroll: func(woxui.Point) {}, Child: woxwidget.Container{Width: width, Height: height, Color: shade}}},
		{Left: left, Top: top, Child: panel},
	}}
}

func (a *App) buildFormTablePanel(snapshot *formTableEditorSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-32)
	header := woxwidget.Container{Width: innerWidth, Height: 52, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
		woxwidget.Text{Value: a.translate(formTableTitle(snapshot.definition)), Style: woxui.TextStyle{Size: 18, Weight: woxui.FontWeightSemibold}, Color: palette.actionText},
		woxwidget.Text{Value: fmt.Sprintf("%d rows · shared Go table editor", len(snapshot.rows)), Style: woxui.TextStyle{Size: 11}, Color: palette.actionHeader},
	}}}
	var body woxwidget.Widget
	if snapshot.appPicker != nil {
		body = a.buildFormTableAppPicker(snapshot.appPicker, palette, innerWidth, max(float32(120), height-84))
	} else if snapshot.rowForm != nil {
		body = a.buildFormTableRowEditor(snapshot, palette, innerWidth, max(float32(120), height-84))
	} else {
		body = a.buildFormTableList(snapshot, palette, innerWidth, max(float32(120), height-84))
	}
	return woxwidget.Container{
		Width: width, Height: height, Radius: 12, Color: palette.actionBackground, Padding: woxwidget.UniformInsets(16),
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{header, body}},
	}
}

func (a *App) buildFormTableList(snapshot *formTableEditorSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	footerHeight := float32(54)
	statusHeight := float32(28)
	viewportHeight := max(float32(48), height-footerHeight-statusHeight)
	a.setFormTableListViewport(viewportHeight)
	rows := make([]woxwidget.Widget, 0, len(snapshot.rows))
	for index, row := range snapshot.rows {
		index := index
		row := row
		background := palette.queryBackground
		foreground := palette.actionText
		if index == snapshot.selected {
			background = palette.selectedBackground
			foreground = palette.selectedTitle
		}
		rows = append(rows, woxwidget.Gesture{
			ID:    fmt.Sprintf("form-table-row-%d", index),
			OnTap: func() { a.selectFormTableRow(index) },
			Child: woxwidget.Container{Width: width, Height: formTableListRowHeight, Radius: 7, Color: background, Padding: woxwidget.Insets{Left: 12, Top: 15, Right: 10}, Child: woxwidget.Text{
				Value: a.formTableRowSummary(snapshot.definition, row), Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: foreground,
			}},
		})
	}
	var list woxwidget.Widget
	if len(rows) == 0 {
		list = woxwidget.Container{Width: width, Height: viewportHeight, Radius: 8, Color: palette.queryBackground, Padding: woxwidget.Insets{Left: 16, Top: 18}, Child: woxwidget.Text{
			Value: "No rows yet. Choose Add row to create one.", Style: woxui.TextStyle{Size: 12}, Color: palette.actionHeader,
		}}
	} else {
		list = woxwidget.Gesture{ID: "form-table-list-scroll", OnScroll: func(delta woxui.Point) { a.scrollFormTableList(-delta.Y) }, Child: woxwidget.ScrollView{
			Width: width, Height: viewportHeight, ContentHeight: max(viewportHeight, float32(len(rows))*formTableListRowHeight), Offset: snapshot.listScroll,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		}}
	}
	status := snapshot.status
	if status == "" {
		status = "↑↓ select · Enter edit · Delete remove · Ctrl+N add · Esc close"
	}
	statusColor := palette.actionHeader
	if snapshot.invalid {
		statusColor = woxui.Color{R: 232, G: 95, B: 95, A: 255}
	}
	deleteLabel := "Delete"
	if snapshot.deleteArmed == snapshot.selected && snapshot.selected >= 0 {
		deleteLabel = "Confirm"
	}
	selectedReadOnly := snapshot.selected >= 0 && snapshot.selected < len(snapshot.rows) && formTableSkillRowReadOnly(snapshot.definition, snapshot.rows[snapshot.selected])
	canEdit := !snapshot.invalid && !snapshot.saving && snapshot.selected >= 0 && snapshot.definition.Value.Key != "AISkills" && !selectedReadOnly
	canDelete := !snapshot.invalid && !snapshot.saving && snapshot.selected >= 0 && !selectedReadOnly
	addLabel := "Add row"
	leftButtons := []woxwidget.Widget{
		a.buildFormTableButton("form-table-add", addLabel, 104, !snapshot.invalid && !snapshot.saving, false, a.beginAddFormTableRow, palette),
		a.buildFormTableButton("form-table-edit", "Edit", 86, canEdit, false, a.beginEditFormTableRow, palette),
		a.buildFormTableButton("form-table-delete", deleteLabel, 86, canDelete, false, a.deleteFormTableRow, palette),
	}
	fixedWidth := float32(104 + 86 + 86 + 104)
	if snapshot.definition.Value.Key == "AISkills" {
		addLabel = "Add local"
		leftButtons[0] = a.buildFormTableButton("form-table-add", addLabel, 104, !snapshot.invalid && !snapshot.saving, false, a.beginAddFormTableRow, palette)
		leftButtons = append(leftButtons, a.buildFormTableButton("form-table-clone", "Clone remote", 112, !snapshot.invalid && !snapshot.saving, false, a.beginCloneRemoteAISkill, palette))
		fixedWidth += 112
	}
	buttonChildren := append([]woxwidget.Widget(nil), leftButtons...)
	buttonChildren = append(buttonChildren, woxwidget.Painter{Width: max(float32(0), width-fixedWidth-float32(len(leftButtons)+1)*8), Height: 38})
	buttonChildren = append(buttonChildren, a.buildFormTableButton("form-table-close", a.translate("i18n:ui_close"), 104, true, true, a.closeFormTableEditor, palette))
	buttons := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: buttonChildren}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		list,
		woxwidget.Container{Width: width, Height: statusHeight, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Text{Value: status, Style: woxui.TextStyle{Size: 10}, Color: statusColor}},
		woxwidget.Container{Width: width, Height: footerHeight, Padding: woxwidget.Insets{Top: 8}, Child: buttons},
	}}
}

func (a *App) buildFormTableRowEditor(snapshot *formTableEditorSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	rowForm := snapshot.rowForm
	footerHeight := float32(54)
	titleHeight := float32(32)
	statusHeight := float32(0)
	if snapshot.status != "" {
		statusHeight = 28
	}
	bodyHeight := max(float32(48), height-titleHeight-footerHeight-statusHeight)
	a.setFormTableRowViewport(bodyHeight)
	callbacks := formFieldCallbacks{idPrefix: "form-table-row", focus: a.focusFormTableRowField, change: a.changeFormTableRowChoice, setCaret: a.setFormTableRowCaret, pickDir: a.pickFormTableRowDirectory, pickApp: a.openFormTableAppPicker, recordKey: a.recordFormTableRowHotkey}
	rows := make([]woxwidget.Widget, 0, len(rowForm.definitions))
	for index, definition := range rowForm.definitions {
		rows = append(rows, a.buildFormField(*rowForm, callbacks, palette, index, definition, width, formDefinitionHeight(definition)))
	}
	body := woxwidget.Gesture{ID: "form-table-row-scroll", OnScroll: func(delta woxui.Point) { a.scrollFormTableRow(-delta.Y) }, Child: woxwidget.ScrollView{
		Width: width, Height: bodyHeight, ContentHeight: max(bodyHeight, formDefinitionsContentHeight(rowForm.definitions)), Offset: rowForm.scroll,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
	}}
	title := "Add row"
	if snapshot.skillClone {
		title = "Clone remote skills"
	} else if snapshot.rowIndex >= 0 {
		title = "Edit row"
	}
	children := []woxwidget.Widget{
		woxwidget.Container{Width: width, Height: titleHeight, Child: woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: palette.actionText}},
		body,
	}
	if statusHeight > 0 {
		children = append(children, woxwidget.Container{Width: width, Height: statusHeight, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Text{
			Value: snapshot.status, Style: woxui.TextStyle{Size: 10}, Color: woxui.Color{R: 232, G: 95, B: 95, A: 255},
		}})
	}
	saveLabel := a.translate("i18n:ui_save")
	if snapshot.skillClone {
		saveLabel = "Clone"
	}
	buttons := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		woxwidget.Painter{Width: max(float32(0), width-210), Height: 38},
		a.buildFormTableButton("form-table-row-cancel", a.translate("i18n:ui_cancel"), 96, true, false, a.cancelFormTableRowEdit, palette),
		a.buildFormTableButton("form-table-row-save", saveLabel, 104, true, true, a.saveFormTableRowEdit, palette),
	}}
	children = append(children, woxwidget.Container{Width: width, Height: footerHeight, Padding: woxwidget.Insets{Top: 8}, Child: buttons})
	return woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}
}

func (a *App) buildFormTableButton(id, label string, width float32, enabled, primary bool, onTap func(), palette uiPalette) woxwidget.Widget {
	color := palette.queryBackground
	foreground := palette.actionText
	if primary {
		color = palette.actionSelected
		foreground = palette.actionSelectedText
	}
	if !enabled {
		foreground.A = 88
		onTap = func() {}
	}
	return woxwidget.Gesture{ID: id, OnTap: onTap, Child: woxwidget.Container{
		Width: width, Height: 38, Radius: 8, Color: color, Padding: woxwidget.Insets{Left: 16, Top: 11, Right: 12},
		Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: foreground},
	}}
}
