package util

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

// TestUnzipWindowsSeparators reproduces plugin packages built with Windows paths.
func TestUnzipWindowsSeparators(t *testing.T) {
	entries := map[string]string{
		"plugin.json":                           "{}",
		`images\`:                               "",
		`images\app.png`:                        "icon",
		`dependencies\wox_plugin\__init__.py`:   "sdk",
		`dependencies/wox_plugin\models/api.py`: "model",
		"normal/file.txt":                       "normal",
	}
	source := writeTestZip(t, entries)
	destination := filepath.Join(t.TempDir(), "plugin")
	if err := Unzip(source, destination); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"plugin.json":                           "{}",
		"images/app.png":                        "icon",
		"dependencies/wox_plugin/__init__.py":   "sdk",
		"dependencies/wox_plugin/models/api.py": "model",
		"normal/file.txt":                       "normal",
	} {
		got, err := os.ReadFile(filepath.Join(destination, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Fatalf("%s: got %q, want %q", name, got, want)
		}
	}
}

// TestUnzipRejectsUnsafePaths ensures separator compatibility cannot escape the destination.
func TestUnzipRejectsUnsafePaths(t *testing.T) {
	for _, name := range []string{
		"../outside.txt", `..\outside.txt`, `images\..\..\outside.txt`,
		"/outside.txt", `\outside.txt`, `C:\outside.txt`, `\\server\share\outside.txt`,
	} {
		t.Run(name, func(t *testing.T) {
			source := writeTestZip(t, map[string]string{"valid.txt": "valid", name: "outside"})
			destination := filepath.Join(t.TempDir(), "plugin")
			if err := Unzip(source, destination); err == nil {
				t.Fatal("expected an invalid path error")
			}
			if _, err := os.Stat(destination); !os.IsNotExist(err) {
				t.Fatalf("extraction wrote files before validating paths: %v", err)
			}
		})
	}
}

// writeTestZip creates an archive without normalizing its entry names.
func writeTestZip(t *testing.T, entries map[string]string) string {
	t.Helper()
	source := filepath.Join(t.TempDir(), "plugin.wox")
	file, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	writer := zip.NewWriter(file)
	for name, content := range entries {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		if name[len(name)-1] == '\\' {
			header.SetMode(os.ModeDir | 0755)
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return source
}
