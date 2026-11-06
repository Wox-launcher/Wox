package woxview

import (
	"encoding/json"
	"testing"
	"time"

	ui "wox/ui/runtime"
	"wox/ui/widget"
	"wox/util"
)

// TestReinsertedFieldStartsFresh covers multiple accepted updates before the next frame.
func TestReinsertedFieldStartsFresh(t *testing.T) {
	for _, prefix := range []string{"woxview", "custom"} {
		t.Run(prefix, func(t *testing.T) {
			v := NewView()
			data := `{"version":1,"root":{"type":"text-field","key":"draft","props":{"label":"Draft","value":"initial"}}}`
			v.SetTree(mustParse(t, data))
			host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
				return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, IDPrefix: prefix})
			})
			host.AttachServices(fakeServices{})
			defer host.Dispose()
			if err := v.EnqueueCommand(host, CommandSelectAll, "draft", nil); err != nil {
				t.Fatal(err)
			}
			frame(host)
			host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "old edit"})
			id := prefix + "/k:draft/text-field"
			if v.fieldValues[id] != "old edit" {
				t.Fatal("edit did not reach the field")
			}
			v.SetTree(mustParse(t, data))
			frame(host)
			if v.fieldValues[id] != "old edit" {
				t.Fatal("ordinary prop echo reset the field")
			}
			v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column"}}`))
			v.SetTree(mustParse(t, data))
			frame(host)
			if got := v.fieldValues[id]; got != "initial" {
				t.Fatalf("removed field revived %v", got)
			}
			if v.fieldStates[id] == nil {
				t.Fatal("new field was not registered")
			}
		})
	}
}

// TestCoalescingKeepsDistinctNodeLifetimes covers legal duplicate keys and shared handlers.
func TestCoalescingKeepsDistinctNodeLifetimes(t *testing.T) {
	v := NewView()
	data := `{"version":1,"root":{"type":"column","children":[{"type":"row","key":"a","children":[{"type":"checkbox","key":"field","props":{"onChange":{"$handler":"shared"}}}]},{"type":"row","key":"b","children":[{"type":"checkbox","key":"field","props":{"onChange":{"$handler":"shared"}}}]}]}}`
	v.SetTree(mustParse(t, data))
	queue := util.NewCoalescingQueue(8, Event.CoalescingKey)
	emit := func(parent string, value bool) {
		owner := v.nodes["woxview/column0/k:"+parent+"/row/k:field/checkbox"]
		b := builder{view: v, nodeKey: "field", nodeOwner: owner, eventOwner: owner, props: BuildProps{OnNodeEvent: func(e Event) { queue.Push(e) }}}
		b.emitChange(&Handler{ID: "shared"})(value)
	}
	emit("a", false)
	emit("b", false)
	emit("a", true)
	for _, want := range []string{"[true]", "[false]"} {
		event, ok := queue.Pop()
		if !ok || string(event.Args) != want {
			t.Fatalf("distinct field lost: %+v", event)
		}
		data, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		var wire map[string]any
		if err := json.Unmarshal(data, &wire); err != nil {
			t.Fatal(err)
		}
		if len(wire) != 3 || wire["Key"] != "field" {
			t.Fatalf("internal identity leaked: %s", data)
		}
	}
	emit("a", true)
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column"}}`))
	v.SetTree(mustParse(t, data))
	emit("a", false)
	for _, want := range []string{"[true]", "[false]"} {
		event, ok := queue.Pop()
		if !ok || string(event.Args) != want {
			t.Fatal("new node merged into a retired lifetime")
		}
	}
}

// TestCommandRejectsReinsertedTarget prevents a deferred operation from crossing a lifetime.
func TestCommandRejectsReinsertedTarget(t *testing.T) {
	v := NewView()
	data := `{"version":1,"root":{"type":"text-field","key":"draft","props":{"label":"Draft"}}}`
	v.SetTree(mustParse(t, data))
	var events []recordedEvent
	host := renderView(t, v, &events)
	defer host.Dispose()
	called := false
	if err := v.EnqueueCommand(host, CommandFocus, "draft", func(err error) {
		called = true
		if err == nil {
			t.Error("command accepted a replacement target")
		}
	}); err != nil {
		t.Fatal(err)
	}
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column"}}`))
	v.SetTree(mustParse(t, data))
	frame(host)
	if !called || host.FocusedKey() != "" {
		t.Fatalf("deferred command reached replacement: %s", host.FocusedKey())
	}
}

// TestReinsertedNodesReplayEntry covers both retained block widgets and paragraph-owned span state.
func TestReinsertedNodesReplayEntry(t *testing.T) {
	for _, data := range []string{
		`{"version":1,"root":{"type":"column","children":[{"type":"view","key":"part","props":{"enter":"fade","style":{"width":80,"height":40,"backgroundColor":"#ff0000"}}}]}}`,
		`{"version":1,"root":{"type":"text","children":[{"type":"span","key":"part","props":{"enter":"fade","text":"Entry","style":{"color":"#ff0000"}}}]}}`,
	} {
		v := NewView()
		original := mustParse(t, data)
		v.SetTree(original)
		host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
			return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height})
		})
		host.AttachServices(fakeServices{})
		now := time.Unix(1, 0)
		host.SetAnimationClock(func() time.Time { return now })
		painted := func() bool {
			img := animationFrameImage(t, host, 1)
			for i := 0; i < len(img.Pix); i += 4 {
				if img.Pix[i] > 0 && img.Pix[i+3] > 0 {
					return true
				}
			}
			return false
		}
		if painted() {
			t.Fatal("entry did not begin transparent")
		}
		now = now.Add(time.Second)
		if !painted() {
			t.Fatal("entry did not finish")
		}
		painted()
		empty, err := NewTree(&Node{Type: original.Root.Type})
		if err != nil {
			t.Fatal(err)
		}
		v.SetTree(empty)
		v.SetTree(mustParse(t, data))
		if painted() {
			t.Fatal("replacement inherited a completed entry")
		}
		host.Dispose()
	}
}
