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
	a := bindTestApp(&App{
		querySession: queryController{editor: woxui.NewTextEditor("")},
		lifecycleCtx: context.Background(),
		services:     &queryHintRegressionServices{instance: instance},
	})
	a.setQuery(newInputQuery("cb "))
	if a.querySession.query.QueryHint == nil || !a.querySession.query.QueryHint.CommandSuggestions || a.querySession.query.QueryText != "cb " || a.querySession.editor.State().Text != "cb " {
		t.Fatalf("replacement lost trigger guidance: %+v", a.querySession.query)
	}
	if len(a.querySession.queryHintEditorState.undo) != 1 {
		t.Fatal("replacement must retain the empty query as one undo step")
	}
	for _, text := range []string{"cb", "cb custom", "unrelated "} {
		a.setQuery(newInputQuery(text))
		if a.querySession.query.QueryHint != nil || a.querySession.query.QueryText != text {
			t.Fatalf("unexpected hint for %q", text)
		}
	}
	explicit := newInputQuery("cb ")
	explicit.QueryHint = &common.QueryHint{Elements: []common.QueryElement{
		{Id: "command", Kind: "text", Text: "cb "}, {Id: "query", Kind: "argument", Placeholder: "Custom"},
	}}
	a.setQuery(explicit)
	if a.querySession.query.QueryHint.Elements[1].Placeholder != "Custom" || a.querySession.query.QueryHint.CommandSuggestions {
		t.Fatal("explicit guidance lost precedence")
	}
}

// ChangeQuery replacements such as Indicator Enter keep the previous query as one undo step.
func TestChangeQueryReplacementIsUndoable(t *testing.T) {
	instance := &plugin.Instance{Metadata: plugin.Metadata{TriggerKeywords: []string{"cb"}, Commands: []plugin.MetadataCommand{{Command: "fav"}}}}
	a := bindTestApp(&App{
		querySession: queryController{
			editor: woxui.NewTextEditor("clip"),
			query:  newInputQuery("clip"),
		},
		lifecycleCtx:    context.Background(),
		services:        &queryHintRegressionServices{instance: instance},
		generalSettings: newGeneralSettingsController(CommonDeps{}, newSharedEditState()),
	})
	a.querySession.editor.SetCaret(4)
	a.setQuery(newInputQuery("cb "))
	if a.querySession.query.QueryText != "cb " || a.querySession.query.QueryHint == nil || !a.querySession.query.QueryHint.CommandSuggestions {
		t.Fatalf("indicator replacement lost trigger guidance: %+v", a.querySession.query)
	}
	if !a.querySession.onQueryHintKey(woxui.KeyEvent{Key: woxui.Key("z"), Modifiers: queryPrimaryModifier(), Down: true}) {
		t.Fatal("undo did not handle ChangeQuery")
	}
	if a.querySession.query.QueryText != "clip" || a.querySession.query.QueryHint != nil || a.querySession.editor.State().Text != "clip" {
		t.Fatalf("undo did not restore previous query: query=%q editor=%q hint=%+v", a.querySession.query.QueryText, a.querySession.editor.State().Text, a.querySession.query.QueryHint)
	}
	if a.querySession.editor.State().Selection != (woxui.TextSelection{Anchor: 4, Focus: 4}) {
		t.Fatalf("undo caret = %+v", a.querySession.editor.State().Selection)
	}
	if !a.querySession.onQueryHintKey(woxui.KeyEvent{Key: woxui.Key("y"), Modifiers: queryPrimaryModifier(), Down: true}) {
		t.Fatal("redo did not handle ChangeQuery")
	}
	if a.querySession.query.QueryText != "cb " || a.querySession.query.QueryHint == nil {
		t.Fatalf("redo lost replacement: %+v", a.querySession.query)
	}
}

// Same-text replacements must retain each distinct plugin context and refinement value.
func TestChangeQueryRoutingUndo(t *testing.T) {
	for _, field := range []string{"context", "refinements"} {
		t.Run(field, func(t *testing.T) {
			a := bindTestApp(&App{
				querySession: queryController{
					editor: woxui.NewTextEditor("same"),
					query:  newInputQuery("same"),
				},
				lifecycleCtx: context.Background(),
			})
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
				if !a.querySession.onQueryHintKey(woxui.KeyEvent{Key: woxui.Key(step.key), Modifiers: queryPrimaryModifier(), Down: true}) {
					t.Fatalf("%s was not handled", step.key)
				}
				values := a.querySession.query.ContextData
				if field == "refinements" {
					values = a.querySession.query.QueryRefinements
				}
				if a.querySession.query.QueryText != "same" || values["id"] != step.want {
					t.Fatalf("%s: query=%+v, want id=%q", step.key, a.querySession.query, step.want)
				}
			}
		})
	}
}

// Empty queries remain undoable between replacements; session resets discard both stacks.
func TestChangeQueryEmptyStateUndo(t *testing.T) {
	a := bindTestApp(&App{
		querySession: queryController{
			editor: woxui.NewTextEditor("older"),
			query:  newInputQuery("older"),
		},
		lifecycleCtx: context.Background(),
	})
	a.setQuery(newInputQuery(""))
	next := newInputQuery("")
	next.QueryScope = queryScope{Plugins: []queryScopePlugin{{PluginID: "explorer"}}}
	a.setQuery(next)
	for _, want := range []string{"", "older"} {
		if !a.querySession.onQueryHintKey(woxui.KeyEvent{Key: woxui.Key("z"), Modifiers: queryPrimaryModifier(), Down: true}) {
			t.Fatal("undo was not handled")
		}
		if a.querySession.query.QueryText != want || a.querySession.editor.State().Text != want || len(a.querySession.query.QueryScope.Plugins) != 0 {
			t.Fatalf("undo = %+v, want unscoped %q", a.querySession.query, want)
		}
	}
	a.resetQuery(newInputQuery(""))
	if len(a.querySession.queryHintEditorState.undo)+len(a.querySession.queryHintEditorState.redo) != 0 {
		t.Fatal("session reset retained undo history")
	}
}

// QueryScope replacements stay on the same undo stack as text and hint changes.
func TestQueryScopeReplacementIsUndoable(t *testing.T) {
	a := bindTestApp(&App{
		querySession: queryController{
			editor: woxui.NewTextEditor("files"),
			query:  newInputQuery("files"),
		},
		lifecycleCtx: context.Background(),
	})
	next := newInputQuery("")
	next.QueryScope = queryScope{Plugins: []queryScopePlugin{{PluginID: "explorer", Command: "browse"}}}
	a.setQuery(next)
	if a.querySession.query.QueryText != "" || len(a.querySession.query.QueryScope.Plugins) != 1 {
		t.Fatalf("scoped replacement: %+v", a.querySession.query)
	}
	if !a.querySession.onQueryHintKey(woxui.KeyEvent{Key: woxui.Key("z"), Modifiers: queryPrimaryModifier(), Down: true}) {
		t.Fatal("undo did not handle scoped ChangeQuery")
	}
	if a.querySession.query.QueryText != "files" || len(a.querySession.query.QueryScope.Plugins) != 0 || a.querySession.editor.State().Text != "files" {
		t.Fatalf("undo lost previous query: %+v", a.querySession.query)
	}
}

func TestClearQueryScopeIsUndoable(t *testing.T) {
	a := bindTestApp(&App{
		querySession: queryController{
			editor: woxui.NewTextEditor(""),
			query:  newInputQuery(""),
		},
		lifecycleCtx: context.Background(),
	})
	a.querySession.query.QueryScope = queryScope{Plugins: []queryScopePlugin{{PluginID: "explorer"}}}
	a.querySession.rememberQueryHint()
	a.clearQueryScopeLocked()
	if !a.querySession.onQueryHintKey(woxui.KeyEvent{Key: woxui.Key("z"), Modifiers: queryPrimaryModifier(), Down: true}) {
		t.Fatal("undo did not restore cleared scope")
	}
	if len(a.querySession.query.QueryScope.Plugins) != 1 || a.querySession.query.QueryScope.Plugins[0].PluginID != "explorer" {
		t.Fatalf("undone scope = %+v", a.querySession.query.QueryScope)
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
				a := bindTestApp(&App{
					querySession: queryController{
						editor: woxui.NewTextEditor(""),
						query:  newInputQuery(""),
					},
					lifecycleCtx: context.Background(),
					services:     &queryHintRegressionServices{instance: instance},
				})
				typeText := func(text string) {
					a.querySession.editor.InsertText(text)
					a.querySession.query.QueryText = a.querySession.updateQueryHintText(a.querySession.editor.State().Text)
				}
				check := func(text, ghost string) {
					t.Helper()
					if a.querySession.query.QueryText != text || a.querySession.editor.State().Text != text || a.querySession.query.QueryHint == nil || a.querySession.query.QueryHint.PlainText() != text {
						t.Fatalf("inconsistent document: want=%q query=%q editor=%q hint=%+v", text, a.querySession.query.QueryText, a.querySession.editor.State().Text, a.querySession.query.QueryHint)
					}
					widget := a.queryHintView(viewSnapshot{hint: a.querySession.query.QueryHint, editing: a.querySession.editor.State()}, 200, 40, 34)
					scroll := widget.(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.ScrollView)
					props := scroll.Child.(woxwidget.Boundary[launcherview.LauncherQueryProps]).Props
					if props.CompletionSuffix != ghost {
						t.Fatalf("text=%q ghost=%q, want %q", text, props.CompletionSuffix, ghost)
					}
				}
				typeText(prefix)
				if a.querySession.query.QueryText != prefix || a.querySession.query.QueryHint != nil || a.querySession.queryHintEditorState.candidate != nil {
					t.Fatal("bare context acquired whitespace or a hint")
				}
				typeText(" ")
				check(prefix+" ", "query time")
				// Hit testing to the right of ghost text must stop at the real document end.
				offset := a.queryOffsetAt(a.querySession.editor.State().Text, woxui.Point{X: 1000}, woxui.TextStyle{Size: 28}, 34)
				a.querySession.editor.SetCaret(offset)
				before := a.querySession.editor.State()
				a.querySession.onQueryHintKey(woxui.KeyEvent{Key: woxui.KeyTab, Down: true})
				a.querySession.editor.HandleKey(woxui.KeyEvent{Key: woxui.KeyArrowRight, Down: true})
				if before.Selection.Focus != len([]rune(prefix))+1 || a.querySession.editor.State() != before {
					t.Fatal("mouse, Tab or arrow moved into an unused separator")
				}
				typeText("sdf")
				check(prefix+" sdf", " time")
				a.querySession.onQueryHintKey(woxui.KeyEvent{Key: woxui.KeyTab, Down: true})
				check(prefix+" sdf ", "time")
				typeText("sdlkfj")
				check(prefix+" sdf sdlkfj", "")
				for i := 0; a.querySession.editor.State().Text != prefix+" " && i < 20; i++ {
					event := woxui.KeyEvent{Key: woxui.KeyBackspace, Modifiers: deletion.mod, Down: true}
					if !a.querySession.onQueryHintKey(event) {
						a.querySession.editor.HandleKey(event)
						a.querySession.query.QueryText = a.querySession.updateQueryHintText(a.querySession.editor.State().Text)
					}
				}
				check(prefix+" ", "query time")
				a.querySession.editor.SelectAll()
				typeText(prefix)
				if a.querySession.query.QueryText != prefix || a.querySession.query.QueryHint != nil {
					t.Fatal("retyping bare context retained its hint or separators")
				}
				typeText(" ")
				check(prefix+" ", "query time")
			})
		}
	}
}
