package launcher

import (
	"context"
	"testing"
	"wox/common"
	"wox/plugin"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Suggestion acceptance and display share the same caret and prefix conditions.
func TestQueryHintSuggestionMatching(t *testing.T) {
	for _, tc := range []struct{ value, suffix string }{
		{"", ""}, {"s", "earch"}, {"se", "arch"}, {"search", ""}, {"SEARCH", ""}, {"CR", "eated"}, {"unknown", ""}, {"搜", "索"},
	} {
		hint := &common.QueryHint{Elements: []common.QueryElement{{Id: "filter", Kind: "argument", Value: tc.value, Suggestions: []string{"search", "subscribed", "created", "搜索", "search longer"}}}}
		state := queryHintCaretAt(hint, 0)
		_, suffix := queryHintSuggestion(hint, 0, state, true)
		if suffix != tc.suffix {
			t.Fatalf("%q: got %q, want %q", tc.value, suffix, tc.suffix)
		}
		if _, suffix := queryHintSuggestion(hint, 0, state, false); suffix != "" {
			t.Fatal("unfocused completion")
		}
		state.Composition = "x"
		if _, suffix := queryHintSuggestion(hint, 0, state, true); suffix != "" {
			t.Fatal("IME completion")
		}
		state.Composition = ""
		state.Selection.Anchor = 0
		if tc.value != "" {
			if _, suffix := queryHintSuggestion(hint, 0, state, true); suffix != "" {
				t.Fatal("selected completion")
			}
		}
		state.Selection = woxui.TextSelection{}
		if _, suffix := queryHintSuggestion(hint, 0, state, true); suffix != "" {
			t.Fatal("completion before argument end")
		}
	}
}

// Exercise automatic command discovery, nested templates, acceptance and history through the editor.
func TestQueryHintSuggestionEditing(t *testing.T) {
	instance := &plugin.Instance{Metadata: plugin.Metadata{TriggerKeywords: []string{"gh"}, Commands: []plugin.MetadataCommand{
		{Command: "issues", QueryHint: &common.QueryHint{Elements: []common.QueryElement{
			{Id: "filter", Kind: "argument", Suggestions: []string{"created", "assigned", "search"}},
			{Id: "separator", Kind: "text", Text: " "}, {Id: "repo", Kind: "argument", Placeholder: "Repository"},
		}}},
	}}}
	a := &App{editor: woxui.NewTextEditor(""), query: newInputQuery(""), lifecycleCtx: context.Background(), services: &queryHintRegressionServices{instance: instance}}
	a.host = woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return woxwidget.EditableText{Key: launcherview.LauncherQueryInputKey, Autofocus: true, Child: woxwidget.Container{Width: 300, Height: 40}}
	})
	a.host.AttachServices(formTableHostServices{})
	t.Cleanup(a.host.Dispose)
	a.host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 300, Height: 40}, Scale: 1})
	typeText := func(text string) {
		a.editor.InsertText(text)
		a.query.QueryText = a.updateQueryHintText(a.editor.State().Text)
	}
	tab := func() { a.onQueryHintKey(woxui.KeyEvent{Key: woxui.KeyTab, Down: true}) }
	typeText("gh ")
	typeText("is")
	tab()
	if a.query.QueryText != "gh issues " || a.editor.State().Selection.Focus != 10 {
		t.Fatalf("command completion: %+v", a.editor.State())
	}
	if a.query.QueryText != "gh issues " || a.query.QueryHint.Elements[1].Id != "filter" {
		t.Fatalf("nested template: %+v", a.query)
	}
	a.onQueryHintKey(woxui.KeyEvent{Key: woxui.Key("z"), Modifiers: queryPrimaryModifier(), Down: true})
	if a.query.QueryText != "gh is" || !a.query.QueryHint.CommandSuggestions {
		t.Fatalf("command and space were not undone together: %+v", a.query)
	}
	tab()
	typeText("cr")
	tab()
	if a.query.QueryText != "gh issues created" || a.editor.State().Selection.Focus != 17 {
		t.Fatalf("argument completion: %+v", a.editor.State())
	}
	a.onQueryHintKey(woxui.KeyEvent{Key: woxui.Key("z"), Modifiers: queryPrimaryModifier(), Down: true})
	if a.query.QueryText != "gh issues cr" {
		t.Fatalf("undo: %q", a.query.QueryText)
	}
	a.onQueryHintKey(woxui.KeyEvent{Key: woxui.Key("y"), Modifiers: queryPrimaryModifier(), Down: true})
	if a.query.QueryText != "gh issues created" {
		t.Fatalf("redo: %q", a.query.QueryText)
	}
	tab()
	typeText("owner/repo")
	if a.query.QueryHint.Argument("repo") != "owner/repo" {
		t.Fatal("second Tab failed to navigate")
	}
	// Complete an earlier argument without losing the following value.
	a.editor.SetSelection(10, 17)
	typeText("as")
	tab()
	if a.query.QueryText != "gh issues assigned owner/repo" || a.editor.State().Selection.Focus != 18 {
		t.Fatalf("middle completion: %+v", a.editor.State())
	}
	a.editor.SetSelection(10, 18)
	typeText("custom")
	if a.query.QueryHint.Argument("filter") != "custom" {
		t.Fatal("free input was constrained")
	}
	instance.Metadata.Commands[0].QueryHint = nil
	generated, _ := plugin.MatchQueryHint("gh ", []*plugin.Instance{instance})
	a.installQueryHintTemplate(generated)
	tab()
	if a.query.QueryText != "gh issues " || a.editor.State().Selection.Focus != 10 {
		t.Fatalf("command without parameters did not get exactly one space: %+v", a.editor.State())
	}
}

// A sole command is immediately actionable, while ordinary empty arguments remain placeholders.
func TestQueryHintSoleCommandCompletion(t *testing.T) {
	for _, tc := range []struct {
		command    bool
		choices    []string
		completion bool
	}{
		{true, []string{"new"}, true},
		{true, []string{"new", "open"}, false},
		{false, []string{"new"}, false},
	} {
		hint := &common.QueryHint{CommandSuggestions: tc.command, Elements: []common.QueryElement{
			{Id: "command", Kind: "text", Text: "screenshot "},
			{Id: "operation", Kind: "argument", Suggestions: tc.choices},
		}}
		state := queryHintCaretAt(hint, 1)
		props := queryHintViewProps(&App{}, viewSnapshot{hint: hint, editing: state, queryFocused: true, queryHintActive: 1})
		if props.TabHint.Visible != tc.completion || props.CompletionInsertion.Visible != tc.completion {
			t.Fatalf("unexpected completion: %+v", props)
		}
		if tc.completion && (props.CompletionSuffix != "new" || len(props.CompletionChips) != 0) {
			t.Fatal("sole command must use plain ghost text")
		}
		if state.Text != "screenshot " || hint.PlainText() != state.Text {
			t.Fatal("preview inserted command text")
		}
		for _, focused := range []bool{true, false} {
			state.Composition = "n"
			if _, suffix := queryHintSuggestion(hint, 1, state, focused); suffix != "" {
				t.Fatal("completion during composition")
			}
		}
	}
}

// Preview selection favors short commands without reordering or removing completion candidates.
func TestQueryHintSuggestionsLabel(t *testing.T) {
	choices := []string{"uninstall", "create", "dev.list", "install", "up"}
	measure := func(text string) float32 { return float32(len([]rune(text))) }
	for _, tc := range []struct {
		width float32
		want  string
	}{
		{100, "up / create / install / …"}, {20, "up / create / …"}, {8, "up / …"}, {3, "…"}, {0, ""},
	} {
		if got := queryHintSuggestionsLabel(choices, true, tc.width, measure); got != tc.want {
			t.Fatalf("width %v: %q, want %q", tc.width, got, tc.want)
		}
	}
	if choices[0] != "uninstall" || len(choices) != 5 {
		t.Fatal("preview mutated candidates")
	}
	if got := queryHintSuggestionsLabel(choices, false, 100, measure); got != "uninstall / create / dev.list / …" {
		t.Fatal(got)
	}
	if got := queryHintSuggestionsLabel([]string{"中", "a", "bb"}, true, 100, measure); got != "中 / a / bb" {
		t.Fatal("equal lengths must preserve declaration order")
	}
}

// Ghost chips become plain completion suffixes, including the sole/last argument.
func TestQueryHintSuggestionView(t *testing.T) {
	a := &App{}
	hint := &common.QueryHint{Elements: []common.QueryElement{{Id: "command", Kind: "text", Text: "gh issues "}, {Id: "filter", Kind: "argument", Suggestions: []string{"created", "assigned", "search"}}}}
	props := queryHintViewProps(a, viewSnapshot{hint: hint, editing: queryHintCaretAt(hint, 1), queryFocused: true, queryHintActive: 1})
	if props.CompletionSuffix != "created / assigned / search" || len(props.CompletionChips) != 1 || props.TabHint.Visible {
		t.Fatalf("empty suggestions: %+v", props)
	}
	hint.Elements[1].Value = "cr"
	props = queryHintViewProps(a, viewSnapshot{hint: hint, editing: queryHintCaretAt(hint, 1), queryFocused: true, queryHintActive: 1})
	if props.CompletionSuffix != "eated" || len(props.CompletionChips) != 0 || !props.TabHint.Visible || !props.CompletionInsertion.Visible {
		t.Fatalf("partial suggestions: %+v", props)
	}
	hint.Elements[1].Value = "created"
	props = queryHintViewProps(a, viewSnapshot{hint: hint, editing: queryHintCaretAt(hint, 1), queryFocused: true, queryHintActive: 1})
	if props.CompletionSuffix != "" || props.TabHint.Visible {
		t.Fatal("exact match still completes")
	}
}
