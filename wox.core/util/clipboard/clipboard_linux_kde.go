//go:build linux

package clipboard

import (
	"fmt"
	"image"
	"os"
	"wox/util"
)

// kdeWaylandClipboard uses the desktop portal on KDE/Plasma Wayland, except
// for images: KWin does not reliably request image payloads from a background
// portal session, so image writes go through the focused UI GTK path.
type kdeWaylandClipboard struct{}

func newKDEWaylandClipboard() kdeWaylandClipboard {
	return kdeWaylandClipboard{}
}

func (kdeWaylandClipboard) name() string {
	return "kde-wayland"
}

func (kdeWaylandClipboard) readContentType() Type {
	if err := portalReady(); err == nil {
		return portalReadContentType()
	}
	return dataControlReadContentType()
}

func (kdeWaylandClipboard) readText() (string, error) {
	if err := portalReady(); err == nil {
		return portalReadText()
	}
	return dataControlReadText()
}

func (kdeWaylandClipboard) readFilePaths() ([]string, error) {
	if err := portalReady(); err == nil {
		return portalReadFilePaths()
	}
	return dataControlReadFilePaths()
}

func (kdeWaylandClipboard) readImage() (image.Image, error) {
	if err := portalReady(); err == nil {
		return portalReadImage()
	}
	return dataControlReadImage()
}

func (kdeWaylandClipboard) writeText(text string) error {
	if err := portalReady(); err == nil {
		return portalWriteText(text)
	}
	return waylandCopy(portalMimeTextPlain, []byte(text))
}

func (kdeWaylandClipboard) writeFilePaths(paths []string) error {
	if err := portalReady(); err == nil {
		return portalWriteFilePaths(paths)
	}
	return newWaylandClipboard().writeFilePaths(paths)
}

func (kdeWaylandClipboard) writeImageBytes(pngData []byte) error {
	if err := writeKDEWaylandImageBytesViaUI(pngData); err != nil {
		return err
	}
	util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf("clipboard: KDE Wayland image write via UI, pngBytes=%d", len(pngData)))
	return nil
}

func (kdeWaylandClipboard) isChanged() bool {
	if err := portalReady(); err == nil {
		return portalIsChanged()
	}
	return dataControlIsChanged()
}

func (kdeWaylandClipboard) watchSnapshot() string {
	if err := portalReady(); err == nil {
		return portalWatchSnapshot()
	}
	return dataControlWatchSnapshot()
}

// writeKDEWaylandImageBytesViaUI delegates image clipboard ownership to UI.
func writeKDEWaylandImageBytesViaUI(pngData []byte) error {
	cacheDir := util.GetLocation().GetCacheDirectory()
	if cacheDir == "" {
		cacheDir = os.TempDir()
	}
	if mkdirErr := os.MkdirAll(cacheDir, 0700); mkdirErr != nil {
		return fmt.Errorf("clipboard: failed to create temporary image directory for KDE Wayland UI write: %w", mkdirErr)
	}

	tempFile, err := os.CreateTemp(cacheDir, "wox-clipboard-*.png")
	if err != nil {
		return fmt.Errorf("clipboard: failed to create temporary image file for KDE Wayland UI write: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if _, err = tempFile.Write(pngData); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("clipboard: failed to write temporary image file for KDE Wayland UI write: %w", err)
	}
	if err = tempFile.Close(); err != nil {
		return fmt.Errorf("clipboard: failed to close temporary image file for KDE Wayland UI write: %w", err)
	}

	if err = writeNativeImageFile(util.NewTraceContext(), tempPath); err != nil {
		return fmt.Errorf("clipboard: KDE Wayland UI image write failed: %w", err)
	}
	return nil
}
