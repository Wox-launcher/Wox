package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectHomebrewAppImage(t *testing.T) {
	root := t.TempDir()
	prefix := filepath.Join(root, "Brew prefix")
	stage := filepath.Join(prefix, "Caskroom", "wox", "2.5.0")
	dataHome := filepath.Join(root, "shared data")
	desktopEntry := filepath.Join(dataHome, "applications", linuxDesktopFile)
	image := filepath.Join(root, "Wox.AppImage")
	portable := filepath.Join(root, "Portable.AppImage")
	for _, dir := range []string{stage, filepath.Dir(desktopEntry)} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	for path, contents := range map[string]string{
		image:        "AppImage",
		portable:     "AppImage",
		desktopEntry: "[Desktop Entry]\nIcon=" + filepath.Join(stage, linuxDesktopIconFile) + "\n",
	} {
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for link, target := range map[string]string{homebrewWoxAppImage: image, linuxDesktopFile: desktopEntry} {
		if err := os.Symlink(target, filepath.Join(stage, link)); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", "")
	t.Setenv("GIO_LAUNCHED_DESKTOP_FILE", "")
	t.Setenv("HOMEBREW_XDG_DATA_HOME", "")

	// Older Brew versions did not install a desktop entry and launched
	// the AppImage from a symlink pointing into the Caskroom.
	legacyStage := filepath.Join(prefix, "Caskroom", "wox", "2.4.9")
	legacyImage := filepath.Join(legacyStage, homebrewWoxAppImage)
	if err := os.MkdirAll(legacyStage, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyImage, []byte("AppImage"), 0644); err != nil {
		t.Fatal(err)
	}
	legacyLink := filepath.Join(root, "Legacy.AppImage")
	if err := os.Symlink(legacyImage, legacyLink); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		image    string
		prefix   string
		dataHome string
		want     homebrewAppImageInstall
	}{
		{
			name: "custom prefix from desktop entry", image: image, dataHome: dataHome,
			want: homebrewAppImageInstall{managed: true, desktopEntry: desktopEntry},
		},
		{
			name: "direct launch via known prefix", image: image, prefix: prefix,
			want: homebrewAppImageInstall{managed: true, desktopEntry: desktopEntry},
		},
		{
			name: "separate portable copy", image: portable, dataHome: dataHome, prefix: prefix,
		},
		{
			name: "legacy symlinked install", image: legacyLink, prefix: prefix,
			want: homebrewAppImageInstall{managed: true},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("APPIMAGE", test.image)
			t.Setenv("XDG_DATA_HOME", test.dataHome)
			t.Setenv("HOMEBREW_PREFIX", test.prefix)
			if got := detectHomebrewAppImage(); got != test.want {
				t.Fatalf("detected install %+v, want %+v", got, test.want)
			}
		})
	}
}
