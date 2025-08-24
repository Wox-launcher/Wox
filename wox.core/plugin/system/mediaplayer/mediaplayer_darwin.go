package mediaplayer

// Using Perl script approach to bypass MediaRemote restrictions

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path"
	"strings"
	"wox/plugin"
	"wox/util"
)

var mediaRetriever = &DarwinRetriever{}

type DarwinRetriever struct {
	api plugin.API
}

func (d *DarwinRetriever) UpdateAPI(api plugin.API) {
	d.api = api
}

func (d *DarwinRetriever) GetPlatform() string {
	return util.PlatformMacOS
}

func (d *DarwinRetriever) IsMediaPlaying(ctx context.Context) bool {
	mediaInfo, err := d.GetCurrentMedia(ctx)
	return err == nil && mediaInfo != nil && mediaInfo.State == PlaybackStatePlaying
}

func (d *DarwinRetriever) GetCurrentMedia(ctx context.Context) (*MediaInfo, error) {
	// Use Perl script to bypass MediaRemote restrictions
	// This leverages /usr/bin/perl's com.apple.perl bundle ID to access MediaRemote
	scriptPath := d.getScriptPath()

	// Execute the Perl script with com.apple.perl privileges
	cmd := exec.CommandContext(ctx, "/usr/bin/perl", scriptPath, "get")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute MediaRemote Perl script: %w", err)
	}

	// Parse JSON response
	var rawInfo map[string]interface{}
	if err := json.Unmarshal(output, &rawInfo); err != nil {
		return nil, fmt.Errorf("failed to parse MediaRemote response: %w", err)
	}

	isPlaying := false
	if v, ok := rawInfo["playing"]; ok {
		switch vv := v.(type) {
		case bool:
			isPlaying = vv
		case float64:
			isPlaying = vv > 0.5
		case string:
			lower := strings.ToLower(vv)
			isPlaying = lower == "true" || lower == "1" || lower == "yes" || lower == "playing"
		}
	}

	return d.parseMediaRemoteInfo(rawInfo, isPlaying)
}

func (d *DarwinRetriever) parseMediaRemoteInfo(rawInfo map[string]interface{}, isPlaying bool) (*MediaInfo, error) {
	mediaInfo := &MediaInfo{}

	// Extract basic media information from MediaRemote response
	// Our Objective-C code returns simplified JSON keys
	if title, ok := rawInfo["title"]; ok {
		if titleStr, ok := title.(string); ok {
			mediaInfo.Title = titleStr
		}
	}

	if artist, ok := rawInfo["artist"]; ok {
		if artistStr, ok := artist.(string); ok {
			mediaInfo.Artist = artistStr
		}
	}

	if album, ok := rawInfo["album"]; ok {
		if albumStr, ok := album.(string); ok {
			mediaInfo.Album = albumStr
		}
	}

	if duration, ok := rawInfo["duration"]; ok {
		if durationFloat, ok := duration.(float64); ok {
			mediaInfo.Duration = int64(durationFloat)
		}
	}

	if position, ok := rawInfo["position"]; ok {
		if positionFloat, ok := position.(float64); ok {
			mediaInfo.Position = int64(positionFloat)
		}
	}

	// Set playback state
	if isPlaying {
		mediaInfo.State = PlaybackStatePlaying
	} else {
		mediaInfo.State = PlaybackStatePaused
	}

	// Get app information from JSON response
	if appName, ok := rawInfo["appName"]; ok {
		if appNameStr, ok := appName.(string); ok {
			mediaInfo.AppName = appNameStr
		}
	}

	if bundleID, ok := rawInfo["bundleIdentifier"]; ok {
		if bundleIDStr, ok := bundleID.(string); ok {
			mediaInfo.AppBundleID = bundleIDStr
		}
	}

	// Artwork base64 string
	if artwork, ok := rawInfo["artwork"]; ok {
		if artStr, ok := artwork.(string); ok {
			mediaInfo.Artwork = []byte(artStr)
		}
	}

	// Set defaults if not found
	if mediaInfo.AppName == "" {
		mediaInfo.AppName = "Unknown Media App"
	}
	if mediaInfo.AppBundleID == "" {
		mediaInfo.AppBundleID = "unknown.media.app"
	}

	return mediaInfo, nil
}

func (d *DarwinRetriever) getScriptPath() string {
	// Return the path to the MediaRemote adapter Perl script in woxmr install dir
	return path.Join(util.GetLocation().GetOthersDirectory(), "woxmr", "adapter.pl")
}

func (d *DarwinRetriever) TogglePlayPause(ctx context.Context) error {
	scriptPath := d.getScriptPath()
	cmd := exec.CommandContext(ctx, "/usr/bin/perl", scriptPath, "toggle")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to toggle via adapter: %w", err)
	}
	return nil
}
