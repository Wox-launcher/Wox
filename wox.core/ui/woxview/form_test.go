package woxview

import (
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"

	woxui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// cachedFormProps carries the built widget snapshot, so Boundary.Build reads only its props.
type cachedFormProps struct {
	child   widget.Widget
	tree    *Tree
	onBuild func()
}

func (p cachedFormProps) Equal(other cachedFormProps) bool { return p.tree == other.tree }

func TestFormValuesSurviveBoundaryCache(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"form","props":{"onSubmit":{"$handler":"save"}},"children":[{"type":"text-field","key":"name","props":{"name":"name"}},{"type":"submit","props":{"label":"Save"}}]}}`))
	var events []recordedEvent
	builds := 0
	onBuild := func() { builds++ }
	onEvent := func(id HandlerID, args json.RawMessage) { events = append(events, recordedEvent{id, string(args)}) }
	host := widget.NewHost(func(info woxui.FrameInfo) widget.Widget {
		child := v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height, OnEvent: onEvent})
		return widget.Boundary[cachedFormProps]{Key: "form-cache", Props: cachedFormProps{child, v.Tree(), onBuild}, Build: func(p cachedFormProps) widget.Widget { p.onBuild(); return p.child }}
	})
	host.AttachServices(fakeServices{})
	frame(host)
	if !host.FocusAutomationID("woxview/form0/k:name/text-field/field") {
		t.Fatal("cannot focus field")
	}
	host.TextInput(woxui.TextInputEvent{Kind: woxui.TextInputCommit, Text: "typed"})
	frame(host)
	frame(host)
	before := builds
	frame(host)
	if builds != before {
		t.Fatal("Boundary did not reuse its cache")
	}
	tap(host, automationBounds(t, host, "woxview/form0/submit1"))
	if len(events) != 1 || events[0].args != `[{"name":"typed"}]` {
		t.Fatalf("cached submit: %+v", events)
	}
	// Removing a field cleans its value even when no layout has run since SetTree.
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"form"}}`))
	if len(v.fieldValues) != 0 {
		t.Fatalf("removed values retained: %+v", v.fieldValues)
	}
}

func TestRevisionResetsFieldsWithoutReplayingChanges(t *testing.T) {
	for _, kind := range []NodeType{NodeTextField, NodeSwitch, NodeCheckbox, NodeSlider, NodeSelect, NodeList} {
		t.Run(string(kind), func(t *testing.T) {
			v := NewView()
			extra := ""
			if kind == NodeSelect || kind == NodeList {
				extra = `,"value":"a","options":[{"value":"a"},{"value":"b"}]`
			}
			data := func(rev int) string {
				return fmt.Sprintf(`{"version":1,"root":{"type":"form","props":{"onSubmit":{"$handler":"save"}},"children":[{"type":%q,"props":{"name":"field","onChange":{"$handler":"change"},"revision":%d%s}},{"type":"submit","props":{"label":"Save"}}]}}`, kind, rev, extra)
			}
			v.SetTree(mustParse(t, data(0)))
			var events []recordedEvent
			host := renderView(t, v, &events)
			id := "woxview/form0/" + string(kind) + "0"
			switch kind {
			case NodeTextField:
				host.FocusAutomationID(id + "/field")
				host.TextInput(woxui.TextInputEvent{Kind: woxui.TextInputCommit, Text: "abc"})
			case NodeSwitch, NodeCheckbox:
				tap(host, automationBounds(t, host, id))
			case NodeSlider:
				host.FocusAutomationID(id)
				host.Key(woxui.KeyEvent{Key: woxui.KeyArrowRight, Down: true})
			case NodeSelect, NodeList:
				if kind == NodeSelect {
					tap(host, automationBounds(t, host, id))
					frame(host)
				}
				tap(host, automationBounds(t, host, id+"/option:Yg"))
			}
			frame(host)
			if len(events) != 1 || events[0].handler != "change" {
				t.Fatalf("edit event: %+v", events)
			}
			events = nil
			initial := v.Tree().Root.Children[0].Props.(fieldOwner).initialValue()
			if reflect.DeepEqual(v.fieldValues[id], initial) {
				t.Fatal("test edit did not change value")
			}
			v.SetTree(mustParse(t, data(0)))
			frame(host)
			if reflect.DeepEqual(v.fieldValues[id], initial) {
				t.Fatal("unchanged revision reset edit")
			}
			v.SetTree(mustParse(t, data(1)))
			frame(host)
			if !reflect.DeepEqual(v.fieldValues[id], initial) {
				t.Fatalf("reset: got %v, want %v", v.fieldValues[id], initial)
			}
			if len(events) != 0 {
				t.Fatalf("reset emitted change: %+v", events)
			}
		})
	}
}

func TestTextRevisionResetsSelectionAndEchoHistory(t *testing.T) {
	v := NewView()
	c := textFieldConfig{view: v, id: "field", props: &TextFieldProps{Value: "你好"}}
	s := &textFieldState{}
	s.InitState(widget.StateContext{}, c)
	s.echo.local("你好abc")
	s.controller.SetText("你好abc", true)
	c.props = &TextFieldProps{Value: "你好", FieldProps: FieldProps{Revision: 1}}
	s.DidUpdateWidget(widget.StateContext{}, nil, c)
	if state := s.controller.State(); state.Text != "你好" || state.Selection.Focus != 2 || !state.Selection.Collapsed() {
		t.Fatalf("reset editing state: %+v", state)
	}
	if len(s.echo.pending) != 0 {
		t.Fatal("reset kept echo history")
	}
	s.echo.local("你好a")
	s.echo.local("你好ab")
	s.echo.receive("你好a", 1)
	if s.echo.value != "你好ab" {
		t.Fatal("stale echo rewound reset field")
	}
}

func TestTextFieldSubmitsFormWithKeyboard(t *testing.T) {
	for _, multiline := range []bool{false, true} {
		for _, ownHandler := range []bool{false, true} {
			v := NewView()
			lines, handler := 1, ""
			if multiline {
				lines = 3
			}
			if ownHandler {
				handler = `,"onSubmit":{"$handler":"field"}`
			}
			v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"form","props":{"onSubmit":{"$handler":"form"}},"children":[{"type":"text-field","props":{"name":"name","value":"hello","maxLines":%d%s}}]}}`, lines, handler)))
			var events []recordedEvent
			host := renderView(t, v, &events)
			host.FocusAutomationID("woxview/form0/text-field0/field")
			event := woxui.KeyEvent{Key: woxui.KeyEnter, Down: true}
			if multiline {
				host.Key(event)
				if len(events) != 0 {
					t.Fatal("plain Enter submitted multiline field")
				}
				event.Modifiers = woxui.KeyModifierControl
				if runtime.GOOS == "darwin" {
					event.Modifiers = woxui.KeyModifierMeta
				}
			}
			host.Key(event)
			want := HandlerID("form")
			if ownHandler {
				want = "field"
			}
			if len(events) != 1 || events[0].handler != want || !strings.Contains(events[0].args, "hello") {
				t.Fatalf("submission: %+v", events)
			}
		}
	}
}

func TestFormDisabledAndSubmitting(t *testing.T) {
	for _, mode := range []string{"disabled", "submitting"} {
		v := NewView()
		v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"form","props":{"%s":true,"onSubmit":{"$handler":"save"}},"children":[{"type":"text-field","props":{"name":"name"}},{"type":"switch","props":{"name":"enabled","onChange":{"$handler":"change"}}},{"type":"submit","props":{"label":"Save"}}]}}`, mode)))
		var events []recordedEvent
		host := renderView(t, v, &events)
		tap(host, automationBounds(t, host, "woxview/form0/submit2"))
		if host.FocusAutomationID("woxview/form0/text-field0/field") {
			host.Key(woxui.KeyEvent{Key: woxui.KeyEnter, Down: true})
		}
		if len(events) != 0 {
			t.Fatalf("%s allowed submit: %+v", mode, events)
		}
		tap(host, automationBounds(t, host, "woxview/form0/switch1"))
		if mode == "disabled" && len(events) != 0 {
			t.Fatal("disabled form allowed edits")
		}
		if mode == "submitting" && len(events) != 1 {
			t.Fatal("submitting form should allow field edits")
		}
	}
}

func TestFieldFeedbackAtDisplayScales(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2} {
		v := NewView()
		v.SetTree(mustParse(t, `{"version":1,"root":{"type":"text-field","props":{"label":"Name","required":true,"error":"Please enter a name"}}}`))
		var events []recordedEvent
		host := renderView(t, v, &events)
		host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 300, Height: 200}, PixelSize: woxui.PixelSize{Width: int(300 * scale), Height: int(200 * scale)}, Scale: scale})
		control := automationBounds(t, host, "woxview/text-field0/field")
		errorBounds := automationBounds(t, host, "woxview/text-field0/error")
		if errorBounds.Y < control.Y+control.Height || errorBounds.Height <= 0 {
			t.Fatalf("feedback overlaps at %v: field=%+v error=%+v", scale, control, errorBounds)
		}
		found := false
		for _, n := range host.Snapshot().Tree.Nodes {
			if n.AutomationID == "woxview/text-field0/field" && n.Label == "Name *" {
				found = true
			}
		}
		if !found {
			t.Fatal("required marker missing from accessible label")
		}
	}
}

func TestFieldFeedbackUpdatePreservesEditing(t *testing.T) {
	v := NewView()
	data := func(extra string) string {
		return `{"version":1,"root":{"type":"text-field","props":{"name":"name","label":"Name"` + extra + `}}}`
	}
	v.SetTree(mustParse(t, data("")))
	var events []recordedEvent
	host := renderView(t, v, &events)
	host.FocusAutomationID("woxview/text-field0/field")
	host.TextInput(woxui.TextInputEvent{Kind: woxui.TextInputCommit, Text: "abc"})
	frame(host)
	for _, extra := range []string{`,"error":"Too short","required":true`, ""} {
		v.SetTree(mustParse(t, data(extra)))
		frame(host)
		if v.fieldValues["woxview/text-field0"] != "abc" {
			t.Fatal("validation feedback discarded local editing")
		}
	}
}

func TestCatalogDeclaresAllHandlers(t *testing.T) {
	for kind, spec := range catalog {
		props := spec.newProps()
		value := reflect.ValueOf(props).Elem()
		count := 0
		for _, field := range reflect.VisibleFields(value.Type()) {
			if field.Type != reflect.TypeFor[*Handler]() {
				continue
			}
			count++
			value.FieldByIndex(field.Index).Set(reflect.ValueOf(&Handler{ID: HandlerID(field.Name)}))
		}
		if count == 0 {
			continue
		}
		owner, ok := props.(handlerOwner)
		if !ok {
			t.Fatalf("%s has handler fields without handlerOwner", kind)
		}
		registered := map[HandlerID]bool{}
		for _, h := range owner.handlers() {
			if h.handler == nil || h.name == "" {
				t.Fatalf("%s omitted a handler", kind)
			}
			registered[h.handler.ID] = true
		}
		if len(registered) != count {
			t.Fatalf("%s declared %d of %d handlers", kind, len(registered), count)
		}
	}
}
