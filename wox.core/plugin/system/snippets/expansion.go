package snippets

import (
	"context"
	"errors"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/rivo/uniseg"

	"wox/util"
	"wox/util/clipboard"
)

var (
	errFocusUnavailable     = errors.New("accessibility focus is unavailable")
	errInputUnavailable     = errors.New("keyboard input access is unavailable")
	errInjectionUnavailable = errors.New("virtual keyboard access is unavailable")
	errLayoutUnavailable    = errors.New("keyboard layout is unavailable")
	errXTestUnavailable     = errors.New("X11 virtual keyboard is unavailable")
)

type expansionIssue string

const (
	expansionIssueNone      expansionIssue = ""
	expansionIssueFocus     expansionIssue = "focus"
	expansionIssueInput     expansionIssue = "input"
	expansionIssueInjection expansionIssue = "injection"
	expansionIssueLayout    expansionIssue = "layout"
	expansionIssueXTest     expansionIssue = "xtest"
	expansionIssueOther     expansionIssue = "other"
)

func classifyExpansionIssue(err error) expansionIssue {
	switch {
	case err == nil:
		return expansionIssueNone
	case errors.Is(err, errFocusUnavailable):
		return expansionIssueFocus
	case errors.Is(err, errInputUnavailable):
		return expansionIssueInput
	case errors.Is(err, errInjectionUnavailable):
		return expansionIssueInjection
	case errors.Is(err, errLayoutUnavailable):
		return expansionIssueLayout
	case errors.Is(err, errXTestUnavailable):
		return expansionIssueXTest
	default:
		return expansionIssueOther
	}
}

// expansionBackend owns platform input observation and focused-target replacement.
type expansionBackend interface {
	start(func(inputEvent)) (func() error, error)
	target() uint64
	replace(context.Context, uint64, int, string, replacementChecks) error
}

// replacementChecks separates the potentially blocking focus query from the in-memory input generation check.
type replacementChecks struct {
	valid   func() bool
	current func() bool
	applied func()
	trigger string
}

// expansion coordinates a single worker and invalidates stale requests on every physical input.
type expansion struct {
	engine  engine
	backend expansionBackend
	jobs    chan replacement
	cancel  context.CancelFunc
	stop    func() error
	done    chan struct{}
	once    sync.Once
}

// startExpansion subscribes before launching the worker so startup failures leave no running session.
func startExpansion(ctx context.Context, backend expansionBackend, records []Record) (*expansion, error) {
	ctx, cancel := context.WithCancel(ctx)
	s := &expansion{backend: backend, jobs: make(chan replacement, 1), cancel: cancel, done: make(chan struct{})}
	s.engine.reload(records)
	stop, err := backend.start(func(event inputEvent) {
		if work := s.engine.onInput(event); work != nil {
			select {
			case s.jobs <- *work:
			default:
				s.engine.invalidate()
			}
		}
	})
	if err != nil {
		cancel()
		return nil, err
	}
	s.stop = stop
	util.Go(ctx, "snippet expansion worker", func() {
		defer close(s.done)
		for {
			select {
			case <-ctx.Done():
				return
			case work := <-s.jobs:
				s.run(ctx, work)
			}
		}
	})
	return s, nil
}

// run resolves variables before deleting anything, then rechecks the focused input and sequence.
func (s *expansion) run(ctx context.Context, work replacement) {
	current := func() bool { return ctx.Err() == nil && s.engine.valid(work) }
	valid := func() bool {
		if !current() || s.backend.target() != work.target {
			return false
		}
		// Checking the target can require IPC; physical input during that call must cancel the job.
		return current()
	}
	if !valid() {
		return
	}
	text := work.record.Text
	if !work.undo {
		var err error
		text, err = expandTemplate(text, time.Now(), clipboard.ReadTextRequired)
		if err != nil {
			util.GetLogger().Warn(ctx, "snippet expansion cancelled: clipboard text unavailable")
			return
		}
	}
	if !valid() {
		return
	}
	codepointBackspace := false
	if uniseg.GraphemeClusterCount(text) != utf8.RuneCountInString(text) {
		if backend, ok := s.backend.(interface{ usesCodepointBackspace(uint64) bool }); ok {
			codepointBackspace = backend.usesCodepointBackspace(work.target)
		}
	}
	var completed sync.Once
	applied := func() { completed.Do(func() { s.engine.completed(work, text, codepointBackspace) }) }
	if err := s.backend.replace(ctx, work.target, work.deletes, text, replacementChecks{
		valid: valid, current: current, applied: applied, trigger: work.record.Keyword,
	}); err != nil {
		util.GetLogger().Warn(ctx, "snippet expansion cancelled: "+err.Error())
		return
	}
	applied()
}

// close releases the listener and waits for its worker before the plugin unloads.
func (s *expansion) close() {
	if s == nil {
		return
	}
	s.once.Do(func() {
		s.cancel()
		s.engine.invalidate()
		if s.stop != nil {
			_ = s.stop()
		}
		<-s.done
	})
}
