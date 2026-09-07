package launcher

import (
	"context"
	"runtime"
	"testing"
	"wox/common"
	"wox/ui/contract"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type queryHintResolverTestServices struct {
	contract.Services
	hint *common.QueryHint
}

func (s queryHintResolverTestServices) ResolveQueryHint(_ context.Context, text string) *common.QueryHint {
	if text == "g " {
		return s.hint.Clone()
	}
	return nil
}

// Word deletion crossing a separator must restore hints when only the context prefix remains.
func TestQueryHintWordDeletionRestoresTemplate(t *testing.T) {
	template := &common.QueryHint{Elements: []common.QueryElement{
		{Id: "command", Kind: "text", Text: "g "}, {Id: "query", Kind: "argument", Placeholder: "query"},
		{Id: "separator", Kind: "text", Text: " "}, {Id: "time", Kind: "argument", Placeholder: "time"},
	}}
	a := &App{editor: woxui.NewTextEditor(""), query: newInputQuery("g "), services: queryHintResolverTestServices{hint: template}}
	filled := template.Clone()
	filled.Elements[1].Value, filled.Elements[3].Value = "sdf", "sdlkfj"
	a.installQueryHint(filled)
	a.editor.SetCaret(len([]rune(filled.PlainText())))
	modifier := woxui.KeyModifierControl
	if runtime.GOOS == "darwin" {
		modifier = woxui.KeyModifierAlt
	}
	for i := 0; i < 2; i++ {
		a.editor.HandleKey(woxui.KeyEvent{Key: woxui.KeyBackspace, Modifiers: modifier, Down: true})
		a.query.QueryText = a.updateQueryHintText(a.editor.State().Text)
	}
	if a.query.QueryText != "g " || a.query.QueryHint == nil || a.query.QueryHint.PlainText() != "g " {
		t.Fatalf("word deletion lost empty template: text=%q hint=%+v", a.query.QueryText, a.query.QueryHint)
	}
}

// Empty arguments are chips; only atomic blocks receive a mark by themselves.
func TestQueryHintBackgroundOnlyForBlocks(t *testing.T) {
	for _, tc := range []struct {
		kind, value, placeholder string
		marks, chips             int
	}{
		{common.QueryElementArgument, "", "Volume (0–100)", 0, 1},
		{common.QueryElementArgument, "23", "", 0, 0},
		{common.QueryElementBlock, "23", "", 1, 0},
	} {
		a := &App{}
		hint := &common.QueryHint{Elements: []common.QueryElement{
			{Kind: common.QueryElementText, Text: "set volume "},
			{Kind: tc.kind, Value: tc.value, Placeholder: common.I18nString(tc.placeholder)},
		}}
		widget := a.queryHintView(viewSnapshot{hint: hint, editing: woxui.TextEditingState{Text: hint.PlainText()}}, 200, 40, 34)
		scroll := widget.(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.ScrollView)
		props := scroll.Child.(woxwidget.Boundary[launcherview.LauncherQueryProps]).Props
		if len(props.Marks) != tc.marks || props.CompletionSuffix != tc.placeholder || len(props.CompletionChips) != tc.chips {
			t.Fatalf("kind %s value %q: marks=%d placeholder=%q chips=%d", tc.kind, tc.value, len(props.Marks), props.CompletionSuffix, len(props.CompletionChips))
		}
		if tc.chips == 1 && props.CompletionChips[0].Text != tc.placeholder {
			t.Fatalf("empty argument chip = %#v, want a hole for %q", props.CompletionChips, tc.placeholder)
		}
	}
}

// Deferred separators remain visible in ghost text without extending the editable document.
func TestQueryHintDeferredSeparatorSpacing(t *testing.T) {
	for _, tc := range []struct {
		value, separator, suffix string
	}{
		{"", "", "query time"},
		{"se", "", " time"},
		{"se", " ", "time"},
	} {
		a := &App{}
		hint := &common.QueryHint{Elements: []common.QueryElement{
			{Id: "command", Kind: "text", Text: "g "},
			{Id: "query", Kind: "argument", Value: tc.value, Placeholder: "query"},
			{Id: "separator", Kind: "text", Text: tc.separator},
			{Id: "time", Kind: "argument", Placeholder: "time"},
		}}
		text := hint.PlainText()
		widget := a.queryHintView(viewSnapshot{hint: hint, editing: woxui.TextEditingState{Text: text}}, 200, 40, 34)
		scroll := widget.(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.ScrollView)
		props := scroll.Child.(woxwidget.Boundary[launcherview.LauncherQueryProps]).Props
		if props.CompletionSuffix != tc.suffix || props.State.Text != text || hint.PlainText() != text {
			t.Fatalf("value=%q separator=%q: suffix=%q text=%q", tc.value, tc.separator, props.CompletionSuffix, props.State.Text)
		}
		if tc.value == "" {
			if len(props.CompletionChips) != 2 || props.CompletionChips[0].Text != "query" || props.CompletionChips[1].Text != "time" {
				t.Fatalf("empty placeholders = %#v, want separate query and time chips", props.CompletionChips)
			}
		} else if len(props.CompletionChips) != 1 || props.CompletionChips[0].Text != "time" {
			t.Fatalf("remaining placeholder = %#v, want a time chip", props.CompletionChips)
		}
	}
}

// Multiple arguments need their own surfaces so values or names that contain spaces stay distinct.
func TestQueryHintMultipleArgumentsUseDecoration(t *testing.T) {
	a := &App{}
	hint := &common.QueryHint{Elements: []common.QueryElement{
		{Id: "command", Kind: "text", Text: "g "},
		{Id: "query", Kind: "argument", Value: "hello world", Placeholder: "search query"},
		{Id: "separator", Kind: "text", Text: " "},
		{Id: "time", Kind: "argument", Placeholder: "time range"},
	}}
	widget := a.queryHintView(viewSnapshot{hint: hint, editing: woxui.TextEditingState{Text: hint.PlainText()}}, 200, 40, 34)
	scroll := widget.(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.ScrollView)
	props := scroll.Child.(woxwidget.Boundary[launcherview.LauncherQueryProps]).Props
	if len(props.Marks) != 1 || props.CompletionSuffix != "time range" || len(props.CompletionChips) != 1 || props.CompletionChips[0].Text != "time range" {
		t.Fatalf("mixed slots = marks=%d suffix=%q chips=%#v", len(props.Marks), props.CompletionSuffix, props.CompletionChips)
	}
}

func queryHintViewProps(a *App, snapshot viewSnapshot) launcherview.LauncherQueryProps {
	widget := a.queryHintView(snapshot, 200, 40, 34)
	if stack, ok := widget.(woxwidget.Stack); ok {
		widget = stack.Children[0].Child
	}
	scroll := widget.(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.ScrollView)
	return scroll.Child.(woxwidget.Boundary[launcherview.LauncherQueryProps]).Props
}

func queryHintCaretAt(hint *common.QueryHint, index int) woxui.TextEditingState {
	_, end := queryElementRange(hint, index)
	return woxui.TextEditingState{Text: hint.PlainText(), Selection: woxui.TextSelection{Anchor: end, Focus: end}}
}

func TestQueryHintForwardTabTarget(t *testing.T) {
	hint := &common.QueryHint{Elements: []common.QueryElement{
		{Id: "command", Kind: "text", Text: "g "},
		{Id: "query", Kind: "argument", Required: true},
		{Id: "separator", Kind: "text", Text: " "},
		{Id: "time", Kind: "argument", Required: true},
	}}
	if _, ok := queryHintForwardTabTarget(hint, 1); ok {
		t.Fatal("empty required argument must not advance")
	}
	hint.Elements[1].Value = "hello"
	if next, ok := queryHintForwardTabTarget(hint, 1); !ok || next != 3 {
		t.Fatalf("filled argument next = %d ok=%t, want 3", next, ok)
	}
	if _, ok := queryHintForwardTabTarget(hint, 3); ok {
		t.Fatal("last argument must not advance")
	}
}

// Tab is advertised only after the destination that a successful Tab would reach.
func TestQueryHintTabHintOnlyWhenTabSucceeds(t *testing.T) {
	a := &App{}
	single := &common.QueryHint{Elements: []common.QueryElement{
		{Id: "command", Kind: "text", Text: "set volume "},
		{Id: "volume", Kind: "argument", Placeholder: "Volume (0–100)"},
	}}
	props := queryHintViewProps(a, viewSnapshot{hint: single, queryHintActive: 1, queryFocused: true, editing: queryHintCaretAt(single, 1)})
	if props.TabHint.Visible || len(props.CompletionChips) != 1 || props.CompletionChips[0].Text != "Volume (0–100)" {
		t.Fatalf("single current argument = tab %#v chips %#v, want a hole and no Tab mark", props.TabHint, props.CompletionChips)
	}

	empty := &common.QueryHint{Elements: []common.QueryElement{
		{Id: "command", Kind: "text", Text: "g "},
		{Id: "query", Kind: "argument", Required: true, Placeholder: "query"},
		{Id: "separator", Kind: "text", Text: " "},
		{Id: "time", Kind: "argument", Required: true, Placeholder: "time"},
	}}
	props = queryHintViewProps(a, viewSnapshot{hint: empty, queryHintActive: 1, queryFocused: true, editing: queryHintCaretAt(empty, 1)})
	if props.TabHint.Visible {
		t.Fatal("empty required argument must not show a Tab mark")
	}

	filled := empty.Clone()
	filled.Elements[1].Value = "hello"
	props = queryHintViewProps(a, viewSnapshot{hint: filled, queryHintActive: 1, queryFocused: true, editing: queryHintCaretAt(filled, 1)})
	if !props.TabHint.Visible || props.TabHint.Label != "Tab" {
		t.Fatalf("next empty argument Tab hint = %#v", props.TabHint)
	}

	props = queryHintViewProps(a, viewSnapshot{hint: filled, queryHintActive: 3, queryFocused: true, editing: queryHintCaretAt(filled, 3)})
	if props.TabHint.Visible {
		t.Fatal("last argument must not show a Tab mark")
	}

	candidate := &common.QueryHint{Elements: []common.QueryElement{
		{Id: "command", Kind: "text", Text: "g"},
		{Id: "query", Kind: "argument", Placeholder: "query"},
		{Id: "separator", Kind: "text", Text: " "},
		{Id: "time", Kind: "argument", Placeholder: "time"},
	}}
	props = queryHintViewProps(a, viewSnapshot{hint: candidate, queryHintCandidate: true, queryFocused: true, editing: woxui.TextEditingState{Text: "g", Selection: woxui.TextSelection{Anchor: 1, Focus: 1}}})
	if props.CompletionSuffix == "" || !props.TabHint.Visible {
		t.Fatalf("candidate ghost Tab hint = suffix %q hint %#v", props.CompletionSuffix, props.TabHint)
	}
}

func TestQueryCompletionTabHintFollowsSuffix(t *testing.T) {
	a := &App{}
	props := a.queryViewProps(viewSnapshot{
		queryFocused:   true,
		editing:        woxui.TextEditingState{Text: "sett", Selection: woxui.TextSelection{Anchor: 4, Focus: 4}},
		completionHint: &queryCompletionHint{InputPrefix: "sett", CompletionText: "setting", Suffix: "ing"},
	}, 400, 40, 34)
	if props.CompletionSuffix != "ing" || !props.TabHint.Visible || props.TabHint.Label != "Tab" {
		t.Fatalf("completion Tab hint = suffix %q hint %#v", props.CompletionSuffix, props.TabHint)
	}
}

// Required arguments must be filled before Tab advances; optional arguments remain skippable.
func TestQueryHintTabRequiresCurrentValue(t *testing.T) {
	for _, tc := range []struct {
		value    string
		required bool
		advance  bool
	}{{"", true, false}, {" ", true, false}, {"hello", true, true}, {"", false, true}} {
		a := &App{editor: woxui.NewTextEditor("")}
		a.installQueryHint(&common.QueryHint{Elements: []common.QueryElement{
			{Id: "command", Kind: "text", Text: "g "},
			{Id: "query", Kind: "argument", Value: tc.value, Required: tc.required},
			{Id: "separator", Kind: "text", Text: " "}, {Id: "time", Kind: "argument", Required: true},
		}})
		before := a.editor.State()
		a.onQueryHintKey(woxui.KeyEvent{Key: woxui.KeyTab, Down: true})
		if (a.queryHintEditorState.active == 3) != tc.advance || (!tc.advance && a.editor.State() != before) || a.editor.State().Text != before.Text {
			t.Fatalf("unexpected Tab navigation for value %q required=%t", tc.value, tc.required)
		}
		if tc.advance {
			a.onQueryHintKey(woxui.KeyEvent{Key: woxui.KeyTab, Modifiers: woxui.KeyModifierShift, Down: true})
			if a.queryHintEditorState.active != 1 {
				t.Fatal("empty required argument blocked backward navigation")
			}
		} else if a.queryTabFeedback != 1 {
			t.Fatal("blocked Tab did not provide caret feedback")
		}
	}
}

// A placeholder is not completion text; exhausted navigation must preserve the edit.
func TestQueryHintTabStopsAtEnds(t *testing.T) {
	for _, value := range []string{"", "23"} {
		a := &App{editor: woxui.NewTextEditor("")}
		a.installQueryHint(&common.QueryHint{Elements: []common.QueryElement{
			{Kind: common.QueryElementText, Text: "set volume "},
			{Kind: common.QueryElementArgument, Value: value, Placeholder: "Volume (0–100)"},
		}})
		before := a.editor.State()
		for _, mods := range []woxui.KeyModifiers{0, 0, woxui.KeyModifierShift} {
			if !a.onQueryHintKey(woxui.KeyEvent{Key: woxui.KeyTab, Modifiers: mods, Down: true}) || a.editor.State() != before {
				t.Fatalf("Tab changed single argument %q with modifiers %v", value, mods)
			}
		}
		if a.queryTabFeedback != 2 {
			t.Fatal("only unavailable forward Tab should trigger feedback")
		}
	}
	a := &App{editor: woxui.NewTextEditor("")}
	a.installQueryHint(&common.QueryHint{Elements: []common.QueryElement{
		{Kind: common.QueryElementText, Text: "command "},
		{Kind: common.QueryElementArgument, Value: "first"},
		{Kind: common.QueryElementText, Text: " to "},
		{Kind: common.QueryElementArgument, Value: "second"},
	}})
	a.onQueryHintKey(woxui.KeyEvent{Key: woxui.KeyTab, Down: true})
	if a.editor.SelectedText() != "second" {
		t.Fatal("Tab must skip nonempty command text between arguments")
	}
	before := a.editor.State()
	a.onQueryHintKey(woxui.KeyEvent{Key: woxui.KeyTab, Down: true})
	if a.editor.State() != before {
		t.Fatal("Tab must not wrap from the last argument")
	}
	a.onQueryHintKey(woxui.KeyEvent{Key: woxui.KeyTab, Modifiers: woxui.KeyModifierShift, Down: true})
	if a.editor.SelectedText() != "first" {
		t.Fatal("Shift+Tab must navigate to the previous argument")
	}
}

func TestSelectEntireQueryHintThenType(t *testing.T) {
	a := &App{editor: woxui.NewTextEditor(""), lifecycleCtx: context.Background(), query: newInputQuery("set volume ")}
	a.installQueryHint(&common.QueryHint{Elements: []common.QueryElement{
		{Id: "command", Kind: "text", Text: "set volume "},
		{Id: "volume", Kind: "argument", Value: "30"},
	}})
	// Reopening with SelectAll must select the document, not only the focused slot.
	a.selectEntireQuery()
	if !a.queryHintEditorState.allSelected || a.editor.SelectedText() != "set volume 30" {
		t.Fatal("launcher selection did not include the query document with hints")
	}
	a.onTextInput(woxui.TextInputEvent{Kind: woxui.TextInputCommit, Text: "new search"})
	if a.query.QueryHint != nil || a.query.QueryText != "new search" || a.editor.State().Text != "new search" {
		t.Fatal("typing after reopen must replace the command and all arguments")
	}
	a.selectEntireQuery()
	if a.queryHintEditorState.allSelected || a.editor.SelectedText() != "new search" {
		t.Fatal("plain queries must retain ordinary select-all behavior")
	}
}

func TestQueryHintEditing(t *testing.T) {
	a := &App{editor: woxui.NewTextEditor(""), lifecycleCtx: context.Background(), query: newInputQuery("gh issues ")}
	s := &common.QueryHint{Elements: []common.QueryElement{
		{Id: "command", Kind: "text", Text: "gh issues "},
		{Id: "repo", Kind: "argument", Value: "owner/repo"},
		{Id: "space", Kind: "text", Text: " "},
		{Id: "issue", Kind: "argument", Value: "6"},
		{Id: "entity", Kind: "block", Value: "selected"},
	}}
	a.installQueryHint(s)
	key := func(k woxui.Key, mods woxui.KeyModifiers) {
		t.Helper()
		if !a.onQueryHintKey(woxui.KeyEvent{Key: k, Modifiers: mods, Down: true}) {
			t.Fatalf("unhandled %s", k)
		}
	}
	if a.editor.State().Text != s.PlainText() {
		t.Fatal("first argument not focused")
	}
	key(woxui.KeyTab, 0)
	if a.queryHintEditorState.active != 3 || a.editor.SelectedText() != "6" {
		t.Fatal("Tab did not skip separator")
	}
	a.editor.InsertText("42")
	text := a.updateQueryHintText(a.editor.State().Text)
	a.query.QueryText = text
	if a.query.QueryHint.Argument("issue") != "42" || s.Argument("issue") != "6" {
		t.Fatal("slot edit leaked or was lost")
	}
	key(woxui.KeyTab, 0)
	key(woxui.KeyBackspace, 0)
	if len(a.query.QueryHint.Elements) != 4 {
		t.Fatal("block was not deleted")
	}
	mods := woxui.KeyModifierControl
	if woxui.KeyModifierMeta.HasPrimary() {
		mods = woxui.KeyModifierMeta
	}
	key(woxui.Key("z"), mods)
	if len(a.query.QueryHint.Elements) != 5 {
		t.Fatal("undo lost the block")
	}
	key(woxui.Key("z"), mods)
	if a.query.QueryHint.Argument("issue") != "6" {
		t.Fatal("undo lost argument value")
	}
}

// Empty template slots must not add selectable text; Tab materializes only the next separator.
func TestQueryHintTemplateDefersSeparators(t *testing.T) {
	a := &App{editor: woxui.NewTextEditor(""), query: newInputQuery("g ")}
	template := &common.QueryHint{Elements: []common.QueryElement{
		{Id: "command", Kind: "text", Text: "g "}, {Id: "query", Kind: "argument", Required: true},
		{Id: "separator", Kind: "text", Text: " "}, {Id: "time", Kind: "argument", Required: true},
	}}
	a.installQueryHintTemplate(template)
	if a.editor.State().Text != "g " || a.query.QueryText != "g " || a.query.QueryHint.PlainText() != "g " {
		t.Fatal("template added text beyond the user's input")
	}
	a.editor.SetCaret(100)
	if a.editor.State().Selection.Focus != 2 {
		t.Fatal("caret can move beyond the actual input")
	}
	a.editor.InsertText("hello world")
	a.query.QueryText = a.updateQueryHintText(a.editor.State().Text)
	a.onQueryHintKey(woxui.KeyEvent{Key: woxui.KeyTab, Down: true})
	if a.editor.State().Text != "g hello world " || a.queryHintEditorState.active != 3 {
		t.Fatal("Tab did not insert the next argument separator")
	}
	a.editor.InsertText("today")
	a.query.QueryText = a.updateQueryHintText(a.editor.State().Text)
	if a.query.QueryHint.Argument("query") != "hello world" || a.query.QueryHint.Argument("time") != "today" {
		t.Fatal("deferred separators lost argument identities")
	}
	if template.Elements[2].Text != " " {
		t.Fatal("activation mutated the registered template")
	}
}

// Backspacing out of a multi-argument template must not leave invisible separators behind.
func TestQueryHintBackspaceThenRetypeTrigger(t *testing.T) {
	a := &App{editor: woxui.NewTextEditor(""), query: newInputQuery("g ")}
	a.installQueryHintTemplate(&common.QueryHint{Elements: []common.QueryElement{
		{Id: "command", Kind: "text", Text: "g "}, {Id: "query", Kind: "argument"},
		{Id: "separator", Kind: "text", Text: " "}, {Id: "time", Kind: "argument"},
	}})
	for _, want := range []string{"g", ""} {
		a.editor.HandleKey(woxui.KeyEvent{Key: woxui.KeyBackspace, Down: true})
		a.query.QueryText = a.updateQueryHintText(a.editor.State().Text)
		if a.query.QueryText != want || a.editor.State().Text != want || a.query.QueryHint != nil {
			t.Fatalf("backspace left template text: query=%q editor=%q", a.query.QueryText, a.editor.State().Text)
		}
	}
	a.editor.InsertText("g")
	if got := a.updateQueryHintText(a.editor.State().Text); got != "g" {
		t.Fatalf("retyping trigger acquired whitespace: %q", got)
	}
}

func TestQueryHintContinuousEditing(t *testing.T) {
	a := &App{editor: woxui.NewTextEditor(""), lifecycleCtx: context.Background(), query: newInputQuery("set volume ")}
	a.installQueryHint(&common.QueryHint{Elements: []common.QueryElement{{Id: "command", Kind: "text", Text: "set volume "}, {Id: "volume", Kind: "argument", Value: "50", Placeholder: "Volume"}}})
	edit := func(text string) {
		t.Helper()
		a.editor.InsertText(text)
		a.query.QueryText = a.updateQueryHintText(a.editor.State().Text)
	}
	a.editor.SetSelection(11, 13)
	edit("30")
	if a.query.QueryHint.Argument("volume") != "30" || a.editor.State().Text != "set volume 30" {
		t.Fatal("continuous argument replacement failed")
	}
	// Selecting across the command and argument must preserve exactly the user's edit.
	a.editor.SetSelection(4, 12)
	if a.editor.SelectedText() != "volume 3" {
		t.Fatal("cross-element selection was split")
	}
	edit("other ")
	if a.query.QueryHint != nil || a.query.QueryText != "set other 0" {
		t.Fatal("cross-element replacement lost text or retained stale hint")
	}
	a.onQueryHintKey(woxui.KeyEvent{Key: woxui.Key("z"), Modifiers: queryPrimaryModifier(), Down: true})
	if a.query.QueryHint == nil || a.editor.State().Text != "set volume 30" {
		t.Fatal("undo did not restore hint and full text")
	}
}

func TestQueryHintUnicodeAndOrdinaryBackspace(t *testing.T) {
	a := &App{editor: woxui.NewTextEditor(""), lifecycleCtx: context.Background(), query: newInputQuery("timer ")}
	a.installQueryHint(&common.QueryHint{Elements: []common.QueryElement{
		{Id: "command", Kind: "text", Text: "timer "}, {Id: "duration", Kind: "argument", Value: "5m"},
		{Id: "separator", Kind: "text", Text: " "}, {Id: "note", Kind: "argument", Value: "泡茶"},
	}})
	a.focusQueryElement(3)
	a.editor.InsertText("喝水")
	a.query.QueryText = a.updateQueryHintText(a.editor.State().Text)
	if a.query.QueryHint.Argument("note") != "喝水" {
		t.Fatal("rune offsets corrupted argument")
	}
	event := woxui.KeyEvent{Key: woxui.KeyBackspace, Down: true}
	if a.onQueryHintKey(event) {
		t.Fatal("argument intercepted ordinary Backspace")
	}
	a.editor.HandleKey(event)
	a.query.QueryText = a.updateQueryHintText(a.editor.State().Text)
	if a.query.QueryHint.Argument("note") != "喝" {
		t.Fatal("Backspace did not delete one character")
	}
}

func TestQueryHintWholeReplacementUndo(t *testing.T) {
	a := &App{editor: woxui.NewTextEditor(""), lifecycleCtx: context.Background(), query: newInputQuery("volume ")}
	a.installQueryHint(&common.QueryHint{Elements: []common.QueryElement{{Id: "command", Kind: "text", Text: "volume "}, {Id: "volume", Kind: "argument", Value: "50"}}})
	a.replaceWholeQueryHint("set volume 75")
	if a.query.QueryHint != nil || a.query.QueryText != "set volume 75" {
		t.Fatal("whole paste was parsed or lost")
	}
	a.onQueryHintKey(woxui.KeyEvent{Key: woxui.Key("z"), Modifiers: queryPrimaryModifier(), Down: true})
	if a.query.QueryHint == nil || a.query.QueryHint.Argument("volume") != "50" {
		t.Fatal("one undo must restore the complete document")
	}
}

func TestInstallBlockHintLeavesCaretAfterBlock(t *testing.T) {
	a := &App{editor: woxui.NewTextEditor("")}
	hint := &common.QueryHint{Elements: []common.QueryElement{
		{Id: "command", Kind: "text", Text: "gh issues "},
		{Id: "issue", Kind: "block", Value: "owner/仓库#6"},
	}}
	a.installQueryHint(hint)
	if a.editor.SelectedText() != "" || a.editor.State().Selection.Focus != len([]rune(hint.PlainText())) {
		t.Fatal("block hint must leave an unselected caret after its value")
	}
	a.editor.InsertText(" more")
	if a.editor.State().Text != hint.PlainText()+" more" {
		t.Fatal("typing must append after the block instead of replacing it")
	}
}
