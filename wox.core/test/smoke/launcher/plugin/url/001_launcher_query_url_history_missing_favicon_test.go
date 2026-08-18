//go:build wox_ui_smoke

package url

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	"wox/test/smokefixture"
)

// Test001LauncherQueryURLHistoryMissingFavicon verifies URL history remains searchable after its cached favicon is deleted.
// Flow: delete the persisted history favicon -> query the URL history text -> wait for the completed result generation.
// Evidence: the historical URL is visible within the query deadline and the query does not recreate the missing favicon.
func Test001LauncherQueryURLHistoryMissingFavicon(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		iconPath := smokefixture.MissingFaviconURLHistoryIconPath(os.Getenv(automationdriver.SharedDataDirectoryEnvironment))
		iconData, err := os.ReadFile(iconPath)
		if err != nil {
			t.Fatalf("read seeded URL history favicon: %v", err)
		}
		t.Cleanup(func() {
			if err := os.WriteFile(iconPath, iconData, 0644); err != nil {
				t.Errorf("restore seeded URL history favicon: %v", err)
			}
		})
		if err := os.Remove(iconPath); err != nil {
			t.Fatalf("delete seeded URL history favicon: %v", err)
		}

		smoke.ShowLauncher(t, ctx, client)
		queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		snapshot := smoke.SetLauncherQueryAndWaitComplete(t, queryCtx, client, smokefixture.MissingFaviconURLHistoryQuery)
		if _, found := smoke.FindLauncherResult(snapshot, smokefixture.MissingFaviconURLHistoryURL); !found {
			t.Fatalf("URL history result %q was not found", smokefixture.MissingFaviconURLHistoryURL)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
		if _, err := os.Stat(iconPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("URL query recreated missing favicon %q; stat error: %v", iconPath, err)
		}
	})
}
