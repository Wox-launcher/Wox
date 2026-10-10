//go:build !windows

package supervisor

import (
	"archive/zip"
	"context"
	"time"
)

// CrashCaptureAssets carries caller-owned bytes this package does not embed.
type CrashCaptureAssets struct {
	WindowsHandler []byte
}

// ConfigureCrashCapture prepares the portable crash report directories.
func (m *Manager) ConfigureCrashCapture(ctx context.Context, _ CrashCaptureAssets) error {
	if err := m.EnsureDirectories(); err != nil {
		return err
	}
	m.retainNewestCrashFiles(m.CrashReportsDirectory(), ".zip", retainedCrashArtifacts)
	m.retainNewestCrashFiles(m.CrashIncidentsDirectory(), ".json", retainedCrashArtifacts)
	return nil
}

func (m *Manager) addWindowsCrashDumps(zipWriter *zip.Writer) {}

func (m *Manager) waitForCrashArtifacts(pid int, runStartedAt time.Time) string {
	return ""
}
