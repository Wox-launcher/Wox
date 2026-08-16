package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenAsAdministratorRejectsInvalidPath(t *testing.T) {
	err := OpenAsAdministrator("invalid\x00path.exe")
	if err == nil {
		t.Fatal("expected invalid path to fail")
	}
	if !strings.Contains(err.Error(), "encode ShellExecute path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunElevatedRejectsInvalidPath(t *testing.T) {
	_, err := RunElevated("invalid\x00path.exe", "-Command echo", "")
	if err == nil {
		t.Fatal("expected invalid path to fail")
	}
	if !strings.Contains(err.Error(), "encode ShellExecute path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunElevatedRejectsInvalidParameters(t *testing.T) {
	_, err := RunElevated("powershell.exe", "invalid\x00args", "")
	if err == nil {
		t.Fatal("expected invalid parameters to fail")
	}
	if !strings.Contains(err.Error(), "encode ShellExecute parameters") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestShellExecuteMaskUsesContextMenuForNamespaceObjects(t *testing.T) {
	mask := shellExecuteMask(`shell:AppsFolder\Example.App_123!App`)
	if mask&seeMaskInvokeIDList == 0 {
		t.Fatal("expected Shell namespace object to use SEE_MASK_INVOKEIDLIST")
	}

	fileMask := shellExecuteMask(`C:\Apps\Editor.exe`)
	if fileMask&seeMaskInvokeIDList != 0 {
		t.Fatal("expected filesystem path to preserve direct Shell execution")
	}
}

func TestCreateShellItemIDListHandlesSpecialPathCharacters(t *testing.T) {
	rootPath := t.TempDir()
	filePath := filepath.Join(rootPath, `$(Start-Process calc)& space dir`, `file $(Start-Process calc)& name.txt`)
	mustMkdirAll(t, filepath.Dir(filePath))
	mustWriteTestFile(t, filePath)

	itemIDList, err := createShellItemIDList(filePath)
	if err != nil {
		t.Fatalf("create Shell item ID list: %v", err)
	}
	if itemIDList == 0 {
		t.Fatal("expected non-zero Shell item ID list")
	}
	procILFree.Call(itemIDList)
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", path, err)
	}
}

func mustWriteTestFile(t *testing.T, path string) {
	t.Helper()

	if err := os.WriteFile(path, []byte("wox shell test"), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}
