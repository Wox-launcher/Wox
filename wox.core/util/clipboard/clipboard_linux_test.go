//go:build linux

package clipboard

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataControlSelectionContentType(t *testing.T) {
	tests := []struct {
		name     string
		mimeType string
		expected Type
	}{
		{name: "URI list", mimeType: "text/uri-list", expected: ClipboardTypeFile},
		{name: "PNG", mimeType: "image/png", expected: ClipboardTypeImage},
		{name: "UTF-8 text", mimeType: "text/plain;charset=utf-8", expected: ClipboardTypeText},
		{name: "plain text", mimeType: "text/plain", expected: ClipboardTypeText},
		{name: "legacy UTF-8 text", mimeType: "UTF8_STRING", expected: ClipboardTypeText},
		{name: "unsupported", mimeType: "text/html", expected: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, dataControlSelectionContentType(dataControlSelection{mimeType: test.mimeType}))
		})
	}
}

func TestSelectLinuxClipboard(t *testing.T) {
	tests := []struct {
		name     string
		wayland  bool
		kde      bool
		gnome    bool
		hyprland bool
		expected string
	}{
		{name: "X11 Openbox", wayland: false, expected: "x11"},
		{name: "GNOME X11", wayland: false, gnome: true, expected: "x11"},
		{name: "KDE X11", wayland: false, kde: true, expected: "x11"},
		{name: "GNOME Wayland", wayland: true, gnome: true, expected: "gnome-wayland"},
		{name: "KDE Wayland", wayland: true, kde: true, expected: "kde-wayland"},
		{name: "Hyprland", wayland: true, hyprland: true, expected: "hyprland"},
		{name: "Sway", wayland: true, expected: "wayland"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, selectLinuxClipboardFor(test.wayland, test.kde, test.gnome, test.hyprland).name())
		})
	}
}

func TestX11TargetsContentType(t *testing.T) {
	assert.Equal(t, ClipboardTypeFile, x11TargetsContentType([]string{"TIMESTAMP", "text/uri-list"}))
	assert.Equal(t, ClipboardTypeImage, x11TargetsContentType([]string{"image/png", "TARGETS"}))
	assert.Equal(t, ClipboardTypeText, x11TargetsContentType([]string{"UTF8_STRING", "STRING"}))
	assert.Equal(t, ClipboardTypeText, x11TargetsContentType([]string{"text/plain;charset=utf-8"}))
	assert.Equal(t, Type(""), x11TargetsContentType([]string{"TIMESTAMP", "TARGETS"}))
}

func TestX11ClipboardWriteDoesNotWaitForSelectionOwner(t *testing.T) {
	started := time.Now()
	require.NoError(t, runX11ClipboardCommandErr("sh", []string{"-c", "(sleep 2) &"}, []byte("clipboard")))
	require.Less(t, time.Since(started), time.Second)
}

// TestWaylandClipboardWriteDoesNotWaitForSelectionOwner models wl-copy's inherited stderr.
func TestWaylandClipboardWriteDoesNotWaitForSelectionOwner(t *testing.T) {
	output := filepath.Join(t.TempDir(), "clipboard")
	payload := []byte("hello 世界\x00")
	started := time.Now()
	require.NoError(t, runWaylandClipboardCommandErr("sh", []string{"-c", "cat > \"$1\"; (sleep 2 > /dev/null) &", "sh", output}, payload))
	require.Less(t, time.Since(started), time.Second)
	actual, err := os.ReadFile(output)
	require.NoError(t, err)
	require.Equal(t, payload, actual)
}

func TestWaylandClipboardWriteReportsCommandFailure(t *testing.T) {
	err := runWaylandClipboardCommandErr("sh", []string{"-c", "cat > /dev/null; exit 42"}, []byte("clipboard"))
	require.ErrorContains(t, err, "exit status 42")
}

// TestWaylandPasteTextPreservesNewlines exercises the wl-paste fallback without a desktop session.
func TestWaylandPasteTextPreservesNewlines(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\nif [ \"$1\" != \"--no-newline\" ]; then exit 1; fi\nprintf '\\r\\nthree\\r\\nfour\\rfive\\nsix\\r\\n'\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "wl-paste"), []byte(script), 0755))
	t.Setenv("PATH", dir)

	text, err := waylandPasteText()
	require.NoError(t, err)
	require.Equal(t, "\r\nthree\r\nfour\rfive\nsix\r\n", text)
}

type testLinuxClipboardWriter struct {
	linuxClipboard
}

func (testLinuxClipboardWriter) writeText(string) error { return nil }

func TestLinuxClipboardWriteInvalidatesCachedSelection(t *testing.T) {
	previousBackend := linuxClipboardBackend()
	linuxBackend = testLinuxClipboardWriter{linuxClipboard: previousBackend}
	t.Cleanup(func() { linuxBackend = previousBackend })

	dataControlMu.Lock()
	previousCache := linuxDataControl
	linuxDataControl.capturedAt = time.Now()
	linuxDataControl.latest = dataControlSelection{mimeType: portalMimeTextUTF8, data: []byte("old")}
	dataControlMu.Unlock()
	t.Cleanup(func() {
		dataControlMu.Lock()
		linuxDataControl = previousCache
		dataControlMu.Unlock()
	})

	require.NoError(t, writeTextData("new"))
	dataControlMu.Lock()
	cachedAt := linuxDataControl.capturedAt
	dataControlMu.Unlock()
	require.True(t, cachedAt.IsZero(), "a write must invalidate the old selection")
}

func TestWaylandClipboardWriteTimesOut(t *testing.T) {
	err := runWaylandClipboardCommandErr("sh", []string{"-c", "exec sleep 5"}, nil)
	require.ErrorContains(t, err, "Wayland clipboard command timed out")
}

func TestLinuxDataControlLiveRead(t *testing.T) {
	if os.Getenv("WOX_LIVE_DATA_CONTROL") != "1" {
		t.Skip("set WOX_LIVE_DATA_CONTROL=1 to read the current Wayland clipboard")
	}

	selection, err := readDataControlSelection()
	require.NoError(t, err)
	require.NotEmpty(t, selection.mimeType)
	require.NotEmpty(t, selection.data)
	t.Logf("mime=%s bytes=%d", selection.mimeType, len(selection.data))
}
