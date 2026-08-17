//go:build linux

package clipboard

import "image"

// gnomeWaylandClipboard uses the desktop portal on GNOME Wayland. If the
// portal RemoteDesktop session cannot be created, reads fall back to
// ext-data-control-v1 and writes fall back to wl-copy.
type gnomeWaylandClipboard struct{}

func newGnomeWaylandClipboard() gnomeWaylandClipboard {
	return gnomeWaylandClipboard{}
}

func (gnomeWaylandClipboard) name() string {
	return "gnome-wayland"
}

func (gnomeWaylandClipboard) readContentType() Type {
	if err := portalReady(); err == nil {
		return portalReadContentType()
	}
	return dataControlReadContentType()
}

func (gnomeWaylandClipboard) readText() (string, error) {
	if err := portalReady(); err == nil {
		return portalReadText()
	}
	return dataControlReadText()
}

func (gnomeWaylandClipboard) readFilePaths() ([]string, error) {
	if err := portalReady(); err == nil {
		return portalReadFilePaths()
	}
	return dataControlReadFilePaths()
}

func (gnomeWaylandClipboard) readImage() (image.Image, error) {
	if err := portalReady(); err == nil {
		return portalReadImage()
	}
	return dataControlReadImage()
}

func (gnomeWaylandClipboard) writeText(text string) error {
	if err := portalReady(); err == nil {
		return portalWriteText(text)
	}
	return waylandCopy(portalMimeTextUTF8, []byte(text))
}

func (gnomeWaylandClipboard) writeFilePaths(paths []string) error {
	if err := portalReady(); err == nil {
		return portalWriteFilePaths(paths)
	}
	return newWaylandClipboard().writeFilePaths(paths)
}

func (gnomeWaylandClipboard) writeImageBytes(pngData []byte) error {
	if err := portalReady(); err == nil {
		return portalWriteImageBytes(pngData)
	}
	return waylandCopy(portalMimePNG, pngData)
}

func (gnomeWaylandClipboard) isChanged() bool {
	if err := portalReady(); err == nil {
		return portalIsChanged()
	}
	return dataControlIsChanged()
}

func (gnomeWaylandClipboard) watchSnapshot() string {
	if err := portalReady(); err == nil {
		return portalWatchSnapshot()
	}
	return dataControlWatchSnapshot()
}
