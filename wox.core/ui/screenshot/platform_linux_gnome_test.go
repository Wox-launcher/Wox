//go:build linux

package screenshot

import "testing"

func TestLinuxGnomePortalCaptureConfigRequiresExplicitSingleMonitorSelection(t *testing.T) {
	config := linuxGnomePortalCaptureConfig()
	if config.backend != "gnome-portal" || config.multiple || !config.disableRestore {
		t.Fatalf("unexpected GNOME capture config: %+v", config)
	}
	if config.initialLatestFrameFor != linuxGnomePortalChooserCloseDelay {
		t.Fatalf("initial latest frame duration = %s", config.initialLatestFrameFor)
	}
}
