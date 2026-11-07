//go:build linux && cgo

package snippets

/*
#cgo LDFLAGS: -lX11 -lXi -lXtst -lxkbcommon
#include <stdint.h>
typedef struct SnippetX11 SnippetX11;
SnippetX11 *snippetX11Open(int *failure);
void snippetX11Poll(SnippetX11 *state, uintptr_t callback);
void snippetX11Close(SnippetX11 *state);
uint64_t snippetX11Target(SnippetX11 *state);
int snippetX11Ready(SnippetX11 *state);
uint64_t snippetX11ClipboardOwner(SnippetX11 *state);
int snippetX11IsTerminal(SnippetX11 *state);
int snippetX11ModifiersHeld(SnippetX11 *state);
int snippetX11DeleteAndPaste(SnippetX11 *state, unsigned int count, int terminal, uint64_t expectedOwner);
*/
import "C"

import (
	"context"
	"errors"
	"runtime/cgo"
	"sync"
	"time"

	"wox/util/clipboard"
)

type x11ExpansionBackend struct {
	mu       sync.Mutex
	state    *C.SnippetX11
	focus    *accessibilityFocus
	callback C.uintptr_t
}

func newX11ExpansionBackend() expansionBackend { return &x11ExpansionBackend{} }

// start selects non-grabbing XInput2 events on a dedicated connection.
func (b *x11ExpansionBackend) start(callback func(inputEvent)) (func() error, error) {
	focus, err := startAccessibilityFocus(func() { callback(inputEvent{kind: inputInvalidate, down: true}) })
	if err != nil {
		return nil, err
	}
	b.focus = focus
	var failure C.int
	b.mu.Lock()
	b.state = C.snippetX11Open(&failure)
	b.mu.Unlock()
	if b.state == nil {
		focus.close()
		if failure == 2 {
			return nil, errXTestUnavailable
		}
		return nil, errInputUnavailable
	}
	handle := cgo.NewHandle(callback)
	b.mu.Lock()
	b.callback = C.uintptr_t(handle)
	b.mu.Unlock()
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			b.mu.Lock()
			b.callback = 0
			b.mu.Unlock()
			handle.Delete()
		}()
		for {
			select {
			case <-stop:
				return
			default:
			}
			b.mu.Lock()
			C.snippetX11Poll(b.state, C.uintptr_t(handle))
			b.mu.Unlock()
		}
	}()
	var once sync.Once
	return func() error {
		once.Do(func() {
			close(stop)
			<-done
			focus.close()
			b.mu.Lock()
			C.snippetX11Close(b.state)
			b.state = nil
			b.mu.Unlock()
		})
		return nil
	}, nil
}

//export woxSnippetX11Event
func woxSnippetX11Event(callback C.uintptr_t, kind C.int, down C.int, key C.uint, target C.uint64_t, character C.uint) {
	handle := cgo.Handle(callback).Value().(func(inputEvent))
	event := inputEvent{kind: inputKind(kind), down: down != 0, key: uint32(key), target: uint64(target)}
	if character != 0 {
		event.text = string(rune(character))
	}
	handle(event)
}

func (b *x11ExpansionBackend) target() uint64 {
	if b.focus == nil || b.focus.verifiedTarget() == 0 {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == nil {
		return 0
	}
	return uint64(C.snippetX11Target(b.state))
}

func (b *x11ExpansionBackend) health() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if C.snippetX11Ready(b.state) == 0 {
		return errXTestUnavailable
	}
	return nil
}

// replace uses an XTest keyboard and restores the previous selection only while Wox still owns it.
func (b *x11ExpansionBackend) replace(ctx context.Context, target uint64, deletes int, text string, checks replacementChecks) error {
	deadline := time.Now().Add(time.Second)
	for {
		b.mu.Lock()
		held := b.state != nil && C.snippetX11ModifiersHeld(b.state) != 0
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
	b.mu.Lock()
	owner := C.snippetX11ClipboardOwner(b.state)
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		owns := b.state != nil && owner != 0 && C.snippetX11ClipboardOwner(b.state) == owner
		b.mu.Unlock()
		if owns && previous != nil {
			_ = clipboard.Write(previous)
		}
	}()
	focus := b.focus.fast()
	if focus == 0 || !checks.valid() || b.focus.fast() != focus {
		return errors.New("input context changed")
	}
	b.mu.Lock()
	if b.state == nil || b.callback == 0 {
		b.mu.Unlock()
		return errors.New("input monitoring stopped")
	}
	// Drain queued input before checking the generation while the listener is locked.
	C.snippetX11Poll(b.state, b.callback)
	if !checks.current() || b.focus.fast() != focus || uint64(C.snippetX11Target(b.state)) != target {
		b.mu.Unlock()
		return errors.New("input context changed")
	}
	terminal := C.snippetX11IsTerminal(b.state)
	ok := C.snippetX11DeleteAndPaste(b.state, C.uint(deletes), terminal, owner)
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
