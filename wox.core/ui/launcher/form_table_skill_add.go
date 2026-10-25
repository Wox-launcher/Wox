package launcher

import (
	"context"
	"fmt"
	"strings"
	"time"
	woxcomponent "wox/ui/launcher/component"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

// formTableSkillAddState backs the Flutter-aligned add-skill dialog that accepts a
// local directory or a remote Git repository in one surface.
type formTableSkillAddState struct {
	tab     int // 0 = local directory, 1 = remote repository
	fields  *formFieldsState
	error   string
	cloning bool
}

// formTableSkillAddSnapshot copies the add-skill dialog for one UI frame.
type formTableSkillAddSnapshot struct {
	tab     int
	fields  *formFieldsSnapshot
	error   string
	cloning bool
}

// openFormTableSkillAdd opens the shared add-skill dialog on the local tab.
func (c *formTableController) openFormTableSkillAdd() {
	c.openFormTableSkillAddTab(0)
}

// openFormTableSkillAddTab keeps the inline table and the list editor footer both
// pointing at the same dialog, matching Flutter's single custom create dialog.
func (c *formTableController) openFormTableSkillAddTab(tab int) {
	state := c.settingsEditor
	if state == nil || state.definition.Value.Key != "AISkills" || state.invalid || state.saving || state.rowForm != nil || state.target != c.aiSettings().Form() {
		return
	}
	if tab != 1 {
		tab = 0
	}
	fields := newFormFieldsState([]formDefinition{
		{Type: "dirPath", Value: formDefinitionValue{Key: "Path", Label: "i18n:ui_ai_skill_add_path", MaxLines: 1}},
		{Type: "textbox", Value: formDefinitionValue{Key: "SourceUrl", Label: "i18n:plugin_ai_chat_skill_source_url", MaxLines: 1}},
	}, nil, true)
	setFormFieldsFocusLocked(&fields, tab)
	state.skillAdd = &formTableSkillAddState{tab: tab, fields: &fields}
	state.status = ""
	state.deletePending = -1
	state.deleteDirect = false
	c.updateFormTableTextInput(true)
	c.invalidateFormTableWindow()
}

// cancelFormTableSkillAdd dismisses the dialog and returns to the settings page.
func (c *formTableController) cancelFormTableSkillAdd() {
	state := c.activeFormTableEditor()
	if state != nil && state.skillAdd != nil && !state.skillAdd.cloning {
		c.closeFormTableEditor()
	}
}

// switchFormTableSkillAddTab changes the dialog between the local and remote inputs.
func (c *formTableController) switchFormTableSkillAddTab(tab int) {
	state := c.activeFormTableEditor()
	if state == nil || state.skillAdd == nil || state.skillAdd.cloning {
		return
	}
	if tab != 0 && tab != 1 {
		return
	}
	state.skillAdd.tab = tab
	state.skillAdd.error = ""
	setFormFieldsFocusLocked(state.skillAdd.fields, tab)
	c.updateFormTableTextInput(true)
	c.invalidateFormTableWindow()
}

func (c *formTableController) focusFormTableSkillAddField(index int) {
	state := c.activeFormTableEditor()
	if state == nil || state.skillAdd == nil || index < 0 || index >= len(state.skillAdd.fields.definitions) || index != state.skillAdd.tab {
		return
	}
	fields := state.skillAdd.fields
	syncFormFieldsEditorLocked(fields)
	fields.active = true
	if fields.focused != index || fields.editor == nil {
		setFormFieldsFocusLocked(fields, index)
	}
	textInput := fields.editor != nil
	c.updateFormTableTextInput(textInput)
	c.invalidateFormTableWindow()
}

func (c *formTableController) setFormTableSkillAddText(index int, value string) {
	state := c.activeFormTableEditor()
	if state == nil || state.skillAdd == nil || !setFormFieldsTextLocked(state.skillAdd.fields, index, value) {
		return
	}
	state.skillAdd.error = ""
	c.invalidateFormTableWindow()
}

// pickFormTableSkillAddDirectory fills the local path field from the platform picker.
func (c *formTableController) pickFormTableSkillAddDirectory(index int) {
	state := c.activeFormTableEditor()
	if state == nil || state.skillAdd == nil || index != 0 || index >= len(state.skillAdd.fields.definitions) || state.skillAdd.fields.definitions[index].Type != "dirPath" {
		return
	}
	fields := state.skillAdd.fields
	c.updateFormTableTextInput(false)
	path, err := c.formTableNativeWindow().PickFile(woxui.FileDialogOptions{Directory: true})
	if c.activeFormTableEditor() != state || state.skillAdd == nil || state.skillAdd.fields != fields {
		return
	}
	if err != nil {
		state.skillAdd.error = err.Error()
	} else if path != "" {
		setFormFieldsFocusLocked(fields, index)
		fields.editor.SetText(path, false)
		syncFormFieldsEditorLocked(fields)
		state.skillAdd.error = ""
	}
	textInput := fields.editor != nil
	c.updateFormTableTextInput(textInput)
	c.invalidateFormTableWindow()
}

// addFormTableSkill validates the active tab and either appends a local skill or
// clones the remote repository, mirroring Flutter's add dialog confirm action.
func (c *formTableController) addFormTableSkill() {
	state := c.activeFormTableEditor()
	if state == nil || state.skillAdd == nil || state.skillAdd.cloning {
		return
	}
	syncFormFieldsEditorLocked(state.skillAdd.fields)
	if state.skillAdd.tab == 0 {
		path := strings.TrimSpace(state.skillAdd.fields.values["Path"])
		if path == "" {
			state.skillAdd.error = c.language.translate("i18n:ui_ai_skill_add_path_required")
			c.invalidateFormTableWindow()
			return
		}
		c.addLocalSkill(state, path)
		return
	}
	url := strings.TrimSpace(state.skillAdd.fields.values["SourceUrl"])
	if url == "" {
		state.skillAdd.error = c.language.translate("i18n:ui_ai_skill_add_url_required")
		c.invalidateFormTableWindow()
		return
	}
	c.beginRemoteSkillClone(state, url)
}

// addLocalSkill appends one Path row and persists it through the shared settings save.
func (c *formTableController) addLocalSkill(state *formTableEditorState, path string) {
	previousValue := state.target.values[state.definition.Value.Key]
	state.rows = append(state.rows, map[string]any{"Path": path})
	state.selected = len(state.rows) - 1
	if err := c.commitFormTableRowsLocked(state); err != nil {
		state.skillAdd.error = err.Error()
		c.invalidateFormTableWindow()
		return
	}
	key := state.definition.Value.Key
	value := state.target.values[key]
	state.saving = true
	c.settings.saving = true
	c.closeFormTableEditor()
	util.Go(c.lifecycleCtx, "save settings table after local skill add", func() {
		c.saveSettingsTable(state, key, value, previousValue)
	})
}

// beginRemoteSkillClone shows the cloning indicator while the repository is fetched.
func (c *formTableController) beginRemoteSkillClone(state *formTableEditorState, url string) {
	previousValue := state.target.values[state.definition.Value.Key]
	state.skillAdd.cloning = true
	state.skillAdd.error = ""
	c.updateFormTableTextInput(false)
	c.invalidateFormTableWindow()
	util.Go(c.lifecycleCtx, "clone AI skills from add dialog", func() {
		c.cloneRemoteSkillsForDialog(state, url, previousValue)
	})
}

// cloneRemoteSkillsForDialog discovers repository skills, appends them atomically,
// and persists the combined setting, keeping the dialog open on failure.
func (c *formTableController) cloneRemoteSkillsForDialog(state *formTableEditorState, url, previousValue string) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	loaded, err := c.services.CloneAISkills(ctx, c.sessionID, url)
	cancel()
	skills := make([]map[string]any, len(loaded))
	for index, skill := range loaded {
		skills[index] = map[string]any{
			"Path": skill.Path, "ManifestPath": skill.ManifestPath, "Name": skill.Name, "Description": skill.Description,
			"Error": skill.Error, "Source": skill.Source, "SourceName": skill.SourceName, "SourceUrl": skill.SourceURL, "Enabled": skill.Enabled,
		}
	}
	if err == nil && len(skills) == 0 {
		err = fmt.Errorf("the repository did not contain any skills")
	}

	var value string
	save := false
	_ = c.runOnUI("apply cloned AI skills to add dialog", func() {
		if c.settingsEditor != state || state.skillAdd == nil {
			return
		}
		state.skillAdd.cloning = false
		if err != nil {
			state.skillAdd.error = err.Error()
			c.updateFormTableTextInput(true)
			c.invalidateFormTableWindow()
			return
		}
		state.rows = append(state.rows, cloneFormTableRows(skills)...)
		state.selected = len(state.rows) - 1
		if commitErr := c.commitFormTableRowsLocked(state); commitErr != nil {
			state.skillAdd.error = commitErr.Error()
			c.updateFormTableTextInput(true)
			c.invalidateFormTableWindow()
			return
		}
		value = state.target.values[state.definition.Value.Key]
		save = true
		state.saving = true
		c.settings.saving = true
		c.closeFormTableEditor()
	})
	if save {
		c.saveSettingsTable(state, "AISkills", value, previousValue)
	}
}

// buildFormTableSkillAddDialog maps the add-skill state onto the shared dialog surface.
func (c *formTableController) buildFormTableSkillAddDialog(snapshot *formTableSkillAddSnapshot, palette woxcomponent.ControlTheme, width, height, imageScale float32) woxwidget.Widget {
	fields := snapshot.fields
	callbacks := formFieldCallbacks{
		idPrefix:   "form-table-skill-add",
		imageScale: imageScale,
		focus:      c.focusFormTableSkillAddField,
		setText:    c.setFormTableSkillAddText,
		onKey:      c.onFormTableKey,
		pickDir:    c.pickFormTableSkillAddDirectory,
	}
	theme := palette
	cancelLabel := c.language.translate("i18n:ui_cancel")
	addLabel := c.language.translate("i18n:ui_add")
	fieldWidth := max(float32(0), min(float32(480), width-140))
	definition := fields.definitions[snapshot.tab]
	field := c.buildFormTableRowField(*fields, callbacks, palette, snapshot.tab, definition, fieldWidth, c.formTableRowLabelWidth(fields.definitions), "")
	return launcherview.FormTableSkillAddDialog(launcherview.FormTableSkillAddDialogProps{
		Width: width, Height: height,
		Title:      c.language.translate("i18n:ui_ai_skill_add"),
		LocalLabel: c.language.translate("i18n:ui_ai_skill_add_local"), RemoteLabel: c.language.translate("i18n:ui_ai_skill_add_remote"),
		LocalHint: c.language.translate("i18n:ui_ai_skill_add_local_hint"), RemoteHint: c.language.translate("i18n:ui_ai_skill_add_remote_hint"),
		Tab: snapshot.tab, Error: snapshot.error, Cloning: snapshot.cloning, CloningLabel: c.language.translate("i18n:ui_ai_skill_cloning"),
		CancelLabel: cancelLabel, AddLabel: addLabel, CancelWidth: c.formTableButtonWidth(cancelLabel, 80), AddWidth: c.formTableButtonWidth(addLabel, 80),
		Field: field, FieldHeight: launcherview.FormTableRowFieldHeight(definition.Type, "", 1), Theme: theme,
		OnTab: c.switchFormTableSkillAddTab, OnCancel: c.cancelFormTableSkillAdd, OnAdd: c.addFormTableSkill,
	})
}
