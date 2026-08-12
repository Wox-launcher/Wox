//go:build linux

package screenshot

/*
#cgo CFLAGS: -std=c11 -D_GNU_SOURCE -D_REENTRANT -Wall -Wextra -Werror -I/usr/include/pipewire-0.3 -I/usr/include/spa-0.2
#cgo LDFLAGS: -ldl
#include <stdlib.h>
#include "../runtime/native_linux.h"
#include "platform_linux_pipewire.h"
*/
import "C"

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
	"wox/util"
	utilscreen "wox/util/screen"

	"github.com/godbus/dbus/v5"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/sys/unix"
)

const (
	linuxPortalBusName               = "org.freedesktop.portal.Desktop"
	linuxPortalObjectPath            = dbus.ObjectPath("/org/freedesktop/portal/desktop")
	linuxPortalScreenCastInterface   = "org.freedesktop.portal.ScreenCast"
	linuxPortalRequestInterface      = "org.freedesktop.portal.Request"
	linuxPortalSessionInterface      = "org.freedesktop.portal.Session"
	linuxPortalRequestResponseSignal = linuxPortalRequestInterface + ".Response"
	linuxPortalRequestTimeout        = 30 * time.Second
	linuxPortalPersistUntilRevoked   = 2
	linuxPortalRestoreTokenFile      = "screenshot-portal-restore-token"
)

type linuxPortalMonitor struct {
	ID     string
	NodeID uint32
	Bounds Rect
}

type linuxPortalStream struct {
	NodeID     uint32
	Properties map[string]dbus.Variant
}

type linuxPortalRestoreStore struct {
	Version int                       `json:"version"`
	Entries []linuxPortalRestoreEntry `json:"entries"`
}

type linuxPortalRestoreEntry struct {
	Token    string               `json:"token"`
	Monitors []linuxPortalMonitor `json:"monitors"`
}

type linuxPortalScreenIdentity struct {
	Logical     utilscreen.Size
	PixelWidth  int
	PixelHeight int
}

type linuxPortalDisplayGeometry struct {
	Name        string
	Logical     utilscreen.Rect
	PixelWidth  int
	PixelHeight int
}

// linuxPortalCaptureConfig keeps compositor policy out of the shared Portal transport.
type linuxPortalCaptureConfig struct {
	backend               string
	multiple              bool
	cursorMode            uint32
	initialLatestFrameFor time.Duration
	disableRestore        bool
	screenSpecificRestore bool
	restoreTokenFile      string
	displayGeometries     func() []linuxPortalDisplayGeometry
}

type linuxPortalIntPair struct {
	First  int32
	Second int32
}

var linuxPortalTokenCounter atomic.Uint64

type linuxPortalDesktopCapture struct {
	mu                    sync.Mutex
	conn                  *dbus.Conn
	sessionPath           dbus.ObjectPath
	monitors              []linuxPortalMonitor
	bounds                Rect
	pipewire              *C.WoxPipeWireCapture
	backend               string
	initialLatestFrameFor time.Duration
	closed                bool
}

func newLinuxPortalDesktopCapture(config linuxPortalCaptureConfig) (*linuxPortalDesktopCapture, error) {
	if config.backend == "" {
		config.backend = "portal"
	}
	if config.cursorMode == 0 {
		config.cursorMode = 1
	}
	displays := linuxScreenshotPortalGTKDisplayGeometries()
	if config.displayGeometries != nil {
		if configured := config.displayGeometries(); len(configured) > 0 {
			displays = configured
		}
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("connect to screenshot portal: %w", err)
	}
	sessionPath, monitors, err := startLinuxPortalCaptureSession(conn, config, displays)
	if err != nil {
		conn.Close()
		return nil, err
	}
	bounds, err := linuxPortalMonitorUnion(monitors)
	if err != nil {
		closeLinuxScreenshotPortalSession(conn, sessionPath)
		conn.Close()
		return nil, err
	}
	remoteFD, err := openLinuxPortalPipeWireRemote(conn, sessionPath)
	if err != nil {
		closeLinuxScreenshotPortalSession(conn, sessionPath)
		conn.Close()
		return nil, err
	}
	nodeMemory := C.malloc(C.size_t(len(monitors)) * C.size_t(unsafe.Sizeof(C.uint32_t(0))))
	if nodeMemory == nil {
		_ = unix.Close(remoteFD)
		closeLinuxScreenshotPortalSession(conn, sessionPath)
		conn.Close()
		return nil, errors.New("allocate ScreenCast PipeWire node state")
	}
	nodeIDs := unsafe.Slice((*C.uint32_t)(nodeMemory), len(monitors))
	for index, monitor := range monitors {
		nodeIDs[index] = C.uint32_t(monitor.NodeID)
	}
	pipewire := C.wox_screenshot_pipewire_create(C.int32_t(remoteFD), (*C.uint32_t)(nodeMemory), C.int32_t(len(monitors)))
	C.free(nodeMemory)
	if pipewire == nil {
		closeLinuxScreenshotPortalSession(conn, sessionPath)
		conn.Close()
		return nil, errors.New("connect to ScreenCast PipeWire streams")
	}
	return &linuxPortalDesktopCapture{
		conn:                  conn,
		sessionPath:           sessionPath,
		monitors:              monitors,
		bounds:                bounds,
		pipewire:              pipewire,
		backend:               config.backend,
		initialLatestFrameFor: config.initialLatestFrameFor,
	}, nil
}

func (capture *linuxPortalDesktopCapture) capture() (image.Image, error) {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	if capture.closed || capture.pipewire == nil {
		return nil, errors.New("ScreenCast PipeWire session is closed")
	}
	startedAt := time.Now()
	source, err := captureLinuxPipeWireFrames(capture.pipewire, capture.monitors, capture.initialLatestFrameFor)
	capture.initialLatestFrameFor = 0
	if err != nil {
		return nil, err
	}
	util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf(
		"[screenshot] captured Wayland desktop source=portal-pipewire backend=%s streams=%d logical=%.0f,%.0f %.0fx%.0f pixels=%dx%d elapsed=%s",
		capture.backend,
		len(capture.monitors),
		capture.bounds.X,
		capture.bounds.Y,
		capture.bounds.Width,
		capture.bounds.Height,
		source.Bounds().Dx(),
		source.Bounds().Dy(),
		time.Since(startedAt).Round(time.Millisecond),
	))
	return source, nil
}

func (capture *linuxPortalDesktopCapture) logicalBounds() Rect {
	return capture.bounds
}

func (capture *linuxPortalDesktopCapture) close() {
	if capture == nil {
		return
	}
	capture.mu.Lock()
	if capture.closed {
		capture.mu.Unlock()
		return
	}
	capture.closed = true
	if capture.pipewire != nil {
		C.wox_screenshot_pipewire_destroy(capture.pipewire)
		capture.pipewire = nil
	}
	closeLinuxScreenshotPortalSession(capture.conn, capture.sessionPath)
	if capture.conn != nil {
		capture.conn.Close()
	}
	capture.mu.Unlock()
}

func startLinuxPortalCaptureSession(conn *dbus.Conn, config linuxPortalCaptureConfig, displays []linuxPortalDisplayGeometry) (dbus.ObjectPath, []linuxPortalMonitor, error) {
	restoreTokenPath := ""
	restoreStore := linuxPortalRestoreStore{Version: 2}
	if !config.disableRestore {
		restoreTokenFile := config.restoreTokenFile
		if restoreTokenFile == "" {
			restoreTokenFile = linuxPortalRestoreTokenFile
		}
		restoreTokenPath = linuxScreenshotPortalRestoreTokenPath(restoreTokenFile)
		var loadErr error
		restoreStore, loadErr = loadLinuxScreenshotPortalRestoreStore(restoreTokenPath)
		if loadErr != nil {
			util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("[screenshot] failed to load portal restore token: %v", loadErr))
			_ = removeLinuxScreenshotPortalRestoreToken(restoreTokenPath)
		}
	}
	restoreEntryIndex := -1
	var restoreEntry linuxPortalRestoreEntry
	currentScreen := linuxScreenshotPortalCurrentScreenIdentity(displays)
	if config.screenSpecificRestore {
		cleanedStore := linuxScreenshotPortalCleanRestoreStore(restoreStore)
		if len(cleanedStore.Entries) != len(restoreStore.Entries) {
			restoreStore = cleanedStore
			if saveErr := saveLinuxScreenshotPortalRestoreStore(restoreTokenPath, restoreStore); saveErr != nil {
				util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("[screenshot] failed to clean legacy portal restore tokens: %v", saveErr))
			}
		}
	}
	if len(restoreStore.Entries) > 0 {
		if config.screenSpecificRestore {
			restoreEntryIndex, restoreEntry = linuxScreenshotPortalRestoreEntryForScreen(restoreStore, currentScreen)
			if restoreEntryIndex < 0 {
				util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf(
					"[screenshot] no ScreenCast restore token for current monitor current=%d,%d %dx%d pixels=%dx%d saved=%s",
					currentScreen.Logical.X,
					currentScreen.Logical.Y,
					currentScreen.Logical.Width,
					currentScreen.Logical.Height,
					currentScreen.PixelWidth,
					currentScreen.PixelHeight,
					linuxScreenshotPortalRestoreStoreSummary(restoreStore),
				))
			}
		} else {
			restoreEntryIndex = 0
			restoreEntry = restoreStore.Entries[0]
		}
	}
	restoreToken := restoreEntry.Token

	createToken := nextLinuxScreenshotPortalToken("create")
	sessionToken := nextLinuxScreenshotPortalToken("session")
	responseCode, results, err := callLinuxScreenshotPortalRequest(conn, createToken, func() (dbus.ObjectPath, error) {
		options := map[string]dbus.Variant{
			"handle_token":         dbus.MakeVariant(createToken),
			"session_handle_token": dbus.MakeVariant(sessionToken),
		}
		return callLinuxPortalObjectPathMethod(conn, linuxPortalScreenCastInterface+".CreateSession", options)
	})
	if err != nil {
		return "", nil, fmt.Errorf("create ScreenCast portal session: %w", err)
	}
	if responseCode != 0 {
		return "", nil, fmt.Errorf("ScreenCast portal session failed with response code %d", responseCode)
	}
	sessionPath, err := linuxPortalSessionPath(results)
	if err != nil {
		return "", nil, err
	}
	closeOnError := true
	defer func() {
		if closeOnError {
			closeLinuxScreenshotPortalSession(conn, sessionPath)
		}
	}()

	selectToken := nextLinuxScreenshotPortalToken("select")
	responseCode, _, err = callLinuxScreenshotPortalRequest(conn, selectToken, func() (dbus.ObjectPath, error) {
		options := linuxScreenshotPortalSelectSourcesOptions(selectToken, restoreToken, config.multiple, config.cursorMode)
		return callLinuxPortalObjectPathMethod(conn, linuxPortalScreenCastInterface+".SelectSources", sessionPath, options)
	})
	if err != nil {
		return "", nil, fmt.Errorf("select ScreenCast monitor sources: %w", err)
	}
	if responseCode != 0 {
		return "", nil, fmt.Errorf("ScreenCast monitor selection failed with response code %d", responseCode)
	}
	// Remove only the token sent for this session. Tokens for other monitors remain valid,
	// allowing each compositor output to be authorized once and then selected by pointer location.
	if restoreEntryIndex >= 0 {
		restoreStore.Entries = append(restoreStore.Entries[:restoreEntryIndex], restoreStore.Entries[restoreEntryIndex+1:]...)
		if saveErr := saveLinuxScreenshotPortalRestoreStore(restoreTokenPath, restoreStore); saveErr != nil {
			util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("[screenshot] failed to remove consumed portal restore token: %v", saveErr))
		}
	}

	startToken := nextLinuxScreenshotPortalToken("start")
	responseCode, results, err = callLinuxScreenshotPortalRequest(conn, startToken, func() (dbus.ObjectPath, error) {
		options := map[string]dbus.Variant{"handle_token": dbus.MakeVariant(startToken)}
		return callLinuxPortalObjectPathMethod(conn, linuxPortalScreenCastInterface+".Start", sessionPath, "", options)
	})
	if err != nil {
		return "", nil, fmt.Errorf("start ScreenCast portal session: %w", err)
	}
	if responseCode != 0 {
		return "", nil, fmt.Errorf("ScreenCast portal start failed with response code %d", responseCode)
	}
	monitors, err := parseLinuxPortalMonitors(results)
	if err != nil {
		return "", nil, err
	}
	monitors = normalizeLinuxPortalMonitors(monitors, currentScreen, displays)
	if nextRestoreToken := linuxScreenshotPortalRestoreToken(results); !config.disableRestore && nextRestoreToken != "" {
		restoreStore = linuxScreenshotPortalUpsertRestoreEntry(restoreStore, linuxPortalRestoreEntry{Token: nextRestoreToken, Monitors: monitors})
		if saveErr := saveLinuxScreenshotPortalRestoreStore(restoreTokenPath, restoreStore); saveErr != nil {
			util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("[screenshot] failed to save portal restore token: %v", saveErr))
		} else {
			util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf(
				"[screenshot] saved ScreenCast portal restore token monitors=%s",
				linuxScreenshotPortalRestoreMonitorSummary(monitors),
			))
		}
	}
	closeOnError = false
	return sessionPath, monitors, nil
}

func linuxScreenshotPortalSelectSourcesOptions(handleToken string, restoreToken string, multiple bool, cursorMode uint32) map[string]dbus.Variant {
	// Keep these optional vardict fields capability-driven by the backend. Current XDPH
	// stacks can implement restore data while the public ScreenCast version still reports 3.
	options := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(handleToken),
		"types":        dbus.MakeVariant(uint32(1)),
		"multiple":     dbus.MakeVariant(multiple),
		"cursor_mode":  dbus.MakeVariant(cursorMode),
		"persist_mode": dbus.MakeVariant(uint32(linuxPortalPersistUntilRevoked)),
	}
	if restoreToken != "" {
		options["restore_token"] = dbus.MakeVariant(restoreToken)
	}
	return options
}

// linuxScreenshotPortalCleanRestoreStore removes entries without monitor ownership metadata.
func linuxScreenshotPortalCleanRestoreStore(store linuxPortalRestoreStore) linuxPortalRestoreStore {
	validEntries := make([]linuxPortalRestoreEntry, 0, len(store.Entries))
	for _, entry := range store.Entries {
		if entry.Token == "" || len(entry.Monitors) == 0 {
			continue
		}
		validEntries = append(validEntries, entry)
	}
	store.Entries = validEntries
	return store
}

func linuxScreenshotPortalRestoreToken(results map[string]dbus.Variant) string {
	value, ok := results["restore_token"]
	if !ok {
		return ""
	}
	token, ok := value.Value().(string)
	if !ok {
		return ""
	}
	return token
}

func linuxScreenshotPortalRestoreTokenPath(fileName string) string {
	return filepath.Join(util.GetLocation().GetCacheDirectory(), fileName)
}

func loadLinuxScreenshotPortalRestoreStore(path string) (linuxPortalRestoreStore, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return linuxPortalRestoreStore{Version: 2}, nil
	}
	if err != nil {
		return linuxPortalRestoreStore{}, err
	}
	if len(data) > 64*1024 {
		return linuxPortalRestoreStore{}, errors.New("portal restore token is unexpectedly large")
	}
	var store linuxPortalRestoreStore
	if err := json.Unmarshal(data, &store); err == nil && store.Version == 2 {
		return store, nil
	}
	var legacyEntry linuxPortalRestoreEntry
	if err := json.Unmarshal(data, &legacyEntry); err == nil && legacyEntry.Token != "" {
		return linuxPortalRestoreStore{Version: 2, Entries: []linuxPortalRestoreEntry{legacyEntry}}, nil
	}
	if !json.Valid(data) {
		// The first restore-token implementation stored the token as plain text.
		// Preserve it only as migration input; missing monitor metadata forces one
		// fresh selection on Hyprland before silent restoration is enabled again.
		return linuxPortalRestoreStore{Version: 2, Entries: []linuxPortalRestoreEntry{{Token: string(data)}}}, nil
	}
	return linuxPortalRestoreStore{}, errors.New("portal restore token store is invalid")
}

func saveLinuxScreenshotPortalRestoreStore(path string, store linuxPortalRestoreStore) error {
	if len(store.Entries) == 0 {
		return removeLinuxScreenshotPortalRestoreToken(path)
	}
	for _, entry := range store.Entries {
		if entry.Token == "" {
			return errors.New("portal restore token is empty")
		}
	}
	store.Version = 2
	data, err := json.Marshal(store)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func linuxScreenshotPortalRestoreEntryMatchesScreen(entry linuxPortalRestoreEntry, screen linuxPortalScreenIdentity) bool {
	if entry.Token == "" || len(entry.Monitors) == 0 || screen.Logical.Width <= 0 || screen.Logical.Height <= 0 {
		return false
	}
	if len(entry.Monitors) > 1 {
		return true
	}
	monitor := entry.Monitors[0].Bounds
	if int(math.Round(float64(monitor.X))) == screen.Logical.X &&
		int(math.Round(float64(monitor.Y))) == screen.Logical.Y &&
		int(math.Round(float64(monitor.Width))) == screen.Logical.Width &&
		int(math.Round(float64(monitor.Height))) == screen.Logical.Height {
		return true
	}
	// Older XDPH responses normalize a selected monitor to 0,0 and expose its
	// physical buffer size. Match that persisted form during migration.
	return screen.PixelWidth > 0 && screen.PixelHeight > 0 &&
		int(math.Round(float64(monitor.Width))) == screen.PixelWidth &&
		int(math.Round(float64(monitor.Height))) == screen.PixelHeight
}

func linuxScreenshotPortalRestoreEntryForScreen(store linuxPortalRestoreStore, screen linuxPortalScreenIdentity) (int, linuxPortalRestoreEntry) {
	for index, entry := range store.Entries {
		if linuxScreenshotPortalRestoreEntryMatchesScreen(entry, screen) {
			return index, entry
		}
	}
	return -1, linuxPortalRestoreEntry{}
}

func linuxScreenshotPortalCurrentScreenIdentity(displays []linuxPortalDisplayGeometry) linuxPortalScreenIdentity {
	logical := utilscreen.GetMouseScreen()
	identity := linuxPortalScreenIdentity{Logical: logical}
	for _, display := range displays {
		if display.Logical.X == logical.X && display.Logical.Y == logical.Y &&
			display.Logical.Width == logical.Width && display.Logical.Height == logical.Height {
			identity.PixelWidth = display.PixelWidth
			identity.PixelHeight = display.PixelHeight
			return identity
		}
	}
	return identity
}

func normalizeLinuxPortalMonitors(monitors []linuxPortalMonitor, current linuxPortalScreenIdentity, displays []linuxPortalDisplayGeometry) []linuxPortalMonitor {
	if len(displays) == 0 {
		return monitors
	}
	normalized := append([]linuxPortalMonitor(nil), monitors...)
	usedDisplays := make(map[int]bool)
	for monitorIndex, monitor := range normalized {
		displayIndex := linuxScreenshotPortalDisplayIndex(monitor.Bounds, displays, usedDisplays, current)
		if displayIndex < 0 {
			continue
		}
		display := displays[displayIndex]
		usedDisplays[displayIndex] = true
		normalized[monitorIndex].Bounds = Rect{
			X:      float32(display.Logical.X),
			Y:      float32(display.Logical.Y),
			Width:  float32(display.Logical.Width),
			Height: float32(display.Logical.Height),
		}
		util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf(
			"[screenshot] mapped ScreenCast source id=%s portal=%.0f,%.0f %.0fx%.0f logical=%d,%d %dx%d pixels=%dx%d",
			monitor.ID,
			monitor.Bounds.X,
			monitor.Bounds.Y,
			monitor.Bounds.Width,
			monitor.Bounds.Height,
			display.Logical.X,
			display.Logical.Y,
			display.Logical.Width,
			display.Logical.Height,
			display.PixelWidth,
			display.PixelHeight,
		))
	}
	return normalized
}

func linuxScreenshotPortalDisplayIndex(bounds Rect, displays []linuxPortalDisplayGeometry, used map[int]bool, current linuxPortalScreenIdentity) int {
	for index, display := range displays {
		if used[index] {
			continue
		}
		if int(math.Round(float64(bounds.X))) == display.Logical.X &&
			int(math.Round(float64(bounds.Y))) == display.Logical.Y &&
			int(math.Round(float64(bounds.Width))) == display.Logical.Width &&
			int(math.Round(float64(bounds.Height))) == display.Logical.Height {
			return index
		}
	}
	physicalCandidates := make([]int, 0, len(displays))
	for index, display := range displays {
		if !used[index] && int(math.Round(float64(bounds.Width))) == display.PixelWidth && int(math.Round(float64(bounds.Height))) == display.PixelHeight {
			physicalCandidates = append(physicalCandidates, index)
		}
	}
	for _, index := range physicalCandidates {
		display := displays[index]
		if display.Logical.X == current.Logical.X && display.Logical.Y == current.Logical.Y &&
			display.Logical.Width == current.Logical.Width && display.Logical.Height == current.Logical.Height {
			return index
		}
	}
	if len(physicalCandidates) == 1 {
		return physicalCandidates[0]
	}
	return -1
}

func linuxScreenshotPortalGTKDisplayGeometries() []linuxPortalDisplayGeometry {
	displays, err := utilscreen.ListDisplays()
	if err != nil {
		return nil
	}
	result := make([]linuxPortalDisplayGeometry, 0, len(displays))
	for _, display := range displays {
		result = append(result, linuxPortalDisplayGeometry{
			Name:        display.Name,
			Logical:     display.Bounds,
			PixelWidth:  display.PixelBounds.Width,
			PixelHeight: display.PixelBounds.Height,
		})
	}
	return result
}

func linuxScreenshotPortalUpsertRestoreEntry(store linuxPortalRestoreStore, next linuxPortalRestoreEntry) linuxPortalRestoreStore {
	for index, entry := range store.Entries {
		if linuxScreenshotPortalMonitorSetKey(entry.Monitors) == linuxScreenshotPortalMonitorSetKey(next.Monitors) {
			store.Entries[index] = next
			return store
		}
	}
	store.Entries = append(store.Entries, next)
	return store
}

func linuxScreenshotPortalMonitorSetKey(monitors []linuxPortalMonitor) string {
	parts := make([]string, 0, len(monitors))
	for _, monitor := range monitors {
		parts = append(parts, fmt.Sprintf("%.0f,%.0f %.0fx%.0f", monitor.Bounds.X, monitor.Bounds.Y, monitor.Bounds.Width, monitor.Bounds.Height))
	}
	sort.Strings(parts)
	return strings.Join(parts, ";")
}

func linuxScreenshotPortalRestoreStoreSummary(store linuxPortalRestoreStore) string {
	parts := make([]string, 0, len(store.Entries))
	for _, entry := range store.Entries {
		parts = append(parts, linuxScreenshotPortalRestoreMonitorSummary(entry.Monitors))
	}
	return strings.Join(parts, "|")
}

func linuxScreenshotPortalRestoreMonitorSummary(monitors []linuxPortalMonitor) string {
	parts := make([]string, 0, len(monitors))
	for _, monitor := range monitors {
		parts = append(parts, fmt.Sprintf("%s@%.0f,%.0f %.0fx%.0f", monitor.ID, monitor.Bounds.X, monitor.Bounds.Y, monitor.Bounds.Width, monitor.Bounds.Height))
	}
	return strings.Join(parts, ";")
}

func removeLinuxScreenshotPortalRestoreToken(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func openLinuxPortalPipeWireRemote(conn *dbus.Conn, sessionPath dbus.ObjectPath) (int, error) {
	var remoteFD dbus.UnixFD
	call := conn.Object(linuxPortalBusName, linuxPortalObjectPath).Call(
		linuxPortalScreenCastInterface+".OpenPipeWireRemote",
		0,
		sessionPath,
		map[string]dbus.Variant{},
	)
	if call.Err != nil {
		return -1, fmt.Errorf("open ScreenCast PipeWire remote: %w", call.Err)
	}
	if err := call.Store(&remoteFD); err != nil {
		return -1, fmt.Errorf("decode ScreenCast PipeWire remote: %w", err)
	}
	defer unix.Close(int(remoteFD))
	duplicatedFD, err := unix.Dup(int(remoteFD))
	if err != nil {
		return -1, fmt.Errorf("duplicate ScreenCast PipeWire remote: %w", err)
	}
	return duplicatedFD, nil
}

func callLinuxPortalObjectPathMethod(conn *dbus.Conn, method string, arguments ...any) (dbus.ObjectPath, error) {
	var requestHandle dbus.ObjectPath
	call := conn.Object(linuxPortalBusName, linuxPortalObjectPath).Call(method, 0, arguments...)
	if call.Err != nil {
		return "", call.Err
	}
	if err := call.Store(&requestHandle); err != nil {
		return "", err
	}
	return requestHandle, nil
}

func callLinuxScreenshotPortalRequest(conn *dbus.Conn, handleToken string, invoke func() (dbus.ObjectPath, error)) (uint32, map[string]dbus.Variant, error) {
	expectedHandle, err := linuxScreenshotPortalExpectedRequestPath(conn, handleToken)
	if err != nil {
		return 0, nil, err
	}
	signals := make(chan *dbus.Signal, 1)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)
	matchOptions := []dbus.MatchOption{
		dbus.WithMatchObjectPath(expectedHandle),
		dbus.WithMatchInterface(linuxPortalRequestInterface),
		dbus.WithMatchMember("Response"),
	}
	if err := conn.AddMatchSignal(matchOptions...); err != nil {
		return 0, nil, fmt.Errorf("subscribe to portal response: %w", err)
	}
	defer func() { _ = conn.RemoveMatchSignal(matchOptions...) }()

	actualHandle, err := invoke()
	if err != nil {
		return 0, nil, err
	}
	if actualHandle != "" && actualHandle != expectedHandle {
		util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf(
			"[screenshot] portal returned non-standard request handle expected=%s actual=%s",
			expectedHandle,
			actualHandle,
		))
	}

	timer := time.NewTimer(linuxPortalRequestTimeout)
	defer timer.Stop()
	for {
		select {
		case signal := <-signals:
			if signal == nil || signal.Name != linuxPortalRequestResponseSignal || len(signal.Body) != 2 {
				continue
			}
			responseCode, ok := signal.Body[0].(uint32)
			if !ok {
				return 0, nil, errors.New("portal response had an invalid response code")
			}
			results, ok := signal.Body[1].(map[string]dbus.Variant)
			if !ok {
				return 0, nil, errors.New("portal response had an invalid result payload")
			}
			return responseCode, results, nil
		case <-timer.C:
			return 0, nil, errors.New("timed out waiting for portal response")
		}
	}
}

func linuxScreenshotPortalExpectedRequestPath(conn *dbus.Conn, handleToken string) (dbus.ObjectPath, error) {
	names := conn.Names()
	if len(names) == 0 || names[0] == "" {
		return "", errors.New("portal D-Bus connection has no unique name")
	}
	sender := strings.ReplaceAll(strings.TrimPrefix(names[0], ":"), ".", "_")
	return dbus.ObjectPath("/org/freedesktop/portal/desktop/request/" + sender + "/" + handleToken), nil
}

func linuxPortalSessionPath(results map[string]dbus.Variant) (dbus.ObjectPath, error) {
	value, ok := results["session_handle"]
	if !ok {
		return "", errors.New("ScreenCast portal did not return a session handle")
	}
	switch session := value.Value().(type) {
	case string:
		return dbus.ObjectPath(session), nil
	case dbus.ObjectPath:
		return session, nil
	default:
		return "", errors.New("ScreenCast portal returned an invalid session handle")
	}
}

func parseLinuxPortalMonitors(results map[string]dbus.Variant) ([]linuxPortalMonitor, error) {
	streamsVariant, ok := results["streams"]
	if !ok {
		return nil, errors.New("ScreenCast portal did not return monitor streams")
	}
	var streams []linuxPortalStream
	if err := dbus.Store([]any{streamsVariant.Value()}, &streams); err != nil {
		return nil, fmt.Errorf("decode ScreenCast monitor streams: %w", err)
	}
	monitors := make([]linuxPortalMonitor, 0, len(streams))
	for _, stream := range streams {
		if linuxPortalVariantUint32(stream.Properties["source_type"]) != 1 {
			continue
		}
		position, positionOK := linuxPortalVariantIntPair(stream.Properties["position"])
		size, sizeOK := linuxPortalVariantIntPair(stream.Properties["size"])
		if !positionOK || !sizeOK || size.First <= 0 || size.Second <= 0 {
			continue
		}
		id := fmt.Sprintf("portal-monitor-%d", stream.NodeID)
		if idVariant, exists := stream.Properties["id"]; exists {
			if portalID, valid := idVariant.Value().(string); valid && portalID != "" {
				id = portalID
			}
		}
		monitors = append(monitors, linuxPortalMonitor{
			ID:     id,
			NodeID: stream.NodeID,
			Bounds: Rect{
				X:      float32(position.First),
				Y:      float32(position.Second),
				Width:  float32(size.First),
				Height: float32(size.Second),
			},
		})
	}
	if len(monitors) == 0 {
		return nil, errors.New("ScreenCast portal did not expose monitor geometry")
	}
	return monitors, nil
}

func linuxPortalVariantIntPair(value dbus.Variant) (linuxPortalIntPair, bool) {
	if value.Signature().Empty() {
		return linuxPortalIntPair{}, false
	}
	var pair linuxPortalIntPair
	if err := dbus.Store([]any{value.Value()}, &pair); err != nil {
		return linuxPortalIntPair{}, false
	}
	return pair, true
}

func linuxPortalVariantUint32(value dbus.Variant) uint32 {
	result, _ := value.Value().(uint32)
	return result
}

func linuxPortalMonitorUnion(monitors []linuxPortalMonitor) (Rect, error) {
	if len(monitors) == 0 {
		return Rect{}, errors.New("no portal monitors are available")
	}
	left := monitors[0].Bounds.X
	top := monitors[0].Bounds.Y
	right := left + monitors[0].Bounds.Width
	bottom := top + monitors[0].Bounds.Height
	for _, monitor := range monitors[1:] {
		left = min(left, monitor.Bounds.X)
		top = min(top, monitor.Bounds.Y)
		right = max(right, monitor.Bounds.X+monitor.Bounds.Width)
		bottom = max(bottom, monitor.Bounds.Y+monitor.Bounds.Height)
	}
	if right <= left || bottom <= top {
		return Rect{}, errors.New("portal monitor union is empty")
	}
	return Rect{X: left, Y: top, Width: right - left, Height: bottom - top}, nil
}

func captureLinuxPipeWireFrames(pipewire *C.WoxPipeWireCapture, monitors []linuxPortalMonitor, latestFor time.Duration) (image.Image, error) {
	if pipewire == nil || len(monitors) == 0 {
		return nil, errors.New("ScreenCast PipeWire capture has no streams")
	}
	frameMemory := C.calloc(C.size_t(len(monitors)), C.size_t(unsafe.Sizeof(C.WoxPipeWireFrame{})))
	if frameMemory == nil {
		C.free(frameMemory)
		return nil, errors.New("allocate ScreenCast PipeWire frame state")
	}
	defer C.free(frameMemory)
	frames := unsafe.Slice((*C.WoxPipeWireFrame)(frameMemory), len(monitors))

	var result C.int32_t
	if latestFor > 0 {
		durationMilliseconds := max(int64(1), latestFor.Milliseconds())
		result = C.wox_screenshot_pipewire_capture_latest(
			pipewire,
			(*C.WoxPipeWireFrame)(frameMemory),
			C.int32_t(len(monitors)),
			C.int32_t(durationMilliseconds),
		)
	} else {
		result = C.wox_screenshot_pipewire_capture(
			pipewire,
			(*C.WoxPipeWireFrame)(frameMemory),
			C.int32_t(len(monitors)),
			C.int32_t(10),
		)
	}
	defer C.wox_screenshot_pipewire_free_frames((*C.WoxPipeWireFrame)(frameMemory), C.int32_t(len(monitors)))
	if result != 0 {
		return nil, fmt.Errorf("capture ScreenCast PipeWire frame failed with status %d", int32(result))
	}

	images := make([]*image.RGBA, len(frames))
	for index, frame := range frames {
		if frame.pixels == nil || frame.width == 0 || frame.height == 0 || frame.stride < frame.width*4 {
			return nil, fmt.Errorf("ScreenCast PipeWire stream %d returned an invalid frame", index)
		}
		pixelBytes := C.GoBytes(unsafe.Pointer(frame.pixels), C.int(uint64(frame.stride)*uint64(frame.height)))
		images[index] = &image.RGBA{
			Pix:    pixelBytes,
			Stride: int(frame.stride),
			Rect:   image.Rect(0, 0, int(frame.width), int(frame.height)),
		}
	}
	return composeLinuxPortalFrames(monitors, images)
}

// composeLinuxPortalFrames normalizes mixed-DPI streams onto one pixel scale so the editor keeps
// a single stable mapping between compositor coordinates and its desktop image.
func composeLinuxPortalFrames(monitors []linuxPortalMonitor, frames []*image.RGBA) (image.Image, error) {
	if len(monitors) == 0 || len(monitors) != len(frames) {
		return nil, errors.New("ScreenCast monitor and frame counts do not match")
	}
	bounds, err := linuxPortalMonitorUnion(monitors)
	if err != nil {
		return nil, err
	}
	scale := 1.0
	for index, monitor := range monitors {
		frame := frames[index]
		if frame == nil || frame.Bounds().Empty() || monitor.Bounds.Width <= 0 || monitor.Bounds.Height <= 0 {
			return nil, fmt.Errorf("ScreenCast monitor %q has invalid frame geometry", monitor.ID)
		}
		scale = max(scale, float64(frame.Bounds().Dx())/float64(monitor.Bounds.Width))
		scale = max(scale, float64(frame.Bounds().Dy())/float64(monitor.Bounds.Height))
	}
	width := int(math.Ceil(float64(bounds.Width) * scale))
	height := int(math.Ceil(float64(bounds.Height) * scale))
	if width <= 0 || height <= 0 {
		return nil, errors.New("ScreenCast composed desktop is empty")
	}
	output := image.NewRGBA(image.Rect(0, 0, width, height))
	stddraw.Draw(output, output.Bounds(), image.NewUniform(color.Black), image.Point{}, stddraw.Src)
	for index, monitor := range monitors {
		left := int(math.Round(float64(monitor.Bounds.X-bounds.X) * scale))
		top := int(math.Round(float64(monitor.Bounds.Y-bounds.Y) * scale))
		right := int(math.Round(float64(monitor.Bounds.X+monitor.Bounds.Width-bounds.X) * scale))
		bottom := int(math.Round(float64(monitor.Bounds.Y+monitor.Bounds.Height-bounds.Y) * scale))
		destination := image.Rect(left, top, right, bottom).Intersect(output.Bounds())
		if destination.Empty() {
			continue
		}
		xdraw.CatmullRom.Scale(output, destination, frames[index], frames[index].Bounds(), stddraw.Src, nil)
	}
	return output, nil
}

func closeLinuxScreenshotPortalSession(conn *dbus.Conn, sessionPath dbus.ObjectPath) {
	if conn == nil || sessionPath == "" {
		return
	}
	_ = conn.Object(linuxPortalBusName, sessionPath).Call(linuxPortalSessionInterface+".Close", 0).Err
}

func nextLinuxScreenshotPortalToken(prefix string) string {
	return fmt.Sprintf("wox_screenshot_%s_%d_%d", prefix, os.Getpid(), linuxPortalTokenCounter.Add(1))
}
