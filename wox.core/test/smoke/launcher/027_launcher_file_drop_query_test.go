//go:build wox_ui_smoke && windows

package query

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test027LauncherFileDropQuery verifies native file drops keep typed text and resize grid results.
// Flow: enter a targeted grid query -> drag an external file into Wox -> type a new search.
// Evidence: one dropped-file result replaces the grid, native height shrinks, and typing resumes input queries.
func Test027LauncherFileDropQuery(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		previous := smoke.OpenGeneralSettingsAndReadSwitch(t, ctx, client, "HideOnLostFocus")
		smoke.SetSettingSwitch(t, ctx, client, "HideOnLostFocus", false)
		t.Cleanup(func() { smoke.RestoreGeneralSettingSwitch(t, client, "HideOnLostFocus", previous) })
		if err := client.Hide(ctx); err != nil {
			t.Fatal(err)
		}
		smoke.ShowLauncher(t, ctx, client)
		path := filepath.Join(t.TempDir(), "drag-in.txt")
		if err := os.WriteFile(path, []byte("native drag smoke"), 0600); err != nil {
			t.Fatal(err)
		}
		text := "wox-smoke drag "
		snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, text)
		before, err := client.Bounds(ctx)
		if err != nil {
			t.Fatal(err)
		}
		hwnd := smoke.NativeDragForeground()
		node, ok := automationdriver.Find(snapshot, "launcher.query.input")
		if !ok {
			t.Fatal("query input missing")
		}
		targetX, targetY := smoke.NativeDragPoint(hwnd, woxui.Point{X: node.Bounds.X + node.Bounds.Width/2, Y: node.Bounds.Y + node.Bounds.Height/2})
		peer := smoke.OpenNativeDragPeer(t, ctx, client, path)
		x, y := peer.Center()
		smoke.NativeDragMouse(x, y, 0)
		smoke.NativeDragMouse(0, 0, 2)
		peer.Wait(t, ctx, client, "started")
		smoke.NativeDragMouse(targetX, targetY, 0)
		peer.Wait(t, ctx, client, "accepted")
		smoke.NativeDragMouse(0, 0, 4)
		peer.Wait(t, ctx, client, "ended")
		snapshot, err = client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
			input, _ := automationdriver.Find(s, "launcher.query.input")
			results, _ := automationdriver.Find(s, "launcher.results")
			return input.Value == text && results.Value == "complete" && hasResultTitle(s, "Dropped file")
		})
		if err != nil {
			t.Fatal(err)
		}
		waitForLauncherResultResize(t, ctx, client, func(height float32) bool { return height < before.Height-1 })
		smoke.AssertNoDiagnostics(t, snapshot)
		snapshot = smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, text+"keep "+path)
		if !hasResultTitle(snapshot, "Drag source") || hasResultTitle(snapshot, "Dropped file") {
			t.Fatal("typing remained in the file-selection query")
		}
	})
}
