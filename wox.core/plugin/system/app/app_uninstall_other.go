//go:build !windows

package app

import (
	"context"
	"errors"
)

func isWindowsUninstallNotFound(err error) bool {
	return errors.Is(err, errWindowsUninstallUnsupported)
}

func isWindowsUninstallNotAllowed(err error) bool {
	return false
}

var errWindowsUninstallUnsupported = errors.New("windows uninstall is not supported on this platform")

func executeWindowsUninstall(ctx context.Context, info appInfo) error {
	return errWindowsUninstallUnsupported
}
