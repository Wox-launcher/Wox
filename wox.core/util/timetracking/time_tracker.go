package timetracking

import (
	"context"
	"strconv"
	"strings"
	"time"
	"wox/util"
)

type valueKind int

const (
	rawString valueKind = iota
	quotedString
	intValue
	int64Value
	boolValue
)

type field struct {
	name       string
	kind       valueKind
	rawValue   string
	intValue   int
	int64Value int64
	boolValue  bool
}

// TimeTracker keeps development-only timing fields in memory and writes one
// aggregate log line when the measured scope is complete.
type TimeTracker struct {
	enabled bool
	stage   string
	fields  []field
	indexes map[string]int
}

var disabledTimeTracker = &TimeTracker{}

// New creates a no-op tracker outside development mode so callers can leave
// timing calls in hot paths without production logging or string building.
func New(stage string) *TimeTracker {
	if !util.IsDev() {
		return disabledTimeTracker
	}

	return &TimeTracker{
		enabled: true,
		stage:   stage,
		indexes: map[string]int{},
	}
}

func (t *TimeTracker) Enabled() bool {
	return t != nil && t.enabled
}

// Start returns a timing boundary only when tracking is active.
func (t *TimeTracker) Start() time.Time {
	if !t.Enabled() {
		return time.Time{}
	}
	return time.Now()
}

// StartMs returns a millisecond timing boundary only when tracking is active.
func (t *TimeTracker) StartMs() int64 {
	if !t.Enabled() {
		return 0
	}
	return util.GetSystemTimestamp()
}

// SetElapsedMs records elapsed milliseconds since start.
func (t *TimeTracker) SetElapsedMs(name string, start int64) {
	if !t.Enabled() || start == 0 {
		return
	}
	t.SetInt64(name, util.GetSystemTimestamp()-start)
}

// SetElapsedUs records elapsed microseconds since start.
func (t *TimeTracker) SetElapsedUs(name string, start time.Time) {
	if !t.Enabled() || start.IsZero() {
		return
	}
	t.SetInt64(name, time.Since(start).Microseconds())
}

// AddElapsedUs accumulates elapsed microseconds since start.
func (t *TimeTracker) AddElapsedUs(name string, start time.Time) {
	if !t.Enabled() || start.IsZero() {
		return
	}
	t.AddInt64(name, time.Since(start).Microseconds())
}

// SetRawString records a string value without quoting for existing log parser compatibility.
func (t *TimeTracker) SetRawString(name string, value string) {
	if !t.Enabled() {
		return
	}
	t.set(field{name: name, kind: rawString, rawValue: value})
}

// SetString records a quoted string value.
func (t *TimeTracker) SetString(name string, value string) {
	if !t.Enabled() {
		return
	}
	t.set(field{name: name, kind: quotedString, rawValue: value})
}

// SetInt records an integer field.
func (t *TimeTracker) SetInt(name string, value int) {
	if !t.Enabled() {
		return
	}
	t.set(field{name: name, kind: intValue, intValue: value})
}

// AddInt accumulates an integer field.
func (t *TimeTracker) AddInt(name string, delta int) {
	if !t.Enabled() {
		return
	}
	if index, ok := t.indexes[name]; ok {
		if t.fields[index].kind == intValue {
			t.fields[index].intValue += delta
			return
		}
		if t.fields[index].kind == int64Value {
			t.fields[index].int64Value += int64(delta)
			return
		}
		t.fields[index] = field{name: name, kind: intValue, intValue: delta}
		return
	}
	t.SetInt(name, delta)
}

// SetInt64 records an int64 field.
func (t *TimeTracker) SetInt64(name string, value int64) {
	if !t.Enabled() {
		return
	}
	t.set(field{name: name, kind: int64Value, int64Value: value})
}

// AddInt64 accumulates an int64 field.
func (t *TimeTracker) AddInt64(name string, delta int64) {
	if !t.Enabled() {
		return
	}
	if index, ok := t.indexes[name]; ok {
		if t.fields[index].kind == int64Value {
			t.fields[index].int64Value += delta
			return
		}
		if t.fields[index].kind == intValue {
			t.fields[index].intValue += int(delta)
			return
		}
		t.fields[index] = field{name: name, kind: int64Value, int64Value: delta}
		return
	}
	t.SetInt64(name, delta)
}

// SetBool records a boolean field.
func (t *TimeTracker) SetBool(name string, value bool) {
	if !t.Enabled() {
		return
	}
	t.set(field{name: name, kind: boolValue, boolValue: value})
}

// Log writes the accumulated fields as a single query_timing line in insertion order.
func (t *TimeTracker) Log(ctx context.Context) {
	if !t.Enabled() {
		return
	}

	var builder strings.Builder
	builder.Grow(96 + len(t.fields)*24)
	builder.WriteString("query_timing stage=")
	builder.WriteString(t.stage)
	if traceId := util.GetContextTraceId(ctx); traceId != "" {
		if _, exists := t.indexes["traceId"]; !exists {
			builder.WriteString(" traceId=")
			builder.WriteString(traceId)
		}
	}
	for _, field := range t.fields {
		builder.WriteByte(' ')
		builder.WriteString(field.name)
		builder.WriteByte('=')
		builder.WriteString(field.format())
	}
	util.GetLogger().Debug(ctx, builder.String())
}

func (t *TimeTracker) set(field field) {
	if index, ok := t.indexes[field.name]; ok {
		t.fields[index] = field
		return
	}
	t.indexes[field.name] = len(t.fields)
	t.fields = append(t.fields, field)
}

func (f field) format() string {
	switch f.kind {
	case rawString:
		return f.rawValue
	case quotedString:
		return strconv.Quote(f.rawValue)
	case intValue:
		return strconv.Itoa(f.intValue)
	case int64Value:
		return strconv.FormatInt(f.int64Value, 10)
	case boolValue:
		return strconv.FormatBool(f.boolValue)
	default:
		return f.rawValue
	}
}
