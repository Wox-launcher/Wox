package launcher

import (
	"wox/ui/launcher/component"
	"wox/util/clipboard"
)

// ClipboardProvider is the text clipboard backend used by text fields for copy/cut/paste.
// Re-exported here so callers that already depend on the launcher package can wire it
// without reaching into the component subpackage.
type ClipboardProvider = component.ClipboardProvider

// SetClipboardProvider registers the clipboard backend used by all Wox text fields.
func SetClipboardProvider(provider ClipboardProvider) {
	component.SetClipboardProvider(provider)
}

// utilClipboardProvider adapts wox/util/clipboard to the component.ClipboardProvider interface.
// ReadText returns "" + nil for an empty or non-text clipboard so Cmd+V is a no-op.
type utilClipboardProvider struct{}

func (utilClipboardProvider) ReadText() (string, error) {
	return clipboard.ReadText()
}

func (utilClipboardProvider) WriteText(text string) error {
	return clipboard.WriteText(text)
}

// NewUtilClipboardProvider returns a ClipboardProvider backed by wox/util/clipboard.
func NewUtilClipboardProvider() ClipboardProvider {
	return utilClipboardProvider{}
}
