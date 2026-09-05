//go:build windows && cgo

package quickjump

import (
	"context"
	"fmt"
	"github.com/lxn/win"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	woxui "wox/ui/runtime"
	"wox/util/window"
)

// TestOwnedFileDialogLifecycle exercises real COM pickers and the monitor bridge.
// Each picker is cancelled automatically; no user files are selected or written.
func TestOwnedFileDialogLifecycle(t *testing.T) {
	if os.Getenv("WOX_WINDOWS_FILE_DIALOG_INTEGRATION") != "1" {
		t.Skip("set WOX_WINDOWS_FILE_DIALOG_INTEGRATION=1 to run native picker lifecycle checks")
	}
	setExplorerMonitorLogger(func(message string) { t.Log(message) })
	defer setExplorerMonitorLogger(nil)
	targetFolder := t.TempDir()
	activated := make(chan OpenSaveDialogActivatedEvent, 16)
	closed := make(chan struct{}, 16)
	StartExplorerOpenSaveMonitor(func(event OpenSaveDialogActivatedEvent) {
		if event.Pid == os.Getpid() {
			activated <- event
		}
	}, func() { closed <- struct{}{} }, nil)
	defer StopExplorerOpenSaveMonitor()
	finished := make(chan error, 1)
	err := woxui.Run(func() error {
		owner, err := woxui.Open(woxui.WindowOptions{Title: "Wox picker lifecycle test"})
		if err != nil {
			return err
		}
		if _, err := owner.Show(); err != nil {
			return err
		}
		go func() {
			defer owner.Close()
			for _, kind := range []string{"folder", "file", "save"} {
				result := make(chan error, 1)
				go func() {
					var err error
					if kind == "save" {
						_, err = owner.SaveFile(woxui.SaveFileOptions{DefaultFileName: "wox-lifecycle-test.txt"})
					} else {
						_, err = owner.PickFile(woxui.FileDialogOptions{Directory: kind == "folder"})
					}
					result <- err
				}()
				select {
				case event := <-activated:
					if event.WindowID == "" || event.WindowID != GetOpenSaveDialogWindowIdByPid(os.Getpid()) {
						finished <- fmt.Errorf("%s: monitor did not retain the exact picker HWND", kind)
						return
					}
					var hwnd uintptr
					fmt.Sscanf(event.WindowID, "%d", &hwnd)
					// OnFolderChange can run before Show has made the HWND visible.
					deadline := time.Now().Add(5 * time.Second)
					for !win.IsWindowVisible(win.HWND(hwnd)) && time.Now().Before(deadline) {
						time.Sleep(10 * time.Millisecond)
					}
					if !win.IsWindowVisible(win.HWND(hwnd)) {
						finished <- fmt.Errorf("%s: picker never became visible", kind)
						return
					}
					// Navigate through the production Quick Jump path, including
					// jumping to the same folder twice (which must never submit).
					quickJump := &QuickJumpPlugin{}
					for attempt := 0; attempt < 2; attempt++ {
						if !quickJump.performFileDialogNavigation(context.Background(), event.WindowID, os.Getpid(), targetFolder) {
							finished <- fmt.Errorf("%s: folder navigation failed", kind)
							return
						}
						if !win.IsWindowVisible(win.HWND(hwnd)) {
							finished <- fmt.Errorf("%s: navigation closed the picker", kind)
							return
						}
						actual := window.GetFileDialogPathByWindowId(event.WindowID, os.Getpid())
						if !strings.EqualFold(filepath.Clean(actual), filepath.Clean(targetFolder)) {
							finished <- fmt.Errorf("%s: navigated to %q, want %q", kind, actual, targetFolder)
							return
						}
					}
					// A Wox hint window shares the PID but must not replace or
					// deactivate the native picker when it appears.
					var hint *woxui.Window
					var openErr error
					err := woxui.Call(func() { hint, openErr = woxui.Open(woxui.WindowOptions{Title: "Wox test hint", Nonactivating: true}) })
					if err == nil {
						err = openErr
					}
					if err != nil {
						finished <- err
						return
					}
					if _, err := hint.Show(); err != nil {
						hint.Close()
						finished <- err
						return
					}
					time.Sleep(350 * time.Millisecond)
					_, _, _, _, active := GetActiveDialogRect()
					if !active || GetOpenSaveDialogWindowIdByPid(os.Getpid()) != event.WindowID {
						hint.Close()
						finished <- fmt.Errorf("%s: Wox hint displaced the picker", kind)
						return
					}
					hint.Close()
					win.PostMessage(win.HWND(hwnd), win.WM_CLOSE, 0, 0)
				case <-time.After(10 * time.Second):
					finished <- fmt.Errorf("%s: no picker activation", kind)
					return
				}
				if err := <-result; err != nil {
					finished <- err
					return
				}
				deadline := time.After(5 * time.Second)
				for GetOpenSaveDialogWindowIdByPid(os.Getpid()) != "" {
					select {
					case <-closed:
					case <-deadline:
						finished <- fmt.Errorf("%s: cancelled picker still registered", kind)
						return
					}
				}
				// Foreground can publish the same HWND more than once while it opens.
				for len(activated) > 0 {
					<-activated
				}
			}
			finished <- nil
		}()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
}
