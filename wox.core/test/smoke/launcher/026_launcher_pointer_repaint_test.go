//go:build wox_ui_smoke

package query

import (
	"context"
	"runtime"
	"testing"
	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
)

// Test026LauncherPointerRepaint checks real hover, scrollbar animation and wheel frames with a fixed preview.
func Test026LauncherPointerRepaint(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows retained-buffer repaint contract")
	}
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "wox-smoke list-500 preview")
		list, found := automationdriver.Find(snapshot, "launcher.results")
		if !found {
			t.Fatal("missing result viewport")
		}
		bounds, err := client.Bounds(ctx)
		if err != nil {
			t.Fatal(err)
		}
		window := woxui.Rect{Width: bounds.Width, Height: bounds.Height}
		point := woxui.Point{X: list.Bounds.X + 20, Y: list.Bounds.Y + list.Bounds.Height/2}
		assertInteractionDamage(t, ctx, client, window, "hover", func() error { return client.MovePointer(ctx, point) })
		assertInteractionDamage(t, ctx, client, window, "continuous hover", func() error {
			for i := 0; i < 15; i++ {
				point.X++
				if err := client.MovePointer(ctx, point); err != nil {
					return err
				}
			}
			return nil
		})
		for _, delta := range []float32{-160, 160} {
			before, err := client.Snapshot(ctx)
			if err != nil {
				t.Fatal(err)
			}
			rowID, found := unselectedLauncherResultID(before)
			if !found {
				t.Fatal("missing scroll witness row")
			}
			row, _ := automationdriver.Find(before, rowID)
			assertInteractionDamage(t, ctx, client, window, "wheel", func() error {
				return client.Pointer(ctx, woxui.PointerEvent{Kind: woxui.PointerScroll, Position: point, Scroll: woxui.Point{Y: delta}})
			})
			after, err := client.Snapshot(ctx)
			if err != nil {
				t.Fatal(err)
			}
			moved, found := automationdriver.Find(after, rowID)
			if found && moved.Bounds == row.Bounds {
				t.Fatal("wheel did not move the result list")
			}
		}

	})
}
