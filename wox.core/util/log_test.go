package util

import (
	"os"
	"path/filepath"
	"testing"
)

// TestClearHistoryPreservesActiveLogWriter covers Windows sharing violations
// and ensures Unix writers keep appending to the file collected by diagnostics.
func TestClearHistoryPreservesActiveLogWriter(t *testing.T) {
	directory := t.TempDir()
	logger := &Log{
		logFolder: directory,
		fileWriter: &Lumberjack{
			Filename: filepath.Join(directory, logFileBaseName), Daily: true, LocalTime: true,
		},
	}
	t.Cleanup(func() { _ = logger.fileWriter.Close() })
	if _, err := logger.fileWriter.Write([]byte("previous core log\n")); err != nil {
		t.Fatal(err)
	}
	activePath := filepath.Join(directory, "supervisor.log")
	active, err := os.OpenFile(activePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer active.Close()
	if _, err := active.WriteString("before cleanup\n"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"archived.log", "crash.log"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("old diagnostics"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := logger.ClearHistory(filepath.Base(activePath)); err != nil {
		t.Fatalf("clear history with an open supervisor log: %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, "archived.log")); !os.IsNotExist(err) {
		t.Fatalf("historical log still exists: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(directory, "crash.log")); err != nil || len(data) != 0 {
		t.Fatalf("crash log was not cleared: data=%q error=%v", data, err)
	}
	if _, err := active.WriteString("after cleanup\n"); err != nil {
		t.Fatalf("supervisor writer stopped after cleanup: %v", err)
	}
	if data, err := os.ReadFile(activePath); err != nil || string(data) != "before cleanup\nafter cleanup\n" {
		t.Fatalf("active log lost output: data=%q error=%v", data, err)
	}
	if _, err := logger.fileWriter.Write([]byte("after cleanup\n")); err != nil {
		t.Fatalf("core writer stopped after cleanup: %v", err)
	}
}
