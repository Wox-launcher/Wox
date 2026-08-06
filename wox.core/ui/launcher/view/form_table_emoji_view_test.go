package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestFormTableEmojiInitialSelection(t *testing.T) {
	groups := []FormTableEmojiGroup{
		{Label: "Recommended", Emojis: []string{"🤖", "💡"}},
		{Label: "Smileys", Emojis: []string{"😀", "😃"}},
	}
	if group, index := formTableEmojiInitialSelection(groups, "😀"); group != 1 || index != 0 {
		t.Fatalf("initial selection for 😀 = group %d index %d, want 1/0", group, index)
	}
	if group, index := formTableEmojiInitialSelection(groups, "🤖"); group != 0 || index != 0 {
		t.Fatalf("initial selection for 🤖 = group %d index %d, want 0/0", group, index)
	}
	if group, index := formTableEmojiInitialSelection(groups, ""); group != 0 || index != 0 {
		t.Fatalf("empty initial selection = group %d index %d, want 0/0", group, index)
	}
	if group, index := formTableEmojiInitialSelection(groups, `{"ImageType":"emoji"}`); group != 0 || index != 0 {
		t.Fatalf("structured image initial selection = group %d index %d, want 0/0", group, index)
	}
}

func TestFormTableEmojiColumnsClamped(t *testing.T) {
	if columns := formTableEmojiColumns(612); columns < 6 || columns > 12 {
		t.Fatalf("columns for 612px = %d, want clamped 6..12", columns)
	}
	if columns := formTableEmojiColumns(40); columns != 6 {
		t.Fatalf("columns for 40px = %d, want minimum 6", columns)
	}
	if columns := formTableEmojiColumns(5000); columns != 12 {
		t.Fatalf("columns for 5000px = %d, want maximum 12", columns)
	}
}

func TestFormTableEmojiGridHeightKeepsActionsInsidePanel(t *testing.T) {
	panel := float32(500)
	for _, tab := range []float32{formTableEmojiTabHeight, formTableEmojiTabHeight*2 + 6, formTableEmojiTabHeight*3 + 12} {
		grid := formTableEmojiGridHeight(panel, tab)
		total := formTableEmojiTitleHeight + tab + grid + formTableEmojiActionsHeight + 30
		if total > panel-48 {
			t.Fatalf("tab height %.0f: body %.0f exceeds padded panel %.0f", tab, total, panel-48)
		}
	}
}

func TestFormTableEmojiPickerRendersDialogAndChooses(t *testing.T) {
	chosen := ""
	props := FormTableEmojiPickerProps{
		OverlayWidth: 900, OverlayHeight: 760, Title: "Select Emoji", CancelLabel: "Cancel", CancelWidth: 80,
		Groups: []FormTableEmojiGroup{
			{Label: "Recommended", Emojis: []string{"🤖", "💡", "🔍", "📊", "📈", "📝", "🛠", "⚙️", "🧠", "✅", "🚀", "🎯"}},
			{Label: "Smileys", Emojis: []string{"😀", "😃", "😄", "😁", "😆", "😅", "😂", "🤣", "😊", "🙂", "😉", "😍"}},
		},
		Theme: woxcomponent.Theme{}, InitialEmoji: "😃", OnChoose: func(emoji string) { chosen = emoji }, OnCancel: func() {},
	}
	state := &formTableEmojiPickerState{}
	state.InitState(woxwidget.StateContext{}, props)
	if state.group != 1 || state.selected != 1 {
		t.Fatalf("initial state = group %d selected %d, want 1/1", state.group, state.selected)
	}

	dialog := state.buildDialog(woxwidget.StateContext{}, props).(woxwidget.Stateful)
	dialogProps := dialog.Widget.(woxcomponent.DialogProps)
	if dialogProps.Width != 660 || dialogProps.Height != 500 || dialogProps.InitialFocus != "form-table-emoji-focus" || dialogProps.OnEscape == nil {
		t.Fatalf("dialog geometry = %.0fx%.0f focus %q", dialogProps.Width, dialogProps.Height, dialogProps.InitialFocus)
	}
	if chosen != "" {
		t.Fatal("rendering must not choose an emoji")
	}
	state.handleKey(woxwidget.StateContext{}, props, woxui.KeyEvent{Key: woxui.KeyEnter, Down: true})
	if chosen != "😃" {
		t.Fatalf("Enter chose %q, want 😃", chosen)
	}
}
