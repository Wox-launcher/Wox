package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"wox/common"
)

// TestProjectToolsUseRequestDirectory verifies parallel chats cannot change one another's cwd.
func TestProjectToolsUseRequestDirectory(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	roots := []string{t.TempDir(), t.TempDir()}
	var work sync.WaitGroup
	for _, root := range roots {
		work.Add(1)
		go func() {
			defer work.Done()
			ctx := common.WithAIWorkingDirectory(context.Background(), root)
			if _, err := writeFileCallback(ctx, map[string]any{"path": "nested/file.txt", "content": root}); err != nil {
				t.Error(err)
				return
			}
			result, err := readFileCallback(ctx, map[string]any{"path": "nested/file.txt"})
			if err != nil || result.Text != root {
				t.Errorf("read: %q, %v", result.Text, err)
			}
			_, err = editCallback(ctx, map[string]any{"path": "nested/file.txt", "edits": []any{map[string]any{"oldText": root, "newText": "changed"}}})
			if err != nil {
				t.Error(err)
			}
			if got := resolveToolPathForContext(ctx, roots[0]); got != roots[0] {
				t.Errorf("absolute path changed: %q", got)
			}
		}()
	}
	work.Wait()
	if now, _ := os.Getwd(); now != original {
		t.Fatalf("process cwd changed from %q to %q", original, now)
	}
	for _, root := range roots {
		data, err := os.ReadFile(filepath.Join(root, "nested", "file.txt"))
		if err != nil || string(data) != "changed" {
			t.Fatalf("edit in %s: %q %v", root, data, err)
		}
	}
}

func TestProjectCommandUsesPrimaryDirectory(t *testing.T) {
	root := t.TempDir()
	ctx := common.WithAIWorkingDirectory(context.Background(), root)
	result, err := bashCallback(ctx, map[string]any{"command": "echo project-context"})
	if err != nil || !strings.Contains(result.Text, "project-context") {
		t.Fatalf("%q %v", result.Text, err)
	}
	// The shell itself resolves this relative output path; no global chdir is allowed.
	if _, err := bashCallback(ctx, map[string]any{"command": "echo scoped > project-command.txt"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "project-command.txt")); err != nil {
		t.Fatal(err)
	}
}
