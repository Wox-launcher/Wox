//go:build windows

package woxui

import (
	"os"
	"testing"
	"time"
)

const windowsAccessibilityCloseIntegrationEnv = "WOX_WINDOWS_ACCESSIBILITY_CLOSE_INTEGRATION"

// TestWindowsAccessibilityWindowCloseIntegration exercises the real HWND and UIA teardown path.
func TestWindowsAccessibilityWindowCloseIntegration(t *testing.T) {
	if os.Getenv(windowsAccessibilityCloseIntegrationEnv) != "1" {
		t.Skip("set WOX_WINDOWS_ACCESSIBILITY_CLOSE_INTEGRATION=1 to run the native window close test")
	}

	closeResult := make(chan error, 1)
	runErr := Run(func() error {
		window, err := Open(WindowOptions{Title: "Wox accessibility close integration"})
		if err != nil {
			return err
		}
		if err := window.UpdateAccessibility(AccessibilityTree{
			Generation: 1,
			RootIDs:    []AccessibilityNodeID{1},
			Nodes: []AccessibilityNode{{
				ID:           1,
				AutomationID: "integration-root",
				Role:         AccessibilityRoleWindow,
				Label:        "Integration window",
				Enabled:      true,
			}},
		}, nil); err != nil {
			return err
		}
		go func() {
			time.Sleep(50 * time.Millisecond)
			closeResult <- window.Close()
		}()
		return nil
	})
	if runErr != nil {
		t.Fatalf("window runtime failed: %v", runErr)
	}
	select {
	case err := <-closeResult:
		if err != nil {
			t.Fatalf("close accessibility window: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("window close did not complete")
	}
}
