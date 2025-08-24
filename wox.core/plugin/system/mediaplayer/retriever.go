package mediaplayer

import (
	"context"
	"wox/plugin"
)

// PlaybackState represents the current playback state
type PlaybackState int

const (
	PlaybackStateStopped PlaybackState = iota
	PlaybackStatePlaying
	PlaybackStatePaused
	PlaybackStateUnknown
)

func (s PlaybackState) String() string {
	switch s {
	case PlaybackStateStopped:
		return "stopped"
	case PlaybackStatePlaying:
		return "playing"
	case PlaybackStatePaused:
		return "paused"
	default:
		return "unknown"
	}
}

// MediaInfo contains information about currently playing media
type MediaInfo struct {
	Title       string        `json:"title"`
	Artist      string        `json:"artist"`
	Album       string        `json:"album"`
	Duration    int64         `json:"duration"` // Duration in seconds
	Position    int64         `json:"position"` // Current position in seconds
	State       PlaybackState `json:"state"`
	AppName     string        `json:"appName"`     // Name of the media application
	AppBundleID string        `json:"appBundleId"` // Bundle ID or process name
	Artwork     []byte        `json:"artwork"`     // Album artwork as image data
}

// MediaRetriever defines the interface for retrieving media information across platforms
type MediaRetriever interface {
	// UpdateAPI updates the plugin API reference
	UpdateAPI(api plugin.API)

	// GetPlatform returns the platform name
	GetPlatform() string

	// GetCurrentMedia retrieves current media information
	GetCurrentMedia(ctx context.Context) (*MediaInfo, error)

	// IsMediaPlaying checks if any media is currently playing
	IsMediaPlaying(ctx context.Context) bool

	// TogglePlayPause toggles playback state if supported on the platform/app
	TogglePlayPause(ctx context.Context) error
}
