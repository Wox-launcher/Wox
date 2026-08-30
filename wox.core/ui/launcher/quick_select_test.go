package launcher

import (
	"runtime"
	"testing"
	"time"

	woxui "wox/ui/runtime"
)

func TestQuickSelectNumbersSkipGroupsAndCapAtNine(t *testing.T) {
	results := []queryResult{
		{ID: "g", Title: "Apps", IsGroup: true},
		{ID: "a", Title: "Alpha"},
		{ID: "b", Title: "Beta"},
		{ID: "c", Title: "Gamma"},
	}
	visible := []bool{true, true, true, false}
	if got := quickSelectNumberFor(results, visible, 1); got != "1" {
		t.Fatalf("first selectable number = %q, want 1", got)
	}
	if got := quickSelectNumberFor(results, visible, 0); got != "" {
		t.Fatalf("group number = %q, want empty", got)
	}
	if got := quickSelectNumberFor(results, visible, 3); got != "" {
		t.Fatalf("hidden row number = %q, want empty", got)
	}
	if got := quickSelectResultIndex(results, visible, 2); got != 2 {
		t.Fatalf("number 2 index = %d, want 2", got)
	}
	if got := quickSelectResultIndex(results, visible, 3); got != -1 {
		t.Fatalf("hidden number 3 index = %d, want -1", got)
	}

	many := make([]queryResult, 12)
	allVisible := allQuickSelectVisible(len(many))
	if got := quickSelectNumberFor(many, allVisible, 8); got != "9" {
		t.Fatalf("ninth visible number = %q, want 9", got)
	}
	if got := quickSelectNumberFor(many, allVisible, 9); got != "" {
		t.Fatalf("tenth visible number = %q, want empty", got)
	}
}

func TestQuickSelectVisibleListRangeIgnoresOverscan(t *testing.T) {
	visible := quickSelectVisibleListResults(make([]queryResult, 10), 0, 200, 0, 50, 0, 0)
	if !visible[0] || !visible[3] || visible[4] {
		t.Fatalf("list visible = %v, want first four 50px rows in a 200px viewport", visible)
	}
}

func TestQuickSelectVisibleListRangeIncludesPartiallyVisibleScaledRow(t *testing.T) {
	visible := quickSelectVisibleListResults(make([]queryResult, 4), 0, 101.2, 0, 50.5, 0, 0)
	if !visible[2] || visible[3] {
		t.Fatalf("list visible = %v, want the partially visible third scaled row", visible)
	}
}

func TestQuickSelectVisibleListRangeUsesGroupHeaderHeight(t *testing.T) {
	results := []queryResult{{ID: "app"}, {ID: "files", IsGroup: true}, {ID: "readme"}}
	visible := quickSelectVisibleListResults(results, 0, 70, 0, 56, 28, 0)
	if !visible[0] || !visible[1] || visible[2] {
		t.Fatalf("list visible = %v, want the first result and compact Files header", visible)
	}
}

func TestQuickSelectModifierAndDigitHelpers(t *testing.T) {
	if quickSelectDigit(woxui.Key("1")) != 1 || quickSelectDigit(woxui.Key("9")) != 9 || quickSelectDigit(woxui.Key("0")) != 0 {
		t.Fatal("quick select digits should accept 1-9 only")
	}
	event := woxui.KeyEvent{Key: woxui.KeyAlt, Modifiers: woxui.KeyModifierAlt, Down: true}
	if runtime.GOOS == "darwin" {
		event = woxui.KeyEvent{Key: woxui.KeyMeta, Modifiers: woxui.KeyModifierMeta, Down: true}
	}
	if !isQuickSelectModifierKeyOnly(event) {
		t.Fatal("platform hold modifier should start quick select")
	}
	event.Modifiers |= woxui.KeyModifierShift
	if isQuickSelectModifierKeyOnly(event) {
		t.Fatal("shift plus the hold modifier should not start quick select")
	}
}

func TestQuickSelectHoldShowsNumbersAndReleaseHidesThem(t *testing.T) {
	app := &App{
		results:      []queryResult{{ID: "one", Title: "One"}, {ID: "two", Title: "Two"}},
		lifecycleCtx: t.Context(),
	}
	event := quickSelectModifierEvent(true)
	if !app.onQuickSelectKey(event) || app.quickSelectTimer == nil {
		t.Fatal("holding the quick select modifier should start the delay timer")
	}
	app.quickSelectTimer.Stop()
	app.quickSelectTimer = nil
	app.activateQuickSelectModeLocked()
	if !app.quickSelectMode {
		t.Fatal("quick select mode should activate after the hold delay")
	}

	if !app.onQuickSelectKey(quickSelectModifierEvent(false)) || app.quickSelectMode {
		t.Fatal("releasing the hold modifier should hide quick select numbers")
	}
}

func TestQuickSelectNumberActivatesVisibleResult(t *testing.T) {
	app := &App{
		results: []queryResult{
			{ID: "group", Title: "Group", IsGroup: true},
			{ID: "one", Title: "One"},
		},
		lifecycleCtx: t.Context(),
	}
	app.activateQuickSelectModeLocked()
	if !app.onQuickSelectKey(woxui.KeyEvent{Key: woxui.Key("1"), Down: true}) {
		t.Fatal("digit 1 should activate the first visible non-group result")
	}
	if app.selected != 1 || app.quickSelectMode {
		t.Fatalf("quick select activation = selected %d mode %v, want 1/false", app.selected, app.quickSelectMode)
	}
}

func TestQuickSelectDoesNotStartWithoutResults(t *testing.T) {
	app := &App{results: []queryResult{{ID: "g", IsGroup: true}}}
	if app.onQuickSelectKey(quickSelectModifierEvent(true)) || app.quickSelectTimer != nil {
		t.Fatal("quick select should not start when there is no selectable result")
	}
}

func TestQuickSelectBlockedByActionPanel(t *testing.T) {
	app := &App{
		results:     []queryResult{{ID: "one", Title: "One"}},
		actionPanel: true,
	}
	if app.onQuickSelectKey(quickSelectModifierEvent(true)) || app.quickSelectTimer != nil {
		t.Fatal("quick select should stay off while the action panel is open")
	}
}

func TestQuickSelectHoldWaitsBeforeActivating(t *testing.T) {
	app := &App{results: []queryResult{{ID: "one", Title: "One"}}, lifecycleCtx: t.Context()}
	if !app.onQuickSelectKey(quickSelectModifierEvent(true)) {
		t.Fatal("modifier hold should be consumed")
	}
	if app.quickSelectMode {
		t.Fatal("quick select should wait for the hold delay before showing numbers")
	}
	if app.quickSelectTimer != nil {
		app.quickSelectTimer.Stop()
		app.quickSelectTimer = nil
	}
}

func quickSelectModifierEvent(down bool) woxui.KeyEvent {
	if runtime.GOOS == "darwin" {
		return woxui.KeyEvent{Key: woxui.KeyMeta, Modifiers: woxui.KeyModifierMeta, Down: down}
	}
	return woxui.KeyEvent{Key: woxui.KeyAlt, Modifiers: woxui.KeyModifierAlt, Down: down}
}

func TestQuickSelectTimerCancelledOnOtherKey(t *testing.T) {
	app := &App{results: []queryResult{{ID: "one", Title: "One"}}, lifecycleCtx: t.Context()}
	if !app.onQuickSelectKey(quickSelectModifierEvent(true)) || app.quickSelectTimer == nil {
		t.Fatal("expected a pending quick select timer")
	}
	app.onQuickSelectKey(woxui.KeyEvent{Key: woxui.KeyArrowDown, Down: true})
	time.Sleep(10 * time.Millisecond)
	if app.quickSelectKeyPressed || app.quickSelectMode {
		t.Fatal("another key should cancel the pending quick select hold")
	}
}

func TestQuickSelectTimerCancelledBeforeFormConsumesKey(t *testing.T) {
	app := &App{
		results:        []queryResult{{ID: "one", Title: "One"}},
		lifecycleCtx:   t.Context(),
		hotkeySettings: newHotkeySettingsController(CommonDeps{}),
	}
	if !app.onQuickSelectKey(quickSelectModifierEvent(true)) || app.quickSelectTimer == nil {
		t.Fatal("expected a pending quick select timer")
	}
	app.form = &formState{}
	if !app.onKey(woxui.KeyEvent{Key: woxui.KeyEnter, Down: true}) {
		t.Fatal("the form should consume Enter")
	}
	if app.quickSelectKeyPressed || app.quickSelectTimer != nil {
		t.Fatal("a feature-local key handler must not leave the quick select timer pending")
	}
}
