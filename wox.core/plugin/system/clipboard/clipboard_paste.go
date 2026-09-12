package system

import (
	"context"
	"fmt"
	"strings"
	"wox/common/icons"
	"wox/plugin"
	"wox/plugin/system"
	"wox/util"
	"wox/util/clipboard"

	"github.com/cdfmlr/ellipsis"
)

const (
	clipboardPasteCommand     = "paste"
	pasteHistorySkipWindowMs  = 2000
	pasteHistoryFallbackLimit = 5000
)

// resolvePasteCursor picks the record `cb paste` should show.
// An empty cursor means newest. A missing ID falls back to newest and clears the cursor.
func resolvePasteCursor(records []ClipboardRecord, cursorID string) (record ClipboardRecord, resolvedCursorID string, found bool) {
	if len(records) == 0 {
		return ClipboardRecord{}, "", false
	}
	if cursorID == "" {
		return records[0], "", true
	}
	for _, rec := range records {
		if rec.ID == cursorID {
			return rec, cursorID, true
		}
	}
	return records[0], "", true
}

// advancePasteCursor moves from the current item to the next older one.
// At the oldest item it stays there until a later reset.
func advancePasteCursor(records []ClipboardRecord, cursorID string) string {
	if len(records) == 0 {
		return ""
	}

	index := 0
	if cursorID != "" {
		index = -1
		for i, rec := range records {
			if rec.ID == cursorID {
				index = i
				break
			}
		}
		if index < 0 {
			index = 0
		}
	}
	if index+1 < len(records) {
		return records[index+1].ID
	}
	return records[index].ID
}

// nextPasteRecord returns the older item after the current cursor, if any.
func nextPasteRecord(records []ClipboardRecord, cursorID string) (ClipboardRecord, bool) {
	current, resolvedCursorID, found := resolvePasteCursor(records, cursorID)
	if !found {
		return ClipboardRecord{}, false
	}

	nextID := advancePasteCursor(records, resolvedCursorID)
	if nextID == "" || nextID == current.ID {
		return ClipboardRecord{}, false
	}
	for _, rec := range records {
		if rec.ID == nextID {
			return rec, true
		}
	}
	return ClipboardRecord{}, false
}

// clipboardRecordDescription is the compact title used for paste rows and the next-item subtitle.
func clipboardRecordDescription(record ClipboardRecord) string {
	if record.Alias != nil && strings.TrimSpace(*record.Alias) != "" {
		return strings.TrimSpace(*record.Alias)
	}
	if record.Type == string(clipboard.ClipboardTypeText) {
		return strings.TrimSpace(ellipsis.Centering(record.Content, 80))
	}
	return strings.TrimSpace(record.Content)
}

// shouldSkipPasteHistoryCapture reports whether a Watch callback is still inside
// the restore window and must not persist or reset the sequential paste cursor.
func shouldSkipPasteHistoryCapture(skipUntil int64, now int64) bool {
	return skipUntil > 0 && now < skipUntil
}

func (c *ClipboardPlugin) resetPasteCursor() {
	c.pasteMu.Lock()
	c.pasteCursorID = ""
	c.pasteMu.Unlock()
}

// markSkipHistoryCapture starts the short window that drops Watch persistence
// for the restore write so an older item cannot be inserted as a new row.
func (c *ClipboardPlugin) markSkipHistoryCapture() {
	c.pasteMu.Lock()
	c.skipHistoryUntil = util.GetSystemTimestamp() + pasteHistorySkipWindowMs
	c.pasteMu.Unlock()
}

func (c *ClipboardPlugin) clearSkipHistoryCapture() {
	c.pasteMu.Lock()
	c.skipHistoryUntil = 0
	c.pasteMu.Unlock()
}

func (c *ClipboardPlugin) shouldSkipHistoryCapture() bool {
	c.pasteMu.Lock()
	defer c.pasteMu.Unlock()
	return shouldSkipPasteHistoryCapture(c.skipHistoryUntil, util.GetSystemTimestamp())
}

// querySequentialPaste returns the single history item currently pointed to by cb paste.
func (c *ClipboardPlugin) querySequentialPaste(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	if c.db == nil {
		c.api.Log(ctx, plugin.LogLevelError, "database not initialized")
		return plugin.NewQueryResponse(nil)
	}

	records, err := c.pasteHistoryRecords(ctx)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to get clipboard history for paste: %s", err.Error()))
		return plugin.NewQueryResponse(nil)
	}

	c.pasteMu.Lock()
	record, resolvedCursorID, found := resolvePasteCursor(records, c.pasteCursorID)
	c.pasteCursorID = resolvedCursorID
	c.pasteMu.Unlock()
	if !found {
		return plugin.NewQueryResponse(nil)
	}

	result := c.convertRecordToResult(ctx, record, query)
	result.Actions = c.sequentialPasteActions(ctx, record, query)
	result.Group = ""
	result.GroupScore = 0
	result.Preview = plugin.WoxPreview{}
	if next, ok := nextPasteRecord(records, resolvedCursorID); ok {
		result.SubTitle = fmt.Sprintf(c.api.GetTranslation(ctx, "plugin_clipboard_paste_next"), clipboardRecordDescription(next))
	}
	return plugin.NewQueryResponse([]plugin.QueryResult{result})
}

func (c *ClipboardPlugin) pasteHistoryRecords(ctx context.Context) ([]ClipboardRecord, error) {
	limit := c.maxHistoryCount
	if limit <= 0 {
		limit = pasteHistoryFallbackLimit
	}
	return c.db.GetRecent(ctx, limit, 0)
}

// sequentialPasteActions builds copy and paste actions that restore the item
// without promoting it in clipboard history.
func (c *ClipboardPlugin) sequentialPasteActions(ctx context.Context, record ClipboardRecord, query plugin.Query) []plugin.QueryResultAction {
	actions := []plugin.QueryResultAction{
		{
			Name: "i18n:plugin_clipboard_copy",
			Icon: icons.Get(icons.ActionCopy),
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				if err := c.restoreRecordForSequentialPaste(ctx, record); err != nil {
					c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to restore clipboard record for sequential paste: id=%s err=%s", record.ID, err.Error()))
				}
			},
		},
	}

	pasteToActiveWindowAction, pasteToActiveWindowErr := system.GetPasteToActiveWindowAction(ctx, c.api, query.Env.ActiveWindowTitle, query.Env.ActiveWindowPid, query.Env.ActiveWindowIcon, func(actionCtx context.Context) error {
		if err := c.restoreRecordForSequentialPaste(actionCtx, record); err != nil {
			return fmt.Errorf("failed to restore clipboard record before paste: %w", err)
		}
		return nil
	})
	if pasteToActiveWindowErr == nil {
		applyCopyPastePrimaryAction(&actions[0], &pasteToActiveWindowAction, primaryActionValuePaste)
		actions = append(actions, pasteToActiveWindowAction)
	} else {
		applyCopyPastePrimaryAction(&actions[0], nil, primaryActionValuePaste)
		c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("skip paste to active window action: %s", pasteToActiveWindowErr.Error()))
	}

	return actions
}

// restoreRecordForSequentialPaste writes the current item back to the system
// clipboard without promoting it in history, then advances the paste cursor.
func (c *ClipboardPlugin) restoreRecordForSequentialPaste(ctx context.Context, record ClipboardRecord) error {
	c.markSkipHistoryCapture()
	if err := c.writeRecordToClipboard(ctx, record); err != nil {
		c.clearSkipHistoryCapture()
		return err
	}
	c.advancePasteCursorFromHistory(ctx)
	return nil
}

// writeRecordToClipboard restores one history record to the system clipboard.
func (c *ClipboardPlugin) writeRecordToClipboard(ctx context.Context, record ClipboardRecord) error {
	switch record.Type {
	case string(clipboard.ClipboardTypeText):
		if err := clipboard.WriteText(record.Content); err != nil {
			return fmt.Errorf("failed to copy text record to clipboard: id=%s err=%s", record.ID, err.Error())
		}
		return nil
	case string(clipboard.ClipboardTypeFile):
		filePaths := clipboardRecordFilePaths(record)
		if err := clipboard.Write(&clipboard.FilePathData{FilePaths: append([]string(nil), filePaths...)}); err != nil {
			return fmt.Errorf("failed to restore file clipboard record: id=%s err=%s", record.ID, err.Error())
		}
		return nil
	case string(clipboard.ClipboardTypeImage):
		return c.restoreImageRecordToClipboard(ctx, record)
	default:
		return fmt.Errorf("unknown clipboard record type: id=%s type=%s", record.ID, record.Type)
	}
}

// advancePasteCursorFromHistory moves the in-memory pointer to the next older record.
func (c *ClipboardPlugin) advancePasteCursorFromHistory(ctx context.Context) {
	if c.db == nil {
		return
	}
	records, err := c.pasteHistoryRecords(ctx)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to advance paste cursor: %s", err.Error()))
		return
	}

	c.pasteMu.Lock()
	c.pasteCursorID = advancePasteCursor(records, c.pasteCursorID)
	c.pasteMu.Unlock()
}
