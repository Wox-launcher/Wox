package mediaplayer

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
	"wox/plugin"
	"wox/util"

	"github.com/godbus/dbus/v5"
)

var mediaRetriever = &LinuxRetriever{}

const (
	mprisBusNamePrefix      = "org.mpris.MediaPlayer2."
	mprisObjectPath         = dbus.ObjectPath("/org/mpris/MediaPlayer2")
	mprisRootInterface      = "org.mpris.MediaPlayer2"
	mprisPlayerInterface    = "org.mpris.MediaPlayer2.Player"
	dbusPropertiesInterface = "org.freedesktop.DBus.Properties"
	dbusListNamesMethod     = "org.freedesktop.DBus.ListNames"

	linuxMediaDBusTimeout = 700 * time.Millisecond
	mprisMicroseconds     = int64(time.Second / time.Microsecond)
)

type LinuxRetriever struct {
	api  plugin.API
	mu   sync.Mutex
	conn *dbus.Conn
}

func (l *LinuxRetriever) UpdateAPI(api plugin.API) {
	l.api = api
}

func (l *LinuxRetriever) GetPlatform() string {
	return util.PlatformLinux
}

// GetCurrentMedia reads the best available MPRIS player from the Linux session bus.
func (l *LinuxRetriever) GetCurrentMedia(ctx context.Context) (*MediaInfo, error) {
	conn, err := l.ensureSessionBus()
	if err != nil {
		return nil, err
	}

	playerNames, err := l.listMPRISPlayerNames(ctx, conn)
	if err != nil {
		l.resetSessionBus(conn)
		return nil, err
	}
	if len(playerNames) == 0 {
		return nil, nil
	}

	var fallback *MediaInfo
	for _, playerName := range playerNames {
		mediaInfo, err := l.getPlayerMediaInfo(ctx, conn, playerName)
		if err != nil || mediaInfo == nil {
			continue
		}
		if mediaInfo.State == PlaybackStatePlaying {
			return mediaInfo, nil
		}
		if fallback == nil {
			fallback = mediaInfo
		}
	}

	return fallback, nil
}

// ControlMedia sends playback commands to the active MPRIS player.
func (l *LinuxRetriever) ControlMedia(ctx context.Context, command string) error {
	method, ok := mapLinuxMediaControlMethod(command)
	if !ok {
		return fmt.Errorf("unsupported Linux media control command: %s", command)
	}

	conn, err := l.ensureSessionBus()
	if err != nil {
		return err
	}

	playerName, err := l.selectMPRISPlayerForControl(ctx, conn)
	if err != nil {
		l.resetSessionBus(conn)
		return err
	}
	if playerName == "" {
		return errors.New("no MPRIS media player is available")
	}

	callCtx, cancel := context.WithTimeout(ctx, linuxMediaDBusTimeout)
	defer cancel()

	obj := conn.Object(playerName, mprisObjectPath)
	call := obj.CallWithContext(callCtx, mprisPlayerInterface+"."+method, dbus.FlagNoAutoStart)
	if call.Err != nil {
		return fmt.Errorf("failed to run MPRIS command %s for %s: %w", command, playerName, call.Err)
	}
	return nil
}

func (l *LinuxRetriever) TogglePlayPause(ctx context.Context) error {
	return l.ControlMedia(ctx, mediaControlToggle)
}

// ensureSessionBus opens and reuses the session bus connection used by MPRIS players.
func (l *LinuxRetriever) ensureSessionBus() (*dbus.Conn, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.conn != nil {
		return l.conn, nil
	}

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Linux session bus for MPRIS: %w", err)
	}
	l.conn = conn
	return conn, nil
}

// resetSessionBus drops the cached connection after bus-level failures.
func (l *LinuxRetriever) resetSessionBus(conn *dbus.Conn) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.conn == conn {
		_ = l.conn.Close()
		l.conn = nil
	}
}

// listMPRISPlayerNames returns active well-known MPRIS names without auto-starting players.
func (l *LinuxRetriever) listMPRISPlayerNames(ctx context.Context, conn *dbus.Conn) ([]string, error) {
	callCtx, cancel := context.WithTimeout(ctx, linuxMediaDBusTimeout)
	defer cancel()

	var busNames []string
	if err := conn.BusObject().CallWithContext(callCtx, dbusListNamesMethod, dbus.FlagNoAutoStart).Store(&busNames); err != nil {
		return nil, fmt.Errorf("failed to list session bus names for MPRIS: %w", err)
	}

	playerNames := make([]string, 0)
	for _, busName := range busNames {
		if strings.HasPrefix(busName, mprisBusNamePrefix) {
			playerNames = append(playerNames, busName)
		}
	}
	sort.Strings(playerNames)
	return playerNames, nil
}

// getPlayerMediaInfo converts one MPRIS player's properties into Wox's shared media model.
func (l *LinuxRetriever) getPlayerMediaInfo(ctx context.Context, conn *dbus.Conn, playerName string) (*MediaInfo, error) {
	playerProperties, err := getMPRISProperties(ctx, conn, playerName, mprisPlayerInterface)
	if err != nil {
		return nil, err
	}

	metadata := mprisVariantMap(playerProperties["Metadata"])
	mediaInfo := &MediaInfo{
		Title:       mprisMetadataString(metadata, "xesam:title"),
		Artist:      strings.Join(mprisMetadataStringSlice(metadata, "xesam:artist"), ", "),
		Album:       mprisMetadataString(metadata, "xesam:album"),
		Duration:    mprisMetadataMicroseconds(metadata, "mpris:length"),
		Position:    mprisPropertyMicroseconds(playerProperties, "Position"),
		State:       parseMPRISPlaybackState(mprisPropertyString(playerProperties, "PlaybackStatus")),
		AppBundleID: playerName,
	}
	if mediaInfo.Title == "" {
		mediaInfo.Title = mprisMetadataString(metadata, "xesam:url")
	}

	rootProperties, err := getMPRISProperties(ctx, conn, playerName, mprisRootInterface)
	if err == nil {
		mediaInfo.AppName = mprisPropertyString(rootProperties, "Identity")
	}
	if mediaInfo.AppName == "" {
		mediaInfo.AppName = strings.TrimPrefix(playerName, mprisBusNamePrefix)
	}

	if artURL := mprisMetadataString(metadata, "mpris:artUrl"); artURL != "" {
		mediaInfo.Artwork = readMPRISArtwork(artURL)
	}

	// Some players expose an MPRIS name while stopped but keep metadata empty.
	// Treat that as no active media instead of showing a blank result.
	if mediaInfo.Title == "" && mediaInfo.Artist == "" && mediaInfo.Album == "" && mediaInfo.State == PlaybackStateStopped {
		return nil, nil
	}
	if mediaInfo.Title == "" {
		mediaInfo.Title = mediaInfo.AppName
	}

	return mediaInfo, nil
}

// selectMPRISPlayerForControl prefers the playing session but falls back to the first controllable player.
func (l *LinuxRetriever) selectMPRISPlayerForControl(ctx context.Context, conn *dbus.Conn) (string, error) {
	playerNames, err := l.listMPRISPlayerNames(ctx, conn)
	if err != nil {
		return "", err
	}

	fallback := ""
	for _, playerName := range playerNames {
		playerProperties, err := getMPRISProperties(ctx, conn, playerName, mprisPlayerInterface)
		if err != nil {
			continue
		}

		if fallback == "" {
			fallback = playerName
		}
		if parseMPRISPlaybackState(mprisPropertyString(playerProperties, "PlaybackStatus")) == PlaybackStatePlaying {
			return playerName, nil
		}
	}
	return fallback, nil
}

// getMPRISProperties reads all properties for one MPRIS interface in a bounded D-Bus call.
func getMPRISProperties(ctx context.Context, conn *dbus.Conn, playerName string, iface string) (map[string]dbus.Variant, error) {
	callCtx, cancel := context.WithTimeout(ctx, linuxMediaDBusTimeout)
	defer cancel()

	properties := make(map[string]dbus.Variant)
	obj := conn.Object(playerName, mprisObjectPath)
	if err := obj.CallWithContext(callCtx, dbusPropertiesInterface+".GetAll", dbus.FlagNoAutoStart, iface).Store(&properties); err != nil {
		return nil, fmt.Errorf("failed to read MPRIS properties for %s interface %s: %w", playerName, iface, err)
	}
	return properties, nil
}

// mapLinuxMediaControlMethod translates Wox command names to MPRIS player methods.
func mapLinuxMediaControlMethod(command string) (string, bool) {
	switch command {
	case mediaControlPlay:
		return "Play", true
	case mediaControlPause:
		return "Pause", true
	case mediaControlToggle:
		return "PlayPause", true
	case mediaControlNext:
		return "Next", true
	case mediaControlPrevious:
		return "Previous", true
	default:
		return "", false
	}
}

// parseMPRISPlaybackState maps MPRIS playback status strings to Wox playback states.
func parseMPRISPlaybackState(status string) PlaybackState {
	switch status {
	case "Playing":
		return PlaybackStatePlaying
	case "Paused":
		return PlaybackStatePaused
	case "Stopped":
		return PlaybackStateStopped
	default:
		return PlaybackStateUnknown
	}
}

// mprisVariantMap unwraps the nested variant map used by the MPRIS Metadata property.
func mprisVariantMap(variant dbus.Variant) map[string]dbus.Variant {
	values, ok := variant.Value().(map[string]dbus.Variant)
	if !ok {
		return nil
	}
	return values
}

// mprisPropertyString reads a string property from a GetAll result.
func mprisPropertyString(properties map[string]dbus.Variant, key string) string {
	if properties == nil {
		return ""
	}
	return mprisVariantString(properties[key])
}

// mprisMetadataString reads a string metadata value from the nested Metadata map.
func mprisMetadataString(metadata map[string]dbus.Variant, key string) string {
	if metadata == nil {
		return ""
	}
	return mprisVariantString(metadata[key])
}

// mprisVariantString returns the variant value only when it is a D-Bus string.
func mprisVariantString(variant dbus.Variant) string {
	value, ok := variant.Value().(string)
	if !ok {
		return ""
	}
	return value
}

// mprisMetadataStringSlice reads string-list metadata and tolerates string-only players.
func mprisMetadataStringSlice(metadata map[string]dbus.Variant, key string) []string {
	if metadata == nil {
		return nil
	}
	switch value := metadata[key].Value().(type) {
	case []string:
		return value
	case string:
		if value == "" {
			return nil
		}
		return []string{value}
	default:
		return nil
	}
}

// mprisPropertyMicroseconds reads a microsecond property and returns seconds for Wox UI fields.
func mprisPropertyMicroseconds(properties map[string]dbus.Variant, key string) int64 {
	if properties == nil {
		return 0
	}
	return mprisVariantMicroseconds(properties[key])
}

// mprisMetadataMicroseconds reads a microsecond metadata value and returns seconds.
func mprisMetadataMicroseconds(metadata map[string]dbus.Variant, key string) int64 {
	if metadata == nil {
		return 0
	}
	return mprisVariantMicroseconds(metadata[key])
}

// mprisVariantMicroseconds normalizes the integer widths returned by different MPRIS players.
func mprisVariantMicroseconds(variant dbus.Variant) int64 {
	switch value := variant.Value().(type) {
	case int64:
		return value / mprisMicroseconds
	case int32:
		return int64(value) / mprisMicroseconds
	case uint64:
		return int64(value / uint64(mprisMicroseconds))
	case uint32:
		return int64(value / uint32(mprisMicroseconds))
	case int:
		return int64(value) / mprisMicroseconds
	case uint:
		return int64(value / uint(mprisMicroseconds))
	default:
		return 0
	}
}

// readMPRISArtwork resolves local artwork URIs into raw bytes for the existing image pipeline.
func readMPRISArtwork(artURL string) []byte {
	if strings.HasPrefix(artURL, "data:") {
		return []byte(artURL)
	}

	parsedURL, err := url.Parse(artURL)
	if err != nil {
		return nil
	}

	if parsedURL.Scheme == "file" {
		data, err := os.ReadFile(parsedURL.Path)
		if err == nil {
			return data
		}
	}
	return nil
}
