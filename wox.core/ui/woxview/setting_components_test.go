package woxview

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"runtime"
	"strings"
	"testing"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// replaceFieldText drives native selection and committed text instead of bypassing the editor.
func replaceFieldText(t *testing.T, host *widget.Host, id, text string) {
	t.Helper()
	if !host.FocusAutomationID(id) {
		t.Fatalf("missing focus target %s", id)
	}
	modifier := ui.KeyModifierControl
	if runtime.GOOS == "darwin" {
		modifier = ui.KeyModifierMeta
	}
	host.Key(ui.KeyEvent{Key: "a", Modifiers: modifier, Down: true})
	host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: text})
}

// TestNumberFieldDraftsStepsAndSubmission checks finite form values, decimal stepping, and blur commit.
func TestNumberFieldDraftsStepsAndSubmission(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2} {
		v := NewView()
		tree := func(value float64, revision int) *Tree {
			return mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"form","props":{"onSubmit":{"$handler":"submit"}},"children":[{"type":"number-field","key":"n","props":{"name":"amount","label":"Amount","value":%g,"revision":%d,"min":-1,"max":10,"step":0.1,"onChange":{"$handler":"change"}}},{"type":"submit","key":"save","props":{"label":"Save"}}]}}`, value, revision))
		}
		v.SetTree(tree(0.2, 0))
		var events []recordedEvent
		host := layoutHost(v, &events)
		defer host.Dispose()
		render := func() { renderLayout(host, 260, 180, scale) }
		id := "woxview/form0/k:n/number-field"
		render()
		host.FocusAutomationID(id + "/field")
		host.Key(ui.KeyEvent{Key: ui.KeyArrowUp, Down: true})
		render()
		if v.fieldValues[id] != float64(0.3) || events[0].args != `[0.3]` {
			t.Fatalf("decimal step: %v %+v", v.fieldValues[id], events)
		}
		replaceFieldText(t, host, id+"/field", "2")
		render()
		v.SetTree(tree(0.3, 0))
		render()
		if v.fieldValues[id] != float64(2) {
			t.Fatal("stale numeric echo rewound input")
		}
		before := len(events)
		replaceFieldText(t, host, id+"/field", "-")
		render()
		if v.fieldValues[id] != float64(2) || len(events) != before {
			t.Fatal("incomplete draft escaped as a value")
		}
		v.SetTree(tree(0.3, 0))
		render()
		for _, node := range host.Snapshot().Tree.Nodes {
			if node.AutomationID == id+"/field" && node.Value != "-" {
				t.Fatalf("rebuild discarded incomplete draft: %q", node.Value)
			}
		}
		replaceFieldText(t, host, id+"/field", "80")
		render()
		tap(host, automationBounds(t, host, "woxview/form0/k:save/submit"))
		if v.fieldValues[id] != float64(10) || events[len(events)-1].args != `[{"amount":10}]` {
			t.Fatalf("blur must clamp before submit: %v %+v", v.fieldValues[id], events)
		}
		v.SetTree(tree(0.3, 1))
		render()
		replaceFieldText(t, host, id+"/field", "NaN")
		host.Key(ui.KeyEvent{Key: ui.KeyEnter, Down: true})
		if events[len(events)-1].args != `[{"amount":0.3}]` {
			t.Fatalf("invalid draft submitted: %+v", events)
		}
	}
}

// TestNumberFieldResetAndRangeChangesApplyOffscreen separates retained numbers from disposed editors.
func TestNumberFieldResetAndRangeChangesApplyOffscreen(t *testing.T) {
	v := NewView()
	set := func(revision int, maximum float64, disabled bool) {
		v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"number-field","key":"n","props":{"value":0,"revision":%d,"max":%g,"disabled":%t,"onChange":{"$handler":"change"}}}}`, revision, maximum, disabled)))
	}
	set(0, 10, false)
	var events []recordedEvent
	host := layoutHost(v, &events)
	frame(host)
	replaceFieldText(t, host, "woxview/k:n/number-field/field", "8")
	frame(host)
	host.Dispose()
	set(0, 4, false)
	if v.fieldValues["woxview/k:n/number-field"] != float64(4) {
		t.Fatal("offscreen range update did not clamp")
	}
	set(1, 10, true)
	host = layoutHost(v, &events)
	defer host.Dispose()
	frame(host)
	if v.fieldValues["woxview/k:n/number-field"] != float64(0) || host.FocusAutomationID("woxview/k:n/number-field/field") {
		t.Fatal("reset/disabled remount failed")
	}
}

// TestHotkeyFieldRecordingAndReset covers local capture, ancestor shortcuts, clearing, and native focus exit.
func TestHotkeyFieldRecordingAndReset(t *testing.T) {
	v := NewView()
	set := func(revision int, disabled bool) {
		v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"form","props":{"disabled":%t,"onSubmit":{"$handler":"submit"},"shortcuts":[{"key":"ctrl+k","onPress":{"$handler":"ancestor"}}]},"children":[{"type":"hotkey","key":"hotkey","props":{"name":"key","label":"Shortcut","placeholder":"Record","revision":%d,"onChange":{"$handler":"change"}}},{"type":"submit","key":"save","props":{"label":"Save"}}]}}`, disabled, revision)))
	}
	set(0, false)
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	id := "woxview/form0/k:hotkey/hotkey"
	for _, scale := range []float32{1, 1.5, 2} {
		renderLayout(host, 300, 180, scale)
		if !host.FocusAutomationID(id + "/field") {
			t.Fatal("recorder not focusable")
		}
		host.Key(ui.KeyEvent{Key: "k", Modifiers: ui.KeyModifierControl, Down: true})
		renderLayout(host, 300, 180, scale)
	}
	if len(events) != 1 || events[0].args != `["ctrl+k"]` || events[0].handler != "change" {
		t.Fatalf("recording leaked to ancestor shortcut: %+v", events)
	}
	host.Key(ui.KeyEvent{Key: "x", Modifiers: ui.KeyModifierControl, Down: true, Composing: true})
	host.Key(ui.KeyEvent{Key: "x", Modifiers: ui.KeyModifierControl, Down: true, Repeat: true})
	host.Key(ui.KeyEvent{Key: "k", Modifiers: ui.KeyModifierControl, Down: true, Repeat: true})
	if len(events) != 1 {
		t.Fatal("composition/repeat recorded")
	}
	host.Key(ui.KeyEvent{Key: ui.KeyBackspace, Down: true})
	frame(host)
	if v.fieldValues[id] != "" || events[len(events)-1].args != `[""]` {
		t.Fatal("recorder clear failed")
	}
	host.Key(ui.KeyEvent{Key: "f6", Down: true})
	frame(host)
	host.Key(ui.KeyEvent{Key: ui.KeyEscape, Down: true})
	if host.FocusedKey() != "" {
		t.Fatal("Escape did not end recording")
	}
	tap(host, automationBounds(t, host, "woxview/form0/k:save/submit"))
	if events[len(events)-1].args != `[{"key":"f6"}]` {
		t.Fatalf("hotkey form value: %+v", events)
	}
	set(1, true)
	frame(host)
	if v.fieldValues[id] != "" || host.FocusAutomationID(id+"/field") {
		t.Fatal("hotkey reset or disabled form failed")
	}
}

type codeClipboard struct{ text string }

func (c *codeClipboard) ReadText() (string, error)    { return c.text, nil }
func (c *codeClipboard) WriteText(value string) error { c.text = value; return nil }

// TestCodeCopiesLiteralSourceAndUpdates verifies exact bytes, including fences and trailing whitespace.
func TestCodeCopiesLiteralSourceAndUpdates(t *testing.T) {
	clipboard := &codeClipboard{}
	component.SetClipboardProvider(clipboard)
	defer component.SetClipboardProvider(nil)
	v := NewView()
	set := func(code string) {
		raw, _ := json.Marshal(code)
		v.SetTree(mustParse(t, `{"version":1,"root":{"type":"code","key":"source","props":{"code":`+string(raw)+`,"language":"go"}}}`))
	}
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	for i, source := range []string{"  ```go\n<literal>\n```\n\n", "i18n:this-is-code\t\n"} {
		set(source)
		renderLayout(host, 260, 300, float32(i+1))
		tap(host, automationBounds(t, host, "woxview/k:source/code/copy"))
		if clipboard.text != source || len(events) != 0 {
			t.Fatalf("code copy changed content or used RPC: %q", clipboard.text)
		}
	}
}

// TestLinkUsesHandlerOrHostOpening keeps callback precedence, original URL bytes, and disabled state explicit.
func TestLinkUsesHandlerOrHostOpening(t *testing.T) {
	const target = "https://example.com/path#x%2Fy"
	v := NewView()
	set := func(handler string, disabled bool) {
		props := fmt.Sprintf(`"url":%q,"label":"i18n:link","disabled":%t`, target, disabled)
		if handler != "" {
			props += fmt.Sprintf(`,"onPress":{"$handler":%q}`, handler)
		}
		v.SetTree(mustParse(t, `{"version":1,"root":{"type":"link","props":{`+props+`}}}`))
	}
	opened := ""
	var events []Event
	host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Translate: func(string) string { return "Open docs" },
			OnOpenLink: func(url string) { opened = url }, OnNodeEvent: func(e Event) { events = append(events, e) }})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	set("", false)
	frame(host)
	host.FocusAutomationID("woxview/link0")
	host.Key(ui.KeyEvent{Key: ui.KeyEnter, Down: true})
	if opened != target || len(events) != 0 {
		t.Fatal("link target was rewritten or incorrectly emitted")
	}
	set("custom", false)
	frame(host)
	opened = ""
	tap(host, automationBounds(t, host, "woxview/link0"))
	if opened != "" || len(events) != 1 || events[0].Handler != "custom" || string(events[0].Args) != `[]` {
		t.Fatal("link callback did not override opening")
	}
	for _, n := range host.Snapshot().Tree.Nodes {
		if n.AutomationID == "woxview/link0" && (n.Role != ui.AccessibilityRoleLink || n.Label != "Open docs") {
			t.Fatalf("link semantics: %+v", n)
		}
	}
	set("custom", true)
	frame(host)
	tap(host, automationBounds(t, host, "woxview/link0"))
	if len(events) != 1 || host.FocusAutomationID("woxview/link0") {
		t.Fatal("disabled link activated")
	}
}

// TestTableScrollsAndRetainsRowActionIdentity verifies both axes and refreshed callbacks after a reorder.
func TestTableScrollsAndRetainsRowActionIdentity(t *testing.T) {
	v := NewView()
	set := func(reverse bool, handler string) {
		rows := make([]string, 20)
		for i := range rows {
			n := i
			if reverse {
				n = len(rows) - 1 - i
			}
			rows[i] = fmt.Sprintf(`{"key":"r%d","cells":["Row %d","Long value"],"actions":[{"key":"open","label":"Open","onPress":{"$handler":%q}},{"key":"blocked","label":"Blocked","disabled":true,"onPress":{"$handler":"blocked"}}]}`, n, n, handler)
		}
		v.SetTree(mustParse(t, `{"version":1,"root":{"type":"table","key":"rows","props":{"label":"Data","style":{"height":180},"columns":[{"key":"name","label":"Name","width":160},{"key":"value","label":"Value","width":160}],"rows":[`+strings.Join(rows, ",")+`]}}}`))
	}
	set(false, "old")
	var events []Event
	host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, OnNodeEvent: func(e Event) { events = append(events, e) }})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	render := func(scale float32) { renderLayout(host, 240, 220, scale); renderLayout(host, 240, 220, scale) }
	encode := func(value string) string { return base64.RawURLEncoding.EncodeToString([]byte(value)) }
	id := "woxview/k:rows/table/row:" + encode("r19") + "/action:" + encode("open")
	render(1)
	header := automationBounds(t, host, "woxview/k:rows/table/header/"+encode("name"))
	if !host.FocusAutomationID(id) {
		t.Fatal("missing table action")
	}
	render(1.5)
	action := automationBounds(t, host, id)
	if action.X < 0 || action.Y < 40 || action.X+action.Width > 240 || action.Y+action.Height > 180 {
		t.Fatalf("focused table action not scrolled into view: %+v", action)
	}
	if automationBounds(t, host, "woxview/k:rows/table/header/"+encode("name")).Y != header.Y {
		t.Fatal("vertical scrolling moved table header")
	}
	host.Key(ui.KeyEvent{Key: ui.KeyEnter, Down: true})
	if len(events) != 1 || events[0].Handler != "old" || string(events[0].Args) != `["r19"]` || events[0].Key != "rows" {
		t.Fatalf("row action: %+v", events)
	}
	set(true, "new")
	render(2)
	host.FocusAutomationID(id)
	render(2)
	host.Key(ui.KeyEvent{Key: ui.KeySpace, Down: true})
	if len(events) != 2 || events[1].Handler != "new" || string(events[1].Args) != `["r19"]` || v.HasHandler("old") {
		t.Fatalf("reordered row used stale action: %+v", events)
	}
	blocked := "woxview/k:rows/table/row:" + encode("r19") + "/action:" + encode("blocked")
	if host.FocusAutomationID(blocked) {
		t.Fatal("disabled row action focusable")
	}
}

// TestSettingComponentsRejectInvalidInput exercises local contract rules in addition to JSON decoding.
func TestSettingComponentsRejectInvalidInput(t *testing.T) {
	for _, node := range []string{
		`{"type":"number-field","props":{"min":3,"max":2}}`,
		`{"type":"number-field","props":{"step":-1}}`,
		`{"type":"number-field","props":{"value":"1"}}`,
		`{"type":"number-field","props":{"onSubmit":{"$handler":""}}}`,
		`{"type":"hotkey","props":{"value":"ctrl+unknown-key"}}`,
		`{"type":"hotkey","props":{"value":"hold:ctrl"}}`,
		`{"type":"table"}`,
		`{"type":"table","props":{"columns":[{"key":"a"},{"key":"a"}]}}`,
		`{"type":"table","props":{"columns":[{"key":"a","width":-1}]}}`,
		`{"type":"table","props":{"columns":[{"key":"a"}],"rows":[{"key":"r","cells":[]}]}}`,
		`{"type":"table","props":{"columns":[{"key":"a"}],"rows":[{"key":"r","cells":[1]}]}}`,
		`{"type":"table","props":{"columns":[{"key":"a"}],"rows":[{"key":"r","cells":["a"],"actions":[{"key":"x","label":"X"}]}]}}`,
		`{"type":"table","props":{"columns":[{"key":"a"}],"rows":[{"key":"r","cells":["a"],"actions":[{"key":"x","label":"X","icon":"missing-icon","onPress":{"$handler":"x"}}]}]}}`,
		`{"type":"code","props":{"code":false}}`,
		`{"type":"link"}`,
		`{"type":"link","props":{"url":"javascript:alert(1)"}}`,
		`{"type":"link","props":{"url":"https://"}}`,
		`{"type":"link","props":{"url":"relative/path"}}`,
	} {
		if _, err := Parse([]byte(`{"version":1,"root":` + node + `}`)); err == nil {
			t.Fatalf("accepted invalid component: %s", node)
		}
	}
	if _, err := NewTree(&Node{Type: NodeNumberField, Props: &NumberFieldProps{Value: math.NaN()}}); err == nil {
		t.Fatal("Go tree accepted NaN")
	}
}
