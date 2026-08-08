package preview

import "testing"

func TestParseUpdateReleaseNotes(t *testing.T) {
	parsed := parseUpdateReleaseNotes("Intro paragraph\n![shot](https://example.com/shot.png)\n\n- Add\n  - [`Dictation`] Add offline dictation\n    Continued detail\n\n- Fix\n  - Fix updater state")
	if parsed.intro != "Intro paragraph\n![shot](https://example.com/shot.png)" {
		t.Fatalf("intro = %q", parsed.intro)
	}
	if len(parsed.sections) != 2 || parsed.sections[0].title != "Add" || len(parsed.sections[0].items) != 1 {
		t.Fatalf("sections = %#v", parsed.sections)
	}
	item := parsed.sections[0].items[0]
	if item.tag != "Dictation" || item.summary != "Add offline dictation" || item.continuation != "Continued detail" {
		t.Fatalf("item = %#v", item)
	}
}
