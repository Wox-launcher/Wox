package launcher

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"math"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"wox/common"
	"wox/common/icons"
	woxcomponent "wox/ui/launcher/component"
	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

// buildPreview resolves controller-owned preview state into a pure preview view.
func (a *App) buildPreview(result queryResult, palette uiPalette, width, height, imageScale float32) woxwidget.Widget {
	return a.buildPreviewWithChatHeader(result, palette, width, height, imageScale, true)
}

// buildPreviewWithChatHeader lets preview-only windows host chat navigation in their platform title bar.
func (a *App) buildPreviewWithChatHeader(result queryResult, palette uiPalette, width, height, imageScale float32, showChatHeader bool) woxwidget.Widget {
	preview := a.resolvePreview(result.Preview)
	if preview.PreviewType == "remote" {
		return woxwidget.Container{Width: width, Height: height}
	}
	if preview.PreviewType == "query_requirement_settings" {
		return a.buildRequirementPreview(result, preview, palette, width, height)
	}
	if preview.PreviewType == "trigger_keyword_conflict" {
		return a.buildTriggerConflictPreview(result, preview, palette, width, height)
	}
	if preview.PreviewType == "media" {
		data, err := decodeMediaPreview(preview.PreviewData)
		if err != nil {
			return previewview.PreviewError(fmt.Sprintf("Invalid media preview: %v", err), width, height, palette.componentTheme())
		}
		return a.buildMediaPreview(result, data, palette, width, height)
	}
	if preview.PreviewType == "chat" {
		return a.buildChatPreview(result, preview, palette, width, height, imageScale, showChatHeader)
	}
	scrollKey := result.QueryID + "\x00" + result.ID + "\x00" + preview.PreviewType
	if preview.PreviewType == "update" {
		data, err := decodeStructuredPreview[updatePreviewData](preview.PreviewData)
		if err != nil {
			return previewview.PreviewError(fmt.Sprintf("Invalid update preview: %v", err), width, height, palette.componentTheme())
		}
		return a.buildUpdatePreview(scrollKey, data, palette, width, height, imageScale)
	}
	tags := append(a.previewTags(preview.PreviewTags), a.previewTags(a.previewBodyTags(preview))...)
	if preview.PreviewType == "terminal" {
		return a.buildTerminalPreview(a.terminalPreviewSnapshotFor(preview), palette, width, height, imageScale, tags)
	}
	layout := previewview.ResolvePreviewLayout(width, height, len(tags) > 0)
	body := a.buildPreviewBody(scrollKey, preview, palette, layout.BodyWidth, layout.BodyHeight, imageScale)
	return previewview.PreviewView(previewview.PreviewProps{
		Width: width, Height: height, Tags: tags, Body: body, Theme: palette.componentTheme(), Window: a.window, OnTagHover: a.setPreviewTooltip,
	})
}

func (a *App) buildPreviewBody(scrollKey string, preview queryPreview, palette uiPalette, width, height, imageScale float32) woxwidget.Widget {
	a.releasePinnedPreviewImage()
	content := func(value string, color woxui.Color) woxwidget.Widget {
		if strings.TrimSpace(value) == "" {
			value = "No preview available"
		}
		return a.buildScrollablePreviewText(scrollKey, value, color, preview.ScrollPosition, width, height, palette.componentTheme())
	}
	errorText := palette.componentTheme().ErrorText
	switch preview.PreviewType {
	case "text":
		return a.buildTextPreview(scrollKey, preview.PreviewData, preview.ScrollPosition, palette, width, height)
	case "markdown":
		return a.buildMarkdownPreview(scrollKey, preview.PreviewData, "", preview.ScrollPosition, palette, width, height, imageScale)
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
		case "loading":
			return previewview.PreviewLoading(width, height, palette.componentTheme().PreviewText)
		case "markdown":
			return a.buildMarkdownPreview(scrollKey, a.filePreviewDisplayText(file), filepath.Dir(preview.PreviewData), preview.ScrollPosition, palette, width, height, imageScale)
		case "webview":
			return a.buildWebViewPreview(file.WebViewData, palette, width, height)
		case "native_file":
			return a.buildNativeFilePreview(file.NativeFilePath, file.NativeFileAutoLoad, palette, width, height)
		case "large", "too_large":
			return a.buildLargeFilePreview(file, palette, width, height)
		case "folder":
			return a.buildFolderPreview(file, palette, width, height, imageScale)
		default:
			// File contents are structured reader data, so keep them top-left aligned instead of using the centered quote treatment for standalone text previews.
			return content(a.filePreviewDisplayText(file), previewColorWithOpacity(palette.previewText, 0.86))
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
		return a.buildDictationHistoryPreview(scrollKey, data, palette, width, height)
	case "hotkey_overview":
		data, err := decodeStructuredPreview[hotkeyOverviewPreviewData](preview.PreviewData)
		if err != nil {
			data = hotkeyOverviewPreviewData{}
		}
		return a.buildHotkeyOverviewPreview(data, palette, width, height)
	case "url":
		return content("URL preview\n\n"+preview.PreviewData+"\n\nThe embedded browser surface will be attached through the platform preview host.", palette.previewText)
	case "webview":
		return a.buildWebViewPreview(preview.PreviewData, palette, width, height)
	default:
		return content(preview.PreviewData, palette.previewText)
	}
}

// buildMarkdownPreview injects launcher-owned image and external-link actions into the shared native component.
func (a *App) buildMarkdownPreview(scrollKey, value, baseDirectory, scrollPosition string, palette uiPalette, width, height, imageScale float32) woxwidget.Widget {
	initialOffset := float32(0)
	if scrollPosition == "bottom" {
		initialOffset = float32(math.MaxFloat32)
	}
	markdown := a.markdownProps(scrollKey, value, baseDirectory, palette, max(float32(0), width-40), imageScale)
	return previewview.MarkdownPreviewView(previewview.MarkdownPreviewProps{
		ID: scrollKey, Document: markdown.Document, Width: width, Height: height, InitialOffset: initialOffset, Theme: palette.componentTheme(), Window: a.window,
		ResolveImage: markdown.ResolveImage, ReleaseImage: markdown.ReleaseImage, OnOpenImage: markdown.OnOpenImage, OnOpenLink: markdown.OnOpenLink,
	})
}

// markdownProps centralizes the image and link actions shared by generic and structured Markdown previews.
func (a *App) markdownProps(id, value, baseDirectory string, palette uiPalette, width, imageScale float32) woxcomponent.MarkdownProps {
	return a.markdownPropsWithDocument(id, a.markdownDocument(value), baseDirectory, palette, width, imageScale)
}

// markdownPropsWithDocument shares actions without reparsing a chat's retained document.
func (a *App) markdownPropsWithDocument(id string, document woxcomponent.MarkdownDocument, baseDirectory string, palette uiPalette, width, imageScale float32) woxcomponent.MarkdownProps {
	resolveSource := func(source string) (woxImage, bool) {
		trimmed := strings.TrimSpace(source)
		if trimmed == "" {
			return woxImage{}, false
		}
		if parsed, err := url.Parse(trimmed); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
			return woxImage{ImageType: "url", ImageData: parsed.String()}, true
		}
		if strings.HasPrefix(trimmed, "file://") {
			parsed, err := url.Parse(trimmed)
			if err != nil {
				return woxImage{}, false
			}
			path, err := url.PathUnescape(parsed.Path)
			if err != nil || path == "" {
				return woxImage{}, false
			}
			return woxImage{ImageType: "absolute", ImageData: filepath.Clean(path)}, true
		}
		if filepath.IsAbs(trimmed) {
			return woxImage{ImageType: "absolute", ImageData: filepath.Clean(trimmed)}, true
		}
		if baseDirectory != "" {
			return woxImage{ImageType: "absolute", ImageData: filepath.Join(baseDirectory, trimmed)}, true
		}
		return woxImage{}, false
	}
	return woxcomponent.MarkdownProps{
		ID: id, Document: document, Width: width, Theme: palette.componentTheme().Controls, Window: a.window,
		ResolveImage: func(source string) (*woxui.Image, string) {
			imageSource, ok := resolveSource(source)
			if !ok {
				return nil, "Unsupported Markdown image: " + source
			}
			return a.imageForViewport(imageSource, markdownImageRequestSize(width, imageScale)), a.imageErrorFor(imageSource)
		},
		ReleaseImage: func(source string) {
			imageSource, ok := resolveSource(source)
			if !ok {
				return
			}
			a.releaseViewportImage(imageSource, markdownImageRequestSize(width, imageScale))
		},
		OnOpenImage: func(source string) {
			if imageSource, ok := resolveSource(source); ok {
				a.openPreviewImageOverlay(imageSource)
			}
		},
		OnOpenLink: func(target string) {
			if err := a.window.OpenExternalURL(target); err != nil {
				log.Printf("open Markdown preview link: %v", err)
			}
		},
	}
}

// markdownImageRequestSize is the physical raster size for one Markdown picture.
func markdownImageRequestSize(width, imageScale float32) int {
	return min(2048, max(256, physicalImageSize(int(math.Ceil(float64(width))), imageScale)))
}

// markdownDocument bounds AST reuse so repeated frames do not reparse unchanged streaming content.
func (a *App) markdownDocument(value string) woxcomponent.MarkdownDocument {
	hash := sha256.Sum256([]byte(value))
	key := fmt.Sprintf("%x", hash)
	if document, ok := a.mdDocs[key]; ok {
		return document
	}
	document := woxcomponent.ParseMarkdown(value)
	if len(a.mdDocs) >= 64 {
		a.mdDocs = map[string]woxcomponent.MarkdownDocument{}
	}
	a.mdDocs[key] = document
	return document
}

// filePreviewDisplayText appends the Flutter-era limited-preview note after a bounded read.
func (a *App) filePreviewDisplayText(file filePreviewContent) string {
	text := file.Text
	if !file.Limited {
		return text
	}
	note := strings.ReplaceAll(a.translate("i18n:ui_file_preview_code_preview_limited"), "{lines}", strconv.Itoa(max(file.DisplayLines, 1)))
	if strings.TrimSpace(text) == "" {
		return note
	}
	return text + "\n\n" + note
}

// buildLargeFilePreview restores Flutter's details-first gate for expensive file previews.
func (a *App) buildLargeFilePreview(file filePreviewContent, palette uiPalette, width, height float32) woxwidget.Widget {
	sizeLabel := formatFileSize(file.Size)
	title := a.translate("i18n:ui_file_preview_large_file_title")
	message := strings.ReplaceAll(a.translate("i18n:ui_file_preview_large_file_message"), "{size}", sizeLabel)
	action := a.translate("i18n:ui_file_preview_load_full_preview")
	if hotkey := strings.Join(formatHotkeyLabels(primaryHotkey("l")), "+"); hotkey != "" {
		action = fmt.Sprintf("%s (%s)", action, hotkey)
	}
	onLoad := func() { a.requestManualFilePreview(file.Path) }
	if file.Kind == "too_large" {
		title = strings.ReplaceAll(a.translate("i18n:ui_file_preview_too_large"), "{size}", formatFileSizeMegabytes(file.Size))
		message = ""
		action = ""
		onLoad = nil
	}
	properties := []previewview.LargeFilePreviewProperty{
		{Label: a.translate("i18n:ui_file_preview_property_type"), Value: file.TypeLabel},
		{Label: a.translate("i18n:ui_file_preview_property_size"), Value: sizeLabel},
	}
	if !file.Modified.IsZero() {
		properties = append(properties, previewview.LargeFilePreviewProperty{Label: a.translate("i18n:ui_file_preview_property_modified"), Value: file.Modified.Format(time.DateTime)})
	}
	if location := filepath.Dir(file.Path); location != "" && location != "." {
		properties = append(properties, previewview.LargeFilePreviewProperty{Label: a.translate("i18n:ui_file_preview_property_location"), Value: location})
	}
	return previewview.LargeFilePreviewView(previewview.LargeFilePreviewProps{
		Width: width, Height: height, Theme: palette.componentTheme(), Title: title, Message: message, Properties: properties,
		Action: action, OnLoad: onLoad,
	})
}

// buildFolderPreview maps inspected directory metadata into the dedicated folder preview.
func (a *App) buildFolderPreview(file filePreviewContent, palette uiPalette, width, height, imageScale float32) woxwidget.Widget {
	folder := file.Folder
	metadata := make([]string, 0, 2)
	if modified := formatFolderPreviewTime(folder.Modified); modified != "" {
		metadata = append(metadata, modified)
	}
	if items := a.folderPreviewItemsValue(folder); items != "" {
		metadata = append(metadata, items)
	}
	entries := make([]previewview.FolderPreviewEntry, 0, len(folder.Entries))
	for _, entry := range folder.Entries {
		item := previewview.FolderPreviewEntry{Name: entry.Name, IsDir: entry.IsDir}
		if !entry.IsDir && entry.HasSize {
			item.Size = formatFileSize(entry.Size)
		}
		entries = append(entries, item)
	}
	more := ""
	if remaining := folderPreviewMoreCount(folder); remaining > 0 {
		more = strings.ReplaceAll(a.translate("i18n:ui_file_preview_folder_more_entries_not_shown"), "{count}", strconv.Itoa(remaining))
	}
	errorText := ""
	if folder.Error != "" {
		errorText = a.translate("i18n:ui_file_preview_folder_read_failed")
	}
	folderIcon := fromCoreImage(icons.Get(icons.PluginFolder))
	fileIcon := fromCoreImage(icons.Get(icons.PluginFile))
	return previewview.FolderPreviewView(previewview.FolderPreviewProps{
		Width: width, Height: height, Theme: palette.componentTheme(),
		Path: folder.Path, Name: folder.Name, Metadata: strings.Join(metadata, "  ·  "),
		Icon:       a.imageForSize(folderIcon, physicalImageSize(32, imageScale)),
		FolderIcon: a.imageForSize(folderIcon, physicalImageSize(20, imageScale)),
		FileIcon:   a.imageForSize(fileIcon, physicalImageSize(20, imageScale)),
		Entries:    entries, More: more,
		Empty: a.translate("i18n:ui_file_preview_folder_empty"), Error: errorText,
	})
}

// folderPreviewItemsValue joins the shallow folder/file counts, omitting empty and unreadable folders.
func (a *App) folderPreviewItemsValue(folder folderPreviewContent) string {
	if folder.Error != "" || (folder.FolderCount == 0 && folder.FileCount == 0) {
		return ""
	}
	parts := make([]string, 0, 2)
	if folder.FolderCount > 0 {
		parts = append(parts, strings.ReplaceAll(a.translate("i18n:ui_file_preview_folder_folders_count"), "{count}", strconv.Itoa(folder.FolderCount)))
	}
	if folder.FileCount > 0 {
		parts = append(parts, strings.ReplaceAll(a.translate("i18n:ui_file_preview_folder_files_count"), "{count}", strconv.Itoa(folder.FileCount)))
	}
	return strings.Join(parts, " · ")
}

// folderPreviewTags replaces the generic FILE/size chips with folder identity and item count.
func (a *App) folderPreviewTags(folder folderPreviewContent) []previewTag {
	tags := []previewTag{{
		Label:   a.translate("i18n:ui_file_preview_type_folder"),
		Tooltip: a.translate("i18n:ui_file_preview_property_type"),
	}}
	total := folder.FolderCount + folder.FileCount
	if folder.Error != "" || total == 0 {
		return tags
	}
	key := "i18n:ui_file_preview_folder_items_count"
	if !folder.CountedAll {
		key = "i18n:ui_file_preview_folder_items_count_limited"
	}
	tags = append(tags, previewTag{
		Label:   strings.ReplaceAll(a.translate(key), "{count}", strconv.Itoa(total)),
		Tooltip: a.translate("i18n:ui_file_preview_property_items"),
	})
	return tags
}

// previewBodyTags resolves metadata before the body is built at its final tagged height.
func (a *App) previewBodyTags(preview queryPreview) []previewTag {
	switch preview.PreviewType {
	case "file":
		file := a.filePreviewFor(preview.PreviewData)
		if file.Kind == "folder" {
			return a.folderPreviewTags(file.Folder)
		}
		return file.Tags
	case "ai_stream":
		data, err := decodeStructuredPreview[aiStreamPreviewData](preview.PreviewData)
		if err == nil {
			return previewTagsForValues(data.StatusLabel)
		}
	}
	return nil
}

// buildDictationHistoryPreview prepares portable text layout before composing the pure comparison view.
func (a *App) buildDictationHistoryPreview(scrollKey string, data dictationHistoryPreviewData, palette uiPalette, width, height float32) woxwidget.Widget {
	scale := a.densityMetrics.normalized().scale
	scaled := func(value float32) float32 { return a.densityMetrics.scaled(value) }
	innerWidth := max(float32(0), width-scaled(52))
	refinedWidth := max(float32(0), innerWidth-scaled(16))
	refinedStyle := woxui.TextStyle{Size: scaled(18), Weight: woxui.FontWeightSemibold}
	originalStyle := woxui.TextStyle{Size: scaled(14)}
	refinedLayout := a.previewTextLayout(scrollKey+"|dictation-refined", data.RefinedText, refinedStyle, refinedWidth, scaled(28))
	originalLayout := a.previewTextLayout(scrollKey+"|dictation-original", data.OriginalText, originalStyle, innerWidth, scaled(22))
	statusWidth := float32(0)
	if status := strings.TrimSpace(data.StatusLabel); status != "" {
		metrics, _ := a.window.MeasureText(status, woxui.TextStyle{Size: scaled(11)})
		statusWidth = scaled(17) + metrics.Size.Width
	}
	props := previewview.DictationHistoryPreviewProps{
		ID: scrollKey, Width: width, Height: height, Scale: scale, Theme: palette.componentTheme(),
		RefinedText: data.RefinedText, OriginalText: data.OriginalText, RefinedLabel: data.RefinedLabel, OriginalLabel: data.OriginalLabel,
		StatusLabel: data.StatusLabel, IsChanged: data.IsChanged, RefinedLayout: refinedLayout, OriginalLayout: originalLayout, StatusWidth: statusWidth,
		AudioLabel: data.AudioLabel, RawAudioLabel: data.RawAudioLabel, RawAudioPath: data.RawAudioPath,
		ProcessedAudioLabel: data.ProcessedAudioLabel, ProcessedAudioPath: data.ProcessedAudioPath,
		RawPlayback: dictationPlaybackProps(a.dictationAudioSnapshot(data.RawAudioPath)), ProcessedPlayback: dictationPlaybackProps(a.dictationAudioSnapshot(data.ProcessedAudioPath)),
		PlayLabel: a.translate("i18n:plugin_mediaplayer_play"), PauseLabel: a.translate("i18n:plugin_mediaplayer_pause"), OnPlayDiagnosticAudio: a.toggleDictationAudio,
	}
	if a.isDev && strings.TrimSpace(data.RawAudioPath) != "" && strings.TrimSpace(data.ProcessedAudioPath) != "" {
		compare := a.ensureDictationModelCompare(data.RawAudioPath)
		compareTextWidth := max(float32(0), innerWidth-scaled(28))
		props.CompareLabel = a.translate("i18n:plugin_dictation_history_model_compare")
		props.CompareAllLabel = a.translate("i18n:plugin_dictation_history_model_compare_run_all")
		props.CompareRunLabel = a.translate("i18n:plugin_dictation_history_model_compare_run")
		props.CompareRunningLabel = a.translate("i18n:plugin_dictation_history_model_compare_running")
		props.CompareEmptyLabel = a.translate("i18n:plugin_dictation_history_model_compare_empty")
		props.CompareModels = a.dictationModelCompareProps(compare, scrollKey, a.translate("i18n:plugin_dictation_history_model_compare_empty_result"), originalStyle, compareTextWidth, scaled(22))
		props.CompareBusy = compare.busy
		props.OnCompareModel = a.compareDictationModel
		props.OnCompareAll = a.compareAllDictationModels
	}
	return previewview.DictationHistoryPreviewView(props)
}

func (a *App) buildScrollablePreviewText(scrollKey, value string, color woxui.Color, scrollPosition string, width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
	fontSize := a.densityMetrics.scaled(woxcomponent.PreviewBodyFontSize)
	lineHeight := a.densityMetrics.scaled(23)
	initialOffset := float32(0)
	if scrollPosition == "bottom" {
		initialOffset = float32(math.MaxFloat32)
	}
	return previewview.ScrollablePreviewText(previewview.ScrollablePreviewTextProps{
		ID: scrollKey, Value: value, Color: color, Width: width, Height: height, FontSize: fontSize, LineHeight: lineHeight, InitialOffset: initialOffset,
		Window: a.window, Theme: theme,
	})
}

func (a *App) buildTextPreview(scrollKey, value, scrollPosition string, palette uiPalette, width, height float32) woxwidget.Widget {
	if strings.TrimSpace(value) == "" {
		value = "No preview available"
	}
	fontSize := a.densityMetrics.scaled(woxcomponent.PreviewQuoteFontSize)
	lineHeight := a.densityMetrics.scaled(25)
	style := woxui.TextStyle{Size: fontSize}
	theme := palette.componentTheme()
	if !previewview.TextPreviewFits(value, a.window, style, width, height, lineHeight) {
		return a.buildScrollablePreviewText(scrollKey, value, previewColorWithOpacity(palette.previewText, 0.86), scrollPosition, width, height, theme)
	}
	return previewview.TextPreview(previewview.TextPreviewProps{
		ID: scrollKey, Value: value, Width: width, Height: height, FontSize: fontSize, LineHeight: lineHeight, Theme: theme, Window: a.window,
	})
}

func (a *App) previewTextLayout(scrollKey, value string, style woxui.TextStyle, width, lineHeight float32) woxwidget.TextBlockLayout {
	// Cache by semantic source, so changing reasoning replaces its previous version.
	const maxSourceBytes = 2 << 20
	cached := a.previewLayouts[scrollKey]
	if cached == nil || cached.value != value {
		retained := len(value)
		for id, entry := range a.previewLayouts {
			if id != scrollKey {
				retained += len(entry.value)
			}
		}
		if retained > maxSourceBytes || (cached == nil && len(a.previewLayouts) >= 128) {
			clear(a.previewLayouts)
		}
	}
	if cached == nil {
		cached = &textLayoutCache{}
	}
	if len(value) <= maxSourceBytes {
		a.previewLayouts[scrollKey] = cached
	}
	font := ""
	if a.generalSettings != nil {
		font = a.generalSettings.Data().AppFontFamily
	}
	return cached.measure(value, textLayoutKey{session: scrollKey, font: font, window: a.window, width: width, lineHeight: lineHeight, style: style})
}

// releasePinnedPreviewImage drops the full-bleed preview pin when another preview is shown.
func (a *App) releasePinnedPreviewImage() {
	if a.pinnedPreview.size == 0 {
		return
	}
	a.releaseViewportImage(a.pinnedPreview.source, a.pinnedPreview.size)
	a.pinnedPreview = viewportPreviewPin{}
}

func (a *App) buildPreviewImage(source, overlay woxImage, palette uiPalette, width, height float32) woxwidget.Widget {
	size := previewImageRequestSize(width, height)
	a.pinnedPreview = viewportPreviewPin{source: source, size: size}
	image := a.imageForViewport(source, size)
	theme := palette.componentTheme()
	message := ""
	if image == nil {
		if imageErr := a.imageErrorFor(source); imageErr != "" {
			message = "Unable to decode image preview:\n" + imageErr
		}
	}
	var onTap func()
	if previewImageOverlayAllowed(a.show) {
		onTap = func() { a.openPreviewImageOverlay(overlay) }
	}
	return previewview.PreviewImage(previewview.PreviewImageProps{
		ID: imageKey(source), Width: width, Height: height, Image: image, Message: message, MessageColor: theme.ErrorText, LoadingColor: theme.PreviewText, OnTap: onTap,
	})
}

// previewImageOverlayAllowed keeps sticker overlays for sidebar and split image
// previews. Full preview-only windows already show the image at window size, so
// another tap should not open a second overlay.
func previewImageOverlayAllowed(show showAppParams) bool {
	return !(show.ShowPreviewTitleBar && show.HideQueryBox && show.HideToolbar)
}

func (a *App) openPreviewImageOverlay(image woxImage) {
	util.Go(a.lifecycleCtx, "open preview image overlay", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := a.services.ShowPreviewImage(ctx, a.sessionID, common.WoxImage{ImageType: image.ImageType, ImageData: image.ImageData}); err != nil {
			log.Printf("open preview image overlay: %v", err)
		}
	})
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

func (a *App) previewTags(tags []previewTag) []previewview.PreviewTag {
	resolved := make([]previewview.PreviewTag, 0, len(tags))
	for _, tag := range tags {
		label := strings.TrimSpace(a.translate(tag.Label))
		tooltip := strings.TrimSpace(a.translate(tag.Tooltip))
		if label == "" {
			label = tooltip
		}
		if label != "" {
			resolved = append(resolved, previewview.PreviewTag{Label: label, Tooltip: tooltip})
		}
	}
	return resolved
}

// setPreviewTooltip anchors preview and chat chrome help to the window that owns the hover.
func (a *App) setPreviewTooltip(inside bool, text string, anchor woxui.Rect) {
	a.setNativeHoverTooltip(&a.previewTooltipRevision, "go-ui-preview-tag", "update preview tag tooltip", inside, text, anchor, "top", a.previewTooltipWindow)
}

// previewTooltipWindow keeps dedicated-chat hover anchors on that window instead of the hidden launcher.
func (a *App) previewTooltipWindow() *woxui.Window {
	if window := a.chatNativeWindow(); window != nil && (a.chatWindowFocused || !a.visible) {
		return window
	}
	return a.window
}

func previewColorWithOpacity(color woxui.Color, opacity float32) woxui.Color {
	opacity = min(max(float32(0), opacity), float32(1))
	color.A = uint8(opacity*255 + 0.5)
	return color
}
