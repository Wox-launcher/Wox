package shell

import "testing"

func TestBuildOpenFileInFolderCommandUsesExplorerArguments(t *testing.T) {
	path := `C:\tmp\$(Start-Process calc)& test\file name.txt`

	name, args := buildOpenFileInFolderCommand(path)

	if name != "explorer.exe" {
		t.Fatalf("expected explorer.exe, got %q", name)
	}
	if len(args) != 2 {
		t.Fatalf("expected two arguments, got %d: %#v", len(args), args)
	}
	if args[0] != "/select," {
		t.Fatalf("expected select flag, got %q", args[0])
	}
	if args[1] != path {
		t.Fatalf("expected path argument to preserve path as data, got %q", args[1])
	}
}
