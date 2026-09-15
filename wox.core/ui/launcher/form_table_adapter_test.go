package launcher

import (
	"testing"
	woxcomponent "wox/ui/launcher/component"

	woxui "wox/ui/runtime"
)

func TestFormTableHeaderWeightKeepsPluginTablesRegular(t *testing.T) {
	if got := formTableHeaderWeight("plugin-settings"); got != woxui.FontWeightRegular {
		t.Fatalf("plugin settings header weight = %v, want regular", got)
	}
	if got := formTableHeaderWeight("hotkey-settings"); got != woxui.FontWeightSemibold {
		t.Fatalf("general settings header weight = %v, want semibold", got)
	}
	if got := formTableHeaderWeight("ai-settings"); got != woxui.FontWeightSemibold {
		t.Fatalf("AI settings header weight = %v, want semibold", got)
	}
}

// TestFormTableHotkeysFormatOnlyTheVisibleLabel protects stored values and search
// text while reusing the same key labels as the hotkey recorder.
func TestFormTableHotkeysFormatOnlyTheVisibleLabel(t *testing.T) {
	a := &App{}
	for _, tc := range []struct{ raw, want string }{
		{"capslock+e", "Caps Lock + E"}, {"ctrl+ctrl", "Ctrl + Ctrl"},
		{"shift+ctrl+l", "Shift + Ctrl + L"}, {"", ""},
	} {
		row := map[string]any{"Hotkey": tc.raw}
		cell := a.formTableViewCell(formTableColumn{Key: "Hotkey", Type: "hotkey"}, row, woxcomponent.ControlTheme{}, 1)
		if cell.Text != tc.want || cell.SearchText != tc.raw || row["Hotkey"] != tc.raw {
			t.Fatalf("hotkey %q: cell=%#v row=%v", tc.raw, cell, row)
		}
	}
}
