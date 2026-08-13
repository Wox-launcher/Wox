package launcher

import "testing"

func TestEffectiveToolbarMessageRestoresFallback(t *testing.T) {
	fallback := &toolbarMessage{ID: "fallback", Title: "Main hotkey unavailable"}
	pluginMessage := &toolbarMessage{ID: "plugin", Title: "Indexing"}
	app := &App{toolbarFallbackMsg: fallback}

	if got := app.effectiveToolbarMessage(); got != fallback {
		t.Fatalf("effective toolbar message = %+v, want fallback", got)
	}
	app.toolbarMsg = pluginMessage
	if got := app.effectiveToolbarMessage(); got != pluginMessage {
		t.Fatalf("effective toolbar message = %+v, want plugin message", got)
	}
	app.toolbarMsg = nil
	if got := app.effectiveToolbarMessage(); got != fallback {
		t.Fatalf("effective toolbar message after clear = %+v, want fallback", got)
	}
}
