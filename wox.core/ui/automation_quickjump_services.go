//go:build wox_automation

package ui

import (
	"context"
	"strings"

	"wox/common"
	"wox/plugin"
	"wox/setting"
	"wox/util/screen"

	"github.com/google/uuid"
)

// AutomationOpenQuickJumpQuery opens the Quick Jump secondary with the
// same bottom-anchored chrome the plugin uses over Explorer/open-save dialogs.
func (s *CoreServices) AutomationOpenQuickJumpQuery(ctx context.Context, sessionID, query string) error {
	ctx = uiServiceContext(ctx, sessionID)
	query = strings.TrimSpace(query)
	query = strings.TrimPrefix(query, "jump ")
	query = strings.TrimSpace(query)

	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	theme := GetUIManager().GetCurrentTheme(ctx)
	windowWidth := woxSetting.AppWidth.Get() / 2
	if windowWidth <= 0 {
		windowWidth = 400
	}
	initialWindowHeight := DensityQueryBoxBaseHeight(ctx) + theme.AppPaddingTop + theme.AppPaddingBottom
	if initialWindowHeight <= 0 {
		initialWindowHeight = 80
	}

	display := screen.GetMouseScreen()
	anchor := common.WindowRect{X: display.X, Y: display.Y, Width: display.Width, Height: display.Height}
	position := quickJumpAutomationWindowPosition(anchor, windowWidth, initialWindowHeight)
	showContext := common.ShowContext{
		HideToolbar:          true,
		QueryBoxAtBottom:     true,
		HideOnBlur:           false,
		ShowSource:           common.ShowSourceQuickJump,
		WindowPosition:       &position,
		WindowPositionHeight: initialWindowHeight,
		WindowWidth:          windowWidth,
	}
	const quickJumpPluginID = "6cde8bec-3f19-44f6-8a8b-d3ba3712d04e"
	plugin.GetPluginManager().GetUI().OpenWoxInstance(ctx, common.OpenWoxInstanceRequest{
		Role:         common.WoxInstanceRoleSecondary,
		InstanceName: string(common.ShowSourceQuickJump),
		Query: common.PlainQuery{
			QueryId:   uuid.NewString(),
			QueryType: plugin.QueryTypeInput,
			QueryText: query,
			QueryScope: common.QueryScope{
				Plugins: []common.QueryScopePlugin{{PluginID: quickJumpPluginID}},
			},
		},
		ShowApp: showContext,
	})
	return nil
}

func quickJumpAutomationWindowPosition(anchorRect common.WindowRect, windowWidth int, windowHeight int) common.WindowPosition {
	const margin = 20
	x := anchorRect.X + anchorRect.Width - windowWidth - margin
	if x < anchorRect.X+10 {
		x = anchorRect.X + 10
	}
	y := anchorRect.Y + anchorRect.Height - windowHeight - margin
	if y < anchorRect.Y+10 {
		y = anchorRect.Y + 10
	}
	return common.WindowPosition{X: x, Y: y}
}
