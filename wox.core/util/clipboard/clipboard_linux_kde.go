//go:build linux

package clipboard

import (
	"fmt"
	"os"
	"wox/util"
)

// kdeWaylandClipboard uses data-control so clipboard history does not start a
// RemoteDesktop portal session and prompt for remote-control access on startup.
// Image writes retain the focused UI GTK path for KWin compatibility.
type kdeWaylandClipboard struct{}

func newKDEWaylandClipboard() kdeWaylandClipboard {
	return kdeWaylandClipboard{}
}

func (kdeWaylandClipboard) name() string {
	return "kde-wayland"
}

func (kdeWaylandClipboard) readContentType() Type {
	return dataControlReadContentType()
}

func (kdeWaylandClipboard) readText() (string, error) {
	return dataControlReadText()
}

func (kdeWaylandClipboard) readFilePaths() ([]string, error) {
	return dataControlReadFilePaths()
}

func (kdeWaylandClipboard) readImageSnapshot() (*ImageSnapshot, error) {
	return dataControlReadImageSnapshot()
}

func (kdeWaylandClipboard) writeText(text string) error {
	return writeDataControlSelection(portalMimeTextUTF8, []byte(text))
}

func (kdeWaylandClipboard) writeFilePaths(paths []string) error {
	payload, err := buildPortalURIListPayload(paths)
	if err != nil {
		return err
	}
	return writeDataControlSelection(portalMimeURIList, payload)
}

func (kdeWaylandClipboard) writeImageBytes(pngData []byte) error {
	if err := writeKDEWaylandImageBytesViaUI(pngData); err != nil {
		return err
	}
	util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf("clipboard: KDE Wayland image write via UI, pngBytes=%d", len(pngData)))
	return nil
}

func (kdeWaylandClipboard) isChanged() bool {
	return dataControlIsChanged()
}

func (kdeWaylandClipboard) watchSnapshot() string {
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
