//go:build linux

package screenshot

import "testing"

func TestLinuxHyprlandPortalCaptureConfigKeepsScreenSpecificRestore(t *testing.T) {
	config := linuxHyprlandPortalCaptureConfig()
	if config.backend != "hyprland-portal" || !config.multiple || !config.screenSpecificRestore {
		t.Fatalf("unexpected Hyprland capture config: %+v", config)
	}
	if config.restoreTokenFile != linuxHyprlandPortalRestoreTokenFile {
		t.Fatalf("restore token file = %q", config.restoreTokenFile)
	}
}
