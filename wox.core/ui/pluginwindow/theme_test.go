package pluginwindow

import (
	"encoding/json"
	"testing"
	"wox/common"
	"wox/plugin"
	"wox/util/overlay"
)

// TestThemeEvents checks document identity, palette changes and closed-window filtering.
func TestThemeEvents(t *testing.T) {
	theme := common.Theme{AppBackgroundColor: "#111111"}
	overlay.SetThemeProvider(func() common.Theme { return theme })
	t.Cleanup(func() { overlay.SetThemeProvider(nil) })
	var events []plugin.WindowEvent
	s := &session{emit: func(e plugin.WindowEvent) { events = append(events, e) }}
	s.themeChanged()
	if len(events) != 0 {
		t.Fatal("theme event preceded document creation")
	}
	s.document = "current"
	for _, background := range []string{"#111111", "#eeeeee", "#ffffff"} {
		theme.AppBackgroundColor = background
		s.themeChanged()
	}
	if len(events) != 3 {
		t.Fatalf("theme updates lost: %+v", events)
	}
	for i, event := range events {
		var data plugin.WindowThemeChangedData
		if err := json.Unmarshal(event.Data, &data); err != nil || data.Dark != (i == 0) || event.DocumentID != "current" || event.Type != plugin.WindowEventThemeChanged {
			t.Fatalf("wrong theme event: %+v", event)
		}
	}
	s.closed = true
	s.themeChanged()
	if len(events) != 3 {
		t.Fatal("closed document received a theme event")
	}
}
