package launcher

import (
	"encoding/json"
	"fmt"
	"strings"

	woxcomponent "wox/ui/launcher/component"
	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
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
	return previewview.PluginDetailPreviewView(previewview.PluginDetailPreviewProps{
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

// buildUpdatePreview maps update state and translated labels into the dedicated Flutter-aligned view.
func (a *App) buildUpdatePreview(id string, data updatePreviewData, palette uiPalette, width, height, imageScale float32) woxwidget.Widget {
	status := ""
	statusColor := woxui.Color{R: 76, G: 175, B: 80, A: 255}
	if !data.AutoUpdateEnabled {
		status = a.translate("i18n:plugin_update_status_auto_update_disabled")
		statusColor = woxui.Color{R: 255, G: 152, B: 0, A: 255}
	} else if !data.HasUpdate {
		version := strings.TrimSpace(data.LatestVersion)
		if version == "" {
			version = strings.TrimSpace(data.CurrentVersion)
		}
		if version == "" {
			status = a.translate("i18n:plugin_update_status_none")
		} else {
			status = fmt.Sprintf(a.translate("i18n:plugin_update_status_none_with_version"), version)
		}
	} else {
		current := strings.TrimSpace(data.CurrentVersion)
		latest := strings.TrimSpace(data.LatestVersion)
		unknown := a.translate("i18n:plugin_update_unknown")
		if current == "" {
			current = unknown
		}
		if latest == "" {
			latest = unknown
		}
		status = current + "\u2192" + latest
		statusColor = woxui.Color{R: 255, G: 152, B: 0, A: 255}
	}
	if data.AutoUpdateEnabled {
		switch strings.ToLower(data.Status) {
		case "error":
			statusColor = woxui.Color{R: 244, G: 67, B: 54, A: 255}
		case "downloading":
			statusColor = woxui.Color{R: 33, G: 150, B: 243, A: 255}
		}
	}
	title := a.translate("i18n:plugin_update_title")
	if data.HasUpdate {
		title = a.translate("i18n:plugin_doctor_version_update_available")
	}
	betaLabel := ""
	if strings.EqualFold(data.ReleaseChannel, "beta") {
		betaLabel = a.translate("i18n:plugin_update_release_channel_beta")
	}
	scale := a.densityMetrics.normalized().scale
	return previewview.UpdatePreviewView(previewview.UpdatePreviewProps{
		ID: id, Width: width, Height: height, Scale: scale, Theme: palette.componentTheme(), Title: title, Error: data.Error,
		BetaLabel: betaLabel, StatusLabel: status, StatusColor: statusColor, AutoUpdateEnabled: data.AutoUpdateEnabled,
		DisabledTitle: a.translate("i18n:plugin_update_auto_update_disabled_title"), DisabledDescription: a.translate("i18n:plugin_update_auto_update_disabled_desc"),
		DisabledAction: a.translate("i18n:plugin_update_action_enable_auto_update") + " (enter)", OnPrimaryAction: a.activateSelected,
		ReleaseNotes: data.ReleaseNotes, NoReleaseNotes: a.translate("i18n:plugin_update_no_release_notes"),
		SectionNew: a.translate("i18n:plugin_update_section_new"), SectionImprovements: a.translate("i18n:plugin_update_section_improvements"),
		SectionFixes: a.translate("i18n:plugin_update_section_fixes"), SectionChanged: a.translate("i18n:plugin_update_section_changed"),
		SectionRemoved: a.translate("i18n:plugin_update_section_removed"), SectionSecurity: a.translate("i18n:plugin_update_section_security"),
		MeasureText: func(value string, style woxui.TextStyle) float32 {
			metrics, _ := a.window.MeasureText(value, style)
			return metrics.Size.Width
		},
		RenderMarkdown: func(markdownID, value string, markdownWidth float32) woxwidget.Widget {
			return woxcomponent.WoxMarkdown(a.markdownProps(markdownID, value, "", palette, markdownWidth, imageScale))
		},
	})
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
		answer = "Waiting for answer\u2026"
	}
	if data.AnswerTitle != "" && len(parts) > 0 {
		answer = data.AnswerTitle + "\n\n" + answer
	}
	parts = append(parts, answer)
	return strings.Join(parts, "\n\n\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\n\n")
}

// buildHotkeyOverviewPreview prepares the live settings and built-in shortcuts for the pure preview view.
func (a *App) buildHotkeyOverviewPreview(data hotkeyOverviewPreviewData, palette uiPalette, width, height float32) woxwidget.Widget {
	settings := a.generalSettings.Data()
	entry := func(shortcut, action, scope, source, detail string, keyboard bool) previewview.HotkeyOverviewPreviewEntry {
		labels := []string{strings.TrimSpace(shortcut)}
		if keyboard {
			labels = formatHotkeyLabels(shortcut)
		}
		return previewview.HotkeyOverviewPreviewEntry{RawShortcut: strings.TrimSpace(shortcut), Labels: labels, Action: action, Detail: detail, Scope: scope, Source: source}
	}
	globalScope := a.translate("i18n:ui_hotkey_overview_global")
	launcherScope := a.translate("i18n:ui_hotkey_overview_launcher")
	previewScope := a.translate("i18n:ui_hotkey_overview_preview")
	settingSource := a.translate("i18n:ui_hotkey_overview_source_setting")
	builtinSource := a.translate("i18n:ui_hotkey_overview_source_builtin")
	userSource := a.translate("i18n:ui_hotkey_overview_source_user")
	sections := []previewview.HotkeyOverviewPreviewSection{
		{Title: globalScope, Entries: []previewview.HotkeyOverviewPreviewEntry{
			entry(settings.MainHotkey, a.translate("i18n:ui_hotkey_overview_open_wox"), globalScope, settingSource, "", true),
			entry(settings.SelectionHotkey, a.translate("i18n:ui_hotkey_overview_search_selection"), globalScope, settingSource, "", true),
		}},
		{Title: launcherScope, Entries: []previewview.HotkeyOverviewPreviewEntry{
			entry(primaryHotkey("j"), a.translate("i18n:ui_hotkey_overview_more_actions"), launcherScope, builtinSource, "", true),
			entry(primaryHotkey("f"), a.translate("i18n:ui_hotkey_overview_filters"), launcherScope, builtinSource, "", true),
			entry(primaryHotkey("u"), a.translate("i18n:ui_hotkey_overview_attention"), launcherScope, builtinSource, "", true),
		}},
		{Title: previewScope, Entries: []previewview.HotkeyOverviewPreviewEntry{
			entry(primaryHotkey("b"), a.translate("i18n:ui_hotkey_overview_preview_fullscreen"), previewScope, builtinSource, "", true),
			entry(primaryHotkey("shift+f"), a.translate("i18n:ui_hotkey_overview_preview_search"), previewScope, builtinSource, "", true),
			entry(primaryHotkey("l"), a.translate("i18n:ui_hotkey_overview_file_preview_load"), previewScope, builtinSource, "", true),
			entry(primaryHotkey("r"), a.translate("i18n:ui_hotkey_overview_webview_refresh"), previewScope, builtinSource, "", true),
			entry(primaryHotkey("["), a.translate("i18n:ui_hotkey_overview_webview_back"), previewScope, builtinSource, "", true),
			entry(primaryHotkey("]"), a.translate("i18n:ui_hotkey_overview_webview_forward"), previewScope, builtinSource, "", true),
		}},
	}
	queryHotkeys := previewview.HotkeyOverviewPreviewSection{Title: a.translate("i18n:ui_hotkey_overview_query_hotkeys")}
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
		queryHotkeys.Entries = append(queryHotkeys.Entries, entry(item.Hotkey, action, queryHotkeys.Title, userSource, detail, true))
	}
	sections = append(sections, queryHotkeys)
	queryShortcuts := previewview.HotkeyOverviewPreviewSection{Title: a.translate("i18n:ui_hotkey_overview_query_shortcuts")}
	for _, item := range settings.QueryShortcuts {
		if !item.Disabled && strings.TrimSpace(item.Shortcut) != "" && strings.TrimSpace(item.Query) != "" {
			queryShortcuts.Entries = append(queryShortcuts.Entries, entry(item.Shortcut, item.Query, queryShortcuts.Title, userSource, "", false))
		}
	}
	sections = append(sections, queryShortcuts)
	return previewview.HotkeyOverviewPreviewView(previewview.HotkeyOverviewPreviewProps{
		Width: width, Height: height, Scale: a.densityMetrics.normalized().scale, Search: data.Search,
		Title: a.translate("i18n:ui_hotkey_overview_title"), Subtitle: a.translate("i18n:ui_hotkey_overview_subtitle"),
		Count: a.translate("i18n:ui_hotkey_overview_count"), Empty: a.translate("i18n:ui_hotkey_overview_empty"), Sections: sections, Theme: palette.componentTheme(),
	})
}
