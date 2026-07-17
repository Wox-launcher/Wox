package launcher

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// buildPreview resolves controller-owned preview state into a pure preview view.
func (a *App) buildPreview(result queryResult, palette uiPalette, width, height float32) woxwidget.Widget {
	preview := a.resolvePreview(result.Preview)
	if preview.PreviewType == "query_requirement_settings" {
		return a.buildRequirementPreview(result, preview, palette, width, height)
	}
	if preview.PreviewType == "trigger_keyword_conflict" {
		return a.buildTriggerConflictPreview(result, preview, palette, width, height)
	}
	if preview.PreviewType == "theme_edit" {
		return a.buildThemeEditorPreview(result, preview, palette, width, height)
	}
	if preview.PreviewType == "media" {
		data, err := decodeMediaPreview(preview.PreviewData)
		if err != nil {
			return previewview.PreviewError(fmt.Sprintf("Invalid media preview: %v", err), width, height, palette.componentTheme())
		}
		return a.buildMediaPreview(result, data, palette, width, height)
	}
	if preview.PreviewType == "chat" {
		return a.buildChatPreview(result, preview, palette, width, height)
	}
	scrollKey := result.QueryID + "\x00" + result.ID + "\x00" + preview.PreviewType
	tags := append(a.previewTagLabels(preview.PreviewTags), a.previewTagLabels(a.previewBodyTags(preview))...)
	layout := previewview.ResolvePreviewLayout(width, height, len(tags) > 0)
	body := a.buildPreviewBody(scrollKey, preview, palette, layout.BodyWidth, layout.BodyHeight)
	return previewview.PreviewView(previewview.PreviewProps{
		Width: width, Height: height, Tags: tags, Body: body, Theme: palette.componentTheme(), Window: a.window,
	})
}

func (a *App) buildPreviewBody(scrollKey string, preview queryPreview, palette uiPalette, width, height float32) woxwidget.Widget {
	content := func(value string, color woxui.Color) woxwidget.Widget {
		if strings.TrimSpace(value) == "" {
			value = "No preview available"
		}
		return a.buildScrollablePreviewText(scrollKey, value, color, preview.ScrollPosition, width, height)
	}
	errorText := palette.componentTheme().ErrorText
	switch preview.PreviewType {
	case "text":
		return a.buildTextPreview(scrollKey, preview.PreviewData, preview.ScrollPosition, palette, width, height)
	case "markdown":
		return content(preview.PreviewData, previewColorWithOpacity(palette.previewText, 0.86))
	case "image":
		source, ok := parsePreviewImage(preview.PreviewData)
		if !ok {
			return content("Invalid image preview data", errorText)
		}
		overlay := source
		if candidate, valid := parsePreviewImage(preview.PreviewOverlayData); valid {
			overlay = candidate
		}
		return a.buildPreviewImage(source, overlay, palette, width, height)
	case "file":
		file := a.filePreviewFor(preview.PreviewData)
		switch file.Kind {
		case "image":
			return a.buildPreviewImage(file.Image, file.Image, palette, width, height)
		case "error":
			return content(file.Text, errorText)
		default:
			return a.buildTextPreview(scrollKey, file.Text, preview.ScrollPosition, palette, width, height)
		}
	case "list":
		data, err := decodePreviewList(preview.PreviewData)
		if err != nil {
			return content(fmt.Sprintf("Invalid list preview data: %v", err), errorText)
		}
		return a.buildListPreview(data, palette, width, height)
	case "plugin_detail":
		data, err := decodeStructuredPreview[pluginDetailPreviewData](preview.PreviewData)
		if err != nil {
			return content(fmt.Sprintf("Invalid plugin detail preview: %v", err), errorText)
		}
		return a.buildPluginDetailPreview(data, palette, width, height)
	case "update":
		data, err := decodeStructuredPreview[updatePreviewData](preview.PreviewData)
		if err != nil {
			return content(fmt.Sprintf("Invalid update preview: %v", err), errorText)
		}
		return content(formatUpdatePreview(data), palette.previewText)
	case "ai_stream":
		data, err := decodeStructuredPreview[aiStreamPreviewData](preview.PreviewData)
		if err != nil {
			return content(fmt.Sprintf("Invalid AI stream preview: %v", err), errorText)
		}
		return content(formatAIStreamPreview(data), palette.previewText)
	case "dictation_history":
		data, err := decodeStructuredPreview[dictationHistoryPreviewData](preview.PreviewData)
		if err != nil {
			return content(fmt.Sprintf("Invalid dictation history preview: %v", err), errorText)
		}
		return content(formatDictationHistoryPreview(data), palette.previewText)
	case "hotkey_overview":
		data, err := decodeStructuredPreview[hotkeyOverviewPreviewData](preview.PreviewData)
		if err != nil {
			data = hotkeyOverviewPreviewData{}
		}
		return content(a.formatHotkeyOverview(data), palette.previewText)
	case "url":
		return content("URL preview\n\n"+preview.PreviewData+"\n\nThe embedded browser surface will be attached through the platform preview host.", palette.previewText)
	case "terminal":
		return a.buildTerminalPreview(a.terminalPreviewSnapshotFor(preview), palette, width, height)
	case "webview":
		return a.buildWebViewPreview(preview.PreviewData, palette, width, height)
	default:
		return content(preview.PreviewData, palette.previewText)
	}
}

// previewBodyTags resolves metadata before the body is built at its final tagged height.
func (a *App) previewBodyTags(preview queryPreview) []previewTag {
	switch preview.PreviewType {
	case "file":
		return a.filePreviewFor(preview.PreviewData).Tags
	case "update":
		data, err := decodeStructuredPreview[updatePreviewData](preview.PreviewData)
		if err == nil {
			return previewTagsForValues(data.ReleaseChannel, data.Status)
		}
	case "ai_stream":
		data, err := decodeStructuredPreview[aiStreamPreviewData](preview.PreviewData)
		if err == nil {
			return previewTagsForValues(data.StatusLabel)
		}
	case "dictation_history":
		data, err := decodeStructuredPreview[dictationHistoryPreviewData](preview.PreviewData)
		if err == nil {
			return previewTagsForValues(data.StatusLabel)
		}
	}
	return nil
}

func (a *App) buildScrollablePreviewText(scrollKey, value string, color woxui.Color, scrollPosition string, width, height float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-48)
	style := woxui.TextStyle{Size: 15}
	layout := a.previewTextLayout(scrollKey, value, style, innerWidth, 23)
	initialOffset := float32(0)
	if scrollPosition == "bottom" {
		initialOffset = float32(math.MaxFloat32)
	}
	return previewview.ScrollablePreviewText(previewview.ScrollablePreviewTextProps{
		ID: scrollKey, Value: value, Color: color, Width: width, Height: height, Layout: layout, InitialOffset: initialOffset,
	})
}

func (a *App) buildTextPreview(scrollKey, value, scrollPosition string, palette uiPalette, width, height float32) woxwidget.Widget {
	if strings.TrimSpace(value) == "" {
		value = "No preview available"
	}
	const horizontalPadding = float32(44)
	style := woxui.TextStyle{Size: 17}
	textWidth := max(float32(0), width-horizontalPadding*2)
	layout := a.previewTextLayout(scrollKey+"|quote", value, style, textWidth, 25)
	if !previewview.TextPreviewFits(layout, width, height) {
		return a.buildScrollablePreviewText(scrollKey, value, previewColorWithOpacity(palette.previewText, 0.86), scrollPosition, width, height)
	}
	return previewview.TextPreview(previewview.TextPreviewProps{
		Value: value, Width: width, Height: height, Layout: layout, Theme: palette.componentTheme(), Window: a.window,
	})
}

func (a *App) previewTextLayout(scrollKey, value string, style woxui.TextStyle, width, lineHeight float32) woxwidget.TextBlockLayout {
	hash := sha256.Sum256([]byte(value))
	key := fmt.Sprintf("%s|%.2f|%.2f|%d|%x", scrollKey, width, style.Size, style.Weight, hash)
	a.mu.RLock()
	if layout, ok := a.previewLayouts[key]; ok {
		a.mu.RUnlock()
		return layout
	}
	a.mu.RUnlock()
	layout := woxwidget.LayoutTextBlock(a.window, value, style, width, 0, lineHeight)
	a.mu.Lock()
	if len(a.previewLayouts) >= 128 {
		a.previewLayouts = map[string]woxwidget.TextBlockLayout{}
	}
	a.previewLayouts[key] = layout
	a.mu.Unlock()
	return layout
}

func (a *App) buildPreviewImage(source, overlay woxImage, palette uiPalette, width, height float32) woxwidget.Widget {
	image := a.imageFor(source)
	message := "Loading image preview…"
	color := palette.resultSubtitle
	if image == nil {
		if imageErr := a.imageErrorFor(source); imageErr != "" {
			message = "Unable to decode image preview:\n" + imageErr
			color = palette.componentTheme().ErrorText
		}
	}
	return previewview.PreviewImage(previewview.PreviewImageProps{
		Width: width, Height: height, Image: image, Message: message, MessageColor: color,
		OnTap: func() { a.openPreviewImageOverlay(overlay) },
	})
}

func (a *App) openPreviewImageOverlay(image woxImage) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := a.client.Post(ctx, "/preview/image/overlay", map[string]any{"Image": image}, nil); err != nil {
			log.Printf("open preview image overlay: %v", err)
		}
	}()
}

func (a *App) buildListPreview(data previewListData, palette uiPalette, width, height float32) woxwidget.Widget {
	items := make([]previewview.PreviewListItem, 0, len(data.Items))
	for index, item := range data.Items {
		tail := ""
		if len(item.Tails) > 0 {
			tail = item.Tails[0].Text
		}
		var icon = (*woxui.Image)(nil)
		if item.Icon != nil {
			icon = a.imageFor(*item.Icon)
		}
		items = append(items, previewview.PreviewListItem{
			Title: item.Title, Subtitle: item.Subtitle, Tail: tail, Icon: icon, FallbackColor: resultColors[index%len(resultColors)],
		})
	}
	return previewview.PreviewList(previewview.PreviewListProps{Width: width, Height: height, Items: items, Theme: palette.componentTheme()})
}

func (a *App) previewTagLabels(tags []previewTag) []string {
	labels := make([]string, 0, len(tags))
	for _, tag := range tags {
		if label := a.translate(tag.Label); strings.TrimSpace(label) != "" {
			labels = append(labels, label)
		}
	}
	return labels
}

func previewColorWithOpacity(color woxui.Color, opacity float32) woxui.Color {
	opacity = min(max(float32(0), opacity), float32(1))
	color.A = uint8(opacity*255 + 0.5)
	return color
}
