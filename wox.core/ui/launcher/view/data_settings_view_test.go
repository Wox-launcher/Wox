package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestDataLogLevelUsesSharedAnchoredDropdown(t *testing.T) {
	var openedAt woxui.Rect
	field := dataLogLevelField(DataSettingsProps{
		LogLevel: "DEBUG",
		Theme:    woxcomponent.Theme{},
		OnOpenLogLevel: func(anchor woxui.Rect) {
			openedAt = anchor
		},
	}, 800)

	container := field.(woxwidget.Container)
	row := container.Child.(woxwidget.Flex)
	keyed := row.Children[1].(woxwidget.Keyed)
	if keyed.Key != SettingChoiceAnchorKey("LogLevel") {
		t.Fatalf("dropdown key = %q, want LogLevel choice anchor", keyed.Key)
	}
	semantics := keyed.Child.(woxwidget.Semantics)
	if semantics.AutomationID != "data-log-level" || semantics.Role != woxui.AccessibilityRoleButton {
		t.Fatalf("dropdown semantics = %#v, want standard button", semantics)
	}
	trigger := semantics.Child.(woxwidget.Focusable).Child.(woxwidget.Gesture)
	if trigger.OnTap != nil || trigger.OnTapBounds == nil {
		t.Fatal("log level should open an anchored dropdown instead of changing directly")
	}
	anchor := woxui.Rect{X: 10, Y: 20, Width: 280, Height: 34}
	trigger.OnTapBounds(anchor)
	if openedAt != anchor {
		t.Fatalf("opened anchor = %#v, want %#v", openedAt, anchor)
	}
}

func TestDataStorageFieldButtonsExpandForLongLocalizedLabels(t *testing.T) {
	field := dataStorageField(DataSettingsProps{
		Labels: DataSettingsLabels{
			Open:           "Open",
			LocationChange: "Change Location Path",
			LocationTitle:  "Location",
		},
	}, 820).(woxwidget.Container)

	row := field.Child.(woxwidget.Flex)
	label := row.Children[0].(woxwidget.Expanded)
	actionsContainer := row.Children[1].(woxwidget.Container)
	actions := actionsContainer.Child.(woxwidget.Flex)
	changeButton := actions.Children[1].(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container)

	if changeButton.Width != 0 {
		t.Fatalf("change button width = %v, want content-sized", changeButton.Width)
	}
	expectedActionsWidth := dataCompactButtonWidth("Open", 76) + 10 + dataCompactButtonWidth("Change Location Path", 112)
	if actionsContainer.Width != expectedActionsWidth {
		t.Fatalf("actions width = %v, want reserved %v", actionsContainer.Width, expectedActionsWidth)
	}
	if label.Child.(woxwidget.Container).Width != 0 {
		t.Fatal("storage label should use the remaining field width")
	}
}
