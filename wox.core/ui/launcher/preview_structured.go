package launcher

import (
	"encoding/json"
	"fmt"
	"strings"

	launcherview "wox/ui/launcher/view"
	woxwidget "wox/ui/widget"
)

type pluginDetailPreviewData struct {
	ID             string   `json:"Id"`
	Name           string   `json:"Name"`
	Description    string   `json:"Description"`
	Author         string   `json:"Author"`
	Version        string   `json:"Version"`
	Icon           woxImage `json:"Icon"`
	Website        string   `json:"Website"`
	Runtime        string   `json:"Runtime"`
	ScreenshotURLs []string `json:"ScreenshotUrls"`
}

// buildPluginDetailPreview resolves controller-owned images before rendering the pure metadata view.
func (a *App) buildPluginDetailPreview(data pluginDetailPreviewData, palette uiPalette, width, height float32) woxwidget.Widget {
	var screenshot woxwidget.Widget
	headerHeight := min(float32(108), height)
	if len(data.ScreenshotURLs) > 0 && height > headerHeight+20 {
		screenshotHeight := max(float32(0), height-headerHeight-10)
		source := woxImage{ImageType: "url", ImageData: data.ScreenshotURLs[0]}
		screenshot = a.buildPreviewImage(source, source, palette, width, screenshotHeight)
	}
	return launcherview.PluginDetailPreviewView(launcherview.PluginDetailPreviewProps{
		Width: width, Height: height, Theme: palette.componentTheme(), Name: data.Name, Description: data.Description,
		Author: data.Author, Version: data.Version, Runtime: data.Runtime, Website: data.Website,
		Icon: a.imageFor(data.Icon), HasIcon: data.Icon.ImageType != "" && data.Icon.ImageData != "", Screenshot: screenshot,
	})
}

type updatePreviewData struct {
	CurrentVersion    string `json:"currentVersion"`
	LatestVersion     string `json:"latestVersion"`
	ReleaseChannel    string `json:"releaseChannel"`
	ReleaseNotes      string `json:"releaseNotes"`
	DownloadURL       string `json:"downloadUrl"`
	Status            string `json:"status"`
	HasUpdate         bool   `json:"hasUpdate"`
	Error             string `json:"error"`
	AutoUpdateEnabled bool   `json:"autoUpdateEnabled"`
}

type aiStreamPreviewData struct {
	Answer         string `json:"answer"`
	Reasoning      string `json:"reasoning"`
	Status         string `json:"status"`
	StatusLabel    string `json:"statusLabel"`
	ReasoningTitle string `json:"reasoningTitle"`
	AnswerTitle    string `json:"answerTitle"`
}

type dictationHistoryPreviewData struct {
	RefinedText         string `json:"refinedText"`
	OriginalText        string `json:"originalText"`
	RefinedLabel        string `json:"refinedLabel"`
	OriginalLabel       string `json:"originalLabel"`
	StatusLabel         string `json:"statusLabel"`
	IsChanged           bool   `json:"isChanged"`
	RawAudioPath        string `json:"rawAudioPath"`
	ProcessedAudioPath  string `json:"processedAudioPath"`
	AudioLabel          string `json:"audioLabel"`
	RawAudioLabel       string `json:"rawAudioLabel"`
	ProcessedAudioLabel string `json:"processedAudioLabel"`
}

type hotkeyOverviewPreviewData struct {
	Search string `json:"search"`
}

type hotkeyOverviewEntry struct {
	shortcut string
	action   string
	detail   string
}

func decodeStructuredPreview[T any](value string) (T, error) {
	var data T
	if err := json.Unmarshal([]byte(value), &data); err != nil {
		return data, err
	}
	return data, nil
}

func previewTagsForValues(values ...string) []previewTag {
	tags := make([]previewTag, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			tags = append(tags, previewTag{Label: value})
		}
	}
	return tags
}

func formatUpdatePreview(data updatePreviewData) string {
	status := data.Status
	if !data.AutoUpdateEnabled {
		status = "Automatic updates are disabled"
	} else if !data.HasUpdate && status == "" {
		status = "Wox is up to date"
	}
	versions := strings.TrimSpace(data.CurrentVersion)
	if versions == "" {
		versions = data.LatestVersion
	} else if data.LatestVersion != "" && data.LatestVersion != data.CurrentVersion {
		versions += "  →  " + data.LatestVersion
	}
	parts := make([]string, 0, 4)
	if versions != "" {
		parts = append(parts, versions)
	}
	if status != "" {
		parts = append(parts, status)
	}
	if data.Error != "" {
		parts = append(parts, "Error\n"+data.Error)
	}
	if data.DownloadURL != "" {
		parts = append(parts, "Download\n"+data.DownloadURL)
	}
	if strings.TrimSpace(data.ReleaseNotes) != "" {
		parts = append(parts, "Release notes\n\n"+data.ReleaseNotes)
	}
	return strings.Join(parts, "\n\n")
}

func formatAIStreamPreview(data aiStreamPreviewData) string {
	parts := make([]string, 0, 2)
	if reasoning := strings.TrimSpace(data.Reasoning); reasoning != "" {
		title := strings.TrimSpace(data.ReasoningTitle)
		if title == "" {
			title = "Reasoning"
		}
		parts = append(parts, title+"\n\n"+reasoning)
	}
	answer := strings.TrimSpace(data.Answer)
	if answer == "" {
		answer = strings.TrimSpace(data.StatusLabel)
	}
	if answer == "" {
		answer = "Waiting for answer…"
	}
	if data.AnswerTitle != "" && len(parts) > 0 {
		answer = data.AnswerTitle + "\n\n" + answer
	}
	parts = append(parts, answer)
	return strings.Join(parts, "\n\n────────\n\n")
}

func formatDictationHistoryPreview(data dictationHistoryPreviewData) string {
	refinedLabel := strings.TrimSpace(data.RefinedLabel)
	if refinedLabel == "" {
		refinedLabel = "Result"
	}
	parts := []string{refinedLabel + "\n\n" + data.RefinedText}
	if strings.TrimSpace(data.OriginalText) != "" {
		label := strings.TrimSpace(data.OriginalLabel)
		if label == "" {
			label = "Original transcript"
		}
		parts = append(parts, label+"\n\n"+data.OriginalText)
	}
	if data.RawAudioPath != "" || data.ProcessedAudioPath != "" {
		label := strings.TrimSpace(data.AudioLabel)
		if label == "" {
			label = "Audio diagnostics"
		}
		audio := []string{label}
		if data.RawAudioPath != "" {
			audio = append(audio, fmt.Sprintf("%s\n%s", data.RawAudioLabel, data.RawAudioPath))
		}
		if data.ProcessedAudioPath != "" {
			audio = append(audio, fmt.Sprintf("%s\n%s", data.ProcessedAudioLabel, data.ProcessedAudioPath))
		}
		parts = append(parts, strings.Join(audio, "\n\n"))
	}
	return strings.Join(parts, "\n\n────────\n\n")
}

func hotkeyOverviewEntryMatches(entry hotkeyOverviewEntry, search string) bool {
	search = strings.ToLower(strings.TrimSpace(search))
	if search == "" {
		return true
	}
	return strings.Contains(strings.ToLower(entry.shortcut), search) || strings.Contains(strings.ToLower(entry.action), search) || strings.Contains(strings.ToLower(entry.detail), search)
}

// formatHotkeyOverview renders current settings and portable launcher commands instead of treating the preview's search-only payload as content.
func (a *App) formatHotkeyOverview(data hotkeyOverviewPreviewData) string {
	a.mu.RLock()
	settings := a.settings
	a.mu.RUnlock()
	type section struct {
		title   string
		entries []hotkeyOverviewEntry
	}
	sections := []section{
		{title: a.translate("i18n:ui_hotkey_overview_global"), entries: []hotkeyOverviewEntry{
			{shortcut: settings.MainHotkey, action: a.translate("i18n:ui_hotkey_overview_open_wox")},
			{shortcut: settings.SelectionHotkey, action: a.translate("i18n:ui_hotkey_overview_search_selection")},
		}},
		{title: a.translate("i18n:ui_hotkey_overview_launcher"), entries: []hotkeyOverviewEntry{
			{shortcut: "Ctrl/Cmd+J", action: a.translate("i18n:ui_hotkey_overview_more_actions")},
			{shortcut: "Ctrl/Cmd+F", action: a.translate("i18n:ui_hotkey_overview_filters")},
			{shortcut: "Ctrl/Cmd+U", action: a.translate("i18n:ui_hotkey_overview_attention")},
		}},
		{title: a.translate("i18n:ui_hotkey_overview_preview"), entries: []hotkeyOverviewEntry{
			{shortcut: "Ctrl/Cmd+B", action: a.translate("i18n:ui_hotkey_overview_preview_fullscreen")},
			{shortcut: "Ctrl/Cmd+Shift+F", action: a.translate("i18n:ui_hotkey_overview_preview_search")},
			{shortcut: "Ctrl/Cmd+L", action: a.translate("i18n:ui_hotkey_overview_file_preview_load")},
			{shortcut: "Ctrl/Cmd+R", action: a.translate("i18n:ui_hotkey_overview_webview_refresh")},
			{shortcut: "Ctrl/Cmd+[", action: a.translate("i18n:ui_hotkey_overview_webview_back")},
			{shortcut: "Ctrl/Cmd+]", action: a.translate("i18n:ui_hotkey_overview_webview_forward")},
			{shortcut: "Ctrl/Cmd+Alt+I", action: a.translate("i18n:ui_hotkey_overview_webview_inspector")},
		}},
	}
	queryHotkeys := section{title: a.translate("i18n:ui_hotkey_overview_query_hotkeys")}
	for _, item := range settings.QueryHotkeys {
		if item.Disabled || strings.TrimSpace(item.Hotkey) == "" || strings.TrimSpace(item.Query) == "" {
			continue
		}
		action := strings.TrimSpace(item.Name)
		if action == "" {
			action = item.Query
		}
		detail := ""
		if strings.TrimSpace(item.Query) != strings.TrimSpace(action) {
			detail = item.Query
		}
		queryHotkeys.entries = append(queryHotkeys.entries, hotkeyOverviewEntry{shortcut: item.Hotkey, action: action, detail: detail})
	}
	sections = append(sections, queryHotkeys)
	queryShortcuts := section{title: a.translate("i18n:ui_hotkey_overview_query_shortcuts")}
	for _, item := range settings.QueryShortcuts {
		if !item.Disabled && strings.TrimSpace(item.Shortcut) != "" && strings.TrimSpace(item.Query) != "" {
			queryShortcuts.entries = append(queryShortcuts.entries, hotkeyOverviewEntry{shortcut: item.Shortcut, action: item.Query})
		}
	}
	sections = append(sections, queryShortcuts)
	blocks := make([]string, 0, len(sections))
	for _, section := range sections {
		lines := make([]string, 0, len(section.entries)+1)
		for _, entry := range section.entries {
			if strings.TrimSpace(entry.shortcut) == "" || !hotkeyOverviewEntryMatches(entry, data.Search) {
				continue
			}
			line := entry.shortcut + "    " + entry.action
			if entry.detail != "" {
				line += "\n    " + entry.detail
			}
			lines = append(lines, line)
		}
		if len(lines) > 0 {
			blocks = append(blocks, section.title+"\n\n"+strings.Join(lines, "\n\n"))
		}
	}
	if len(blocks) == 0 {
		return "No shortcuts match the current search."
	}
	return strings.Join(blocks, "\n\n────────\n\n")
}
