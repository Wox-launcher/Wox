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
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"wox/util"

	"github.com/godbus/dbus/v5"
)

const (
	portalBusName                       = "org.freedesktop.portal.Desktop"
	portalObjectPath                    = dbus.ObjectPath("/org/freedesktop/portal/desktop")
	portalClipboardIFace                = "org.freedesktop.portal.Clipboard"
	portalRemoteDesktopIFace            = "org.freedesktop.portal.RemoteDesktop"
	portalRequestIFace                  = "org.freedesktop.portal.Request"
	portalSessionIFace                  = "org.freedesktop.portal.Session"
	portalRequestResponseSignal         = portalRequestIFace + ".Response"
	portalSelectionOwnerChangedSignal   = portalClipboardIFace + ".SelectionOwnerChanged"
	portalSelectionTransferSignal       = portalClipboardIFace + ".SelectionTransfer"
	portalMimeTextUTF8                  = "text/plain;charset=utf-8"
	portalMimeTextPlain                 = "text/plain"
	portalMimeURIList                   = "text/uri-list"
	portalMimePNG                       = "image/png"
	portalClipboardRequestTimeout       = 30 * time.Second
	portalClipboardNoDataLogInterval    = 30 * time.Second
	portalClipboardUnsupportedLogPrefix = "clipboard: Linux portal"
)

type linuxPortalClipboard struct {
	conn          *dbus.Conn
	sessionPath   dbus.ObjectPath
	signals       chan *dbus.Signal
	latest        portalClipboardOffer
	lastNoData    time.Time
	writePayloads map[string][]byte
	initialized   bool
	unavailable   bool
	unavailableE  error
}

type portalClipboardOffer struct {
	contentType Type
	mimeTypes   []string
	fingerprint string
}

var (
	linuxClipboardMu     sync.Mutex
	linuxClipboardPortal linuxPortalClipboard
	linuxPortalCounter   uint64
)

// readClipboardContentType returns the latest type advertised by the portal.
func readClipboardContentType() Type {
	linuxClipboardMu.Lock()
	defer linuxClipboardMu.Unlock()

	if err := ensureLinuxPortalClipboardLocked(); err != nil {
		return ""
	}
	return linuxClipboardPortal.latest.contentType
}

func readText() (string, error) {
	linuxClipboardMu.Lock()
	defer linuxClipboardMu.Unlock()

	if err := ensureLinuxPortalClipboardLocked(); err != nil {
		return "", err
	}
	mimeType := choosePortalMimeType(linuxClipboardPortal.latest.mimeTypes, portalMimeTextUTF8, portalMimeTextPlain)
	if mimeType == "" {
		return "", noDataErr
	}

	data, err := readLinuxPortalSelectionLocked(mimeType)
	if err != nil {
		return "", err
	}
	text := strings.TrimRight(string(data), "\x00")
	if text == "" {
		return "", noDataErr
	}
	return text, nil
}

func readFilePaths() ([]string, error) {
	linuxClipboardMu.Lock()
	defer linuxClipboardMu.Unlock()

	if err := ensureLinuxPortalClipboardLocked(); err != nil {
		return nil, err
	}
	if !portalMimeTypesContain(linuxClipboardPortal.latest.mimeTypes, portalMimeURIList) {
		return nil, noDataErr
	}

	data, err := readLinuxPortalSelectionLocked(portalMimeURIList)
	if err != nil {
		return nil, err
	}

	paths := parsePortalURIList(string(data))
	if len(paths) == 0 {
		return nil, noDataErr
	}
	return paths, nil
}

func readImage() (image.Image, error) {
	linuxClipboardMu.Lock()
	defer linuxClipboardMu.Unlock()

	if err := ensureLinuxPortalClipboardLocked(); err != nil {
		return nil, err
	}
	if !portalMimeTypesContain(linuxClipboardPortal.latest.mimeTypes, portalMimePNG) {
		return nil, noDataErr
	}

	data, err := readLinuxPortalSelectionLocked(portalMimePNG)
	if err != nil {
		return nil, err
	}
	img, decodeErr := png.Decode(bytes.NewReader(data))
	if decodeErr != nil {
		return nil, fmt.Errorf("clipboard: failed to decode portal clipboard PNG: %w", decodeErr)
	}
	return img, nil
}

func writeTextData(text string) error {
	linuxClipboardMu.Lock()
	defer linuxClipboardMu.Unlock()

	if err := ensureLinuxPortalClipboardLocked(); err != nil {
		return err
	}

	payload := []byte(text)
	return setLinuxPortalSelectionLocked(
		ClipboardTypeText,
		[]string{portalMimeTextUTF8, portalMimeTextPlain},
		map[string][]byte{
			portalMimeTextUTF8:  payload,
			portalMimeTextPlain: payload,
		},
	)
}

func writeFilePaths(filePaths []string) error {
	payload, err := buildPortalURIListPayload(filePaths)
	if err != nil {
		return err
	}

	linuxClipboardMu.Lock()
	defer linuxClipboardMu.Unlock()

	if err := ensureLinuxPortalClipboardLocked(); err != nil {
		return err
	}

	return setLinuxPortalSelectionLocked(
		ClipboardTypeFile,
		[]string{portalMimeURIList},
		map[string][]byte{
			portalMimeURIList: payload,
		},
	)
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

	linuxClipboardMu.Lock()
	defer linuxClipboardMu.Unlock()

	if err := ensureLinuxPortalClipboardLocked(); err != nil {
		return err
	}
	return setLinuxPortalSelectionLocked(
		ClipboardTypeImage,
		[]string{portalMimePNG},
		map[string][]byte{
			portalMimePNG: pngData,
		},
	)
}

func isClipboardChanged() bool {
	linuxClipboardMu.Lock()
	defer linuxClipboardMu.Unlock()

	if err := ensureLinuxPortalClipboardLocked(); err != nil {
		logLinuxPortalNoDataLocked(err)
		return false
	}

	return drainLinuxPortalClipboardSignalsLocked()
}

func buildWatchSnapshot() string {
	linuxClipboardMu.Lock()
	defer linuxClipboardMu.Unlock()

	if err := ensureLinuxPortalClipboardLocked(); err != nil {
		return fmt.Sprintf("portal_error=%s", err.Error())
	}
	return fmt.Sprintf("type=%s mimes=%s", linuxClipboardPortal.latest.contentType, strings.Join(linuxClipboardPortal.latest.mimeTypes, ","))
}

func ensureLinuxPortalClipboardLocked() error {
	if linuxClipboardPortal.initialized {
		return nil
	}
	if linuxClipboardPortal.unavailable {
		return linuxClipboardPortal.unavailableE
	}

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return markLinuxPortalUnavailableLocked(fmt.Errorf("clipboard: failed to connect to session bus: %w", err))
	}
	if !conn.SupportsUnixFDs() {
		_ = conn.Close()
		return markLinuxPortalUnavailableLocked(errors.New("clipboard: session bus does not support UnixFD transfer"))
	}

	if err := verifyLinuxPortalClipboardVersion(conn); err != nil {
		_ = conn.Close()
		return markLinuxPortalUnavailableLocked(err)
	}

	signals := make(chan *dbus.Signal, 32)
	conn.Signal(signals)

	sessionPath, err := createLinuxPortalClipboardSession(conn)
	if err != nil {
		conn.RemoveSignal(signals)
		_ = conn.Close()
		return markLinuxPortalUnavailableLocked(err)
	}

	if err := requestLinuxPortalClipboard(conn, sessionPath); err != nil {
		closeLinuxPortalSession(conn, sessionPath)
		conn.RemoveSignal(signals)
		_ = conn.Close()
		return markLinuxPortalUnavailableLocked(err)
	}

	clipboardEnabled, err := startLinuxPortalClipboardSession(conn, sessionPath)
	if err != nil {
		closeLinuxPortalSession(conn, sessionPath)
		conn.RemoveSignal(signals)
		_ = conn.Close()
		return markLinuxPortalUnavailableLocked(err)
	}
	if !clipboardEnabled {
		closeLinuxPortalSession(conn, sessionPath)
		conn.RemoveSignal(signals)
		_ = conn.Close()
		return markLinuxPortalUnavailableLocked(errors.New("clipboard: portal session did not enable clipboard access"))
	}

	if err := conn.AddMatchSignal(
		dbus.WithMatchInterface(portalClipboardIFace),
		dbus.WithMatchMember("SelectionOwnerChanged"),
	); err != nil {
		closeLinuxPortalSession(conn, sessionPath)
		conn.RemoveSignal(signals)
		_ = conn.Close()
		return markLinuxPortalUnavailableLocked(fmt.Errorf("clipboard: failed to subscribe to portal clipboard owner changes: %w", err))
	}
	if err := conn.AddMatchSignal(
		dbus.WithMatchInterface(portalClipboardIFace),
		dbus.WithMatchMember("SelectionTransfer"),
	); err != nil {
		closeLinuxPortalSession(conn, sessionPath)
		conn.RemoveSignal(signals)
		_ = conn.Close()
		return markLinuxPortalUnavailableLocked(fmt.Errorf("clipboard: failed to subscribe to portal clipboard transfers: %w", err))
	}

	linuxClipboardPortal.conn = conn
	linuxClipboardPortal.sessionPath = sessionPath
	linuxClipboardPortal.signals = signals
	linuxClipboardPortal.initialized = true
	util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf("clipboard: Linux portal backend active session=%s", sessionPath))
	drainLinuxPortalClipboardSignalsLocked()
	return nil
}

func markLinuxPortalUnavailableLocked(err error) error {
	linuxClipboardPortal.unavailable = true
	linuxClipboardPortal.unavailableE = err
	util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("%s unavailable: %s", portalClipboardUnsupportedLogPrefix, err.Error()))
	return err
}

func verifyLinuxPortalClipboardVersion(conn *dbus.Conn) error {
	portalObject := conn.Object(portalBusName, portalObjectPath)
	versionVariant, err := portalObject.GetProperty(portalClipboardIFace + ".version")
	if err != nil {
		return fmt.Errorf("clipboard: portal clipboard interface is not available: %w", err)
	}
	version, ok := versionVariant.Value().(uint32)
	if !ok || version == 0 {
		return errors.New("clipboard: portal clipboard interface returned an invalid version")
	}
	return nil
}

func createLinuxPortalClipboardSession(conn *dbus.Conn) (dbus.ObjectPath, error) {
	handleToken := nextLinuxPortalToken("clipboard_create")
	sessionToken := nextLinuxPortalToken("clipboard_session")
	options := map[string]dbus.Variant{
		"handle_token":         dbus.MakeVariant(handleToken),
		"session_handle_token": dbus.MakeVariant(sessionToken),
	}

	responseCode, results, err := callLinuxPortalRequest(conn, handleToken, func() (dbus.ObjectPath, error) {
		var requestHandle dbus.ObjectPath
		call := conn.Object(portalBusName, portalObjectPath).Call(
			portalRemoteDesktopIFace+".CreateSession",
			0,
			options,
		)
		if call.Err != nil {
			return "", fmt.Errorf("clipboard: failed to create portal remote desktop session: %w", call.Err)
		}
		if err := call.Store(&requestHandle); err != nil {
			return "", fmt.Errorf("clipboard: failed to decode portal session request handle: %w", err)
		}
		return requestHandle, nil
	})
	if err != nil {
		return "", fmt.Errorf("clipboard: failed waiting for portal create session response: %w", err)
	}
	if responseCode != 0 {
		return "", fmt.Errorf("clipboard: portal session request failed with response code %d", responseCode)
	}
	return parseLinuxPortalSessionHandle(results)
}

func requestLinuxPortalClipboard(conn *dbus.Conn, sessionPath dbus.ObjectPath) error {
	call := conn.Object(portalBusName, portalObjectPath).Call(
		portalClipboardIFace+".RequestClipboard",
		0,
		sessionPath,
		map[string]dbus.Variant{},
	)
	if call.Err != nil {
		return fmt.Errorf("clipboard: failed to request portal clipboard access: %w", call.Err)
	}
	return nil
}

func startLinuxPortalClipboardSession(conn *dbus.Conn, sessionPath dbus.ObjectPath) (bool, error) {
	handleToken := nextLinuxPortalToken("clipboard_start")
	options := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(handleToken),
	}

	responseCode, results, err := callLinuxPortalRequest(conn, handleToken, func() (dbus.ObjectPath, error) {
		var requestHandle dbus.ObjectPath
		call := conn.Object(portalBusName, portalObjectPath).Call(
			portalRemoteDesktopIFace+".Start",
			0,
			sessionPath,
			"",
			options,
		)
		if call.Err != nil {
			return "", fmt.Errorf("clipboard: failed to start portal clipboard session: %w", call.Err)
		}
		if err := call.Store(&requestHandle); err != nil {
			return "", fmt.Errorf("clipboard: failed to decode portal start request handle: %w", err)
		}
		return requestHandle, nil
	})
	if err != nil {
		return false, fmt.Errorf("clipboard: failed waiting for portal start response: %w", err)
	}
	if responseCode != 0 {
		return false, fmt.Errorf("clipboard: portal start request failed with response code %d", responseCode)
	}

	clipboardEnabledVariant, ok := results["clipboard_enabled"]
	if !ok {
		return false, nil
	}
	clipboardEnabled, ok := clipboardEnabledVariant.Value().(bool)
	return ok && clipboardEnabled, nil
}

func callLinuxPortalRequest(conn *dbus.Conn, handleToken string, invoke func() (dbus.ObjectPath, error)) (uint32, map[string]dbus.Variant, error) {
	expectedHandle, err := linuxPortalExpectedRequestPath(conn, handleToken)
	if err != nil {
		return 0, nil, err
	}

	signals := make(chan *dbus.Signal, 1)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)

	matchOptions := []dbus.MatchOption{
		dbus.WithMatchObjectPath(expectedHandle),
		dbus.WithMatchInterface(portalRequestIFace),
		dbus.WithMatchMember("Response"),
	}
	if err := conn.AddMatchSignal(matchOptions...); err != nil {
		return 0, nil, fmt.Errorf("clipboard: failed to subscribe to portal request response: %w", err)
	}
	defer func() {
		_ = conn.RemoveMatchSignal(matchOptions...)
	}()

	actualHandle, err := invoke()
	if err != nil {
		return 0, nil, err
	}
	if actualHandle != "" && actualHandle != expectedHandle {
		util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf(
			"clipboard: portal returned non-standard request handle expected=%s actual=%s",
			expectedHandle,
			actualHandle,
		))
	}

	timeout := time.NewTimer(portalClipboardRequestTimeout)
	defer timeout.Stop()

	for {
		select {
		case signal := <-signals:
			if signal == nil || signal.Name != portalRequestResponseSignal || len(signal.Body) != 2 {
				continue
			}

			responseCode, ok := signal.Body[0].(uint32)
			if !ok {
				return 0, nil, errors.New("clipboard: portal request response had an invalid response code")
			}

			results, ok := signal.Body[1].(map[string]dbus.Variant)
			if !ok {
				return 0, nil, errors.New("clipboard: portal request response had invalid result payload")
			}

			return responseCode, results, nil
		case <-timeout.C:
			return 0, nil, errors.New("clipboard: timed out waiting for portal request response")
		}
	}
}

func linuxPortalExpectedRequestPath(conn *dbus.Conn, handleToken string) (dbus.ObjectPath, error) {
	names := conn.Names()
	if len(names) == 0 || names[0] == "" {
		return "", errors.New("clipboard: failed to get portal D-Bus unique name")
	}
	sender := strings.TrimPrefix(names[0], ":")
	sender = strings.ReplaceAll(sender, ".", "_")
	return dbus.ObjectPath("/org/freedesktop/portal/desktop/request/" + sender + "/" + handleToken), nil
}

func parseLinuxPortalSessionHandle(results map[string]dbus.Variant) (dbus.ObjectPath, error) {
	sessionHandleVariant, ok := results["session_handle"]
	if !ok {
		return "", errors.New("clipboard: portal response did not include a session handle")
	}

	sessionHandle, ok := sessionHandleVariant.Value().(string)
	if !ok {
		return "", errors.New("clipboard: portal response returned an invalid session handle type")
	}
	if sessionHandle == "" {
		return "", errors.New("clipboard: portal response returned an empty session handle")
	}
	return dbus.ObjectPath(sessionHandle), nil
}

func closeLinuxPortalSession(conn *dbus.Conn, sessionPath dbus.ObjectPath) {
	if conn == nil || sessionPath == "" {
		return
	}
	_ = conn.Object(portalBusName, sessionPath).Call(portalSessionIFace+".Close", 0).Err
}

func drainLinuxPortalClipboardSignalsLocked() bool {
	changed := false
	for {
		select {
		case signal := <-linuxClipboardPortal.signals:
			if signal == nil {
				continue
			}
			switch signal.Name {
			case portalSelectionOwnerChangedSignal:
				if handleLinuxPortalSelectionOwnerChangedLocked(signal) {
					changed = true
				}
			case portalSelectionTransferSignal:
				handleLinuxPortalSelectionTransferLocked(signal)
			}
		default:
			if linuxClipboardPortal.latest.contentType == "" {
				logLinuxPortalNoDataLocked(noDataErr)
			}
			return changed
		}
	}
}

func handleLinuxPortalSelectionOwnerChangedLocked(signal *dbus.Signal) bool {
	if len(signal.Body) != 2 {
		return false
	}
	sessionPath, ok := signal.Body[0].(dbus.ObjectPath)
	if !ok || sessionPath != linuxClipboardPortal.sessionPath {
		return false
	}

	options, ok := signal.Body[1].(map[string]dbus.Variant)
	if !ok {
		return false
	}
	if sessionIsOwnerVariant, ok := options["session_is_owner"]; ok {
		if sessionIsOwner, valid := sessionIsOwnerVariant.Value().(bool); valid && sessionIsOwner {
			return false
		}
	}

	mimeTypes := portalVariantStringSlice(options["mime_types"])
	contentType := portalMimeTypesContentType(mimeTypes)
	fingerprint := strings.Join(mimeTypes, "\x00")
	if contentType == ClipboardTypeText {
		if mimeType := choosePortalMimeType(mimeTypes, portalMimeTextUTF8, portalMimeTextPlain); mimeType != "" {
			if data, err := readLinuxPortalSelectionLocked(mimeType); err == nil {
				fingerprint = "text:" + hashLinuxClipboardBytes(data)
			}
		}
	}

	offer := portalClipboardOffer{
		contentType: contentType,
		mimeTypes:   mimeTypes,
		fingerprint: fingerprint,
	}
	if offer.fingerprint == linuxClipboardPortal.latest.fingerprint {
		return false
	}

	linuxClipboardPortal.latest = offer
	util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf(
		"clipboard: Linux portal owner changed type=%s mimes=%s",
		contentType,
		strings.Join(mimeTypes, ","),
	))
	return contentType != ""
}

func handleLinuxPortalSelectionTransferLocked(signal *dbus.Signal) {
	if len(signal.Body) != 3 {
		return
	}
	sessionPath, ok := signal.Body[0].(dbus.ObjectPath)
	if !ok || sessionPath != linuxClipboardPortal.sessionPath {
		return
	}
	mimeType, ok := signal.Body[1].(string)
	if !ok {
		return
	}
	serial, ok := signal.Body[2].(uint32)
	if !ok {
		return
	}
	payload, ok := linuxClipboardPortal.writePayloads[mimeType]
	if !ok {
		finishLinuxPortalSelectionWriteLocked(serial, false)
		return
	}

	fd, err := createLinuxPortalSelectionWriteFDLocked(serial)
	if err != nil {
		util.GetLogger().Warn(util.NewTraceContext(), err.Error())
		finishLinuxPortalSelectionWriteLocked(serial, false)
		return
	}

	file := os.NewFile(uintptr(fd), "portal-clipboard-write")
	if file == nil {
		finishLinuxPortalSelectionWriteLocked(serial, false)
		return
	}
	written, writeErr := file.Write(payload)
	closeErr := file.Close()
	success := writeErr == nil && closeErr == nil && written == len(payload)
	if !success {
		util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf(
			"clipboard: failed to write portal selection mime=%s bytes=%d written=%d writeErr=%v closeErr=%v",
			mimeType,
			len(payload),
			written,
			writeErr,
			closeErr,
		))
	}
	finishLinuxPortalSelectionWriteLocked(serial, success)
}

// setLinuxPortalSelectionLocked advertises MIME types and stores the bytes served later by SelectionTransfer.
func setLinuxPortalSelectionLocked(contentType Type, mimeTypes []string, mimePayloads map[string][]byte) error {
	payloads := make(map[string][]byte, len(mimePayloads))
	for _, mimeType := range mimeTypes {
		payload, ok := mimePayloads[mimeType]
		if !ok {
			return fmt.Errorf("clipboard: missing portal payload for %s", mimeType)
		}
		payloads[mimeType] = append([]byte(nil), payload...)
	}

	previousPayloads := linuxClipboardPortal.writePayloads
	previousLatest := linuxClipboardPortal.latest
	linuxClipboardPortal.writePayloads = payloads

	options := map[string]dbus.Variant{
		"mime_types": dbus.MakeVariant(append([]string(nil), mimeTypes...)),
	}
	call := linuxClipboardPortal.conn.Object(portalBusName, portalObjectPath).Call(
		portalClipboardIFace+".SetSelection",
		0,
		linuxClipboardPortal.sessionPath,
		options,
	)
	if call.Err != nil {
		linuxClipboardPortal.writePayloads = previousPayloads
		linuxClipboardPortal.latest = previousLatest
		return fmt.Errorf("clipboard: failed to set portal selection: %w", call.Err)
	}

	linuxClipboardPortal.latest = portalClipboardOffer{
		contentType: contentType,
		mimeTypes:   append([]string(nil), mimeTypes...),
		fingerprint: hashLinuxPortalSelectionPayload(contentType, mimeTypes, payloads),
	}
	return nil
}

func createLinuxPortalSelectionWriteFDLocked(serial uint32) (int, error) {
	var fd dbus.UnixFD
	call := linuxClipboardPortal.conn.Object(portalBusName, portalObjectPath).Call(
		portalClipboardIFace+".SelectionWrite",
		0,
		linuxClipboardPortal.sessionPath,
		serial,
	)
	if call.Err != nil {
		return 0, fmt.Errorf("clipboard: failed to open portal selection write fd: %w", call.Err)
	}
	if err := call.Store(&fd); err != nil {
		return 0, fmt.Errorf("clipboard: failed to decode portal selection write fd: %w", err)
	}
	return int(fd), nil
}

// finishLinuxPortalSelectionWriteLocked tells the portal whether serving a requested selection succeeded.
func finishLinuxPortalSelectionWriteLocked(serial uint32, success bool) {
	_ = linuxClipboardPortal.conn.Object(portalBusName, portalObjectPath).Call(
		portalClipboardIFace+".SelectionWriteDone",
		0,
		linuxClipboardPortal.sessionPath,
		serial,
		success,
	).Err
}

func readLinuxPortalSelectionLocked(mimeType string) ([]byte, error) {
	var fd dbus.UnixFD
	call := linuxClipboardPortal.conn.Object(portalBusName, portalObjectPath).Call(
		portalClipboardIFace+".SelectionRead",
		0,
		linuxClipboardPortal.sessionPath,
		mimeType,
	)
	if call.Err != nil {
		return nil, fmt.Errorf("clipboard: failed to read portal selection %s: %w", mimeType, call.Err)
	}
	if err := call.Store(&fd); err != nil {
		return nil, fmt.Errorf("clipboard: failed to decode portal selection fd: %w", err)
	}

	file := os.NewFile(uintptr(fd), "portal-clipboard-read")
	if file == nil {
		return nil, errors.New("clipboard: failed to create file from portal selection fd")
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("clipboard: failed to read portal selection fd: %w", err)
	}
	if len(data) == 0 {
		return nil, noDataErr
	}
	return data, nil
}

func logLinuxPortalNoDataLocked(err error) {
	now := time.Now()
	if !linuxClipboardPortal.lastNoData.IsZero() && now.Sub(linuxClipboardPortal.lastNoData) < portalClipboardNoDataLogInterval {
		return
	}
	linuxClipboardPortal.lastNoData = now
	util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf("clipboard: Linux portal watcher sees no readable clipboard content: %v", err))
}

func portalMimeTypesContentType(mimeTypes []string) Type {
	if portalMimeTypesContain(mimeTypes, portalMimeURIList) {
		return ClipboardTypeFile
	}
	if portalMimeTypesContain(mimeTypes, portalMimePNG) {
		return ClipboardTypeImage
	}
	if choosePortalMimeType(mimeTypes, portalMimeTextUTF8, portalMimeTextPlain) != "" {
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

func portalVariantStringSlice(value dbus.Variant) []string {
	switch typed := value.Value().(type) {
	case []string:
		return typed
	case []interface{}:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
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

func nextLinuxPortalToken(prefix string) string {
	return fmt.Sprintf("wox_%s_%d", prefix, atomic.AddUint64(&linuxPortalCounter, 1))
}

func hashLinuxClipboardBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// hashLinuxPortalSelectionPayload fingerprints the owned selection across type, MIME order, and bytes.
func hashLinuxPortalSelectionPayload(contentType Type, mimeTypes []string, payloads map[string][]byte) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(contentType))
	for _, mimeType := range mimeTypes {
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(mimeType))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write(payloads[mimeType])
	}
	return hex.EncodeToString(hash.Sum(nil))
}
