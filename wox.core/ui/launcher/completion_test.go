package launcher

import "testing"

// Deleting several characters before another backend response must keep the hint visible.
func TestCompletionHintRapidDeletion(t *testing.T) {
	a := &App{completionHint: &queryCompletionHint{
		InputPrefix: "settin", CompletionText: "setting", Suffix: "g", DeletionReuseMinLength: 3,
	}}
	for _, text := range []string{"setti", "sett", "set"} {
		a.reuseCompletionHintLocked(text)
		if a.completionHint == nil || a.completionHint.InputPrefix != text || text+a.completionHint.Suffix != "setting" {
			t.Fatalf("hint after deletion to %q = %#v", text, a.completionHint)
		}
	}
	a.reuseCompletionHintLocked("se")
	if a.completionHint != nil {
		t.Fatal("hint must disappear below the history minimum")
	}
}

func TestCompletionHintReuseBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, prefix, completion, text string
		minimum                        int
		want                           bool
	}{
		{"append", "set", "setting", "sett", 0, true},
		{"complete", "sett", "setting", "setting", 3, false},
		{"clear", "sett", "setting", "", 3, false},
		{"replace", "sett", "setting", "song", 3, false},
		{"plugin command", "cmd sta", "cmd start ", "cmd st", 0, false},
		{"unicode", "你好世界", "你好世界啊", "你好世", 3, true},
		{"unicode minimum", "你好世", "你好世界", "你好", 3, false},
		{"whitespace minimum", "  sett", "  setting", "  se", 3, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &App{completionHint: &queryCompletionHint{InputPrefix: tc.prefix, CompletionText: tc.completion, DeletionReuseMinLength: tc.minimum}}
			a.reuseCompletionHintLocked(tc.text)
			if (a.completionHint != nil) != tc.want {
				t.Fatalf("hint = %#v, want present %t", a.completionHint, tc.want)
			}
		})
	}
}
