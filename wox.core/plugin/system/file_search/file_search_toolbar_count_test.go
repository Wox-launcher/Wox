package system

import (
	"context"
	"errors"
	"testing"
	"wox/plugin"
	"wox/util"
	"wox/util/filesearch"
)

type countToolbarAPI struct {
	fullProgressToolbarAPI
	cleared []string
}

func (a *countToolbarAPI) ClearToolbarMsg(_ context.Context, id string) {
	a.cleared = append(a.cleared, id)
}

type toolbarStatsReader struct {
	read func() (filesearch.IndexStatsSnapshot, error)
}

func (r *toolbarStatsReader) GetIndexStats(context.Context) (filesearch.IndexStatsSnapshot, error) {
	return r.read()
}

// TestIndexedFileCountVisits verifies the published file count, deduplication,
// and re-entry without depending on queries, filters, or result selection.
func TestIndexedFileCountVisits(t *testing.T) {
	ctx := context.Background()
	api := &countToolbarAPI{}
	c := &FileSearchPlugin{api: api}
	files, reads := int64(20646), 0
	index := &toolbarStatsReader{read: func() (filesearch.IndexStatsSnapshot, error) {
		reads++
		return filesearch.IndexStatsSnapshot{FileCount: files, EntryCount: files + 200}, nil
	}}
	c.syncIndexedFileCount(ctx, index, 0)
	if reads != 0 {
		t.Fatal("inactive toolbar read the index")
	}
	c.enterToolbarQuery()
	c.syncIndexedFileCount(ctx, index, 0)
	c.syncIndexedFileCount(ctx, index, 0)
	if len(api.messages) != 1 {
		t.Fatalf("identical counts produced %d messages", len(api.messages))
	}
	msg := api.messages[0]
	if msg.Title != "20,646 files" || !msg.HideIcon || !msg.Icon.IsEmpty() || msg.Indeterminate || msg.Progress != nil || len(msg.Actions) != 0 {
		t.Fatalf("expected a text-only file count: %+v", msg)
	}
	c.resetToolbarMsgState()
	c.syncIndexedFileCount(ctx, index, 0)
	c.enterToolbarQuery()
	c.syncIndexedFileCount(ctx, index, 0)
	if len(api.messages) != 2 || reads != 3 {
		t.Fatalf("re-entry did not restore count: messages=%d reads=%d", len(api.messages), reads)
	}
	c.resetToolbarMsgState()
	backgroundCompletion := c.armCompletionToolbarHold()
	c.enterToolbarQuery()
	c.clearCompletionToolbarIfCurrent(ctx, backgroundCompletion)
	files = 0
	c.syncIndexedFileCount(ctx, index, 0)
	if got := api.messages[2].Title; got != "0 files" {
		t.Fatalf("empty index: %q", got)
	}
}

// TestIndexedFileCountUnavailable never turns missing statistics into a zero count.
func TestIndexedFileCountUnavailable(t *testing.T) {
	for _, failed := range []bool{false, true} {
		api := &countToolbarAPI{}
		c := &FileSearchPlugin{api: api}
		c.enterToolbarQuery()
		c.lastToolbarMsgSignature = "previous count"
		index := &toolbarStatsReader{read: func() (filesearch.IndexStatsSnapshot, error) {
			if failed {
				return filesearch.IndexStatsSnapshot{}, errors.New("index unavailable")
			}
			return filesearch.IndexStatsSnapshot{Error: "index unavailable"}, nil
		}}
		c.syncIndexedFileCount(context.Background(), index, 0)
		c.syncIndexedFileCount(context.Background(), index, 0)
		if len(api.messages) != 0 || len(api.cleared) != 1 {
			t.Fatalf("unavailable count was displayed or repeatedly cleared: %+v", api)
		}
	}
}

// TestIndexedFileCountDiscardsStaleReads exercises events arriving during a stats read.
func TestIndexedFileCountDiscardsStaleReads(t *testing.T) {
	for _, event := range []string{"leave", "reenter", "progress", "content", "completion"} {
		t.Run(event, func(t *testing.T) {
			ctx := context.Background()
			api := &countToolbarAPI{}
			c := &FileSearchPlugin{api: api}
			c.enterToolbarQuery()
			index := &toolbarStatsReader{read: func() (filesearch.IndexStatsSnapshot, error) {
				switch event {
				case "leave":
					c.resetToolbarMsgState()
				case "reenter":
					c.resetToolbarMsgState()
					c.enterToolbarQuery()
				case "progress":
					c.syncToolbarMsgWithStatus(ctx, filesearch.StatusSnapshot{IsIndexing: true, ActiveRunKind: filesearch.RunKindFull}, false)
				case "content":
					c.showContentToolbarMsg(ctx, plugin.ToolbarMsg{Id: contentSearchToolbarMsgID, Title: "content indexing"})
				case "completion":
					c.armCompletionToolbarHold()
				}
				return filesearch.IndexStatsSnapshot{FileCount: 123}, nil
			}}
			c.syncIndexedFileCount(ctx, index, 0)
			for _, msg := range api.messages {
				if msg.Title == "123 files" {
					t.Fatal("stale statistics replaced a newer toolbar state")
				}
			}
		})
	}
}

// TestIndexedFileCountRestoresAfterHolds checks both hold types and stale timers.
func TestIndexedFileCountRestoresAfterHolds(t *testing.T) {
	ctx := context.Background()
	api := &countToolbarAPI{}
	c := &FileSearchPlugin{api: api}
	c.enterToolbarQuery()
	reads := 0
	index := &toolbarStatsReader{read: func() (filesearch.IndexStatsSnapshot, error) {
		reads++
		return filesearch.IndexStatsSnapshot{FileCount: 42}, nil
	}}
	completion := c.armCompletionToolbarHold()
	c.syncIndexedFileCount(ctx, index, 0)
	if reads != 0 {
		t.Fatal("count replaced completion hold")
	}
	c.completionHoldUntilMs = util.GetSystemTimestamp() - 1
	c.clearCompletionToolbarIfCurrent(ctx, completion)
	c.syncIndexedFileCount(ctx, index, c.toolbarRevision)
	if api.messages[len(api.messages)-1].Title != "42 files" {
		t.Fatal("count did not return after completion")
	}
	old := c.showContentToolbarMsg(ctx, plugin.ToolbarMsg{Id: contentSearchToolbarMsgID, Title: "content"})
	current := c.showContentToolbarMsg(ctx, plugin.ToolbarMsg{Id: contentSearchToolbarMsgID, Title: "content complete"})
	c.clearContentToolbarMsg(ctx, old)
	c.syncIndexedFileCount(ctx, index, 0)
	if api.messages[len(api.messages)-1].Title != "content complete" || reads != 1 {
		t.Fatal("count or old timer replaced the current content message")
	}
	c.clearContentToolbarMsg(ctx, current)
	c.syncIndexedFileCount(ctx, index, 0)
	if api.messages[len(api.messages)-1].Title != "42 files" {
		t.Fatal("count did not return after content completion")
	}
	cleared := len(api.cleared)
	c.resetToolbarMsgState()
	c.enterToolbarQuery()
	c.syncIndexedFileCount(ctx, index, 0)
	c.clearCompletionToolbarIfCurrent(ctx, completion)
	c.clearContentToolbarMsg(ctx, current)
	if len(api.cleared) != cleared {
		t.Fatal("an old timer cleared the new visit")
	}
	staleRevision := c.toolbarRevision
	c.takeToolbarMsgUpdate("new progress")
	c.syncIndexedFileCount(ctx, index, staleRevision)
	if c.lastToolbarMsgSignature != "new progress" {
		t.Fatal("delayed completion restoration replaced new progress")
	}
}
