package terminal

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSessionManagerSubscribeAndSearch(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewSessionManager(Config{
		MaxBufferBytes:       1024,
		MaxBufferLines:       100,
		ChunkBytes:           1024,
		InitialSnapshotBytes: 1024,
		OutputDirectory:      filepath.Join(tempDir, "sessions"),
	})

	var mu sync.Mutex
	var chunkEvents []TerminalChunk
	manager.SetEmitter(func(_ context.Context, uiSessionID string, method string, data any) {
		if uiSessionID != "ui-1" || method != "TerminalChunk" {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if chunk, ok := data.(TerminalChunk); ok {
			chunkEvents = append(chunkEvents, chunk)
		}
	})

	session, err := manager.CreateSession(context.Background(), CreateSessionParams{
		Command:     "echo hello",
		Interpreter: "bash",
	})
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	if _, err := manager.Subscribe(context.Background(), "ui-1", session.ID, 0); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	manager.AppendChunk(context.Background(), session.ID, "hello world\n")

	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	eventCount := len(chunkEvents)
	mu.Unlock()
	if eventCount == 0 {
		t.Fatalf("expected chunk event after append")
	}

	searchResult, err := manager.Search(context.Background(), TerminalSearchRequest{
		SessionID:     session.ID,
		Pattern:       "world",
		Cursor:        0,
		Backward:      false,
		CaseSensitive: false,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if !searchResult.Found {
		t.Fatalf("expected search to find a match")
	}
}

func TestSessionManagerSubscribeUsesTrimmedStartCursor(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewSessionManager(Config{
		MaxBufferBytes:       12,
		MaxBufferLines:       100,
		ChunkBytes:           8,
		InitialSnapshotBytes: 12,
		OutputDirectory:      filepath.Join(tempDir, "sessions"),
	})

	var mu sync.Mutex
	var chunkEvents []TerminalChunk
	manager.SetEmitter(func(_ context.Context, uiSessionID string, method string, data any) {
		if uiSessionID != "ui-2" || method != "TerminalChunk" {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if chunk, ok := data.(TerminalChunk); ok {
			chunkEvents = append(chunkEvents, chunk)
		}
	})

	session, err := manager.CreateSession(context.Background(), CreateSessionParams{
		Command:     "tail -f file.log",
		Interpreter: "zsh",
	})
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	manager.AppendChunk(context.Background(), session.ID, "aaaa\nbbbb\ncccc\n")
	startCursor := session.ringBuffer.StartCursor()
	if startCursor == 0 {
		t.Fatalf("expected start cursor to move after trim")
	}

	if _, err := manager.Subscribe(context.Background(), "ui-2", session.ID, 0); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(chunkEvents) == 0 {
		t.Fatalf("expected initial snapshot chunks")
	}
	if chunkEvents[0].CursorStart != startCursor {
		t.Fatalf("expected first chunk cursor start %d, got %d", startCursor, chunkEvents[0].CursorStart)
	}
}

func TestSessionManagerSubscribeLoadsSessionFromFile(t *testing.T) {
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "sessions")
	manager := NewSessionManager(Config{
		MaxBufferBytes:       4096,
		MaxBufferLines:       100,
		ChunkBytes:           4096,
		InitialSnapshotBytes: 4096,
		OutputDirectory:      outputDir,
	})

	sessionID := "session-from-file"
	outputPath := filepath.Join(outputDir, sessionID+".log")
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(outputPath, []byte("line-1\nline-2\nline-3\n"), 0o644); err != nil {
		t.Fatalf("write output file failed: %v", err)
	}

	var mu sync.Mutex
	var chunks []TerminalChunk
	manager.SetEmitter(func(_ context.Context, uiSessionID string, method string, data any) {
		if uiSessionID != "ui-load-file" || method != "TerminalChunk" {
			return
		}
		if chunk, ok := data.(TerminalChunk); ok {
			mu.Lock()
			chunks = append(chunks, chunk)
			mu.Unlock()
		}
	})

	if _, err := manager.Subscribe(context.Background(), "ui-load-file", sessionID, 0); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(chunks) == 0 {
		t.Fatalf("expected chunk events for restored session")
	}
	var content strings.Builder
	for _, chunk := range chunks {
		content.WriteString(chunk.Content)
	}
	if !strings.Contains(content.String(), "line-2") {
		t.Fatalf("expected restored content, got: %q", content.String())
	}
}

func TestSessionManagerDeleteSessionRemovesFileAndMemory(t *testing.T) {
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "sessions")
	manager := NewSessionManager(Config{
		MaxBufferBytes:       4096,
		MaxBufferLines:       100,
		ChunkBytes:           4096,
		InitialSnapshotBytes: 4096,
		OutputDirectory:      outputDir,
	})

	session, err := manager.CreateSession(context.Background(), CreateSessionParams{
		Command:     "tail -f /tmp/a.log",
		Interpreter: "zsh",
	})
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}
	manager.AppendChunk(context.Background(), session.ID, "line-1\n")

	if err := manager.DeleteSession(session.ID); err != nil {
		t.Fatalf("delete session failed: %v", err)
	}

	if _, ok := manager.GetState(session.ID); ok {
		t.Fatalf("expected session state removed from memory")
	}
	if _, err := os.Stat(session.OutputPath); !os.IsNotExist(err) {
		t.Fatalf("expected output file removed, got err=%v", err)
	}
}
