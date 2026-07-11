//go:build linux && cgo

package keyboard

import (
	"fmt"
	"reflect"
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
	portalRegistryIFace           = "org.freedesktop.host.portal.Registry"
)

type waylandHotkeyRegistration struct {
	shortcutID string
	callback   func()
}

type waylandHotkeySessionRegistration struct {
	conn         *dbus.Conn
	sessionPath  dbus.ObjectPath
	shortcutIDs  []string
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
	waylandPortalRegistrations = map[dbus.ObjectPath]map[string]*waylandHotkeyRegistration{}
	waylandPortalCounter       uint64

	// waylandPortalUnavailable is set to true the first time ensureWaylandPortalReady
	// determines that the XDG GlobalShortcuts portal is not available on this system.
	// Subsequent calls short-circuit immediately instead of repeating the probe.
	waylandPortalUnavailable bool
)

func registerGlobalHotkeyLinuxWayland(modifiers Modifier, key Key, callback func()) (HotkeyRegistration, error) {
	return registerGlobalHotkeysLinuxWayland([]GlobalHotkeySpec{
		{
			Modifiers: modifiers,
			Key:       key,
			Callback:  callback,
		},
	})
}

func registerGlobalHotkeysLinuxWayland(specs []GlobalHotkeySpec) (HotkeyRegistration, error) {
	if len(specs) == 0 {
		return &waylandHotkeySessionRegistration{}, nil
	}

	shortcutSpecs := make([]portalShortcutSpec, 0, len(specs))
	registrations := make(map[string]*waylandHotkeyRegistration, len(specs))
	preferredTriggers := make(map[string]string, len(specs))

	for _, spec := range specs {
		if spec.Callback == nil {
			return nil, fmt.Errorf("hotkey callback is required")
		}

		preferredTrigger, err := formatWaylandPreferredTrigger(spec.Modifiers, spec.Key)
		if err != nil {
			return nil, err
		}
		shortcutID := portalShortcutIDForTrigger(preferredTrigger)
		if _, exists := registrations[shortcutID]; exists {
			return nil, fmt.Errorf("duplicate wayland global hotkey shortcut id: %s", shortcutID)
		}

		registrations[shortcutID] = &waylandHotkeyRegistration{
			shortcutID: shortcutID,
			callback:   spec.Callback,
		}
		preferredTriggers[shortcutID] = preferredTrigger
		// preferred_trigger is the standard XDG portal option for suggesting a
		// trigger string. Some backends (notably xdg-desktop-portal-hyprland)
		// do not implement it and log "unknown shortcut data type preferred_trigger".
		// Including it is harmless on backends that understand it, and is ignored
		// (with a warning) on those that do not. Hyprland users bind shortcuts
		// manually in hyprland.conf using the "global" keyword.
		shortcutSpecs = append(shortcutSpecs, portalShortcutSpec{
			ID: shortcutID,
			Options: map[string]dbus.Variant{
				"description":       dbus.MakeVariant("Wox global hotkey"),
				"preferred_trigger": dbus.MakeVariant(preferredTrigger),
			},
		})
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

	bindRequestHandle, err := bindWaylandShortcuts(conn, sessionPath, shortcutSpecs)
	if err != nil {
		_ = closeWaylandPortalSession(conn, sessionPath)
		return nil, err
	}

	responseCode, bindResults, err := waitPortalRequestResponse(conn, bindRequestHandle)
	if err != nil {
		_ = closeWaylandPortalSession(conn, sessionPath)
		return nil, err
	}
	if responseCode != 0 {
		_ = closeWaylandPortalSession(conn, sessionPath)
		return nil, fmt.Errorf("wayland global hotkey bind request failed with response code %d", responseCode)
	}

	for shortcutID := range registrations {
		triggerDescription, ok := portalResponseShortcutTriggerDescription(bindResults, shortcutID)
		if !ok {
			_ = closeWaylandPortalSession(conn, sessionPath)
			return nil, fmt.Errorf("wayland global hotkey was not bound by portal")
		}
		// Some portal backends (e.g. xdg-desktop-portal-hyprland) accept the
		// shortcut and register it successfully but do not populate
		// trigger_description in the bind response. An empty description is not
		// an error — the shortcut is still active and the compositor will fire
		// Activated signals for it. Only warn so the log is debuggable.
		if strings.TrimSpace(triggerDescription) == "" {
			util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf(
				"[hotkey] wayland portal bind returned empty trigger_description for shortcut=%s (portal may still have registered it)", shortcutID))
		}
	}

	waylandPortalMu.Lock()
	waylandPortalRegistrations[sessionPath] = registrations
	waylandPortalMu.Unlock()

	shortcutIDs := make([]string, 0, len(registrations))
	for shortcutID := range registrations {
		shortcutIDs = append(shortcutIDs, shortcutID)
		triggerDescription, _ := portalResponseShortcutTriggerDescription(bindResults, shortcutID)
		preferredTrigger := preferredTriggers[shortcutID]
		util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf(
			"[hotkey] wayland portal registered: session=%s shortcut=%s trigger=%s trigger_description=%s",
			sessionPath, shortcutID, preferredTrigger, triggerDescription))
	}

	return &waylandHotkeySessionRegistration{
		conn:        conn,
		sessionPath: sessionPath,
		shortcutIDs: shortcutIDs,
	}, nil
}

func (r *waylandHotkeySessionRegistration) Unregister() error {
	if r == nil || r.sessionPath == "" {
		return nil
	}

	var unregisterErr error
	r.unregisterMu.Do(func() {
		waylandPortalMu.Lock()
		delete(waylandPortalRegistrations, r.sessionPath)
		waylandPortalMu.Unlock()
		unregisterErr = closeWaylandPortalSession(r.conn, r.sessionPath)
	})
	return unregisterErr
}

func closeWaylandPortalSession(conn *dbus.Conn, sessionPath dbus.ObjectPath) error {
	if conn == nil || sessionPath == "" {
		return nil
	}
	call := conn.Object(portalBusName, sessionPath).Call(portalSessionIFace+".Close", 0)
	return call.Err
}

func addRawKeyListenerLinuxWayland(handler RawKeyHandler) (RawKeySubscription, error) {
	if handler == nil {
		return nil, fmt.Errorf("raw key handler is required")
	}
	return nil, unsupportedWaylandRawListenerError()
}

func isWaylandGlobalShortcutsPortalAvailableLinux() bool {
	if !IsWaylandSession() {
		return false
	}
	if _, err := ensureWaylandPortalReady(); err != nil {
		util.GetLogger().Debug(util.NewTraceContext(), fmt.Sprintf(
			"[hotkey] wayland global shortcuts portal is not available: %s", err.Error()))
		return false
	}
	return true
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

	// Register the well-known bus name matching Wox's desktop entry so that
	// portal backends (e.g. xdg-desktop-portal-hyprland) can identify the caller.
	// Without this, CreateSession fails with "An app id is required" because the
	// portal maps the D-Bus sender name to the application identity.
	reply, err := conn.RequestName(util.LinuxDesktopAppID, dbus.NameFlagDoNotQueue)
	if err != nil {
		util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("[hotkey] failed to request well-known bus name %s: %v", util.LinuxDesktopAppID, err))
	} else if reply != 1 && reply != 4 {
		util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("[hotkey] well-known bus name %s not acquired (reply=%d); portal app-id resolution may fail", util.LinuxDesktopAppID, reply))
	}

	// xdg-desktop-portal >= 1.22 requires applications to register their app id
	// via the org.freedesktop.host.portal.Registry interface before using portal
	// features like GlobalShortcuts. Without this, CreateSession fails with
	// "An app id is required". The Register call is best-effort: older portal
	// versions that do not implement the Registry interface will return an error
	// which we safely ignore.
	registerCall := conn.Object(portalBusName, portalObjectPath).Call(portalRegistryIFace+".Register", 0, util.LinuxDesktopAppID, map[string]dbus.Variant{})
	if registerCall.Err != nil {
		util.GetLogger().Debug(util.NewTraceContext(), fmt.Sprintf("[hotkey] portal Registry.Register failed (may be unsupported on older portal versions): %v", registerCall.Err))
	} else {
		util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf("[hotkey] portal Registry.Register succeeded for app_id=%s", util.LinuxDesktopAppID))
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
			util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf(
				"[hotkey] wayland portal activation had invalid session handle: value=%#v", signal.Body[0]))
			continue
		}

		shortcutID, ok := signal.Body[1].(string)
		if !ok {
			continue
		}

		waylandPortalMu.Lock()
		sessionRegistrations := waylandPortalRegistrations[sessionPath]
		registration := sessionRegistrations[shortcutID]
		waylandPortalMu.Unlock()
		if registration == nil || registration.callback == nil {
			util.GetLogger().Debug(util.NewTraceContext(), fmt.Sprintf(
				"[hotkey] wayland portal activation ignored: session=%s shortcut=%s registered=%t",
				sessionPath, shortcutID, registration != nil))
			continue
		}

		util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf(
			"[hotkey] wayland portal activated: session=%s shortcut=%s", sessionPath, shortcutID))
		util.Go(util.NewTraceContext(), "wayland global hotkey callback", func() {
			registration.callback()
		})
	}
}

// portalShortcutIDForTrigger keeps the shortcut id stable across restarts so
// portal permission stores can associate the same configured shortcut with Wox.
func portalShortcutIDForTrigger(preferredTrigger string) string {
	var builder strings.Builder
	builder.WriteString("wox_shortcut_")
	for _, r := range preferredTrigger {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			continue
		}
		builder.WriteByte('_')
	}
	return builder.String()
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

func bindWaylandShortcuts(conn *dbus.Conn, sessionPath dbus.ObjectPath, shortcuts []portalShortcutSpec) (dbus.ObjectPath, error) {
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

// portalResponseShortcutTriggerDescription validates the portal's accepted
// shortcut list so a confirmed dialog with no actual trigger does not look like success.
func portalResponseShortcutTriggerDescription(results map[string]dbus.Variant, shortcutID string) (string, bool) {
	shortcutsVariant, ok := results["shortcuts"]
	if !ok {
		util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf(
			"[hotkey] wayland portal bind response did not include shortcuts: shortcut=%s response=%#v",
			shortcutID, results))
		return "", false
	}

	shortcutsValue := shortcutsVariant.Value()
	triggerDescription, ok := shortcutResultTriggerDescription(reflect.ValueOf(shortcutsValue), shortcutID)
	if !ok {
		util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf(
			"[hotkey] wayland portal bind response did not include shortcut: shortcut=%s response=%#v",
			shortcutID, shortcutsValue))
		return "", false
	}
	if strings.TrimSpace(triggerDescription) == "" {
		util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf(
			"[hotkey] wayland portal bind response included shortcut without trigger description: shortcut=%s response=%#v",
			shortcutID, shortcutsValue))
	}
	return triggerDescription, true
}

// shortcutResultTriggerDescription inspects the godbus-decoded a(sa{sv}) tuples
// and returns the user-readable trigger assigned by the portal for our shortcut.
func shortcutResultTriggerDescription(value reflect.Value, shortcutID string) (string, bool) {
	if !value.IsValid() {
		return "", false
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return "", false
		}
		value = value.Elem()
	}

	if value.Kind() != reflect.Slice && value.Kind() != reflect.Array {
		return "", false
	}

	for i := 0; i < value.Len(); i++ {
		item := value.Index(i)
		skipItem := false
		for item.Kind() == reflect.Interface || item.Kind() == reflect.Pointer {
			if item.IsNil() {
				skipItem = true
				break
			}
			item = item.Elem()
		}
		if skipItem {
			continue
		}

		var id string
		var options reflect.Value
		switch item.Kind() {
		case reflect.Struct:
			if item.NumField() > 0 {
				id, _ = reflectStringValue(item.Field(0))
			}
			if item.NumField() > 1 {
				options = item.Field(1)
			}
		case reflect.Slice, reflect.Array:
			if item.Len() > 0 {
				id, _ = reflectStringValue(item.Index(0))
			}
			if item.Len() > 1 {
				options = item.Index(1)
			}
		}

		if id == shortcutID {
			triggerDescription, _ := reflectOptionStringValue(options, "trigger_description")
			return triggerDescription, true
		}
	}
	return "", false
}

func reflectStringValue(value reflect.Value) (string, bool) {
	if !value.IsValid() {
		return "", false
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return "", false
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.String {
		return "", false
	}
	return value.String(), true
}

func reflectOptionStringValue(value reflect.Value, key string) (string, bool) {
	if !value.IsValid() {
		return "", false
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return "", false
		}
		value = value.Elem()
	}

	if value.CanInterface() {
		if options, ok := value.Interface().(map[string]dbus.Variant); ok {
			optionValue, exists := options[key]
			if !exists {
				return "", false
			}
			if optionString, ok := optionValue.Value().(string); ok {
				return optionString, true
			}
			return "", false
		}
	}

	if value.Kind() != reflect.Map {
		return "", false
	}
	for _, mapKey := range value.MapKeys() {
		optionKey, ok := reflectStringValue(mapKey)
		if !ok || optionKey != key {
			continue
		}
		optionValue := value.MapIndex(mapKey)
		if optionValue.CanInterface() {
			if variant, ok := optionValue.Interface().(dbus.Variant); ok {
				if optionString, ok := variant.Value().(string); ok {
					return optionString, true
				}
				return "", false
			}
		}
		return reflectStringValue(optionValue)
	}
	return "", false
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

	sessionHandle, ok := sessionHandleVariant.Value().(string)
	if !ok {
		return "", fmt.Errorf("portal response returned an invalid session handle type")
	}
	if sessionHandle == "" {
		return "", fmt.Errorf("portal response returned an empty session handle")
	}
	return dbus.ObjectPath(sessionHandle), nil
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
	case KeyBackquote:
		return "grave", nil
	default:
		return "", fmt.Errorf("unsupported Wayland hotkey key: %d", key)
	}
}

func nextPortalToken(prefix string) string {
	return fmt.Sprintf("wox_%s_%d", prefix, atomic.AddUint64(&waylandPortalCounter, 1))
}
