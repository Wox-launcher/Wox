package pathquery

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestIsBrowseQuery covers complete paths and child-name completion inside existing directories.
func TestIsBrowseQuery(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "notes with spaces.txt")
	if err := os.WriteFile(file, []byte("notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WOX_FOLDER_QUERY_ROOT", root)
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	for _, tc := range []struct {
		input string
		want  bool
	}{
		{root, true},
		{root + string(os.PathSeparator), true},
		{file, true},
		{"  " + file + "  ", true},
		{"~", true},
		{"~/notes with spaces.txt", true},
		{"%WOX_FOLDER_QUERY_ROOT%/notes with spaces.txt", true},
		{filepath.Join(root, "missing"), true},
		{filepath.Join(root, "notes with"), true},
		{"~/missing", true},
		{"%WOX_FOLDER_QUERY_ROOT%/missing", true},
		{filepath.Join(root, "missing", "child"), false},
		{filepath.Join(file, "child"), false},
		{"%WOX_FOLDER_QUERY_ROOT", false},
		{"notes with spaces.txt", false},
		{".", false},
		{"", false},
	} {
		t.Run(tc.input, func(t *testing.T) {
			if got := IsBrowseQuery(tc.input); got != tc.want {
				t.Fatalf("IsBrowseQuery(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseExpandsWindowsEnv(t *testing.T) {
	root := t.TempDir()
	cursorPath := filepath.Join(root, "Programs", "cursor")
	if err := os.MkdirAll(cursorPath, 0o755); err != nil {
		t.Fatalf("create cursor folder: %v", err)
	}
	t.Setenv("LOCALAPPDATA", root)

	inputs := []string{`%LOCALAPPDATA%/Programs/cursor/`}
	if runtime.GOOS == "windows" {
		inputs = append(inputs, `%LOCALAPPDATA%\Programs\cursor\`)
	}

	for _, input := range inputs {
		path, shouldListChildren, ok := Parse(input)
		if !ok || !shouldListChildren {
			t.Fatalf("parse %q: ok=%v shouldListChildren=%v", input, ok, shouldListChildren)
		}
		if path != filepath.Clean(cursorPath) {
			t.Fatalf("parse %q = %q, want %q", input, path, cursorPath)
		}
	}
}

func TestParseExpandsEnvInsideAbsolutePath(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "tester", "docs")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("create docs folder: %v", err)
	}
	t.Setenv("WOX_FOLDER_TEST_USER", "tester")

	path, shouldListChildren, ok := Parse(filepath.Join(root, "%WOX_FOLDER_TEST_USER%", "docs"))
	if !ok || shouldListChildren {
		t.Fatalf("ok=%v shouldListChildren=%v", ok, shouldListChildren)
	}
	if path != filepath.Clean(target) {
		t.Fatalf("path = %q, want %q", path, target)
	}
}

func TestParseRejectsUnsetEnv(t *testing.T) {
	_ = os.Unsetenv("WOX_FOLDER_MISSING_ENV")

	if _, _, ok := Parse(`%WOX_FOLDER_MISSING_ENV%/foo`); ok {
		t.Fatal("unset environment variable should not parse as a folder path")
	}
}

func TestParseAcceptsProgramFilesX86(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ProgramFiles(x86)", root)

	path, shouldListChildren, ok := Parse(`%ProgramFiles(x86)%`)
	if !ok || shouldListChildren {
		t.Fatalf("ok=%v shouldListChildren=%v", ok, shouldListChildren)
	}
	if path != filepath.Clean(root) {
		t.Fatalf("path = %q, want %q", path, root)
	}
}

func TestParseRejectsPartialPercentToken(t *testing.T) {
	if _, _, ok := Parse(`%LOCALAPPDATA`); ok {
		t.Fatal("unclosed environment variable should not parse as a folder path")
	}
	if _, _, ok := Parse(`50% complete`); ok {
		t.Fatal("percent text should not parse as a folder path")
	}
}
