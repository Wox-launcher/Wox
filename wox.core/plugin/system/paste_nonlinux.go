//go:build !linux

package system

import (
	"context"
	"wox/plugin"
	"wox/util/window"
)

func pasteToKDEWindow(context.Context, plugin.API, plugin.QueryEnv) error {
	return window.ErrWindowManagementUnsupported
}
