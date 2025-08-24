package mediaplayer

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"wox/plugin"
	"wox/util"
)

var mediaRetriever = &LinuxRetriever{}

type LinuxRetriever struct {
	api plugin.API
}

func (l *LinuxRetriever) UpdateAPI(api plugin.API) {
	l.api = api
}

func (l *LinuxRetriever) GetPlatform() string {
	return util.PlatformLinux
}

func (l *LinuxRetriever) GetCurrentMedia(ctx context.Context) (*MediaInfo, error) {
	// Get list of MPRIS-enabled media players
	players, err := l.getMPRISPlayers(ctx)
	if err != nil {
		return nil, err
	}

	// Try to get media info from each player
	for _, player := range players {
		if mediaInfo, err := l.getMediaFromMPRIS(ctx, player); err == nil {
			return mediaInfo, nil
		}
	}

	return nil, errors.New("no media playing")
}

func (l *LinuxRetriever) IsMediaPlaying(ctx context.Context) bool {
	mediaInfo, err := l.GetCurrentMedia(ctx)
	return err == nil && mediaInfo != nil && mediaInfo.State == PlaybackStatePlaying
}

func (l *LinuxRetriever) TogglePlayPause(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "playerctl", "play-pause")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("playerctl toggle failed: %w", err)
	}
	return nil
}

func (l *LinuxRetriever) getMPRISPlayers(ctx context.Context) ([]string, error) {
	// Use dbus-send to list MPRIS players
	cmd := exec.CommandContext(ctx, "dbus-send", "--session", "--dest=org.freedesktop.DBus",
		"--type=method_call", "--print-reply", "/org/freedesktop/DBus",
		"org.freedesktop.DBus.ListNames")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list D-Bus names: %w", err)
	}

	var players []string
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "org.mpris.MediaPlayer2.") {
			// Extract the service name
			start := strings.Index(line, "org.mpris.MediaPlayer2.")
			if start != -1 {
				end := strings.Index(line[start:], "\"")
				if end != -1 {
					serviceName := line[start : start+end]
					players = append(players, serviceName)
				}
			}
		}
	}

	return players, nil
}

func (l *LinuxRetriever) getMediaFromMPRIS(ctx context.Context, playerService string) (*MediaInfo, error) {
	// Get playback status
	playbackStatus, err := l.getMPRISProperty(ctx, playerService, "PlaybackStatus")
	if err != nil {
		return nil, err
	}

	// Skip if not playing or paused
	if !strings.Contains(playbackStatus, "Playing") && !strings.Contains(playbackStatus, "Paused") {
		return nil, errors.New("not playing")
	}

	// Get metadata
	metadata, err := l.getMPRISProperty(ctx, playerService, "Metadata")
	if err != nil {
		return nil, err
	}

	// Get position
	position, _ := l.getMPRISProperty(ctx, playerService, "Position")

	// Parse metadata and create MediaInfo
	mediaInfo := &MediaInfo{
		AppName:     l.getAppNameFromService(playerService),
		AppBundleID: playerService,
	}

	// Parse playback status
	if strings.Contains(playbackStatus, "Playing") {
		mediaInfo.State = PlaybackStatePlaying
	} else if strings.Contains(playbackStatus, "Paused") {
		mediaInfo.State = PlaybackStatePaused
	} else {
		mediaInfo.State = PlaybackStateStopped
	}

	// Parse metadata (D-Bus format is complex, so we'll use simple string parsing)
	mediaInfo.Title = l.extractMetadataValue(metadata, "xesam:title")
	mediaInfo.Artist = l.extractMetadataValue(metadata, "xesam:artist")
	mediaInfo.Album = l.extractMetadataValue(metadata, "xesam:album")

	// Parse duration (in microseconds)
	if durationStr := l.extractMetadataValue(metadata, "mpris:length"); durationStr != "" {
		if duration, err := strconv.ParseInt(durationStr, 10, 64); err == nil {
			mediaInfo.Duration = duration / 1000000 // Convert to seconds
		}
	}

	// Parse position (in microseconds)
	if position != "" {
		if pos, err := strconv.ParseInt(strings.TrimSpace(position), 10, 64); err == nil {
			mediaInfo.Position = pos / 1000000 // Convert to seconds
		}
	}

	return mediaInfo, nil
}

func (l *LinuxRetriever) getMPRISProperty(ctx context.Context, playerService, property string) (string, error) {
	cmd := exec.CommandContext(ctx, "dbus-send", "--session", "--print-reply",
		"--dest="+playerService, "/org/mpris/MediaPlayer2",
		"org.freedesktop.DBus.Properties.Get",
		"string:org.mpris.MediaPlayer2.Player", "string:"+property)

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get property %s: %w", property, err)
	}

	return string(output), nil
}

func (l *LinuxRetriever) extractMetadataValue(metadata, key string) string {
	// Simple extraction from D-Bus output
	// Look for the key and extract the associated string value
	lines := strings.Split(metadata, "\n")

	for i, line := range lines {
		if strings.Contains(line, key) {
			// Look for string value in the next few lines
			for j := i; j < len(lines) && j < i+5; j++ {
				if strings.Contains(lines[j], "string") {
					// Extract string value
					start := strings.Index(lines[j], "\"")
					if start != -1 {
						end := strings.LastIndex(lines[j], "\"")
						if end > start {
							return lines[j][start+1 : end]
						}
					}
				}
			}
		}
	}

	return ""
}

func (l *LinuxRetriever) getAppNameFromService(serviceName string) string {
	// Extract app name from MPRIS service name
	// Format: org.mpris.MediaPlayer2.{appname}
	parts := strings.Split(serviceName, ".")
	if len(parts) >= 4 {
		appName := parts[3]
		// Handle special cases
		switch strings.ToLower(appName) {
		case "spotify":
			return "Spotify"
		case "vlc":
			return "VLC"
		case "rhythmbox":
			return "Rhythmbox"
		case "amarok":
			return "Amarok"
		case "clementine":
			return "Clementine"
		case "audacious":
			return "Audacious"
		case "mpd":
			return "MPD"
		default:
			// Capitalize first letter
			if len(appName) > 0 {
				return strings.ToUpper(appName[:1]) + appName[1:]
			}
			return appName
		}
	}

	return serviceName
}
