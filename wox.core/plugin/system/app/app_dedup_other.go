//go:build !windows

package app

import "context"

func populateAppLaunchKey(ctx context.Context, info *appInfo) {}

func deduplicateAppLaunches(apps []appInfo) []appInfo { return apps }
