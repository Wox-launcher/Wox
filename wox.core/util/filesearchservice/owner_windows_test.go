//go:build windows

package filesearchservice

import (
	"path/filepath"
	"testing"
)

func TestValidateIndexDirectoryForProfile(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "owner")
	want := filepath.Join(profile, ".wox", "filesearch", IndexDirectory)
	if err := validateIndexDirectoryForProfile(profile, want); err != nil {
		t.Fatal(err)
	}
	if err := validateIndexDirectoryForProfile(profile, filepath.Join(profile, ".wox", "filesearch", "other")); err == nil {
		t.Fatal("accepted a sibling cache directory")
	}
	if err := validateIndexDirectoryForProfile(profile, filepath.Join(profile, "..", "other", IndexDirectory)); err == nil {
		t.Fatal("accepted a directory outside the owner profile")
	}
}
