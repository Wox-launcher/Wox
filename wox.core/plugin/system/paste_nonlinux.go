//go:build !linux

package system

import (
	"context"
	"wox/plugin"
	"wox/util/keyboard"
)

func simulatePasteAfterFocus(_ context.Context, _ plugin.QueryEnv, focus func() error) error {
	return keyboard.SimulatePasteAfterFocus(focus)
}
