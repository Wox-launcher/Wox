package autostart

import (
	"context"
)

func SetAutostart(ctx context.Context, enable bool) error {
	return setAutostart(enable)
}
