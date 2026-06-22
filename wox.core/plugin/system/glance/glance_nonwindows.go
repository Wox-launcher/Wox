//go:build !windows

package glance

import (
	"context"
	"wox/plugin"
)

func (p *GlancePlugin) windowsBatteryGlance(ctx context.Context) (plugin.GlanceItem, bool) {
	_ = ctx
	return plugin.GlanceItem{}, false
}
