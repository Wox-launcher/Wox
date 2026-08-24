package launcher

import (
	"runtime"
	"testing"
	"time"

	woxui "wox/ui/runtime"
)

func TestSetSettingChoiceTooltipUsesInlineFallbackOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only tooltip fallback")
	}
	app := newApp(false, nil, woxui.NewWindowManager(), newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer app.cancel()
	app.settingsOpen = true

	anchor := woxui.Rect{X: 320, Y: 180, Width: 14, Height: 14}
	app.setSettingChoiceTooltip(true, "  tooltip content  ", anchor)
	if app.settingsInlineTooltip != nil {
		t.Fatal("inline tooltip must wait for the shared hover dwell")
	}
	deadline := time.Now().Add(nativeHoverTooltipDelay + 300*time.Millisecond)
	for app.settingsInlineTooltip == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if app.settingsInlineTooltip == nil {
		t.Fatal("expected inline tooltip state on linux after the hover dwell")
	}
	if app.settingsInlineTooltip.Text != "tooltip content" {
		t.Fatalf("tooltip text = %q, want trimmed content", app.settingsInlineTooltip.Text)
	}
	if app.settingsInlineTooltip.Anchor != anchor {
		t.Fatalf("tooltip anchor = %#v, want %#v", app.settingsInlineTooltip.Anchor, anchor)
	}
	if app.settingsInlineTooltip.Side != "top" {
		t.Fatalf("tooltip side = %q, want top", app.settingsInlineTooltip.Side)
	}

	app.setSettingChoiceTooltip(false, "", woxui.Rect{})
	if app.settingsInlineTooltip != nil {
		t.Fatalf("tooltip state = %#v, want nil after hide", app.settingsInlineTooltip)
	}
}
