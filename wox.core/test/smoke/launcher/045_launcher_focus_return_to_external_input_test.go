//go:build wox_ui_smoke && windows

package query

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/lxn/win"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	"wox/util/clipboard"
	"wox/util/window"
)

// Test045LauncherFocusReturnToExternalInput verifies that paste targets the original editable window.
// Flow: focus the second of two external windows -> show Wox and paste a Clipboard result -> show again and press Escape.
// Evidence: only the original edit control receives the marker and Escape returns focus to the same HWND.
func Test045LauncherFocusReturnToExternalInput(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.PreserveClipboard(t)
		if err := client.Hide(ctx); err != nil {
			t.Fatal(err)
		}
		decoy, target := newPasteFocusEditors(t)
		id := strconv.FormatUint(uint64(target.hwnd), 10)
		focusCtx, cancel := context.WithTimeout(ctx, automationdriver.ActionTimeout)
		defer cancel()
		if err := window.FocusWindow(focusCtx, id, os.Getpid()); err != nil {
			t.Fatalf("focus external editor: %v", err)
		}

		marker := fmt.Sprintf("wox-smoke-focus-%d", time.Now().UnixNano())
		if err := clipboard.WriteText(marker); err != nil {
			t.Fatal(err)
		}
		smoke.ShowLauncher(t, ctx, client)
		queryCtx, cancelQuery := context.WithTimeout(ctx, automationdriver.ActionTimeout)
		defer cancelQuery()
		for suffix := ""; ; {
			snapshot := smoke.SetLauncherQueryAndWaitComplete(t, queryCtx, client, "cb "+marker+suffix)
			if _, found := smoke.FindLauncherResult(snapshot, marker); found {
				break
			}
			if suffix == "" {
				suffix = " "
			} else {
				suffix = ""
			}
			if queryCtx.Err() != nil {
				t.Fatalf("clipboard watcher did not record the marker: %v", queryCtx.Err())
			}
		}
		smoke.SelectLauncherResultLabelPrefix(t, ctx, client, marker)
		actions := smoke.OpenResultActionPanel(t, ctx, client)
		pasteID := ""
		for _, node := range actions.Tree.Nodes {
			if strings.HasPrefix(node.AutomationID, "action-result-") && strings.Contains(node.Label, "Original editor") {
				pasteID = node.AutomationID
				break
			}
		}
		if pasteID == "" {
			t.Fatalf("Clipboard result did not expose an action for the original editor: %+v", actions.Tree.Nodes)
		}
		if err := client.Perform(ctx, pasteID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		pasteCtx, cancelPaste := context.WithTimeout(ctx, automationdriver.ActionTimeout)
		defer cancelPaste()
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for win.GetForegroundWindow() != target.hwnd || target.text() != marker {
			select {
			case <-pasteCtx.Done():
				t.Fatalf("wait for paste in exact window %v: foreground %v, text %q, decoy %q: %v",
					target.hwnd, win.GetForegroundWindow(), target.text(), decoy.text(), pasteCtx.Err())
			case <-ticker.C:
			}
		}
		if text := decoy.text(); text != "" {
			t.Fatalf("paste also reached the other window: %q", text)
		}

		smoke.ShowLauncher(t, ctx, client)
		if err := client.PressKey(ctx, woxui.KeyEscape, 0); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool {
			return !state.Visible && win.GetForegroundWindow() == target.hwnd
		}); err != nil {
			t.Fatalf("Escape did not hide Wox and return exact focus: %v", err)
		}
	})
}

type pasteFocusEditor struct {
	hwnd win.HWND
	edit win.HWND
}

func (e pasteFocusEditor) text() string {
	var text [256]uint16
	win.SendMessage(e.edit, win.WM_GETTEXT, uintptr(len(text)), uintptr(unsafe.Pointer(&text[0])))
	return syscall.UTF16ToString(text[:])
}

// newPasteFocusEditors runs two windows with independent native edit controls on the same thread.
func newPasteFocusEditors(t *testing.T) (pasteFocusEditor, pasteFocusEditor) {
	t.Helper()
	ready := make(chan [2]pasteFocusEditor, 1)
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(done)
		class, _ := syscall.UTF16PtrFromString("WoxSmokePasteEditor")
		editClass, _ := syscall.UTF16PtrFromString("EDIT")
		instance := win.GetModuleHandle(nil)
		var registration win.WNDCLASSEX
		registration.CbSize = uint32(unsafe.Sizeof(registration))
		registration.HInstance = instance
		registration.LpszClassName = class
		registration.LpfnWndProc = syscall.NewCallback(func(hwnd win.HWND, message uint32, wParam, lParam uintptr) uintptr {
			switch message {
			case win.WM_SETFOCUS:
				if child := win.GetWindow(hwnd, win.GW_CHILD); child != 0 {
					win.SetFocus(child)
				}
				return 0
			case win.WM_CLOSE:
				win.PostQuitMessage(0)
				return 0
			}
			return win.DefWindowProc(hwnd, message, wParam, lParam)
		})
		var editors [2]pasteFocusEditor
		if win.RegisterClassEx(&registration) == 0 {
			ready <- editors
			return
		}
		unregisterClass := syscall.NewLazyDLL("user32.dll").NewProc("UnregisterClassW")
		defer func() {
			unregisterClass.Call(uintptr(unsafe.Pointer(class)), uintptr(instance))
		}()
		for i, name := range []string{"Other editor", "Original editor"} {
			title, _ := syscall.UTF16PtrFromString(name)
			hwnd := win.CreateWindowEx(0, class, title, win.WS_OVERLAPPEDWINDOW, 50+int32(i)*90, 70+int32(i)*70, 440, 160, 0, 0, instance, nil)
			if hwnd == 0 {
				ready <- editors
				return
			}
			defer win.DestroyWindow(hwnd)
			edit := win.CreateWindowEx(0, editClass, nil, win.WS_CHILD|win.WS_VISIBLE|win.WS_BORDER, 12, 12, 380, 32, hwnd, 0, instance, nil)
			editors[i] = pasteFocusEditor{hwnd: hwnd, edit: edit}
			win.ShowWindow(hwnd, win.SW_SHOW)
		}
		ready <- editors
		var message win.MSG
		for win.GetMessage(&message, 0, 0, 0) > 0 {
			win.TranslateMessage(&message)
			win.DispatchMessage(&message)
		}
	}()
	editors := <-ready
	if editors[1].hwnd != 0 {
		t.Cleanup(func() {
			win.PostMessage(editors[1].hwnd, win.WM_CLOSE, 0, 0)
			<-done
		})
	}
	if editors[0].edit == 0 || editors[1].edit == 0 {
		t.Fatal("could not create both native edit controls")
	}
	return editors[0], editors[1]
}
