package woxview

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// drainCommands models the frames requested by the command queue and scrollers.
func drainCommands(t *testing.T, v *View, host *widget.Host, scale float32) {
	t.Helper()
	for i := 0; i < 150 && len(v.commands) > 0; i++ {
		renderLayout(host, 250, 150, scale)
	}
	renderLayout(host, 250, 150, scale)
	if len(v.commands) > 0 {
		t.Fatal("command queue did not settle")
	}
}

// TestCommandRunsThroughCachedBoundary checks imperative work when immutable tree props stay equal.
func TestCommandRunsThroughCachedBoundary(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"text-field","key":"draft","props":{"value":"old"}}}`))
	builds := 0
	onBuild := func() { builds++ }
	onEvent := func(HandlerID, json.RawMessage) {}
	host := widget.NewHost(func(info ui.FrameInfo) widget.Widget {
		child := v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height, OnEvent: onEvent})
		return widget.Boundary[cachedFormProps]{Key: "cached", Props: cachedFormProps{child, v.Tree(), onBuild}, Build: func(p cachedFormProps) widget.Widget { p.onBuild(); return p.child }}
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	renderLayout(host, 250, 150, 1)
	renderLayout(host, 250, 150, 1)
	before := builds
	renderLayout(host, 250, 150, 1)
	if builds != before {
		t.Fatal("fixture did not hit Boundary cache")
	}
	if err := v.EnqueueCommand(host, CommandSelectAll, "draft", nil); err != nil {
		t.Fatal(err)
	}
	drainCommands(t, v, host, 1)
	host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "new"})
	if v.fieldValues["woxview/k:draft/text-field"] != "new" {
		t.Fatal("cached boundary skipped command selection")
	}
}

// TestCommandReevaluatesLazyIndex follows the key rather than its former row position.
func TestCommandReevaluatesLazyIndex(t *testing.T) {
	v := NewView()
	v.SetTree(lazyForm(t, 0, "", 100))
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	renderLayout(host, 250, 150, 1)
	if err := v.EnqueueCommand(host, CommandFocus, "99", nil); err != nil {
		t.Fatal(err)
	}
	var rows []string
	for i := 99; i >= 0; i-- {
		rows = append(rows, fmt.Sprintf(`{"type":"text-field","key":"%d"}`, i))
	}
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"form","children":[{"type":"lazy-list","key":"rows","props":{"itemHeight":40,"style":{"width":200,"height":100}},"children":[`+strings.Join(rows, ",")+`]}]}}`))
	drainCommands(t, v, host, 2)
	if host.FocusedKey() != "woxview/form0/k:rows/lazy-list/k:99/text-field/field" || v.scrolls["woxview/form0/k:rows/lazy-list"].Offset() != 0 {
		t.Fatalf("reordered command: %s", host.FocusedKey())
	}
}

// TestCommandFocusCallbackCanCancelItsDocument guards reentrant embedders during focus notification.
func TestCommandFocusCallbackCanCancelItsDocument(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"text-field","key":"draft","props":{"onFocusChange":{"$handler":"focus"}}}}`))
	notified := 0
	host := widget.NewHost(func(info ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height, OnNodeEvent: func(Event) { notified++; v.CancelPending() }})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	if err := v.EnqueueCommand(host, CommandFocus, "draft", nil); err != nil {
		t.Fatal(err)
	}
	drainCommands(t, v, host, 1)
	if notified != 1 {
		t.Fatalf("focus notification count = %d", notified)
	}
}

// TestCommandsFocusSelectAndRejectInvalidTargets exercises real editor selection and FIFO execution.
func TestCommandsFocusSelectAndRejectInvalidTargets(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","key":"group","children":[
		{"type":"text-field","key":"disabled","props":{"disabled":true}},
		{"type":"text-field","key":"draft","props":{"value":"Hello 世界🙂"}},
		{"type":"text","key":"label","props":{"text":"Label"}}
	]}}`))
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	var completed []Command
	for _, command := range []Command{CommandFocus, CommandSelectAll} {
		if err := v.EnqueueCommand(host, command, "group", func(err error) {
			if err != nil {
				t.Fatal(err)
			}
			completed = append(completed, command)
		}); err != nil {
			t.Fatal(err)
		}
	}
	drainCommands(t, v, host, 1.5)
	if fmt.Sprint(completed) != "[focus selectAll]" {
		t.Fatalf("command order: %v", completed)
	}
	host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "replaced"})
	if v.fieldValues["woxview/k:group/column/k:draft/text-field"] != "replaced" {
		t.Fatalf("selectAll did not replace complete Unicode text: %v", v.fieldValues)
	}
	for _, key := range []string{"disabled", "label"} {
		for _, command := range []Command{CommandFocus, CommandSelectAll} {
			var failure error
			if err := v.EnqueueCommand(host, command, key, func(err error) { failure = err }); err != nil {
				t.Fatal(err)
			}
			drainCommands(t, v, host, 2)
			if failure == nil {
				t.Fatalf("accepted %s on %s", command, key)
			}
		}
	}
	for _, tc := range []struct {
		command Command
		key     string
	}{{"unknown", "draft"}, {CommandFocus, ""}, {CommandFocus, "missing"}} {
		if v.EnqueueCommand(host, tc.command, tc.key, nil) == nil {
			t.Fatalf("accepted invalid command %+v", tc)
		}
	}
}

// TestCommandsMaterializeNestedLazyRows ensures offscreen descendants are reachable at mixed scales.
func TestCommandsMaterializeNestedLazyRows(t *testing.T) {
	var outer []string
	for i := 0; i < 30; i++ {
		var inner []string
		for j := 0; j < 20; j++ {
			inner = append(inner, fmt.Sprintf(`{"type":"text-field","key":"%d/%d","props":{"value":"row","style":{"height":32}}}`, i, j))
		}
		outer = append(outer, fmt.Sprintf(`{"type":"lazy-list","key":"list%d","props":{"itemHeight":40,"style":{"height":80}},"children":[%s]}`, i, strings.Join(inner, ",")))
	}
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"lazy-list","key":"outer","props":{"itemHeight":100,"style":{"height":120}},"children":[`+strings.Join(outer, ",")+`]}}`))
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	for i, scale := range []float32{1, 1.5, 2, 1} {
		key, id := "29/19", "woxview/k:outer/lazy-list/k:list29/lazy-list/k:29%2F19/text-field/field"
		if i%2 != 0 {
			key, id = "0/0", "woxview/k:outer/lazy-list/k:list0/lazy-list/k:0%2F0/text-field/field"
		}
		if err := v.EnqueueCommand(host, CommandScrollTo, key, func(err error) {
			if err != nil {
				t.Fatal(err)
			}
		}); err != nil {
			t.Fatal(err)
		}
		drainCommands(t, v, host, scale)
		bounds := automationBounds(t, host, id)
		if bounds.Y < 0 || bounds.Y+bounds.Height > 120 || host.FocusedKey() != "" {
			t.Fatalf("scrollTo geometry/focus at %v: %+v, %s", scale, bounds, host.FocusedKey())
		}
	}
	if err := v.EnqueueCommand(host, CommandSelectAll, "29/19", func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
	drainCommands(t, v, host, 2)
	host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "new"})
	if v.fieldValues["woxview/k:outer/lazy-list/k:list29/lazy-list/k:29%2F19/text-field"] != "new" || len(v.fieldStates) > 60 {
		t.Fatal("virtualized selection failed or built all rows")
	}
}

// TestCommandIdentityAndCapacity rejects ambiguous and replaced targets without redirecting input.
func TestCommandIdentityAndCapacity(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","children":[{"type":"text-field","key":"x"},{"type":"column","children":[{"type":"text-field","key":"x"}]}]}}`))
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	if err := v.EnqueueCommand(host, CommandFocus, "x", nil); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguous target accepted: %v", err)
	}
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"text-field","key":"x"}}`))
	for i := 0; i < 64; i++ {
		if err := v.EnqueueCommand(host, CommandFocus, "x", nil); err != nil {
			t.Fatal(err)
		}
	}
	if v.EnqueueCommand(host, CommandFocus, "x", nil) == nil {
		t.Fatal("unbounded command queue")
	}
	drainCommands(t, v, host, 1)
	var failure error
	_ = v.EnqueueCommand(host, CommandFocus, "x", func(err error) { failure = err })
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","children":[{"type":"text-field","key":"x"}]}}`))
	drainCommands(t, v, host, 1)
	if failure == nil || !strings.Contains(failure.Error(), "replaced") {
		t.Fatalf("redirected replaced target: %v", failure)
	}
	failure = nil
	_ = v.EnqueueCommand(host, CommandFocus, "x", func(err error) { failure = err })
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"text","props":{"text":"removed"}}}`))
	drainCommands(t, v, host, 1)
	if failure == nil || !strings.Contains(failure.Error(), "not found") {
		t.Fatalf("removed target command: %v", failure)
	}
}
