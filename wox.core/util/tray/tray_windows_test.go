package tray

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWriteTrayIcon covers the startup failure where CreateTemp returned a nil file.
func TestWriteTrayIcon(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("TMP", directory)
	t.Setenv("TEMP", directory)
	data := []byte("tray icon fixture")
	path, err := writeTrayIcon(data, "wox-tray-*.ico")
	if err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(stored, data) {
		t.Fatalf("icon data = %q, error = %v", stored, err)
	}
	missing := filepath.Join(directory, "missing")
	t.Setenv("TMP", missing)
	t.Setenv("TEMP", missing)
	path, err = writeTrayIcon(data, "wox-tray-*.ico")
	if err == nil || path != "" || !strings.Contains(err.Error(), "create temporary icon") {
		t.Fatalf("missing temp directory: path=%q error=%v", path, err)
	}
}
