//go:build !windows

package ui

import "context"

func ensureUIRuntimeDependencies(ctx context.Context, appPath string) error {
	return nil
}

func handleUIRuntimeLaunchFailure(ctx context.Context, waitErr error) {
}
