package launcher

import (
	"encoding/json"
	"fmt"
	"strings"

	launcherview "wox/ui/launcher/view"
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
	return launcherview.FormTableField(launcherview.FormTableFieldProps{
		ID: fmt.Sprintf("%s-field-%d", callbacks.idPrefix, index), Label: a.translate(formTableTitle(definition)), CountLabel: countLabel, Preview: preview,
		Width: width, Height: height, Focused: fields.active && fields.focused == index, Theme: palette.componentTheme(),
		OnTap: func() {
			callbacks.focus(index)
			if callbacks.openTable != nil {
				callbacks.openTable(index)
			}
		},
	})
}

// buildFormTableOverlay maps table editor state into the shared modal view.
func (a *App) buildFormTableOverlay(snapshot *formTableEditorSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	panelWidth := max(float32(0), min(float32(760), width-28))
	panelHeight := max(float32(0), min(float32(640), height-28))
	innerWidth := max(float32(0), panelWidth-32)
	bodyHeight := max(float32(120), panelHeight-84)
	var body woxwidget.Widget
	if snapshot.appPicker != nil {
		body = a.buildFormTableAppPicker(snapshot.appPicker, palette, innerWidth, bodyHeight)
	} else if snapshot.rowForm != nil {
		body = a.buildFormTableRowEditor(snapshot, palette, innerWidth, bodyHeight)
	} else {
		body = a.buildFormTableList(snapshot, palette, innerWidth, bodyHeight)
	}
	return launcherview.FormTableOverlay(launcherview.FormTableOverlayProps{
		Width: width, Height: height, Title: a.translate(formTableTitle(snapshot.definition)),
		Subtitle: fmt.Sprintf("%d rows · shared Go table editor", len(snapshot.rows)), Body: body, Theme: palette.componentTheme(),
	})
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
	callbacks := formFieldCallbacks{idPrefix: "form-table-row", focus: a.focusFormTableRowField, change: a.changeFormTableRowChoice, setCaret: a.setFormTableRowCaret, pickDir: a.pickFormTableRowDirectory, pickApp: a.openFormTableAppPicker, recordKey: a.recordFormTableRowHotkey}
	rows := make([]woxwidget.Widget, 0, len(rowForm.definitions))
	for index, definition := range rowForm.definitions {
		rows = append(rows, a.buildFormField(*rowForm, callbacks, palette, index, definition, width, formDefinitionHeight(definition)))
	}
	title := "Add row"
	if snapshot.skillClone {
		title = "Clone remote skills"
	} else if snapshot.rowIndex >= 0 {
		title = "Edit row"
	}
	saveLabel := a.translate("i18n:ui_save")
	if snapshot.skillClone {
		saveLabel = "Clone"
	}
	return launcherview.FormTableRowEditor(launcherview.FormTableRowEditorProps{
		Width: width, Height: height, Title: title, Rows: rows, ContentHeight: formDefinitionsContentHeight(rowForm.definitions), Scroll: rowForm.scroll,
		Status: snapshot.status, CancelLabel: a.translate("i18n:ui_cancel"), SaveLabel: saveLabel, Theme: palette.componentTheme(),
		OnSetViewport: a.setFormTableRowViewport, OnScroll: a.scrollFormTableRow, OnCancel: a.cancelFormTableRowEdit, OnSave: a.saveFormTableRowEdit,
	})
}
