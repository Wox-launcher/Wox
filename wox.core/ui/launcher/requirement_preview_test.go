package launcher

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

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
