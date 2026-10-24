package shell

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"golang.org/x/sys/windows"
)

func TestDesktopLaunchPreservesShortcutMetadataAndQuotesAddedArguments(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "应用.lnk")
	target := filepath.Join(os.Getenv("WINDIR"), "explorer.exe")
	parameters := `--profile-directory="Profile 1" --user-data-dir="C:\用户 data"`
	writeLaunchShortcut(t, path, target, parameters, root, 3)
	launch, ok := ResolveDesktopLaunch(path)
	if !ok {
		t.Fatal("valid shortcut was not resolved")
	}
	launch.Verb = "runas"
	args := []string{"--new-window", "a&b", `C:\space dir\`, ""}
	req := launch.requestWithArguments(args)
	if req.File != target || req.Directory != root || req.Show != 3 || req.Verb != "runas" {
		t.Fatalf("shortcut metadata lost: %+v", req)
	}
	got, err := windows.DecomposeCommandLine("wox " + req.Parameters)
	want := append([]string{"wox", "--profile-directory=Profile 1", `--user-data-dir=C:\用户 data`}, args...)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("argv=%q error=%v, want %q", got, err, want)
	}
	if launch.Parameters != parameters {
		t.Fatal("original shortcut arguments mutated")
	}
}

func TestDesktopLaunchRefusesOpaqueTargets(t *testing.T) {
	for _, path := range []string{"relative.exe", `shell:AppsFolder\Example!App`, `C:\missing.lnk`, `C:\example.url`} {
		if _, ok := ResolveDesktopLaunch(path); ok {
			t.Errorf("unsupported target resolved: %s", path)
		}
	}
}
