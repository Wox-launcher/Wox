package view

import (
	"fmt"
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/emojisearch"
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
	if columns := formTableEmojiColumns(612); columns < 6 || columns > 10 {
		t.Fatalf("columns for 612px = %d, want clamped 6..10", columns)
	}
	if columns := formTableEmojiColumns(40); columns != 6 {
		t.Fatalf("columns for 40px = %d, want minimum 6", columns)
	}
	if columns := formTableEmojiColumns(5000); columns != 10 {
		t.Fatalf("columns for 5000px = %d, want maximum 10", columns)
	}
}

func TestFormTableEmojiPickerRendersDialogAndChooses(t *testing.T) {
	chosen := ""
	props := FormTableEmojiPickerProps{
		OverlayWidth: 900, OverlayHeight: 760, Title: "Select Emoji", SearchLabel: "Search emoji", SearchResultsLabel: "Search Results", NoResultsLabel: "No matching emoji", CloseLabel: "Close",
		Groups: []FormTableEmojiGroup{
			{Label: "Recommended", Marker: "👍", Emojis: []string{"🤖", "💡", "🔍", "📊", "📈", "📝", "🛠", "⚙️", "🧠", "✅", "🚀", "🎯"}},
			{Label: "Smileys", Marker: "😊", Emojis: []string{"😀", "😃", "😄", "😁", "😆", "😅", "😂", "🤣", "😊", "🙂", "😉", "😍"}},
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
	if dialogProps.Width != formTableEmojiPanelWidth || dialogProps.Height != formTableEmojiPanelHeight || dialogProps.InitialFocus != "form-table-emoji-search" || dialogProps.OnEscape == nil {
		t.Fatalf("dialog geometry = %.0fx%.0f focus %q", dialogProps.Width, dialogProps.Height, dialogProps.InitialFocus)
	}
	if chosen != "" {
		t.Fatal("rendering must not choose an emoji")
	}
	state.gridActive = true
	state.handleKey(woxwidget.StateContext{}, props, woxui.KeyEvent{Key: woxui.KeyEnter, Down: true})
	if chosen != "😃" {
		t.Fatalf("Enter chose %q, want 😃", chosen)
	}
}

func TestFormTableEmojiSearchFiltersByCategoryAndGlyph(t *testing.T) {
	props := FormTableEmojiPickerProps{
		SearchResultsLabel: "Search Results",
		Groups: []FormTableEmojiGroup{
			{Label: "Recommended", Emojis: []string{"🤖", "💡"}},
			{Label: "Smileys", Emojis: []string{"😀", "🤖", "😃"}},
		},
	}
	state := &formTableEmojiPickerState{}
	state.InitState(woxwidget.StateContext{}, props)
	state.query.SetText("smile", false)
	label, emojis := state.visibleEmojis(props)
	if label != "Search Results" || len(emojis) != 3 {
		t.Fatalf("category search = %q %v, want Search Results with 3 emoji", label, emojis)
	}
	state.query.SetText("🤖", false)
	_, emojis = state.visibleEmojis(props)
	if len(emojis) != 1 || emojis[0] != "🤖" {
		t.Fatalf("glyph search = %v, want one deduplicated robot", emojis)
	}
}

func TestFormTableEmojiSearchUsesMultilingualCatalogAndLimit(t *testing.T) {
	entries := []emojisearch.Entry{
		{Emoji: "🤖", SearchTerms: []string{"robot", "机器人", "smileys & emotion", "表情与情感"}},
		{Emoji: "🚀", SearchTerms: []string{"rocket", "火箭", "travel & places", "旅行与地点"}},
	}
	for index := 0; index < formTableEmojiSearchLimit; index++ {
		entries = append(entries, emojisearch.Entry{Emoji: fmt.Sprintf("result-%d", index), SearchTerms: []string{"common"}})
	}
	props := FormTableEmojiPickerProps{
		SearchResultsLabel: "Search Results",
		Groups:             []FormTableEmojiGroup{{Label: "Recommended", Emojis: []string{"🤖"}}},
		SearchEntries:      entries,
	}
	state := &formTableEmojiPickerState{}
	state.InitState(woxwidget.StateContext{}, props)
	for _, query := range []string{"robot", "机器人", "emotion", "情感"} {
		state.query.SetText(query, false)
		_, emojis := state.visibleEmojis(props)
		if len(emojis) != 1 || emojis[0] != "🤖" {
			t.Fatalf("query %q results = %v, want robot", query, emojis)
		}
	}
	state.query.SetText("common", false)
	_, emojis := state.visibleEmojis(props)
	if len(emojis) != formTableEmojiSearchLimit {
		t.Fatalf("search result count = %d, want limit %d", len(emojis), formTableEmojiSearchLimit)
	}
}
