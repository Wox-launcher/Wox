//go:build linux

package window_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"wox/common"
	"wox/plugin"
	"wox/util"
	"wox/util/screen"
	"wox/util/window"
)

const hyprlandFocusCommand = "focus-window"

// hyprlandWindowClient is the compositor identity captured before Wox takes focus.
type hyprlandWindowClient struct {
	Address    string `json:"address"`
	Pid        int    `json:"pid"`
	Monitor    int    `json:"monitor"`
	At         [2]int `json:"at"`
	Size       [2]int `json:"size"`
	Floating   bool   `json:"floating"`
	Fullscreen int    `json:"fullscreen"`
	Workspace  struct {
		ID int `json:"id"`
	} `json:"workspace"`
	Title  string `json:"title"`
	Class  string `json:"class"`
	Mapped bool   `json:"mapped"`
	Hidden bool   `json:"hidden"`
}

// hyprlandCommands exposes only operations that preserve Hyprland's layout policy.
func hyprlandCommands() []windowManagerCommand {
	commands := []windowManagerCommand{
		{Command: "swap-left", TitleKey: "plugin_window_manager_command_swap_left", Aliases: []string{"swap left", "move window left", "向左交换", "窗口向左"}, Op: "swap-left"},
		{Command: "swap-right", TitleKey: "plugin_window_manager_command_swap_right", Aliases: []string{"swap right", "move window right", "向右交换", "窗口向右"}, Op: "swap-right"},
		{Command: "swap-up", TitleKey: "plugin_window_manager_command_swap_up", Aliases: []string{"swap up", "move window up", "向上交换", "窗口向上"}, Op: "swap-up"},
		{Command: "swap-down", TitleKey: "plugin_window_manager_command_swap_down", Aliases: []string{"swap down", "move window down", "向下交换", "窗口向下"}, Op: "swap-down"},
		{Command: "toggle-fullscreen", TitleKey: "plugin_window_manager_command_toggle_fullscreen", Aliases: []string{"toggle fullscreen", "fullscreen", "切换全屏", "全屏"}, Op: "toggle-fullscreen"},
		{Command: "toggle-floating", TitleKey: "plugin_window_manager_command_toggle_floating", Aliases: []string{"toggle floating", "float window", "切换浮动"}, Op: "toggle-floating"},
		{Command: "maximize", TitleKey: "plugin_window_manager_command_toggle_maximize", Aliases: []string{"toggle maximize", "maximize window", "restore window", "最大化", "还原"}, Op: operationMaximize},
		{Command: "next-display", TitleKey: "plugin_window_manager_command_next_display", Aliases: []string{"next monitor", "下一个显示器"}, Op: operationNextDisplay},
		{Command: "previous-display", TitleKey: "plugin_window_manager_command_previous_display", Aliases: []string{"previous monitor", "上一个显示器"}, Op: operationPreviousDisplay},
	}
	return commands
}

// findHyprlandCommand resolves only commands implemented by this backend.
func findHyprlandCommand(id string) (windowManagerCommand, bool) {
	for _, command := range hyprlandCommands() {
		if strings.EqualFold(command.Command, id) {
			return command, true
		}
	}
	if strings.HasPrefix(id, "workspace-") {
		number, err := strconv.Atoi(strings.TrimPrefix(id, "workspace-"))
		if err == nil && number >= 1 && number <= 10 && id == fmt.Sprintf("workspace-%d", number) {
			return hyprlandWorkspaceCommand(number), true
		}
	}
	return windowManagerCommand{}, false
}

// hyprlandWorkspaceCommand represents a single directly executable workspace result.
func hyprlandWorkspaceCommand(index int) windowManagerCommand {
	return windowManagerCommand{Command: fmt.Sprintf("workspace-%d", index), TitleKey: fmt.Sprintf("plugin_window_manager_command_workspace_%d", index), Aliases: []string{fmt.Sprintf("move to workspace %d", index)}, Op: windowOperation(fmt.Sprintf("workspace-%d", index))}
}

// hyprlandMetadataCommands registers the native commands for query routing.
func hyprlandMetadataCommands() []plugin.MetadataCommand {
	commands := []plugin.MetadataCommand{{Command: hyprlandFocusCommand, Description: "i18n:plugin_window_manager_command_focus_window", Aliases: []string{"focus window", "聚焦窗口"}}}
	for _, command := range hyprlandCommands() {
		if isHyprlandDisplayCommand(command.Op) || isHyprlandSwapCommand(command.Op) || command.Op == "toggle-fullscreen" || isHyprlandMaximizeCommand(command.Op) {
			continue
		}
		commands = append(commands, plugin.MetadataCommand{Command: command.Command, Description: common.I18nString("i18n:" + command.TitleKey), Aliases: command.Aliases})
	}
	return commands
}

// queryHyprlandWindows lists supported commands and allows selecting an existing window by name.
func (p *WindowManagerPlugin) queryHyprlandWindows(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	if strings.EqualFold(query.Command, hyprlandFocusCommand) {
		return p.queryHyprlandFocusWindows(ctx, query)
	}
	if !hasActiveWindow(query.Env) || query.Env.ActiveWindowId == "" {
		if query.Command != "" {
			return plugin.NewQueryResponse([]plugin.QueryResult{p.noActiveWindowResult()})
		}
		return plugin.QueryResponse{}
	}
	if query.Command != "" {
		command, ok := findHyprlandCommand(query.Command)
		if !ok {
			return plugin.QueryResponse{}
		}
		if isHyprlandSwapCommand(command.Op) && !hyprlandSwapAvailable(query.Env, command.Op) {
			return plugin.QueryResponse{}
		}
		if isHyprlandDisplayCommand(command.Op) && !hyprlandHasMultipleDisplays() {
			return plugin.QueryResponse{}
		}
		if strings.HasPrefix(command.Command, "workspace-") {
			workspace, err := hyprlandTargetWorkspace(query.Env)
			if err != nil || command.Op == windowOperation(fmt.Sprintf("workspace-%d", workspace)) {
				return plugin.QueryResponse{}
			}
		}
		return plugin.NewQueryResponse([]plugin.QueryResult{p.commandResult(ctx, query, command, 1000)})
	}
	var results []plugin.QueryResult
	commands := hyprlandCommands()
	if workspace, err := hyprlandTargetWorkspace(query.Env); err == nil {
		for _, index := range hyprlandWorkspaceIDs(ctx) {
			if index != workspace {
				commands = append(commands, hyprlandWorkspaceCommand(index))
			}
		}
	} else {
		util.GetLogger().Debug(ctx, fmt.Sprintf("Hyprland target workspace: %v", err))
	}
	for _, command := range commands {
		if isHyprlandSwapCommand(command.Op) && !hyprlandSwapAvailable(query.Env, command.Op) {
			continue
		}
		if isHyprlandDisplayCommand(command.Op) && !hyprlandHasMultipleDisplays() {
			continue
		}
		if matched, score := p.commandMatches(ctx, command, query.Search); matched {
			results = append(results, p.commandResult(ctx, query, command, score))
		}
	}
	return plugin.NewQueryResponse(results)
}

// queryHyprlandFocusWindows lists mapped windows with an exact target per action.
func (p *WindowManagerPlugin) queryHyprlandFocusWindows(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	clients, err := readHyprlandClients(ctx)
	if err != nil {
		p.api.Log(ctx, plugin.LogLevelWarning, "list Hyprland windows: "+err.Error())
		return plugin.QueryResponse{}
	}
	var results []plugin.QueryResult
	for _, client := range clients {
		if !client.Mapped || client.Hidden || client.Pid <= 0 || !validHyprlandAddress(client.Address) {
			continue
		}
		title := strings.TrimSpace(client.Title)
		if title == "" {
			title = client.Class
		}
		if title == "" {
			continue
		}
		if search := strings.TrimSpace(query.Search); search != "" && !strings.Contains(strings.ToLower(title+" "+client.Class), strings.ToLower(search)) {
			continue
		}
		env := query.Env
		env.ActiveWindowId, env.ActiveWindowPid, env.ActiveWindowTitle = client.Address, client.Pid, title
		env.ActiveWindowIcon = common.WoxImage{}
		result := p.commandResult(ctx, plugin.Query{Env: env}, windowManagerCommand{Command: hyprlandFocusCommand, TitleKey: "plugin_window_manager_command_focus_window", Op: "focus-window"}, 1000)
		result.Title = title
		result.SubTitle = client.Class
		result.IdentityKey = "hyprland-window:" + client.Address
		result.Actions[0].ContextData = map[string]string{windowManagerMRUTypeKey: "hyprland-window", "windowId": client.Address, "pid": strconv.Itoa(client.Pid)}
		results = append(results, result)
	}
	return plugin.NewQueryResponse(results)
}

// readHyprlandClients queries a bounded snapshot of compositor windows.
func readHyprlandClients(ctx context.Context) ([]hyprlandWindowClient, error) {
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	output, err := exec.CommandContext(ctx, "hyprctl", "-j", "clients").Output()
	if err != nil {
		return nil, err
	}
	var clients []hyprlandWindowClient
	if err := json.Unmarshal(output, &clients); err != nil {
		return nil, err
	}
	return clients, nil
}

// validHyprlandAddress restricts selectors to compositor address literals.
func validHyprlandAddress(address string) bool {
	if len(address) < 3 || len(address) > 34 || !strings.HasPrefix(address, "0x") {
		return false
	}
	for _, char := range address[2:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", char) {
			return false
		}
	}
	return true
}

// hyprlandWindowDispatch uses only validated addresses and fixed compositor operations.
func hyprlandWindowDispatch(op windowOperation, address string) (string, error) {
	if !validHyprlandAddress(address) {
		return "", fmt.Errorf("invalid Hyprland window address")
	}
	target := "address:" + address
	switch op {
	case "focus-window":
		return fmt.Sprintf("hl.dsp.focus({ window = %q })", target), nil
	case "swap-left", "swap-right", "swap-up", "swap-down":
		return fmt.Sprintf("hl.dsp.focus({ window = %q })", target), nil
	case "toggle-fullscreen":
		return fmt.Sprintf("hl.dsp.window.fullscreen({ window = %q, mode = 'fullscreen', action = 'toggle' })", target), nil
	case "toggle-floating":
		return fmt.Sprintf("hl.dsp.window.float({ window = %q, action = 'toggle' })", target), nil
	case operationMaximize:
		return fmt.Sprintf("hl.dsp.window.fullscreen({ window = %q, mode = 'maximized', action = 'toggle' })", target), nil
	case operationNextDisplay:
		return fmt.Sprintf("hl.dsp.window.move({ window = %q, monitor = '+1' })", target), nil
	case operationPreviousDisplay:
		return fmt.Sprintf("hl.dsp.window.move({ window = %q, monitor = '-1' })", target), nil
	}
	if number, err := strconv.Atoi(strings.TrimPrefix(string(op), "workspace-")); strings.HasPrefix(string(op), "workspace-") && err == nil && number >= 1 && number <= 10 {
		return fmt.Sprintf("hl.dsp.window.move({ window = %q, workspace = %q, follow = true })", target, strconv.Itoa(number)), nil
	}
	return "", fmt.Errorf("unsupported Hyprland window operation")
}

// applyHyprlandWindowCommand revalidates the captured address before dispatching.
func applyHyprlandWindowCommand(ctx context.Context, command windowManagerCommand, env plugin.QueryEnv) error {
	script, err := hyprlandWindowDispatch(command.Op, env.ActiveWindowId)
	if err != nil {
		return err
	}
	clients, err := readHyprlandClients(ctx)
	if err != nil {
		return err
	}
	found := false
	var target hyprlandWindowClient
	for _, client := range clients {
		if client.Address == env.ActiveWindowId && client.Pid == env.ActiveWindowPid && client.Mapped && !client.Hidden {
			found = true
			target = client
			break
		}
	}
	if !found {
		return fmt.Errorf("target Hyprland window is no longer available")
	}
	if isHyprlandSwapCommand(command.Op) {
		neighbor, ok := findHyprlandSwapNeighbor(clients, env, command.Op)
		if !ok {
			return fmt.Errorf("no adjacent tiled window in this workspace")
		}
		if err := dispatchHyprlandWindow(ctx, script); err != nil {
			return err
		}
		active, err := exec.CommandContext(ctx, "hyprctl", "-j", "activewindow").Output()
		if err != nil {
			return err
		}
		var focused hyprlandWindowClient
		if err := json.Unmarshal(active, &focused); err != nil || focused.Address != env.ActiveWindowId || focused.Pid != env.ActiveWindowPid {
			return fmt.Errorf("could not focus target window for swap")
		}
		script = fmt.Sprintf("hl.dsp.window.swap({ target = %q })", "address:"+neighbor.Address)
	}
	if command.Op == operationNextDisplay || command.Op == operationPreviousDisplay {
		monitors, monitorErr := screen.GetHyprlandMonitors()
		if monitorErr != nil {
			return monitorErr
		}
		name, targetErr := adjacentHyprlandMonitor(monitors, target.Monitor, command.Op == operationNextDisplay)
		if targetErr != nil {
			return targetErr
		}
		script = fmt.Sprintf("hl.dsp.window.move({ window = %q, monitor = %q })", "address:"+env.ActiveWindowId, name)
	}
	if err := dispatchHyprlandWindow(ctx, script); err != nil {
		return err
	}
	if strings.HasPrefix(string(command.Op), "workspace-") || isHyprlandMaximizeCommand(command.Op) || command.Op == "toggle-fullscreen" {
		hyprlandTargetWorkspaceCache.Lock()
		hyprlandTargetWorkspaceCache.at = time.Time{}
		hyprlandTargetWorkspaceCache.Unlock()
	}
	return nil
}

// restoreHyprlandWindowMRU only restores a result while its original client still exists.
func (p *WindowManagerPlugin) restoreHyprlandWindowMRU(ctx context.Context, data plugin.MRUData) (*plugin.QueryResult, error) {
	pid, err := strconv.Atoi(data.ContextData["pid"])
	if err != nil {
		return nil, err
	}
	clients, err := readHyprlandClients(ctx)
	if err != nil {
		return nil, err
	}
	for _, client := range clients {
		if client.Address != data.ContextData["windowId"] || client.Pid != pid || !client.Mapped || client.Hidden {
			continue
		}
		title := strings.TrimSpace(client.Title)
		if title == "" {
			title = client.Class
		}
		if title == "" {
			break
		}
		env := plugin.QueryEnv{ActiveWindowId: client.Address, ActiveWindowPid: client.Pid, ActiveWindowTitle: title}
		command := windowManagerCommand{Command: hyprlandFocusCommand, TitleKey: "plugin_window_manager_command_focus_window", Op: "focus-window"}
		result := p.commandResult(ctx, plugin.Query{Env: env}, command, 0)
		result.Title, result.SubTitle, result.IdentityKey = title, client.Class, "hyprland-window:"+client.Address
		result.Actions[0].ContextData = data.ContextData
		return &result, nil
	}
	return nil, fmt.Errorf("Hyprland window no longer exists")
}

// adjacentHyprlandMonitor uses logical desktop positions, including negative origins.
func adjacentHyprlandMonitor(monitors []screen.HyprlandMonitor, currentID int, next bool) (string, error) {
	if len(monitors) < 2 {
		return "", window.ErrWindowManagementNoAdjacentDisplay
	}
	ordered := append([]screen.HyprlandMonitor(nil), monitors...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].X != ordered[j].X {
			return ordered[i].X < ordered[j].X
		}
		if ordered[i].Y != ordered[j].Y {
			return ordered[i].Y < ordered[j].Y
		}
		return ordered[i].ID < ordered[j].ID
	})
	for i, monitor := range ordered {
		if monitor.ID == currentID {
			if next {
				i = (i + 1) % len(ordered)
			} else {
				i = (i + len(ordered) - 1) % len(ordered)
			}
			if ordered[i].Name == "" {
				return "", window.ErrWindowManagementDisplayNotFound
			}
			return ordered[i].Name, nil
		}
	}
	return "", window.ErrWindowManagementDisplayNotFound
}

// hyprlandWorkspaceCache keeps compositor I/O out of the query path.
var hyprlandWorkspaceCache = struct {
	sync.Mutex
	ids     []int
	checked time.Time
	running bool
}{}

// hyprlandWorkspaceIDs mirrors Omarchy's bar while refreshing active workspaces in the background.
func hyprlandWorkspaceIDs(ctx context.Context) []int {
	hyprlandWorkspaceCache.Lock()
	ids := append([]int(nil), hyprlandWorkspaceCache.ids...)
	if len(ids) == 0 {
		ids = []int{1, 2, 3, 4, 5}
	}
	if !hyprlandWorkspaceCache.running && time.Since(hyprlandWorkspaceCache.checked) >= time.Second {
		hyprlandWorkspaceCache.running = true
		hyprlandWorkspaceCache.checked = time.Now()
		util.Go(ctx, "refresh Hyprland workspaces", func() {
			data, err := screen.ReadHyprlandIPC("j/workspaces", 1024*1024)
			var workspaces []struct {
				ID int `json:"id"`
			}
			if err == nil {
				err = json.Unmarshal(data, &workspaces)
			}
			var active []int
			for _, workspace := range workspaces {
				active = append(active, workspace.ID)
			}
			hyprlandWorkspaceCache.Lock()
			if err == nil {
				hyprlandWorkspaceCache.ids = omarchyWorkspaceIDs(active)
			}
			hyprlandWorkspaceCache.running = false
			hyprlandWorkspaceCache.Unlock()
			if err != nil {
				util.GetLogger().Debug(ctx, fmt.Sprintf("Hyprland workspace list: %v", err))
			}
		})
	}
	hyprlandWorkspaceCache.Unlock()
	return ids
}

// omarchyWorkspaceIDs follows the bar: 1–5 plus existing numbered workspaces through 10.
func omarchyWorkspaceIDs(active []int) []int {
	ids := []int{1, 2, 3, 4, 5}
	seen := [11]bool{1: true, 2: true, 3: true, 4: true, 5: true}
	for _, id := range active {
		if id > 5 && id <= 10 && !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	sort.Ints(ids)
	return ids
}

// hyprlandTargetWorkspaceCache shares a short-lived compositor snapshot across keystrokes.
var hyprlandTargetWorkspaceCache = struct {
	sync.Mutex
	address   string
	pid       int
	workspace int
	clients   []hyprlandWindowClient
	at        time.Time
}{}

// hyprlandTargetWorkspace finds the captured window, not the launcher-focused workspace.
func hyprlandTargetWorkspace(env plugin.QueryEnv) (int, error) {
	if !validHyprlandAddress(env.ActiveWindowId) || env.ActiveWindowPid <= 0 {
		return 0, fmt.Errorf("invalid target window")
	}
	hyprlandTargetWorkspaceCache.Lock()
	defer hyprlandTargetWorkspaceCache.Unlock()
	if hyprlandTargetWorkspaceCache.address == env.ActiveWindowId && hyprlandTargetWorkspaceCache.pid == env.ActiveWindowPid && time.Since(hyprlandTargetWorkspaceCache.at) < 200*time.Millisecond {
		return hyprlandTargetWorkspaceCache.workspace, nil
	}
	raw, err := screen.ReadHyprlandIPC("j/clients", 4*1024*1024)
	if err != nil {
		return 0, err
	}
	workspace, err := hyprlandWorkspaceFromClients(raw, env)
	if err != nil {
		return 0, err
	}
	var clients []hyprlandWindowClient
	if err := json.Unmarshal(raw, &clients); err != nil {
		return 0, err
	}
	hyprlandTargetWorkspaceCache.clients = clients
	hyprlandTargetWorkspaceCache.address = env.ActiveWindowId
	hyprlandTargetWorkspaceCache.pid = env.ActiveWindowPid
	hyprlandTargetWorkspaceCache.workspace = workspace
	hyprlandTargetWorkspaceCache.at = time.Now()
	return workspace, nil
}

// hyprlandWorkspaceFromClients validates the exact client before using its workspace.
func hyprlandWorkspaceFromClients(raw []byte, env plugin.QueryEnv) (int, error) {
	var clients []hyprlandWindowClient
	if err := json.Unmarshal(raw, &clients); err != nil {
		return 0, err
	}
	for _, client := range clients {
		if client.Address == env.ActiveWindowId && client.Pid == env.ActiveWindowPid && client.Mapped && !client.Hidden {
			if client.Workspace.ID == 0 {
				return 0, fmt.Errorf("target window has no workspace")
			}
			return client.Workspace.ID, nil
		}
	}
	return 0, fmt.Errorf("target window is no longer available")
}

func isHyprlandDisplayCommand(operation windowOperation) bool {
	return operation == operationNextDisplay || operation == operationPreviousDisplay
}

// hyprlandDisplayCache limits monitor queries during continuous typing.
var hyprlandDisplayCache = struct {
	sync.Mutex
	count int
	at    time.Time
}{}

// hyprlandHasMultipleDisplays hides display movement until the compositor confirms another display.
func hyprlandHasMultipleDisplays() bool {
	hyprlandDisplayCache.Lock()
	defer hyprlandDisplayCache.Unlock()
	if time.Since(hyprlandDisplayCache.at) < 200*time.Millisecond {
		return hyprlandDisplayCache.count > 1
	}
	monitors, err := screen.GetHyprlandMonitors()
	hyprlandDisplayCache.at = time.Now()
	if err != nil {
		hyprlandDisplayCache.count = 0
		return false
	}
	hyprlandDisplayCache.count = len(monitors)
	return len(monitors) > 1
}

// dispatchHyprlandWindow executes one bounded compositor operation.
func dispatchHyprlandWindow(ctx context.Context, script string) error {
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	output, err := exec.CommandContext(ctx, "hyprctl", "dispatch", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("Hyprland dispatch: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if strings.TrimSpace(string(output)) != "ok" {
		return fmt.Errorf("Hyprland dispatch: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func isHyprlandSwapCommand(op windowOperation) bool {
	return op == "swap-left" || op == "swap-right" || op == "swap-up" || op == "swap-down"
}

// findHyprlandSwapNeighbor chooses a tiled window on the same workspace and monitor.
// Compositor positions and sizes are logical desktop coordinates, so no DPI conversion is needed.
func findHyprlandSwapNeighbor(clients []hyprlandWindowClient, env plugin.QueryEnv, op windowOperation) (hyprlandWindowClient, bool) {
	var target hyprlandWindowClient
	found := false
	for _, c := range clients {
		if c.Address == env.ActiveWindowId && c.Pid == env.ActiveWindowPid && c.Mapped && !c.Hidden && !c.Floating && c.Workspace.ID != 0 && c.Size[0] > 0 && c.Size[1] > 0 {
			target, found = c, true
			break
		}
	}
	if !found || !isHyprlandSwapCommand(op) {
		return hyprlandWindowClient{}, false
	}
	var best hyprlandWindowClient
	bestGap, bestOverlap, ok := 0, 0, false
	for _, c := range clients {
		if c.Address == target.Address || !validHyprlandAddress(c.Address) || !c.Mapped || c.Hidden || c.Floating || c.Workspace.ID != target.Workspace.ID || c.Monitor != target.Monitor || c.Size[0] <= 0 || c.Size[1] <= 0 {
			continue
		}
		var overlap, gap int
		switch op {
		case "swap-left", "swap-right":
			overlap = min(target.At[1]+target.Size[1], c.At[1]+c.Size[1]) - max(target.At[1], c.At[1])
			if op == "swap-left" {
				gap = target.At[0] - (c.At[0] + c.Size[0])
			} else {
				gap = c.At[0] - (target.At[0] + target.Size[0])
			}
		case "swap-up", "swap-down":
			overlap = min(target.At[0]+target.Size[0], c.At[0]+c.Size[0]) - max(target.At[0], c.At[0])
			if op == "swap-up" {
				gap = target.At[1] - (c.At[1] + c.Size[1])
			} else {
				gap = c.At[1] - (target.At[1] + target.Size[1])
			}
		}
		if overlap <= 0 {
			continue
		}
		if gap < 0 {
			continue
		}
		if !ok || gap < bestGap || (gap == bestGap && overlap > bestOverlap) {
			best, bestGap, bestOverlap, ok = c, gap, overlap, true
		}
	}
	return best, ok
}

// hyprlandSwapAvailable shares the same short-lived client snapshot as workspace filtering.
func hyprlandSwapAvailable(env plugin.QueryEnv, op windowOperation) bool {
	clients, err := hyprlandTargetClients(env)
	if err != nil {
		return false
	}
	_, ok := findHyprlandSwapNeighbor(clients, env, op)
	return ok
}

// hyprlandTargetClients returns a detached client snapshot for the captured target.
func hyprlandTargetClients(env plugin.QueryEnv) ([]hyprlandWindowClient, error) {
	if _, err := hyprlandTargetWorkspace(env); err != nil {
		return nil, err
	}
	hyprlandTargetWorkspaceCache.Lock()
	defer hyprlandTargetWorkspaceCache.Unlock()
	if hyprlandTargetWorkspaceCache.address != env.ActiveWindowId || hyprlandTargetWorkspaceCache.pid != env.ActiveWindowPid {
		return nil, fmt.Errorf("target changed")
	}
	return append([]hyprlandWindowClient(nil), hyprlandTargetWorkspaceCache.clients...), nil
}

func isHyprlandMaximizeCommand(op windowOperation) bool {
	return op == operationMaximize
}
