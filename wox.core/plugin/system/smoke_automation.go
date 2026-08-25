//go:build wox_automation

package system

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"wox/common"
	"wox/plugin"
	"wox/util"

	"github.com/google/uuid"
)

const (
	smokeAutomationTrigger            = "wox-smoke"
	smokeAutomationSlowCommand        = "slow"
	smokeAutomationStreamingCommand   = "streaming-preview"
	smokeAutomationToolbarCommand     = "toolbar"
	smokeAutomationToolbarLongCommand = "toolbar-long"
	smokeAutomationAttentionCommand   = "attention"
	smokeAutomationQuickSelectCommand = "quick-select"
	smokeAutomationTooltipCommand     = "tooltip"
	smokeAutomationToolbarMessageID   = "wox-smoke-toolbar-message"
	smokeAutomationKeepOpenAction     = "keep-open"
	smokeAutomationClearAction        = "clear"
	smokeAutomationResultAction       = "hide-launcher"
	smokeAutomationSecondaryAction    = "open-folder"
	smokeAutomationLongToolbarTitle   = "Indexing file contents across every configured search root: 18365 files are already processed and the catalog is still updating with additional paths that must remain visible in the launcher status"
	smokeAutomationAttentionKey       = "attention-smoke-item"
	smokeAutomationAttentionTitle     = "Attention smoke item"
	smokeAutomationAttentionQuery     = "1+1"
	smokeStepDelayEnvironment         = "WOX_GO_UI_SMOKE_STEP_DELAY"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &smokeAutomationPlugin{})
}

type smokeAutomationPlugin struct {
	api                  plugin.API
	attentionMu          sync.Mutex
	attentionDescription string
}

// GetMetadata exposes one explicit smoke trigger with command-scoped fixture behaviors.
func (*smokeAutomationPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id: "0cb0d21c-45ce-4fe0-987e-24d645eca58c", Name: "Smoke Test Fixture", Runtime: "Go", Version: "1.0.0",
		TriggerKeywords: []string{smokeAutomationTrigger},
		Commands: []plugin.MetadataCommand{
			{Command: smokeAutomationSlowCommand, Description: "Delayed query loading fixture"},
			{Command: smokeAutomationStreamingCommand, Description: "Streaming preview fixture"},
			{Command: smokeAutomationToolbarCommand, Description: "Toolbar message fixture"},
			{Command: smokeAutomationToolbarLongCommand, Description: "Long toolbar message fixture"},
			{Command: smokeAutomationAttentionCommand, Description: "Persistent attention fixture"},
			{Command: smokeAutomationQuickSelectCommand, Description: "Two numbered results for Quick Select"},
			{Command: smokeAutomationTooltipCommand, Description: "Preview tag tooltip fixture"},
			{Command: smokeAutomationListCommand, Description: "500 list results"},
			{Command: smokeAutomationGridCommand, Description: "500 grid results with group headers"},
			{Command: smokeAutomationChatCommand, Description: "200 chat messages with streaming updates"},
			{Command: smokeAutomationWarmCacheCommand, Description: "Repeated text and image warm-cache fixture"},
		},
		SupportedOS: []string{util.PlatformMacOS, util.PlatformWindows, util.PlatformLinux},
	}
}

func (p *smokeAutomationPlugin) Init(_ context.Context, initParams plugin.InitParams) {
	p.api = initParams.API
}

// Query dispatches the deterministic native smoke behaviors by metadata command.
func (p *smokeAutomationPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	switch query.Command {
	case smokeAutomationSlowCommand:
		return p.querySlow(ctx)
	case smokeAutomationStreamingCommand:
		return p.queryStreamingPreview()
	case smokeAutomationToolbarCommand:
		return p.queryToolbar(ctx)
	case smokeAutomationToolbarLongCommand:
		return p.queryToolbarLong(ctx)
	case smokeAutomationAttentionCommand:
		return p.queryAttentionFixture()
	case smokeAutomationQuickSelectCommand:
		return p.queryQuickSelect()
	case smokeAutomationTooltipCommand:
		return queryTooltipPreview()
	case smokeAutomationListCommand:
		return queryListFixture()
	case smokeAutomationGridCommand:
		return queryGridFixture()
	case smokeAutomationChatCommand:
		return p.queryChatFixture()
	case smokeAutomationWarmCacheCommand:
		return queryWarmCacheFixture()
	default:
		return plugin.QueryResponse{}
	}
}

// queryAttentionFixture exposes fresh and repeated pushes through the real plugin API boundary.
func (p *smokeAutomationPlugin) queryAttentionFixture() plugin.QueryResponse {
	return plugin.NewQueryResponse([]plugin.QueryResult{{
		Id:    "attention-smoke-fixture",
		Title: "Attention smoke fixture",
		Icon:  common.PluginAppIcon,
		Actions: []plugin.QueryResultAction{
			{
				Id:                     "push-fresh-attention",
				Name:                   "Push fresh attention",
				IsDefault:              true,
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					p.pushAttentionFixture(ctx, true)
					p.completeAttentionFixtureAction(ctx, actionContext.ResultId, "Attention smoke fixture: fresh pushed")
				},
			},
			{
				Id:                     "repeat-attention",
				Name:                   "Repeat attention",
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					p.pushAttentionFixture(ctx, false)
					p.completeAttentionFixtureAction(ctx, actionContext.ResultId, "Attention smoke fixture: repeat pushed")
				},
			},
		},
	}})
}

func (p *smokeAutomationPlugin) completeAttentionFixtureAction(ctx context.Context, resultID string, title string) {
	p.api.UpdateResult(ctx, plugin.UpdatableResult{Id: resultID, Title: &title})
}

// pushAttentionFixture keeps repeated pushes byte-identical until a fresh generation is requested.
func (p *smokeAutomationPlugin) pushAttentionFixture(ctx context.Context, fresh bool) {
	p.attentionMu.Lock()
	if fresh || p.attentionDescription == "" {
		p.attentionDescription = fmt.Sprintf("Attention smoke generation %d", time.Now().UnixNano())
	}
	description := p.attentionDescription
	p.attentionMu.Unlock()

	p.api.PushAttention(ctx, plugin.PushAttentionRequest{
		Key:         smokeAutomationAttentionKey,
		Title:       smokeAutomationAttentionTitle,
		Description: description,
		Action: &plugin.AttentionAction{
			Type:  plugin.AttentionActionTypeChangeQuery,
			Query: smokeAutomationAttentionQuery,
		},
	})
}

// querySlow delays a real plugin response long enough for smoke tests to observe query loading.
func (*smokeAutomationPlugin) querySlow(ctx context.Context) plugin.QueryResponse {
	timer := time.NewTimer(1500 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return plugin.QueryResponse{}
	case <-timer.C:
		return plugin.NewQueryResponse([]plugin.QueryResult{{Title: "Slow query completed", Icon: common.PluginAppIcon}})
	}
}

// queryStreamingPreview returns a compact result before publishing its first preview chunk.
func (p *smokeAutomationPlugin) queryStreamingPreview() plugin.QueryResponse {
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

// queryTooltipPreview exposes one selected result whose preview tag opens a native tooltip.
func queryTooltipPreview() plugin.QueryResponse {
	return plugin.NewQueryResponse([]plugin.QueryResult{{
		Id:    "tooltip-smoke-fixture",
		Title: "Tooltip smoke fixture",
		Icon:  common.PluginAppIcon,
		Preview: plugin.WoxPreview{
			PreviewType: plugin.WoxPreviewTypeText,
			PreviewData: "Hover the preview tag to show a native tooltip.",
			PreviewTags: []plugin.WoxPreviewTag{{
				Label:   "Tooltip",
				Tooltip: "Native tooltip must not hide the launcher",
			}},
		},
		Actions: []plugin.QueryResultAction{{
			Id:                     smokeAutomationKeepOpenAction,
			Name:                   "Keep open",
			IsDefault:              true,
			PreventHideAfterAction: true,
		}},
	}})
}

// queryQuickSelect returns two named results so a hold-to-number activation can target the second row.
func (p *smokeAutomationPlugin) queryQuickSelect() plugin.QueryResponse {
	return plugin.NewQueryResponse([]plugin.QueryResult{
		{
			Id:    "quick-select-first-fixture",
			Title: "Quick select first fixture",
			Icon:  common.PluginAppIcon,
			Actions: []plugin.QueryResultAction{{
				Id:                     "keep-open",
				Name:                   "Keep open",
				IsDefault:              true,
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					p.completeAttentionFixtureAction(ctx, actionContext.ResultId, "Quick select first activated")
				},
			}},
		},
		{
			Id:    "quick-select-second-fixture",
			Title: "Quick select second fixture",
			Icon:  common.PluginAppIcon,
			Actions: []plugin.QueryResultAction{{
				Id:                     smokeAutomationResultAction,
				Name:                   "Hide launcher",
				IsDefault:              true,
				PreventHideAfterAction: false,
				Action: func(ctx context.Context, _ plugin.ActionContext) {
					p.api.Log(ctx, plugin.LogLevelInfo, "Quick select second fixture activated")
				},
			}},
		},
	})
}

// queryToolbar publishes a persistent toolbar message through the real plugin API boundary.
func (p *smokeAutomationPlugin) queryToolbar(ctx context.Context) plugin.QueryResponse {
	return p.queryToolbarWithStatus(ctx, "Toolbar fixture ready")
}

// queryToolbarLong uses a status long enough to consume leftover footer width.
func (p *smokeAutomationPlugin) queryToolbarLong(ctx context.Context) plugin.QueryResponse {
	return p.queryToolbarWithStatus(ctx, smokeAutomationLongToolbarTitle)
}

// queryToolbarWithStatus publishes one toolbar status plus the shared Enter and extra result actions.
func (p *smokeAutomationPlugin) queryToolbarWithStatus(ctx context.Context, title string) plugin.QueryResponse {
	p.showToolbarMessage(ctx, title)
	return plugin.NewQueryResponse([]plugin.QueryResult{{
		Title: "Toolbar smoke fixture", Icon: common.PluginAppIcon,
		Actions: []plugin.QueryResultAction{
			{
				Id:                     smokeAutomationResultAction,
				Name:                   "Hide launcher",
				IsDefault:              true,
				Hotkey:                 "enter",
				PreventHideAfterAction: false,
				Action: func(callbackCtx context.Context, _ plugin.ActionContext) {
					p.api.Log(callbackCtx, plugin.LogLevelInfo, "Toolbar fixture result action executed")
				},
			},
			{
				Id:                     smokeAutomationSecondaryAction,
				Name:                   "Open folder",
				Hotkey:                 util.PrimaryHotkey("enter"),
				PreventHideAfterAction: true,
				Action: func(callbackCtx context.Context, _ plugin.ActionContext) {
					p.api.Log(callbackCtx, plugin.LogLevelInfo, "Toolbar fixture secondary action executed")
				},
			},
		},
	}})
}

// showToolbarMessage publishes the current toolbar state and keeps its callbacks deterministic.
func (p *smokeAutomationPlugin) showToolbarMessage(ctx context.Context, title string) {
	p.api.ShowToolbarMsg(ctx, plugin.ToolbarMsg{
		Id:            smokeAutomationToolbarMessageID,
		Title:         title,
		Icon:          common.PluginAppIcon,
		Indeterminate: true,
		Actions: []plugin.ToolbarMsgAction{
			{
				Id:                     smokeAutomationKeepOpenAction,
				Name:                   "Keep open",
				Hotkey:                 util.PrimaryHotkey("k"),
				PreventHideAfterAction: true,
				ContextData:            common.ContextData{"marker": "context-round-trip"},
				Action: func(callbackCtx context.Context, actionContext plugin.ToolbarMsgActionContext) {
					p.showToolbarMessage(callbackCtx, fmt.Sprintf("Toolbar fixture keep-open: %s", actionContext.ContextData["marker"]))
				},
			},
			{
				Id:                     smokeAutomationClearAction,
				Name:                   "Clear",
				Hotkey:                 util.PrimaryHotkey("l"),
				PreventHideAfterAction: true,
				Action: func(callbackCtx context.Context, _ plugin.ToolbarMsgActionContext) {
					p.api.ClearToolbarMsg(callbackCtx, smokeAutomationToolbarMessageID)
				},
			},
		},
	})
}

// streamingPreviewUpdateDelay leaves the compact result observable after an optional smoke step pause.
func streamingPreviewUpdateDelay() time.Duration {
	delay := 250 * time.Millisecond
	if stepDelay, err := time.ParseDuration(os.Getenv(smokeStepDelayEnvironment)); err == nil && stepDelay > 0 {
		delay += stepDelay
	}
	return delay
}
