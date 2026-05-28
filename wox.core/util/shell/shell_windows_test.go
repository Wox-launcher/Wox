package shell

import "testing"

func TestBuildOpenFileInFolderCommandUsesExplorerArguments(t *testing.T) {
	path := `C:\tmp\$(Start-Process calc)& test\file name.txt`

	name, args := buildOpenFileInFolderCommand(path)

	if name != "explorer.exe" {
		t.Fatalf("expected explorer.exe, got %q", name)
	}
	if len(args) != 1 {
		t.Fatalf("expected one argument, got %d: %#v", len(args), args)
	}
	if args[0] != `/select,`+path {
		t.Fatalf("expected select argument to preserve path as data, got %q", args[0])
	}
}
