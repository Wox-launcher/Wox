package launcher

import (
	"context"
	"runtime"
	"testing"
	"wox/common"
	"wox/plugin"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type queryHintRegressionServices struct {
	sendQueryRecorderServices
	instance *plugin.Instance
}

func (s *queryHintRegressionServices) ResolveQueryHint(_ context.Context, text string) *common.QueryHint {
	hint, _ := plugin.MatchQueryHint(text, []*plugin.Instance{s.instance})
	return hint
}

// Query replacement must resolve trigger hints just like typing the context separator.
func TestQueryHintAfterChangeQuery(t *testing.T) {
	instance := &plugin.Instance{Metadata: plugin.Metadata{TriggerKeywords: []string{"cb"}, Commands: []plugin.MetadataCommand{{Command: "fav"}, {Command: "paste"}}}}
	a := &App{editor: woxui.NewTextEditor(""), lifecycleCtx: context.Background(), services: &queryHintRegressionServices{instance: instance}}
	a.setQuery(newInputQuery("cb "))
	if a.query.QueryHint == nil || !a.query.QueryHint.CommandSuggestions || a.query.QueryText != "cb " || a.editor.State().Text != "cb " {
		t.Fatalf("replacement lost trigger guidance: %+v", a.query)
	}
	if len(a.queryHintEditorState.undo) != 1 {
		t.Fatal("replacement must retain the empty query as one undo step")
	}
	for _, text := range []string{"cb", "cb custom", "unrelated "} {
		a.setQuery(newInputQuery(text))
		if a.query.QueryHint != nil || a.query.QueryText != text {
			t.Fatalf("unexpected hint for %q", text)
		}
	}
	explicit := newInputQuery("cb ")
	explicit.QueryHint = &common.QueryHint{Elements: []common.QueryElement{
		{Id: "command", Kind: "text", Text: "cb "}, {Id: "query", Kind: "argument", Placeholder: "Custom"},
	}}
	a.setQuery(explicit)
	if a.query.QueryHint.Elements[1].Placeholder != "Custom" || a.query.QueryHint.CommandSuggestions {
		t.Fatal("explicit guidance lost precedence")
	}
}

// ChangeQuery replacements such as Indicator Enter keep the previous query as one undo step.
func TestChangeQueryReplacementIsUndoable(t *testing.T) {
	instance := &plugin.Instance{Metadata: plugin.Metadata{TriggerKeywords: []string{"cb"}, Commands: []plugin.MetadataCommand{{Command: "fav"}}}}
	a := &App{
		editor:          woxui.NewTextEditor("clip"),
		query:           newInputQuery("clip"),
		lifecycleCtx:    context.Background(),
		services:        &queryHintRegressionServices{instance: instance},
		generalSettings: newGeneralSettingsController(CommonDeps{}, newSharedEditState()),
	}
	a.editor.SetCaret(4)
	a.setQuery(newInputQuery("cb "))
	if a.query.QueryText != "cb " || a.query.QueryHint == nil || !a.query.QueryHint.CommandSuggestions {
		t.Fatalf("indicator replacement lost trigger guidance: %+v", a.query)
	}
	if !a.onQueryHintKey(woxui.KeyEvent{Key: woxui.Key("z"), Modifiers: queryPrimaryModifier(), Down: true}) {
		t.Fatal("undo did not handle ChangeQuery")
	}
	if a.query.QueryText != "clip" || a.query.QueryHint != nil || a.editor.State().Text != "clip" {
		t.Fatalf("undo did not restore previous query: query=%q editor=%q hint=%+v", a.query.QueryText, a.editor.State().Text, a.query.QueryHint)
	}
	if a.editor.State().Selection != (woxui.TextSelection{Anchor: 4, Focus: 4}) {
		t.Fatalf("undo caret = %+v", a.editor.State().Selection)
	}
	if !a.onQueryHintKey(woxui.KeyEvent{Key: woxui.Key("y"), Modifiers: queryPrimaryModifier(), Down: true}) {
		t.Fatal("redo did not handle ChangeQuery")
	}
	if a.query.QueryText != "cb " || a.query.QueryHint == nil {
		t.Fatalf("redo lost replacement: %+v", a.query)
	}
}

// Same-text replacements must retain each distinct plugin context and refinement value.
func TestChangeQueryRoutingUndo(t *testing.T) {
	for _, field := range []string{"context", "refinements"} {
		t.Run(field, func(t *testing.T) {
			a := &App{editor: woxui.NewTextEditor("same"), query: newInputQuery("same"), lifecycleCtx: context.Background()}
			for _, value := range []string{"first", "second"} {
				next := newInputQuery("same")
				if field == "context" {
					next.ContextData = map[string]string{"id": value}
				} else {
					next.QueryRefinements = map[string]string{"id": value}
				}
				a.setQuery(next)
			}
			for _, step := range []struct{ key, want string }{{"z", "first"}, {"z", ""}, {"y", "first"}, {"y", "second"}} {
				if !a.onQueryHintKey(woxui.KeyEvent{Key: woxui.Key(step.key), Modifiers: queryPrimaryModifier(), Down: true}) {
					t.Fatalf("%s was not handled", step.key)
				}
				values := a.query.ContextData
				if field == "refinements" {
					values = a.query.QueryRefinements
				}
				if a.query.QueryText != "same" || values["id"] != step.want {
					t.Fatalf("%s: query=%+v, want id=%q", step.key, a.query, step.want)
				}
			}
		})
	}
}

// Empty queries remain undoable between replacements; session resets discard both stacks.
func TestChangeQueryEmptyStateUndo(t *testing.T) {
	a := &App{editor: woxui.NewTextEditor("older"), query: newInputQuery("older"), lifecycleCtx: context.Background()}
	a.setQuery(newInputQuery(""))
	next := newInputQuery("")
	next.QueryScope = queryScope{Plugins: []queryScopePlugin{{PluginID: "explorer"}}}
	a.setQuery(next)
	for _, want := range []string{"", "older"} {
		if !a.onQueryHintKey(woxui.KeyEvent{Key: woxui.Key("z"), Modifiers: queryPrimaryModifier(), Down: true}) {
			t.Fatal("undo was not handled")
		}
		if a.query.QueryText != want || a.editor.State().Text != want || len(a.query.QueryScope.Plugins) != 0 {
			t.Fatalf("undo = %+v, want unscoped %q", a.query, want)
		}
	}
	a.resetQuery(newInputQuery(""))
	if len(a.queryHintEditorState.undo)+len(a.queryHintEditorState.redo) != 0 {
		t.Fatal("session reset retained undo history")
	}
}

// QueryScope replacements stay on the same undo stack as text and hint changes.
func TestQueryScopeReplacementIsUndoable(t *testing.T) {
	a := &App{editor: woxui.NewTextEditor("files"), query: newInputQuery("files"), lifecycleCtx: context.Background()}
	next := newInputQuery("")
	next.QueryScope = queryScope{Plugins: []queryScopePlugin{{PluginID: "explorer", Command: "browse"}}}
	a.setQuery(next)
	if a.query.QueryText != "" || len(a.query.QueryScope.Plugins) != 1 {
		t.Fatalf("scoped replacement: %+v", a.query)
	}
	if !a.onQueryHintKey(woxui.KeyEvent{Key: woxui.Key("z"), Modifiers: queryPrimaryModifier(), Down: true}) {
		t.Fatal("undo did not handle scoped ChangeQuery")
	}
	if a.query.QueryText != "files" || len(a.query.QueryScope.Plugins) != 0 || a.editor.State().Text != "files" {
		t.Fatalf("undo lost previous query: %+v", a.query)
	}
}

func TestClearQueryScopeIsUndoable(t *testing.T) {
	a := &App{editor: woxui.NewTextEditor(""), query: newInputQuery(""), lifecycleCtx: context.Background()}
	a.query.QueryScope = queryScope{Plugins: []queryScopePlugin{{PluginID: "explorer"}}}
	a.rememberQueryHint()
	a.clearQueryScopeLocked()
	if !a.onQueryHintKey(woxui.KeyEvent{Key: woxui.Key("z"), Modifiers: queryPrimaryModifier(), Down: true}) {
		t.Fatal("undo did not restore cleared scope")
	}
	if len(a.query.QueryScope.Plugins) != 1 || a.query.QueryScope.Plugins[0].PluginID != "explorer" {
		t.Fatalf("undone scope = %+v", a.query.QueryScope)
	}
}

// Exercise the real resolver, editor, navigation and ghost rendering together across context types.
func TestQueryHintRegressionFlows(t *testing.T) {
	wordModifier := woxui.KeyModifierControl
	if runtime.GOOS == "darwin" {
		wordModifier = woxui.KeyModifierAlt
	}
	for _, prefix := range []string{"g", "set volume", "gh find"} {
		for _, deletion := range []struct {
			name string
			mod  woxui.KeyModifiers
		}{{"backspace", 0}, {"word_backspace", wordModifier}} {
			t.Run(prefix+"/"+deletion.name, func(t *testing.T) {
				template := &common.QueryHint{Elements: []common.QueryElement{
					{Id: "query", Kind: "argument", Placeholder: "query", Required: true},
					{Id: "separator", Kind: "text", Text: " "},
					{Id: "time", Kind: "argument", Placeholder: "time", Required: true},
				}}
				instance := &plugin.Instance{Metadata: plugin.Metadata{TriggerKeywords: []string{"g"}, TriggerQueryHints: map[string]*common.QueryHint{"g": template}}}
				if prefix == "set volume" {
					instance.Metadata = plugin.Metadata{TriggerKeywords: []string{"*"}, Commands: []plugin.MetadataCommand{{Command: "set-volume", Aliases: []string{prefix}, QueryHint: template}}}
				} else if prefix == "gh find" {
					instance.Metadata = plugin.Metadata{TriggerKeywords: []string{"gh"}, Commands: []plugin.MetadataCommand{{Command: "find", QueryHint: template}}}
				}
				a := &App{editor: woxui.NewTextEditor(""), query: newInputQuery(""), lifecycleCtx: context.Background(), services: &queryHintRegressionServices{instance: instance}}
				typeText := func(text string) {
					a.editor.InsertText(text)
					a.query.QueryText = a.updateQueryHintText(a.editor.State().Text)
				}
				check := func(text, ghost string) {
					t.Helper()
					if a.query.QueryText != text || a.editor.State().Text != text || a.query.QueryHint == nil || a.query.QueryHint.PlainText() != text {
						t.Fatalf("inconsistent document: want=%q query=%q editor=%q hint=%+v", text, a.query.QueryText, a.editor.State().Text, a.query.QueryHint)
					}
					widget := a.queryHintView(viewSnapshot{hint: a.query.QueryHint, editing: a.editor.State()}, 200, 40, 34)
					scroll := widget.(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.ScrollView)
					props := scroll.Child.(woxwidget.Boundary[launcherview.LauncherQueryProps]).Props
					if props.CompletionSuffix != ghost {
						t.Fatalf("text=%q ghost=%q, want %q", text, props.CompletionSuffix, ghost)
					}
				}
				typeText(prefix)
				if a.query.QueryText != prefix || a.query.QueryHint != nil || a.queryHintEditorState.candidate != nil {
					t.Fatal("bare context acquired whitespace or a hint")
				}
				typeText(" ")
				check(prefix+" ", "query time")
				// Hit testing to the right of ghost text must stop at the real document end.
				offset := a.queryOffsetAt(a.editor.State().Text, woxui.Point{X: 1000}, woxui.TextStyle{Size: 28}, 34)
				a.editor.SetCaret(offset)
				before := a.editor.State()
				a.onQueryHintKey(woxui.KeyEvent{Key: woxui.KeyTab, Down: true})
				a.editor.HandleKey(woxui.KeyEvent{Key: woxui.KeyArrowRight, Down: true})
				if before.Selection.Focus != len([]rune(prefix))+1 || a.editor.State() != before {
					t.Fatal("mouse, Tab or arrow moved into an unused separator")
				}
				typeText("sdf")
				check(prefix+" sdf", " time")
				a.onQueryHintKey(woxui.KeyEvent{Key: woxui.KeyTab, Down: true})
				check(prefix+" sdf ", "time")
				typeText("sdlkfj")
				check(prefix+" sdf sdlkfj", "")
				for i := 0; a.editor.State().Text != prefix+" " && i < 20; i++ {
					event := woxui.KeyEvent{Key: woxui.KeyBackspace, Modifiers: deletion.mod, Down: true}
					if !a.onQueryHintKey(event) {
						a.editor.HandleKey(event)
						a.query.QueryText = a.updateQueryHintText(a.editor.State().Text)
					}
				}
				check(prefix+" ", "query time")
				a.editor.SelectAll()
				typeText(prefix)
				if a.query.QueryText != prefix || a.query.QueryHint != nil {
					t.Fatal("retyping bare context retained its hint or separators")
				}
				typeText(" ")
				check(prefix+" ", "query time")
			})
		}
	}
}
