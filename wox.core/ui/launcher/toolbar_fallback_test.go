package launcher

import (
	"context"
	"testing"

	"wox/common"
)

func TestSystemToolbarMessageTakesPriorityOverPlugin(t *testing.T) {
	app := bindTestApp(&App{lifecycleCtx: context.Background(), toolbar: toolbarController{toolbarMsg: &toolbarMessage{ID: "plugin", Title: "Indexing"}}})
	app.toolbar.applySystemToolbarMessage(toolbarMessage{ID: "warning", Title: "Main hotkey unavailable"}, common.SystemToolbarPriorityWarning, 0)
	if got := app.toolbar.effectiveToolbarMessage(); got == nil || got.ID != "warning" {
		t.Fatalf("system message = %+v", got)
	}
	app.toolbar.clearSystemToolbarMessage("warning")
	if got := app.toolbar.effectiveToolbarMessage(); got == nil || got.ID != "plugin" {
		t.Fatalf("restored plugin message = %+v", got)
	}
}
