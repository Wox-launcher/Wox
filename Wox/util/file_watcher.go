package util

import (
	"context"
	"fmt"
	"github.com/fsnotify/fsnotify"
)

func WatchDirectories(ctx context.Context, filePath string, callback func()) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to create file watcher: %s", err.Error()))
		return
	}
	defer watcher.Close()

	err = watcher.Add(filePath)
	if err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to add file to watcher: %s", err.Error()))
		return
	}

	for {
		select {
		case <-watcher.Events:
			callback()
		case err := <-watcher.Errors:
			GetLogger().Error(ctx, fmt.Sprintf("failed to watch file: %s", err.Error()))
			return
		}
	}
}
