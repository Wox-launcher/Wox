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
	"wox/util/filesearchservice"
)

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
	if column.Key == "Name" || column.Key == "AppCount" || column.Key == "DisplayCount" {
		if _, ok := row["Screens"]; ok {
			return windowManagerGroupDisplayValue(column, row)
		}
		if _, ok := row["Id"]; ok {
			return windowManagerGroupDisplayValue(column, row)
		}
	}
	value := formTableColumnValue(column, row)
	if column.Type == "aiMCPServerTools" {
		switch tools := row[column.Key].(type) {
		case []any:
			return fmt.Sprintf("%d tools", len(tools))
		case []string:
			return fmt.Sprintf("%d tools", len(tools))
		}
	}
	if column.Type == "aiSkillSource" {
		if strings.EqualFold(value, "remote") {
			return a.translate("i18n:ui_ai_skill_type_remote")
		}
		return a.translate("i18n:ui_ai_skill_type_local")
	}
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
	return launcherview.FormTableField(a.formTableFieldProps(fields, callbacks, palette, index, definition, width, height))
}

// formTableFieldProps maps one portable table definition into the shared Flutter-style table surface.
func (a *App) formTableFieldProps(fields formFieldsSnapshot, callbacks formFieldCallbacks, palette uiPalette, index int, definition formDefinition, width, height float32) launcherview.FormTableFieldProps {
	rows, err := decodeFormTableRows(fields.values[definition.Value.Key])
	if err != nil {
		rows = nil
	}
	theme := palette.componentTheme()
	foreground := theme.ResultSubtitle
	disabledForeground := foreground
	disabledForeground.A = woxcomponent.DisabledContentAlpha
	infoIconRasterSize := physicalImageSize(14, callbacks.imageScale)
	headerIconRasterSize := physicalImageSize(15, callbacks.imageScale)
	rowIconRasterSize := physicalImageSize(16, callbacks.imageScale)
	emptyIconRasterSize := physicalImageSize(24, callbacks.imageScale)
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
	viewRows := a.formTableViewRows(definition, visibleColumns, rows, theme, callbacks.imageScale)
	if isFileSearchRootsTable(callbacks.idPrefix, definition, a.selectedPluginID()) {
		volumeRows := fileSearchServiceVolumeRows(visibleColumns, filesearchservice.IndexedVolumeRoots(), a.translate("i18n:plugin_file_setting_service_volume_tooltip"))
		if len(volumeRows) > 0 {
			viewRows = append(volumeRows, viewRows...)
		}
	}
	onTooltip := (func(bool, string, woxui.Rect))(nil)
	if callbacks.idPrefix == "hotkey-settings" || callbacks.idPrefix == "plugin-settings" || callbacks.idPrefix == "ai-settings" {
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
	secondaryLabel := ""
	var secondaryIcon *woxui.Image
	var onSecondary func()
	demoKind := ""
	if callbacks.idPrefix == "hotkey-settings" {
		switch definition.Value.Key {
		case "QueryHotkeys":
			demoKind = "query-hotkeys"
		case "QueryShortcuts":
			demoKind = "query-shortcuts"
		case "TrayQueries":
			demoKind = "tray-queries"
		}
	}
	var demoIcon *woxui.Image
	if demoKind != "" {
		demoIcon = a.imageForTint(settingControlIconSource("demo"), &theme.ResultTitle, physicalImageSize(18, callbacks.imageScale))
	}
	if callbacks.idPrefix == "plugin-settings" && definition.Value.Key == "commands" && a.selectedPluginID() == aiCommandPluginID {
		secondaryLabel = a.translate("i18n:ui_ai_command_template_add_from_store")
		secondaryIcon = a.imageForTint(settingControlIconSource("store"), &foreground, headerIconRasterSize)
		onSecondary = func() { a.openAICommandTemplatePicker(index) }
	}
	hideCloneAction := false
	if callbacks.idPrefix == "plugin-settings" && isWindowManagerGroupsTable(definition) && a.selectedPluginID() == windowManagerPluginID {
		hideCloneAction = true
	}
	return launcherview.FormTableFieldProps{
		ID: fmt.Sprintf("%s-field-%d", callbacks.idPrefix, index), Title: a.translate(formTableTitle(definition)), Description: a.translate(definition.Value.Tooltip),
		Width: width, Height: height, LabelWidth: callbacks.labelWidth, MaxHeight: definition.Value.MaxHeight, InlineTitle: definition.Value.InlineTable, Invalid: err != nil,
		Columns: columns, Rows: viewRows, SecondaryLabel: secondaryLabel, HideCloneAction: hideCloneAction, AddLabel: a.translate("i18n:ui_add"), EditLabel: a.translate("i18n:ui_setting_theme_edit"), CloneLabel: a.translate("i18n:ui_clone_row"), DeleteLabel: a.translate("i18n:ui_delete"),
		OperationLabel: a.translate("i18n:ui_operation"), EmptyLabel: a.translate("i18n:ui_no_data"),
		InfoIcon: a.imageForTint(settingNavIconSource("about"), &foreground, infoIconRasterSize), DemoIcon: demoIcon, DemoKind: demoKind, SecondaryIcon: secondaryIcon, AddIcon: a.imageForTint(settingControlIconSource("add"), &foreground, headerIconRasterSize),
		EditIcon: a.imageForTint(settingControlIconSource("edit"), &foreground, rowIconRasterSize), CloneIcon: a.imageForTint(settingControlIconSource("copy"), &foreground, rowIconRasterSize), DeleteIcon: a.imageForTint(settingControlIconSource("delete"), &foreground, rowIconRasterSize),
		DisabledEditIcon: a.imageForTint(settingControlIconSource("edit"), &disabledForeground, rowIconRasterSize), DisabledCloneIcon: a.imageForTint(settingControlIconSource("copy"), &disabledForeground, rowIconRasterSize), DisabledDeleteIcon: a.imageForTint(settingControlIconSource("delete"), &disabledForeground, rowIconRasterSize),
		EmptyIcon: a.imageForTint(settingControlIconSource("inbox"), &foreground, emptyIconRasterSize),
		Theme:     theme, OnTooltip: onTooltip, OnDemoHover: a.setSettingsDemoHover, OnSecondary: onSecondary,
		OnAdd: func() {
			openTable()
			a.beginAddFormTableRowDirect()
		},
		OnOpenRow: func(rowIndex int) {
			if rowIndex < 0 || rowIndex >= len(rows) {
				return
			}
			openTable()
			a.selectFormTableRow(rowIndex)
			a.beginEditFormTableRowDirect()
		},
		OnCloneRow: func(rowIndex int) {
			if rowIndex < 0 || rowIndex >= len(rows) {
				return
			}
			openTable()
			a.selectFormTableRow(rowIndex)
			a.beginCloneFormTableRowDirect()
		},
		OnDeleteRow: func(rowIndex int) {
			if rowIndex < 0 || rowIndex >= len(rows) || formTableSkillRowReadOnly(definition, rows[rowIndex]) {
				return
			}
			openTable()
			a.selectFormTableRow(rowIndex)
			a.beginDeleteFormTableRowDirect()
		},
	}
}

func (a *App) formTableViewRows(definition formDefinition, columns []formTableColumn, rows []map[string]any, theme woxcomponent.Theme, imageScale float32) []launcherview.FormTableRow {
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
			cells[columnIndex] = a.formTableViewCell(column, current.row, theme, imageScale)
		}
		viewRows = append(viewRows, launcherview.FormTableRow{Index: current.index, Cells: cells})
	}
	return viewRows
}

func (a *App) formTableViewCell(column formTableColumn, row map[string]any, theme woxcomponent.Theme, imageScale float32) launcherview.FormTableCell {
	cell := launcherview.FormTableCell{Text: compactFormTableText(a.formTableDisplayValue(column, row), 80)}
	if column.Type == "aiModelStatus" {
		statusColor := woxui.Color{R: 69, G: 184, B: 88, A: 255}
		cell.Text = ""
		cell.IndicatorColor = &statusColor
		return cell
	}
	if column.Type == "checkbox" {
		cell.Text = ""
		if formTableColumnValue(column, row) == "true" {
			cell.Text = a.translate("i18n:ui_disabled")
		}
		return cell
	}
	if column.Type == "select" {
		value := formTableColumnValue(column, row)
		for _, option := range column.SelectOptions {
			if option.Value == value && option.Icon.ImageType != "" {
				cell.IconSize = 18
				cell.Icon = a.imageForSize(option.Icon, physicalImageSize(18, imageScale))
				break
			}
		}
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
		cell.Text = ""
		cell.IconSize = 24
		encoded, _ := json.Marshal(row[column.Key])
		var icon woxImage
		if json.Unmarshal(encoded, &icon) == nil {
			cell.Icon = a.imageForSize(icon, physicalImageSize(24, imageScale))
		}
		return cell
	}
	return cell
}

// buildFormTableOverlay maps table editor state into the shared modal view.
func (a *App) buildFormTableOverlay(snapshot *formTableEditorSnapshot, palette uiPalette, width, height, imageScale float32) woxwidget.Widget {
	if snapshot.skillAdd != nil {
		return a.buildFormTableSkillAddDialog(snapshot.skillAdd, palette, width, height, imageScale)
	}
	if snapshot.deletePending >= 0 && snapshot.deleteDirect {
		return a.buildFormTableDeleteDialog(palette, width, height)
	}
	if snapshot.windowGroupEditor != nil {
		return a.buildWindowManagerGroupEditor(snapshot.windowGroupEditor, palette, width, height, imageScale)
	}
	if snapshot.appPicker != nil {
		return a.buildFormTableAppPicker(snapshot.appPicker, palette, width, height, imageScale)
	}
	panelWidth := max(float32(0), min(float32(760), width-28))
	panelHeight := max(float32(0), min(float32(640), height-28))
	innerWidth := max(float32(0), panelWidth-32)
	bodyHeight := max(float32(120), panelHeight-84)
	rowEditor := snapshot.rowForm != nil
	if rowEditor {
		labelWidth := a.formTableRowLabelWidth(snapshot.rowForm.definitions)
		contentWidth := a.formTableRowEditorContentWidth(snapshot.definition, labelWidth)
		panelWidth = max(float32(0), min(contentWidth+48, width-64))
		innerWidth = max(float32(0), panelWidth-48)
		contentHeight := a.formTableRowContentHeightForWidth(snapshot.rowForm.definitions, snapshot.fieldErrors, max(float32(0), innerWidth-20), labelWidth)
		statusHeight := float32(0)
		if snapshot.status != "" {
			statusHeight = 28
		}
		panelHeight = max(float32(0), min(max(float32(48), contentHeight)+launcherview.FormTableRowEditorFooterHeight+statusHeight+48, height-56))
		if snapshot.definition.Value.Key == "QueryHotkeys" {
			panelHeight = max(float32(0), min(float32(632), height-56))
		}
		bodyHeight = max(float32(48), panelHeight-48)
	}
	var body woxwidget.Widget
	if snapshot.rowForm != nil {
		body = a.buildFormTableRowEditor(snapshot, palette, innerWidth, bodyHeight, imageScale)
	} else {
		body = a.buildFormTableList(snapshot, palette, innerWidth, bodyHeight)
	}
	overlay := launcherview.FormTableOverlay(launcherview.FormTableOverlayProps{
		Width: width, Height: height, PanelWidth: panelWidth, PanelHeight: panelHeight, Title: a.translate(formTableTitle(snapshot.definition)), RowEditor: rowEditor,
		Subtitle: fmt.Sprintf("%d rows · shared Go table editor", len(snapshot.rows)), Body: body, Theme: palette.componentTheme(),
	})
	layers := []woxwidget.StackChild{{Child: overlay}}
	if snapshot.deletePending >= 0 {
		layers = append(layers, woxwidget.StackChild{Child: a.buildFormTableDeleteDialog(palette, width, height)})
	}
	if snapshot.choicePicker != nil {
		layers = append(layers, woxwidget.StackChild{Child: a.buildFormTableChoicePicker(snapshot.choicePicker, palette, width, height, imageScale)})
	}
	if snapshot.emojiPicker != nil {
		layers = append(layers, woxwidget.StackChild{Child: a.buildFormTableEmojiPicker(snapshot.emojiPicker, palette, width, height, imageScale)})
	}
	if snapshot.queryVariable != nil {
		layers = append(layers, woxwidget.StackChild{Child: a.buildFormTableQueryVariablePicker(snapshot.queryVariable, palette, width, height, imageScale)})
	}
	return woxwidget.Stack{Width: width, Height: height, Children: layers}
}

func (a *App) buildFormTableQueryVariablePicker(snapshot *formTableQueryVariablePickerSnapshot, palette uiPalette, width, height, imageScale float32) woxwidget.Widget {
	surface := woxui.Color{R: 255, G: 255, B: 255, A: 255}
	if themeColorIsDark(palette.background) {
		surface = woxui.Color{R: 36, G: 36, B: 36, A: 255}
	}
	options := a.filteredQueryHotkeyVariables(snapshot.query)
	choices := make([]launcherview.QueryVariableChoice, 0, len(options))
	for _, option := range options {
		choices = append(choices, launcherview.QueryVariableChoice{
			Label: a.translate(option.label), Description: a.translate(option.description),
			Icon: a.imageForTint(settingControlIconSource(option.icon), &palette.resultTitle, physicalImageSize(18, imageScale)),
		})
	}
	return launcherview.QueryVariablePicker(launcherview.QueryVariablePickerProps{
		Width: width, Height: height, Anchor: snapshot.anchor, Choices: choices, Selected: min(snapshot.selected, len(choices)-1), Surface: surface, Theme: palette.componentTheme(),
		OnChoose: a.chooseFormTableQueryVariable, OnHover: func(index int) {
			if state := a.activeFormTableEditor(); state != nil && state.queryVariable != nil && state.queryVariable.selected != index {
				state.queryVariable.selected = index
				a.invalidateFormTableWindow()
			}
		}, OnCancel: a.closeFormTableQueryVariablePicker,
	})
}

func (a *App) buildFormTableDeleteDialog(palette uiPalette, width, height float32) woxwidget.Widget {
	return launcherview.FormTableDeleteDialog(launcherview.FormTableDeleteDialogProps{
		Width: width, Height: height, Message: a.translate("i18n:ui_delete_row_confirm"),
		CancelLabel: a.translate("i18n:ui_cancel"), DeleteLabel: a.translate("i18n:ui_delete"),
		Theme: palette.componentTheme(), OnCancel: a.cancelFormTableRowDelete, OnDelete: a.confirmFormTableRowDelete,
	})
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

func (a *App) formTableRowContentHeight(definitions []formDefinition, fieldErrors map[string]string) float32 {
	return a.formTableRowContentHeightForWidth(definitions, fieldErrors, 0, 0)
}

// formTableRowContentHeightForWidth sizes the scroll content with Flutter's field gap and wrapped markdown tips.
func (a *App) formTableRowContentHeightForWidth(definitions []formDefinition, fieldErrors map[string]string, fieldWidth, labelWidth float32) float32 {
	height := float32(0)
	visible := 0
	controlWidth := max(float32(0), fieldWidth-labelWidth-10)
	for _, definition := range definitions {
		visible++
		markdown := formTableRowFieldMarkdown(definition)
		height += launcherview.FormTableRowFieldHeightFor(definition.Type, a.translate(definition.Value.Tooltip), fieldErrors[definition.Value.Key], definition.Value.MaxLines, markdown, controlWidth)
	}
	if visible > 1 {
		height += float32(visible-1) * 10
	}
	return height
}

// formTableRowFieldMarkdown matches Flutter tooltips that render as markdown in the row editor.
func formTableRowFieldMarkdown(definition formDefinition) bool {
	return definition.Value.Tooltip == "i18n:ui_query_hotkeys_query_tooltip"
}

func (a *App) buildFormTableList(snapshot *formTableEditorSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	rows := make([]string, 0, len(snapshot.rows))
	for _, row := range snapshot.rows {
		rows = append(rows, a.formTableRowSummary(snapshot.definition, row))
	}
	selectedReadOnly := snapshot.selected >= 0 && snapshot.selected < len(snapshot.rows) && formTableSkillRowReadOnly(snapshot.definition, snapshot.rows[snapshot.selected])
	canEdit := !snapshot.invalid && !snapshot.saving && snapshot.selected >= 0 && snapshot.definition.Value.Key != "AISkills" && !selectedReadOnly
	canDelete := !snapshot.invalid && !snapshot.saving && snapshot.selected >= 0 && !selectedReadOnly
	addLabel := "Add row"
	onAdd := a.beginAddFormTableRow
	if snapshot.definition.Value.Key == "AISkills" {
		// The skills list shares Flutter's tabbed local/remote add dialog.
		addLabel = a.translate("i18n:ui_ai_skill_add")
		onAdd = a.openFormTableSkillAdd
	}
	return launcherview.FormTableList(launcherview.FormTableListProps{
		Width: width, Height: height, Rows: rows, Selected: snapshot.selected,
		Status: snapshot.status, StatusError: snapshot.invalid, AddLabel: addLabel, DeleteLabel: a.translate("i18n:ui_delete"), CloseLabel: a.translate("i18n:ui_close"),
		CanAdd: !snapshot.invalid && !snapshot.saving, CanEdit: canEdit, CanDelete: canDelete, Theme: palette.componentTheme(),
		OnSelect: a.selectFormTableRow,
		OnAdd:    onAdd, OnEdit: a.beginEditFormTableRow, OnDelete: a.deleteFormTableRow, OnClose: a.closeFormTableEditor,
	})
}

func (a *App) buildFormTableRowEditor(snapshot *formTableEditorSnapshot, palette uiPalette, width, height, imageScale float32) woxwidget.Widget {
	rowForm := snapshot.rowForm
	callbacks := formFieldCallbacks{idPrefix: "form-table-row", imageScale: imageScale, focus: a.focusFormTableRowField, change: a.changeFormTableRowChoice, setText: a.setFormTableRowText, onKey: a.onFormTableKey, openChoice: a.openFormTableRowChoice, pickDir: a.pickFormTableRowDirectory, pickApp: a.openFormTableAppPicker, recordKey: a.recordFormTableRowHotkey}
	definitions := rowForm.definitions
	if snapshot.definition.Value.Key == "QueryHotkeys" {
		visible := make([]formDefinition, 0, len(definitions))
		for _, definition := range definitions {
			if queryHotkeyFieldVisible(snapshot.queryPreset, definition.Value.Key, snapshot.rowIndex >= 0) {
				visible = append(visible, definition)
			}
		}
		definitions = visible
	}
	labelWidth := a.formTableRowLabelWidth(definitions)
	fieldWidth := max(float32(0), width-20)
	controlWidth := max(float32(0), fieldWidth-labelWidth-10)
	rows := make([]woxwidget.Widget, 0, len(rowForm.definitions))
	contentHeight := float32(0)
	var keepVisible *woxwidget.ScrollRange
	for index, definition := range rowForm.definitions {
		if snapshot.definition.Value.Key == "QueryHotkeys" && !queryHotkeyFieldVisible(snapshot.queryPreset, definition.Value.Key, snapshot.rowIndex >= 0) {
			continue
		}
		fieldError := snapshot.fieldErrors[definition.Value.Key]
		markdown := formTableRowFieldMarkdown(definition)
		fieldHeight := launcherview.FormTableRowFieldHeightFor(definition.Type, a.translate(definition.Value.Tooltip), fieldError, definition.Value.MaxLines, markdown, controlWidth)
		if len(rows) > 0 {
			contentHeight += 10
		}
		if rowForm.focused == index {
			keepVisible = &woxwidget.ScrollRange{Start: contentHeight, End: contentHeight + fieldHeight}
		}
		contentHeight += fieldHeight
		rows = append(rows, a.buildFormTableRowField(*rowForm, callbacks, palette, index, definition, fieldWidth, labelWidth, fieldError))
	}
	title := ""
	if snapshot.definition.Value.Key == "QueryHotkeys" {
		title = a.translate("i18n:ui_query_hotkeys_dialog_create_title")
		if snapshot.rowIndex >= 0 {
			title = a.translate("i18n:ui_query_hotkeys_dialog_edit_title")
		}
	}
	saveLabel := a.translate("i18n:ui_save")
	props := launcherview.FormTableRowEditorProps{
		Width: width, Height: height, Title: title, Rows: rows, ContentHeight: contentHeight, KeepVisible: keepVisible,
		Status: snapshot.status, CancelLabel: a.translate("i18n:ui_cancel"), SaveLabel: saveLabel, Theme: palette.componentTheme(),
		OnCancel: a.cancelFormTableRowEdit, OnSave: a.saveFormTableRowEdit,
	}
	if snapshot.definition.Value.Key == "QueryHotkeys" {
		demoIcon := a.imageForTint(settingControlIconSource("demo"), &palette.resultTitle, physicalImageSize(15, imageScale))
		props.HeaderHeight = 122
		props.Header = launcherview.QueryHotkeyEditorHeader(launcherview.QueryHotkeyEditorHeaderProps{
			Width: width, Title: title, Selected: string(snapshot.queryPreset), Description: a.translate("i18n:ui_query_hotkeys_preset_" + strings.ReplaceAll(string(snapshot.queryPreset), "-", "_") + "_description"),
			NormalLabel: a.translate("i18n:ui_query_hotkeys_preset_normal"), WebPanelLabel: a.translate("i18n:ui_query_hotkeys_preset_web_panel"),
			SilentLabel: a.translate("i18n:ui_query_hotkeys_preset_silent"), CustomLabel: a.translate("i18n:ui_query_hotkeys_preset_custom"),
			DemoIcon: demoIcon, DemoLabel: a.translate("i18n:ui_demo_preview"), Theme: palette.componentTheme(), OnSelect: a.applyQueryHotkeyPreset,
			OnOpenLink: a.openAboutLink,
			OnDemoHover: func(preset string, inside bool, anchor woxui.Rect) {
				a.setSettingsDemoHover("query-hotkey-preset-"+preset, inside, anchor)
			},
		})
	}
	return launcherview.FormTableRowEditor(props)
}

// buildFormTableRowField maps the portable field definition onto the compact table-editor controls.
func (a *App) buildFormTableRowField(fields formFieldsSnapshot, callbacks formFieldCallbacks, palette uiPalette, index int, definition formDefinition, width, labelWidth float32, fieldError string) woxwidget.Widget {
	value := definition.Value
	fieldValue := fields.values[value.Key]
	focused := fields.active && fields.focused == index
	state := fields.editing
	var controller *woxwidget.TextEditingController
	if !focused {
		state = woxui.TextEditingState{Text: fieldValue}
	} else {
		controller = fields.editor
	}
	markdown := formTableRowFieldMarkdown(definition)
	controlWidth := max(float32(0), width-labelWidth-10)
	height := launcherview.FormTableRowFieldHeightFor(definition.Type, a.translate(value.Tooltip), fieldError, value.MaxLines, markdown, controlWidth)
	props := launcherview.FormTableRowFieldProps{
		ID: fmt.Sprintf("form-table-row-field-%d", index), Kind: definition.Type, Label: a.translate(value.Label), Description: a.translate(value.Tooltip),
		DescriptionMarkdown: markdown, Error: fieldError, Value: fieldValue, Width: width, Height: height, LabelWidth: labelWidth, State: state, Focused: focused, Protected: definition.Type == "password",
		Controller: controller, MaxLines: max(1, value.MaxLines), Window: a.formTableNativeWindow(), Theme: palette.componentTheme(),
		EmojiLabel: a.translate("i18n:ui_image_editor_emoji"), UploadLabel: a.translate("i18n:ui_image_editor_upload_image"), BrowseLabel: a.translate("i18n:ui_runtime_browse"),
		SelectLabel: a.translate("i18n:ui_hotkey_ignore_apps_select"),
		OnFocus:     func() { callbacks.focus(index) },
		OnFocusChange: func(focused bool) {
			state := a.activeFormTableEditor()
			if focused || state == nil || state.rowForm == nil || !state.rowForm.active || state.rowForm.focused != index {
				return
			}
			syncFormFieldsEditorLocked(state.rowForm)
			state.rowForm.active = false
			a.updateFormTableTextInput(false)
			a.invalidateFormTableWindow()
		},
		OnChanged: func(value string) {
			if callbacks.setText != nil {
				callbacks.setText(index, value)
			}
		},
		OnSelectionChanged: func(selection woxui.TextSelection) {
			if state := a.activeFormTableEditor(); state != nil && state.rowForm != nil && state.rowForm.focused == index && state.rowForm.editor != nil {
				state.rowForm.editor.SetSelection(selection.Anchor, selection.Focus)
				if state.definition.Value.Key == "QueryHotkeys" && state.rowForm.definitions[index].Value.Key == "Query" {
					a.updateFormTableQueryVariableTrigger(index)
				}
			}
		},
		OnKey: callbacks.onKey,
	}
	if markdown {
		props.TrailingLabel = "{}"
		props.OnTrailingTap = func(anchor woxui.Rect) { a.openFormTableQueryVariablePicker(index, anchor) }
		props.OnOpenLink = a.openAboutLink
		actionTint := palette.componentTheme().ResultSubtitle
		props.ActionIcon = a.imageForTint(settingControlIconSource("bolt"), &actionTint, physicalImageSize(18, callbacks.imageScale))
		props.ActionLabel = a.translate("i18n:ui_query_hotkeys_test_query")
		props.OnActionTap = a.runFormTableQueryHotkeyTest
		props.OnActionHover = func(inside bool, anchor woxui.Rect) {
			a.setSettingChoiceTooltip(inside, a.translate("i18n:ui_query_hotkeys_test_query"), anchor)
		}
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
		var selectedIcon woxImage
		for _, option := range value.Options {
			if option.Value == fieldValue {
				selectedLabel = a.translate(option.Label)
				selectedIcon = option.Icon
				break
			}
		}
		props.Value = selectedLabel
		if selectedIcon.ImageType != "" {
			props.SelectIcon = a.imageForSize(selectedIcon, physicalImageSize(18, callbacks.imageScale))
		}
		props.OnChoiceTap = func(anchor woxui.Rect) { callbacks.openChoice(index, anchor) }
	case "hotkey", "dictationHotkey":
		presentation := a.hotkeyRecordingFieldStatus("form-table-row", index)
		if presentation.Active {
			fieldValue = presentation.Value
		}
		props.HotkeyLabels = formatHotkeyLabels(fieldValue)
		props.Recording = presentation.Active
		props.RecordingStatus = presentation.Status
		props.RecordingError = presentation.Error
		props.Hold = strings.HasPrefix(strings.TrimSpace(fieldValue), "hold:")
		props.HoldPrefix = a.translate("i18n:ui_hotkey_hold_prefix")
		props.Placeholder = a.translate("i18n:ui_hotkey_click_to_set")
		if presentation.Active {
			props.Placeholder = a.translate("i18n:ui_hotkey_recording")
		}
		props.OnTap = func() {
			callbacks.focus(index)
			callbacks.recordKey(index)
		}
		props.OnFocusChange = func(focused bool) {
			if focused {
				callbacks.focus(index)
				callbacks.recordKey(index)
				return
			}
			if a.hotkeyRecordingFieldStatus("form-table-row", index).Active {
				a.stopHotkeyRecording()
			}
		}
	case "dirPath":
		props.OnBrowse = func() { callbacks.pickDir(index) }
	case "app":
		var app ignoredHotkeyApp
		_ = json.Unmarshal([]byte(fieldValue), &app)
		props.Value = app.Name
		if strings.TrimSpace(props.Value) == "" {
			props.Value = a.translate("i18n:ui_hotkey_ignore_apps_app_placeholder")
		}
		props.Detail = app.Path
		if strings.TrimSpace(props.Detail) == "" {
			props.Detail = app.Identity
		}
		props.Image = a.imageForSize(app.Icon, physicalImageSize(24, callbacks.imageScale))
		props.SelectWidth = a.formTableButtonWidth(props.SelectLabel, 98)
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
		props.EmojiIcon = a.imageForTint(settingControlIconSource("emoji"), &iconTint, physicalImageSize(16, callbacks.imageScale))
		props.UploadIcon = a.imageForTint(settingControlIconSource("upload"), &iconTint, physicalImageSize(16, callbacks.imageScale))
		props.EmojiWidth = a.formTableImageButtonWidth(props.EmojiLabel)
		props.UploadWidth = a.formTableImageButtonWidth(props.UploadLabel)
		props.OnEmoji = func() { a.openFormTableEmojiPicker(index) }
		props.OnUpload = func() { a.pickFormTableRowImage(index) }
	case "label":
		props.Value = a.translate(value.Content)
	default:
		props.OnTap = func() { callbacks.focus(index) }
	}
	return launcherview.FormTableRowField(props)
}

// formTableButtonWidth preserves Flutter's intrinsic translated button width.
func (a *App) formTableButtonWidth(label string, minimum float32) float32 {
	window := a.formTableNativeWindow()
	if window == nil {
		return minimum
	}
	metrics, err := window.MeasureText(label, woxui.TextStyle{Size: 13})
	if err != nil {
		return minimum
	}
	return max(minimum, metrics.Size.Width+40)
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
