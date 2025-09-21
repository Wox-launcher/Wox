package mediaplayer

import (
	"context"
	"fmt"
	"wox/common"
	"wox/plugin"
)

var mediaIcon = plugin.PluginMediaPlayerIcon

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &MediaPlayerPlugin{})
}

type MediaPlayerPlugin struct {
	api             plugin.API
	pluginDirectory string
	retriever       MediaRetriever
}

type mediaContextData struct {
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Album       string `json:"album"`
	AppName     string `json:"appName"`
	AppBundleID string `json:"appBundleId"`
}

func (m *MediaPlayerPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "b8f3d4e5-6c7a-4b9c-8d1e-2f3a4b5c6d7e",
		Name:          "Media Player",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "Get information about currently playing media",
		Icon:          mediaIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
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
}

func (m *MediaPlayerPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	var results []plugin.QueryResult

	// Get current media information
	mediaInfo, err := m.retriever.GetCurrentMedia(ctx)
	if err != nil {
		m.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to get media info: %s", err.Error()))
		return results
	}

	// No media playing
	if mediaInfo == nil {
		result := plugin.QueryResult{
			Title: "i18n:plugin_mediaplayer_no_media",
			Icon:  mediaIcon,
		}
		results = append(results, result)
		return results
	}

	result := plugin.QueryResult{
		Title:    mediaInfo.Title,
		SubTitle: m.formatSubTitle(mediaInfo),
		Icon:     m.formatIcon(mediaInfo),
		Preview:  m.formatPreview(mediaInfo),
		Tails:    plugin.NewQueryResultTailTexts(m.formatProgress(mediaInfo)),
		Actions: []plugin.QueryResultAction{
			{
				Name:                   "i18n:plugin_mediaplayer_toggle",
				IsDefault:              true,
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					_ = m.retriever.TogglePlayPause(ctx)
				},
			},
		},
		RefreshInterval: 1000,
		OnRefresh: func(ctx context.Context, current plugin.RefreshableResult) plugin.RefreshableResult {
			updated, err := m.retriever.GetCurrentMedia(ctx)
			if err != nil || updated == nil {
				return current
			}

			current.Title = updated.Title
			current.SubTitle = m.formatSubTitle(updated)
			current.Icon = m.formatIcon(updated)
			current.Preview = m.formatPreview(updated)
			current.Tails = plugin.NewQueryResultTailTexts(m.formatProgress(updated))
			return current
		},
	}

	results = append(results, result)

	return results
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
	if mediaInfo.State == PlaybackStatePlaying {
		return plugin.MediaPlayingIcon
	} else {
		return mediaIcon
	}
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
