//go:build wox_ui_smoke

package query

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test006LauncherInputRepaintDamage verifies idle caret blinking stays local in both launcher editors.
// Flow: settle a completed query -> observe query-box caret frames -> open the action panel -> observe its filter caret frames.
// Evidence: every settled frame reports non-empty logical damage contained by the focused input instead of full-window damage.
// Windows restores the caret backdrop; macOS and Linux repaint the renderer-blurred panel.
func Test006LauncherInputRepaintDamage(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		if err := client.SetRepaintDebugMode(ctx, woxwidget.RepaintDebugOff); err != nil {
			t.Fatal(err)
		}
		smoke.ShowLauncher(t, ctx, client)
		snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, "1+1")
		assertIdleInputDamage(t, ctx, client, snapshot, "launcher.query.input", woxui.Rect{}, false)

		snapshot = smoke.OpenResultActionPanel(t, ctx, client)
		var surface woxui.Rect
		if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
			// The renderer-blurred floating material samples the back buffer under the panel, so
			// any repaint inside the panel must cover the whole panel plus the blur's sampling
			// margin. Derive the panel from its rows and filter; the outset absorbs panel padding,
			// the header above the rows, the blur margin and the host paint outset.
			for _, node := range snapshot.Tree.Nodes {
				if strings.HasPrefix(node.AutomationID, "action-") {
					surface = unionRect(surface, node.Bounds)
				}
			}
			surface = expandRect(surface, 96)
		}
		assertIdleInputDamage(t, ctx, client, snapshot, "action-search", surface, false)
		if runtime.GOOS == "windows" {
			if err := client.SimulateRendererDeviceRemoved(ctx); err != nil {
				t.Fatal(err)
			}
			snapshot, err := client.Snapshot(ctx)
			if err != nil {
				t.Fatal(err)
			}
			assertIdleInputDamage(t, ctx, client, snapshot, "action-search", surface, false)
			if err := client.SetRepaintDebugMode(ctx, woxwidget.RepaintDebugRainbow); err != nil {
				t.Fatal(err)
			}
			snapshot, err = client.Snapshot(ctx)
			if err != nil {
				t.Fatal(err)
			}
			assertIdleInputDamage(t, ctx, client, snapshot, "action-search", surface, true)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}

// assertIdleInputDamage waits through one settling caret frame, then checks two complete blink phases.
// A non-empty surface widens the allowed damage from the input to that floating surface.
func assertIdleInputDamage(t *testing.T, ctx context.Context, client *automationdriver.Client, snapshot woxwidget.AutomationSnapshot, inputID string, surface woxui.Rect, highlights bool) {
	t.Helper()
	input, found := automationdriver.Find(snapshot, inputID)
	if !found || !input.Focused {
		t.Fatalf("idle repaint input %q = found %v focused %v", inputID, found, input.Focused)
	}

	settled, err := client.WaitForChange(ctx, snapshot.Tree.Generation)
	if err != nil {
		t.Fatalf("wait for %q to settle on a caret frame: %v", inputID, err)
	}
	if err := client.ResetFrameMetrics(ctx); err != nil {
		t.Fatalf("reset frame metrics for %q: %v", inputID, err)
	}
	// Host damage includes a 4px paint outset; one extra logical pixel absorbs fractional layout bounds.
	allowed := expandRect(input.Bounds, 5)
	if surface.Width > 0 && surface.Height > 0 {
		allowed = unionRect(allowed, surface)
	}
	generation := settled.Tree.Generation
	lastFrameID := uint64(0)
	consecutiveLocal := 0
	observed := make([]woxui.FrameMetricsSample, 0, 16)
	idleCtx, cancelIdle := context.WithTimeout(ctx, 6*time.Second)
	defer cancelIdle()
	for idleCtx.Err() == nil {
		next, waitErr := client.WaitForChange(idleCtx, generation)
		if waitErr != nil {
			break
		}
		generation = next.Tree.Generation
		metrics, metricsErr := client.FrameMetrics(ctx)
		if metricsErr != nil {
			t.Fatalf("read idle frame metrics for %q: %v", inputID, metricsErr)
		}
		for _, sample := range metrics.Recent {
			if sample.FrameID <= lastFrameID || !sample.HostCompleted || !sample.Presented {
				continue
			}
			lastFrameID = sample.FrameID
			observed = append(observed, sample)
			if sample.LogicalDamage.Width <= 0 || sample.LogicalDamage.Height <= 0 {
				continue
			}
			// Query chrome can still paint while the action-panel filter is focused.
			// Ignore damage that does not touch the input under test.
			if !rectsIntersect(allowed, sample.LogicalDamage) {
				continue
			}
			local := containsRect(allowed, sample.LogicalDamage)
			if runtime.GOOS == "windows" {
				local = local && sample.LogicalDamage.Width <= 2
				if !highlights {
					local = local && sample.RendererResources.CacheHits > 0 && sample.RendererResources.TextRasterizations == 0
				}
			}
			if local {
				consecutiveLocal++
				if consecutiveLocal >= 2 {
					if runtime.GOOS == "windows" && !highlights {
						assertCaretPixelChanges(t, ctx, client, sample.LogicalDamage)
					}
					return
				}
			} else {
				consecutiveLocal = 0
			}
		}
	}
	t.Fatalf("idle repaint for %q never settled to two consecutive local frames within %+v; allowed bounds %+v", inputID, observed, allowed)
}

// assertCaretPixelChanges checks real composited pixels across blink phases, including
// translucent material restoration. Scale comes from the captured image, not an assumed DPI.
func assertCaretPixelChanges(t *testing.T, ctx context.Context, client *automationdriver.Client, caret woxui.Rect) {
	t.Helper()
	bounds, err := client.Bounds(ctx)
	if err != nil {
		t.Fatal(err)
	}
	directory, err := os.MkdirTemp("", "wox-caret-pixels-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("caret captures: %s", directory)
		} else {
			os.RemoveAll(directory)
		}
	})
	var previous, otherPhase image.Image
	changes := 0
	deadline := time.Now().Add(6 * time.Second)
	for phase := 0; time.Now().Before(deadline) && changes < 2; phase++ {
		snapshot, err := client.Snapshot(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitForChange(ctx, snapshot.Tree.Generation); err != nil {
			t.Fatal(err)
		}
		// A semantics frame is not a blink phase, and capture can race presentation.
		// Sample across several periods instead of requiring five frames to contain two flips.
		time.Sleep(100 * time.Millisecond)
		path := filepath.Join(directory, fmt.Sprintf("caret-%d.png", phase))
		if err := client.Capture(ctx, path); err != nil {
			t.Fatal(err)
		}
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		current, err := png.Decode(file)
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
		if previous != nil {
			if current.Bounds() != previous.Bounds() {
				t.Fatal("window moved or resized during idle blink")
			}
			sx := float64(current.Bounds().Dx()) / float64(bounds.Width)
			sy := float64(current.Bounds().Dy()) / float64(bounds.Height)
			// Host caret paints include a 4px outset and stroke antialiasing. A
			// 1px physical pad is not enough at 150%+ scaling.
			pad := int(math.Ceil(max(sx, sy)))
			if pad < 1 {
				pad = 1
			}
			allowed := image.Rect(int(math.Floor(float64(caret.X)*sx))-pad, int(math.Floor(float64(caret.Y)*sy))-pad,
				int(math.Ceil(float64(caret.X+caret.Width)*sx))+pad, int(math.Ceil(float64(caret.Y+caret.Height)*sy))+pad)
			changed := false
			// Desktop capture includes live pixels behind the translucent window.
			// Check the caret and adjacent pixels, allowing two levels of compositor rounding.
			nearby := allowed.Inset(-3).Intersect(current.Bounds())
			for y := nearby.Min.Y; y < nearby.Max.Y; y++ {
				for x := nearby.Min.X; x < nearby.Max.X; x++ {
					r, g, b, a := current.At(x, y).RGBA()
					pr, pg, pb, pa := previous.At(x, y).RGBA()
					if absPixelDelta(r, pr) <= 2*257 && absPixelDelta(g, pg) <= 2*257 && absPixelDelta(b, pb) <= 2*257 && a == pa {
						continue
					}
					if !image.Pt(x, y).In(allowed) {
						t.Fatalf("blink changed pixel (%d,%d) outside caret %v at scale %.2fx%.2f", x, y, allowed, sx, sy)
					}
					changed = true
				}
			}
			if changed {
				if otherPhase != nil {
					for y := nearby.Min.Y; y < nearby.Max.Y; y++ {
						for x := nearby.Min.X; x < nearby.Max.X; x++ {
							r, g, b, a := current.At(x, y).RGBA()
							pr, pg, pb, pa := otherPhase.At(x, y).RGBA()
							if absPixelDelta(r, pr) > 2*257 || absPixelDelta(g, pg) > 2*257 || absPixelDelta(b, pb) > 2*257 || a != pa {
								t.Fatalf("caret phase accumulated a pixel change at (%d,%d)", x, y)
							}
						}
					}
				}
				otherPhase = previous
				previous = current
				changes++
			}
		} else {
			previous = current
		}
	}
	if changes < 2 {
		t.Fatal("caret pixels did not blink through two phases")
	}
}

func absPixelDelta(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func expandRect(rect woxui.Rect, outset float32) woxui.Rect {
	return woxui.Rect{X: rect.X - outset, Y: rect.Y - outset, Width: rect.Width + 2*outset, Height: rect.Height + 2*outset}
}

func unionRect(left, right woxui.Rect) woxui.Rect {
	if left.Width <= 0 || left.Height <= 0 {
		return right
	}
	if right.Width <= 0 || right.Height <= 0 {
		return left
	}
	x := min(left.X, right.X)
	y := min(left.Y, right.Y)
	return woxui.Rect{X: x, Y: y, Width: max(left.X+left.Width, right.X+right.Width) - x, Height: max(left.Y+left.Height, right.Y+right.Height) - y}
}

func containsRect(outer, inner woxui.Rect) bool {
	return inner.X >= outer.X && inner.Y >= outer.Y && inner.X+inner.Width <= outer.X+outer.Width && inner.Y+inner.Height <= outer.Y+outer.Height
}

func rectsIntersect(left, right woxui.Rect) bool {
	return left.X < right.X+right.Width && right.X < left.X+left.Width && left.Y < right.Y+right.Height && right.Y < left.Y+left.Height
}
