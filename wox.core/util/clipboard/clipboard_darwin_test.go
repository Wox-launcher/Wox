//go:build darwin

package clipboard

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestReadDistinguishesRemoteAndFileURLs covers Safari's URL-plus-text pasteboard payload.
func TestReadDistinguishesRemoteAndFileURLs(t *testing.T) {
	const remoteURL = "https://github.com/Wox-launcher/Wox/issues/4532"
	setRemoteURL := exec.Command("osascript", "-l", "JavaScript", "-e", `
ObjC.import("AppKit");
const pasteboard = $.NSPasteboard.generalPasteboard;
pasteboard.clearContents;
pasteboard.setStringForType("https://github.com/Wox-launcher/Wox/issues/4532", "public.url");
pasteboard.setStringForType("https://github.com/Wox-launcher/Wox/issues/4532", $.NSPasteboardTypeString);
`)
	output, err := setRemoteURL.CombinedOutput()
	require.NoError(t, err, string(output))

	data, err := Read()
	require.NoError(t, err)
	require.Equal(t, ClipboardTypeText, data.GetType())
	require.Equal(t, remoteURL, data.String())

	filePath := filepath.Join(t.TempDir(), "clipboard-file.txt")
	require.NoError(t, Write(&FilePathData{FilePaths: []string{filePath}}))

	data, err = Read()
	require.NoError(t, err)
	require.Equal(t, ClipboardTypeFile, data.GetType())
	require.Equal(t, filePath, data.(*FilePathData).FilePaths[0])
}
