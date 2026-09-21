package launcher

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestSubmitRequirementFormAttachesErrorsToFields(t *testing.T) {
	app := &App{requirementForm: &requirementFormState{
		formFieldsState: newFormFieldsState([]formDefinition{{
			Type: "textbox", Value: formDefinitionValue{Key: "api_key", Validators: []formValidator{{Type: "not_empty"}}},
		}, {
			Type: "textbox", Value: formDefinitionValue{Key: "client_id", Validators: []formValidator{{Type: "not_empty"}}},
		}}, map[string]string{"api_key": "", "client_id": "ok"}, false),
		key: "req",
	}}
	setFormFieldsFocusLocked(&app.requirementForm.formFieldsState, 1)
	app.submitRequirementForm()
	if app.requirementForm.error != "" {
		t.Fatalf("footer error = %q, want field-scoped validation", app.requirementForm.error)
	}
	if got := app.requirementForm.fieldErrors["api_key"]; got == "" {
		t.Fatal("empty api_key should show an error under that field")
	}
	if _, exists := app.requirementForm.fieldErrors["client_id"]; exists {
		t.Fatalf("filled client_id should not have a field error, got %#v", app.requirementForm.fieldErrors)
	}
	if app.requirementForm.focused != 0 {
		t.Fatalf("focused field = %d, want the first invalid field", app.requirementForm.focused)
	}
	syncFormFieldsEditorLocked(&app.requirementForm.formFieldsState)
	if got := app.requirementForm.values["api_key"]; got != "" {
		t.Fatalf("validation copied client_id into api_key: %q", got)
	}
	app.requirementForm.editor.SetText("new-key", false)
	syncFormFieldsEditorLocked(&app.requirementForm.formFieldsState)
	if app.requirementForm.values["api_key"] != "new-key" || app.requirementForm.values["client_id"] != "ok" {
		t.Fatalf("editing after validation changed the wrong field: %#v", app.requirementForm.values)
	}
}

func TestRequirementSaveLabelIncludesPrimaryEnterHotkey(t *testing.T) {
	got := requirementSaveLabel(func(key string) string {
		if key == "i18n:ui_save" {
			return "Save"
		}
		return key
	})
	wantKey := "Ctrl"
	if runtime.GOOS == "darwin" {
		wantKey = "Cmd"
	}
	if !strings.HasPrefix(got, "Save (") || !strings.Contains(got, wantKey) || !strings.Contains(got, "Enter") {
		t.Fatalf("save label = %q, want Save (%s+Enter)", got, wantKey)
	}
}

func TestRequirementFormPrimaryEnterSavesWithoutFieldFocus(t *testing.T) {
	app := &App{}
	event := woxui.KeyEvent{Key: woxui.KeyEnter, Modifiers: woxui.KeyModifierControl, Down: true}
	if runtime.GOOS == "darwin" {
		event.Modifiers = woxui.KeyModifierMeta
	}
	if app.onRequirementFormKey(event) {
		t.Fatal("ctrl/cmd+enter must not save when no requirement form is visible")
	}
	app.requirementForm = &requirementFormState{saving: true}
	if !app.onRequirementFormKey(event) {
		t.Fatal("ctrl/cmd+enter should save while the requirement preview is visible, even if the query box has focus")
	}
}

func TestRequirementFormTabMovesOneHostFocusPerPress(t *testing.T) {
	fields := newFormFieldsState([]formDefinition{
		{Type: "textbox", Value: formDefinitionValue{Key: "url"}},
		{Type: "textbox", Value: formDefinitionValue{Key: "token"}},
	}, nil, true)
	app := &App{requirementForm: &requirementFormState{formFieldsState: fields}}
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			woxwidget.Focusable{Key: "requirement-form-field-0", OnKey: app.onRequirementFormKey, Child: woxwidget.Container{Width: 100, Height: 30}},
			woxwidget.Focusable{Key: "requirement-form-field-1", OnKey: app.onRequirementFormKey, OnFocusChange: func(focused bool) {
				if focused {
					app.focusRequirementFormField(1)
				}
			}, Child: woxwidget.Container{Width: 100, Height: 30}},
		}}
	})
	host.AttachServices(formTableHostServices{})
	app.host = host
	displayList := woxui.DisplayList{}
	host.Frame(&displayList, woxui.FrameInfo{Size: woxui.Size{Width: 100, Height: 80}, PixelSize: woxui.PixelSize{Width: 100, Height: 80}, Scale: 1})
	host.RequestFocus("requirement-form-field-0")

	if !host.Key(woxui.KeyEvent{Key: woxui.KeyTab, Down: true}) || app.requirementForm.focused != 1 {
		t.Fatal("Tab from the first requirement field did not focus the second field")
	}
	host.Key(woxui.KeyEvent{Key: woxui.KeyTab})
	if app.requirementForm.focused != 1 {
		t.Fatal("Tab key release moved requirement focus back to the first field")
	}
}

func TestRequirementPreviewRendersSettingTooltipAsMarkdown(t *testing.T) {
	data := queryRequirementPreviewData{
		PluginID:   "spotify",
		PluginName: "spotify",
		Title:      "Spotify needs configuration",
		Message:    "Spotify Client ID is required.",
		SettingDefinitions: []formDefinition{{
			Type: "textbox",
			Value: formDefinitionValue{
				Key:      "clientId",
				Label:    "Spotify Client ID",
				Tooltip:  "Create an app at [Spotify Dashboard](https://developer.spotify.com/dashboard).",
				MaxLines: 1,
			},
		}},
		Values: map[string]string{"clientId": ""},
	}
	previewBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	result := queryResult{QueryID: "q1", ID: "r1"}
	preview := queryPreview{PreviewData: string(previewBytes)}
	_, key, err := requirementPreviewDataAndKey(result, preview)
	if err != nil {
		t.Fatal(err)
	}

	app := &App{
		aiSettings: newAISettingsController(CommonDeps{Translate: func(s string) string { return s }}),
		requirementForm: &requirementFormState{
			formFieldsState: newFormFieldsState(data.SettingDefinitions, data.Values, false),
			key:             key,
			pluginID:        data.PluginID,
			pluginName:      data.PluginName,
			title:           data.Title,
			message:         data.Message,
		},
	}
	widget := app.buildRequirementPreview(result, preview, uiPalette{}, 420, 360)
	root := widget.(woxwidget.Container)
	column := root.Child.(woxwidget.Flex)
	scroll := column.Children[2].(woxwidget.ScrollView)
	field := scroll.Child.(woxwidget.Flex).Children[0].(woxwidget.Keyed).Child.(woxwidget.Container)
	controlColumn := field.Child.(woxwidget.Flex).Children[1].(woxwidget.Expanded).Child.(woxwidget.Flex)
	if _, ok := controlColumn.Children[1].(woxwidget.TextBlock); ok {
		t.Fatal("requirement setting tooltip rendered as plain text instead of Markdown")
	}
	if !hasMarkdownLink(controlColumn.Children[1], "Spotify Dashboard") {
		t.Fatal("requirement setting tooltip was not rendered as Markdown")
	}
}

func hasMarkdownLink(widget woxwidget.Widget, label string) bool {
	switch node := widget.(type) {
	case woxwidget.Semantics:
		if node.Role == woxui.AccessibilityRoleLink && node.Label == label {
			return true
		}
		return hasMarkdownLink(node.Child, label)
	case woxwidget.Keyed:
		return hasMarkdownLink(node.Child, label)
	case woxwidget.Container:
		return hasMarkdownLink(node.Child, label)
	case woxwidget.Expanded:
		return hasMarkdownLink(node.Child, label)
	case woxwidget.Align:
		return hasMarkdownLink(node.Child, label)
	case woxwidget.ScrollView:
		return hasMarkdownLink(node.Child, label)
	case woxwidget.Flex:
		for _, child := range node.Children {
			if hasMarkdownLink(child, label) {
				return true
			}
		}
	case woxwidget.Wrap:
		for _, child := range node.Children {
			if hasMarkdownLink(child, label) {
				return true
			}
		}
	}
	return false
}
