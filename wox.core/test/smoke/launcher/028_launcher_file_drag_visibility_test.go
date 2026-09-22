//go:build wox_ui_smoke && windows

package query

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test028LauncherFileDragVisibility verifies real OLE outcomes obey the result's hide policy.
// Flow: drag keep-open and default-hide results to an external target, then cancel outside and inside Wox.
// Evidence: copied file bytes, callback status/files/session, and actual HWND visibility agree for every outcome.
func Test028LauncherFileDragVisibility(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		path := filepath.Join(t.TempDir(), "drag-out.txt")
		content := []byte("native drag export evidence")
		if err := os.WriteFile(path, content, 0600); err != nil {
			t.Fatal(err)
		}
		smoke.ShowLauncher(t, ctx, client)
		previous := smoke.OpenGeneralSettingsAndReadSwitch(t, ctx, client, "HideOnLostFocus")
		smoke.SetSettingSwitch(t, ctx, client, "HideOnLostFocus", true)
		t.Cleanup(func() { smoke.RestoreGeneralSettingSwitch(t, client, "HideOnLostFocus", previous) })
		if err := client.Hide(ctx); err != nil {
			t.Fatal(err)
		}
		smoke.ShowLauncher(t, ctx, client)
		peer := smoke.OpenNativeDragPeer(t, ctx, client, path)
		for _, mode := range []string{"keep", "hide", "cancel", "cancel-in-source"} {
			smoke.ShowLauncher(t, ctx, client)
			prevent := "keep"
			if mode == "hide" || mode == "cancel-in-source" {
				prevent = "hide"
			}
			snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "wox-smoke drag "+prevent+" "+path)
			smoke.AssertNoDiagnostics(t, snapshot)
			sourceID, ok := smoke.FindLauncherResult(snapshot, "Drag source")
			if !ok {
				t.Fatal("source result missing")
			}
			// Wait for the normal show transaction to finish before testing drag-specific focus behavior.
			if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool { return state.Visible && state.BlurReady }); err != nil {
				t.Fatal(err)
			}
			source, _ := automationdriver.Find(snapshot, sourceID)
			hwnd := smoke.NativeDragForeground()
			x, y := smoke.NativeDragPoint(hwnd, woxui.Point{X: source.Bounds.X + source.Bounds.Width/2, Y: source.Bounds.Y + source.Bounds.Height/2})
			t.Logf("drag mode=%s sourceLogical=%+v sourcePhysical=(%d,%d) sourceHWND=%x peerHWND=%x", mode, source.Bounds, x, y, hwnd, peer.Handle)
			smoke.LogNativeDragState(t, "before source drag "+mode, hwnd)
			_ = os.Remove(path + ".event.json")
			_ = os.Remove(filepath.Join(peer.Root, "entered"))
			_ = os.Remove(filepath.Join(peer.Root, "received"))
			logPath := filepath.Join(os.Getenv(automationdriver.SharedDataDirectoryEnvironment), "log", "wox.log")
			oldLog, _ := os.ReadFile(logPath)
			smoke.NativeDragMouse(x, y, 0)
			smoke.NativeDragMouse(0, 0, 2)
			if _, err := client.WaitFor(ctx, func(woxwidget.AutomationSnapshot) bool { return smoke.NativeDragCaptured(hwnd) }); err != nil {
				t.Fatalf("%s source mouse capture: %v", mode, err)
			}
			smoke.NativeDragMouse(x+20, y+20, 0)
			smoke.WaitForFile(t, ctx, logPath, func(data []byte) bool {
				return len(data) > len(oldLog) && strings.Contains(string(data[len(oldLog):]), "result file drag start")
			})
			if mode == "cancel" {
				smoke.NativeDragEscape()
			} else if mode == "cancel-in-source" {
				smoke.NativeDragMouse(0, 0, 4)
			} else {
				tx, ty := peer.Center()
				t.Logf("drag mode=%s targetPhysical=(%d,%d)", mode, tx, ty)
				smoke.NativeDragMouse(tx, ty, 0)
				peer.Wait(t, ctx, client, "entered")
				smoke.NativeDragMouse(0, 0, 4)
				peer.Wait(t, ctx, client, "received")
				got, err := os.ReadFile(filepath.Join(peer.Root, "received-drag-out.txt"))
				if err != nil || string(got) != string(content) {
					t.Fatalf("native drop copy: %q %v", got, err)
				}
			}
			smoke.NativeDragMouse(0, 0, 4)
			expected := "success"
			if mode == "cancel" {
				expected = "cancel"
			} else if mode == "cancel-in-source" {
				expected = "cancel_in_source"
			}
			var event struct {
				ResultId           string
				Status             string
				Files              []string
				SessionID, QueryID string
			}
			smoke.WaitForFile(t, ctx, path+".event.json", func(data []byte) bool { return json.Unmarshal(data, &event) == nil })
			if event.ResultId != strings.TrimPrefix(sourceID, "launcher.result.") || event.Status != expected || len(event.Files) != 1 || event.Files[0] != path || event.SessionID == "" || event.QueryID == "" {
				t.Fatalf("%s callback: %+v", mode, event)
			}
			if _, err := client.WaitFor(ctx, func(woxwidget.AutomationSnapshot) bool { return smoke.NativeDragVisible(hwnd) == (mode != "hide") }); err != nil {
				t.Fatalf("%s native visibility: %v", mode, err)
			}
			if mode == "keep" {
				// A real click must end the keep-visible hold, even if native blur events were swallowed.
				fresh, err := client.Snapshot(ctx)
				if err != nil {
					t.Fatal(err)
				}
				input, ok := automationdriver.Find(fresh, "launcher.query.input")
				if !ok {
					t.Fatal("query input missing after drag")
				}
				qx, qy := smoke.NativeDragPoint(hwnd, woxui.Point{X: input.Bounds.X + 20, Y: input.Bounds.Y + input.Bounds.Height/2})
				smoke.NativeDragMouse(qx, qy, 0)
				smoke.NativeDragMouse(0, 0, 2)
				smoke.NativeDragMouse(0, 0, 4)
				if _, err := client.WaitFor(ctx, func(woxwidget.AutomationSnapshot) bool { return smoke.NativeDragForeground() == hwnd }); err != nil {
					t.Fatal(err)
				}
				// The peer title bar is native chrome, so clicking it changes focus without starting another drag.
				tx, ty := smoke.NativeDragPoint(peer.Handle, woxui.Point{X: 40, Y: -8})
				smoke.NativeDragMouse(tx, ty, 0)
				smoke.NativeDragMouse(0, 0, 2)
				smoke.NativeDragMouse(0, 0, 4)
				if _, err := client.WaitFor(ctx, func(woxwidget.AutomationSnapshot) bool { return smoke.NativeDragForeground() == peer.Handle }); err != nil {
					t.Fatalf("peer focus: foreground=%x source=%x peer=%x: %v", smoke.NativeDragForeground(), hwnd, peer.Handle, err)
				}
				if _, err := client.WaitFor(ctx, func(woxwidget.AutomationSnapshot) bool { return !smoke.NativeDragVisible(hwnd) }); err != nil {
					t.Fatalf("normal blur hiding did not resume after clicking Wox: %v", err)
				}
			}

		}
	})
}
