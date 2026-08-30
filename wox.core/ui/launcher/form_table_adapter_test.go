package launcher

import (
	"testing"

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
