//go:build !windows

package app

import "context"

func populateLaunchpadVisibility(ctx context.Context, info *appInfo, userDirectories ...appDirectory) {
}

func populateLaunchpadIconMetadata(ctx context.Context, info *appInfo) {
}

func defaultAppDirectories() string { return "[]" }
