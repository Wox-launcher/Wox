//go:build !linux

package util

import "context"

func ShouldRelaunchLinuxFromDesktopEntry(args []string) bool {
	return false
}

func RelaunchLinuxFromDesktopEntry(ctx context.Context) error {
	return nil
}

func IsLinuxWaylandSession() bool {
	return false
}

func IsLinuxLaunchedFromStableDesktopEntry() bool {
	return false
}
