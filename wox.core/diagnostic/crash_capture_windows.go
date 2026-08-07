//go:build windows

package diagnostic

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"
)

// ConfigureCrashCapture asks Windows Error Reporting to retain local minidumps for Wox.
func (m *Manager) ConfigureCrashCapture(ctx context.Context) error {
	if err := m.EnsureDirectories(); err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	keyPath := `Software\Microsoft\Windows\Windows Error Reporting\LocalDumps\` + filepath.Base(executable)
	key, _, err := registry.CreateKey(registry.CURRENT_USER, keyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if err := key.SetExpandStringValue("DumpFolder", m.CrashDumpsDirectory()); err != nil {
		return err
	}
	if err := key.SetDWordValue("DumpType", 1); err != nil {
		return err
	}
	if err := key.SetDWordValue("DumpCount", retainedCrashArtifacts); err != nil {
		return err
	}
	m.retainNewestCrashFiles(m.CrashDumpsDirectory(), ".dmp", retainedCrashArtifacts)
	m.retainNewestCrashFiles(m.CrashReportsDirectory(), ".zip", retainedCrashArtifacts)
	m.retainNewestCrashFiles(m.CrashIncidentsDirectory(), ".json", retainedCrashArtifacts)
	return nil
}

func (m *Manager) addWindowsCrashDumps(zipWriter *zip.Writer) {
	// WER names dumps itself, so use the run start boundary to select only dumps
	// that can belong to the current supervised process.
	entries, err := os.ReadDir(m.CrashDumpsDirectory())
	if err != nil {
		return
	}
	state := m.LoadState()
	startedAt := time.UnixMilli(state.StartedAt).Add(-2 * time.Minute)
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".dmp") {
			continue
		}
		info, statErr := entry.Info()
		if statErr != nil || info.ModTime().Before(startedAt) {
			continue
		}
		addExistingFile(zipWriter, filepath.Join(m.CrashDumpsDirectory(), entry.Name()), filepath.Join("windows-dumps", entry.Name()))
	}
}

func (m *Manager) waitForCrashArtifacts(runStartedAt time.Time) {
	// WER can finish writing just after the process handle signals. Give it a
	// short bounded window so the first generated report contains the minidump.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(m.CrashDumpsDirectory())
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".dmp") {
					continue
				}
				info, statErr := entry.Info()
				if statErr == nil && !info.ModTime().Before(runStartedAt.Add(-2*time.Second)) {
					return
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}
