//go:build !windows && !darwin

package app

import (
	"context"
	"errors"
)

func isAppUninstallNotFound(err error) bool {
	return errors.Is(err, errAppUninstallUnsupported)
}

func isAppUninstallNotAllowed(err error) bool {
	return false
}

var errAppUninstallUnsupported = errors.New("app uninstall is not supported on this platform")

func executeAppUninstall(ctx context.Context, info appInfo) error {
	return errAppUninstallUnsupported
}
