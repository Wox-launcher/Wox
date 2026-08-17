//go:build linux

package clipboard

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/png"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"wox/util"
)

const (
	portalMimeTextUTF8              = "text/plain;charset=utf-8"
	portalMimeTextPlain             = "text/plain"
	portalMimeURIList               = "text/uri-list"
	portalMimePNG                   = "image/png"
	linuxClipboardNoDataLogInterval = 30 * time.Second
	dataControlCacheDuration        = 100 * time.Millisecond
)

// linuxClipboard is one desktop-session backend. Selection happens once so
// X11 sessions never probe portal or Wayland data-control.
type linuxClipboard interface {
	name() string
	readContentType() Type
	readText() (string, error)
	readFilePaths() ([]string, error)
	readImage() (image.Image, error)
	writeText(text string) error
	writeFilePaths(paths []string) error
	writeImageBytes(pngData []byte) error
	isChanged() bool
	watchSnapshot() string
}

var (
	linuxBackendOnce sync.Once
	linuxBackend     linuxClipboard
	dataControlMu    sync.Mutex
	linuxDataControl struct {
		latest      dataControlSelection
		fingerprint string
		capturedAt  time.Time
		lastNoData  time.Time
		activeLog   bool
	}
)

func linuxClipboardBackend() linuxClipboard {
	linuxBackendOnce.Do(func() {
		linuxBackend = selectLinuxClipboard()
		util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf("clipboard: Linux backend selected: %s", linuxBackend.name()))
	})
	return linuxBackend
}

func selectLinuxClipboard() linuxClipboard {
	return selectLinuxClipboardFor(
		util.IsLinuxWaylandSession(),
		util.IsKDEDesktopSession(),
		util.IsGnomeDesktopSession(),
		util.IsHyprlandSession(),
	)
}

// selectLinuxClipboardFor picks a backend from session facts. GNOME/KDE Wayland
// keep the portal path; every X11 desktop uses the X11 backend.
func selectLinuxClipboardFor(wayland, kde, gnome, hyprland bool) linuxClipboard {
	if wayland {
		if kde {
			return newKDEWaylandClipboard()
		}
		if gnome {
			return newGnomeWaylandClipboard()
		}
		if hyprland {
			return newHyprlandClipboard()
		}
		return newWaylandClipboard()
	}
	return newX11Clipboard()
}

func readClipboardContentType() Type {
	return linuxClipboardBackend().readContentType()
}

func readText() (string, error) {
	return linuxClipboardBackend().readText()
}

func readFilePaths() ([]string, error) {
	return linuxClipboardBackend().readFilePaths()
}

func readImage() (image.Image, error) {
	return linuxClipboardBackend().readImage()
}

func writeTextData(text string) error {
	return linuxClipboardBackend().writeText(text)
}

func writeFilePaths(filePaths []string) error {
	return linuxClipboardBackend().writeFilePaths(filePaths)
}

func writeImageData(img image.Image) error {
	if img == nil {
		return errors.New("clipboard: image is nil")
	}
	buf := new(bytes.Buffer)
	if err := png.Encode(buf, img); err != nil {
		return fmt.Errorf("clipboard: failed to encode image to PNG: %w", err)
	}
	return writeImageBytes(buf.Bytes(), nil)
}

func writeImageBytes(pngData []byte, dibData []byte) error {
	if len(pngData) == 0 {
		return errors.New("clipboard: PNG data is empty")
	}
	return linuxClipboardBackend().writeImageBytes(pngData)
}

func isClipboardChanged() bool {
	return linuxClipboardBackend().isChanged()
}

func buildWatchSnapshot() string {
	return linuxClipboardBackend().watchSnapshot()
}

func dataControlReadContentType() Type {
	selection, err := readDataControlSelectionLocked(false)
	if err != nil {
		return ""
	}
	return dataControlSelectionContentType(selection)
}

func dataControlReadText() (string, error) {
	selection, err := readDataControlSelectionLocked(false)
	if err != nil {
		return "", err
	}
	if dataControlSelectionContentType(selection) != ClipboardTypeText {
		return "", noDataErr
	}
	text := strings.TrimRight(string(selection.data), "\x00")
	if text == "" {
		return "", noDataErr
	}
	return text, nil
}

func dataControlReadFilePaths() ([]string, error) {
	selection, err := readDataControlSelectionLocked(false)
	if err != nil {
		return nil, err
	}
	if dataControlSelectionContentType(selection) != ClipboardTypeFile {
		return nil, noDataErr
	}
	paths := parsePortalURIList(string(selection.data))
	if len(paths) == 0 {
		return nil, noDataErr
	}
	return paths, nil
}

func dataControlReadImage() (image.Image, error) {
	selection, err := readDataControlSelectionLocked(false)
	if err != nil {
		return nil, err
	}
	if dataControlSelectionContentType(selection) != ClipboardTypeImage {
		return nil, noDataErr
	}
	img, decodeErr := png.Decode(bytes.NewReader(selection.data))
	if decodeErr != nil {
		return nil, fmt.Errorf("clipboard: failed to decode data-control PNG: %w", decodeErr)
	}
	return img, nil
}

func dataControlIsChanged() bool {
	selection, err := readDataControlSelectionLocked(true)
	if err != nil {
		logLinuxDataControlNoDataLocked(err)
		return false
	}
	fingerprint := selection.mimeType + ":" + hashLinuxClipboardBytes(selection.data)
	dataControlMu.Lock()
	defer dataControlMu.Unlock()
	if fingerprint == linuxDataControl.fingerprint {
		return false
	}
	linuxDataControl.fingerprint = fingerprint
	if !linuxDataControl.activeLog {
		linuxDataControl.activeLog = true
		util.GetLogger().Info(util.NewTraceContext(), "clipboard: Linux ext-data-control-v1 backend active")
	}
	return dataControlSelectionContentType(selection) != ""
}

func dataControlWatchSnapshot() string {
	selection, err := readDataControlSelectionLocked(false)
	if err != nil {
		return fmt.Sprintf("backend=ext-data-control-v1 error=%s", err.Error())
	}
	return fmt.Sprintf(
		"backend=ext-data-control-v1 type=%s mime=%s bytes=%d",
		dataControlSelectionContentType(selection),
		selection.mimeType,
		len(selection.data),
	)
}

// readDataControlSelectionLocked caches one short-lived snapshot so Read can inspect
// its type and payload without opening two independent Wayland selections.
func readDataControlSelectionLocked(force bool) (dataControlSelection, error) {
	dataControlMu.Lock()
	defer dataControlMu.Unlock()

	if !force && !linuxDataControl.capturedAt.IsZero() && time.Since(linuxDataControl.capturedAt) < dataControlCacheDuration {
		if linuxDataControl.latest.mimeType == "" {
			return dataControlSelection{}, noDataErr
		}
		return linuxDataControl.latest, nil
	}

	selection, err := readDataControlSelection()
	linuxDataControl.capturedAt = time.Now()
	if err != nil {
		linuxDataControl.latest = dataControlSelection{}
		return dataControlSelection{}, err
	}
	linuxDataControl.latest = selection
	return selection, nil
}

func logLinuxDataControlNoDataLocked(err error) {
	dataControlMu.Lock()
	defer dataControlMu.Unlock()

	now := time.Now()
	if !linuxDataControl.lastNoData.IsZero() && now.Sub(linuxDataControl.lastNoData) < linuxClipboardNoDataLogInterval {
		return
	}
	linuxDataControl.lastNoData = now
	util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf("clipboard: Linux data-control watcher sees no readable clipboard content: %v", err))
}

func dataControlSelectionContentType(selection dataControlSelection) Type {
	switch {
	case strings.EqualFold(selection.mimeType, portalMimeURIList):
		return ClipboardTypeFile
	case strings.EqualFold(selection.mimeType, portalMimePNG):
		return ClipboardTypeImage
	case strings.EqualFold(selection.mimeType, portalMimeTextUTF8),
		strings.EqualFold(selection.mimeType, portalMimeTextPlain),
		strings.EqualFold(selection.mimeType, "UTF8_STRING"):
		return ClipboardTypeText
	default:
		return ""
	}
}

func portalMimeTypesContentType(mimeTypes []string) Type {
	if portalMimeTypesContain(mimeTypes, portalMimeURIList) {
		return ClipboardTypeFile
	}
	if portalMimeTypesContain(mimeTypes, portalMimePNG) {
		return ClipboardTypeImage
	}
	if choosePortalMimeType(mimeTypes, portalMimeTextUTF8, portalMimeTextPlain, "UTF8_STRING", "STRING", "TEXT") != "" {
		return ClipboardTypeText
	}
	return ""
}

func choosePortalMimeType(mimeTypes []string, candidates ...string) string {
	for _, candidate := range candidates {
		if portalMimeTypesContain(mimeTypes, candidate) {
			return candidate
		}
	}
	return ""
}

func portalMimeTypesContain(mimeTypes []string, target string) bool {
	for _, mimeType := range mimeTypes {
		if strings.EqualFold(strings.TrimSpace(mimeType), target) {
			return true
		}
	}
	return false
}

func parsePortalURIList(uriList string) []string {
	lines := strings.Split(uriList, "\n")
	paths := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parsed, err := url.Parse(line)
		if err != nil || parsed.Scheme != "file" {
			continue
		}
		path, err := url.PathUnescape(parsed.Path)
		if err != nil || path == "" {
			continue
		}
		paths = append(paths, path)
	}
	return paths
}

// buildPortalURIListPayload encodes local file paths using the text/uri-list format expected by portals.
func buildPortalURIListPayload(filePaths []string) ([]byte, error) {
	uris := make([]string, 0, len(filePaths))
	for _, filePath := range filePaths {
		trimmedPath := strings.TrimSpace(filePath)
		if trimmedPath == "" {
			continue
		}

		absolutePath, err := filepath.Abs(trimmedPath)
		if err != nil {
			return nil, fmt.Errorf("clipboard: failed to make file path absolute %q: %w", trimmedPath, err)
		}
		uri := url.URL{
			Scheme: "file",
			Path:   filepath.Clean(absolutePath),
		}
		uris = append(uris, uri.String())
	}

	if len(uris) == 0 {
		return nil, errors.New("clipboard: file paths are empty")
	}

	return []byte(strings.Join(uris, "\r\n") + "\r\n"), nil
}

func hashLinuxClipboardBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
