package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type parallelIndexRetriever struct {
	Retriever
	root         string
	fileStarted  chan struct{}
	extraStarted chan struct{}
}

func (r *parallelIndexRetriever) GetAppDirectories(context.Context) []appDirectory {
	return []appDirectory{{Path: r.root}}
}

func (r *parallelIndexRetriever) GetAppExtensions(context.Context) []string {
	return []string{"exe"}
}

func (r *parallelIndexRetriever) GetExtraApps(context.Context) ([]appInfo, error) {
	return []appInfo{{Name: "Catalog", Path: "virtual:catalog", Type: AppTypeWindowsSetting}}, nil
}

// ParseAppInfo requires catalog preparation to start before file parsing can finish.
func (r *parallelIndexRetriever) ParseAppInfo(ctx context.Context, path string) (appInfo, error) {
	close(r.fileStarted)
	select {
	case <-r.extraStarted:
		return appInfo{Name: "File", Path: path, Type: AppTypeDesktop}, nil
	case <-ctx.Done():
		return appInfo{}, ctx.Err()
	}
}

// PrepareExtraApps requires file parsing to start before catalog preparation can finish.
func (r *parallelIndexRetriever) PrepareExtraApps(ctx context.Context, apps []appInfo) []appInfo {
	close(r.extraStarted)
	select {
	case <-r.fileStarted:
	case <-ctx.Done():
	}
	return apps
}

// TestAppIndexPreparesSourcesConcurrently exercises both workers and the final published index.
func TestAppIndexPreparesSourcesConcurrently(t *testing.T) {
	// Windows app unit tests leave Location uninitialized, so its relative cache
	// path is isolated here instead of writing a real application's index cache.
	t.Chdir(t.TempDir())
	if err := os.Mkdir("cache", 0o755); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file.exe"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	retriever := &parallelIndexRetriever{Retriever: appRetriever, root: root, fileStarted: make(chan struct{}), extraStarted: make(chan struct{})}
	app := &ApplicationPlugin{api: emptyAPIImpl{}, retriever: retriever}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	app.indexApps(ctx)
	if ctx.Err() != nil {
		t.Fatal("file parsing and catalog preparation did not run concurrently")
	}
	if len(app.apps) != 2 {
		t.Fatalf("published apps = %+v, want both file and catalog entries", app.apps)
	}
}
