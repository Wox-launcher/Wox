//go:build !windows

package diagnostic

import (
	"archive/zip"
	"context"
	"time"
)

// ConfigureCrashCapture prepares the portable crash report directories.
func (m *Manager) ConfigureCrashCapture(ctx context.Context) error {
	if err := m.EnsureDirectories(); err != nil {
		return err
	}
	m.retainNewestCrashFiles(m.CrashReportsDirectory(), ".zip", retainedCrashArtifacts)
	m.retainNewestCrashFiles(m.CrashIncidentsDirectory(), ".json", retainedCrashArtifacts)
	return nil
}

func (m *Manager) addWindowsCrashDumps(zipWriter *zip.Writer) {}

func (m *Manager) waitForCrashArtifacts(runStartedAt time.Time) {}
