package updater

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomebrewAppImageSkipsInAppUpdate(t *testing.T) {
	const childEnv = "WOX_TEST_HOMEBREW_UPDATE_CHILD"
	const command = "brew upgrade --cask wox"
	if os.Getenv(childEnv) == "1" {
		ctx := context.Background()
		var info UpdateInfo
		CheckForUpdatesWithCallback(ctx, func(update UpdateInfo) { info = update })
		if info.CurrentVersion != CURRENT_VERSION || info.Status != UpdateStatusNone || info.HasUpdate {
			t.Fatalf("managed update check returned %+v", info)
		}
		// A running instance stays managed if Brew removes its receipt.
		artifact := filepath.Join(os.Getenv("HOMEBREW_PREFIX"), "Caskroom", "wox", "2.5.0", "wox-linux-amd64.AppImage")
		if err := os.Remove(artifact); err != nil {
			t.Fatal(err)
		}
		if err := ApplyUpdate(ctx, nil); err == nil || !strings.Contains(err.Error(), command) {
			t.Fatalf("applying a managed update returned %v, want Homebrew instructions", err)
		}
		return
	}

	// Use a fresh process because install ownership is cached at startup.
	root := t.TempDir()
	image := filepath.Join(root, "Wox.AppImage")
	cask := filepath.Join(root, "Caskroom", "wox", "2.5.0")
	if err := os.MkdirAll(cask, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(image, nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(image, filepath.Join(cask, "wox-linux-amd64.AppImage")); err != nil {
		t.Fatal(err)
	}

	child := exec.Command(os.Args[0], "-test.run=^TestHomebrewAppImageSkipsInAppUpdate$")
	child.Env = append(os.Environ(), childEnv+"=1", "APPIMAGE="+image, "HOMEBREW_PREFIX="+root)
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("managed updater test failed: %v\n%s", err, output)
	}
}
