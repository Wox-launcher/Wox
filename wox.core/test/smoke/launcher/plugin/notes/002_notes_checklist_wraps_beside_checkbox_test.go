//go:build wox_ui_smoke

package notes

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
)

// Test002NotesChecklistWrapsBesideCheckbox verifies a long checklist keeps the first glyphs beside the checkbox.
// Flow: create a Notes window -> enter one overflowing CJK task -> read the editor's visual lines.
// Evidence: the first painted line still contains the checkbox prefix and following CJK instead of leaving the marker alone.
func Test002NotesChecklistWrapsBesideCheckbox(t *testing.T) {
	const title = "Wox Notes Checklist Wrap"
	markdown := "# " + title + "\n- [ ] " + longChecklistBody
	projected := title + "\n☐ " + longChecklistBody

	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		openNewNoteEditor(t, ctx, client)
		snapshot := enterNoteMarkdown(t, ctx, client, markdown, projected)
		smoke.AssertNoDiagnostics(t, snapshot)
		lines := waitForEditorVisualLines(t, ctx, client, checklistKeepsTextBesideCheckbox)
		index, _ := findChecklistVisualLine(lines)
		if utf8.RuneCountInString(lines[index].Text) <= 2 {
			t.Fatalf("checklist visual line = %q, want checkbox plus task text", lines[index].Text)
		}
	})
}

func checklistKeepsTextBesideCheckbox(lines []woxui.AccessibilityTextLine) bool {
	index, ok := findChecklistVisualLine(lines)
	if !ok {
		return false
	}
	first := lines[index].Text
	return strings.Contains(first, "顺") && first != "☐ " && first != "☐"
}
