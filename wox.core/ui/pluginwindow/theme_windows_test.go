package pluginwindow

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"wox/common"
	"wox/plugin"
	ui "wox/ui/runtime"
	"wox/util/overlay"
)

// TestNativeHiddenWindowThemeChanges exercises the real subscription/lifecycle
// path without showing or activating any window or starting the Wox launcher.
func TestNativeHiddenWindowThemeChanges(t *testing.T) {
	if os.Getenv("WOX_WINDOW_THEME_NATIVE_TEST") != "1" {
		t.Skip("set WOX_WINDOW_THEME_NATIVE_TEST=1 for hidden-window theme integration")
	}
	err := ui.Run(func() error {
		previous := overlay.WindowManager()
		overlay.SetWindowManager(ui.NewWindowManager())
		defer overlay.SetWindowManager(previous)
		theme := common.Theme{AppBackgroundColor: "#111111"}
		overlay.SetThemeProvider(func() common.Theme { return theme })
		defer overlay.SetThemeProvider(nil)
		var events []plugin.WindowEvent
		driver, err := Create("hidden-theme-test", plugin.CreateWindowOption{Content: plugin.WindowContent{Type: plugin.WindowContentText, Data: "hidden"}}, func(e plugin.WindowEvent) { events = append(events, e) })
		if err != nil {
			return err
		}
		defer driver.Close()
		if len(events) != 1 || events[0].Type != plugin.WindowEventReady {
			return fmt.Errorf("unexpected initial events: %+v", events)
		}
		theme.AppBackgroundColor = "#eeeeee"
		// The receiving overlay's appearance takes precedence over the launcher argument.
		overlay.NotifyThemeChanged(true)
		if len(events) != 2 || events[1].Type != plugin.WindowEventThemeChanged {
			return fmt.Errorf("hidden window missed update: %+v", events)
		}
		old := events[1]
		var data plugin.WindowThemeChangedData
		if err := json.Unmarshal(old.Data, &data); err != nil || data.Dark || !driver.AcceptEvent(old) {
			return fmt.Errorf("incorrect theme event: %+v", old)
		}
		if err := driver.SetContent(plugin.WindowContent{Type: plugin.WindowContentWoxView, Data: `{"version":1,"root":{"type":"view"}}`}); err != nil {
			return err
		}
		if driver.AcceptEvent(old) {
			return fmt.Errorf("replaced document accepted queued theme event")
		}
		overlay.NotifyThemeChanged(false)
		latest := events[len(events)-1]
		if latest.Type != plugin.WindowEventThemeChanged || latest.DocumentID == old.DocumentID || !driver.AcceptEvent(latest) {
			return fmt.Errorf("new document lost theme update: %+v", latest)
		}
		if err := driver.Close(); err != nil {
			return err
		}
		count := len(events)
		overlay.NotifyThemeChanged(true)
		if len(events) != count || driver.(*session).unsubscribeTheme != nil || driver.AcceptEvent(latest) {
			return fmt.Errorf("closed window retained theme delivery")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
