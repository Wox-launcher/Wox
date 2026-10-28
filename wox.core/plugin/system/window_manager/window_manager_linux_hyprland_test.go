//go:build linux

package window_manager

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"wox/plugin"
	"wox/util/screen"
)

func TestHyprlandWindowDispatchTargetsExactWindow(t *testing.T) {
	for _, tc := range []struct {
		op       windowOperation
		fragment string
	}{
		{"focus-window", "hl.dsp.focus"}, {"toggle-floating", "hl.dsp.window.float"},
		{"toggle-fullscreen", "mode = 'fullscreen', action = 'toggle'"},
		{"swap-up", "hl.dsp.focus"}, {"swap-down", "hl.dsp.focus"},
		{operationMaximize, "mode = 'maximized', action = 'toggle'"}, {operationNextDisplay, "monitor = '+1'"},
		{operationPreviousDisplay, "monitor = '-1'"}, {"workspace-10", `workspace = "10", follow = true`},
	} {
		script, err := hyprlandWindowDispatch(tc.op, "0xabcdef")
		if err != nil || !strings.Contains(script, `window = "address:0xabcdef"`) || !strings.Contains(script, tc.fragment) {
			t.Fatalf("op=%s script=%q err=%v", tc.op, script, err)
		}
	}
	for _, address := range []string{"", "0x", "0x12'; crash", "123", "0x-1"} {
		if _, err := hyprlandWindowDispatch("toggle-floating", address); err == nil {
			t.Fatalf("accepted address %q", address)
		}
	}
	for _, op := range []windowOperation{"workspace-0", "workspace-11", "left-half"} {
		if _, err := hyprlandWindowDispatch(op, "0x1"); err == nil {
			t.Fatalf("accepted operation %q", op)
		}
	}
}

func TestHyprlandWindowRevalidatesAddressAndPID(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls")
	bin := filepath.Join(dir, "hyprctl")
	script := "#!/bin/sh\nif [ \"$1\" = '-j' ]; then printf '%s' '[{\"address\":\"0xab\",\"pid\":42,\"mapped\":true}]'; else printf '%s' \"$2\" > \"$WOX_WINDOW_TEST_LOG\"; printf ok; fi\n"
	if err := os.WriteFile(bin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("WOX_WINDOW_TEST_LOG", logPath)
	command := windowManagerCommand{Op: "toggle-floating"}
	if err := applyHyprlandWindowCommand(context.Background(), command, plugin.QueryEnv{ActiveWindowId: "0xab", ActiveWindowPid: 99}); err == nil {
		t.Fatal("wrong PID was accepted")
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatal("dispatched a stale window")
	}
	if err := applyHyprlandWindowCommand(context.Background(), command, plugin.QueryEnv{ActiveWindowId: "0xab", ActiveWindowPid: 42}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil || !strings.Contains(string(data), `address:0xab`) {
		t.Fatalf("dispatch=%q error=%v", data, err)
	}
}

func TestHyprlandMetadataExcludesUnsupportedLayouts(t *testing.T) {
	seen := map[string]bool{}
	for _, command := range hyprlandMetadataCommands() {
		seen[command.Command] = true
	}
	for _, id := range []string{"focus-window", "toggle-floating"} {
		if !seen[id] {
			t.Fatalf("missing %s", id)
		}
	}
	if command, ok := findHyprlandCommand("toggle-fullscreen"); !ok || command.Op != "toggle-fullscreen" {
		t.Fatal("fullscreen toggle is not available as an executable result")
	}
	for _, id := range []string{"left-half", "minimize", "group", "workspace-1", "workspace-10", "next-display", "previous-display", "toggle-fullscreen", "maximize", "restore-maximized"} {
		if seen[id] {
			t.Fatalf("unsupported command %s exposed", id)
		}
	}
}

func TestAdjacentHyprlandMonitorUsesTargetWindowDisplay(t *testing.T) {
	monitors := []screen.HyprlandMonitor{{ID: 3, Name: "right", X: 1920}, {ID: 2, Name: "left", X: -1200}, {ID: 1, Name: "center", X: 0}}
	for _, tc := range []struct {
		id   int
		next bool
		want string
	}{{2, true, "center"}, {1, false, "left"}, {3, true, "left"}} {
		got, err := adjacentHyprlandMonitor(monitors, tc.id, tc.next)
		if err != nil || got != tc.want {
			t.Fatalf("id=%d next=%v got=%q err=%v", tc.id, tc.next, got, err)
		}
	}
	if _, err := adjacentHyprlandMonitor(monitors, 9, true); err == nil {
		t.Fatal("missing target monitor accepted")
	}
}

func TestHyprlandFocusSearchKeepsExactWindowIdentity(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\nprintf '%s' '[{\"address\":\"0xaa\",\"pid\":42,\"mapped\":true,\"title\":\"Editor One\",\"class\":\"editor\"},{\"address\":\"0xbb\",\"pid\":42,\"mapped\":true,\"title\":\"Editor Two\",\"class\":\"editor\"},{\"address\":\"0xcc\",\"pid\":42,\"mapped\":false,\"title\":\"Editor Hidden\"}]'\n"
	if err := os.WriteFile(filepath.Join(dir, "hyprctl"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	response := (&WindowManagerPlugin{}).queryHyprlandFocusWindows(context.Background(), plugin.Query{Search: "two"})
	if len(response.Results) != 1 {
		t.Fatalf("results = %+v", response.Results)
	}
	result := response.Results[0]
	if result.Title != "Editor Two" || result.IdentityKey != "hyprland-window:0xbb" || result.Actions[0].ContextData["windowId"] != "0xbb" {
		t.Fatalf("wrong target: %+v", result)
	}
}

func TestOmarchyWorkspaceIDsMatchBar(t *testing.T) {
	for _, tc := range []struct {
		active []int
		want   []int
	}{
		{nil, []int{1, 2, 3, 4, 5}},
		{[]int{7, 3, 10, 7, -99, 11}, []int{1, 2, 3, 4, 5, 7, 10}},
	} {
		if got := omarchyWorkspaceIDs(tc.active); !slices.Equal(got, tc.want) {
			t.Fatalf("active=%v got=%v want=%v", tc.active, got, tc.want)
		}
	}
	for _, id := range []string{"workspace-1", "workspace-10"} {
		if _, ok := findHyprlandCommand(id); !ok {
			t.Fatalf("previous workspace command %s is not restorable", id)
		}
	}
}

func TestHyprlandQueryReturnsOneExecutableResultPerVisibleWorkspace(t *testing.T) {
	hyprlandWorkspaceCache.Lock()
	oldIDs, oldChecked, oldRunning := hyprlandWorkspaceCache.ids, hyprlandWorkspaceCache.checked, hyprlandWorkspaceCache.running
	hyprlandWorkspaceCache.ids = []int{1, 2, 3, 4, 5, 7}
	hyprlandWorkspaceCache.checked = time.Now()
	hyprlandWorkspaceCache.running = false
	hyprlandWorkspaceCache.Unlock()
	t.Cleanup(func() {
		hyprlandWorkspaceCache.Lock()
		hyprlandWorkspaceCache.ids, hyprlandWorkspaceCache.checked, hyprlandWorkspaceCache.running = oldIDs, oldChecked, oldRunning
		hyprlandWorkspaceCache.Unlock()
	})
	hyprlandTargetWorkspaceCache.Lock()
	oldAddress, oldPID, oldWorkspace, oldAt := hyprlandTargetWorkspaceCache.address, hyprlandTargetWorkspaceCache.pid, hyprlandTargetWorkspaceCache.workspace, hyprlandTargetWorkspaceCache.at
	hyprlandTargetWorkspaceCache.address, hyprlandTargetWorkspaceCache.pid, hyprlandTargetWorkspaceCache.workspace, hyprlandTargetWorkspaceCache.at = "0xab", 42, 1, time.Now()
	hyprlandTargetWorkspaceCache.Unlock()
	t.Cleanup(func() {
		hyprlandTargetWorkspaceCache.Lock()
		hyprlandTargetWorkspaceCache.address, hyprlandTargetWorkspaceCache.pid, hyprlandTargetWorkspaceCache.workspace, hyprlandTargetWorkspaceCache.at = oldAddress, oldPID, oldWorkspace, oldAt
		hyprlandTargetWorkspaceCache.Unlock()
	})
	response := (&WindowManagerPlugin{}).queryHyprlandWindows(context.Background(), plugin.Query{Env: plugin.QueryEnv{ActiveWindowId: "0xab", ActiveWindowPid: 42}})
	seen := map[string]bool{}
	for _, result := range response.Results {
		if strings.HasPrefix(result.IdentityKey, "window-command:workspace-") {
			if len(result.Actions) != 1 {
				t.Fatalf("workspace result has no action: %+v", result)
			}
			seen[result.IdentityKey] = true
		}
	}
	for _, number := range []string{"2", "3", "4", "5", "7"} {
		if !seen["window-command:workspace-"+number] {
			t.Fatalf("workspace %s missing", number)
		}
	}
	if seen["window-command:workspace-1"] {
		t.Fatal("current workspace was offered")
	}
	if len(seen) != 5 {
		t.Fatalf("unexpected workspaces: %v", seen)
	}
	same := (&WindowManagerPlugin{}).queryHyprlandWindows(context.Background(), plugin.Query{Command: "workspace-1", Env: plugin.QueryEnv{ActiveWindowId: "0xab", ActiveWindowPid: 42}})
	if len(same.Results) != 0 {
		t.Fatal("current workspace was offered by exact command")
	}
}

func TestHyprlandWorkspaceFromClientsUsesExactWindow(t *testing.T) {
	raw := []byte(`[{"address":"0xaa","pid":42,"mapped":true,"workspace":{"id":1}},{"address":"0xbb","pid":42,"mapped":true,"workspace":{"id":3}}]`)
	workspace, err := hyprlandWorkspaceFromClients(raw, plugin.QueryEnv{ActiveWindowId: "0xbb", ActiveWindowPid: 42})
	if err != nil || workspace != 3 {
		t.Fatalf("workspace=%d err=%v", workspace, err)
	}
	if _, err := hyprlandWorkspaceFromClients(raw, plugin.QueryEnv{ActiveWindowId: "0xbb", ActiveWindowPid: 99}); err == nil {
		t.Fatal("accepted mismatched PID")
	}
	if _, err := hyprlandWorkspaceFromClients([]byte(`[{"address":"0xbb","pid":42,"mapped":true}]`), plugin.QueryEnv{ActiveWindowId: "0xbb", ActiveWindowPid: 42}); err == nil {
		t.Fatal("accepted missing workspace")
	}
}

// TestHyprlandDisplayCommandsFollowMonitorCount covers search and direct command queries.
func TestHyprlandDisplayCommandsFollowMonitorCount(t *testing.T) {
	env := plugin.QueryEnv{ActiveWindowId: "0xab", ActiveWindowPid: 42}
	p := &WindowManagerPlugin{}
	hyprlandTargetWorkspaceCache.Lock()
	hyprlandTargetWorkspaceCache.address, hyprlandTargetWorkspaceCache.pid, hyprlandTargetWorkspaceCache.workspace, hyprlandTargetWorkspaceCache.at = "0xab", 42, 1, time.Now()
	hyprlandTargetWorkspaceCache.Unlock()
	hyprlandWorkspaceCache.Lock()
	hyprlandWorkspaceCache.ids, hyprlandWorkspaceCache.checked = []int{1, 2, 3, 4, 5}, time.Now()
	hyprlandWorkspaceCache.Unlock()
	t.Cleanup(func() {
		hyprlandDisplayCache.Lock()
		hyprlandDisplayCache.at = time.Time{}
		hyprlandDisplayCache.Unlock()
		hyprlandTargetWorkspaceCache.Lock()
		hyprlandTargetWorkspaceCache.at = time.Time{}
		hyprlandTargetWorkspaceCache.Unlock()
	})
	for _, count := range []int{1, 2, 1} {
		hyprlandDisplayCache.Lock()
		hyprlandDisplayCache.count = count
		hyprlandDisplayCache.at = time.Now()
		hyprlandDisplayCache.Unlock()
		query := plugin.Query{Command: "next-display", Env: env}
		results := p.queryHyprlandWindows(context.Background(), query).Results
		if (len(results) == 1) != (count > 1) {
			t.Fatalf("monitors=%d results=%d", count, len(results))
		}
		results = p.queryHyprlandWindows(context.Background(), plugin.Query{Env: env}).Results
		var moves int
		for _, result := range results {
			if result.IdentityKey == "window-command:next-display" || result.IdentityKey == "window-command:previous-display" {
				moves++
			}
		}
		if (moves == 2) != (count > 1) {
			t.Fatalf("monitors=%d display results=%d", count, moves)
		}
	}
}

func TestHyprlandSwapNeighborStaysInWorkspaceAndUsesGeometry(t *testing.T) {
	var clients []hyprlandWindowClient
	raw := `[
		{"address":"0xa","pid":42,"mapped":true,"monitor":1,"workspace":{"id":3},"at":[0,0],"size":[800,900]},
		{"address":"0xb","pid":43,"mapped":true,"monitor":1,"workspace":{"id":3},"at":[-800,0],"size":[790,900]},
		{"address":"0xc","pid":44,"mapped":true,"monitor":1,"workspace":{"id":3},"at":[810,0],"size":[700,900]},
		{"address":"0xd","pid":45,"mapped":true,"monitor":1,"workspace":{"id":4},"at":[805,0],"size":[700,900]},
		{"address":"0xe","pid":46,"mapped":true,"monitor":1,"workspace":{"id":3},"floating":true,"at":[805,0],"size":[700,900]},
		{"address":"0xf","pid":47,"mapped":true,"monitor":1,"workspace":{"id":3},"at":[805,950],"size":[700,400]},
		{"address":"0x10","pid":48,"mapped":true,"monitor":1,"workspace":{"id":3},"at":[0,-700],"size":[800,690]},
		{"address":"0x11","pid":49,"mapped":true,"monitor":1,"workspace":{"id":3},"at":[0,910],"size":[800,400]}
	]`
	if err := json.Unmarshal([]byte(raw), &clients); err != nil {
		t.Fatal(err)
	}
	env := plugin.QueryEnv{ActiveWindowId: "0xa", ActiveWindowPid: 42}
	for _, tc := range []struct {
		op   windowOperation
		want string
	}{{"swap-left", "0xb"}, {"swap-right", "0xc"}, {"swap-up", "0x10"}, {"swap-down", "0x11"}} {
		neighbor, ok := findHyprlandSwapNeighbor(clients, env, tc.op)
		if !ok || neighbor.Address != tc.want {
			t.Fatalf("%s neighbor=%+v ok=%v", tc.op, neighbor, ok)
		}
	}
	env.ActiveWindowPid = 99
	if _, ok := findHyprlandSwapNeighbor(clients, env, "swap-left"); ok {
		t.Fatal("accepted wrong target PID")
	}
	clients[0].Floating = true
	env.ActiveWindowPid = 42
	if _, ok := findHyprlandSwapNeighbor(clients, env, "swap-right"); ok {
		t.Fatal("accepted floating target")
	}
}

func TestHyprlandSwapFocusesTargetBeforeSwapping(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls")
	script := `#!/bin/sh
if [ "$1" = -j ] && [ "$2" = clients ]; then
 printf '%s' '[{"address":"0xaa","pid":42,"mapped":true,"workspace":{"id":1},"at":[0,0],"size":[800,900]},{"address":"0xbb","pid":43,"mapped":true,"workspace":{"id":1},"at":[810,0],"size":[800,900]}]'
elif [ "$1" = -j ] && [ "$2" = activewindow ]; then
 printf '%s' '{"address":"0xaa","pid":42}'
else
 printf '%s\n' "$2" >> "$WOX_WINDOW_TEST_LOG"
 printf ok
fi
`
	if err := os.WriteFile(filepath.Join(dir, "hyprctl"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("WOX_WINDOW_TEST_LOG", logPath)
	if err := applyHyprlandWindowCommand(context.Background(), windowManagerCommand{Op: "swap-right"}, plugin.QueryEnv{ActiveWindowId: "0xaa", ActiveWindowPid: 42}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `hl.dsp.focus({ window = "address:0xaa" })`) || !strings.Contains(lines[1], `hl.dsp.window.swap({ target = "address:0xbb" })`) {
		t.Fatalf("dispatches = %q", data)
	}
}

func TestHyprlandSwapResultsOnlyAppearWithNeighbor(t *testing.T) {
	var clients []hyprlandWindowClient
	if err := json.Unmarshal([]byte(`[{"address":"0xaa","pid":42,"mapped":true,"monitor":1,"workspace":{"id":1},"at":[0,0],"size":[800,900]},{"address":"0xbb","pid":43,"mapped":true,"monitor":1,"workspace":{"id":1},"at":[810,0],"size":[800,900]}]`), &clients); err != nil {
		t.Fatal(err)
	}
	hyprlandTargetWorkspaceCache.Lock()
	old := hyprlandTargetWorkspaceCache.clients
	hyprlandTargetWorkspaceCache.address, hyprlandTargetWorkspaceCache.pid, hyprlandTargetWorkspaceCache.workspace, hyprlandTargetWorkspaceCache.at = "0xaa", 42, 1, time.Now()
	hyprlandTargetWorkspaceCache.clients = clients
	hyprlandTargetWorkspaceCache.Unlock()
	t.Cleanup(func() {
		hyprlandTargetWorkspaceCache.Lock()
		hyprlandTargetWorkspaceCache.at = time.Time{}
		hyprlandTargetWorkspaceCache.clients = old
		hyprlandTargetWorkspaceCache.Unlock()
	})
	env := plugin.QueryEnv{ActiveWindowId: "0xaa", ActiveWindowPid: 42}
	p := &WindowManagerPlugin{}
	for _, tc := range []struct {
		command string
		want    int
	}{{"swap-left", 0}, {"swap-right", 1}, {"swap-up", 0}, {"swap-down", 0}} {
		results := p.queryHyprlandWindows(context.Background(), plugin.Query{Command: tc.command, Env: env}).Results
		if len(results) != tc.want {
			t.Fatalf("%s results=%d want=%d", tc.command, len(results), tc.want)
		}
	}
}

func TestHyprlandMaximizeToggleAvailableInEveryWindowState(t *testing.T) {
	env := plugin.QueryEnv{ActiveWindowId: "0xab", ActiveWindowPid: 42}
	client := hyprlandWindowClient{Address: "0xab", Pid: 42, Mapped: true}
	t.Cleanup(func() {
		hyprlandTargetWorkspaceCache.Lock()
		hyprlandTargetWorkspaceCache.at = time.Time{}
		hyprlandTargetWorkspaceCache.clients = nil
		hyprlandTargetWorkspaceCache.Unlock()
	})
	for _, state := range []int{0, 1, 2, 3} {
		client.Fullscreen = state
		hyprlandTargetWorkspaceCache.Lock()
		hyprlandTargetWorkspaceCache.address, hyprlandTargetWorkspaceCache.pid = "0xab", 42
		hyprlandTargetWorkspaceCache.workspace, hyprlandTargetWorkspaceCache.at = 1, time.Now()
		hyprlandTargetWorkspaceCache.clients = []hyprlandWindowClient{client}
		hyprlandTargetWorkspaceCache.Unlock()
		p := &WindowManagerPlugin{}
		maximize := p.queryHyprlandWindows(context.Background(), plugin.Query{Command: "maximize", Env: env}).Results
		if len(maximize) != 1 {
			t.Fatalf("state=%d maximize=%d", state, len(maximize))
		}
	}
}
