package util

import (
	"context"
	"fmt"
	"github.com/fsnotify/fsnotify"
)

func WatchDirectoryChanges(ctx context.Context, directory string, callback func(event fsnotify.Event)) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to create directory watcher: %s", err.Error()))
		return nil, err
	}

	err = watcher.Add(directory)
	if err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to add directory to watcher: %s", err.Error()))
		watcher.Close()
		return nil, err
	}

	Go(ctx, "watch directory change", func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					GetLogger().Error(ctx, "watch directory closed: not receive event")
					watcher.Close()
					return
				}
				callback(event)
			case <-ctx.Done():
				watcher.Close()
				return
			case watchErr, ok := <-watcher.Errors:
				if !ok {
					GetLogger().Error(ctx, "watch directory closed: not receive error")
					watcher.Close()
					return
				}

				GetLogger().Error(ctx, fmt.Sprintf("failed to watch file: %s", watchErr.Error()))
			}
		}
	}, func() {
		watcher.Close()
	})

	return watcher, nil
}
