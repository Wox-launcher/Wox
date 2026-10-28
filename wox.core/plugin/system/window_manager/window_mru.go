package window_manager

import (
	"context"
	"fmt"
	"strings"
	"wox/plugin"
	"wox/util"
)

// handleMRURestore rebuilds window manager results from stable action context data.
func (p *WindowManagerPlugin) handleMRURestore(ctx context.Context, mruData plugin.MRUData) (*plugin.QueryResult, error) {
	switch strings.TrimSpace(mruData.ContextData[windowManagerMRUTypeKey]) {
	case windowManagerMRUTypeCommand:
		return p.restoreCommandMRU(ctx, mruData)
	case windowManagerMRUTypeGroup:
		if util.IsHyprlandSession() {
			return nil, fmt.Errorf("saved window layouts are unavailable on Hyprland")
		}
		return p.restoreGroupMRU(ctx, mruData)
	case "hyprland-window":
		if util.IsHyprlandSession() {
			return p.restoreHyprlandWindowMRU(ctx, mruData)
		}
		return nil, fmt.Errorf("Hyprland window is unavailable")
	default:
		return nil, fmt.Errorf("unknown window manager mru type")
	}
}

// restoreCommandMRU restores a command only when the MRU query has an active target window.
func (p *WindowManagerPlugin) restoreCommandMRU(ctx context.Context, mruData plugin.MRUData) (*plugin.QueryResult, error) {
	commandName := strings.TrimSpace(mruData.ContextData[windowManagerMRUCommandKey])
	command, ok := findWindowManagerCommand(commandName)
	if util.IsHyprlandSession() {
		command, ok = findHyprlandCommand(commandName)
	}
	if !ok {
		return nil, fmt.Errorf("window manager command not found: %s", commandName)
	}
	if !hasActiveWindow(mruData.Env) {
		return nil, fmt.Errorf("window manager command mru requires an active window")
	}
	if util.IsHyprlandSession() && isHyprlandSwapCommand(command.Op) && !hyprlandSwapAvailable(mruData.Env, command.Op) {
		return nil, fmt.Errorf("no adjacent tiled window")
	}
	if util.IsHyprlandSession() && isHyprlandDisplayCommand(command.Op) && !hyprlandHasMultipleDisplays() {
		return nil, fmt.Errorf("no other display is available")
	}
	if util.IsHyprlandSession() && strings.HasPrefix(commandName, "workspace-") {
		workspace, err := hyprlandTargetWorkspace(mruData.Env)
		if err != nil || commandName == fmt.Sprintf("workspace-%d", workspace) {
			return nil, fmt.Errorf("workspace target is unavailable or already current")
		}
	}

	result := p.commandResult(ctx, plugin.Query{Env: mruData.Env}, command, 0)
	return &result, nil
}

// restoreGroupMRU restores the latest saved workspace layout for the recorded group id.
func (p *WindowManagerPlugin) restoreGroupMRU(ctx context.Context, mruData plugin.MRUData) (*plugin.QueryResult, error) {
	groupID := strings.TrimSpace(mruData.ContextData[windowManagerMRUGroupIDKey])
	if groupID == "" {
		return nil, fmt.Errorf("empty window manager group id")
	}

	for _, group := range p.loadWindowGroups(ctx) {
		if group.Id == groupID {
			result := p.windowGroupResult(ctx, group, 0)
			return &result, nil
		}
	}
	return nil, fmt.Errorf("window manager group not found: %s", groupID)
}
