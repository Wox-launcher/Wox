package system

import (
	"path/filepath"
	"testing"
)

func TestParseFileSearchRootSettingJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	docs := filepath.Join(home, "Docs")

	got, err := parseFileSearchRootSettingJSON(`[{"Path":"` + filepath.ToSlash(docs) + `"},{"Path":""},{"Path":"~/Notes"}]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("parsed roots = %#v, want two expanded paths", got)
	}
	if got[0] != filepath.Clean(docs) {
		t.Fatalf("absolute root = %q, want %q", got[0], filepath.Clean(docs))
	}
	if got[1] != filepath.Clean(filepath.Join(home, "Notes")) {
		t.Fatalf("home-relative root = %q, want expanded Notes", got[1])
	}
}

func TestParseFileSearchRootSettingJSONRejectsInvalidJSON(t *testing.T) {
	if _, err := parseFileSearchRootSettingJSON("{"); err == nil {
		t.Fatal("expected invalid JSON to fail")
	}
}

func TestParseFileSearchRootSettingJSONEmptyArray(t *testing.T) {
	got, err := parseFileSearchRootSettingJSON("[]")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("empty table = %#v, want no paths", got)
	}
}
