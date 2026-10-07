//go:build darwin

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMoveMacAppToTrashRemovesBundle(t *testing.T) {
	appPath := filepath.Join(t.TempDir(), "Wox Uninstall Test.app")
	require.NoError(t, os.Mkdir(appPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(appPath, "Info.plist"), []byte("test"), 0o644))

	require.NoError(t, moveMacAppToTrash(appPath))
	_, err := os.Lstat(appPath)
	require.ErrorIs(t, err, os.ErrNotExist)
	cleanupTrashedTestApp(t, "Wox Uninstall Test")
}

func TestMoveMacAppToTrashRemovesSymlinkOnly(t *testing.T) {
	root := t.TempDir()
	realApp := filepath.Join(root, "real", "Foo.app")
	require.NoError(t, os.MkdirAll(realApp, 0o755))
	link := filepath.Join(root, "Wox Uninstall Link.app")
	require.NoError(t, os.Symlink(realApp, link))

	require.NoError(t, moveMacAppToTrash(link))
	_, err := os.Lstat(link)
	require.ErrorIs(t, err, os.ErrNotExist)
	_, err = os.Lstat(realApp)
	require.NoError(t, err)
	cleanupTrashedTestApp(t, "Wox Uninstall Link")
}

func TestMoveMacAppToTrashMissingBundle(t *testing.T) {
	err := moveMacAppToTrash(filepath.Join(t.TempDir(), "Missing.app"))
	require.ErrorIs(t, err, errMacUninstallNotFound)
}

func cleanupTrashedTestApp(t *testing.T, name string) {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	trashDir := filepath.Join(home, ".Trash")
	entries, err := os.ReadDir(trashDir)
	if err != nil {
		t.Logf("could not clean %s from Trash: %v", name, err)
		return
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), name) {
			if removeErr := os.RemoveAll(filepath.Join(trashDir, entry.Name())); removeErr != nil {
				t.Logf("could not remove trashed test app %s: %v", entry.Name(), removeErr)
			}
		}
	}
}
