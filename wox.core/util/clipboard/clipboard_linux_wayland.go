//go:build linux

package clipboard

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os/exec"
	"time"
)

const waylandClipboardCommandTimeout = 3 * time.Second

// waylandClipboard is the generic Wayland backend for compositors that are not
// GNOME or KDE, including Hyprland and Sway. Reads prefer ext-data-control-v1;
// writes use wl-copy when the portal RemoteDesktop session is not available.
type waylandClipboard struct{}

func newWaylandClipboard() waylandClipboard {
	return waylandClipboard{}
}

func (waylandClipboard) name() string {
	return "wayland"
}

func (waylandClipboard) readContentType() Type {
	if contentType := dataControlReadContentType(); contentType != "" {
		return contentType
	}
	return waylandPasteContentType()
}

func (waylandClipboard) readText() (string, error) {
	text, err := dataControlReadText()
	if err == nil {
		return text, nil
	}
	return waylandPasteText()
}

func (waylandClipboard) readFilePaths() ([]string, error) {
	paths, err := dataControlReadFilePaths()
	if err == nil {
		return paths, nil
	}
	return waylandPasteFilePaths()
}

func (waylandClipboard) readImage() (image.Image, error) {
	img, err := dataControlReadImage()
	if err == nil {
		return img, nil
	}
	return waylandPasteImage()
}

func (waylandClipboard) writeText(text string) error {
	return waylandCopy(portalMimeTextPlain, []byte(text))
}

func (waylandClipboard) writeFilePaths(paths []string) error {
	payload, err := buildPortalURIListPayload(paths)
	if err != nil {
		return err
	}
	return waylandCopy(portalMimeURIList, payload)
}

func (waylandClipboard) writeImageBytes(pngData []byte) error {
	return waylandCopy(portalMimePNG, pngData)
}

func (waylandClipboard) isChanged() bool {
	if dataControlIsChanged() {
		return true
	}
	return false
}

func (waylandClipboard) watchSnapshot() string {
	return dataControlWatchSnapshot()
}

func waylandCopy(mimeType string, payload []byte) error {
	bin, err := exec.LookPath("wl-copy")
	if err != nil {
		return errors.New("clipboard: wl-copy is not available")
	}
	return runWaylandClipboardCommandErr(bin, []string{"--type", mimeType}, payload)
}

func waylandPasteText() (string, error) {
	data, err := waylandPaste(portalMimeTextUTF8, portalMimeTextPlain)
	if err != nil {
		return "", err
	}
	text := string(bytes.TrimRight(data, "\x00"))
	if text == "" {
		return "", noDataErr
	}
	return text, nil
}

func waylandPasteFilePaths() ([]string, error) {
	data, err := waylandPaste(portalMimeURIList)
	if err != nil {
		return nil, err
	}
	paths := parsePortalURIList(string(data))
	if len(paths) == 0 {
		return nil, noDataErr
	}
	return paths, nil
}

func waylandPasteImage() (image.Image, error) {
	data, err := waylandPaste(portalMimePNG)
	if err != nil {
		return nil, err
	}
	img, decodeErr := png.Decode(bytes.NewReader(data))
	if decodeErr != nil {
		return nil, fmt.Errorf("clipboard: failed to decode wl-paste PNG: %w", decodeErr)
	}
	return img, nil
}

func waylandPasteContentType() Type {
	bin, err := exec.LookPath("wl-paste")
	if err != nil {
		return ""
	}
	output, pasteErr := runWaylandClipboardCommand(bin, []string{"--list-types"}, nil)
	if pasteErr != nil {
		return ""
	}
	return portalMimeTypesContentType(splitClipboardLines(string(output)))
}

func waylandPaste(mimeTypes ...string) ([]byte, error) {
	bin, err := exec.LookPath("wl-paste")
	if err != nil {
		return nil, errors.New("clipboard: wl-paste is not available")
	}
	var lastErr error
	for _, mimeType := range mimeTypes {
		output, pasteErr := runWaylandClipboardCommand(bin, []string{"--type", mimeType}, nil)
		if pasteErr != nil {
			lastErr = pasteErr
			continue
		}
		if len(output) == 0 {
			lastErr = noDataErr
			continue
		}
		return output, nil
	}
	if lastErr == nil {
		return nil, noDataErr
	}
	return nil, lastErr
}

func runWaylandClipboardCommand(bin string, args []string, stdin []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), waylandClipboardCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	output, err := cmd.Output()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("clipboard: Wayland clipboard command timed out: %s", bin)
		}
		return nil, err
	}
	return output, nil
}

func runWaylandClipboardCommandErr(bin string, args []string, stdin []byte) error {
	_, err := runWaylandClipboardCommand(bin, args, stdin)
	return err
}
