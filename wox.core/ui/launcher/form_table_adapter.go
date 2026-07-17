package launcher

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
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
	rows, err := decodeFormTableRows(fields.values[definition.Value.Key])
	if err != nil {
		rows = nil
	}
	theme := palette.componentTheme()
	foreground := theme.ResultSubtitle
	visibleColumns := make([]formTableColumn, 0, len(definition.Value.Columns))
	for _, column := range definition.Value.Columns {
		if !column.HideInTable {
			visibleColumns = append(visibleColumns, column)
		}
	}
	columns := make([]launcherview.FormTableColumn, len(visibleColumns))
	for columnIndex, column := range visibleColumns {
		columns[columnIndex] = launcherview.FormTableColumn{Label: a.translate(column.Label), Tooltip: a.translate(column.Tooltip), Width: float32(column.Width)}
	}
	viewRows := a.formTableViewRows(definition, visibleColumns, rows, theme)
	onTooltip := (func(bool, string, woxui.Rect))(nil)
	if callbacks.idPrefix == "hotkey-settings" || callbacks.idPrefix == "plugin-settings" {
		onTooltip = a.setSettingChoiceTooltip
	}
	openTable := func() {
		if callbacks.focus != nil {
			callbacks.focus(index)
		}
		if callbacks.openTable != nil {
			callbacks.openTable(index)
		}
	}
	return launcherview.FormTableField(launcherview.FormTableFieldProps{
		ID: fmt.Sprintf("%s-field-%d", callbacks.idPrefix, index), Title: a.translate(formTableTitle(definition)), Description: a.translate(definition.Value.Tooltip),
		Width: width, Height: height, MaxHeight: definition.Value.MaxHeight, InlineTitle: definition.Value.InlineTable, Invalid: err != nil,
		Columns: columns, Rows: viewRows, AddLabel: a.translate("i18n:ui_add"), EditLabel: a.translate("i18n:ui_setting_theme_edit"), DeleteLabel: a.translate("i18n:ui_delete"),
		OperationLabel: a.translate("i18n:ui_operation"), EmptyLabel: a.translate("i18n:ui_no_data"),
		InfoIcon: a.imageForTint(settingNavIconSource("about"), &foreground, 16), AddIcon: a.imageForTint(settingControlIconSource("add"), &foreground, 16),
		EditIcon: a.imageForTint(settingControlIconSource("edit"), &foreground, 16), DeleteIcon: a.imageForTint(settingControlIconSource("delete"), &foreground, 16), EmptyIcon: a.imageForTint(settingControlIconSource("inbox"), &foreground, 24),
		Theme: theme, OnTooltip: onTooltip,
		OnAdd: func() {
			openTable()
			a.beginAddFormTableRowDirect()
		},
		OnOpenRow: func(rowIndex int) {
			openTable()
			a.selectFormTableRow(rowIndex)
			a.beginEditFormTableRowDirect()
		},
		OnDeleteRow: func(rowIndex int) {
			openTable()
			a.selectFormTableRow(rowIndex)
			a.deleteFormTableRow()
		},
	})
}

func (a *App) formTableViewRows(definition formDefinition, columns []formTableColumn, rows []map[string]any, theme woxcomponent.Theme) []launcherview.FormTableRow {
	type indexedRow struct {
		index int
		row   map[string]any
	}
	ordered := make([]indexedRow, len(rows))
	for index, row := range rows {
		ordered[index] = indexedRow{index: index, row: row}
	}
	if definition.Value.SortColumnKey != "" {
		sort.SliceStable(ordered, func(left, right int) bool {
			leftValue := fmt.Sprint(ordered[left].row[definition.Value.SortColumnKey])
			rightValue := fmt.Sprint(ordered[right].row[definition.Value.SortColumnKey])
			if strings.EqualFold(definition.Value.SortOrder, "desc") {
				return leftValue > rightValue
			}
			return leftValue < rightValue
		})
	}
	viewRows := make([]launcherview.FormTableRow, 0, len(ordered))
	for _, current := range ordered {
		cells := make([]launcherview.FormTableCell, len(columns))
		for columnIndex, column := range columns {
			cells[columnIndex] = a.formTableViewCell(column, current.row, theme)
		}
		viewRows = append(viewRows, launcherview.FormTableRow{Index: current.index, Cells: cells})
	}
	return viewRows
}

func (a *App) formTableViewCell(column formTableColumn, row map[string]any, theme woxcomponent.Theme) launcherview.FormTableCell {
	cell := launcherview.FormTableCell{Text: compactFormTableText(a.formTableDisplayValue(column, row), 80)}
	iconTint := theme.ResultTitle
	if column.Type == "checkbox" {
		iconName := "checkbox.unchecked"
		if formTableColumnValue(column, row) == "true" {
			iconName = "checkbox.checked"
		}
		cell.Text = ""
		cell.Icon = a.imageForTint(settingControlIconSource(iconName), &iconTint, 16)
		return cell
	}
	if column.Type == "app" {
		encoded, _ := json.Marshal(row[column.Key])
		var app ignoredHotkeyApp
		if json.Unmarshal(encoded, &app) == nil {
			cell.Text = app.Name
			if cell.Text == "" {
				cell.Text = app.Identity
			}
			cell.Icon = a.imageFor(app.Icon)
		}
		return cell
	}
	if column.Type == "woxImage" {
		encoded, _ := json.Marshal(row[column.Key])
		var icon woxImage
		if json.Unmarshal(encoded, &icon) == nil {
			cell.Icon = a.imageFor(icon)
			if icon.ImageType == "emoji" {
				cell.Text = icon.ImageData
			}
		}
	}
	return cell
}

// buildFormTableOverlay maps table editor state into the shared modal view.
func (a *App) buildFormTableOverlay(snapshot *formTableEditorSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	panelWidth := max(float32(0), min(float32(760), width-28))
	panelHeight := max(float32(0), min(float32(640), height-28))
	innerWidth := max(float32(0), panelWidth-32)
	bodyHeight := max(float32(120), panelHeight-84)
	rowEditor := snapshot.rowForm != nil && snapshot.appPicker == nil
	if rowEditor {
		labelWidth := a.formTableRowLabelWidth(snapshot.rowForm.definitions)
		contentWidth := a.formTableRowEditorContentWidth(snapshot.definition, labelWidth)
		panelWidth = max(float32(0), min(contentWidth+48, width-64))
		innerWidth = max(float32(0), panelWidth-48)
		contentHeight := formTableRowContentHeight(snapshot.rowForm.definitions)
		statusHeight := float32(0)
		if snapshot.status != "" {
			statusHeight = 28
		}
		titleHeight := float32(0)
		if snapshot.skillClone {
			titleHeight = 32
		}
		panelHeight = max(float32(0), min(contentHeight+titleHeight+62+statusHeight+48, height-56))
		bodyHeight = max(float32(120), panelHeight-48)
	}
	var body woxwidget.Widget
	if snapshot.appPicker != nil {
		body = a.buildFormTableAppPicker(snapshot.appPicker, palette, innerWidth, bodyHeight)
	} else if snapshot.rowForm != nil {
		body = a.buildFormTableRowEditor(snapshot, palette, innerWidth, bodyHeight)
	} else {
		body = a.buildFormTableList(snapshot, palette, innerWidth, bodyHeight)
	}
	overlay := launcherview.FormTableOverlay(launcherview.FormTableOverlayProps{
		Width: width, Height: height, PanelWidth: panelWidth, PanelHeight: panelHeight, Title: a.translate(formTableTitle(snapshot.definition)), RowEditor: rowEditor,
		Subtitle: fmt.Sprintf("%d rows · shared Go table editor", len(snapshot.rows)), Body: body, Theme: palette.componentTheme(),
	})
	if snapshot.choicePicker == nil {
		return overlay
	}
	return woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
		{Child: overlay},
		{Child: a.buildFormTableChoicePicker(snapshot.choicePicker, palette, width, height)},
	}}
}

// formTableRowLabelWidth mirrors Flutter's measured and bounded label column.
func (a *App) formTableRowLabelWidth(definitions []formDefinition) float32 {
	width := float32(60)
	window := a.formTableNativeWindow()
	if window == nil {
		return width
	}
	style := woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}
	for _, definition := range definitions {
		label := strings.TrimSpace(a.translate(definition.Value.Label))
		if label == "" {
			continue
		}
		if metrics, err := window.MeasureText(label, style); err == nil {
			width = max(width, metrics.Size.Width+8)
		}
	}
	return min(width, float32(180))
}

// formTableRowEditorContentWidth preserves the table override while keeping Flutter's adaptive default.
func (a *App) formTableRowEditorContentWidth(definition formDefinition, labelWidth float32) float32 {
	if definition.Value.UpdateDialogWidth > 0 {
		return float32(definition.Value.UpdateDialogWidth)
	}
	maxColumnWidth := float32(100)
	for _, column := range definition.Value.Columns {
		maxColumnWidth = max(maxColumnWidth, float32(column.Width))
	}
	return max(float32(600), labelWidth+max(float32(320), maxColumnWidth))
}

func formTableRowContentHeight(definitions []formDefinition) float32 {
	height := float32(0)
	for _, definition := range definitions {
		height += launcherview.FormTableRowFieldHeight(definition.Type, definition.Value.Tooltip, definition.Value.MaxLines)
	}
	return height
}

func (a *App) buildFormTableList(snapshot *formTableEditorSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	rows := make([]string, 0, len(snapshot.rows))
	for _, row := range snapshot.rows {
		rows = append(rows, a.formTableRowSummary(snapshot.definition, row))
	}
	deleteLabel := "Delete"
	if snapshot.deleteArmed == snapshot.selected && snapshot.selected >= 0 {
		deleteLabel = "Confirm"
	}
	selectedReadOnly := snapshot.selected >= 0 && snapshot.selected < len(snapshot.rows) && formTableSkillRowReadOnly(snapshot.definition, snapshot.rows[snapshot.selected])
	canEdit := !snapshot.invalid && !snapshot.saving && snapshot.selected >= 0 && snapshot.definition.Value.Key != "AISkills" && !selectedReadOnly
	canDelete := !snapshot.invalid && !snapshot.saving && snapshot.selected >= 0 && !selectedReadOnly
	addLabel := "Add row"
	showClone := snapshot.definition.Value.Key == "AISkills"
	if showClone {
		addLabel = "Add local"
	}
	return launcherview.FormTableList(launcherview.FormTableListProps{
		Width: width, Height: height, Rows: rows, Selected: snapshot.selected, Scroll: snapshot.listScroll,
		Status: snapshot.status, StatusError: snapshot.invalid, AddLabel: addLabel, DeleteLabel: deleteLabel, CloseLabel: a.translate("i18n:ui_close"),
		CanAdd: !snapshot.invalid && !snapshot.saving, CanEdit: canEdit, CanDelete: canDelete, ShowClone: showClone, Theme: palette.componentTheme(),
		OnSetViewport: a.setFormTableListViewport, OnScroll: a.scrollFormTableList, OnSelect: a.selectFormTableRow,
		OnAdd: a.beginAddFormTableRow, OnEdit: a.beginEditFormTableRow, OnDelete: a.deleteFormTableRow, OnClone: a.beginCloneRemoteAISkill, OnClose: a.closeFormTableEditor,
	})
}

func (a *App) buildFormTableRowEditor(snapshot *formTableEditorSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	rowForm := snapshot.rowForm
	callbacks := formFieldCallbacks{idPrefix: "form-table-row", focus: a.focusFormTableRowField, change: a.changeFormTableRowChoice, setCaret: a.setFormTableRowCaret, openChoice: a.openFormTableRowChoice, pickDir: a.pickFormTableRowDirectory, pickApp: a.openFormTableAppPicker, recordKey: a.recordFormTableRowHotkey}
	labelWidth := a.formTableRowLabelWidth(rowForm.definitions)
	fieldWidth := max(float32(0), width-20)
	rows := make([]woxwidget.Widget, 0, len(rowForm.definitions))
	for index, definition := range rowForm.definitions {
		rows = append(rows, a.buildFormTableRowField(*rowForm, callbacks, palette, index, definition, fieldWidth, labelWidth))
	}
	title := ""
	if snapshot.skillClone {
		title = "Clone remote skills"
	}
	saveLabel := a.translate("i18n:ui_save")
	if snapshot.skillClone {
		saveLabel = "Clone"
	}
	return launcherview.FormTableRowEditor(launcherview.FormTableRowEditorProps{
		Width: width, Height: height, Title: title, Rows: rows, ContentHeight: formTableRowContentHeight(rowForm.definitions), Scroll: rowForm.scroll,
		Status: snapshot.status, CancelLabel: a.translate("i18n:ui_cancel"), SaveLabel: saveLabel, Theme: palette.componentTheme(),
		OnSetViewport: a.setFormTableRowViewport, OnScroll: a.scrollFormTableRow, OnCancel: a.cancelFormTableRowEdit, OnSave: a.saveFormTableRowEdit,
	})
}

// buildFormTableRowField maps the portable field definition onto the compact table-editor controls.
func (a *App) buildFormTableRowField(fields formFieldsSnapshot, callbacks formFieldCallbacks, palette uiPalette, index int, definition formDefinition, width, labelWidth float32) woxwidget.Widget {
	value := definition.Value
	fieldValue := fields.values[value.Key]
	focused := fields.active && fields.focused == index
	state := fields.editing
	if !focused {
		state = woxui.TextEditingState{Text: fieldValue}
	}
	if definition.Type == "password" {
		state.Text = strings.Repeat("•", len([]rune(state.Text)))
		state.Composition = strings.Repeat("•", len([]rune(state.Composition)))
	}
	height := launcherview.FormTableRowFieldHeight(definition.Type, a.translate(value.Tooltip), value.MaxLines)
	props := launcherview.FormTableRowFieldProps{
		ID: fmt.Sprintf("form-table-row-field-%d", index), Kind: definition.Type, Label: a.translate(value.Label), Description: a.translate(value.Tooltip),
		Value: fieldValue, Width: width, Height: height, LabelWidth: labelWidth, State: state, Focused: focused, Protected: definition.Type == "password",
		MaxLines: max(1, value.MaxLines), Window: a.formTableNativeWindow(), Theme: palette.componentTheme(),
		EmojiLabel: a.translate("i18n:ui_image_editor_emoji"), UploadLabel: a.translate("i18n:ui_image_editor_upload_image"), BrowseLabel: a.translate("i18n:ui_runtime_browse"),
		OnCaret: func(offset int) {
			callbacks.focus(index)
			callbacks.setCaret(index, offset)
		},
	}
	switch definition.Type {
	case "checkbox":
		props.Checked = fieldValue == "true"
		props.OnTap = func() {
			callbacks.focus(index)
			callbacks.change(index, 1)
		}
	case "select", "selectAIModel":
		selectedLabel := fieldValue
		for _, option := range value.Options {
			if option.Value == fieldValue {
				selectedLabel = a.translate(option.Label)
				break
			}
		}
		props.Value = selectedLabel
		props.OnChoiceTap = func(anchor woxui.Rect) { callbacks.openChoice(index, anchor) }
	case "hotkey", "dictationHotkey":
		recording, status := a.hotkeyRecordingFieldStatus("form-table-row", index)
		props.HotkeyLabels = formatHotkeyLabels(fieldValue)
		props.Recording = recording
		props.RecordingStatus = status
		props.Placeholder = a.translate("i18n:ui_hotkey_click_to_set")
		if recording {
			props.Placeholder = a.translate("i18n:ui_hotkey_recording")
		}
		props.OnTap = func() {
			callbacks.focus(index)
			callbacks.recordKey(index)
		}
	case "dirPath":
		props.OnBrowse = func() { callbacks.pickDir(index) }
	case "app":
		var app ignoredHotkeyApp
		_ = json.Unmarshal([]byte(fieldValue), &app)
		props.Value = app.Name
		if strings.TrimSpace(props.Value) == "" {
			props.Value = "Select application"
		}
		props.Detail = app.Path
		if strings.TrimSpace(props.Detail) == "" {
			props.Detail = app.Identity
		}
		props.OnTap = func() {
			callbacks.focus(index)
			callbacks.pickApp(index)
		}
	case "woxImage":
		image, emoji := formTableRowImagePreview(fieldValue)
		if emoji != "" {
			props.ImageEmoji = emoji
		} else if image.ImageType != "" {
			props.Image = a.imageFor(image)
		}
		iconTint := palette.componentTheme().ActionText
		props.EmojiIcon = a.imageForTint(settingControlIconSource("emoji"), &iconTint, 16)
		props.UploadIcon = a.imageForTint(settingControlIconSource("upload"), &iconTint, 16)
		props.EmojiWidth = a.formTableImageButtonWidth(props.EmojiLabel)
		props.UploadWidth = a.formTableImageButtonWidth(props.UploadLabel)
		props.OnEmoji = func() { a.beginFormTableRowEmojiEdit(index) }
		props.OnUpload = func() { a.pickFormTableRowImage(index) }
	case "label":
		props.Value = a.translate(value.Content)
	default:
		props.OnTap = func() { callbacks.focus(index) }
	}
	return launcherview.FormTableRowField(props)
}

// formTableImageButtonWidth keeps translated labels readable without widening compact Chinese buttons.
func (a *App) formTableImageButtonWidth(label string) float32 {
	window := a.formTableNativeWindow()
	if window == nil {
		return 98
	}
	metrics, err := window.MeasureText(label, woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold})
	if err != nil {
		return 98
	}
	return max(float32(98), metrics.Size.Width+42)
}

// formTableRowImagePreview separates directly rendered emoji from structured image sources.
func formTableRowImagePreview(value string) (woxImage, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return woxImage{}, "🤖"
	}
	if !strings.HasPrefix(value, "{") {
		return woxImage{ImageType: "emoji", ImageData: value}, value
	}
	var image woxImage
	if json.Unmarshal([]byte(value), &image) != nil {
		return woxImage{}, value
	}
	if image.ImageType == "emoji" {
		return image, image.ImageData
	}
	return image, ""
}
