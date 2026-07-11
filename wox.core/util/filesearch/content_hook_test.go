package filesearch

import (
	"context"
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

	hook := NewContentIndexHook(contentDB, nameDB, ContentExtensionsFromList([]string{"txt"}), ContentDefaultMaxReadBytes)
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
