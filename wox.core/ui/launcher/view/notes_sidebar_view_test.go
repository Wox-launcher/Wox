package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestNotesSidebarSearchFieldGeometryAndBackground(t *testing.T) {
	for _, density := range []float32{.9, 1, 1.1} {
		for _, text := range []woxui.Color{{R: 240, G: 240, B: 240, A: 255}, {R: 20, G: 20, B: 20, A: 255}} {
			theme := woxcomponent.Theme{Controls: woxcomponent.ControlTheme{DensityScale: density, BodyText: text}}
			panel := NotesSidebar(NotesSidebarProps{Width: theme.Controls.Scaled(240), Height: 400, Theme: theme}).(woxwidget.Container)
			field := panel.Child.(woxwidget.Flex).Children[1].(woxwidget.Stack).Children[0].Child.(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps)
			if field.Padding.Top != theme.Controls.Scaled(4) || field.Padding.Bottom != theme.Controls.Scaled(4) || field.LineHeight != field.Height-field.Padding.Top-field.Padding.Bottom || field.Height != theme.Controls.Scaled(32) {
				t.Fatalf("density %v: search line cannot fit its field: %+v", density, field)
			}
			if field.Style.Size != woxcomponent.SettingsControlFontSize || field.TextAlignmentY != .5 {
				t.Fatalf("density %v: search typography = %+v, alignment %v", density, field.Style, field.TextAlignmentY)
			}
			if field.Background.A == 0 || field.Background.R != text.R || field.Background.G != text.G || field.Background.B != text.B {
				t.Fatalf("density %v: search has no theme-adaptive resting background: %+v", density, field.Background)
			}
		}
	}
}

func TestNotesSidebarHeaderUsesIconActionsWithoutTitle(t *testing.T) {
	for _, expanded := range []bool{false, true} {
		p := NotesSidebarProps{Width: 240, Height: 400, Title: "Notes", RootLabel: "Select root", NewLabel: "New", GroupLabel: "Group", ToggleAllLabel: "Toggle all", HasGroups: true, AnyExpanded: expanded, OnRootSelect: func() {}}
		panel := NotesSidebar(p).(woxwidget.Container)
		header := panel.Child.(woxwidget.Flex).Children[0].(woxwidget.Flex)
		if len(header.Children) != 4 {
			t.Fatalf("unexpected header layout: %+v", header.Children)
		}
		spacer := header.Children[0].(woxwidget.Expanded).Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
		if spacer.Icon != nil || spacer.Label != "Select root" {
			t.Fatal("root selection should not draw a Notes title")
		}
		newButton := header.Children[2].(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
		allButton := header.Children[3].(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
		if newButton.ID != "notes.sidebar.new" || allButton.ID != "notes.sidebar.toggle-all" || allButton.Disabled {
			t.Fatal("bulk toggle should follow the new note action")
		}
		want := woxcomponent.ExpandAllGlyph(16, p.Theme.Controls.TextSecondary, expanded).(woxwidget.Image).Source
		if allButton.Icon.(woxwidget.Image).Source != want {
			t.Fatal("bulk toggle icon does not match its action")
		}
	}
	panel := NotesSidebar(NotesSidebarProps{Width: 240, Height: 400}).(woxwidget.Container)
	header := panel.Child.(woxwidget.Flex).Children[0].(woxwidget.Flex)
	if !header.Children[3].(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps).Disabled {
		t.Fatal("bulk toggle should be disabled without groups")
	}
}

func TestNotesSidebarRowsDoNotReserveDragHandleSpace(t *testing.T) {
	for _, density := range []float32{.9, 1, 1.1} {
		theme := woxcomponent.Theme{Controls: woxcomponent.ControlTheme{DensityScale: density}}
		for _, group := range []bool{false, true} {
			row := NotesSidebarRow{ID: "item", Title: "Item", Group: group, Depth: 1}
			retained := notesSidebarTreeRow(NotesSidebarProps{Theme: theme}, row, 0, 220).(woxwidget.Stateful)
			tree := retained.CreateState().Build(woxwidget.StateContext{}, retained.Widget).(woxwidget.Gesture).Child.(woxwidget.Stack)
			if len(tree.Children) != 5 {
				t.Fatalf("density %v: unexpected drag handle layer", density)
			}
			button := tree.Children[0].Child.(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Stateful)
			container := button.CreateState().Build(woxwidget.StateContext{}, button.Widget).(woxwidget.Gesture).Child.(woxwidget.Container)
			want := theme.Controls.Scaled(16) + theme.Controls.Scaled(8)
			if group {
				want += theme.Controls.Scaled(16)
			}
			if container.Padding.Left != want {
				t.Fatalf("density %v group %v: left padding %v, want %v", density, group, container.Padding.Left, want)
			}
		}
	}
}

func TestNotesDeleteDialogUsesConfirmationLayout(t *testing.T) {
	for _, density := range []float32{.9, 1, 1.1} {
		for _, width := range []float32{280, 700} {
			theme := woxcomponent.ControlTheme{DensityScale: density}
			dialog := NotesDeleteDialog(NotesDeleteDialogProps{Size: woxui.Size{Width: width, Height: 600}, Theme: theme, Title: "Delete", Name: "Note", Message: "Delete this note?", Error: "Failure", CancelLabel: "Cancel", DeleteLabel: "Delete", OnCancel: func() {}}).(woxwidget.Stateful).Widget.(woxcomponent.DialogProps)
			if dialog.InitialFocus != "notes.organize.cancel" || dialog.OnBackdrop == nil || dialog.OnEscape == nil || dialog.Label != "Delete this note?" {
				t.Fatalf("density %v width %v: dialog focus, dismissal or label incorrect", density, width)
			}
			body := dialog.Child.(woxwidget.Flex)
			actions := body.Children[len(body.Children)-1].(woxwidget.Align).Child.(woxwidget.Flex)
			cancel := actions.Children[0].(woxwidget.Semantics)
			confirm := actions.Children[1].(woxwidget.Semantics)
			if cancel.AutomationID != "notes.organize.cancel" || confirm.AutomationID != "notes.organize.confirm" || cancel.Label != "Cancel" || confirm.Label != "Delete" {
				t.Fatalf("density %v width %v: incorrect confirmation actions", density, width)
			}
			if (actions.Axis == woxwidget.Vertical) != (width-theme.Scaled(56) < theme.Scaled(300)) {
				t.Fatalf("density %v width %v: incorrect action layout", density, width)
			}
		}
	}
}
