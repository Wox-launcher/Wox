//go:build !windows

package filesearchservice

import (
	"context"
	"fmt"
)

func Serve(context.Context, string, RequestHandler) error {
	return fmt.Errorf("file index service is only available on Windows")
}

func ServeWithStats(context.Context, string, func() IndexStats, RequestHandler) error {
	return fmt.Errorf("file index service is only available on Windows")
}

func Health(context.Context) (Response, error) {
	return Response{}, fmt.Errorf("file index service is only available on Windows")
}

func GetIndexStats(context.Context) (IndexStats, error) {
	return IndexStats{}, fmt.Errorf("file index service is only available on Windows")
}

func Snapshot(context.Context, string, func(SnapshotEntry) error) error {
	return fmt.Errorf("file index service is only available on Windows")
}

func SnapshotRoots(context.Context, []string, func(SnapshotEntry) error) error {
	return fmt.Errorf("file index service is only available on Windows")
}

func Search(context.Context, string, int, bool) ([]SnapshotEntry, error) {
	return nil, fmt.Errorf("file index service is only available on Windows")
}

func Rebuild(context.Context) error {
	return fmt.Errorf("file index service is only available on Windows")
}

func Pause(context.Context) error {
	return fmt.Errorf("file index service is only available on Windows")
}

func Resume(context.Context, string) error {
	return fmt.Errorf("file index service is only available on Windows")
}

func Update(context.Context, string, string, string) error {
	return fmt.Errorf("file index service is only available on Windows")
}
