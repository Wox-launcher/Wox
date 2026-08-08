//go:build wox_automation

package system

import (
	"context"
	"time"

	"wox/common"
	"wox/plugin"
	"wox/util"
)

const slowQueryAutomationTrigger = "wox-smoke-slow"

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &slowQueryAutomationPlugin{})
}

type slowQueryAutomationPlugin struct{}

// GetMetadata keeps the fixture isolated behind an explicit trigger keyword.
func (*slowQueryAutomationPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id: "0cb0d21c-45ce-4fe0-987e-24d645eca58c", Name: "Slow Query Smoke Fixture", Runtime: "Go", Version: "1.0.0",
		TriggerKeywords: []string{slowQueryAutomationTrigger}, SupportedOS: []string{util.PlatformMacOS, util.PlatformWindows, util.PlatformLinux},
	}
}

func (*slowQueryAutomationPlugin) Init(context.Context, plugin.InitParams) {}

// Query delays a real plugin response long enough for smoke tests to observe query loading.
func (*slowQueryAutomationPlugin) Query(ctx context.Context, _ plugin.Query) plugin.QueryResponse {
	timer := time.NewTimer(1500 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return plugin.QueryResponse{}
	case <-timer.C:
		return plugin.NewQueryResponse([]plugin.QueryResult{{Title: "Slow query completed", Icon: common.PluginAppIcon}})
	}
}
