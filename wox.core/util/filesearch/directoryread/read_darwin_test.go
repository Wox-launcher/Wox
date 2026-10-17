//go:build darwin && cgo

package directoryread

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestNativeBulkMatchesLstat covers metadata, separate hard-link names and a
// directory symlink, which must remain a link instead of a traversal target.
func TestNativeBulkMatchesLstat(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "项目.txt")
	if err := os.WriteFile(file, []byte("bulk attributes"), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(file, filepath.Join(root, "hardlink")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "directory"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("directory", filepath.Join(root, "symlink")); err != nil {
		t.Fatal(err)
	}
	entries, err := ReadDir(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("entries=%d", len(entries))
	}
	for _, entry := range entries {
		got, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.Lstat(filepath.Join(root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		// Directory st_size is not part of the bulk contract; the decoded entry
		// must still be complete so the scanner does not Lstat every directory.
		wantSize := want.Size()
		if want.IsDir() {
			wantSize = 0
			if _, ok := got.(bulkInfo); !ok {
				t.Fatalf("%s: directory metadata fell back to Lstat", entry.Name())
			}
		}
		if got.Size() != wantSize || got.Mode() != want.Mode() || !got.ModTime().Equal(want.ModTime()) || got.IsDir() != want.IsDir() {
			t.Fatalf("%s: got %+v, want %+v", entry.Name(), got, want)
		}
	}
}
