package system

import (
	"context"
	"testing"
	"wox/setting/definition"
	"wox/util/filesearch"
)

func TestBuildFileSearchIndexStatsSettingUsesPersistedVolume(t *testing.T) {
	item := buildFileSearchIndexStatsSetting(context.Background(), fileSearchToolbarTestAPI{}, filesearch.IndexStatsSnapshot{
		FileCount:     130945,
		EntryCount:    144571,
		DiskBytes:     30_834_688,
		LastElapsedMs: 6074,
	}, nil)
	if item.Type != definition.PluginSettingDefinitionTypeStats {
		t.Fatalf("setting type = %q, want stats", item.Type)
	}
	value, ok := item.Value.(*definition.PluginSettingValueStats)
	if !ok {
		t.Fatalf("setting value type = %T, want stats", item.Value)
	}
	if value.Key != fileIndexStatsSettingKey || value.Title != "i18n:plugin_file_setting_index_stats_title" {
		t.Fatalf("stats identity = %+v", value)
	}
	expected := []definition.PluginSettingValueStatsRow{
		{Label: "i18n:plugin_file_setting_index_stats_disk_usage", Value: "29.4 MB"},
		{Label: "i18n:plugin_file_setting_index_stats_total_entries", Value: "144,571"},
		{Label: "i18n:plugin_file_setting_index_stats_files", Value: "130,945"},
		{Label: "i18n:plugin_file_setting_index_stats_directories", Value: "13,626"},
		{Label: "i18n:plugin_file_setting_index_stats_last_duration", Value: "6s 74ms"},
	}
	if len(value.Rows) != len(expected) {
		t.Fatalf("stats rows = %#v", value.Rows)
	}
	for index, row := range expected {
		if value.Rows[index] != row {
			t.Fatalf("row %d = %+v, want %+v", index, value.Rows[index], row)
		}
	}
}

func TestBuildFileSearchIndexStatsSettingIncludesContentRows(t *testing.T) {
	item := buildFileSearchIndexStatsSetting(context.Background(), fileSearchToolbarTestAPI{}, filesearch.IndexStatsSnapshot{
		FileCount:  8,
		EntryCount: 10,
		DiskBytes:  2048,
	}, &filesearch.ContentStats{DocCount: 1500, IndexedTextBytes: 50 * 1024 * 1024})
	value := item.Value.(*definition.PluginSettingValueStats)
	if len(value.Rows) != 7 {
		t.Fatalf("stats row count = %d, want content rows appended", len(value.Rows))
	}
	if value.Rows[5].Label != "i18n:plugin_file_setting_index_stats_content_documents" || value.Rows[5].Value != "1,500" {
		t.Fatalf("content documents = %+v", value.Rows[5])
	}
	if value.Rows[6].Label != "i18n:plugin_file_setting_index_stats_content_size" || value.Rows[6].Value != "50 MB" {
		t.Fatalf("content size = %+v", value.Rows[6])
	}
}

func TestFormatFileSearchBytes(t *testing.T) {
	cases := []struct {
		value int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1 KB"},
		{1536, "1.5 KB"},
		{10 * 1024, "10 KB"},
		{30_834_688, "29.4 MB"},
	}
	for _, testCase := range cases {
		if got := formatFileSearchBytes(testCase.value); got != testCase.want {
			t.Fatalf("formatFileSearchBytes(%d) = %q, want %q", testCase.value, got, testCase.want)
		}
	}
}
