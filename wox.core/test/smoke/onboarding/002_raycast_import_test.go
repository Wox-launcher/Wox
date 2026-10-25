//go:build wox_ui_smoke

package onboarding

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"wox/plugin/thirdparty/raycast"
	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

const (
	raycastSmokePassphrase = "wox-smoke-pass"
	raycastSmokeKeyword    = "woxsmokesearch"
	raycastSmokeTitle      = "Wox Smoke Search"
	raycastSmokeItem       = "onboarding-migrate-item-quicklink:0"
)

// Test002RaycastImport verifies a detected Raycast install can unlock one generated export and use the quicklink without a restart.
func Test002RaycastImport(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		exportPath := writeRaycastSmokeExport(t)
		before, err := os.ReadFile(exportPath)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), smoke.CaseTimeout)
			defer cancel()
			closeOnboardingIfOpen(t, cleanupCtx, client)
			_ = os.Remove(filepath.Join(os.Getenv(util.TestRaycastInstallEnv), "installed"))
		})

		openOnboarding(t, ctx, client)
		for _, step := range onboardingWizardSteps() {
			waitOnboardingStep(t, ctx, client, step)
			if step == "migrate" {
				unlockRaycastSmokeExport(t, ctx, client)
				importRaycastSmokeQuicklink(t, ctx, client)
			}
			if step == "finish" {
				activateOnboarding(t, ctx, client, "onboarding-finish")
				break
			}
			activateOnboarding(t, ctx, client, "onboarding-next")
		}
		waitOnboardingSetting(t, ctx, "OnboardingFinished", "true")
		smoke.ShowLauncher(t, ctx, client)
		// Trigger routing requires the delimiter; the bare keyword is a global search.
		snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, raycastSmokeKeyword+" ")
		if !smoke.HasLauncherResultLabel(snapshot, raycastSmokeTitle) {
			t.Fatalf("imported quicklink %q did not show %q", raycastSmokeKeyword, raycastSmokeTitle)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
		after, err := os.ReadFile(exportPath)
		if err != nil || string(after) != string(before) {
			t.Fatal("raycast export file changed")
		}
	})
}

func writeRaycastSmokeExport(t *testing.T) string {
	t.Helper()
	root := os.Getenv(util.TestRaycastInstallEnv)
	path := os.Getenv(util.TestRaycastFileEnv)
	if root == "" || path == "" {
		t.Fatal("raycast smoke paths are not configured; run smoke through make smoke")
	}
	if err := os.WriteFile(filepath.Join(root, "installed"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"quicklinks":{"quicklinks":[{"name":"Wox Smoke Search","link":"https://example.com/search"}]}}`)
	blob, err := raycast.Seal(payload, raycastSmokePassphrase)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, blob, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func unlockRaycastSmokeExport(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		button, found := automationdriver.Find(snapshot, "onboarding-migrate-unlock")
		return found && button.Enabled
	}); err != nil {
		t.Fatalf("wait for raycast unlock: %v", err)
	}
	if err := client.Perform(ctx, "onboarding-migrate-passphrase", woxui.AccessibilityActionSetValue, raycastSmokePassphrase); err != nil {
		t.Fatalf("enter raycast passphrase: %v", err)
	}
	activateOnboarding(t, ctx, client, "onboarding-migrate-unlock")
}

func importRaycastSmokeQuicklink(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		item, found := automationdriver.Find(snapshot, raycastSmokeItem)
		return found && item.Checked
	}); err != nil {
		t.Fatalf("wait for raycast quicklink: %v", err)
	}
	activateOnboarding(t, ctx, client, "onboarding-next")
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, visible := automationdriver.Find(snapshot, raycastSmokeItem)
		next, nextFound := automationdriver.Find(snapshot, "onboarding-next")
		return !visible && nextFound && next.Enabled
	}); err != nil {
		t.Fatalf("wait for imported raycast quicklink: %v", err)
	}
}
