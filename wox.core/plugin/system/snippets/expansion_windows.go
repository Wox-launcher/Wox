package snippets

/*
#include <windows.h>
#include <imm.h>
#include <stdint.h>
#include <stdlib.h>
#include "focus_windows.h"

#cgo LDFLAGS: -limm32 -lole32 -loleaut32 -luuid -luiautomationcore

int snippetWindowsReplace(unsigned int count, const uint16_t *text, unsigned int length, int paste);
unsigned int snippetCharacter(unsigned int vk, unsigned int scan);
int snippetClipboardStageText(DWORD expected, const uint16_t *text, unsigned int length, const uint8_t *marker);
int snippetClipboardStillStaged(const uint8_t *marker, const uint16_t *text, unsigned int length);
int snippetClipboardIsEmpty(DWORD expected);
int snippetClipboardRestoreEmpty(const uint8_t *marker);
int snippetClipboardRestoreText(const uint8_t *marker, const uint16_t *text, unsigned int length);
int snippetClipboardRestoreFiles(const uint8_t *marker, const uint16_t *paths, unsigned int length);
int snippetClipboardRestoreImage(const uint8_t *marker, const uint8_t *png, unsigned int pngLength, const uint8_t *dib, unsigned int dibLength);

// The focused child is used as the target because the foreground window may
// contain several independent editors.
static uintptr_t snippetTarget(void) {
	HWND foreground = GetForegroundWindow();
	if (!foreground) return 0;
	DWORD thread = GetWindowThreadProcessId(foreground, NULL);
	GUITHREADINFO info = { .cbSize = sizeof(GUITHREADINFO) };
	if (!thread || !GetGUIThreadInfo(thread, &info) || !info.hwndFocus) return 0;
	WCHAR name[32] = {0};
	if (GetClassNameW(info.hwndFocus, name, 32) && lstrcmpiW(name, L"Edit") == 0 &&
		(GetWindowLongPtrW(info.hwndFocus, GWL_STYLE) & ES_PASSWORD)) return 0;
	HIMC imc = ImmGetContext(info.hwndFocus);
	if (imc) {
		BOOL open = ImmGetOpenStatus(imc);
		ImmReleaseContext(info.hwndFocus, imc);
		if (open) return 0;
	}
	return (uintptr_t)info.hwndFocus;
}

static int snippetModifiersHeld(void) {
	return (GetAsyncKeyState(VK_SHIFT) & 0x8000) || (GetAsyncKeyState(VK_CONTROL) & 0x8000) ||
		(GetAsyncKeyState(VK_MENU) & 0x8000) || (GetAsyncKeyState(VK_LWIN) & 0x8000) ||
		(GetAsyncKeyState(VK_RWIN) & 0x8000);
}

static unsigned long snippetClipboardSequence(void) { return GetClipboardSequenceNumber(); }
*/
import "C"

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/google/uuid"
	"golang.org/x/image/bmp"

	"wox/util/clipboard"
	"wox/util/keyboard"
)

type windowsExpansionBackend struct{}

type windowsClipboardRestoreKind uint8

const (
	windowsClipboardRestoreUnavailable windowsClipboardRestoreKind = iota
	windowsClipboardRestoreEmpty
	windowsClipboardRestoreText
	windowsClipboardRestoreFiles
	windowsClipboardRestoreImage
)

func newExpansionBackend() expansionBackend { return windowsExpansionBackend{} }

type windowsInputObserver struct{ deadKeyPending atomic.Bool }

const windowsSnippetInputTag = 0x574f5854

// translate invalidates ambiguous dead-key sequences and ignores injected input.
func (o *windowsInputObserver) translate(raw keyboard.RawKeyEvent, target uint64, character func(uint32, uint32) uint32) (inputEvent, bool) {
	if raw.NativeFlags&0x10 != 0 { // LLKHF_INJECTED
		if raw.NativeExtraInfo == windowsSnippetInputTag {
			return inputEvent{}, false
		}
		return inputEvent{kind: inputInvalidate, down: true}, true
	}
	event := inputEvent{key: raw.NativeKeyCode, down: raw.Type == keyboard.EventTypeKeyDown, target: target}
	if target == 0 {
		o.deadKeyPending.Store(false)
		event.kind = inputInvalidate
		return event, true
	}
	afterDeadKey := event.down && !raw.Key.IsModifier() && o.deadKeyPending.Swap(false)
	switch {
	case raw.Key.IsModifier():
		event.kind = inputModifier
	case afterDeadKey:
		// The foreground application may receive a composed character, not this key's ASCII value.
		event.kind = inputInvalidate
		if character(raw.NativeKeyCode, raw.NativeScanCode) == 0xffffffff {
			o.deadKeyPending.Store(true)
		}
	case raw.Modifiers&(keyboard.ModifierCtrl|keyboard.ModifierAlt|keyboard.ModifierSuper) != 0:
		event.kind = inputInvalidate
	case raw.NativeKeyCode == 0x08:
		event.kind = inputBackspace
	case event.down:
		character := character(raw.NativeKeyCode, raw.NativeScanCode)
		if character == 0xffffffff {
			o.deadKeyPending.Store(true)
			event.kind = inputInvalidate
		} else if character != 0 {
			event.kind = inputText
			event.text = string(rune(character))
		} else {
			event.kind = inputInvalidate
		}
	}
	return event, true
}

// start subscribes to the existing shared hook and ignores synthetic input.
func (windowsExpansionBackend) start(handle func(inputEvent)) (func() error, error) {
	var observer windowsInputObserver
	subscription, err := keyboard.AddRawKeyListener(func(raw keyboard.RawKeyEvent) bool {
		target := uint64(0)
		if raw.NativeFlags&0x10 == 0 {
			target = (windowsExpansionBackend{}).target()
		}
		event, ok := observer.translate(raw, target, func(vk, scan uint32) uint32 {
			return uint32(C.snippetCharacter(C.uint(vk), C.uint(scan)))
		})
		if ok {
			handle(event)
		}
		return false
	})
	if err != nil {
		return nil, err
	}
	stopPointer, err := startPointerObserver(func() {
		// Pointer input only cancels matching. A synchronous UI Automation query here delays mouse delivery.
		handle(inputEvent{kind: inputInvalidate, down: true})
	})
	if err != nil {
		_ = subscription.Close()
		return nil, err
	}
	return func() error {
		stopPointer()
		return subscription.Close()
	}, nil
}

// target fails closed when UI Automation cannot identify a safe focused control.
func (windowsExpansionBackend) target() uint64 {
	target := uint64(C.snippetTarget())
	if target == 0 || C.snippetWindowsFocusedControlSafe() == 0 || uint64(C.snippetTarget()) != target {
		return 0
	}
	return target
}

func (windowsExpansionBackend) usesCodepointBackspace(target uint64) bool {
	return uint64(C.snippetTarget()) == target && C.snippetWindowsUsesCodepointBackspace(C.uint64_t(target)) != 0
}

// replace stages multiline text before deletion and restores the clipboard only if no other writer changed it.
func (windowsExpansionBackend) replace(ctx context.Context, target uint64, deletes int, text string, checks replacementChecks) error {
	deadline := time.Now().Add(time.Second)
	for C.snippetModifiersHeld() != 0 {
		if !checks.valid() || time.Now().After(deadline) {
			return errors.New("input context changed while waiting for modifiers")
		}
		time.Sleep(5 * time.Millisecond)
	}
	var stagedMarker uuid.UUID
	var stagedText []uint16
	if strings.ContainsAny(text, "\r\n") {
		beforeRead := C.snippetClipboardSequence()
		previous, err := clipboard.Read()
		if err != nil && !errors.Is(err, clipboard.NoDataErr()) {
			return errors.New("clipboard cannot be preserved")
		}
		var old []uint16
		var pngData, dibData []byte
		var restoreKind windowsClipboardRestoreKind
		switch data := previous.(type) {
		case nil:
			if beforeRead == 0 || C.snippetClipboardIsEmpty(C.DWORD(beforeRead)) != 1 {
				return errors.New("clipboard contents cannot be preserved")
			}
			restoreKind = windowsClipboardRestoreEmpty
		case *clipboard.TextData:
			restoreKind = windowsClipboardRestoreText
			old = utf16.Encode([]rune(data.Text))
		case *clipboard.FilePathData:
			if len(data.FilePaths) > 0 {
				restoreKind = windowsClipboardRestoreFiles
				for _, path := range data.FilePaths {
					old = append(old, utf16.Encode([]rune(path))...)
					old = append(old, 0)
				}
				old = append(old, 0)
			}
		case *clipboard.ImageData:
			if data.Image == nil {
				return errors.New("clipboard image cannot be preserved")
			}
			var pngBuffer, bmpBuffer bytes.Buffer
			if png.Encode(&pngBuffer, data.Image) != nil || bmp.Encode(&bmpBuffer, data.Image) != nil || bmpBuffer.Len() <= 14 {
				return errors.New("clipboard image cannot be preserved")
			}
			pngData, dibData = pngBuffer.Bytes(), bmpBuffer.Bytes()[14:]
			restoreKind = windowsClipboardRestoreImage
		}
		if restoreKind == windowsClipboardRestoreUnavailable {
			return errors.New("clipboard contents cannot be preserved")
		}
		if beforeRead == 0 || C.snippetClipboardSequence() != beforeRead {
			return errors.New("clipboard changed before staging")
		}
		marker, markerErr := uuid.NewRandom()
		if markerErr != nil {
			return errors.New("clipboard marker unavailable")
		}
		units := utf16.Encode([]rune(text))
		if C.snippetClipboardStageText(C.DWORD(beforeRead), (*C.uint16_t)(unsafe.Pointer(&units[0])), C.uint(len(units)), (*C.uint8_t)(unsafe.Pointer(&marker[0]))) != 1 {
			return errors.New("clipboard changed or staging failed")
		}
		stagedMarker, stagedText = marker, units
		defer func() {
			var oldPointer *C.uint16_t
			if len(old) > 0 {
				oldPointer = (*C.uint16_t)(unsafe.Pointer(&old[0]))
			}
			switch restoreKind {
			case windowsClipboardRestoreEmpty:
				C.snippetClipboardRestoreEmpty((*C.uint8_t)(unsafe.Pointer(&marker[0])))
			case windowsClipboardRestoreImage:
				C.snippetClipboardRestoreImage((*C.uint8_t)(unsafe.Pointer(&marker[0])), (*C.uint8_t)(unsafe.Pointer(&pngData[0])), C.uint(len(pngData)), (*C.uint8_t)(unsafe.Pointer(&dibData[0])), C.uint(len(dibData)))
			case windowsClipboardRestoreFiles:
				C.snippetClipboardRestoreFiles((*C.uint8_t)(unsafe.Pointer(&marker[0])), oldPointer, C.uint(len(old)))
			case windowsClipboardRestoreText:
				C.snippetClipboardRestoreText((*C.uint8_t)(unsafe.Pointer(&marker[0])), oldPointer, C.uint(len(old)))
			}
		}()
	}
	multiline := strings.ContainsAny(text, "\r\n")
	var units []uint16
	var textPointer *C.uint16_t
	var paste C.int
	if multiline {
		paste = 1
	} else {
		units = utf16.Encode([]rune(text))
		if len(units) != 0 {
			textPointer = (*C.uint16_t)(&units[0])
		}
	}
	if !checks.valid() || uint64(C.snippetTarget()) != target {
		return errors.New("input context changed")
	}
	if checks.trigger != "" && deletes <= 32768 {
		trigger := utf16.Encode([]rune(checks.trigger))
		selection := C.snippetWindowsSelectOmniboxTrigger((*C.uint16_t)(&trigger[0]), C.int(len(trigger)))
		if selection == C.SNIPPET_TRIGGER_CHANGED || !checks.current() || uint64(C.snippetTarget()) != target {
			return errors.New("trigger or input context changed")
		}
		if selection == C.SNIPPET_TRIGGER_SELECTED {
			// Typing or pasting replaces the selection. Empty snippets need one Backspace to clear it.
			deletes = 0
			if text == "" {
				deletes = 1
			}
		}
	}
	if multiline && (C.snippetClipboardStillStaged((*C.uint8_t)(unsafe.Pointer(&stagedMarker[0])), (*C.uint16_t)(unsafe.Pointer(&stagedText[0])), C.uint(len(stagedText))) == 0 || !checks.current() || uint64(C.snippetTarget()) != target) {
		return errors.New("clipboard or input context changed")
	}
	if C.snippetWindowsReplace(C.uint(deletes), textPointer, C.uint(len(units)), paste) == 0 {
		return errors.New("text replacement failed")
	}
	// Undo must be available while the clipboard remains staged for the target.
	checks.applied()
	if multiline {
		// Let the target consume its clipboard offer before restoring it.
		select {
		case <-ctx.Done():
		case <-time.After(150 * time.Millisecond):
		}
		return nil
	}
	return nil
}
