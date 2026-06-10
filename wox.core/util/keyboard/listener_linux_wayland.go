//go:build linux && cgo

package keyboard

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"wox/util"

	"github.com/godbus/dbus/v5"
)

const (
	portalBusName                 = "org.freedesktop.portal.Desktop"
	portalObjectPath              = dbus.ObjectPath("/org/freedesktop/portal/desktop")
	portalGlobalShortcutsIFace    = "org.freedesktop.portal.GlobalShortcuts"
	portalRequestIFace            = "org.freedesktop.portal.Request"
	portalSessionIFace            = "org.freedesktop.portal.Session"
	portalRequestResponseSignal   = portalRequestIFace + ".Response"
	portalShortcutActivatedSignal = portalGlobalShortcutsIFace + ".Activated"
)

type waylandHotkeyRegistration struct {
	conn         *dbus.Conn
	sessionPath  dbus.ObjectPath
	shortcutID   string
	callback     func()
	unregisterMu sync.Once
}

type portalShortcutSpec struct {
	ID      string
	Options map[string]dbus.Variant
}

var (
	waylandPortalMu            sync.Mutex
	waylandPortalConn          *dbus.Conn
	waylandPortalSignals       chan *dbus.Signal
	waylandPortalRegistrations = map[dbus.ObjectPath]*waylandHotkeyRegistration{}
	waylandPortalCounter       uint64

	// waylandPortalUnavailable is set to true the first time ensureWaylandPortalReady
	// determines that the XDG GlobalShortcuts portal is not available on this system
	// (e.g. xdg-desktop-portal-gnome < 47 which lacks the implementation).
	// Subsequent calls short-circuit immediately instead of repeating the probe.
	waylandPortalUnavailable bool
)

func registerGlobalHotkeyLinuxWayland(modifiers Modifier, key Key, callback func()) (HotkeyRegistration, error) {
	if callback == nil {
		return nil, fmt.Errorf("hotkey callback is required")
	}

	preferredTrigger, err := formatWaylandPreferredTrigger(modifiers, key)
	if err != nil {
		return nil, err
	}

	conn, err := ensureWaylandPortalReady()
	if err != nil {
		return nil, err
	}

	requestToken := nextPortalToken("request")
	sessionToken := nextPortalToken("session")
	requestHandle, err := createWaylandGlobalShortcutsSession(conn, requestToken, sessionToken)
	if err != nil {
		return nil, err
	}

	responseCode, results, err := waitPortalRequestResponse(conn, requestHandle)
	if err != nil {
		return nil, err
	}
	if responseCode != 0 {
		return nil, fmt.Errorf("wayland global hotkey session request failed with response code %d", responseCode)
	}

	sessionPath, err := parsePortalSessionHandle(results)
	if err != nil {
		return nil, err
	}

	registration := &waylandHotkeyRegistration{
		conn:        conn,
		sessionPath: sessionPath,
		shortcutID:  nextPortalToken("shortcut"),
		callback:    callback,
	}

	bindRequestHandle, err := bindWaylandShortcut(conn, registration.sessionPath, registration.shortcutID, preferredTrigger)
	if err != nil {
		_ = registration.closeSession()
		return nil, err
	}

	responseCode, _, err = waitPortalRequestResponse(conn, bindRequestHandle)
	if err != nil {
		_ = registration.closeSession()
		return nil, err
	}
	if responseCode != 0 {
		_ = registration.closeSession()
		return nil, fmt.Errorf("wayland global hotkey bind request failed with response code %d", responseCode)
	}

	waylandPortalMu.Lock()
	waylandPortalRegistrations[registration.sessionPath] = registration
	waylandPortalMu.Unlock()

	return registration, nil
}

func (r *waylandHotkeyRegistration) Unregister() error {
	if r == nil {
		return nil
	}

	var unregisterErr error
	r.unregisterMu.Do(func() {
		waylandPortalMu.Lock()
		delete(waylandPortalRegistrations, r.sessionPath)
		waylandPortalMu.Unlock()
		unregisterErr = r.closeSession()
	})
	return unregisterErr
}

func (r *waylandHotkeyRegistration) closeSession() error {
	call := r.conn.Object(portalBusName, r.sessionPath).Call(portalSessionIFace+".Close", 0)
	return call.Err
}

func addRawKeyListenerLinuxWayland(handler RawKeyHandler) (RawKeySubscription, error) {
	if handler == nil {
		return nil, fmt.Errorf("raw key handler is required")
	}
	return nil, unsupportedWaylandRawListenerError()
}

// IsWaylandPortalAvailable returns true when the XDG GlobalShortcuts portal
// has been successfully initialised at least once. It is safe to call from
// any goroutine.
func IsWaylandPortalAvailable() bool {
	waylandPortalMu.Lock()
	defer waylandPortalMu.Unlock()
	return waylandPortalConn != nil
}

func ensureWaylandPortalReady() (*dbus.Conn, error) {
	waylandPortalMu.Lock()
	defer waylandPortalMu.Unlock()

	if waylandPortalConn != nil {
		return waylandPortalConn, nil
	}

	// If we already probed and the portal was not available, skip the expensive
	// D-Bus round-trip and return the cached error immediately.
	if waylandPortalUnavailable {
		return nil, fmt.Errorf("wayland global shortcuts portal is not available on this system")
	}

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to session bus: %w", err)
	}

	portalObject := conn.Object(portalBusName, portalObjectPath)
	versionVariant, err := portalObject.GetProperty(portalGlobalShortcutsIFace + ".version")
	if err != nil {
		_ = conn.Close()
		// Mark the portal as permanently unavailable so we do not re-probe.
		waylandPortalUnavailable = true
		return nil, fmt.Errorf("wayland global shortcuts portal is not available: %w", err)
	}

	version, ok := versionVariant.Value().(uint32)
	if !ok || version == 0 {
		_ = conn.Close()
		return nil, fmt.Errorf("wayland global shortcuts portal returned an invalid version")
	}

	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath(portalObjectPath),
		dbus.WithMatchInterface(portalGlobalShortcutsIFace),
		dbus.WithMatchMember("Activated"),
	); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to subscribe to wayland global shortcut activation signals: %w", err)
	}

	signals := make(chan *dbus.Signal, 16)
	conn.Signal(signals)

	waylandPortalConn = conn
	waylandPortalSignals = signals
	go processWaylandPortalSignals(signals)

	return conn, nil
}

func processWaylandPortalSignals(signals <-chan *dbus.Signal) {
	for signal := range signals {
		if signal == nil || signal.Name != portalShortcutActivatedSignal || len(signal.Body) < 2 {
			continue
		}

		sessionPath, ok := signal.Body[0].(dbus.ObjectPath)
		if !ok {
			continue
		}

		shortcutID, ok := signal.Body[1].(string)
		if !ok {
			continue
		}

		waylandPortalMu.Lock()
		registration := waylandPortalRegistrations[sessionPath]
		waylandPortalMu.Unlock()
		if registration == nil || registration.shortcutID != shortcutID || registration.callback == nil {
			continue
		}

		util.Go(util.NewTraceContext(), "wayland global hotkey callback", func() {
			registration.callback()
		})
	}
}

func createWaylandGlobalShortcutsSession(conn *dbus.Conn, handleToken string, sessionToken string) (dbus.ObjectPath, error) {
	options := map[string]dbus.Variant{
		"handle_token":         dbus.MakeVariant(handleToken),
		"session_handle_token": dbus.MakeVariant(sessionToken),
	}

	var requestHandle dbus.ObjectPath
	call := conn.Object(portalBusName, portalObjectPath).Call(
		portalGlobalShortcutsIFace+".CreateSession",
		0,
		options,
	)
	if call.Err != nil {
		return "", fmt.Errorf("failed to create wayland global shortcuts session: %w", call.Err)
	}
	if err := call.Store(&requestHandle); err != nil {
		return "", fmt.Errorf("failed to decode wayland global shortcuts session request handle: %w", err)
	}
	return requestHandle, nil
}

func bindWaylandShortcut(conn *dbus.Conn, sessionPath dbus.ObjectPath, shortcutID string, preferredTrigger string) (dbus.ObjectPath, error) {
	shortcuts := []portalShortcutSpec{
		{
			ID: shortcutID,
			Options: map[string]dbus.Variant{
				"description":       dbus.MakeVariant("Wox global hotkey"),
				"preferred_trigger": dbus.MakeVariant(preferredTrigger),
			},
		},
	}

	options := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(nextPortalToken("bind")),
	}

	var requestHandle dbus.ObjectPath
	call := conn.Object(portalBusName, portalObjectPath).Call(
		portalGlobalShortcutsIFace+".BindShortcuts",
		0,
		sessionPath,
		shortcuts,
		"",
		options,
	)
	if call.Err != nil {
		return "", fmt.Errorf("failed to bind wayland global hotkey: %w", call.Err)
	}
	if err := call.Store(&requestHandle); err != nil {
		return "", fmt.Errorf("failed to decode wayland global hotkey bind request handle: %w", err)
	}
	return requestHandle, nil
}

func waitPortalRequestResponse(conn *dbus.Conn, requestHandle dbus.ObjectPath) (uint32, map[string]dbus.Variant, error) {
	signals := make(chan *dbus.Signal, 1)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)

	matchOptions := []dbus.MatchOption{
		dbus.WithMatchObjectPath(requestHandle),
		dbus.WithMatchInterface(portalRequestIFace),
		dbus.WithMatchMember("Response"),
	}
	if err := conn.AddMatchSignal(matchOptions...); err != nil {
		return 0, nil, fmt.Errorf("failed to subscribe to portal request response: %w", err)
	}
	defer func() {
		_ = conn.RemoveMatchSignal(matchOptions...)
	}()

	timeout := time.NewTimer(30 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case signal := <-signals:
			if signal == nil || signal.Name != portalRequestResponseSignal || len(signal.Body) != 2 {
				continue
			}

			responseCode, ok := signal.Body[0].(uint32)
			if !ok {
				return 0, nil, fmt.Errorf("portal request response had an invalid response code")
			}

			results, ok := signal.Body[1].(map[string]dbus.Variant)
			if !ok {
				return 0, nil, fmt.Errorf("portal request response had invalid result payload")
			}

			return responseCode, results, nil
		case <-timeout.C:
			return 0, nil, fmt.Errorf("timed out waiting for portal request response")
		}
	}
}

func parsePortalSessionHandle(results map[string]dbus.Variant) (dbus.ObjectPath, error) {
	sessionHandleVariant, ok := results["session_handle"]
	if !ok {
		return "", fmt.Errorf("portal response did not include a session handle")
	}

	switch sessionHandle := sessionHandleVariant.Value().(type) {
	case dbus.ObjectPath:
		if sessionHandle == "" {
			return "", fmt.Errorf("portal response returned an empty session handle")
		}
		return sessionHandle, nil
	case string:
		if sessionHandle == "" {
			return "", fmt.Errorf("portal response returned an empty session handle")
		}
		return dbus.ObjectPath(sessionHandle), nil
	default:
		return "", fmt.Errorf("portal response returned an invalid session handle type")
	}
}

func formatWaylandPreferredTrigger(modifiers Modifier, key Key) (string, error) {
	keyName, err := keyToWaylandTriggerName(key)
	if err != nil {
		return "", err
	}

	parts := make([]string, 0, 5)
	if modifiers&ModifierCtrl != 0 {
		parts = append(parts, "CTRL")
	}
	if modifiers&ModifierAlt != 0 {
		parts = append(parts, "ALT")
	}
	if modifiers&ModifierShift != 0 {
		parts = append(parts, "SHIFT")
	}
	if modifiers&ModifierSuper != 0 {
		parts = append(parts, "LOGO")
	}
	parts = append(parts, keyName)
	return strings.Join(parts, "+"), nil
}

func keyToWaylandTriggerName(key Key) (string, error) {
	switch key {
	case KeyA:
		return "a", nil
	case KeyB:
		return "b", nil
	case KeyC:
		return "c", nil
	case KeyD:
		return "d", nil
	case KeyE:
		return "e", nil
	case KeyF:
		return "f", nil
	case KeyG:
		return "g", nil
	case KeyH:
		return "h", nil
	case KeyI:
		return "i", nil
	case KeyJ:
		return "j", nil
	case KeyK:
		return "k", nil
	case KeyL:
		return "l", nil
	case KeyM:
		return "m", nil
	case KeyN:
		return "n", nil
	case KeyO:
		return "o", nil
	case KeyP:
		return "p", nil
	case KeyQ:
		return "q", nil
	case KeyR:
		return "r", nil
	case KeyS:
		return "s", nil
	case KeyT:
		return "t", nil
	case KeyU:
		return "u", nil
	case KeyV:
		return "v", nil
	case KeyW:
		return "w", nil
	case KeyX:
		return "x", nil
	case KeyY:
		return "y", nil
	case KeyZ:
		return "z", nil
	case Key0:
		return "0", nil
	case Key1:
		return "1", nil
	case Key2:
		return "2", nil
	case Key3:
		return "3", nil
	case Key4:
		return "4", nil
	case Key5:
		return "5", nil
	case Key6:
		return "6", nil
	case Key7:
		return "7", nil
	case Key8:
		return "8", nil
	case Key9:
		return "9", nil
	case KeySpace:
		return "space", nil
	case KeyReturn:
		return "Return", nil
	case KeyEscape:
		return "Escape", nil
	case KeyTab:
		return "Tab", nil
	case KeyDelete:
		return "Delete", nil
	case KeyLeft:
		return "Left", nil
	case KeyRight:
		return "Right", nil
	case KeyUp:
		return "Up", nil
	case KeyDown:
		return "Down", nil
	case KeyF1:
		return "F1", nil
	case KeyF2:
		return "F2", nil
	case KeyF3:
		return "F3", nil
	case KeyF4:
		return "F4", nil
	case KeyF5:
		return "F5", nil
	case KeyF6:
		return "F6", nil
	case KeyF7:
		return "F7", nil
	case KeyF8:
		return "F8", nil
	case KeyF9:
		return "F9", nil
	case KeyF10:
		return "F10", nil
	case KeyF11:
		return "F11", nil
	case KeyF12:
		return "F12", nil
	case KeyCapsLock:
		return "Caps_Lock", nil
	default:
		return "", fmt.Errorf("unsupported Wayland hotkey key: %d", key)
	}
}

func nextPortalToken(prefix string) string {
	return fmt.Sprintf("wox_%s_%d", prefix, atomic.AddUint64(&waylandPortalCounter, 1))
}
