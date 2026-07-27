package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestFormSwitchFieldUsesAccessibleSwitch(t *testing.T) {
	field := FormSwitchField(FormSwitchFieldProps{ID: "history", Label: "History", Width: 400, Height: 40, LabelWidth: 100, Checked: true, Theme: woxcomponent.Theme{}, OnChange: func(bool) {}})
	row := field.(woxwidget.Container).Child.(woxwidget.Flex)
	controlColumn := row.Children[1].(woxwidget.Flex)
	control := controlColumn.Children[0].(woxwidget.Semantics)
	if control.Role != woxui.AccessibilityRoleCheckBox || !control.Checked {
		t.Fatalf("switch semantics = role %q checked %v", control.Role, control.Checked)
	}
}

func TestFormSelectFieldUsesOutlinedDropdown(t *testing.T) {
	field := FormSelectField(FormSelectFieldProps{ID: "action", Label: "Action", Value: "Paste", Width: 400, Height: 44, LabelWidth: 100, Theme: woxcomponent.Theme{}})
	row := field.(woxwidget.Container).Child.(woxwidget.Flex)
	controlColumn := row.Children[1].(woxwidget.Flex)
	semantics := controlColumn.Children[0].(woxwidget.Semantics)
	gesture := semantics.Child.(woxwidget.Gesture)
	control := gesture.Child.(woxwidget.Container)
	if control.BorderWidth != 1 {
		t.Fatalf("dropdown border width = %.0f, want 1", control.BorderWidth)
	}
	content := control.Child.(woxwidget.Flex)
	if len(content.Children) != 2 {
		t.Fatalf("dropdown child count = %d, want value and indicator", len(content.Children))
	}
}

func TestFormTextFieldKeepsSuffixOutsideInput(t *testing.T) {
	field := FormTextField(FormTextFieldProps{ID: "days", Label: "Days", Suffix: "天", Width: 400, Height: 44, LabelWidth: 100, MaxLines: 1, Theme: woxcomponent.Theme{}})
	row := field.(woxwidget.Container).Child.(woxwidget.Flex)
	controlColumn := row.Children[1].(woxwidget.Flex)
	valueRow := controlColumn.Children[0].(woxwidget.Flex)
	suffix := valueRow.Children[1].(woxwidget.Align).Child.(woxwidget.Text)
	if suffix.Value != "天" {
		t.Fatalf("suffix = %q, want 天", suffix.Value)
	}
}
