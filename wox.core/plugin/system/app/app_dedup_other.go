//go:build !windows

package app

func populateAppLaunchKey(info *appInfo) {}

func deduplicateAppLaunches(apps []appInfo) []appInfo { return apps }
