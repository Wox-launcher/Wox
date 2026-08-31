//go:build linux

package clipboard

import (
	"os"
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
