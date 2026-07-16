package launcher

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"strings"
	"time"

	woxui "github.com/Wox-launcher/wox.ui.go"
	woxwidget "github.com/Wox-launcher/wox.ui.go/widget"
)

func (a *App) buildPreview(result queryResult, palette uiPalette, width, height float32) woxwidget.Widget {
	preview := a.resolvePreview(result.Preview)
	if preview.PreviewType != "trigger_keyword_conflict" {
		a.deactivateTriggerConflictPreview()
	}
	if preview.PreviewType != "theme_edit" {
		a.deactivateThemeEditorPreview()
	}
	if preview.PreviewType != "chat" {
		a.deactivateChatPreview()
	}
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
			return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.UniformInsets(18), Child: woxwidget.TextBlock{Value: fmt.Sprintf("Invalid media preview: %v", err), Width: max(float32(0), width-36), Height: max(float32(0), height-36), Style: woxui.TextStyle{Size: 13}, Color: woxui.Color{R: 232, G: 95, B: 95, A: 255}}}
		}
		return a.buildMediaPreview(result, data, palette, width, height)
	}
	if preview.PreviewType == "chat" {
		return a.buildChatPreview(result, preview, palette, width, height)
	}
	scrollKey := result.QueryID + "\x00" + result.ID + "\x00" + preview.PreviewType
	innerWidth := max(float32(0), width-36)
	innerHeight := max(float32(0), height-20)
	headerHeight := float32(48)
	tags := append([]previewTag(nil), preview.PreviewTags...)
	tagHeight := float32(0)
	bodyHeight := max(float32(0), innerHeight-headerHeight)
	body, extraTags := a.buildPreviewBody(scrollKey, preview, palette, innerWidth, bodyHeight)
	tags = append(tags, extraTags...)
	if len(tags) > 0 {
		tagHeight = 34
		bodyHeight = max(float32(0), bodyHeight-tagHeight)
		body, extraTags = a.buildPreviewBody(scrollKey, preview, palette, innerWidth, bodyHeight)
		_ = extraTags
	}
	children := []woxwidget.StackChild{
		{Child: woxwidget.Text{Value: result.Title, Style: woxui.TextStyle{Size: 18, Weight: woxui.FontWeightSemibold}, Color: palette.previewText}},
		{Top: 27, Child: woxwidget.Text{Value: preview.PreviewType, Style: woxui.TextStyle{Size: 12}, Color: palette.resultSubtitle}},
		{Top: headerHeight, Child: body},
	}
	if tagHeight > 0 {
		children = append(children, woxwidget.StackChild{Top: headerHeight + bodyHeight + 7, Child: a.buildPreviewTags(tags, palette, innerWidth)})
	}
	return woxwidget.Container{
		Width: width, Height: height, Padding: woxwidget.Insets{Left: 18, Top: 8, Right: 18, Bottom: 12},
		Child: woxwidget.Stack{Width: innerWidth, Height: innerHeight, Children: children},
	}
}

func (a *App) buildPreviewBody(scrollKey string, preview queryPreview, palette uiPalette, width, height float32) (woxwidget.Widget, []previewTag) {
	if preview.PreviewType != "terminal" {
		a.deactivateTerminalPreview()
	}
	if preview.PreviewType != "webview" {
		a.deactivateWebViewPreview()
	}
	content := func(value string, color woxui.Color) woxwidget.Widget {
		if strings.TrimSpace(value) == "" {
			value = "No preview available"
		}
		return a.buildScrollablePreviewText(scrollKey, value, color, preview.ScrollPosition, palette, width, height)
	}
	switch preview.PreviewType {
	case "text", "markdown":
		return content(preview.PreviewData, palette.previewText), nil
	case "image":
		source, ok := parsePreviewImage(preview.PreviewData)
		if !ok {
			return content("Invalid image preview data", woxui.Color{R: 232, G: 95, B: 95, A: 255}), nil
		}
		overlay := source
		if candidate, valid := parsePreviewImage(preview.PreviewOverlayData); valid {
			overlay = candidate
		}
		return a.buildPreviewImage(source, overlay, palette, width, height), nil
	case "file":
		file := a.filePreviewFor(preview.PreviewData)
		switch file.Kind {
		case "image":
			return a.buildPreviewImage(file.Image, file.Image, palette, width, height), file.Tags
		case "error":
			return content(file.Text, woxui.Color{R: 232, G: 95, B: 95, A: 255}), file.Tags
		default:
			return content(file.Text, palette.previewText), file.Tags
		}
	case "list":
		data, err := decodePreviewList(preview.PreviewData)
		if err != nil {
			return content(fmt.Sprintf("Invalid list preview data: %v", err), woxui.Color{R: 232, G: 95, B: 95, A: 255}), nil
		}
		return a.buildListPreview(data, palette, width, height), nil
	case "plugin_detail":
		data, err := decodeStructuredPreview[pluginDetailPreviewData](preview.PreviewData)
		if err != nil {
			return content(fmt.Sprintf("Invalid plugin detail preview: %v", err), woxui.Color{R: 232, G: 95, B: 95, A: 255}), nil
		}
		return a.buildPluginDetailPreview(data, palette, width, height), nil
	case "update":
		data, err := decodeStructuredPreview[updatePreviewData](preview.PreviewData)
		if err != nil {
			return content(fmt.Sprintf("Invalid update preview: %v", err), woxui.Color{R: 232, G: 95, B: 95, A: 255}), nil
		}
		tags := previewTagsForValues(data.ReleaseChannel, data.Status)
		return content(formatUpdatePreview(data), palette.previewText), tags
	case "ai_stream":
		data, err := decodeStructuredPreview[aiStreamPreviewData](preview.PreviewData)
		if err != nil {
			return content(fmt.Sprintf("Invalid AI stream preview: %v", err), woxui.Color{R: 232, G: 95, B: 95, A: 255}), nil
		}
		return content(formatAIStreamPreview(data), palette.previewText), previewTagsForValues(data.StatusLabel)
	case "dictation_history":
		data, err := decodeStructuredPreview[dictationHistoryPreviewData](preview.PreviewData)
		if err != nil {
			return content(fmt.Sprintf("Invalid dictation history preview: %v", err), woxui.Color{R: 232, G: 95, B: 95, A: 255}), nil
		}
		return content(formatDictationHistoryPreview(data), palette.previewText), previewTagsForValues(data.StatusLabel)
	case "hotkey_overview":
		data, err := decodeStructuredPreview[hotkeyOverviewPreviewData](preview.PreviewData)
		if err != nil {
			data = hotkeyOverviewPreviewData{}
		}
		return content(a.formatHotkeyOverview(data), palette.previewText), nil
	case "url":
		return content("URL preview\n\n"+preview.PreviewData+"\n\nThe embedded browser surface will be attached through the platform preview host.", palette.previewText), nil
	case "terminal":
		return a.buildTerminalPreview(a.terminalPreviewFor(preview), palette, width, height), nil
	case "webview":
		return a.buildWebViewPreview(preview.PreviewData, palette, width, height), nil
	default:
		return content(preview.PreviewData, palette.previewText), nil
	}
}

func (a *App) buildScrollablePreviewText(scrollKey, value string, color woxui.Color, scrollPosition string, palette uiPalette, width, height float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-28)
	innerHeight := max(float32(0), height-24)
	style := woxui.TextStyle{Size: 13}
	layout := a.previewTextLayout(scrollKey, value, style, innerWidth, 19)
	contentHeight := max(innerHeight, layout.Size.Height)
	maxOffset := max(float32(0), contentHeight-innerHeight)
	a.mu.Lock()
	offset, initialized := a.previewScroll[scrollKey]
	if !initialized && scrollPosition == "bottom" {
		offset = maxOffset
		a.previewScroll[scrollKey] = offset
	}
	offset = min(max(float32(0), offset), maxOffset)
	a.mu.Unlock()
	return woxwidget.Container{
		Width: width, Height: height, Radius: 10, Color: palette.queryBackground,
		Padding: woxwidget.Insets{Left: 14, Top: 12, Right: 14, Bottom: 12},
		Child: woxwidget.Gesture{
			ID: "preview-scroll-" + scrollKey,
			OnScroll: func(delta woxui.Point) {
				a.scrollPreview(scrollKey, -delta.Y, maxOffset)
			},
			Child: woxwidget.ScrollView{
				Width: innerWidth, Height: innerHeight, ContentHeight: contentHeight, Offset: offset,
				Child: woxwidget.TextBlock{Value: value, Width: innerWidth, Height: contentHeight, Style: style, LineHeight: 19, Color: color, Layout: &layout},
			},
		},
	}
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

func (a *App) scrollPreview(key string, delta, maxOffset float32) {
	if delta == 0 || maxOffset <= 0 {
		return
	}
	a.mu.Lock()
	a.previewScroll[key] = min(max(float32(0), a.previewScroll[key]+delta), maxOffset)
	if len(a.previewScroll) > 256 {
		current := a.previewScroll[key]
		a.previewScroll = map[string]float32{key: current}
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) buildPreviewImage(source, overlay woxImage, palette uiPalette, width, height float32) woxwidget.Widget {
	image := a.imageFor(source)
	if image == nil {
		message := "Loading image preview…"
		color := palette.resultSubtitle
		if imageErr := a.imageErrorFor(source); imageErr != "" {
			message = "Unable to decode image preview:\n" + imageErr
			color = woxui.Color{R: 232, G: 95, B: 95, A: 255}
		}
		return woxwidget.Container{
			Width: width, Height: height, Radius: 10, Color: palette.queryBackground, Padding: woxwidget.UniformInsets(14),
			Child: woxwidget.TextBlock{Value: message, Width: max(float32(0), width-28), Height: max(float32(0), height-28), Style: woxui.TextStyle{Size: 13}, Color: color},
		}
	}
	availableWidth := max(float32(0), width-24)
	availableHeight := max(float32(0), height-24)
	scale := min(availableWidth/float32(image.Width), availableHeight/float32(image.Height))
	drawWidth := float32(image.Width) * scale
	drawHeight := float32(image.Height) * scale
	left := (width - drawWidth) * 0.5
	top := (height - drawHeight) * 0.5
	return woxwidget.Gesture{
		ID:    "preview-image-overlay",
		OnTap: func() { a.openPreviewImageOverlay(overlay) },
		Child: woxwidget.Container{Width: width, Height: height, Radius: 10, Color: palette.queryBackground, Child: woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
			{Left: left, Top: top, Child: woxwidget.Image{Source: image, Width: drawWidth, Height: drawHeight}},
		}}},
	}
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
	if len(data.Items) == 0 {
		return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: palette.queryBackground, Padding: woxwidget.UniformInsets(14), Child: woxwidget.Text{Value: "No items", Style: woxui.TextStyle{Size: 13}, Color: palette.resultSubtitle}}
	}
	const rowHeight = float32(54)
	visibleCount := min(len(data.Items), max(1, int(height/rowHeight)))
	rows := make([]woxwidget.Widget, 0, visibleCount)
	for index := 0; index < visibleCount; index++ {
		item := data.Items[index]
		var icon woxwidget.Widget = woxwidget.Container{Width: 30, Height: 30, Radius: 7, Color: resultColors[index%len(resultColors)]}
		if item.Icon != nil {
			if image := a.imageFor(*item.Icon); image != nil {
				icon = woxwidget.Image{Source: image, Width: 30, Height: 30}
			}
		}
		tailWidth := float32(0)
		var tail woxwidget.Widget
		if len(item.Tails) > 0 && item.Tails[0].Text != "" {
			tailWidth = 78
			tail = woxwidget.Container{Width: tailWidth, Height: 30, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Text{Value: item.Tails[0].Text, Style: woxui.TextStyle{Size: 11}, Color: palette.resultSubtitle}}
		}
		labelWidth := max(float32(40), width-30-tailWidth-58)
		rows = append(rows, woxwidget.Container{Width: max(float32(0), width-20), Height: rowHeight, Padding: woxwidget.Insets{Left: 10, Top: 10, Right: 10, Bottom: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
			icon,
			woxwidget.Container{Width: labelWidth, Height: 36, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 2, Children: []woxwidget.Widget{
				woxwidget.Text{Value: item.Title, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: palette.previewText},
				woxwidget.Text{Value: item.Subtitle, Style: woxui.TextStyle{Size: 11}, Color: palette.resultSubtitle},
			}}},
			tail,
		}}})
	}
	return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: palette.queryBackground, Padding: woxwidget.Insets{Top: 4}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}}
}

func (a *App) buildPreviewTags(tags []previewTag, palette uiPalette, width float32) woxwidget.Widget {
	children := make([]woxwidget.Widget, 0, len(tags))
	used := float32(0)
	for _, tag := range tags {
		label := a.translate(tag.Label)
		if strings.TrimSpace(label) == "" {
			continue
		}
		metrics, _ := a.window.MeasureText(label, woxui.TextStyle{Size: 11})
		chipWidth := min(max(float32(36), metrics.Size.Width+18), max(float32(36), width))
		if used > 0 && used+8+chipWidth > width {
			break
		}
		children = append(children, woxwidget.Container{Width: chipWidth, Height: 25, Radius: 12, Color: palette.queryBackground, Padding: woxwidget.Insets{Left: 9, Top: 6, Right: 9, Bottom: 5}, Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 11}, Color: palette.resultSubtitle}})
		used += chipWidth + 8
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: children}
}
