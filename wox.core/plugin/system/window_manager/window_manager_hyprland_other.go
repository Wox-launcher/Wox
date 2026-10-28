//go:build !linux

package window_manager

import (
	"context"
	"wox/plugin"
)

func hyprlandMetadataCommands() []plugin.MetadataCommand      { return nil }
func findHyprlandCommand(string) (windowManagerCommand, bool) { return windowManagerCommand{}, false }
func (p *WindowManagerPlugin) queryHyprlandWindows(context.Context, plugin.Query) plugin.QueryResponse {
	return plugin.QueryResponse{}
}
func applyHyprlandWindowCommand(context.Context, windowManagerCommand, plugin.QueryEnv) error {
	return nil
}

func (p *WindowManagerPlugin) restoreHyprlandWindowMRU(context.Context, plugin.MRUData) (*plugin.QueryResult, error) {
	return nil, nil
}

func hyprlandWorkspaceIDs(context.Context) []int { return nil }

func hyprlandTargetWorkspace(plugin.QueryEnv) (int, error) { return 0, nil }

func isHyprlandDisplayCommand(windowOperation) bool { return false }
func hyprlandHasMultipleDisplays() bool             { return false }

func isHyprlandSwapCommand(windowOperation) bool                  { return false }
func hyprlandSwapAvailable(plugin.QueryEnv, windowOperation) bool { return false }

func isHyprlandMaximizeCommand(windowOperation) bool { return false }
