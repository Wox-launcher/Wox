//go:build wox_ui_smoke

package filesearch

import "testing"

// Test007LauncherPluginFileSearchRealContent opts into reading document content
// separately from filename enumeration, with the same isolated Wox lifecycle.
func Test007LauncherPluginFileSearchRealContent(t *testing.T) {
	runRealFileSearchSmoke(t, true)
}
