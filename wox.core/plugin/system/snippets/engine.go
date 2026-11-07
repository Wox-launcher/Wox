package snippets

import (
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

type inputKind uint8

const (
	inputText inputKind = iota
	inputBackspace
	inputModifier
	inputInvalidate
)

// inputEvent is a physical input notification, never a synthesized Wox event.
type inputEvent struct {
	kind   inputKind
	down   bool
	key    uint32
	target uint64
	text   string
}

// replacement holds only the trigger and target needed by the asynchronous worker.
type replacement struct {
	sequence uint64
	target   uint64
	key      uint32
	record   Record
	undo     bool
	deletes  int
}

type undoState struct {
	target  uint64
	trigger string
	count   int
}

// engine serializes short-lived matching state shared by native callbacks and the worker.
type engine struct {
	mu       sync.Mutex
	matcher  matcher
	sequence uint64
	target   uint64
	pending  *replacement
	undo     *undoState
}

// reload invalidates queued work before installing a fresh snapshot of the records.
func (e *engine) reload(records []Record) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sequence++
	e.pending = nil
	e.undo = nil
	e.matcher.replace(records)
}

// invalidate discards pending work when input events cannot be delivered safely.
func (e *engine) invalidate() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sequence++
	e.pending = nil
	e.undo = nil
	e.matcher.reset()
}

// onInput waits for the trigger key release before returning work so the target receives the key first.
func (e *engine) onInput(event inputEvent) *replacement {
	e.mu.Lock()
	defer e.mu.Unlock()
	if event.target == 0 || event.target != e.target {
		e.sequence++
		e.target = event.target
		e.pending = nil
		e.undo = nil
		e.matcher.reset()
	}
	if event.target == 0 {
		return nil
	}
	if !event.down {
		if e.pending == nil || event.key != e.pending.key {
			return nil
		}
		work := *e.pending
		e.pending = nil
		return &work
	}
	if event.kind == inputModifier {
		return nil
	}
	e.sequence++
	e.pending = nil
	if event.kind == inputBackspace && e.undo != nil && e.undo.target == event.target {
		// The user's Backspace has already removed one editing unit.
		count := e.undo.count - 1
		if count >= 0 {
			e.pending = &replacement{sequence: e.sequence, target: event.target, key: event.key,
				record: Record{Text: e.undo.trigger}, deletes: count, undo: true}
		}
		e.undo = nil
		e.matcher.reset()
		return nil
	}
	e.undo = nil
	switch event.kind {
	case inputText:
		if event.text == "" {
			e.matcher.reset()
			return nil
		}
		if record, ok := e.matcher.input(event.text); ok {
			e.pending = &replacement{sequence: e.sequence, target: event.target, key: event.key, record: record, deletes: len(record.Keyword)}
		}
	case inputBackspace:
		e.matcher.backspace()
	default:
		e.matcher.reset()
	}
	return nil
}

// valid checks whether later physical input or a record reload cancelled queued work.
func (e *engine) valid(work replacement) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sequence == work.sequence && e.target == work.target
}

// completed records undo only while the triggering context is still current.
func (e *engine) completed(work replacement, expanded string, codepointBackspace bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	// Unknown controls may delete a whole grapheme or a single code point.
	// Only use code-point counts for a control whose behavior was verified.
	count := uniseg.GraphemeClusterCount(expanded)
	runes := utf8.RuneCountInString(expanded)
	if codepointBackspace {
		count = utf8.RuneCountInString(strings.ReplaceAll(expanded, "\r\n", "\n"))
	} else if count != runes {
		return
	}
	if !work.undo && e.sequence == work.sequence && e.target == work.target && expanded != "" {
		e.undo = &undoState{target: work.target, trigger: work.record.Keyword, count: count}
	}
}
