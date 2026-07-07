package dictation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/util"
	"wox/util/clipboard"
	"wox/util/keyboard"
	"wox/util/window"

	"github.com/google/uuid"
)

// settingKeyHistory is the plugin setting key under which dictation history is
// persisted as a JSON array. Storing it as a regular plugin setting makes the
// history ride on cloud sync for free, matching how other plugins keep small
// user state (e.g. UrlPlugin.recentUrls, Emoji.frequentlyUsed).
const settingKeyHistory = "history"

// historyMaxRecords caps the number of retained transcripts. 500 is large
// enough for typical daily usage while keeping the stored JSON payload small.
const historyMaxRecords = 500

// historyTitleMaxRunes truncates the Title shown in the result list so long
// transcripts do not break list layout. The full text is still available in
// the preview pane.
const historyTitleMaxRunes = 80

// historyRecord is one persisted dictation transcript.
type historyRecord struct {
	ID              string              `json:"id"`
	OriginalContent string              `json:"originalContent,omitempty"`
	Content         string              `json:"content"`
	Timestamp       int64               `json:"timestamp"` // unix millis, matches util.FormatTimestamp
	Corrections     []historyCorrection `json:"corrections,omitempty"`
}

// historyCorrection records one user-approved preview correction.
type historyCorrection struct {
	SelectedText    string `json:"selectedText"`
	ReplacementText string `json:"replacementText"`
	PreviousContent string `json:"previousContent"`
	UpdatedContent  string `json:"updatedContent"`
	Timestamp       int64  `json:"timestamp"`
}

// dictationHistoryPreviewData is the JSON payload for the dedicated Flutter
// preview that owns inline correction UI.
type dictationHistoryPreviewData struct {
	RecordID        string              `json:"recordId"`
	OriginalContent string              `json:"originalContent"`
	Content         string              `json:"content"`
	Timestamp       int64               `json:"timestamp"`
	Corrections     []historyCorrection `json:"corrections,omitempty"`
}

type historyCorrectRequest struct {
	RecordID        string
	PreviousContent string
	SelectedText    string
	ReplacementText string
	UpdatedContent  string
}

// historyStore keeps the in-memory copy of the dictation history and guards it
// with a mutex. Reads come from memory; writes persist to plugin settings.
type historyStore struct {
	mu      sync.Mutex
	records []historyRecord
	api     dictationHistoryAPI
}

func newHistoryStore(api plugin.API) *historyStore {
	return newHistoryStoreWithAPI(api)
}

func newHistoryStoreWithAPI(api dictationHistoryAPI) *historyStore {
	return &historyStore{api: api}
}

// load reads the persisted history from plugin settings into memory. A
// corrupt or missing value resets to an empty slice instead of failing Init.
func (h *historyStore) load(ctx context.Context) {
	raw := h.api.GetSetting(ctx, settingKeyHistory)
	if strings.TrimSpace(raw) == "" {
		h.records = nil
		return
	}

	var records []historyRecord
	if err := json.Unmarshal([]byte(raw), &records); err != nil {
		h.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to parse dictation history: %s", err.Error()))
		h.records = nil
		return
	}
	h.records = records
}

// save serializes the in-memory history and persists it via plugin settings.
// Failures are logged but do not surface to the caller; history is best-effort
// and must never block the typing output path.
func (h *historyStore) save(ctx context.Context) {
	data, err := json.Marshal(h.records)
	if err != nil {
		h.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to marshal dictation history: %s", err.Error()))
		return
	}
	h.api.SaveSetting(ctx, settingKeyHistory, string(data), false)
}

// correct applies one preview correction when the submitted base content still
// matches the stored record, preventing stale edits from overwriting newer ones.
func (h *historyStore) correct(ctx context.Context, req historyCorrectRequest) (historyRecord, error) {
	req.RecordID = strings.TrimSpace(req.RecordID)
	if req.RecordID == "" {
		return historyRecord{}, fmt.Errorf("recordId is required")
	}
	if strings.TrimSpace(req.SelectedText) == "" {
		return historyRecord{}, fmt.Errorf("selectedText is required")
	}
	if strings.TrimSpace(req.ReplacementText) == "" {
		return historyRecord{}, fmt.Errorf("replacementText is required")
	}
	if req.UpdatedContent == "" {
		return historyRecord{}, fmt.Errorf("updatedContent is required")
	}
	if req.PreviousContent == req.UpdatedContent {
		return historyRecord{}, fmt.Errorf("updatedContent must be different")
	}
	if !strings.Contains(req.PreviousContent, req.SelectedText) {
		return historyRecord{}, fmt.Errorf("selectedText is not part of previousContent")
	}

	h.mu.Lock()
	index := -1
	for i := range h.records {
		if h.records[i].ID == req.RecordID {
			index = i
			break
		}
	}
	if index == -1 {
		h.mu.Unlock()
		return historyRecord{}, fmt.Errorf("dictation history record not found")
	}
	if h.records[index].Content != req.PreviousContent {
		h.mu.Unlock()
		return historyRecord{}, fmt.Errorf("dictation history content changed")
	}

	record := h.records[index]
	if record.OriginalContent == "" {
		record.OriginalContent = req.PreviousContent
	}
	record.Content = req.UpdatedContent
	record.Corrections = append(record.Corrections, historyCorrection{
		SelectedText:    req.SelectedText,
		ReplacementText: req.ReplacementText,
		PreviousContent: req.PreviousContent,
		UpdatedContent:  req.UpdatedContent,
		Timestamp:       util.GetSystemTimestamp(),
	})
	h.records[index] = record
	h.mu.Unlock()

	h.save(ctx)
	return record, nil
}

// add prepends a new record, trims to the cap, and persists. New records go to
// the front so the in-memory slice stays newest-first, matching the order the
// Query surface renders.
func (h *historyStore) add(ctx context.Context, content string, timestamp int64) {
	if strings.TrimSpace(content) == "" {
		return
	}

	record := historyRecord{
		ID:        uuid.NewString(),
		Content:   content,
		Timestamp: timestamp,
	}

	h.mu.Lock()
	h.records = append([]historyRecord{record}, h.records...)
	if len(h.records) > historyMaxRecords {
		h.records = h.records[:historyMaxRecords]
	}
	h.mu.Unlock()

	h.save(ctx)
}

// remove deletes a record by id and persists the result.
func (h *historyStore) remove(ctx context.Context, id string) {
	h.mu.Lock()
	kept := h.records[:0]
	for _, r := range h.records {
		if r.ID != id {
			kept = append(kept, r)
		}
	}
	h.records = kept
	h.mu.Unlock()

	h.save(ctx)
}

// snapshot returns a copy of the current history filtered by the search term.
// Empty search returns everything. Results are newest-first because the
// in-memory slice is maintained in that order.
func (h *historyStore) snapshot(search string) []historyRecord {
	search = strings.TrimSpace(strings.ToLower(search))

	h.mu.Lock()
	defer h.mu.Unlock()

	if search == "" {
		out := make([]historyRecord, len(h.records))
		copy(out, h.records)
		return out
	}

	out := make([]historyRecord, 0, len(h.records))
	for _, r := range h.records {
		if strings.Contains(strings.ToLower(r.Content), search) {
			out = append(out, r)
		}
	}
	return out
}

// recentContextMaxRecords caps how many prior transcripts are fed to the AI
// refiner as context. Five sentences is enough for the model to pick up on
// topic and tone without bloating the prompt.
const recentContextMaxRecords = 5

// recentContextWindow is the maximum age (from now) of a record that still
// counts as context. Anything older is treated as unrelated to the current
// dictation session.
const recentContextWindow = 10 * time.Minute

// recentContext returns the finalized transcripts (after AI refinement) from
// the last 10 minutes, up to recentContextMaxRecords, oldest-first so the AI
// reads them in chronological order. The current utterance is not yet in the
// store when this is called, so it is never included.
func (h *historyStore) recentContext(nowMillis int64) []string {
	cutoff := nowMillis - recentContextWindow.Milliseconds()

	h.mu.Lock()
	defer h.mu.Unlock()

	var picked []historyRecord
	for _, r := range h.records {
		if r.Timestamp < cutoff {
			continue
		}
		picked = append(picked, r)
		if len(picked) >= recentContextMaxRecords {
			break
		}
	}

	// h.records is newest-first; reverse to oldest-first for the AI prompt.
	out := make([]string, 0, len(picked))
	for i := len(picked) - 1; i >= 0; i-- {
		out = append(out, picked[i].Content)
	}
	return out
}

// isEmpty reports whether the history has any records.
func (h *historyStore) isEmpty() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.records) == 0
}

// buildHistoryResults converts history records into QueryResults. Each result
// groups by local calendar day (today / yesterday / older) and carries copy +
// paste-to-active-window + delete actions, mirroring the clipboard text-record
// action layout the user already knows.
func (h *historyStore) buildHistoryResults(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	records := h.snapshot(query.Search)
	results := make([]plugin.QueryResult, 0, len(records))

	for i := range records {
		results = append(results, h.buildHistoryResult(ctx, records[i], query))
	}
	return results
}

func (h *historyStore) buildHistoryResult(ctx context.Context, record historyRecord, query plugin.Query) plugin.QueryResult {
	group, groupScore := historyGroup(record.Timestamp)

	// Copy is the default action when no active window is available for paste.
	// When paste is possible, it takes over IsDefault and copy is demoted, so
	// the user's Enter key matches their current context.
	copyAction := plugin.QueryResultAction{
		Name:      "i18n:plugin_dictation_history_copy",
		Icon:      common.CopyIcon,
		IsDefault: true,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			if err := clipboard.WriteText(record.Content); err != nil {
				h.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to copy dictation history: id=%s err=%s", record.ID, err.Error()))
			}
		},
	}

	actions := []plugin.QueryResultAction{copyAction}

	// Paste to active window mirrors the clipboard plugin's text-record action.
	// Inlined here instead of calling system.GetPasteToActiveWindowAction to
	// avoid an import cycle (dictation -> system -> ui -> dictation).
	if pasteAction, ok := buildPasteToActiveWindowAction(ctx, h.api, query, record.Content, record.ID); ok {
		copyAction.IsDefault = false
		actions[0] = copyAction
		actions = append(actions, pasteAction)
	}

	actions = append(actions, plugin.QueryResultAction{
		Name: "i18n:plugin_dictation_history_delete",
		Icon: common.TrashIcon,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			h.remove(ctx, record.ID)
			h.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
		},
	})

	return plugin.QueryResult{
		Id:         record.ID,
		Title:      truncateHistoryTitle(record.Content),
		SubTitle:   util.FormatTimestamp(record.Timestamp),
		Icon:       dictationIcon,
		Group:      group,
		GroupScore: groupScore,
		Score:      record.Timestamp,
		Preview: plugin.WoxPreview{
			PreviewType: plugin.WoxPreviewTypeDictationHistory,
			PreviewData: record.previewData(ctx, h.api),
		},
		Actions: actions,
	}
}

func (r historyRecord) previewData(ctx context.Context, api dictationSettingAPI) string {
	originalContent := r.OriginalContent
	if originalContent == "" {
		originalContent = r.Content
	}

	data, err := json.Marshal(dictationHistoryPreviewData{
		RecordID:        r.ID,
		OriginalContent: originalContent,
		Content:         r.Content,
		Timestamp:       r.Timestamp,
		Corrections:     r.Corrections,
	})
	if err != nil {
		api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to marshal dictation history preview: %s", err.Error()))
		return r.Content
	}
	return string(data)
}

// historyEmptyResult is shown when the user opens the dictation query with no
// history and no search term, so the empty state is self-explanatory instead
// of a blank list.
func historyEmptyResult() plugin.QueryResult {
	return plugin.QueryResult{
		Title:    "i18n:plugin_dictation_history_empty_title",
		SubTitle: "i18n:plugin_dictation_history_empty_subtitle",
		Icon:     dictationIcon,
	}
}

// historyGroup buckets a timestamp into today / yesterday / older. The group
// score controls group ordering in the result list; today wins, then
// yesterday, then the catch-all history bucket. This mirrors the screenshot
// plugin's grouping behavior.
func historyGroup(timestampMs int64) (string, int64) {
	now := time.Now()
	itemTime := time.UnixMilli(timestampMs)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)

	if !itemTime.Before(today) {
		return "i18n:plugin_dictation_group_today", 90
	}
	if !itemTime.Before(yesterday) {
		return "i18n:plugin_dictation_group_yesterday", 80
	}
	return "i18n:plugin_dictation_group_history", 10
}

// truncateHistoryTitle shortens long transcripts for the list title. Subtitle
// still shows the timestamp and the preview pane shows the full text, so the
// truncation only protects list layout.
func truncateHistoryTitle(content string) string {
	if utf8.RuneCountInString(content) <= historyTitleMaxRunes {
		return content
	}
	runes := []rune(content)
	return string(runes[:historyTitleMaxRunes]) + "…"
}

// buildPasteToActiveWindowAction constructs a paste-to-foreground-window
// action for a history record. It returns (zero, false) when no active window
// is detected, so the caller can fall back to copy as the default action.
// Inlined here instead of calling system.GetPasteToActiveWindowAction to
// avoid an import cycle (dictation -> system -> ui -> dictation). The logic
// mirrors system.pasteToActiveWindow.
func buildPasteToActiveWindowAction(ctx context.Context, api dictationSettingAPI, query plugin.Query, text string, recordID string) (plugin.QueryResultAction, bool) {
	if strings.TrimSpace(query.Env.ActiveWindowTitle) == "" {
		return plugin.QueryResultAction{}, false
	}

	action := plugin.QueryResultAction{
		Name:      fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_paste_to_window"), query.Env.ActiveWindowTitle),
		IsDefault: true,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			if err := clipboard.WriteText(text); err != nil {
				api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to copy dictation history before paste: id=%s err=%s", recordID, err.Error()))
				return
			}
			util.Go(ctx, "dictation paste", func() {
				if query.Env.ActiveWindowPid > 0 {
					if !window.ActivateWindowByPid(query.Env.ActiveWindowPid) {
						api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("activate window failed, pid=%d", query.Env.ActiveWindowPid))
					}
					time.Sleep(150 * time.Millisecond)
				} else {
					time.Sleep(150 * time.Millisecond)
				}
				if err := keyboard.SimulatePaste(); err != nil {
					api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("simulate paste failed: %s", err.Error()))
				}
			})
		},
	}

	if !query.Env.ActiveWindowIcon.IsEmpty() {
		action.Icon = query.Env.ActiveWindowIcon
	}
	return action, true
}
