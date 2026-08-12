//go:build linux

package clipboard

import (
	"os"
	"testing"

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
