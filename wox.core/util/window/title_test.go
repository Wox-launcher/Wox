package window

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestAppNameExtractsTrailingApplication(t *testing.T) {
	cases := []struct {
		title string
		want  string
	}{
		{title: "Untitled-1 - Wox (Workspace) - Visual Studio Code", want: "Visual Studio Code"},
		{title: "README.md — Mozilla Firefox", want: "Mozilla Firefox"},
		{title: "notes.txt - Notepad", want: "Notepad"},
		{title: "Visual Studio Code", want: "Visual Studio Code"},
	}
	for _, tc := range cases {
		if got := AppName(tc.title); got != tc.want {
			t.Fatalf("AppName(%q) = %q, want %q", tc.title, got, tc.want)
		}
		if got := CompactTitle(tc.title, ActionTitleMaxRunes); got != tc.want {
			t.Fatalf("CompactTitle(%q) = %q, want %q", tc.title, got, tc.want)
		}
	}
}

func TestCompactTitleTruncatesTitlesWithoutAppSeparator(t *testing.T) {
	longTitle := strings.Repeat("文档", 36)
	got := CompactTitle(longTitle, ActionTitleMaxRunes)
	if utf8.RuneCountInString(got) != ActionTitleMaxRunes || !strings.HasSuffix(got, "…") {
		t.Fatalf("truncated title = %q (%d runes), want %d runes ending with ellipsis", got, utf8.RuneCountInString(got), ActionTitleMaxRunes)
	}
}
