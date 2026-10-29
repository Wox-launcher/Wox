package tray

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestNativeTrayMenu runs the actual C implementation, including GDI resource checks.
func TestNativeTrayMenu(t *testing.T) {
	compiler, err := exec.Command("go", "env", "CC").Output()
	if err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(t.TempDir(), "tray-menu-test.exe")
	build := exec.Command(strings.TrimSpace(string(compiler)), "-std=c11", "-Wall", "-Wextra", "-Wno-cast-function-type", "testdata/menu_windows_test.c", "-o", executable, "-lgdi32", "-luser32")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build native menu test: %v\n%s", err, output)
	}
	if output, err := exec.Command(executable).CombinedOutput(); err != nil {
		t.Fatalf("native menu test: %v\n%s", err, output)
	}
}

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
