//go:build wox_ui_smoke

package query

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test025LauncherRepaintInteractions checks every frame, including resize and buffer-repair frames.
// Flow: type/filter/pause/clear in the action panel, with and without repaint highlights.
// Evidence: action damage stays inside the panel and blur halo, including restored result rows.
func Test025LauncherRepaintInteractions(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows retained-buffer repaint contract")
	}
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "> "+actionPanelShellCommand)
		snapshot := smoke.OpenResultActionPanel(t, ctx, client)
		var panel woxui.Rect
		for _, node := range snapshot.Tree.Nodes {
			if strings.HasPrefix(node.AutomationID, "action-") {
				panel = unionRect(panel, node.Bounds)
			}
		}
		panel = expandRect(panel, 96)
		for _, mode := range []woxwidget.RepaintDebugMode{woxwidget.RepaintDebugOff, woxwidget.RepaintDebugRainbow} {
			if err := client.SetRepaintDebugMode(ctx, mode); err != nil {
				t.Fatal(err)
			}
			for cycle := 0; cycle < 2; cycle++ {
				assertInteractionDamage(t, ctx, client, panel, "filter", func() error { return client.EnterText(ctx, "__missing__") })
				assertInteractionDamage(t, ctx, client, panel, "continued typing", func() error { return client.EnterText(ctx, "x") })
				filtered, err := client.Snapshot(ctx)
				if err != nil {
					t.Fatal(err)
				}
				input, found := automationdriver.Find(filtered, "action-search")
				if !found || input.Value != "__missing__x" || len(actionPanelResultNodes(filtered)) != 0 {
					t.Fatal("filter did not produce the expected empty panel")
				}
				assertInteractionDamage(t, ctx, client, panel, "clear after buffer history settles", func() error {
					if err := client.PressKey(ctx, woxui.Key("a"), woxui.KeyModifierControl); err != nil {
						return err
					}
					return client.PressKey(ctx, woxui.KeyBackspace, 0)
				})
				cleared, err := client.Snapshot(ctx)
				if err != nil {
					t.Fatal(err)
				}
				input, found = automationdriver.Find(cleared, "action-search")
				if !found || input.Value != "" || len(actionPanelResultNodes(cleared)) != len(actionPanelResultNodes(snapshot)) {
					t.Fatal("clearing did not restore the action rows")
				}
			}
		}
		if err := client.PressKey(ctx, woxui.KeyEscape, 0); err != nil {
			t.Fatal(err)
		}
		if err := client.SetRepaintDebugMode(ctx, woxwidget.RepaintDebugOff); err != nil {
			t.Fatal(err)
		}
	})
}

// assertInteractionDamage drains prior native-buffer history before observing every new frame.
func assertInteractionDamage(t *testing.T, ctx context.Context, client *automationdriver.Client, allowed woxui.Rect, label string, action func() error) {
	t.Helper()
	time.Sleep(1200 * time.Millisecond)
	if err := client.ResetFrameMetrics(ctx); err != nil {
		t.Fatal(err)
	}
	if err := action(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(900 * time.Millisecond)
	metrics, err := client.FrameMetrics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, frame := range metrics.Recent {
		if !frame.HostCompleted {
			continue
		}
		checked++
		damage := frame.LogicalDamage
		if damage.Width <= 0 || damage.Height <= 0 || !containsRect(allowed, damage) {
			t.Fatalf("%s frame %d damage=%+v allowed=%+v", label, frame.FrameID, damage, allowed)
		}
	}
	if checked == 0 {
		t.Fatalf("%s produced no observed frames", label)
	}
}
