//go:build windows

package woxui

import "testing"

func TestNativeFilePreviewCommandsIgnoreOlderGenerations(t *testing.T) {
	window := &platformWindow{nativeFilePreviewGeneration: 2}

	window.executeCommand(windowCommand{
		kind:                        windowCommandShowNativeFilePreview,
		nativeFilePath:              "stale.docx",
		nativeFilePreviewGeneration: 1,
	})
	if window.nativeFilePreviewGeneration != 2 || window.nativeFilePreview != nil {
		t.Fatalf("stale show changed native preview state: generation %d preview %#v", window.nativeFilePreviewGeneration, window.nativeFilePreview)
	}

	window.executeCommand(windowCommand{
		kind:                        windowCommandHideNativeFilePreview,
		nativeFilePreviewGeneration: 3,
	})
	window.executeCommand(windowCommand{
		kind:                        windowCommandShowNativeFilePreview,
		nativeFilePath:              "stale-again.docx",
		nativeFilePreviewGeneration: 2,
	})
	if window.nativeFilePreviewGeneration != 3 || window.nativeFilePreview != nil {
		t.Fatalf("older show resurrected after hide: generation %d preview %#v", window.nativeFilePreviewGeneration, window.nativeFilePreview)
	}
}
