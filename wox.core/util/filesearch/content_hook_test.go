package filesearch

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

type observingContentHook struct {
	onClose func()
}

func (h *observingContentHook) Notify(ctx context.Context, notification ContentHookNotification) {}

func (h *observingContentHook) Close() {
	if h.onClose != nil {
		h.onClose()
	}
}

func TestEngineCloseContentHookDetachesScannerBeforeClosing(t *testing.T) {
	scanner := NewScanner(nil)
	hook := &observingContentHook{
		onClose: func() {
			if scanner.contentHook != nil {
				t.Fatal("scanner content hook should be detached before hook close")
			}
		},
	}
	scanner.SetContentHook(hook)

	engine := &Engine{
		scanner:     scanner,
		contentHook: hook,
	}
	engine.closeContentHookLocked()
}

func TestContentIndexHookCloseCancelsScopeReconcile(t *testing.T) {
	contentDB := newTestContentSearchDB(t)
	defer contentDB.Close()
	nameDB, ctx := openTestFileSearchDB(t)

	scopePath := t.TempDir()
	contentPath := filepath.Join(scopePath, "stale.txt")
	if _, err := contentDB.IndexContent(ctx, contentPath, 1, 1, "txt", "hook close should cancel reconcile"); err != nil {
		t.Fatalf("index stale content: %v", err)
	}

	contentDB.db.SetMaxOpenConns(1)
	conn, err := contentDB.db.Conn(ctx)
	if err != nil {
		t.Fatalf("reserve content db connection: %v", err)
	}

	hook := NewContentIndexHook(contentDB, nameDB, []RootRecord{{ID: "root", Path: scopePath}}, Policy{}, ContentExtensionsFromList([]string{"txt"}), ContentDefaultMaxReadBytes)
	hook.Notify(ctx, ContentHookNotification{Kind: ContentHookKindScopeReplaced, RootID: "root", ScopePath: scopePath})

	time.Sleep(50 * time.Millisecond)

	closeDone := make(chan struct{})
	go func() {
		hook.Close()
		close(closeDone)
	}()

	select {
	case <-closeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("content hook close did not finish")
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("release content db connection: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	results, err := contentDB.SearchContent(ctx, "cancel reconcile", 10)
	if err != nil {
		t.Fatalf("search content after close: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("content entry was deleted by a scope reconcile after hook close")
	}
}

// TestContentNotificationOverflowReconcilesRoots bounds event bursts without
// dropping the eventual reconciliation of files omitted from the point queue.
func TestContentNotificationOverflowReconcilesRoots(t *testing.T) {
	root := RootRecord{ID: "content", Path: t.TempDir()}
	hook := &ContentIndexHook{selection: newContentSelection([]RootRecord{root}, Policy{}, false), extensions: map[string]bool{"txt": true}, pending: map[string]ContentHookNotification{}, wakeCh: make(chan struct{}, 1), stopCh: make(chan struct{}), ctx: context.Background()}
	for i := range 9000 {
		hook.Notify(context.Background(), ContentHookNotification{Kind: ContentHookKindUpsert, Path: filepath.Join(root.Path, fmt.Sprintf("%d.txt", i))})
	}
	if len(hook.pending) != 1 {
		t.Fatalf("queue did not coalesce: %d", len(hook.pending))
	}
	for _, notification := range hook.pending {
		if notification.Kind != ContentHookKindScopeReplaced || notification.ScopePath != root.Path {
			t.Fatalf("wrong overflow recovery: %+v", notification)
		}
	}
	hook.Notify(context.Background(), ContentHookNotification{Kind: ContentHookKindUpsert, Path: filepath.Join(t.TempDir(), "outside.txt")})
	if len(hook.pending) != 1 {
		t.Fatal("outside content root event was queued")
	}
}
