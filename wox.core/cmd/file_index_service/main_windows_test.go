//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestServiceHelperHasMinimalDependencies(t *testing.T) {
	output, err := exec.Command("go", "list", "-f", `{{range .Deps}}{{println .}}{{end}}`, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go list failed: %v\n%s", err, output)
	}
	disallowed := []string{
		"wox/util",
		"wox/util/shell",
		"net/http",
		"go.uber.org/zap",
		"github.com/klauspost/compress",
	}
	for _, dep := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		dep = strings.TrimSpace(dep)
		for _, forbidden := range disallowed {
			if dep == forbidden {
				t.Fatalf("file index service helper must not depend on %s", forbidden)
			}
		}
	}
}
