//go:build linux

package clipboard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestKDEClipboardDoesNotRequestRemoteDesktopAccess keeps even failure paths from opening a portal session.
func TestKDEClipboardDoesNotRequestRemoteDesktopAccess(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", filepath.Join(t.TempDir(), "missing-wayland"))
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(t.TempDir(), "missing-bus"))
	linuxPortalMu.Lock()
	previousPortal := linuxClipboardPortal
	linuxClipboardPortal = linuxPortalClipboard{}
	linuxPortalMu.Unlock()
	t.Cleanup(func() {
		linuxPortalMu.Lock()
		linuxClipboardPortal = previousPortal
		linuxPortalMu.Unlock()
	})

	backend := newKDEWaylandClipboard()
	require.Empty(t, backend.readContentType())
	_, err := backend.readText()
	require.Error(t, err)
	_, err = backend.readFilePaths()
	require.Error(t, err)
	_, err = backend.readImageSnapshot()
	require.Error(t, err)
	require.Error(t, backend.writeText("clipboard"))
	require.Error(t, backend.writeFilePaths([]string{"/tmp/clipboard"}))
	require.False(t, backend.isChanged())
	require.Contains(t, backend.watchSnapshot(), "backend=ext-data-control-v1")

	linuxPortalMu.Lock()
	defer linuxPortalMu.Unlock()
	require.False(t, linuxClipboardPortal.initialized)
	require.False(t, linuxClipboardPortal.unavailable, "KDE must not attempt RemoteDesktop even when data-control is unavailable")
}

// TestKDEClipboardLiveRoundTrip verifies background publication, format decoding, and source replacement on real KWin.
func TestKDEClipboardLiveRoundTrip(t *testing.T) {
	if os.Getenv("WOX_LIVE_KDE_CLIPBOARD") != "1" {
		t.Skip("set WOX_LIVE_KDE_CLIPBOARD=1 under KDE Wayland to replace and restore the clipboard")
	}
	original, err := readDataControlSelection()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, writeDataControlSelection(original.mimeType, original.data))
		// Give Klipper time to persist the restored offer before the test process exits.
		time.Sleep(200 * time.Millisecond)
	})
	t.Setenv("PATH", t.TempDir())
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(t.TempDir(), "missing-bus"))
	backend := newKDEWaylandClipboard()
	for _, text := range []string{"\r\nWox 剪贴板\n", "replacement", strings.Repeat("Wox 世界\n", 32768)} {
		require.NoError(t, backend.writeText(text))
		actual, readErr := backend.readText()
		require.NoError(t, readErr)
		require.Equal(t, text, actual)
		require.Equal(t, ClipboardTypeText, backend.readContentType())
	}
	paths := []string{filepath.Join(t.TempDir(), "a b.txt"), filepath.Join(t.TempDir(), "测试.txt")}
	require.NoError(t, backend.writeFilePaths(paths))
	actualPaths, err := backend.readFilePaths()
	require.NoError(t, err)
	require.Equal(t, paths, actualPaths)
	require.Equal(t, ClipboardTypeFile, backend.readContentType())

	payload := []byte("immutable offer")
	require.NoError(t, writeDataControlSelection(portalMimeTextUTF8, payload))
	clear(payload)
	for range 3 {
		actual, readErr := backend.readText()
		require.NoError(t, readErr)
		require.Equal(t, "immutable offer", actual)
	}
	require.True(t, backend.isChanged())
	require.False(t, backend.isChanged())
}
