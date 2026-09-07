//go:build wox_ui_smoke

package notes

import (
	"context"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
)

// Test003NotesChecklistHangingIndent verifies wrapped checklist text continues under the first line of task text.
// Flow: create a Notes window -> enter one overflowing CJK task -> read the editor's visual lines.
// Evidence: later painted lines keep a hanging indent and do not restart under the checkbox.
func Test003NotesChecklistHangingIndent(t *testing.T) {
	const title = "Wox Notes Checklist Indent"
	markdown := "# " + title + "\n- [ ] " + longChecklistBody
	projected := title + "\n☐ " + longChecklistBody

	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		openNewNoteEditor(t, ctx, client)
		snapshot := enterNoteMarkdown(t, ctx, client, markdown, projected)
		smoke.AssertNoDiagnostics(t, snapshot)
		lines := waitForEditorVisualLines(t, ctx, client, checklistUsesHangingIndent)
		index, _ := findChecklistVisualLine(lines)
		if lines[index].Indent != 0 {
			t.Fatalf("checklist first line indent = %v, want 0", lines[index].Indent)
		}
		if strings.Contains(lines[index+1].Text, "☐") {
			t.Fatalf("wrapped visual line = %q, want continuation text without the checkbox", lines[index+1].Text)
		}
	})
}

func checklistUsesHangingIndent(lines []woxui.AccessibilityTextLine) bool {
	index, ok := findChecklistVisualLine(lines)
	if !ok || index+1 >= len(lines) {
		return false
	}
	return lines[index].Indent == 0 && lines[index+1].Indent > 0 && !strings.HasPrefix(lines[index+1].Text, "☐")
}
