package launcher

import (
	"testing"

	woxui "wox/ui/runtime"
)

func TestSettingsWindowSessionTransitionsDropPageOverlays(t *testing.T) {
	app := newApp(false, nil, woxui.NewWindowManager(), newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer app.cancel()
	c := &app.settingsWindow
	showOverlays := func() {
		c.demo = &settingsDemoState{kind: "query-hotkeys"}
		c.cloudPlanTooltip = &cloudPlanTooltipState{}
		c.inlineTooltip = &settingsInlineTooltipState{Text: "tip"}
	}

	showOverlays()
	c.beginSessionLocked("hotkey")
	if !c.open || c.tab != "hotkey" || c.row != 0 {
		t.Fatalf("session = open %t tab %q row %d, want open on hotkey row 0", c.open, c.tab, c.row)
	}
	if c.demo != nil || c.cloudPlanTooltip != nil || c.inlineTooltip != nil {
		t.Fatal("opening Settings must drop overlays anchored to the previous page")
	}

	c.row = 3
	showOverlays()
	demoRevision := c.demoRevision.Load()
	c.selectTabLocked("ai")
	if c.tab != "ai" || c.row != 0 || c.demo != nil || c.cloudPlanTooltip != nil || c.inlineTooltip != nil {
		t.Fatal("switching tabs must reset the row and overlays")
	}
	if c.demoRevision.Load() == demoRevision {
		t.Fatal("switching tabs must invalidate pending delayed overlays")
	}

	showOverlays()
	c.saving = true
	c.tableEditor = &formTableEditorState{}
	c.flashLocked("built-in:MainHotkey")
	if !c.endSessionLocked() {
		t.Fatal("endSessionLocked must report the open session")
	}
	if c.open || c.saving || c.tableEditor != nil || c.flash != "" || c.flashTimer != nil {
		t.Fatal("closing Settings must release session state")
	}
	if c.demo != nil || c.cloudPlanTooltip != nil || c.inlineTooltip != nil {
		t.Fatal("closing Settings must drop overlays")
	}
	if c.endSessionLocked() {
		t.Fatal("a second close must not report an open session")
	}
}
