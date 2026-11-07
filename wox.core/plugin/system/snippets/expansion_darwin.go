package snippets

/*
#cgo CFLAGS: -fblocks
#cgo LDFLAGS: -framework AppKit -framework ApplicationServices -framework Carbon
#include <stdint.h>

int snippetMacTrusted(void);
uint64_t snippetMacTarget(void);
int snippetMacStartFocusObserver(void);
void snippetMacStopFocusObserver(void);
int snippetMacModifiersHeld(void);
int snippetMacBackspaces(unsigned int count);
int snippetMacType(const uint16_t *text, unsigned int length);
int snippetMacPaste(void);
int64_t snippetMacClipboardSequence(void);
*/
import "C"

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf16"

	"wox/util/clipboard"
	"wox/util/keyboard"
)

type macExpansionBackend struct{}

func newExpansionBackend() expansionBackend { return macExpansionBackend{} }

// start observes produced characters without consuming the user's events.
func (macExpansionBackend) start(handle func(inputEvent)) (func() error, error) {
	if C.snippetMacTrusted() == 0 {
		return nil, errFocusUnavailable
	}
	if C.snippetMacStartFocusObserver() == 0 {
		return nil, errFocusUnavailable
	}
	subscription, err := keyboard.AddRawKeyListenerWithReset(func(raw keyboard.RawKeyEvent) bool {
		event := inputEvent{key: raw.NativeKeyCode, down: raw.Type == keyboard.EventTypeKeyDown, target: uint64(C.snippetMacTarget())}
		switch {
		case raw.Key.IsModifier():
			event.kind = inputModifier
		case raw.Modifiers&(keyboard.ModifierCtrl|keyboard.ModifierAlt|keyboard.ModifierSuper) != 0:
			event.kind = inputInvalidate
		case raw.NativeKeyCode == 0x33:
			event.kind = inputBackspace
		case event.down && raw.Text != "":
			event.kind = inputText
			event.text = raw.Text
		default:
			event.kind = inputInvalidate
		}
		handle(event)
		return false
	}, func() { handle(inputEvent{kind: inputInvalidate, down: true}) })
	if err != nil {
		C.snippetMacStopFocusObserver()
		return nil, err
	}
	stopPointer, err := startPointerObserver(func() {
		handle(inputEvent{kind: inputInvalidate, down: true})
	})
	if err != nil {
		_ = subscription.Close()
		C.snippetMacStopFocusObserver()
		return nil, err
	}
	return func() error {
		stopPointer()
		err := subscription.Close()
		C.snippetMacStopFocusObserver()
		return err
	}, nil
}

func (macExpansionBackend) target() uint64 { return uint64(C.snippetMacTarget()) }

// replace tags generated events so Wox's shared event tap does not process them again.
func (macExpansionBackend) replace(ctx context.Context, target uint64, deletes int, text string, checks replacementChecks) error {
	deadline := time.Now().Add(time.Second)
	for C.snippetMacModifiersHeld() != 0 {
		if !checks.valid() || time.Now().After(deadline) {
			return errors.New("input context changed while waiting for modifiers")
		}
		time.Sleep(5 * time.Millisecond)
	}
	var previous clipboard.Data
	var clipboardSequence C.int64_t
	if strings.ContainsAny(text, "\r\n") {
		var err error
		previous, err = clipboard.Read()
		if err != nil && !errors.Is(err, clipboard.NoDataErr()) {
			return errors.New("clipboard cannot be preserved")
		}
		if err := clipboard.WriteText(text); err != nil {
			return errors.New("clipboard write failed")
		}
		clipboardSequence = C.snippetMacClipboardSequence()
		defer func() {
			if previous != nil && C.snippetMacClipboardSequence() == clipboardSequence {
				_ = clipboard.Write(previous)
			}
		}()
	}
	multiline := strings.ContainsAny(text, "\r\n")
	var units []uint16
	if !multiline {
		units = utf16.Encode([]rune(text))
	}
	if !checks.valid() || uint64(C.snippetMacTarget()) != target {
		return errors.New("input context changed")
	}
	if C.snippetMacBackspaces(C.uint(deletes)) == 0 {
		return errors.New("could not delete trigger")
	}
	if multiline {
		if C.snippetMacPaste() == 0 {
			return errors.New("paste failed")
		}
		select {
		case <-ctx.Done():
		case <-time.After(150 * time.Millisecond):
		}
		return nil
	}
	if len(units) > 0 && C.snippetMacType((*C.uint16_t)(&units[0]), C.uint(len(units))) == 0 {
		return errors.New("text input failed")
	}
	return nil
}
