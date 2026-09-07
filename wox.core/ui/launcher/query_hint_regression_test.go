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
