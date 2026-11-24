//go:build !windows

package windowhook

import (
	"context"
	"io/fs"
)

func PrepareRuntime(fs.FS) (func(string) bool, error) { return nil, nil }

func CleanupRuntime(context.Context) {}
