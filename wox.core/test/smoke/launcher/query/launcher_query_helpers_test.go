//go:build wox_ui_smoke

package query

import (
	"errors"
	"strings"
	"testing"

	woxwidget "wox/ui/widget"
	"wox/util/clipboard"
)

// preserveClipboard restores the platform clipboard after a smoke case changes it.
func preserveClipboard(t *testing.T) {
	t.Helper()
	previousClipboard, err := clipboard.Read()
	if err != nil && !errors.Is(err, clipboard.NoDataErr()) {
		t.Fatalf("read clipboard before smoke case: %v", err)
	}
	t.Cleanup(func() {
		if previousClipboard != nil {
			if restoreErr := clipboard.Write(previousClipboard); restoreErr != nil {
				t.Errorf("restore clipboard after smoke case: %v", restoreErr)
			}
			return
		}
		if restoreErr := clipboard.WriteText(""); restoreErr != nil {
			t.Errorf("clear clipboard after smoke case: %v", restoreErr)
		}
	})
}

// hasLauncherResultLabel reports whether the current launcher generation exposes one matching result.
func hasLauncherResultLabel(snapshot woxwidget.AutomationSnapshot, label string) bool {
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") && node.Label == label {
			return true
		}
	}
	return false
}
