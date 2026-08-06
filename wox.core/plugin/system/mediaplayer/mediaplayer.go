package mediaplayer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"strings"
	"time"
	"wox/common"
	"wox/plugin"
	"wox/util"

	_ "image/jpeg"

	"github.com/google/uuid"
	xdraw "golang.org/x/image/draw"
)

var mediaIcon = common.PluginMediaPlayerIcon

const (
	// PluginID identifies the built-in media player plugin for internal plugin commands.
	PluginID = "b8f3d4e5-6c7a-4b9c-8d1e-2f3a4b5c6d7e"

	PluginCommandPauseIfPlaying      = "pause_if_playing"
	PluginCommandResultPaused        = "paused"
	PluginCommandResultNotPlaying    = "not_playing"
	PluginCommandResultNoActiveMedia = "no_active_media"
	PluginCommandPlay                = "play"

	mediaControlPlay     = "play"
	mediaControlPause    = "pause"
	mediaControlToggle   = "toggle"
	mediaControlNext     = "next"
	mediaControlPrevious = "previous"

	mediaControlGlobalResultScore int64 = 200
	recordArtworkSize                   = 96
)

type mediaControlAction struct {
	command string
	aliases []string
}

var mediaControlActions = []mediaControlAction{
	{command: mediaControlPlay},
	{command: mediaControlPause},
	{command: mediaControlToggle, aliases: []string{"playpause", "play/pause"}},
	{command: mediaControlNext},
	{command: mediaControlPrevious, aliases: []string{"prev", "back"}},
}

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &MediaPlayerPlugin{})
}

type MediaPlayerPlugin struct {
	api             plugin.API
	pluginDirectory string
	retriever       MediaRetriever

	// Track results that need periodic refresh
	trackedResults *util.HashMap[string, mediaTrackedResult]
}

type mediaContextData struct {
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Album       string `json:"album"`
	AppName     string `json:"appName"`
	AppBundleID string `json:"appBundleId"`
}

// mediaPreviewData is the internal payload for UI's dedicated now-playing surface.
type mediaPreviewData struct {
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	Album     string `json:"album"`
	AppName   string `json:"appName"`
	Artwork   string `json:"artwork"`
	Position  int64  `json:"position"`
	Duration  int64  `json:"duration"`
	IsPlaying bool   `json:"isPlaying"`
}

type mediaTrackedResult struct {
	playbackState          PlaybackState
	artworkFingerprint     [sha256.Size]byte
	showOpenMediaAction    bool
	preferredDefaultAction string
}

func (m *MediaPlayerPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            PluginID,
		Name:          "i18n:plugin_media_player_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_media_player_plugin_description",
		Icon:          mediaIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"*",
			"media",
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
		Features: []plugin.MetadataFeature{},
	}
}

func (m *MediaPlayerPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	m.api = initParams.API
	m.pluginDirectory = initParams.PluginDirectory
	m.retriever = mediaRetriever
	m.retriever.UpdateAPI(m.api)
	m.trackedResults = util.NewHashMap[string, mediaTrackedResult]()

	// Handle plugin-to-plugin commands for media control (e.g. dictation
	// pauses media during voice input and resumes afterwards).
	m.api.OnHandlePluginCommand(ctx, func(ctx context.Context, request plugin.PluginCommandRequest) plugin.PluginCommandResult {
		switch request.Command {
		case PluginCommandPauseIfPlaying:
			return m.pauseIfPlaying(ctx)
		case mediaControlPause, mediaControlPlay, mediaControlToggle, mediaControlNext, mediaControlPrevious:
			if err := m.retriever.ControlMedia(ctx, request.Command); err != nil {
				return plugin.PluginCommandResult{Handled: false, Message: err.Error()}
			}
			return plugin.PluginCommandResult{Handled: true}
		default:
			return plugin.PluginCommandResult{Handled: false, Message: "unknown command: " + request.Command}
		}
	})

	// Start global refresh timer
	util.Go(ctx, "refresh media player", func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			m.refreshMediaPlayer(util.NewTraceContext())
		}
	})
}

// pauseIfPlaying only sends a pause command when the active media session is
// currently playing, allowing callers to restore playback only when they
// actually changed it.
func (m *MediaPlayerPlugin) pauseIfPlaying(ctx context.Context) plugin.PluginCommandResult {
	mediaInfo, err := m.retriever.GetCurrentMedia(ctx)
	if err != nil {
		return plugin.PluginCommandResult{Handled: true, Message: err.Error()}
	}
	if mediaInfo == nil {
		return plugin.PluginCommandResult{Handled: true, Message: PluginCommandResultNoActiveMedia}
	}
	if mediaInfo.State != PlaybackStatePlaying {
		return plugin.PluginCommandResult{Handled: true, Message: PluginCommandResultNotPlaying}
	}
	if err := m.retriever.ControlMedia(ctx, mediaControlPause); err != nil {
		return plugin.PluginCommandResult{Handled: true, Message: err.Error()}
	}
	return plugin.PluginCommandResult{Handled: true, Message: PluginCommandResultPaused}
}

func (m *MediaPlayerPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	if query.IsGlobalQuery() {
		return plugin.NewQueryResponse(m.queryGlobalControls(ctx, query))
	}

	var results []plugin.QueryResult

	// Get current media information
	mediaInfo, err := m.retriever.GetCurrentMedia(ctx)
	if err != nil {
		m.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to get media info: %s", err.Error()))
		return plugin.NewQueryResponse(results)
	}

	// No media playing
	if mediaInfo == nil {
		result := plugin.QueryResult{
			Title: "i18n:plugin_mediaplayer_no_media",
			Icon:  mediaIcon,
		}
		results = append(results, result)
		return plugin.NewQueryResponse(results)
	}

	result := m.buildMediaResult(mediaInfo, false, "")

	results = append(results, result)
	response := plugin.NewQueryResponse(results)
	fullPreviewWidthRatio := 0.0
	response.Layout.ResultPreviewWidthRatio = &fullPreviewWidthRatio
	return response
}

// queryGlobalControls keeps MediaRemote lookups limited to inputs that look like media commands.
func (m *MediaPlayerPlugin) queryGlobalControls(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	actions := m.matchMediaControlActions(query.RawQuery)
	if len(actions) == 0 {
		return nil
	}

	mediaInfo, err := m.retriever.GetCurrentMedia(ctx)
	if err != nil || mediaInfo == nil {
		return nil
	}

	// When the user types a specific command (e.g. "next"/"prev"), the result's
	// default action should be that command instead of the state-aware play/pause.
	preferredDefault := resolvePreferredDefaultCommand(query.RawQuery, actions)
	result := m.buildMediaResult(mediaInfo, true, preferredDefault)
	result.Score = mediaControlGlobalResultScore
	return []plugin.QueryResult{result}
}

// resolvePreferredDefaultCommand picks the most specific matched command so a
// global query's default action mirrors what the user asked for (exact over prefix).
func resolvePreferredDefaultCommand(search string, actions []mediaControlAction) string {
	normalized := strings.ToLower(strings.TrimSpace(search))
	for _, action := range actions {
		if action.command == normalized {
			return action.command
		}
		for _, alias := range action.aliases {
			if alias == normalized {
				return action.command
			}
		}
	}
	for _, action := range actions {
		if strings.HasPrefix(action.command, normalized) {
			return action.command
		}
	}
	return ""
}

// matchMediaControlActions resolves short command prefixes without making one-letter global input noisy.
func (m *MediaPlayerPlugin) matchMediaControlActions(search string) []mediaControlAction {
	normalized := strings.ToLower(strings.TrimSpace(search))
	if len(normalized) < 2 {
		return nil
	}

	matches := make([]mediaControlAction, 0, len(mediaControlActions))
	for _, action := range mediaControlActions {
		if action.matches(normalized) {
			matches = append(matches, action)
		}
	}
	return matches
}

// matches treats aliases as exact terms so "play" does not also match "playpause".
func (a mediaControlAction) matches(search string) bool {
	if a.command == search || strings.HasPrefix(a.command, search) {
		return true
	}

	for _, alias := range a.aliases {
		if alias == search {
			return true
		}
	}
	return false
}

// buildMediaResult creates the shared media status result used by both the media keyword and global commands.
func (m *MediaPlayerPlugin) buildMediaResult(mediaInfo *MediaInfo, showOpenMediaAction bool, preferredDefaultCommand string) plugin.QueryResult {
	actions := m.buildMediaActions(mediaInfo, preferredDefaultCommand)
	if showOpenMediaAction {
		actions = append(actions, m.buildOpenMediaAction())
	}

	result := plugin.QueryResult{
		Id:       uuid.NewString(),
		Title:    mediaInfo.Title,
		SubTitle: m.formatSubTitle(mediaInfo),
		Icon:     m.formatIcon(mediaInfo),
		Preview:  m.formatPreview(mediaInfo),
		Tails:    plugin.NewQueryResultTailTexts(m.formatProgress(mediaInfo)),
		Actions:  actions,
	}
	m.trackMediaResult(result.Id, mediaTrackedResult{
		playbackState:          mediaInfo.State,
		artworkFingerprint:     sha256.Sum256(mediaInfo.Artwork),
		showOpenMediaAction:    showOpenMediaAction,
		preferredDefaultAction: preferredDefaultCommand,
	})
	return result
}

// trackMediaResult is nil-safe so tests and direct plugin construction still get refresh behavior.
func (m *MediaPlayerPlugin) trackMediaResult(resultId string, tracked mediaTrackedResult) {
	if m.trackedResults == nil {
		m.trackedResults = util.NewHashMap[string, mediaTrackedResult]()
	}
	m.trackedResults.Store(resultId, tracked)
}

// buildMediaActions exposes one state-aware default action plus track navigation commands.
// preferredDefaultCommand overrides the state-aware default when the user typed a
// specific media command (e.g. "next"/"prev" should default to the matching action).
func (m *MediaPlayerPlugin) buildMediaActions(mediaInfo *MediaInfo, preferredDefaultCommand string) []plugin.QueryResultAction {
	defaultCommand := preferredDefaultCommand
	defaultName := ""
	switch preferredDefaultCommand {
	case mediaControlPlay:
		defaultName = "i18n:plugin_mediaplayer_play"
	case mediaControlPause:
		defaultName = "i18n:plugin_mediaplayer_pause"
	case mediaControlNext:
		defaultName = "i18n:plugin_mediaplayer_next"
	case mediaControlPrevious:
		defaultName = "i18n:plugin_mediaplayer_previous"
	}

	if defaultCommand == "" {
		defaultCommand = mediaControlPlay
		defaultName = "i18n:plugin_mediaplayer_play"
		if mediaInfo.State == PlaybackStatePlaying {
			defaultCommand = mediaControlPause
			defaultName = "i18n:plugin_mediaplayer_pause"
		}
	}

	return []plugin.QueryResultAction{
		m.buildMediaCommandAction(defaultCommand, defaultName, true),
		m.buildMediaCommandAction(mediaControlNext, "i18n:plugin_mediaplayer_next", false),
		m.buildMediaCommandAction(mediaControlPrevious, "i18n:plugin_mediaplayer_previous", false),
	}
}

// buildMediaCommandAction captures the command value so every action invokes its own playback operation.
func (m *MediaPlayerPlugin) buildMediaCommandAction(command string, name string, isDefault bool) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Id:                     "media-control-" + command,
		Name:                   name,
		IsDefault:              isDefault,
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			if err := m.retriever.ControlMedia(ctx, command); err != nil && m.api != nil {
				m.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to run media control %s: %s", command, err.Error()))
			}
		},
	}
}

// buildOpenMediaAction enters the dedicated media query without changing playback state.
func (m *MediaPlayerPlugin) buildOpenMediaAction() plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Id:                     "media-open-player",
		Name:                   "i18n:plugin_mediaplayer_open",
		Icon:                   mediaIcon,
		Hotkey:                 util.PrimaryHotkey("enter"),
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			if m.api != nil {
				m.api.ChangeQuery(ctx, common.PlainQuery{QueryType: plugin.QueryTypeInput, QueryText: "media "})
			}
		},
	}
}

func (m *MediaPlayerPlugin) formatProgress(mediaInfo *MediaInfo) string {
	durationStr := m.formatDuration(mediaInfo.Duration)
	positionStr := m.formatDuration(mediaInfo.Position)
	progressStr := fmt.Sprintf("%s / %s", positionStr, durationStr)

	return progressStr
}

func (m *MediaPlayerPlugin) formatSubTitle(mediaInfo *MediaInfo) string {
	newSubtitle := ""
	if mediaInfo.Artist != "" && mediaInfo.Album != "" {
		newSubtitle = fmt.Sprintf("%s - %s", mediaInfo.Artist, mediaInfo.Album)
	} else if mediaInfo.Artist != "" {
		newSubtitle = mediaInfo.Artist
	} else if mediaInfo.Album != "" {
		newSubtitle = mediaInfo.Album
	}

	return newSubtitle
}

func (m *MediaPlayerPlugin) formatIcon(mediaInfo *MediaInfo) common.WoxImage {
	if len(mediaInfo.Artwork) == 0 {
		if mediaInfo.State == PlaybackStatePlaying {
			return common.MediaPlayingIcon
		}
		return mediaIcon
	}

	coverDataURI, ok := formatRecordArtworkDataURI(mediaInfo.Artwork)
	if !ok {
		if mediaInfo.State == PlaybackStatePlaying {
			return common.MediaPlayingIcon
		}
		return mediaIcon
	}
	return common.NewWoxImageBase64(coverDataURI)
}

// formatRecordArtworkDataURI keeps the result icon small and gives its artwork a clean circular edge.
func formatRecordArtworkDataURI(artwork []byte) (string, bool) {
	decodedArtwork, err := decodeArtworkImageData(artwork)
	if err != nil {
		return "", false
	}

	source, _, err := image.Decode(bytes.NewReader(decodedArtwork))
	if err != nil {
		return "", false
	}
	scaled := image.NewNRGBA(image.Rect(0, 0, recordArtworkSize, recordArtworkSize))
	xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), source, centerSquare(source.Bounds()), xdraw.Src, nil)
	applyCircleAlpha(scaled)

	var output bytes.Buffer
	if err := png.Encode(&output, scaled); err != nil {
		return "", false
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(output.Bytes()), true
}

// decodeArtworkImageData accepts the macOS base64 payload and Windows raw image bytes.
func decodeArtworkImageData(artwork []byte) ([]byte, error) {
	encodedArtwork := strings.TrimSpace(string(artwork))
	if encodedArtwork != "" {
		if commaIndex := strings.Index(encodedArtwork, ","); commaIndex >= 0 && strings.HasPrefix(strings.ToLower(encodedArtwork[:commaIndex]), "data:") {
			encodedArtwork = encodedArtwork[commaIndex+1:]
		}

		decodedArtwork, err := base64.StdEncoding.DecodeString(encodedArtwork)
		if err == nil {
			return decodedArtwork, nil
		}
		decodedArtwork, err = base64.RawStdEncoding.DecodeString(encodedArtwork)
		if err == nil {
			return decodedArtwork, nil
		}
	}

	if len(artwork) == 0 {
		return nil, fmt.Errorf("empty artwork")
	}
	return artwork, nil
}

// centerSquare returns the largest centered crop inside the source artwork.
func centerSquare(bounds image.Rectangle) image.Rectangle {
	width := bounds.Dx()
	height := bounds.Dy()
	size := min(width, height)
	x := bounds.Min.X + (width-size)/2
	y := bounds.Min.Y + (height-size)/2
	return image.Rect(x, y, x+size, y+size)
}

// applyCircleAlpha masks the thumbnail because the artwork sits inside a round record label.
func applyCircleAlpha(img *image.NRGBA) {
	center := (float64(recordArtworkSize) - 1) / 2
	radius := float64(recordArtworkSize) / 2
	for y := 0; y < recordArtworkSize; y++ {
		for x := 0; x < recordArtworkSize; x++ {
			dx := float64(x) - center
			dy := float64(y) - center
			distance := math.Sqrt(dx*dx + dy*dy)
			alphaScale := 1.0
			if distance >= radius {
				alphaScale = 0
			} else if distance > radius-1 {
				alphaScale = radius - distance
			}

			pixelOffset := img.PixOffset(x, y)
			img.Pix[pixelOffset+3] = uint8(float64(img.Pix[pixelOffset+3]) * alphaScale)
		}
	}
}

func (m *MediaPlayerPlugin) formatPreview(mediaInfo *MediaInfo) plugin.WoxPreview {
	artwork := m.getMediaIcon(mediaInfo)
	previewData, _ := json.Marshal(mediaPreviewData{
		Title:     mediaInfo.Title,
		Artist:    mediaInfo.Artist,
		Album:     mediaInfo.Album,
		AppName:   mediaInfo.AppName,
		Artwork:   artwork.String(),
		Position:  mediaInfo.Position,
		Duration:  mediaInfo.Duration,
		IsPlaying: mediaInfo.State == PlaybackStatePlaying,
	})
	return plugin.WoxPreview{
		PreviewType: plugin.WoxPreviewTypeMedia,
		PreviewData: string(previewData),
	}
}

func (m *MediaPlayerPlugin) formatDuration(seconds int64) string {
	if seconds <= 0 {
		return "0:00"
	}

	minutes := seconds / 60
	secs := seconds % 60

	if minutes >= 60 {
		hours := minutes / 60
		minutes = minutes % 60
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, secs)
	}

	return fmt.Sprintf("%d:%02d", minutes, secs)
}

func (m *MediaPlayerPlugin) getMediaIcon(mediaInfo *MediaInfo) common.WoxImage {
	if len(mediaInfo.Artwork) > 0 {
		if artworkDataURI, ok := formatArtworkDataURI(mediaInfo.Artwork); ok {
			return common.NewWoxImageBase64(artworkDataURI)
		}
	}

	return mediaIcon
}

// formatArtworkDataURI validates and encodes artwork for the UI image preview contract.
func formatArtworkDataURI(artwork []byte) (string, bool) {
	decodedArtwork, err := decodeArtworkImageData(artwork)
	if err != nil {
		return "", false
	}

	_, format, err := image.DecodeConfig(bytes.NewReader(decodedArtwork))
	if err != nil {
		return "", false
	}
	if format == "jpg" {
		format = "jpeg"
	}
	if format == "" {
		format = "png"
	}

	return fmt.Sprintf("data:image/%s;base64,%s", format, base64.StdEncoding.EncodeToString(decodedArtwork)), true
}

func (m *MediaPlayerPlugin) refreshMediaPlayer(ctx context.Context) {
	if m.trackedResults == nil {
		return
	}

	// Skip refresh if window is hidden (for periodic updates like media player status)
	if !m.api.IsVisible(ctx) {
		return
	}

	var toRemove []string
	trackedUpdates := make(map[string]mediaTrackedResult)

	m.trackedResults.Range(func(resultId string, tracked mediaTrackedResult) bool {
		// Try to get the result, if it returns nil, the result is no longer visible
		updatableResult := m.api.GetUpdatableResult(ctx, resultId)
		if updatableResult == nil {
			// Mark for removal from tracking queue
			toRemove = append(toRemove, resultId)
			return true
		}

		// Get updated media information
		mediaInfo, err := m.retriever.GetCurrentMedia(ctx)
		if err != nil || mediaInfo == nil {
			// Keep current state if we can't get updated info
			return true
		}

		// Update all fields
		title := mediaInfo.Title
		subTitle := m.formatSubTitle(mediaInfo)
		preview := m.formatPreview(mediaInfo)
		tails := plugin.NewQueryResultTailTexts(m.formatProgress(mediaInfo))
		actions := m.buildMediaActions(mediaInfo, tracked.preferredDefaultAction)
		if tracked.showOpenMediaAction {
			actions = append(actions, m.buildOpenMediaAction())
		}

		updatableResult.Title = &title
		updatableResult.SubTitle = &subTitle
		nextArtworkFingerprint := sha256.Sum256(mediaInfo.Artwork)
		var nextTracked *mediaTrackedResult
		if tracked.playbackState != mediaInfo.State || tracked.artworkFingerprint != nextArtworkFingerprint {
			icon := m.formatIcon(mediaInfo)
			updatableResult.Icon = &icon
			updatedTracked := mediaTrackedResult{
				playbackState:          mediaInfo.State,
				artworkFingerprint:     nextArtworkFingerprint,
				showOpenMediaAction:    tracked.showOpenMediaAction,
				preferredDefaultAction: tracked.preferredDefaultAction,
			}
			nextTracked = &updatedTracked
		}
		updatableResult.Preview = &preview
		updatableResult.Tails = &tails
		updatableResult.Actions = &actions

		// Push update to UI
		// If UpdateResult returns false, the result is no longer visible in UI
		if !m.api.UpdateResult(ctx, *updatableResult) {
			toRemove = append(toRemove, resultId)
		} else if nextTracked != nil {
			trackedUpdates[resultId] = *nextTracked
		}
		return true
	})

	// Range holds a read lock, so changed tracking state must be written back after iteration.
	for resultId, tracked := range trackedUpdates {
		m.trackedResults.Store(resultId, tracked)
	}

	// Clean up results that are no longer visible
	for _, resultId := range toRemove {
		m.trackedResults.Delete(resultId)
	}
}
