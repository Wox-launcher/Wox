package util

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func useExecutablePath(t *testing.T, executable string) {
	t.Helper()
	previous := currentExecutablePath
	currentExecutablePath = func() (string, error) {
		return executable, nil
	}
	t.Cleanup(func() {
		currentExecutablePath = previous
	})
}

func TestPortableUserDataDirectoryRequiresExistingDirectory(t *testing.T) {
	executableDir := t.TempDir()
	_, ok := portableUserDataDirectoryIn(executableDir)
	assert.False(t, ok)

	require.NoError(t, os.WriteFile(filepath.Join(executableDir, portableUserDataDirectoryName), []byte("x"), 0o644))
	_, ok = portableUserDataDirectoryIn(executableDir)
	assert.False(t, ok)

	require.NoError(t, os.Remove(filepath.Join(executableDir, portableUserDataDirectoryName)))
	require.NoError(t, os.Mkdir(filepath.Join(executableDir, portableUserDataDirectoryName), 0o755))
	got, ok := portableUserDataDirectoryIn(executableDir)
	assert.True(t, ok)
	assert.Equal(t, filepath.Join(executableDir, portableUserDataDirectoryName), got)

	_, ok = portableUserDataDirectoryIn("  ")
	assert.False(t, ok)
}

func TestInitUsesExistingWoxUserBesideExecutable(t *testing.T) {
	dataDir := t.TempDir()
	executableDir := t.TempDir()
	portableDir := filepath.Join(executableDir, portableUserDataDirectoryName)
	require.NoError(t, os.Mkdir(portableDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, ".userdata.location"), []byte("/keep/this/path"), 0o644))
	t.Setenv(TestWoxDataDirEnv, dataDir)
	t.Setenv(TestUserDataDirEnv, "")
	useExecutablePath(t, filepath.Join(executableDir, "Wox.exe"))

	require.NoError(t, GetLocation().Init())

	assert.True(t, GetLocation().IsPortableUserData())
	assert.Equal(t, portableDir, GetLocation().GetUserDataDirectory())
	assert.Equal(t, dataDir, GetLocation().GetWoxDataDirectory())
	shortcut, err := os.ReadFile(filepath.Join(dataDir, ".userdata.location"))
	require.NoError(t, err)
	assert.Equal(t, "/keep/this/path", string(shortcut))

	info, statErr := os.Stat(filepath.Join(portableDir, "plugins"))
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
	_, statErr = os.Stat(filepath.Join(dataDir, "cache"))
	require.NoError(t, statErr)
	_, statErr = os.Stat(filepath.Join(executableDir, "cache"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestInitWithoutPortableDirectoryKeepsUserDataShortcut(t *testing.T) {
	dataDir := t.TempDir()
	executableDir := t.TempDir()
	t.Setenv(TestWoxDataDirEnv, dataDir)
	t.Setenv(TestUserDataDirEnv, "")
	useExecutablePath(t, filepath.Join(executableDir, "Wox.exe"))

	require.NoError(t, GetLocation().Init())

	assert.False(t, GetLocation().IsPortableUserData())
	assert.Equal(t, path.Join(dataDir, "wox-user"), GetLocation().GetUserDataDirectory())
	shortcut, err := os.ReadFile(filepath.Join(dataDir, ".userdata.location"))
	require.NoError(t, err)
	assert.Equal(t, path.Join(dataDir, "wox-user"), string(shortcut))
	_, statErr := os.Stat(filepath.Join(executableDir, portableUserDataDirectoryName))
	assert.True(t, os.IsNotExist(statErr))
}

func TestInitUserDataOverrideIgnoresPortableDirectory(t *testing.T) {
	dataDir := t.TempDir()
	executableDir := t.TempDir()
	userDir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(executableDir, portableUserDataDirectoryName), 0o755))
	t.Setenv(TestWoxDataDirEnv, dataDir)
	t.Setenv(TestUserDataDirEnv, userDir)
	useExecutablePath(t, filepath.Join(executableDir, "Wox.exe"))

	require.NoError(t, GetLocation().Init())

	assert.False(t, GetLocation().IsPortableUserData())
	assert.Equal(t, userDir, GetLocation().GetUserDataDirectory())
}

func TestPortableUserDataFollowsExecutableSymlink(t *testing.T) {
	realDir := t.TempDir()
	linkDir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(realDir, portableUserDataDirectoryName), 0o755))
	realExe := filepath.Join(realDir, "Wox.exe")
	require.NoError(t, os.WriteFile(realExe, []byte("x"), 0o644))
	linkExe := filepath.Join(linkDir, "Wox.exe")
	if err := os.Symlink(realExe, linkExe); err != nil {
		t.Skip("symlink not permitted")
	}
	useExecutablePath(t, linkExe)

	got, ok := portableUserDataDirectory()
	assert.True(t, ok)
	assert.Equal(t, filepath.Join(realDir, portableUserDataDirectoryName), got)
}
