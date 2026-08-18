//go:build wox_ui_smoke

package emoji

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
)

// Test001LauncherQueryEmoji verifies that the Emoji plugin searches its grid catalog by glyph and text.
// Flow: query the robot glyph -> replace the query with "dog" -> inspect each completed result generation.
// Evidence: the glyph query has one robot result and the text query contains the localized exact dog result.
func Test001LauncherQueryEmoji(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)

		glyphSnapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "emoji 🤖")
		glyphResults := emojiResults(glyphSnapshot)
		if len(glyphResults) != 1 || (glyphResults[0].Label != "robot" && glyphResults[0].Label != "机器人") {
			t.Fatalf("robot glyph results = %+v, want one localized robot result", glyphResults)
		}
		if !glyphResults[0].Selected {
			t.Fatal("robot glyph result is not selected")
		}
		smoke.AssertNoDiagnostics(t, glyphSnapshot)

		dogSnapshot := smoke.ReplaceLauncherQuery(t, ctx, client, "emoji dog")
		dog, found := emojiResultByLabel(dogSnapshot, "dog", "狗")
		if !found {
			t.Fatalf("text query results = %+v, want localized exact dog result", emojiResults(dogSnapshot))
		}
		if dog.AutomationID == "" {
			t.Fatal("dog result does not expose an automation ID")
		}
		smoke.AssertNoDiagnostics(t, dogSnapshot)
	})
}
