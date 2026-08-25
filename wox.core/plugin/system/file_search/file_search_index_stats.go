package system

import (
	"context"
	"fmt"
	"time"
	"wox/plugin"
	"wox/setting/definition"
	"wox/util/filesearch"
)

// dynamicIndexStatsSetting fills the settings-page index card from cheap engine
// stats instead of the full `f status` diagnostic snapshot.
func (c *FileSearchPlugin) dynamicIndexStatsSetting(ctx context.Context, key string) definition.PluginSettingDefinitionItem {
	if key != fileIndexStatsSettingKey {
		return definition.PluginSettingDefinitionItem{}
	}

	stats := filesearch.IndexStatsSnapshot{}
	if c.engine != nil {
		statsCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		loaded, err := c.engine.GetIndexStats(statsCtx)
		if err != nil {
			c.api.Log(ctx, plugin.LogLevelWarning, "Failed to load file search index stats: "+err.Error())
			stats.Error = err.Error()
		} else {
			stats = loaded
		}
	}

	var contentStats *filesearch.ContentStats
	if c.engine != nil && c.isContentSearchEnabled(ctx) {
		loaded, err := c.engine.ContentStats(ctx)
		if err == nil {
			contentStats = &loaded
		}
	}

	return buildFileSearchIndexStatsSetting(ctx, c.api, stats, contentStats)
}

// buildFileSearchIndexStatsSetting maps persisted index volume into the settings card.
func buildFileSearchIndexStatsSetting(ctx context.Context, api plugin.API, stats filesearch.IndexStatsSnapshot, contentStats *filesearch.ContentStats) definition.PluginSettingDefinitionItem {
	directoryCount := stats.EntryCount - stats.FileCount
	if directoryCount < 0 {
		directoryCount = 0
	}
	unavailable := api.GetTranslation(ctx, "plugin_file_setting_index_stats_unavailable")
	diskUsage := formatFileSearchBytes(stats.DiskBytes)
	if stats.DiskBytes <= 0 && stats.Error != "" {
		diskUsage = unavailable
	}
	countsAvailable := stats.Error == ""
	rows := []definition.PluginSettingValueStatsRow{
		{Label: "i18n:plugin_file_setting_index_stats_disk_usage", Value: diskUsage},
		{Label: "i18n:plugin_file_setting_index_stats_total_entries", Value: formatFileSearchIndexStatsCount(stats.EntryCount, countsAvailable, unavailable)},
		{Label: "i18n:plugin_file_setting_index_stats_files", Value: formatFileSearchIndexStatsCount(stats.FileCount, countsAvailable, unavailable)},
		{Label: "i18n:plugin_file_setting_index_stats_directories", Value: formatFileSearchIndexStatsCount(directoryCount, countsAvailable, unavailable)},
		{Label: "i18n:plugin_file_setting_index_stats_last_duration", Value: formatFileSearchIndexStatsDuration(ctx, api, stats.LastElapsedMs)},
	}
	if contentStats != nil {
		rows = append(rows,
			definition.PluginSettingValueStatsRow{
				Label: "i18n:plugin_file_setting_index_stats_content_documents",
				Value: formatFileSearchCount(int64(contentStats.DocCount)),
			},
			definition.PluginSettingValueStatsRow{
				Label: "i18n:plugin_file_setting_index_stats_content_size",
				Value: formatFileSearchBytes(contentStats.IndexedTextBytes),
			},
		)
	}

	return definition.PluginSettingDefinitionItem{
		Type: definition.PluginSettingDefinitionTypeStats,
		Value: &definition.PluginSettingValueStats{
			Key:     fileIndexStatsSettingKey,
			Title:   "i18n:plugin_file_setting_index_stats_title",
			Tooltip: "i18n:plugin_file_setting_index_stats_tooltip",
			Rows:    rows,
		},
		SearchAliases: []string{
			"i18n:plugin_file_setting_index_stats_title",
			"index",
			"stats",
		},
	}
}

func formatFileSearchIndexStatsCount(value int64, available bool, unavailable string) string {
	if !available {
		return unavailable
	}
	return formatFileSearchCount(value)
}

// formatFileSearchBytes renders on-disk index size with one decimal when needed.
func formatFileSearchBytes(value int64) string {
	if value < 0 {
		value = 0
	}
	const unit = int64(1024)
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	div, exp := unit, 0
	for n := value / unit; n >= unit && exp < 4; n /= unit {
		div *= unit
		exp++
	}
	scaled := float64(value) / float64(div)
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	if scaled == float64(int64(scaled)) {
		return fmt.Sprintf("%.0f %s", scaled, units[exp])
	}
	return fmt.Sprintf("%.1f %s", scaled, units[exp])
}

// formatFileSearchIndexStatsDuration keeps millisecond precision for short runs
// and reuses the toolbar minute/hour buckets for longer ones.
func formatFileSearchIndexStatsDuration(ctx context.Context, api plugin.API, elapsedMs int64) string {
	if elapsedMs <= 0 {
		return api.GetTranslation(ctx, "plugin_file_setting_index_stats_unavailable")
	}
	if elapsedMs < 1000 {
		return fmt.Sprintf(api.GetTranslation(ctx, "plugin_file_setting_index_stats_duration_ms"), elapsedMs)
	}
	if elapsedMs < 60_000 {
		return fmt.Sprintf(api.GetTranslation(ctx, "plugin_file_setting_index_stats_duration_seconds_ms"), elapsedMs/1000, elapsedMs%1000)
	}

	seconds := (elapsedMs + 999) / 1000
	minutes := seconds / 60
	remainingSeconds := seconds % 60
	if minutes < 60 {
		return fmt.Sprintf(api.GetTranslation(ctx, "plugin_file_status_index_duration_minutes"), minutes, remainingSeconds)
	}
	hours := minutes / 60
	remainingMinutes := minutes % 60
	return fmt.Sprintf(api.GetTranslation(ctx, "plugin_file_status_index_duration_hours"), hours, remainingMinutes)
}
