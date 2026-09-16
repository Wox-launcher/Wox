//go:build wox_ui_smoke

package wpm

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
)

// Test001LauncherPluginWpmDevLocalDirectory verifies a local plugin directory added in WPM settings appears in the table and in wpm dev.list, then disappears from both after the row is deleted.
// Flow: create a plugin.json fixture -> add its directory in WPM settings -> confirm the path cell -> query wpm dev.list -> delete the row -> query wpm dev.list again.
// Evidence: the settings table shows the stored path, then the completed launcher list shows and later omits the local plugin name.
func Test001LauncherPluginWpmDevLocalDirectory(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		fixture := newWpmDevPluginFixture(t)
		rowIndex := addWpmLocalDirectory(t, ctx, client, fixture.Directory)
		t.Cleanup(func() { removeWpmLocalDirectory(t, client, fixture.Directory, rowIndex) })
		confirmWpmLocalDirectoryPersisted(t, ctx, client, fixture.Directory, rowIndex)

		waitForWpmDevListPlugin(t, ctx, client, fixture.Name, true)

		removeWpmLocalDirectory(t, client, fixture.Directory, rowIndex)
		waitForWpmDevListPlugin(t, ctx, client, fixture.Name, false)
	})
}
