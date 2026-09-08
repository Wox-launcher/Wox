package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveLinuxDesktopIconFindsUnthemedXDGDataHomeIcon(t *testing.T) {
	dataHome, _ := isolateLinuxIconEnv(t)
	iconPath := writeLinuxIconFile(t, filepath.Join(dataHome, "icons"), "appmanager-demo.png")

	assert.Equal(t, iconPath, resolveLinuxDesktopIcon("appmanager-demo"))
}

func TestResolveLinuxDesktopIconFindsUnthemedXDGDataDirsIcon(t *testing.T) {
	_, dataDirs := isolateLinuxIconEnv(t)
	iconPath := writeLinuxIconFile(t, filepath.Join(dataDirs, "icons"), "appimage-loose.png")

	assert.Equal(t, iconPath, resolveLinuxDesktopIcon("appimage-loose"))
}

func TestResolveLinuxDesktopIconLoadsAbsoluteIconPath(t *testing.T) {
	isolateLinuxIconEnv(t)
	iconPath := writeLinuxIconFile(t, t.TempDir(), "absolute-app.png")

	assert.Equal(t, iconPath, resolveLinuxDesktopIcon(iconPath))
}

func TestResolveLinuxDesktopIconPrefersThemedIconOverLooseFile(t *testing.T) {
	dataHome, _ := isolateLinuxIconEnv(t)
	writeLinuxIconFile(t, filepath.Join(dataHome, "icons"), "themed-app.png")
	themedPath := writeLinuxIconFile(t, filepath.Join(dataHome, "icons", "hicolor", "48x48", "apps"), "themed-app.png")

	assert.Equal(t, themedPath, resolveLinuxDesktopIcon("themed-app"))
}

func isolateLinuxIconEnv(t *testing.T) (string, string) {
	t.Helper()

	homeDir := t.TempDir()
	dataHome := filepath.Join(homeDir, ".local", "share")
	dataDirs := filepath.Join(homeDir, "usr", "share")
	require.NoError(t, os.MkdirAll(dataHome, 0o755))
	require.NoError(t, os.MkdirAll(dataDirs, 0o755))

	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_DATA_DIRS", dataDirs)

	return dataHome, dataDirs
}

func writeLinuxIconFile(t *testing.T, dir string, name string) string {
	t.Helper()

	require.NoError(t, os.MkdirAll(dir, 0o755))
	iconPath := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(iconPath, []byte("png"), 0o644))
	return iconPath
}
