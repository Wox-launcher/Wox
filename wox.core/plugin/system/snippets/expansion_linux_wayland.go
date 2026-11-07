//go:build linux && cgo

package snippets

/*
#cgo LDFLAGS: -lwayland-client -lxkbcommon
#include <stdint.h>
typedef struct SnippetWayland SnippetWayland;
SnippetWayland *snippetWaylandOpen(int *failure);
int snippetWaylandPoll(SnippetWayland *state, uintptr_t callback);
void snippetWaylandClose(SnippetWayland *state);
int snippetWaylandModifiersHeld(SnippetWayland *state);
int snippetWaylandHasLayout(SnippetWayland *state);
unsigned int snippetWaylandLayoutCount(SnippetWayland *state);
int snippetWaylandGroup(SnippetWayland *state);
void snippetWaylandClearGroup(SnippetWayland *state);
int snippetWaylandSetGroup(SnippetWayland *state, unsigned int group);
int snippetWaylandDeleteAndPaste(SnippetWayland *state, unsigned int count, int terminal);
*/
import "C"

import (
	"context"
	"errors"
	"runtime/cgo"
	"sync"
	"sync/atomic"
	"time"

	"wox/util"
	"wox/util/clipboard"
)

type waylandExpansionBackend struct {
	mu       sync.Mutex
	state    *C.SnippetWayland
	focus    *accessibilityFocus
	window   *hyprlandFocus
	failed   atomic.Bool
	callback C.uintptr_t
	layout   *kdeLayout
}

type waylandCallback struct {
	backend *waylandExpansionBackend
	handle  func(inputEvent)
}

func newWaylandExpansionBackend() expansionBackend { return &waylandExpansionBackend{} }

// start observes physical devices, the compositor's XKB keymap, and focus.
func (b *waylandExpansionBackend) start(handle func(inputEvent)) (func() error, error) {
	focus, err := startAccessibilityFocus(func() { handle(inputEvent{kind: inputInvalidate, down: true}) })
	if err != nil && !util.IsHyprlandSession() {
		return nil, err
	}
	b.focus = focus
	if util.IsHyprlandSession() {
		b.window, err = startHyprlandFocus(func() {
			focus.reset()
			handle(inputEvent{kind: inputInvalidate, down: true})
		})
		if err != nil && focus == nil {
			return nil, errFocusUnavailable
		}
	}
	var failure C.int
	b.state = C.snippetWaylandOpen(&failure)
	if b.state == nil {
		focus.close()
		b.window.close()
		switch failure {
		case 2:
			return nil, errLayoutUnavailable
		case 3:
			return nil, errInjectionUnavailable
		case 4:
			return nil, errInputUnavailable
		default:
			return nil, errors.New("Wayland input monitoring unavailable")
		}
	}
	if util.IsKDEDesktopSession() {
		b.layout, err = startKDELayout(func(group uint32, known bool) {
			b.applyLayout(group, known, handle)
		})
	}
	if C.snippetWaylandLayoutCount(b.state) > 1 && b.layout == nil {
		C.snippetWaylandClose(b.state)
		b.state = nil
		focus.close()
		b.window.close()
		return nil, errLayoutUnavailable
	}
	callback := cgo.NewHandle(waylandCallback{backend: b, handle: handle})
	b.mu.Lock()
	b.callback = C.uintptr_t(callback)
	b.mu.Unlock()
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			b.mu.Lock()
			b.callback = 0
			b.mu.Unlock()
			callback.Delete()
		}()
		for {
			select {
			case <-stop:
				return
			default:
			}
			b.mu.Lock()
			ok := C.snippetWaylandPoll(b.state, C.uintptr_t(callback)) != 0
			b.mu.Unlock()
			if !ok {
				b.failed.Store(true)
				handle(inputEvent{kind: inputInvalidate, down: true})
				return
			}
		}
	}()
	var once sync.Once
	return func() error {
		once.Do(func() {
			close(stop)
			<-done
			b.layout.close()
			b.window.close()
			focus.close()
			b.mu.Lock()
			C.snippetWaylandClose(b.state)
			b.state = nil
			b.mu.Unlock()
		})
		return nil
	}, nil
}

// applyLayout invalidates buffered input before updating the XKB group.
func (b *waylandExpansionBackend) applyLayout(group uint32, known bool, handle func(inputEvent)) {
	handle(inputEvent{kind: inputInvalidate, down: true})
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == nil || C.snippetWaylandLayoutCount(b.state) <= 1 {
		return
	}
	if !known || C.snippetWaylandSetGroup(b.state, C.uint(group)) == 0 {
		C.snippetWaylandClearGroup(b.state)
	}
}

func (b *waylandExpansionBackend) health() error {
	if b.failed.Load() {
		return errInputUnavailable
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if C.snippetWaylandHasLayout(b.state) == 0 {
		return errLayoutUnavailable
	}
	return nil
}

//export woxSnippetWaylandEvent
func woxSnippetWaylandEvent(callback C.uintptr_t, kind C.int, down C.int, key C.uint, character C.uint) {
	listener := cgo.Handle(callback).Value().(waylandCallback)
	if kind == 4 {
		listener.handle(inputEvent{kind: inputInvalidate, down: true})
		if listener.backend.layout != nil {
			listener.backend.layout.requestRefresh()
		}
		return
	}
	event := inputEvent{kind: inputKind(kind), down: down != 0, key: uint32(key), target: listener.backend.fastTarget()}
	if character != 0 {
		event.text = string(rune(character))
	}
	listener.handle(event)
}

func (b *waylandExpansionBackend) target() uint64 {
	if b.focus == nil && b.window == nil {
		return 0
	}
	b.mu.Lock()
	count, cached := C.snippetWaylandLayoutCount(b.state), C.snippetWaylandGroup(b.state)
	b.mu.Unlock()
	if count > 1 {
		if b.layout == nil {
			return 0
		}
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		group, err := b.layout.current(ctx)
		cancel()
		if err != nil || cached < 0 || uint32(cached) != group {
			b.layout.requestRefresh()
			return 0
		}
	}
	if b.window != nil {
		if b.focus != nil && (b.focus.blocked.Load() || (b.focus.fast() != 0 && b.focus.verifiedTarget() == 0)) {
			return 0
		}
		return b.window.verifiedTarget()
	}
	return b.focus.verifiedTarget()
}

// fastTarget is safe to call from the native input callback without compositor IPC.
func (b *waylandExpansionBackend) fastTarget() uint64 {
	if b.window != nil {
		if b.focus != nil && b.focus.blocked.Load() {
			return 0
		}
		return b.window.fast()
	}
	if b.focus == nil {
		return 0
	}
	return b.focus.fast()
}

// replace checks the focused target before using the virtual keyboard.
func (b *waylandExpansionBackend) replace(ctx context.Context, target uint64, deletes int, text string, checks replacementChecks) error {
	deadline := time.Now().Add(time.Second)
	for {
		b.mu.Lock()
		held := b.state != nil && C.snippetWaylandModifiersHeld(b.state) != 0
		b.mu.Unlock()
		if !held {
			break
		}
		if !checks.valid() || time.Now().After(deadline) {
			return errors.New("input context changed while waiting for modifiers")
		}
		time.Sleep(5 * time.Millisecond)
	}
	previous, err := clipboard.Read()
	if err != nil && !errors.Is(err, clipboard.NoDataErr()) {
		return errors.New("clipboard cannot be preserved")
	}
	if err := clipboard.WriteText(text); err != nil {
		return errors.New("clipboard write failed")
	}
	sequence := clipboard.SequenceNumber()
	defer func() {
		// Some Wayland backends cannot identify the current selection owner. Avoid
		// overwriting a newer user copy when ownership cannot be proven.
		if previous != nil && sequence != 0 && clipboard.SequenceNumber() == sequence {
			_ = clipboard.Write(previous)
		}
	}()
	if !checks.valid() {
		return errors.New("input context changed")
	}
	terminal := 0
	if (b.window != nil && b.window.isTerminal(target)) || (b.focus != nil && b.focus.isTerminal(b.focus.fast())) {
		terminal = 1
	}
	b.mu.Lock()
	if b.state == nil || b.callback == 0 {
		b.mu.Unlock()
		return errors.New("input monitoring stopped")
	}
	// Process queued device events before the final in-memory check; do not wait for IPC while holding the input lock.
	drained := false
	for i := 0; i < 8; i++ {
		status := C.snippetWaylandPoll(b.state, b.callback)
		if status == 0 {
			b.failed.Store(true)
			b.mu.Unlock()
			return errors.New("input monitoring stopped")
		}
		if status == 1 {
			drained = true
			break
		}
	}
	if !drained || !checks.current() || b.fastTarget() != target {
		b.mu.Unlock()
		return errors.New("input context changed")
	}
	ok := C.snippetWaylandDeleteAndPaste(b.state, C.uint(deletes), C.int(terminal))
	b.mu.Unlock()
	if ok == 0 {
		return errors.New("text replacement failed")
	}
	select {
	case <-ctx.Done():
	case <-time.After(150 * time.Millisecond):
	}
	return nil
}
