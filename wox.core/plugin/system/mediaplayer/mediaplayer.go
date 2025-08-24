package mediaplayer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"wox/common"
	"wox/plugin"
	"wox/plugin/system"

	"github.com/google/uuid"
)

var mediaIcon = plugin.PluginMusicIcon

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
	m.retriever = m.getRetriever(ctx)
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

	if mediaInfo == nil {
		// No media playing
		result := plugin.QueryResult{
			Id:       uuid.NewString(),
			Title:    "No media playing",
			SubTitle: "No media application is currently playing",
			Icon:     mediaIcon,
			Score:    100,
			Actions: []plugin.QueryResultAction{
				{
					Name: "i18n:plugin_mediaplayer_no_media",
					Icon: plugin.SearchIcon,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						m.api.Notify(ctx, "No media is currently playing")
					},
				},
			},
		}
		results = append(results, result)
		return results
	}

	// Check if query matches media information
	searchTerm := strings.ToLower(query.Search)
	titleMatch, titleScore := system.IsStringMatchScore(ctx, mediaInfo.Title, query.Search)
	artistMatch, artistScore := system.IsStringMatchScore(ctx, mediaInfo.Artist, query.Search)
	albumMatch, albumScore := system.IsStringMatchScore(ctx, mediaInfo.Album, query.Search)
	appMatch, appScore := system.IsStringMatchScore(ctx, mediaInfo.AppName, query.Search)

	if titleMatch || artistMatch || albumMatch || appMatch || searchTerm == "" {
		contextData := mediaContextData{
			Title:       mediaInfo.Title,
			Artist:      mediaInfo.Artist,
			Album:       mediaInfo.Album,
			AppName:     mediaInfo.AppName,
			AppBundleID: mediaInfo.AppBundleID,
		}
		contextDataJson, _ := json.Marshal(contextData)

		// Format duration and position
		durationStr := m.formatDuration(mediaInfo.Duration)
		positionStr := m.formatDuration(mediaInfo.Position)
		progressStr := fmt.Sprintf("%s / %s", positionStr, durationStr)

		// Create subtitle with artist and album info
		subtitle := ""
		if mediaInfo.Artist != "" && mediaInfo.Album != "" {
			subtitle = fmt.Sprintf("%s - %s", mediaInfo.Artist, mediaInfo.Album)
		} else if mediaInfo.Artist != "" {
			subtitle = mediaInfo.Artist
		} else if mediaInfo.Album != "" {
			subtitle = mediaInfo.Album
		}

		if subtitle != "" {
			subtitle += fmt.Sprintf(" (%s) [%s]", progressStr, mediaInfo.State.String())
		} else {
			subtitle = fmt.Sprintf("%s [%s]", progressStr, mediaInfo.State.String())
		}

		// Calculate best match score
		maxScore := titleScore
		if artistScore > maxScore {
			maxScore = artistScore
		}
		if albumScore > maxScore {
			maxScore = albumScore
		}
		if appScore > maxScore {
			maxScore = appScore
		}

		result := plugin.QueryResult{
			Id:          uuid.NewString(),
			Title:       mediaInfo.Title,
			SubTitle:    subtitle,
			Icon:        m.getMediaIcon(mediaInfo),
			Score:       maxScore,
			ContextData: string(contextDataJson),
			Actions: []plugin.QueryResultAction{
				{
					Name:                   "i18n:plugin_mediaplayer_toggle",
					IsDefault:              true,
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						_ = m.retriever.TogglePlayPause(ctx)
					},
				},
				{
					Name: "i18n:plugin_mediaplayer_copy_info",
					Icon: plugin.CopyIcon,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						info := fmt.Sprintf("%s - %s", mediaInfo.Title, mediaInfo.Artist)
						m.api.ChangeQuery(ctx, common.PlainQuery{QueryText: info})
					},
				},
			},
			RefreshInterval: 1000,
			OnRefresh: func(ctx context.Context, current plugin.RefreshableResult) plugin.RefreshableResult {
				updated, err := m.retriever.GetCurrentMedia(ctx)
				if err != nil || updated == nil {
					return current
				}
				durationStr := m.formatDuration(updated.Duration)
				positionStr := m.formatDuration(updated.Position)
				progressStr := fmt.Sprintf("%s / %s", positionStr, durationStr)
				newSubtitle := ""
				if updated.Artist != "" && updated.Album != "" {
					newSubtitle = fmt.Sprintf("%s - %s", updated.Artist, updated.Album)
				} else if updated.Artist != "" {
					newSubtitle = updated.Artist
				} else if updated.Album != "" {
					newSubtitle = updated.Album
				}
				if newSubtitle != "" {
					newSubtitle += fmt.Sprintf(" (%s) [%s]", progressStr, updated.State.String())
				} else {
					newSubtitle = fmt.Sprintf("%s [%s]", progressStr, updated.State.String())
				}

				current.Title = updated.Title
				current.SubTitle = newSubtitle
				current.Icon = m.getMediaIcon(updated)
				return current
			},
		}

		results = append(results, result)
	}

	return results
}

func (m *MediaPlayerPlugin) getRetriever(ctx context.Context) MediaRetriever {
	return mediaRetriever
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
