package woxview

import (
	"encoding/json"
	"strings"
	"testing"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// TestSpanContractsRejectBlockNesting verifies both Parse and Go construction rules.
func TestSpanContractsRejectBlockNesting(t *testing.T) {
	for _, root := range []string{
		`{"type":"span","props":{"text":"orphan"}}`,
		`{"type":"column","children":[{"type":"span"}]}`,
		`{"type":"text","children":[{"type":"button"}]}`,
		`{"type":"text","children":[{"type":"span","children":[{"type":"span"}]}]}`,
		`{"type":"text","children":[{"type":"span","props":{"style":{"width":0}}}]}`,
		`{"type":"text","children":[{"type":"span","props":{"style":{"fontWeight":"heavy"}}}]}`,
		`{"type":"text","children":[{"type":"span","props":{"style":{"color":"red"}}}]}`,
		`{"type":"text","children":[{"type":"span","props":{"style":{"italic":"yes"}}}]}`,
		`{"type":"text","children":[{"type":"span","props":{"onPress":{"$handler":""}}}]}`,
		`{"type":"text","children":[{"type":"span","key":"x"},{"type":"span","key":"x"}]}`,
	} {
		if _, err := Parse([]byte(`{"version":1,"root":` + root + `}`)); err == nil {
			t.Fatalf("accepted invalid inline tree %s", root)
		}
	}
	if _, err := NewTree(&Node{Type: NodeSpan, Props: &SpanProps{Text: "orphan"}}); err == nil {
		t.Fatal("Go tree accepted orphan span")
	}
}

// TestSpanStyleInheritanceTranslationAndHandlers checks immutable display conversion and handler indexing.
func TestSpanStyleInheritanceTranslationAndHandlers(t *testing.T) {
	tree := mustParse(t, `{"version":1,"root":{"type":"text","props":{"text":"i18n:prefix","style":{"fontSize":16,"fontWeight":"semibold","italic":true,"color":"#234"}},"children":[
		{"type":"span","key":"link","props":{"text":"i18n:link","style":{"fontWeight":"regular","fontFamily":"monospace","italic":false,"underline":false,"color":"#00000000"},"onPress":{"$handler":"open"},"onFocusChange":{"$handler":"focus"}}}
	]}}`)
	v := NewView()
	v.SetTree(tree)
	b := &builder{view: v, props: BuildProps{Theme: component.ControlTheme{DensityScale: 1.1}, Translate: func(value string) string { return strings.TrimPrefix(value, "i18n:") }, OnEvent: func(HandlerID, json.RawMessage) {}}}
	runs := b.richTextRuns(tree.Root, "text")
	if runs[0].Text != "prefix" || runs[1].Text != "link" || runs[1].Style.Size != 16 || runs[1].Style.Italic || runs[1].Style.Weight == ui.FontWeightSemibold || runs[1].Style.Family != ui.FontFamilyMonospace || runs[1].Underline || runs[1].Color.A != 0 {
		t.Fatalf("span inheritance/translation: %+v", runs)
	}
	if !v.HasHandler("open") || !v.HasHandler("focus") || tree.Root.Children[0].Props.(*SpanProps).Text != "i18n:link" {
		t.Fatal("span handler index or immutable props changed")
	}
}

// TestSpanEventsIdentityAndCommands covers a real span widget, reordering, and handler revocation.
func TestSpanEventsIdentityAndCommands(t *testing.T) {
	v := NewView()
	set := func(reverse bool, handler string) {
		children := []string{`{"type":"span","key":"plain","props":{"text":"prefix "}}`, `{"type":"span","key":"link","props":{"text":"open docs","onPress":{"$handler":"` + handler + `"}}}`}
		if reverse {
			children[0], children[1] = children[1], children[0]
		}
		v.SetTree(mustParse(t, `{"version":1,"root":{"type":"text","key":"paragraph","children":[`+strings.Join(children, ",")+`]}}`))
	}
	set(false, "old")
	var events []Event
	host := widget.NewHost(func(info ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height, OnNodeEvent: func(event Event) { events = append(events, event) }})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	id := "woxview/k:paragraph/text/k:link/span"
	for i, scale := range []float32{1, 1.5, 2} {
		renderLayout(host, 200, 120, scale)
		if err := v.EnqueueCommand(host, CommandFocus, "link", func(err error) {
			if err != nil {
				t.Fatal(err)
			}
		}); err != nil {
			t.Fatal(err)
		}
		drainCommands(t, v, host, scale)
		if host.FocusedKey() != widget.Key(id) {
			t.Fatalf("span command focus: %s", host.FocusedKey())
		}
		host.Key(ui.KeyEvent{Key: ui.KeyEnter, Down: true})
		got := events[len(events)-1]
		want := HandlerID("new")
		if i == 0 {
			want = "old"
		}
		if got.Handler != want || got.Key != "link" || string(got.Args) != `[]` {
			t.Fatalf("span event: %+v", got)
		}
		set(true, "new")
	}
	if v.HasHandler("old") {
		t.Fatal("span retained stale handler after update")
	}
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"text","props":{"text":"removed"}}}`))
	renderLayout(host, 200, 120, 1)
	if v.HasHandler("new") || hasAutomation(host, id) {
		t.Fatal("removed span retained an action")
	}
}
