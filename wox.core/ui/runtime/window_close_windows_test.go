//go:build windows

package woxui

import (
	"fmt"
	"testing"

	"github.com/lxn/win"
)

// TestWindowsUserCloseHonorsApplicationPolicy covers the close message produced by Alt+F4.
func TestWindowsUserCloseHonorsApplicationPolicy(t *testing.T) {
	err := Run(func() error {
		requests, closed := 0, 0
		window, err := Open(WindowOptions{Title: "Wox close policy test", OnCloseRequested: func() { requests++ }, OnClosed: func() { closed++ }})
		if err != nil {
			return err
		}
		defer window.Close()
		// SC_CLOSE is the native system command dispatched by Alt+F4.
		win.SendMessage(window.native.hwnd, win.WM_SYSCOMMAND, 0xF060, 0)
		if requests != 1 || closed != 0 || !window.isOpen() {
			return fmt.Errorf("user close bypassed policy: requests=%d closed=%d open=%t", requests, closed, window.isOpen())
		}
		if err := window.Close(); err != nil {
			return err
		}
		if requests != 1 || closed != 1 {
			return fmt.Errorf("programmatic close re-entered policy: requests=%d closed=%d", requests, closed)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestWindowsForcedCloseBypassesApplicationPolicy keeps fatal renderer teardown unconditional.
func TestWindowsForcedCloseBypassesApplicationPolicy(t *testing.T) {
	err := Run(func() error {
		requests, closed := 0, 0
		window, err := Open(WindowOptions{Title: "Wox forced close test", OnCloseRequested: func() { requests++ }, OnClosed: func() { closed++ }})
		if err != nil {
			return err
		}
		defer window.Close()
		win.SendMessage(window.native.hwnd, windowForceCloseMessage, 0, 0)
		if requests != 0 || closed != 1 || window.isOpen() {
			return fmt.Errorf("forced close: requests=%d closed=%d open=%t", requests, closed, window.isOpen())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestWindowsCloseWithoutApplicationPolicyRetainsDefaultDestruction covers utility windows.
func TestWindowsCloseWithoutApplicationPolicyRetainsDefaultDestruction(t *testing.T) {
	err := Run(func() error {
		closed := 0
		window, err := Open(WindowOptions{Title: "Wox default close test", OnClosed: func() { closed++ }})
		if err != nil {
			return err
		}
		defer window.Close()
		win.SendMessage(window.native.hwnd, win.WM_CLOSE, 0, 0)
		if closed != 1 || window.isOpen() {
			return fmt.Errorf("default close: closed=%d open=%t", closed, window.isOpen())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
