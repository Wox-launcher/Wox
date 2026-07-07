package dictation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"wox/plugin"
)

type dictationStoreTestAPI struct {
	settings map[string]string
	saved    map[string]string
	logs     []string
	refresh  int
}

func newDictationStoreTestAPI() *dictationStoreTestAPI {
	return &dictationStoreTestAPI{
		settings: map[string]string{},
		saved:    map[string]string{},
	}
}

func (a *dictationStoreTestAPI) GetSetting(ctx context.Context, key string) string {
	return a.settings[key]
}

func (a *dictationStoreTestAPI) SaveSetting(ctx context.Context, key string, value string, isPlatformSpecific bool) {
	a.settings[key] = value
	a.saved[key] = value
}

func (a *dictationStoreTestAPI) Log(ctx context.Context, level plugin.LogLevel, msg string) {
	a.logs = append(a.logs, msg)
}

func (a *dictationStoreTestAPI) RefreshQuery(ctx context.Context, param plugin.RefreshQueryParam) {
	a.refresh++
}

func TestHistoryCorrectPreservesOriginalAndUsesUpdatedContent(t *testing.T) {
	ctx := context.Background()
	api := newDictationStoreTestAPI()
	store := newHistoryStoreWithAPI(api)
	store.records = []historyRecord{
		{
			ID:        "record-1",
			Content:   "Open the wolf console",
			Timestamp: 1000,
		},
	}

	record, err := store.correct(ctx, historyCorrectRequest{
		RecordID:        "record-1",
		PreviousContent: "Open the wolf console",
		SelectedText:    "wolf",
		ReplacementText: "Wox",
		UpdatedContent:  "Open the Wox console",
	})
	if err != nil {
		t.Fatalf("correct returned error: %v", err)
	}
	if record.OriginalContent != "Open the wolf console" {
		t.Fatalf("original content = %q, want initial content", record.OriginalContent)
	}
	if record.Content != "Open the Wox console" {
		t.Fatalf("content = %q, want corrected content", record.Content)
	}
	if len(record.Corrections) != 1 {
		t.Fatalf("correction count = %d, want 1", len(record.Corrections))
	}
	if record.Corrections[0].SelectedText != "wolf" || record.Corrections[0].ReplacementText != "Wox" {
		t.Fatalf("correction = %#v, want wolf -> Wox", record.Corrections[0])
	}
	if !strings.Contains(api.saved[settingKeyHistory], "Open the Wox console") {
		t.Fatalf("saved history does not contain updated content: %s", api.saved[settingKeyHistory])
	}

	result := store.buildHistoryResult(ctx, record, plugin.Query{})
	if result.Title != "Open the Wox console" {
		t.Fatalf("result title = %q, want corrected content", result.Title)
	}
	if result.Preview.PreviewType != plugin.WoxPreviewTypeDictationHistory {
		t.Fatalf("preview type = %q, want dictation history preview", result.Preview.PreviewType)
	}

	var preview dictationHistoryPreviewData
	if err := json.Unmarshal([]byte(result.Preview.PreviewData), &preview); err != nil {
		t.Fatalf("preview data is not json: %v", err)
	}
	if preview.RecordID != "record-1" || preview.OriginalContent != "Open the wolf console" || preview.Content != "Open the Wox console" {
		t.Fatalf("preview data = %#v, want corrected history payload", preview)
	}
}

func TestHistoryCorrectRejectsEmptyAndStaleRequests(t *testing.T) {
	ctx := context.Background()
	api := newDictationStoreTestAPI()
	store := newHistoryStoreWithAPI(api)
	store.records = []historyRecord{{ID: "record-1", Content: "hello wox", Timestamp: 1000}}

	tests := []struct {
		name string
		req  historyCorrectRequest
	}{
		{
			name: "empty selection",
			req: historyCorrectRequest{
				RecordID:        "record-1",
				PreviousContent: "hello wox",
				SelectedText:    " ",
				ReplacementText: "Wox",
				UpdatedContent:  "hello Wox",
			},
		},
		{
			name: "empty replacement",
			req: historyCorrectRequest{
				RecordID:        "record-1",
				PreviousContent: "hello wox",
				SelectedText:    "wox",
				ReplacementText: " ",
				UpdatedContent:  "hello ",
			},
		},
		{
			name: "stale previous content",
			req: historyCorrectRequest{
				RecordID:        "record-1",
				PreviousContent: "hello old wox",
				SelectedText:    "wox",
				ReplacementText: "Wox",
				UpdatedContent:  "hello Wox",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := store.correct(ctx, tt.req); err == nil {
				t.Fatalf("correct returned nil error")
			}
		})
	}

	if store.records[0].Content != "hello wox" {
		t.Fatalf("content changed after rejected request: %q", store.records[0].Content)
	}
}

func TestDictionaryStoreRecordsContextAndAppliesOnlyInMatchingContext(t *testing.T) {
	ctx := context.Background()
	api := newDictationStoreTestAPI()
	store := newDictionaryStore(api)

	if err := store.addOrUpdateCorrection(ctx, "Open the wolf console", "wolf", "Wox", 1000); err != nil {
		t.Fatalf("add correction returned error: %v", err)
	}
	if got := store.applyExact("Open the wolf console"); got != "Open the Wox console" {
		t.Fatalf("applyExact = %q, want exact replacement", got)
	}
	if got := store.applyExact("The wolf ran away"); got != "The wolf ran away" {
		t.Fatalf("applyExact replaced outside the saved context: %q", got)
	}
	if got := store.applyExact("wolfish should stay"); got != "wolfish should stay" {
		t.Fatalf("applyExact replaced inside a word: %q", got)
	}

	entries := store.activeEntries()
	if len(entries) != 1 {
		t.Fatalf("active entries = %d, want 1", len(entries))
	}
	if entries[0].Context != "Open the wolf console" || entries[0].WrongPhrase != "wolf" || entries[0].CorrectPhrase != "Wox" || entries[0].Source != dictionarySourceCorrection {
		t.Fatalf("entry = %#v, want correction dictionary entry", entries[0])
	}
	if !strings.Contains(api.saved[settingKeyDictionary], "wolf") {
		t.Fatalf("saved dictionary does not contain correction: %s", api.saved[settingKeyDictionary])
	}
}

func TestExtractCorrectionContextReturnsContainingSentence(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		selectedText string
		want         string
	}{
		{
			name:         "english sentence",
			content:      "Start here. Open the wolf console. End.",
			selectedText: "wolf",
			want:         "Open the wolf console.",
		},
		{
			name:         "chinese sentence",
			content:      "前一句。这里要认证这个用户。下一句。",
			selectedText: "认证",
			want:         "这里要认证这个用户。",
		},
		{
			name:         "fallback whole content",
			content:      "Open the wolf console",
			selectedText: "wolf",
			want:         "Open the wolf console",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractCorrectionContext(tt.content, tt.selectedText); got != tt.want {
				t.Fatalf("context = %q, want %q", got, tt.want)
			}
		})
	}
}
