//go:build !windows

package screenshot

import "wox/util/screen"

// restoreScreenshotDocumentDesktop preserves the logical capture area on unified desktop platforms.
func restoreScreenshotDocumentDesktop(platform *screenshotEditorPlatform, document *screenshotDocument, displays []screen.Display) error {
	bounds, available := screenshotDocumentDesktopBounds(document, displays, false)
	if !available {
		return ErrScreenshotDisplayLayoutChanged
	}
	platform.setWindowBounds = func(window *Window) error { return window.SetBounds(bounds) }
	platform.logicalSelection = func(selection Rect, _ Size) Rect {
		selection.X += bounds.X
		selection.Y += bounds.Y
		return selection
	}
	platform.chromeScale = nil
	return nil
}
