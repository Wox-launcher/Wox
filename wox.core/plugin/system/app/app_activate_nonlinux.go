//go:build !linux

package app

import (
	"context"

	"wox/plugin"
)

func tryActivateExistingLinuxApp(ctx context.Context, api plugin.API, info appInfo) bool {
	return false
}
