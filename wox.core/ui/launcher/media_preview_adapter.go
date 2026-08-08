package launcher

import (
	"strings"

	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// buildMediaPreview maps media metadata and result actions into the shared preview view.
func (a *App) buildMediaPreview(result queryResult, data mediaPreviewData, palette uiPalette, width, height float32) woxwidget.Widget {
	title := strings.TrimSpace(data.Title)
	if title == "" {
		title = "Unknown track"
	}
	artist := strings.TrimSpace(data.Artist)
	if artist == "" {
		artist = "Unknown artist"
	}
	duration := max(int64(0), data.Duration)
	position := min(max(int64(0), data.Position), duration)
	var artwork *woxui.Image
	if source, ok := parsePreviewImage(data.Artwork); ok {
		artwork = a.imageFor(source)
	}
	action := func(id string) func() {
		return func() { a.activateResultActionByID(result.QueryID, result.ID, id) }
	}
	toggleLabelKey := "i18n:plugin_mediaplayer_play"
	if data.IsPlaying {
		toggleLabelKey = "i18n:plugin_mediaplayer_pause"
	}
	return previewview.MediaPreviewView(previewview.MediaPreviewProps{
		Width: width, Height: height, Title: title, Artist: artist, Album: strings.TrimSpace(data.Album), AppName: strings.TrimSpace(data.AppName), Artwork: artwork,
		Position: position, Duration: duration, Playing: data.IsPlaying, Theme: palette.componentTheme(),
		PreviousLabel: a.translate("i18n:plugin_mediaplayer_previous"), ToggleLabel: a.translate(toggleLabelKey), NextLabel: a.translate("i18n:plugin_mediaplayer_next"),
		ActionIdentity: result.QueryID + "\x00" + result.ID,
		Window:         a.window,
		OnPrevious:     action("media-control-previous"), OnPlay: action("media-control-play"), OnPause: action("media-control-pause"), OnNext: action("media-control-next"),
	})
}
