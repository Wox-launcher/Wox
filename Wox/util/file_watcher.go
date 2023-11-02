package util

import (
	"context"
	"fmt"
	"github.com/fsnotify/fsnotify"
)

func WatchDirectories(ctx context.Context, filePath string, callback func(event fsnotify.Event)) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to create directory watcher: %s", err.Error()))
		return err
	}

	err = watcher.Add(filePath)
	if err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to add directory to watcher: %s", err.Error()))
		return err
	}

	Go(ctx, "watch directory change", func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					watcher.Close()
					return
				}
				callback(event)
			case watchErr, ok := <-watcher.Errors:
				if !ok {
					watcher.Close()
					return
				}

				GetLogger().Error(ctx, fmt.Sprintf("failed to watch file: %s", watchErr.Error()))
			}
		}
	}, func() {
		watcher.Close()
	})

	return nil
}
