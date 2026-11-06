package pluginwindow

import (
	"encoding/json"
	"testing"
	"wox/common"
	"wox/plugin"
	"wox/util/overlay"
)

// TestThemeEventWaitsForReady preserves readiness ordering and the latest
// surface appearance when a hidden HTML document is still loading.
func TestThemeEventWaitsForReady(t *testing.T) {
	theme := common.Theme{AppBackgroundColor: "#111111"}
	overlay.SetThemeProvider(func() common.Theme { return theme })
	t.Cleanup(func() { overlay.SetThemeProvider(nil) })
	var events []plugin.WindowEvent
	s := &session{document: "current", content: plugin.WindowContent{Type: plugin.WindowContentHTML}, emit: func(e plugin.WindowEvent) { events = append(events, e) }}
	s.themeChanged()
	theme.AppBackgroundColor = "#eeeeee"
	s.themeChanged()
	if len(events) != 0 {
		t.Fatal("theme event preceded document ready")
	}
	s.message(`{"Token":"current","Type":"ready"}`)
	if len(events) != 2 || events[0].Type != plugin.WindowEventReady || events[1].Type != plugin.WindowEventThemeChanged {
		t.Fatalf("ready/theme ordering: %+v", events)
	}
	var data plugin.WindowThemeChangedData
	if err := json.Unmarshal(events[1].Data, &data); err != nil || data.Dark || events[1].DocumentID != "current" {
		t.Fatalf("wrong theme payload: %+v", events[1])
	}
	s.flushThemeChange()
	if len(events) != 2 {
		t.Fatal("duplicate pending theme event")
	}
	// A palette update still matters when it preserves the same light appearance.
	s.themeChanged()
	if len(events) != 3 {
		t.Fatal("same-appearance theme update was lost")
	}
	s.closed = true
	s.themeChanged()
	if len(events) != 3 {
		t.Fatal("closed document received a theme event")
	}
}
