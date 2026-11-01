//go:build !windows

package quickjump

import (
	"context"
	"errors"
)

// Directory Opus is Windows-only; these stubs keep shared Quick Jump code platform-neutral.

func findDirectoryOpusLister(pid int, windowID string) (directoryOpusLister, bool) {
	return directoryOpusLister{}, false
}

func (l directoryOpusLister) sourcePath(ctx context.Context) string {
	return ""
}

func (l directoryOpusLister) goTo(ctx context.Context, targetPath string) error {
	return errors.New("Directory Opus is only supported on Windows")
}
