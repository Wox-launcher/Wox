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
	app.settingsWindow.settingsOpen = true
	// settingsNativeWindow always hops through runOnUI. The real Linux dispatcher
	// runs that inline when the caller is already the UI thread. A one-slot queue
	// that does not reenter fills up inside invalidate and blocks the hide path.
	queued := make(chan func(), 1)
	depth := 0
	app.uiCall = func(fn func()) error {
		if depth > 0 {
			fn()
			return nil
		}
		queued <- fn
		return nil
	}

	anchor := woxui.Rect{X: 320, Y: 180, Width: 14, Height: 14}
	app.setSettingChoiceTooltip(true, "  tooltip content  ", anchor)
	if app.settingsWindow.settingsInlineTooltip != nil {
		t.Fatal("inline tooltip must wait for the shared hover dwell")
	}
	// Apply queued UI work on the test thread, just like the native event loop.
	select {
	case apply := <-queued:
		depth++
		apply()
		depth--
	case <-time.After(nativeHoverTooltipDelay + time.Second):
		t.Fatal("inline tooltip did not dispatch after dwell")
	}
	if app.settingsWindow.settingsInlineTooltip == nil {
		t.Fatal("expected inline tooltip state on linux after the hover dwell")
	}
	if app.settingsWindow.settingsInlineTooltip.Text != "tooltip content" {
		t.Fatalf("tooltip text = %q, want trimmed content", app.settingsWindow.settingsInlineTooltip.Text)
	}
	if app.settingsWindow.settingsInlineTooltip.Anchor != anchor {
		t.Fatalf("tooltip anchor = %#v, want %#v", app.settingsWindow.settingsInlineTooltip.Anchor, anchor)
	}
	if app.settingsWindow.settingsInlineTooltip.Side != "top" {
		t.Fatalf("tooltip side = %q, want top", app.settingsWindow.settingsInlineTooltip.Side)
	}

	app.setSettingChoiceTooltip(false, "", woxui.Rect{})
	depth++
	select {
	case apply := <-queued:
		apply()
	default:
	}
	depth--
	if app.settingsWindow.settingsInlineTooltip != nil {
		t.Fatalf("tooltip state = %#v, want nil after hide", app.settingsWindow.settingsInlineTooltip)
	}
}
