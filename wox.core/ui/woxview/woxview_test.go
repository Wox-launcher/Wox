package woxview

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	component "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	widget "wox/ui/widget"
)

type fakeServices struct{}

func (fakeServices) MeasureText(text string, style woxui.TextStyle) (woxui.TextMetrics, error) {
	return woxui.TextMetrics{Size: woxui.Size{Width: float32(len([]rune(text))) * max(style.Size/2, 1), Height: max(style.Size, 1)}}, nil
}
func (fakeServices) Invalidate() error                            { return nil }
func (fakeServices) InvalidateRect(woxui.Rect) error              { return nil }
func (fakeServices) SetTextInputState(woxui.TextInputState) error { return nil }
func (fakeServices) SetPointerCursor(woxui.PointerCursor) error   { return nil }
func (fakeServices) UpdateAccessibility(woxui.AccessibilityTree, woxui.AccessibilityActionHandler) error {
	return nil
}

type recordedEvent struct {
	handler HandlerID
	args    string
}

// renderView mounts a view in a headless Host so tests exercise real layout and hit-testing.
func renderView(t *testing.T, view *View, events *[]recordedEvent) *widget.Host {
	t.Helper()
	theme := component.ControlTheme{Text: woxui.Color{R: 255, G: 255, B: 255, A: 255}, Accent: woxui.Color{B: 255, A: 255}}
	host := widget.NewHost(func(frame woxui.FrameInfo) widget.Widget {
		return view.Build(BuildProps{Width: frame.Size.Width, Height: frame.Size.Height, Theme: theme, OnEvent: func(handler HandlerID, args json.RawMessage) {
			*events = append(*events, recordedEvent{handler: handler, args: string(args)})
		}})
	})
	host.AttachServices(fakeServices{})
	frame(host)
	return host
}

func frame(host *widget.Host) {
	host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 300, Height: 200}, PixelSize: woxui.PixelSize{Width: 300, Height: 200}, Scale: 1})
}

func mustParse(t *testing.T, data string) *Tree {
	t.Helper()
	tree, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return tree
}

func automationBounds(t *testing.T, host *widget.Host, id string) woxui.Rect {
	t.Helper()
	for _, node := range host.Snapshot().Tree.Nodes {
		if node.AutomationID == id {
			return node.Bounds
		}
	}
	t.Fatalf("automation node %q not found", id)
	return woxui.Rect{}
}

func tap(host *widget.Host, bounds woxui.Rect) {
	point := woxui.Point{X: bounds.X + bounds.Width/2, Y: bounds.Y + bounds.Height/2}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: point})
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: point})
}

func TestParseAcceptsCatalogTree(t *testing.T) {
	tree := mustParse(t, `{"version":1,"root":{"type":"column","key":"root","props":{"style":{"padding":{"horizontal":12,"top":4},"gap":8,"backgroundColor":"$surface"}},"children":[
		{"type":"text","props":{"text":"00:59","style":{"fontSize":28,"fontWeight":"semibold","color":"#fff"}}},
		{"type":"row","children":[{"type":"spacer"},{"type":"button","props":{"label":"Cancel","variant":"primary","onPress":{"$handler":"h1"}}}]},
		{"type":"markdown","props":{"content":"**bold**"}},
		{"type":"image","props":{"icon":"control.close"}},
		{"type":"progress","props":{"value":0.5}}
	]}}`)
	padding := tree.Root.Props.(*BoxProps).Style.Padding
	if padding != (Edges{Top: Size{Value: 4}, Right: Size{Value: 12}, Bottom: Size{}, Left: Size{Value: 12}}) {
		t.Fatalf("padding shorthand = %+v", padding)
	}
	if len(tree.Root.Children) != 5 || tree.Root.Children[1].Children[1].Props.(*ButtonProps).OnPress.ID != "h1" {
		t.Fatal("children or handler not decoded")
	}
}

func TestParseRejectsInvalidTrees(t *testing.T) {
	deep := strings.Repeat(`{"type":"view","children":[`, MaxDepth+1) + `{"type":"text"}` + strings.Repeat(`]}`, MaxDepth+1)
	many := `{"type":"column","children":[` + strings.TrimSuffix(strings.Repeat(`{"type":"spacer"},`, MaxNodes), ",") + `]}`
	cases := map[string]string{
		"version":       `{"version":2,"root":{"type":"text"}}`,
		"no root":       `{"version":1}`,
		"unknown type":  `{"version":1,"root":{"type":"div"}}`,
		"unknown prop":  `{"version":1,"root":{"type":"text","props":{"txt":"x"}}}`,
		"unknown style": `{"version":1,"root":{"type":"text","props":{"style":{"float":"left"}}}}`,
		"bad color":     `{"version":1,"root":{"type":"text","props":{"style":{"color":"red"}}}}`,
		"bad token":     `{"version":1,"root":{"type":"text","props":{"style":{"color":"$nope"}}}}`,
		"bad enum":      `{"version":1,"root":{"type":"row","props":{"style":{"alignItems":"baseline"}}}}`,
		"negative":      `{"version":1,"root":{"type":"view","props":{"style":{"width":-1}}}}`,
		"leaf children": `{"version":1,"root":{"type":"text","children":[{"type":"text"}]}}`,
		"empty handler": `{"version":1,"root":{"type":"button","props":{"onPress":{"$handler":""}}}}`,
		"unknown icon":  `{"version":1,"root":{"type":"image","props":{"icon":"no.such.icon"}}}`,
		"remote image":  `{"version":1,"root":{"type":"image","props":{"source":{"imageType":"url","imageData":"https://x"}}}}`,
		"trailing":      `{"version":1,"root":{"type":"text"}} {}`,
		"too deep":      `{"version":1,"root":` + deep + `}`,
		"too many":      `{"version":1,"root":` + many + `}`,
	}
	for name, data := range cases {
		if _, err := Parse([]byte(data)); err == nil {
			t.Errorf("%s: accepted invalid tree", name)
		}
	}
}

func TestNewTreeValidatesGoBuiltTrees(t *testing.T) {
	if _, err := NewTree(&Node{Type: NodeText, Props: &ButtonProps{}}); err == nil {
		t.Fatal("accepted props of the wrong type")
	}
	tree, err := NewTree(&Node{Type: NodeColumn, Children: []*Node{{Type: NodeMarkdown, Props: &MarkdownProps{Content: "# Title"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := tree.Root.Props.(*BoxProps); !ok {
		t.Fatal("missing props were not defaulted")
	}
}

func TestColorsResolveThemeTokensAndHex(t *testing.T) {
	theme := component.ControlTheme{Accent: woxui.Color{R: 1, G: 2, B: 3, A: 255}}
	if got := ColorAccent.resolve(theme, woxui.Color{}); got != theme.Accent {
		t.Fatalf("token = %+v", got)
	}
	if got := (Color{Value: "#11223380"}).resolve(theme, woxui.Color{}); got != (woxui.Color{R: 0x11, G: 0x22, B: 0x33, A: 0x80}) {
		t.Fatalf("hex = %+v", got)
	}
	if got := (Color{Value: "#abc"}).resolve(theme, woxui.Color{}); got != (woxui.Color{R: 0xaa, G: 0xbb, B: 0xcc, A: 0xff}) {
		t.Fatalf("short hex = %+v", got)
	}
}

func TestButtonAndPressableBoxEmitHandlers(t *testing.T) {
	view := NewView()
	view.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","children":[
		{"type":"button","key":"cancel","props":{"label":"Cancel","onPress":{"$handler":"h-cancel"}}},
		{"type":"view","key":"card","props":{"style":{"width":100,"height":40},"label":"Card","onPress":{"$handler":"h-card"}}}
	]}}`))
	var events []recordedEvent
	host := renderView(t, view, &events)
	tap(host, automationBounds(t, host, "woxview/column0/k:cancel/button"))
	if len(events) != 1 || events[0] != (recordedEvent{handler: "h-cancel", args: "[]"}) {
		t.Fatalf("button events = %+v", events)
	}
	tap(host, automationBounds(t, host, "woxview/column0/k:card/view#press"))
	if len(events) != 2 || events[1].handler != "h-card" {
		t.Fatalf("card events = %+v", events)
	}
}

func TestSwitchReportsRequestedValue(t *testing.T) {
	view := NewView()
	view.SetTree(mustParse(t, `{"version":1,"root":{"type":"row","children":[{"type":"switch","key":"s","props":{"value":false,"onChange":{"$handler":"h"}}}]}}`))
	var events []recordedEvent
	host := renderView(t, view, &events)
	tap(host, automationBounds(t, host, "woxview/row0/k:s/switch"))
	if len(events) != 1 || events[0].args != "[true]" {
		t.Fatalf("switch events = %+v", events)
	}
}

func TestFlexChildrenShareRemainingSpace(t *testing.T) {
	view := NewView()
	view.SetTree(mustParse(t, `{"version":1,"root":{"type":"row","children":[
		{"type":"view","props":{"style":{"width":100,"height":10}}},
		{"type":"view","key":"grow","props":{"style":{"flex":1,"height":10},"label":"Grow","onPress":{"$handler":"h"}}}
	]}}`))
	var events []recordedEvent
	host := renderView(t, view, &events)
	// The flexible box spans 100..300; a tap near the right edge must land on it.
	tap(host, woxui.Rect{X: 290, Y: 0, Width: 4, Height: 10})
	if len(events) != 1 {
		t.Fatalf("flex child did not fill the remaining width: %+v", events)
	}
}

func TestEchoStateIgnoresStaleRoundTrips(t *testing.T) {
	var s echoState[string]
	s.init("", 0)
	s.local("a")
	s.local("ab")
	s.local("abc")
	// Echoes of earlier keystrokes must not rewind the visible text.
	s.receive("a", 0)
	if s.value != "abc" {
		t.Fatalf("stale echo rewound value to %q", s.value)
	}
	s.receive("abc", 0)
	if s.value != "abc" || len(s.pending) != 0 {
		t.Fatalf("final echo: value=%q pending=%v", s.value, s.pending)
	}
	// A value the user never typed is a programmatic update, such as clearing after submit.
	s.receive("", 0)
	if s.value != "" {
		t.Fatalf("programmatic clear ignored, value=%q", s.value)
	}
	// Uncontrolled producers keep sending the same initial value; that must not reset edits.
	s.local("typed")
	s.receive("", 0)
	if s.value != "typed" {
		t.Fatalf("unchanged prop reset local edit to %q", s.value)
	}
}

func TestNodeIDsPreferKeys(t *testing.T) {
	if got := nodeID("p", &Node{Type: NodeText, Key: "title"}, 3); got != "p/k:title/text" {
		t.Fatal(got)
	}
	if got := nodeID("p", &Node{Type: NodeText}, 3); got != "p/text3" {
		t.Fatal(got)
	}
}

func TestFormSubmitsCurrentValuesAndChoiceEvents(t *testing.T) {
	view := NewView()
	view.SetTree(mustParse(t, `{"version":1,"root":{"type":"form","props":{"onSubmit":{"$handler":"submit"}},"children":[
		{"type":"switch","key":"enabled","props":{"name":"enabled","label":"Enabled","onChange":{"$handler":"toggle"}}},
		{"type":"list","key":"mode","props":{"name":"mode","value":"a","options":[{"value":"a","label":"A"},{"value":"b","label":"B"}],"onChange":{"$handler":"choice"}}},
		{"type":"submit","key":"save","props":{"label":"Save"}}
	]}}`))
	var events []recordedEvent
	host := renderView(t, view, &events)
	tap(host, automationBounds(t, host, "woxview/form0/k:enabled/switch"))
	frame(host)
	tap(host, automationBounds(t, host, "woxview/form0/k:mode/list/option:Yg"))
	frame(host)
	tap(host, automationBounds(t, host, "woxview/form0/k:save/submit"))
	if len(events) != 3 || events[0].handler != "toggle" || events[0].args != "[true]" || events[1].handler != "choice" || events[1].args != `["b"]` || events[2].handler != "submit" || events[2].args != `[{"enabled":true,"mode":"b"}]` {
		t.Fatalf("events = %+v", events)
	}
}

func TestFormAndKeysRejectAmbiguity(t *testing.T) {
	for _, data := range []string{
		`{"version":1,"root":{"type":"column","children":[{"type":"text","key":"x"},{"type":"button","key":"x"}]}}`,
		`{"version":1,"root":{"type":"form","children":[{"type":"text-field","props":{"name":"x"}},{"type":"switch","props":{"name":"x"}}]}}`,
		`{"version":1,"root":{"type":"form","children":[{"type":"form"}]}}`,
		`{"version":1,"root":{"type":"select","props":{"options":[{"value":"a"},{"value":"a"}]}}}`,
		`{"version":1,"root":{"type":"submit","props":{"label":"Save"}}}`,
	} {
		if _, err := Parse([]byte(data)); err == nil {
			t.Errorf("accepted invalid tree %s", data)
		}
	}
}

func TestFormRetainsInputAcrossTreeUpdate(t *testing.T) {
	view := NewView()
	tree := `{"version":1,"root":{"type":"form","props":{"onSubmit":{"$handler":"save"}},"children":[{"type":"text-field","key":"name","props":{"name":"name","value":"","onChange":{"$handler":"change"}}},{"type":"submit","key":"save","props":{"label":"Save"}}]}}`
	view.SetTree(mustParse(t, tree))
	var events []recordedEvent
	host := renderView(t, view, &events)
	if !host.FocusAutomationID("woxview/form0/k:name/text-field/field") {
		t.Fatal("cannot focus input")
	}
	host.TextInput(woxui.TextInputEvent{Kind: woxui.TextInputCommit, Text: "hello"})
	frame(host)
	view.SetTree(mustParse(t, tree))
	frame(host)
	tap(host, automationBounds(t, host, "woxview/form0/k:save/submit"))
	if len(events) < 2 || events[len(events)-1].args != `[{"name":"hello"}]` {
		t.Fatalf("input or submit lost after update: %+v", events)
	}
}

func TestRemovedHandlerNoLongerBelongsToTree(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"button","props":{"label":"Go","onPress":{"$handler":"go"}}}}`))
	if !v.HasHandler("go") {
		t.Fatal("handler missing")
	}
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"text","props":{"text":"done"}}}`))
	if v.HasHandler("go") {
		t.Fatal("removed handler retained")
	}
}

func TestChoiceSelectionUpdatesWhenOptionIsRemoved(t *testing.T) {
	s := &choiceState{echo: &echoState[string]{}}
	s.echo.init("a", 0)
	s.echo.local("b")
	s.DidUpdateWidget(widget.StateContext{}, nil, choiceConfig{view: NewView(), props: &ChoiceProps{Value: "a", Options: []ChoiceOption{{Value: "a", Label: "A"}}}})
	if s.echo.value != "a" {
		t.Fatalf("removed selection retained: %q", s.echo.value)
	}
}

func TestKeyedFieldsSurviveReorderAndRemovedFieldsDoNotSubmit(t *testing.T) {
	view := NewView()
	first := `{"version":1,"root":{"type":"form","props":{"onSubmit":{"$handler":"save"}},"children":[{"type":"switch","key":"a","props":{"name":"a","label":"A"}},{"type":"switch","key":"b","props":{"name":"b","label":"B"}},{"type":"submit","key":"save","props":{"label":"Save"}}]}}`
	next := `{"version":1,"root":{"type":"form","props":{"onSubmit":{"$handler":"save"}},"children":[{"type":"switch","key":"b","props":{"name":"b","label":"B"}},{"type":"switch","key":"a","props":{"name":"a","label":"A"}},{"type":"submit","key":"save","props":{"label":"Save"}}]}}`
	removed := `{"version":1,"root":{"type":"form","props":{"onSubmit":{"$handler":"save"}},"children":[{"type":"switch","key":"b","props":{"name":"b","label":"B"}},{"type":"submit","key":"save","props":{"label":"Save"}}]}}`
	view.SetTree(mustParse(t, first))
	var events []recordedEvent
	host := renderView(t, view, &events)
	tap(host, automationBounds(t, host, "woxview/form0/k:b/switch"))
	frame(host)
	view.SetTree(mustParse(t, next))
	frame(host)
	tap(host, automationBounds(t, host, "woxview/form0/k:save/submit"))
	if events[len(events)-1].args != `[{"a":false,"b":true}]` {
		t.Fatalf("reordered: %+v", events)
	}
	view.SetTree(mustParse(t, removed))
	frame(host)
	tap(host, automationBounds(t, host, "woxview/form0/k:save/submit"))
	if events[len(events)-1].args != `[{"b":true}]` {
		t.Fatalf("removed: %+v", events)
	}
}

func TestSliderKeyboardReportsChange(t *testing.T) {
	view := NewView()
	view.SetTree(mustParse(t, `{"version":1,"root":{"type":"slider","props":{"label":"Volume","value":0.5,"step":0.1,"onChange":{"$handler":"change"}}}}`))
	var events []recordedEvent
	host := renderView(t, view, &events)
	if !host.FocusAutomationID("woxview/slider0") || !host.Key(woxui.KeyEvent{Key: woxui.KeyArrowRight, Down: true}) {
		t.Fatal("slider keyboard unavailable")
	}
	if len(events) != 1 || events[0].handler != "change" || events[0].args != "[0.6]" {
		t.Fatalf("events = %+v", events)
	}
}

func TestSelectKeyboardAndDisabledOption(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"select","props":{"label":"Mode","value":"a","options":[{"value":"a","label":"A"},{"value":"b","label":"B","disabled":true},{"value":"c","label":"C"}],"onChange":{"$handler":"choose"}}}}`))
	var events []recordedEvent
	host := renderView(t, v, &events)
	if !host.FocusAutomationID("woxview/select0") || !host.Key(woxui.KeyEvent{Key: woxui.KeyEnter, Down: true}) {
		t.Fatal("select cannot open from keyboard")
	}
	frame(host)
	tap(host, automationBounds(t, host, "woxview/select0/option:Yg"))
	if len(events) != 0 {
		t.Fatalf("disabled option emitted event: %+v", events)
	}
	tap(host, automationBounds(t, host, "woxview/select0/option:Yw"))
	if len(events) != 1 || events[0].args != `["c"]` {
		t.Fatalf("selection: %+v", events)
	}
	frame(host)
	if !host.FocusAutomationID("woxview/select0") || !host.Key(woxui.KeyEvent{Key: woxui.KeyEnter, Down: true}) {
		t.Fatal("select cannot open from keyboard")
	}
}

func TestImageCacheDropsUnusedEntries(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 2 2"><rect width="2" height="2" fill="var(--wox-theme-icon-color)"/></svg>`
	tree := func(data string) *Tree {
		return mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"image","props":{"source":{"ImageType":"svg","ImageData":%q},"style":{"width":8}}}}`, data))
	}
	view := NewView()
	view.SetTree(tree(svg))
	var events []recordedEvent
	host := renderView(t, view, &events)
	if len(view.images.entries) != 1 {
		t.Fatalf("entries after first build = %d", len(view.images.entries))
	}
	for key, entry := range view.images.entries {
		if entry.image == nil || key.lightTheme {
			t.Fatalf("themed svg not decoded for dark theme: %+v", key)
		}
	}
	view.SetTree(tree(strings.Replace(svg, "rect", "circle r=\"1\" cx=\"1\" cy=\"1\"", 1)))
	frame(host)
	if len(view.images.entries) != 1 {
		t.Fatalf("replaced image was retained: %d entries", len(view.images.entries))
	}
}
