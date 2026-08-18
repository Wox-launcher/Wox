//go:build wox_ui_smoke && !darwin && !windows

package clipboard

import (
	"context"
	"testing"
)

func requireClipboardIgnoredAppRuntime(t *testing.T) {
	t.Helper()
	t.Skip("Clipboard ignored applications are not exposed on this platform")
}

func ignoredClipboardAppTarget(t *testing.T) (string, string) {
	t.Helper()
	return "", ""
}

func copyTextFromIgnoredApplication(t *testing.T, ctx context.Context, text string) {
	t.Helper()
	t.Skip("native ignored-application clipboard input is unavailable on this platform")
}
