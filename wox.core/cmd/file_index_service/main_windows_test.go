//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNonEmptyArgsTreatsLegacyEmptySCMArgumentAsServiceMode(t *testing.T) {
	if args := nonEmptyArgs([]string{""}); len(args) != 0 {
		t.Fatalf("legacy empty SCM argument = %q, want service mode", args)
	}
	args := nonEmptyArgs([]string{"install", "", "--result", `C:\result.txt`})
	if len(args) != 3 || argumentValue(args, "--result") != `C:\result.txt` {
		t.Fatalf("filtered command arguments = %q", args)
	}
}

func TestRemoveInstallDirectoryStaysInsideWoxServiceRoot(t *testing.T) {
	programFiles := filepath.Join(t.TempDir(), "Program Files")
	t.Setenv("ProgramFiles", programFiles)
	serviceRoot := filepath.Join(programFiles, "Wox", "FileIndexService")
	serviceBinary := filepath.Join(serviceRoot, "2.4.1", "wox-file-index-service.exe")
	if err := os.MkdirAll(filepath.Dir(serviceBinary), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(serviceBinary, []byte("service"), 0644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(programFiles, "keep.exe")
	if err := os.WriteFile(outside, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	removeInstallDirectory(outside)
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside file was removed: %v", err)
	}
	removeInstallDirectory(serviceBinary)
	if _, err := os.Stat(serviceRoot); !os.IsNotExist(err) {
		t.Fatalf("service root still exists: %v", err)
	}
}
