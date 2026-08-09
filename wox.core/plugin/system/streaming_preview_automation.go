//go:build wox_automation

package system

import (
	"context"
	"os"
	"time"
	"wox/common"
	"wox/plugin"
	"wox/util"

	"github.com/google/uuid"
)

const (
	streamingPreviewAutomationTrigger = "wox-smoke-streaming-preview"
	smokeStepDelayEnvironment         = "WOX_GO_UI_SMOKE_STEP_DELAY"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &streamingPreviewAutomationPlugin{})
}

type streamingPreviewAutomationPlugin struct {
	api plugin.API
}

// GetMetadata keeps the streaming-preview fixture isolated behind an explicit smoke trigger.
func (*streamingPreviewAutomationPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id: "7df6c658-4cf7-4758-9c88-a856faa25b01", Name: "Streaming Preview Smoke Fixture", Runtime: "Go", Version: "1.0.0",
		TriggerKeywords: []string{streamingPreviewAutomationTrigger}, SupportedOS: []string{util.PlatformMacOS, util.PlatformWindows, util.PlatformLinux},
	}
}

func (p *streamingPreviewAutomationPlugin) Init(_ context.Context, initParams plugin.InitParams) {
	p.api = initParams.API
}

// Query returns a compact result before asynchronously publishing the first streaming preview chunk.
func (p *streamingPreviewAutomationPlugin) Query(_ context.Context, _ plugin.Query) plugin.QueryResponse {
	resultID := uuid.NewString()
	api := p.api
	time.AfterFunc(streamingPreviewUpdateDelay(), func() {
		title := "Streaming preview received"
		api.UpdateResult(context.Background(), plugin.UpdatableResult{
			Id: resultID, Title: &title,
			Preview: &plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeMarkdown, PreviewData: "# Streaming preview\n\nFirst preview chunk"},
		})
	})
	return plugin.NewQueryResponse([]plugin.QueryResult{{Id: resultID, Title: "Streaming preview pending", Icon: common.PluginAppIcon}})
}

// streamingPreviewUpdateDelay leaves the compact result observable after an optional smoke step pause.
func streamingPreviewUpdateDelay() time.Duration {
	delay := 250 * time.Millisecond
	if stepDelay, err := time.ParseDuration(os.Getenv(smokeStepDelayEnvironment)); err == nil && stepDelay > 0 {
		delay += stepDelay
	}
	return delay
}
