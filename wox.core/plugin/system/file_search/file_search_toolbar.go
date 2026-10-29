package system

import (
	"context"
	"fmt"
	"time"
	"wox/plugin"
	"wox/util"
	"wox/util/filesearch"
)

// fileSearchToolbarStats reads the same published file count used by Settings.
type fileSearchToolbarStats interface {
	GetIndexStats(context.Context) (filesearch.IndexStatsSnapshot, error)
}

// enterToolbarQuery invalidates reads from a previous visit before loading status.
func (c *FileSearchPlugin) enterToolbarQuery() {
	c.toolbarMsgStateMu.Lock()
	defer c.toolbarMsgStateMu.Unlock()
	c.toolbarQueryActive = true
	c.toolbarRevision++
	c.lastToolbarMsgSignature = ""
	// A completion received while this context was closed was never visible here.
	c.completionHoldUntilMs = 0
	c.completionHoldGeneration++
}

// syncIndexedFileCount only reads statistics for a visible, otherwise idle toolbar.
// The revision prevents a slow read from replacing newer progress or another visit.
func (c *FileSearchPlugin) syncIndexedFileCount(ctx context.Context, index fileSearchToolbarStats, expectedRevision uint64) {
	c.toolbarMsgStateMu.Lock()
	if !c.toolbarQueryActive || (expectedRevision != 0 && c.toolbarRevision != expectedRevision) || c.completionHoldUntilMs > util.GetSystemTimestamp() {
		c.toolbarMsgStateMu.Unlock()
		return
	}
	if c.contentToolbarMsg != nil {
		signature := buildToolbarMsgSignature(*c.contentToolbarMsg)
		if c.lastToolbarMsgSignature != signature {
			c.lastToolbarMsgSignature = signature
			c.api.ShowToolbarMsg(ctx, *c.contentToolbarMsg)
		}
		c.toolbarMsgStateMu.Unlock()
		return
	}
	c.toolbarRevision++
	revision := c.toolbarRevision
	c.toolbarMsgStateMu.Unlock()

	statsCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	// A nil engine has no usable index, even though its statistics API returns zeros.
	var stats filesearch.IndexStatsSnapshot
	var err error
	available := index != nil && index != (*filesearch.Engine)(nil)
	if available {
		stats, err = index.GetIndexStats(statsCtx)
		available = err == nil && stats.Error == "" && stats.FileCount >= 0
	}
	if err != nil {
		util.GetLogger().Warn(ctx, "Failed to load file search toolbar count: "+err.Error())
	}
	msg := plugin.ToolbarMsg{
		Id: fileSearchToolbarMsgID, HideIcon: true,
		Title: fmt.Sprintf(c.api.GetTranslation(ctx, "plugin_file_status_indexed_files"), formatFileSearchCount(stats.FileCount)),
	}
	signature := ""
	if available {
		signature = buildToolbarMsgSignature(msg)
	}
	c.toolbarMsgStateMu.Lock()
	defer c.toolbarMsgStateMu.Unlock()
	if !c.toolbarQueryActive || c.toolbarRevision != revision || c.lastToolbarMsgSignature == signature {
		return
	}
	c.lastToolbarMsgSignature = signature
	if available {
		c.api.ShowToolbarMsg(ctx, msg)
	} else {
		c.api.ClearToolbarMsg(ctx, fileSearchToolbarMsgID)
	}
}

// showContentToolbarMsg reserves the idle slot while content work owns the toolbar.
func (c *FileSearchPlugin) showContentToolbarMsg(ctx context.Context, msg plugin.ToolbarMsg) uint64 {
	c.toolbarMsgStateMu.Lock()
	defer c.toolbarMsgStateMu.Unlock()
	c.contentToolbarMsg = &msg
	c.contentToolbarGeneration++
	c.toolbarRevision++
	c.lastToolbarMsgSignature = buildToolbarMsgSignature(msg)
	c.api.ShowToolbarMsg(ctx, msg)
	return c.contentToolbarGeneration
}

// clearContentToolbarMsg restores filename status without letting an old completion
// timer remove a newer content message. A zero generation clears current work.
func (c *FileSearchPlugin) clearContentToolbarMsg(ctx context.Context, generation uint64) {
	c.toolbarMsgStateMu.Lock()
	if generation != 0 && generation != c.contentToolbarGeneration {
		c.toolbarMsgStateMu.Unlock()
		return
	}
	c.contentToolbarMsg = nil
	c.contentToolbarGeneration++
	c.toolbarRevision++
	c.lastToolbarMsgSignature = ""
	c.api.ClearToolbarMsg(ctx, contentSearchToolbarMsgID)
	c.toolbarMsgStateMu.Unlock()
	c.syncToolbarMsg(ctx, false)
}
