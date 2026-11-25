package view

import (
	"fmt"
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
		Theme: woxcomponent.ControlTheme{}, OnConfirm: func(index int) { confirmed = index },
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
	list := children[3].(woxwidget.Expanded).Child.(woxwidget.LayoutBuilder).Build(woxui.Size{Width: 760, Height: 460}).(woxwidget.Stack)
	scroll := list.Children[0].Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	first := scroll.Content.(woxwidget.Flex).Children[0].(woxwidget.Semantics)
	if first.Role != woxui.AccessibilityRoleRadioButton || first.Checked {
		t.Fatal("empty initial selection should render unchecked radio rows")
	}
	row := first.Child.(woxwidget.Gesture).Child.(woxwidget.Container).Child.(woxwidget.Flex)
	if row.CrossAxisAlignment != woxwidget.CrossAxisCenter {
		t.Fatal("app row contents should be vertically centered")
	}
	labels := row.Children[len(row.Children)-1].(woxwidget.Expanded).Child.(woxwidget.Container).Child.(woxwidget.Flex).Children
	name := labels[0].(woxwidget.TextBlock)
	detail := labels[1].(woxwidget.TextBlock)
	if name.Value != "Finder" || detail.Value != "/System/Finder.app" || name.Height != 0 || detail.Height != 0 {
		t.Fatalf("app labels = %+v and %+v, want natural text height", name, detail)
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

func TestFormAppPickerRowGrowsWithMeasuredLabels(t *testing.T) {
	props := FormAppPickerProps{Theme: woxcomponent.ControlTheme{DensityScale: 1.2}}
	item := visibleFormAppCandidate{candidate: FormAppCandidate{Name: "Finder", Detail: "/System/Finder.app"}}
	row := formAppPickerRow(woxwidget.StateContext{}, props, &formAppPickerState{}, item, 0, 500)
	if height := woxwidget.MeasureStateless(catalogTallFontServices{}, row, 500).Height; height <= formAppPickerMinRowHeight {
		t.Fatalf("app row height = %g, want the row to grow for larger font metrics", height)
	}
}

func TestFormAppPickerKeyboardRevealsMeasuredRow(t *testing.T) {
	confirmed := -1
	props := FormAppPickerProps{OverlayWidth: 800, OverlayHeight: 500, Title: "Select app", SearchPlaceholder: "Search apps",
		CancelLabel: "Cancel", ConfirmLabel: "OK", Theme: woxcomponent.ControlTheme{DensityScale: 1.2},
		OnConfirm: func(index int) { confirmed = index }}
	for index := range 20 {
		name := fmt.Sprintf("Application %d", index)
		props.Candidates = append(props.Candidates, FormAppCandidate{Name: name, Detail: name + " details", Identity: name})
	}
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget { return FormAppPickerView(props) })
	defer host.Dispose()
	host.AttachServices(catalogTallFontServices{})
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 800, Height: 500}, PixelSize: woxui.PixelSize{Width: 2000, Height: 1250}, Scale: 2.5}
	host.Frame(&woxui.DisplayList{}, frame)
	if !host.Key(woxui.KeyEvent{Key: woxui.KeyArrowUp, Down: true}) {
		t.Fatal("app search did not handle keyboard selection")
	}
	// The first layout measures the row, and the next frame applies the scroll offset.
	host.Frame(&woxui.DisplayList{}, frame)
	host.Frame(&woxui.DisplayList{}, frame)
	row, found := host.BoundsForKey("form-table-app-19")
	footer, footerFound := host.BoundsForKey("form-table-app-confirm")
	if !found || !footerFound || row.Y < 0 || row.Y+row.Height > footer.Y {
		t.Fatalf("selected row %+v should scroll into view above the footer %+v", row, footer)
	}
	if !host.Key(woxui.KeyEvent{Key: woxui.KeyEnter, Down: true}) || confirmed != 19 {
		t.Fatalf("confirmed app = %d, want the selected last app", confirmed)
	}
}
