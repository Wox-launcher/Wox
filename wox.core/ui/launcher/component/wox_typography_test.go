package component

import (
	"testing"

	woxwidget "wox/ui/widget"
)

func TestSettingFieldUsesSharedTypography(t *testing.T) {
	field := WoxSettingField(SettingFieldProps{Label: "Font", Description: "Used throughout Wox", Width: 400, Height: 66, LabelWidth: 180}).(woxwidget.Container)
	label := field.Child.(woxwidget.Flex).Children[0].(woxwidget.Container).Child.(woxwidget.Flex)
	labelText := label.Children[0].(woxwidget.Text)
	description := label.Children[1].(woxwidget.Text)

	if labelText.Style.Size != SettingsLabelFontSize || description.Style.Size != SettingsHelpFontSize {
		t.Fatalf("setting typography = %v/%v, want %v/%v", labelText.Style.Size, description.Style.Size, SettingsLabelFontSize, SettingsHelpFontSize)
	}
}

func TestSettingFieldCanAllocateRemainingWidthToLabel(t *testing.T) {
	field := WoxSettingField(SettingFieldProps{Label: "Backups", Width: 400, Child: woxwidget.Container{Width: 80}}).(woxwidget.Container)
	row := field.Child.(woxwidget.Flex)
	if _, ok := row.Children[0].(woxwidget.Expanded); !ok {
		t.Fatalf("automatic setting label slot = %T, want Expanded", row.Children[0])
	}
}
