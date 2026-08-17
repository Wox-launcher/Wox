package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestFormAppPickerMatchesFlutterDialogAndDefersCommit(t *testing.T) {
	confirmed := -2
	props := FormAppPickerProps{
		OverlayWidth: 1000, OverlayHeight: 800, Title: "Select Apps to Ignore", SearchPlaceholder: "Search apps",
		CancelLabel: "Cancel", ConfirmLabel: "OK", CancelWidth: 80, ConfirmWidth: 70,
		Candidates: []FormAppCandidate{
			{Name: "Finder", Identity: "com.apple.finder", Detail: "/System/Finder.app"},
			{Name: "Safari", Identity: "com.apple.Safari", Detail: "/Applications/Safari.app"},
		},
		Theme: woxcomponent.Theme{}, OnConfirm: func(index int) { confirmed = index },
	}
	for index := 0; index < 10; index++ {
		props.Candidates = append(props.Candidates, FormAppCandidate{Name: "Utility", Identity: string(rune('a' + index))})
	}
	state := &formAppPickerState{}
	state.InitState(woxwidget.StateContext{}, props)
	visible := filteredFormAppCandidates(props.Candidates, "safari.app")
	if len(visible) != 1 || visible[0].originalIndex != 1 {
		t.Fatalf("filtered apps = %#v, want Safari by path", visible)
	}

	dialog := buildFormAppPickerDialog(woxwidget.StateContext{}, props, state, filteredFormAppCandidates(props.Candidates, "")).(woxwidget.Stateful)
	dialogProps := dialog.Widget.(woxcomponent.DialogProps)
	if dialogProps.Width != 808 || dialogProps.Height != 640 || dialogProps.Radius != 20 || dialogProps.InitialFocus != "form-table-app-search" {
		t.Fatalf("dialog geometry = %.0fx%.0f radius %.0f focus %q", dialogProps.Width, dialogProps.Height, dialogProps.Radius, dialogProps.InitialFocus)
	}
	children := dialogProps.Child.(woxwidget.Flex).Children
	search := children[1].(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps)
	if search.Hint != "Search apps" || !search.Autofocus {
		t.Fatalf("search field = hint %q autofocus %v", search.Hint, search.Autofocus)
	}
	footer := children[len(children)-1].(woxwidget.Container)
	if footer.Height != SettingsDialogActionsHeight || footer.Padding.Top != SettingsDialogActionsHeight-settingsDialogActionHeight {
		t.Fatalf("app picker footer = height %v padding %+v, want shared action height plus top spacing", footer.Height, footer.Padding)
	}
	list := children[3].(woxwidget.Stack)
	scroll := list.Children[0].Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	first := scroll.Content.(woxwidget.Flex).Children[0].(woxwidget.Semantics)
	if first.Role != woxui.AccessibilityRoleRadioButton || first.Checked {
		t.Fatal("empty initial selection should render unchecked radio rows")
	}
	row := first.Child.(woxwidget.Gesture).Child.(woxwidget.Container).Child.(woxwidget.Flex)
	labelSlot := row.Children[len(row.Children)-1].(woxwidget.Align)
	if labelSlot.Height != formAppPickerRowHeight || labelSlot.Vertical != 0.5 {
		t.Fatalf("app label alignment = %#v, want a full-height centered slot", labelSlot)
	}
	labels := labelSlot.Child.(woxwidget.Flex).Children
	name := labels[0].(woxwidget.TextBlock)
	detail := labels[1].(woxwidget.TextBlock)
	if name.Value != "Finder" || detail.Value != "/System/Finder.app" || name.Height < name.LineHeight || detail.Height < detail.LineHeight {
		t.Fatalf("app labels = %q %.0f/%.0f and %q %.0f/%.0f", name.Value, name.Height, name.LineHeight, detail.Value, detail.Height, detail.LineHeight)
	}
	if confirmed != -2 {
		t.Fatal("rendering or selecting a row must not commit before confirmation")
	}
	state.selectedIdentity = normalizedFormAppIdentity("com.apple.Safari")
	state.confirm(props)
	if confirmed != 1 {
		t.Fatalf("confirmed index = %d, want Safari", confirmed)
	}
}
