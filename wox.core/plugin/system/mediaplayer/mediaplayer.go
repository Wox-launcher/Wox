package mediaplayer

import (
	"bytes"
	"context"
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
	mediaControlPlay     = "play"
	mediaControlPause    = "pause"
	mediaControlToggle   = "toggle"
	mediaControlNext     = "next"
	mediaControlPrevious = "previous"

	mediaControlGlobalResultScore int64 = 200

	recordArtworkSize = 96

	recordAnimatedRotationJSON = `"r":{"a":1,"k":[{"t":0,"s":[0]},{"t":119,"s":[360]}]}`
	recordStaticRotationJSON   = `"r":{"a":0,"k":0}`
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

type mediaTrackedResult struct{}

func (m *MediaPlayerPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "b8f3d4e5-6c7a-4b9c-8d1e-2f3a4b5c6d7e",
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

	// Start global refresh timer
	util.Go(ctx, "refresh media player", func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			m.refreshMediaPlayer(util.NewTraceContext())
		}
	})
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

	result := m.buildMediaResult(mediaInfo)

	results = append(results, result)

	return plugin.NewQueryResponse(results)
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

	result := m.buildMediaResult(mediaInfo)
	result.Score = mediaControlGlobalResultScore
	return []plugin.QueryResult{result}
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
func (m *MediaPlayerPlugin) buildMediaResult(mediaInfo *MediaInfo) plugin.QueryResult {
	result := plugin.QueryResult{
		Id:       uuid.NewString(),
		Title:    mediaInfo.Title,
		SubTitle: m.formatSubTitle(mediaInfo),
		Icon:     m.formatIcon(mediaInfo),
		Preview:  m.formatPreview(mediaInfo),
		Tails:    plugin.NewQueryResultTailTexts(m.formatProgress(mediaInfo)),
		Actions:  m.buildMediaActions(mediaInfo),
	}
	m.trackMediaResult(result.Id, mediaTrackedResult{})
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
func (m *MediaPlayerPlugin) buildMediaActions(mediaInfo *MediaInfo) []plugin.QueryResultAction {
	defaultCommand := mediaControlPlay
	defaultName := "i18n:plugin_mediaplayer_play"
	if mediaInfo.State == PlaybackStatePlaying {
		defaultCommand = mediaControlPause
		defaultName = "i18n:plugin_mediaplayer_pause"
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

	coverDataURI := m.formatRecordArtworkDataURI(mediaInfo.Artwork)
	if mediaInfo.State != PlaybackStatePlaying {
		return common.NewWoxImageLottie(m.buildStaticRecordLottie(coverDataURI))
	}
	return common.NewWoxImageLottie(m.buildRecordLottie(coverDataURI))
}

// formatRecordArtworkDataURI normalizes album artwork before Lottie overlays are drawn on top.
func (m *MediaPlayerPlugin) formatRecordArtworkDataURI(artwork []byte) string {
	normalizedArtwork, err := buildCircularArtworkPNG(artwork)
	if err != nil {
		return "data:image/png;base64," + string(artwork)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(normalizedArtwork)
}

// buildCircularArtworkPNG crops, scales, and alpha-masks the artwork so Lottie does not need image masks.
func buildCircularArtworkPNG(artwork []byte) ([]byte, error) {
	decodedArtwork, err := decodeArtworkBase64(artwork)
	if err != nil {
		return nil, err
	}

	source, _, err := image.Decode(bytes.NewReader(decodedArtwork))
	if err != nil {
		return nil, err
	}

	scaled := image.NewNRGBA(image.Rect(0, 0, recordArtworkSize, recordArtworkSize))
	xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), source, centerSquare(source.Bounds()), xdraw.Src, nil)
	applyCircleAlpha(scaled)

	var buf bytes.Buffer
	if err := png.Encode(&buf, scaled); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decodeArtworkBase64 accepts both raw base64 strings and data URI payloads.
func decodeArtworkBase64(artwork []byte) ([]byte, error) {
	encodedArtwork := strings.TrimSpace(string(artwork))
	if commaIndex := strings.Index(encodedArtwork, ","); commaIndex >= 0 && strings.HasPrefix(strings.ToLower(encodedArtwork[:commaIndex]), "data:") {
		encodedArtwork = encodedArtwork[commaIndex+1:]
	}

	decodedArtwork, err := base64.StdEncoding.DecodeString(encodedArtwork)
	if err != nil {
		decodedArtwork, err = base64.RawStdEncoding.DecodeString(encodedArtwork)
	}
	return decodedArtwork, err
}

// centerSquare returns the largest centered square inside the source artwork bounds.
func centerSquare(bounds image.Rectangle) image.Rectangle {
	width := bounds.Dx()
	height := bounds.Dy()
	size := min(width, height)
	x := bounds.Min.X + (width-size)/2
	y := bounds.Min.Y + (height-size)/2
	return image.Rect(x, y, x+size, y+size)
}

// applyCircleAlpha cuts the scaled artwork into a circle with a one-pixel antialiased edge.
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

// buildRecordLottie embeds artwork into a spinning vinyl-style Lottie icon.
func (m *MediaPlayerPlugin) buildRecordLottie(coverDataURI string) string {
	return fmt.Sprintf(`{"v":"5.7.4","fr":30,"ip":0,"op":120,"w":100,"h":100,"nm":"Media Record","ddd":0,"assets":[{"id":"cover","w":96,"h":96,"u":"","p":%q,"e":1}],"layers":[{"ddd":0,"ind":1,"ty":4,"nm":"Center Hub","sr":1,"ks":{"a":{"a":0,"k":[50,50,0]},"p":{"a":0,"k":[50,50,0]},"s":{"a":0,"k":[100,100,100]},"r":{"a":0,"k":0},"o":{"a":0,"k":100}},"ip":0,"op":120,"st":0,"shapes":[{"ty":"gr","it":[{"ty":"el","p":{"a":0,"k":[50,50]},"s":{"a":0,"k":[22,22]}},{"ty":"fl","c":{"a":0,"k":[0.54,0.02,0.04,1]},"o":{"a":0,"k":100},"r":1},{"ty":"tr","p":{"a":0,"k":[0,0]},"a":{"a":0,"k":[0,0]},"s":{"a":0,"k":[100,100]},"r":{"a":0,"k":0},"o":{"a":0,"k":100}}]}]},{"ddd":0,"ind":2,"ty":4,"nm":"Record Needle","sr":1,"ks":{"a":{"a":0,"k":[0,0,0]},"p":{"a":0,"k":[0,0,0]},"s":{"a":0,"k":[100,100,100]},"r":{"a":0,"k":0},"o":{"a":0,"k":100}},"ip":0,"op":120,"st":0,"shapes":[{"ty":"gr","it":[{"ty":"sh","ks":{"a":0,"k":{"c":false,"i":[[0,0],[0,0]],"o":[[0,0],[0,0]],"v":[[73,17],[54,46]]}}},{"ty":"st","c":{"a":0,"k":[0.82,0.76,0.66,1]},"o":{"a":0,"k":100},"w":{"a":0,"k":4},"lc":2,"lj":2},{"ty":"tr","p":{"a":0,"k":[0,0]},"a":{"a":0,"k":[0,0]},"s":{"a":0,"k":[100,100]},"r":{"a":0,"k":0},"o":{"a":0,"k":100}}]},{"ty":"gr","it":[{"ty":"el","p":{"a":0,"k":[73,17]},"s":{"a":0,"k":[9,9]}},{"ty":"fl","c":{"a":0,"k":[0.42,0.38,0.34,1]},"o":{"a":0,"k":100},"r":1},{"ty":"tr","p":{"a":0,"k":[0,0]},"a":{"a":0,"k":[0,0]},"s":{"a":0,"k":[100,100]},"r":{"a":0,"k":0},"o":{"a":0,"k":100}}]},{"ty":"gr","it":[{"ty":"sh","ks":{"a":0,"k":{"c":true,"i":[[0,0],[0,0],[0,0]],"o":[[0,0],[0,0],[0,0]],"v":[[53,44],[59,48],[52,51]]}}},{"ty":"fl","c":{"a":0,"k":[0.92,0.88,0.76,1]},"o":{"a":0,"k":100},"r":1},{"ty":"tr","p":{"a":0,"k":[0,0]},"a":{"a":0,"k":[0,0]},"s":{"a":0,"k":[100,100]},"r":{"a":0,"k":0},"o":{"a":0,"k":100}}]}]},{"ddd":0,"ind":3,"ty":4,"nm":"Vinyl Overlay","sr":1,"ks":{"a":{"a":0,"k":[50,50,0]},"p":{"a":0,"k":[50,50,0]},"s":{"a":0,"k":[100,100,100]},"r":{"a":1,"k":[{"t":0,"s":[0]},{"t":119,"s":[360]}]},"o":{"a":0,"k":100}},"ip":0,"op":120,"st":0,"shapes":[{"ty":"gr","it":[{"ty":"el","p":{"a":0,"k":[50,50]},"s":{"a":0,"k":[76,76]}},{"ty":"st","c":{"a":0,"k":[0.015,0.015,0.018,1]},"o":{"a":0,"k":100},"w":{"a":0,"k":20},"lc":1,"lj":1},{"ty":"tr","p":{"a":0,"k":[0,0]},"a":{"a":0,"k":[0,0]},"s":{"a":0,"k":[100,100]},"r":{"a":0,"k":0},"o":{"a":0,"k":100}}]},{"ty":"gr","it":[{"ty":"el","p":{"a":0,"k":[50,50]},"s":{"a":0,"k":[82,82]}},{"ty":"st","c":{"a":0,"k":[0.11,0.11,0.12,1]},"o":{"a":0,"k":75},"w":{"a":0,"k":1.4},"lc":1,"lj":1},{"ty":"tr","p":{"a":0,"k":[0,0]},"a":{"a":0,"k":[0,0]},"s":{"a":0,"k":[100,100]},"r":{"a":0,"k":0},"o":{"a":0,"k":100}}]},{"ty":"gr","it":[{"ty":"el","p":{"a":0,"k":[50,50]},"s":{"a":0,"k":[72,72]}},{"ty":"st","c":{"a":0,"k":[0.07,0.07,0.08,1]},"o":{"a":0,"k":65},"w":{"a":0,"k":1.2},"lc":1,"lj":1},{"ty":"tr","p":{"a":0,"k":[0,0]},"a":{"a":0,"k":[0,0]},"s":{"a":0,"k":[100,100]},"r":{"a":0,"k":0},"o":{"a":0,"k":100}}]},{"ty":"gr","it":[{"ty":"el","p":{"a":0,"k":[50,50]},"s":{"a":0,"k":[62,62]}},{"ty":"st","c":{"a":0,"k":[0.04,0.04,0.045,1]},"o":{"a":0,"k":55},"w":{"a":0,"k":1},"lc":1,"lj":1},{"ty":"tr","p":{"a":0,"k":[0,0]},"a":{"a":0,"k":[0,0]},"s":{"a":0,"k":[100,100]},"r":{"a":0,"k":0},"o":{"a":0,"k":100}}]}]},{"ddd":0,"ind":4,"ty":2,"nm":"Artwork","refId":"cover","sr":1,"ks":{"a":{"a":0,"k":[48,48,0]},"p":{"a":0,"k":[50,50,0]},"s":{"a":0,"k":[100,100,100]},"r":{"a":1,"k":[{"t":0,"s":[0]},{"t":119,"s":[360]}]},"o":{"a":0,"k":100},"sk":{"a":0,"k":0},"sa":{"a":0,"k":0}},"ip":0,"op":120,"st":0}]}`, coverDataURI)
}

// buildStaticRecordLottie keeps the vinyl-style icon visible without rotation for paused media.
func (m *MediaPlayerPlugin) buildStaticRecordLottie(coverDataURI string) string {
	lottieJSON := strings.ReplaceAll(m.buildRecordLottie(coverDataURI), recordAnimatedRotationJSON, recordStaticRotationJSON)
	return removeLottieLayer(lottieJSON, "Record Needle")
}

// removeLottieLayer deletes a named top-level layer while preserving a valid Lottie document.
func removeLottieLayer(lottieJSON string, layerName string) string {
	var data map[string]any
	if err := json.Unmarshal([]byte(lottieJSON), &data); err != nil {
		return lottieJSON
	}

	layers, ok := data["layers"].([]any)
	if !ok {
		return lottieJSON
	}

	filteredLayers := make([]any, 0, len(layers))
	for _, layer := range layers {
		layerData, ok := layer.(map[string]any)
		if ok && layerData["nm"] == layerName {
			continue
		}
		filteredLayers = append(filteredLayers, layer)
	}
	data["layers"] = filteredLayers

	encoded, err := json.Marshal(data)
	if err != nil {
		return lottieJSON
	}
	return string(encoded)
}

func (m *MediaPlayerPlugin) formatPreview(mediaInfo *MediaInfo) plugin.WoxPreview {
	coverImg := m.getMediaIcon(mediaInfo)
	return plugin.WoxPreview{
		PreviewType: plugin.WoxPreviewTypeImage,
		PreviewData: coverImg.String(),
		PreviewProperties: map[string]string{
			"i18n:plugin_mediaplayer_artist":   mediaInfo.Artist,
			"i18n:plugin_mediaplayer_album":    mediaInfo.Album,
			"i18n:plugin_mediaplayer_duration": m.formatDuration(mediaInfo.Duration),
		},
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
	// Try to use artwork if available
	if len(mediaInfo.Artwork) > 0 {
		return common.WoxImage{
			ImageType: common.WoxImageTypeBase64,
			ImageData: "data:image/png;base64," + string(mediaInfo.Artwork),
		}
	}

	// Fall back to default media icon
	return mediaIcon
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

	m.trackedResults.Range(func(resultId string, _ mediaTrackedResult) bool {
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
		icon := m.formatIcon(mediaInfo)
		preview := m.formatPreview(mediaInfo)
		tails := plugin.NewQueryResultTailTexts(m.formatProgress(mediaInfo))
		actions := m.buildMediaActions(mediaInfo)

		updatableResult.Title = &title
		updatableResult.SubTitle = &subTitle
		updatableResult.Icon = &icon
		updatableResult.Preview = &preview
		updatableResult.Tails = &tails
		updatableResult.Actions = &actions

		// Push update to UI
		// If UpdateResult returns false, the result is no longer visible in UI
		if !m.api.UpdateResult(ctx, *updatableResult) {
			toRemove = append(toRemove, resultId)
		}
		return true
	})

	// Clean up results that are no longer visible
	for _, resultId := range toRemove {
		m.trackedResults.Delete(resultId)
	}
}
