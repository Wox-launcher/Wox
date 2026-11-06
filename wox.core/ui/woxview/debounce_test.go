package woxview

import (
	"fmt"
	"strings"
	"testing"
	"time"

	ui "wox/ui/runtime"
)

type testChangeTimer struct {
	at      time.Duration
	call    func()
	stopped bool
}

type testChangeClock struct {
	now    time.Duration
	timers []*testChangeTimer
}

// schedule keeps timer delivery deterministic and lets tests replay already-posted callbacks.
func (c *testChangeClock) schedule(delay time.Duration, callback func()) func() {
	timer := &testChangeTimer{at: c.now + delay, call: callback}
	c.timers = append(c.timers, timer)
	return func() { timer.stopped = true }
}

// advance fires due timers once on the test's UI thread.
func (c *testChangeClock) advance(delay time.Duration) {
	c.now += delay
	for _, timer := range c.timers {
		if !timer.stopped && timer.at <= c.now {
			timer.stopped = true
			timer.call()
		}
	}
}

// debounceForm keeps field identity fixed while changing producer-owned state.
func debounceForm(t *testing.T, value string, revision, delay int, disabled, formDisabled bool, handler string) *Tree {
	t.Helper()
	return mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"form","props":{"disabled":%t,"onSubmit":{"$handler":"save"}},"children":[
		{"type":"text-field","key":"draft","props":{"name":"draft","value":%q,"revision":%d,"changeDebounce":%d,"disabled":%t,"onChange":{"$handler":%q}}},
		{"type":"text-field","key":"other"}
	]}}`, formDisabled, value, revision, delay, disabled, handler))
}

// TestTextChangeDebounceTrailingEdgeAndSubmit verifies timing without sleeping, plus event ordering and local values.
func TestTextChangeDebounceTrailingEdgeAndSubmit(t *testing.T) {
	v := NewView()
	clock := &testChangeClock{}
	v.scheduleChange = clock.schedule
	v.SetTree(debounceForm(t, "", 0, 200, false, false, "edit"))
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	render := func() { renderLayout(host, 250, 150, 1.5) }
	render()
	id := "woxview/form0/k:draft/text-field"
	host.FocusAutomationID(id + "/field")
	host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "a"})
	clock.advance(100 * time.Millisecond)
	host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "b"})
	clock.timers[0].call()
	if len(events) != 0 || v.formValues(v.Tree().Root, "woxview/form0")["draft"] != "ab" {
		t.Fatal("debounce delayed the local value or emitted an obsolete timer")
	}
	// A stale producer echo must not cancel delivery of the newer local edit.
	v.SetTree(debounceForm(t, "a", 0, 200, false, false, "edit"))
	render()
	clock.advance(199 * time.Millisecond)
	if len(events) != 0 {
		t.Fatal("change emitted before trailing edge")
	}
	clock.advance(time.Millisecond)
	if len(events) != 1 || events[0].args != `["ab"]` {
		t.Fatalf("trailing change: %+v", events)
	}
	host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "c"})
	host.FocusAutomationID("woxview/form0/k:other/text-field/field")
	if len(events) != 2 || events[1].args != `["abc"]` {
		t.Fatalf("blur did not flush: %+v", events)
	}
	clock.advance(time.Second)
	if len(events) != 2 {
		t.Fatal("blur flushed more than once")
	}
	host.FocusAutomationID(id + "/field")
	host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "d"})
	host.Key(ui.KeyEvent{Key: ui.KeyEnter, Down: true})
	if len(events) != 4 || events[2].handler != "edit" || events[2].args != `["abcd"]` || events[3].handler != "save" || events[3].args != `[{"draft":"abcd"}]` {
		t.Fatalf("submit ordering/snapshot: %+v", events)
	}
	clock.advance(time.Second)
	if len(events) != 4 {
		t.Fatal("late change arrived after submit")
	}
}

// TestTextChangeDebounceRevokesStaleDelivery covers cancellation before any rebuilding frame.
func TestTextChangeDebounceRevokesStaleDelivery(t *testing.T) {
	cases := []struct {
		name string
		next func(*testing.T) *Tree
	}{
		{"revision", func(t *testing.T) *Tree { return debounceForm(t, "", 1, 200, false, false, "edit") }},
		{"disabled", func(t *testing.T) *Tree { return debounceForm(t, "", 0, 200, true, false, "edit") }},
		{"form-disabled", func(t *testing.T) *Tree { return debounceForm(t, "", 0, 200, false, true, "edit") }},
		{"handler", func(t *testing.T) *Tree { return debounceForm(t, "", 0, 200, false, false, "new-handler") }},
		{"delay", func(t *testing.T) *Tree { return debounceForm(t, "", 0, 0, false, false, "edit") }},
		{"authoritative-value", func(t *testing.T) *Tree { return debounceForm(t, "server", 0, 200, false, false, "edit") }},
		{"removed", func(t *testing.T) *Tree {
			return mustParse(t, `{"version":1,"root":{"type":"text","props":{"text":"gone"}}}`)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := NewView()
			clock := &testChangeClock{}
			v.scheduleChange = clock.schedule
			v.SetTree(debounceForm(t, "", 0, 200, false, false, "edit"))
			var events []recordedEvent
			host := layoutHost(v, &events)
			defer host.Dispose()
			renderLayout(host, 250, 150, 2)
			host.FocusAutomationID("woxview/form0/k:draft/text-field/field")
			host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "local"})
			v.SetTree(tc.next(t))
			clock.timers[0].call()
			clock.advance(time.Second)
			if len(events) != 0 || len(v.changes) != 0 {
				t.Fatalf("stale delivery: %+v", events)
			}
		})
	}
}

// TestTextChangeDebounceVirtualizationAndDispose distinguishes row unmount from document teardown.
func TestTextChangeDebounceVirtualizationAndDispose(t *testing.T) {
	var rows []string
	for i := 0; i < 100; i++ {
		rows = append(rows, fmt.Sprintf(`{"type":"text-field","key":"%d","props":{"changeDebounce":200,"onChange":{"$handler":"edit"}}}`, i))
	}
	v := NewView()
	clock := &testChangeClock{}
	v.scheduleChange = clock.schedule
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"lazy-list","props":{"itemHeight":40,"style":{"height":100}},"children":[`+strings.Join(rows, ",")+`]}}`))
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	renderLayout(host, 250, 150, 1)
	id := "woxview/lazy-list0/k:0/text-field"
	host.FocusAutomationID(id + "/field")
	host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "offscreen"})
	host.Pointer(ui.PointerEvent{Kind: ui.PointerScroll, Position: ui.Point{X: 10, Y: 10}, Scroll: ui.Point{Y: -1500}})
	renderLayout(host, 250, 150, 2)
	clock.advance(time.Second)
	if hasAutomation(host, id+"/field") || len(events) != 1 || events[0].args != `["offscreen"]` || v.fieldValues[id] != "offscreen" {
		t.Fatalf("virtualized edit lost: %+v", events)
	}
	_ = v.EnqueueCommand(host, CommandFocus, "0", nil)
	drainCommands(t, v, host, 1.5)
	host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "discard"})
	if len(v.changes) != 1 {
		t.Fatal("missing pending edit before disposal")
	}
	host.Dispose()
	for _, timer := range clock.timers {
		timer.call()
	}
	if len(events) != 1 || len(v.changes) != 0 {
		t.Fatal("disposed document delivered a delayed change")
	}
}

// TestTextChangeDebounceValidation keeps JSON, Go construction, and schema bounds aligned.
func TestTextChangeDebounceValidation(t *testing.T) {
	for _, delay := range []int{-1, 60001} {
		if _, err := NewTree(&Node{Type: NodeTextField, Props: &TextFieldProps{ChangeDebounce: delay}}); err == nil {
			t.Fatalf("accepted Go changeDebounce %d", delay)
		}
	}
	for _, value := range []string{"-1", "60001", "0.5", `"200"`, "true"} {
		if _, err := Parse([]byte(`{"version":1,"root":{"type":"text-field","props":{"changeDebounce":` + value + `}}}`)); err == nil {
			t.Fatalf("accepted changeDebounce %s", value)
		}
	}
	for _, delay := range []int{0, 1, 200, 60000} {
		_ = debounceForm(t, "", 0, delay, false, false, "edit")
	}
}
