package woxview

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"

	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// TestTimerFormats covers stopwatch, countdown, long sessions and subsecond displays.
func TestTimerFormats(t *testing.T) {
	value := 27*time.Hour + 2*time.Minute + 3*time.Second + 456*time.Millisecond
	for _, tc := range []struct {
		format TimerFormat
		want   string
	}{
		{"", "1622:03"}, {TimerFormatHours, "27"}, {TimerFormatMinutes, "1622"},
		{TimerFormatSeconds, "97323"}, {TimerFormatHoursMinutes, "27:02"},
		{TimerFormatMinutesSeconds, "1622:03"}, {TimerFormatHoursMinutesSeconds, "27:02:03"},
		{TimerFormatSecondsMillis, "97323.456"}, {TimerFormatMinutesSecondsMillis, "1622:03.456"},
		{TimerFormatHoursMinutesSecondsMillis, "27:02:03.456"},
	} {
		if got := timerText(value, tc.format, false); got != tc.want {
			t.Errorf("%q: %s, want %s", tc.format, got, tc.want)
		}
	}
	for _, tc := range []struct {
		value  time.Duration
		format TimerFormat
		down   bool
		want   string
	}{
		{time.Millisecond, "MM:SS", true, "00:01"},
		{time.Millisecond, "MM:SS", false, "00:00"},
		{time.Nanosecond, "SS.SSS", true, "00.001"},
		{time.Nanosecond, "SS.SSS", false, "00.000"},
		{time.Hour + time.Nanosecond, "HH:MM", true, "01:01"},
		{time.Hour + time.Nanosecond, "HH", true, "02"},
		{59*time.Second + time.Nanosecond, "MM:SS", true, "01:00"},
		{0, "HH:MM:SS.SSS", true, "00:00:00.000"},
		{-time.Second, "MM:SS", true, "00:00"},
		{100*time.Hour + time.Second, "HH:MM:SS", false, "100:00:01"},
	} {
		if got := timerText(tc.value, tc.format, tc.down); got != tc.want {
			t.Errorf("%s/%s/down=%t: %s, want %s", tc.value, tc.format, tc.down, got, tc.want)
		}
	}
}

type timerServices struct {
	fakeServices
	measured strings.Builder
}

// MeasureText observes the actual glyphs laid out by the view without depending on platform fonts.
func (s *timerServices) MeasureText(text string, style ui.TextStyle) (ui.TextMetrics, error) {
	s.measured.WriteString(text)
	return s.fakeServices.MeasureText(text, style)
}

type timerFixture struct {
	t        *testing.T
	view     *View
	host     *widget.Host
	services *timerServices
	now      time.Time
	events   []recordedEvent
}

// newTimerFixture runs parsed wire documents on a deterministic native Host clock.
func newTimerFixture(t *testing.T) *timerFixture {
	f := &timerFixture{t: t, view: NewView(), services: &timerServices{}, now: time.Unix(100, 0)}
	f.host = layoutHost(f.view, &f.events)
	f.host.AttachServices(f.services)
	f.host.SetAnimationClock(func() time.Time { return f.now })
	t.Cleanup(f.host.Dispose)
	return f
}

func (f *timerFixture) set(props string) {
	f.t.Helper()
	f.view.SetTree(mustParse(f.t, `{"version":1,"root":{"type":"timer","key":"clock","props":{`+props+`}}}`))
}

// check advances only the native clock; no producer update is needed to change text or emit completion.
func (f *timerFixture) check(after time.Duration, want string, eventCount int) {
	f.t.Helper()
	f.now = f.now.Add(after)
	f.services.measured.Reset()
	renderLayout(f.host, 400, 100, 1.5)
	if !strings.HasSuffix(f.services.measured.String(), want) {
		f.t.Fatalf("laid out %q, want final glyphs %q", f.services.measured.String(), want)
	}
	if len(f.events) != eventCount {
		f.t.Fatalf("events = %+v, want %d", f.events, eventCount)
	}
}

// TestTimerStopwatchControls checks legacy defaults, pause/resume, format changes and replay.
func TestTimerStopwatchControls(t *testing.T) {
	f := newTimerFixture(t)
	f.set(`"running":true,"elapsed":59000,"playbackId":"one"`)
	f.check(0, "00:59", 0)
	f.check(1500*time.Millisecond, "01:00", 0)
	f.set(`"running":true,"elapsed":0,"playbackId":"one","format":"HH:MM:SS","animate":true`)
	f.check(0, "00:01:00", 0)
	f.set(`"running":true,"playbackId":"one","paused":true`)
	f.check(0, "01:00", 0)
	f.check(time.Hour, "01:00", 0)
	f.set(`"running":true,"playbackId":"one"`)
	f.check(0, "01:00", 0)
	f.check(500*time.Millisecond, "01:01", 0)
	f.set(`"running":true,"playbackId":"two","elapsed":1234,"format":"SS.SSS"`)
	f.check(0, "01.234", 0)
	f.check(123*time.Millisecond, "01.357", 0)
	f.set(`"running":false,"elapsed":42000`)
	f.check(0, "00:42", 0)
	f.check(time.Hour, "00:42", 0)
}

// TestTimerCountdownCompletion covers fractional deadlines, single completion and explicit replay.
func TestTimerCountdownCompletion(t *testing.T) {
	f := newTimerFixture(t)
	props := `"running":true,"direction":"down","duration":5500,"onComplete":{"$handler":"done"},"animate":true`
	f.set(props)
	f.check(0, "00:06", 0)
	f.check(500*time.Millisecond, "00:05", 0)
	f.check(4999*time.Millisecond, "00:01", 0)
	f.check(time.Millisecond, "00:00", 1)
	if f.events[0] != (recordedEvent{handler: "done", args: "[]"}) {
		t.Fatalf("completion payload: %+v", f.events)
	}
	f.set(props + `,"format":"HH:MM:SS"`)
	f.check(time.Hour, "00:00:00", 1)
	f.set(props + `,"playbackId":"again","elapsed":5000`)
	f.check(0, "00:01", 1)
	f.check(500*time.Millisecond, "00:00", 2)
}

// TestTimerBoundedVariants exercises a count-up deadline, paused countdown and zero-duration timer.
func TestTimerBoundedVariants(t *testing.T) {
	f := newTimerFixture(t)
	f.set(`"running":true,"direction":"up","duration":1500,"format":"SS.SSS","onComplete":{"$handler":"up"}`)
	f.check(0, "00.000", 0)
	f.check(2*time.Second, "01.500", 1)
	f.set(`"running":true,"direction":"down","duration":10000,"elapsed":1000,"onComplete":{"$handler":"down"}`)
	f.check(0, "00:09", 1)
	f.check(time.Second, "00:08", 1)
	f.set(`"running":true,"direction":"down","duration":10000,"paused":true,"onComplete":{"$handler":"down"}`)
	f.check(0, "00:08", 1)
	f.check(time.Hour, "00:08", 1)
	f.set(`"running":true,"direction":"down","duration":10000,"onComplete":{"$handler":"down"}`)
	f.check(0, "00:08", 1)
	f.check(8*time.Second, "00:00", 2)
	f.set(`"direction":"down","duration":0,"onComplete":{"$handler":"zero"}`)
	f.check(time.Second, "00:00", 2)
	f.set(`"running":true,"direction":"down","duration":0,"paused":true,"onComplete":{"$handler":"zero"}`)
	f.check(0, "00:00", 2)
	f.set(`"running":true,"direction":"down","duration":0,"onComplete":{"$handler":"zero"}`)
	f.check(0, "00:00", 3)
	f.check(time.Second, "00:00", 3)
}

// TestTimerRemovedBeforeDeadline prevents detached clocks from sending late events.
func TestTimerRemovedBeforeDeadline(t *testing.T) {
	f := newTimerFixture(t)
	f.set(`"running":true,"duration":1000,"onComplete":{"$handler":"done"}`)
	f.check(0, "00:00", 0)
	f.view.SetTree(mustParse(t, `{"version":1,"root":{"type":"view"}}`))
	f.now = f.now.Add(2 * time.Second)
	renderLayout(f.host, 400, 100, 2)
	if len(f.events) != 0 {
		t.Fatal("removed timer emitted completion")
	}
}

// TestTimerValidation enforces a reusable bounded wire contract before creating native state.
func TestTimerValidation(t *testing.T) {
	for _, props := range []string{
		`"direction":"backwards"`, `"format":"hh:mm"`, `"direction":"down"`,
		`"direction":"down","duration":null`, `"onComplete":{"$handler":"done"}`,
		`"elapsed":-1`, `"duration":-1`, `"duration":0.5`,
		fmt.Sprintf(`"duration":%d`, maxTimerMilliseconds+1),
		fmt.Sprintf(`"elapsed":%d`, maxTimerMilliseconds+1),
		fmt.Sprintf(`"playbackId":%q`, strings.Repeat("x", 257)),
	} {
		if _, err := Parse([]byte(`{"version":1,"root":{"type":"timer","props":{` + props + `}}}`)); err == nil {
			t.Errorf("accepted %s", props)
		}
	}
	mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"timer","props":{"elapsed":%d,"duration":%d}}}`, maxTimerMilliseconds, maxTimerMilliseconds))
}

// TestTimerRemountBeforeNextFrame gives a new node its own clock even when the old one has not been pruned yet.
func TestTimerRemountBeforeNextFrame(t *testing.T) {
	f := newTimerFixture(t)
	f.set(`"running":true`)
	f.check(0, "00:00", 0)
	f.check(3*time.Second, "00:03", 0)
	f.view.SetTree(mustParse(t, `{"version":1,"root":{"type":"view"}}`))
	f.set(`"running":true`)
	f.check(0, "00:00", 0)
}

// TestTimerCompletionRevokedByReplay rejects an old completion queued earlier in the same frame.
func TestTimerCompletionRevokedByReplay(t *testing.T) {
	view := NewView()
	tree := func(playback string) *Tree {
		return mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"column","children":[
            {"type":"timer","key":"first","props":{"duration":0,"running":true,"onComplete":{"$handler":"first"}}},
            {"type":"timer","key":"second","props":{"duration":0,"running":true,"playbackId":%q,"onComplete":{"$handler":"second"}}}
        ]}}`, playback))
	}
	view.SetTree(tree("one"))
	var events []HandlerID
	host := widget.NewHost(func(frame ui.FrameInfo) widget.Widget {
		return view.Build(BuildProps{Width: frame.Size.Width, Height: frame.Size.Height, OnEvent: func(id HandlerID, _ json.RawMessage) {
			events = append(events, id)
			if id == "first" {
				view.SetTree(tree("two"))
			}
		}})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	renderLayout(host, 400, 100, 1)
	if len(events) != 1 || events[0] != "first" {
		t.Fatalf("stale completion escaped: %v", events)
	}
	renderLayout(host, 400, 100, 1)
	if len(events) != 2 || events[1] != "second" {
		t.Fatalf("new playback lost completion: %v", events)
	}
}

// TestTimerSchemaMatchesRuntime keeps conditional requirements and numeric limits consistent for producers.
func TestTimerSchemaMatchesRuntime(t *testing.T) {
	var schema jsonschema.Schema
	if err := json.Unmarshal(JSONSchema(), &schema); err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, props := range []string{
		`{}`, `{"direction":"up","format":"HH:MM:SS"}`,
		`{"direction":"down","duration":0,"onComplete":{"$handler":"done"}}`,
		`{"direction":"down","duration":1500000,"elapsed":120000,"format":"MM:SS"}`,
		`{"duration":31536000000,"elapsed":31536000000,"format":"SS.SSS"}`,
		`{"direction":"down"}`, `{"direction":"down","duration":null}`,
		`{"onComplete":{"$handler":"done"}}`, `{"onComplete":null}`,
		`{"direction":"backwards"}`, `{"format":"hh:mm:ss"}`, `{"duration":-1}`,
		`{"elapsed":31536000001}`, `{"duration":31536000001}`, `{"duration":1.5}`,
	} {
		wire := []byte(`{"version":1,"root":{"type":"timer","props":` + props + `}}`)
		var document any
		if err := json.Unmarshal(wire, &document); err != nil {
			t.Fatal(err)
		}
		_, parseErr := Parse(wire)
		schemaErr := resolved.Validate(document)
		if (parseErr == nil) != (schemaErr == nil) {
			t.Errorf("%s: Core=%v, Schema=%v", props, parseErr, schemaErr)
		}
	}
}
