package autostart

import (
	"context"
)

func SetAutostart(ctx context.Context, enable bool) error {
	return setAutostart(enable)
}

func IsAutostart(ctx context.Context) (bool, error) {
	return isAutostart()
}
