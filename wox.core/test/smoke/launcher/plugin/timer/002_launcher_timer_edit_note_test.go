//go:build wox_ui_smoke

package timer

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test002LauncherTimerEditNote verifies editing an active Timer refreshes both Launcher and desktop overlay content.
// Flow: start a pinned Timer -> edit its note through the result form -> hover the existing overlay -> delete the Timer.
// Evidence: the refreshed Timer result and the same native overlay both expose the new note before cleanup removes the overlay.
func Test002LauncherTimerEditNote(t *testing.T) {
	const (
		originalNote = "wox smoke old timer note"
		updatedNote  = "wox smoke updated timer note"
	)

	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		cleanupTimer(t, client, originalNote)
		cleanupTimer(t, client, updatedNote)
		smoke.ShowLauncher(t, ctx, client)
		timerID := startPinnedTimer(t, ctx, client, "2m", originalNote)
		overlayInstance := timerOverlayInstancePrefix + timerID

		selectTimerByNote(t, ctx, client, originalNote)
		smoke.ActivateSelectedResultAction(t, ctx, client, "action-result-"+timerID+":edit_note-")
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			field, fieldFound := automationdriver.Find(snapshot, "action-form-field-0")
			_, saveFound := automationdriver.Find(snapshot, "form-save")
			return fieldFound && field.Value == originalNote && saveFound
		}); err != nil {
			t.Fatalf("wait for Timer note form: %v", err)
		}
		if err := client.Perform(ctx, "action-form-field-0", woxui.AccessibilityActionSetValue, updatedNote); err != nil {
			t.Fatalf("edit Timer note: %v", err)
		}
		if err := client.Perform(ctx, "form-save", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("save Timer note: %v", err)
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			result, found := timerResultByNote(snapshot, updatedNote)
			_, formOpen := automationdriver.Find(snapshot, "form-save")
			results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
			return found && result.AutomationID == "launcher.result."+timerID && !formOpen && resultsFound && results.Value == "complete"
		})
		if err != nil {
			t.Fatalf("wait for Timer result note refresh: %v", err)
		}

		if err := client.FocusInstance(ctx, overlayInstance); err != nil {
			t.Fatalf("focus Timer desktop overlay after editing note: %v", err)
		}
		if err := client.Pointer(ctx, woxui.PointerEvent{Kind: woxui.PointerEnter}); err != nil {
			t.Fatalf("hover Timer desktop overlay after editing note: %v", err)
		}
		snapshot, err = client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			note, found := automationdriver.Find(snapshot, "timer-overlay-note")
			return found && note.Value == updatedNote
		})
		if err != nil {
			t.Fatalf("wait for updated Timer overlay note: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)

		deleteTimer(t, ctx, client, updatedNote)
		if _, err := client.WaitForWindowState(ctx, overlayInstance, func(state automationdriver.WindowState) bool {
			return !state.Exists
		}); err != nil {
			t.Fatalf("wait for edited Timer overlay cleanup: %v", err)
		}
	})
}
