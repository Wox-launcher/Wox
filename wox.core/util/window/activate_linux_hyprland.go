//go:build linux

package window

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"wox/util"
)

const hyprlandActivationSettle = 40 * time.Millisecond

// linuxActivationWindow is one compositor client that might belong to an app.
type linuxActivationWindow struct {
	Address        string
	Class          string
	InitialClass   string
	Pid            int
	FocusHistoryID int
	Mapped         bool
	Hidden         bool
}

// listHyprlandAppWindows reads top-level clients for running-app attribution.
func listHyprlandAppWindows(ctx context.Context) []LinuxAppWindow {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 200*time.Millisecond)
		defer cancel()
	}
	output, err := exec.CommandContext(ctx, "hyprctl", "-j", "clients").Output()
	if err != nil {
		return nil
	}
	parsed, err := parseHyprlandActivationWindows(output)
	if err != nil {
		return nil
	}
	windows := make([]LinuxAppWindow, 0, len(parsed))
	for _, item := range parsed {
		if item.Pid <= 0 {
			continue
		}
		windows = append(windows, LinuxAppWindow{Pid: item.Pid, Class: item.Class, AltID: item.InitialClass})
	}
	return windows
}

// findHyprlandActivationAddress picks the best existing window for this launcher.
func findHyprlandActivationAddress(ctx context.Context, app linuxDesktopApp) (string, bool) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
	}
	output, err := exec.CommandContext(ctx, "hyprctl", "-j", "clients").Output()
	if err != nil {
		util.GetLogger().Info(ctx, fmt.Sprintf("linux app activation: hyprctl clients: %s", err.Error()))
		return "", false
	}
	windows, err := parseHyprlandActivationWindows(output)
	if err != nil {
		util.GetLogger().Info(ctx, fmt.Sprintf("linux app activation: hyprctl clients: %s", err.Error()))
		return "", false
	}
	chosen, ok := selectLinuxActivationWindow(app, windows, linuxProcCommandIDs)
	if !ok {
		return "", false
	}
	return chosen.Address, true
}

// focusHyprlandActivation asks Hyprland 0.56+ to focus one window by address.
// The older `dispatch focuswindow address:` form is rejected by the Lua dispatcher.
func focusHyprlandActivation(ctx context.Context, address string) bool {
	script := hyprlandFocusDispatch(address)
	if script == "" {
		return false
	}
	deadline := time.Now().Add(300 * time.Millisecond)
	var logged bool
	for {
		if err := hyprlandDispatch(ctx, script); err != nil {
			if !logged {
				util.GetLogger().Info(ctx, fmt.Sprintf("linux app activation: %s", err.Error()))
				logged = true
			}
		} else if hyprlandActiveAddress(ctx) == address && hyprlandFocusHeld(ctx, address) {
			return true
		}
		if time.Now().After(deadline) {
			return hyprlandActiveAddress(ctx) == address
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// hyprlandFocusHeld confirms the compositor did not immediately restore another window.
func hyprlandFocusHeld(ctx context.Context, address string) bool {
	timer := time.NewTimer(hyprlandActivationSettle)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
	}
	return hyprlandActiveAddress(ctx) == address
}

func hyprlandDispatch(ctx context.Context, script string) error {
	output, err := exec.CommandContext(ctx, "hyprctl", "dispatch", script).CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		if text == "" {
			return err
		}
		return fmt.Errorf("%s", text)
	}
	if text != "ok" {
		return fmt.Errorf("%s", text)
	}
	return nil
}

func hyprlandActiveAddress(ctx context.Context) string {
	output, err := exec.CommandContext(ctx, "hyprctl", "-j", "activewindow").Output()
	if err != nil {
		return ""
	}
	var active struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal(output, &active); err != nil {
		return ""
	}
	return active.Address
}

// hyprlandFocusDispatch builds the Lua dispatcher for one validated address.
func hyprlandFocusDispatch(address string) string {
	if !hyprlandWindowAddressOK(address) {
		return ""
	}
	return fmt.Sprintf("hl.dsp.focus({ window = 'address:%s' })", address)
}

func hyprlandWindowAddressOK(address string) bool {
	if len(address) < 3 || (address[0] != '0' || (address[1] != 'x' && address[1] != 'X')) {
		return false
	}
	hex := address[2:]
	if len(hex) > 32 {
		return false
	}
	for _, r := range hex {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

func parseHyprlandActivationWindows(output []byte) ([]linuxActivationWindow, error) {
	var raw []struct {
		Address        string `json:"address"`
		Class          string `json:"class"`
		InitialClass   string `json:"initialClass"`
		Pid            int    `json:"pid"`
		FocusHistoryID int    `json:"focusHistoryID"`
		Mapped         bool   `json:"mapped"`
		Hidden         bool   `json:"hidden"`
	}
	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("hyprctl clients: %w", err)
	}
	windows := make([]linuxActivationWindow, 0, len(raw))
	for _, item := range raw {
		if !hyprlandWindowAddressOK(item.Address) {
			continue
		}
		windows = append(windows, linuxActivationWindow{
			Address:        item.Address,
			Class:          item.Class,
			InitialClass:   item.InitialClass,
			Pid:            item.Pid,
			FocusHistoryID: item.FocusHistoryID,
			Mapped:         item.Mapped,
			Hidden:         item.Hidden,
		})
	}
	return windows, nil
}

func linuxProcCommandIDs(pid int) []string {
	exe, comm := linuxProcCommand(pid)
	ids := make([]string, 0, 2)
	if exe != "" {
		ids = append(ids, exe)
	}
	if comm != "" {
		ids = append(ids, comm)
	}
	return ids
}

type linuxActivationCandidate struct {
	window linuxActivationWindow
	score  int
}

// selectLinuxActivationWindow chooses the matching window the user focused most recently.
// Mapped windows outrank unmapped ones. A hidden grouped window still wins when it was used last.
func selectLinuxActivationWindow(app linuxDesktopApp, windows []linuxActivationWindow, idsForPid func(int) []string) (linuxActivationWindow, bool) {
	var mapped []linuxActivationCandidate
	var rest []linuxActivationCandidate
	for _, item := range windows {
		var extra []string
		if idsForPid != nil && item.Pid > 0 {
			extra = idsForPid(item.Pid)
		}
		ids := make([]string, 0, 2+len(extra))
		ids = append(ids, item.Class, item.InitialClass)
		ids = append(ids, extra...)
		score := linuxDesktopScore(app, ids...)
		if score <= 0 {
			continue
		}
		chosen := linuxActivationCandidate{window: item, score: score}
		if item.Mapped {
			mapped = append(mapped, chosen)
		} else {
			rest = append(rest, chosen)
		}
	}
	pool := mapped
	if len(pool) == 0 {
		pool = rest
	}
	if len(pool) == 0 {
		return linuxActivationWindow{}, false
	}
	best := pool[0]
	for _, item := range pool[1:] {
		if betterLinuxActivation(item, best) {
			best = item
		}
	}
	return best.window, true
}

func betterLinuxActivation(item linuxActivationCandidate, best linuxActivationCandidate) bool {
	if item.score != best.score {
		return item.score > best.score
	}
	if item.window.FocusHistoryID != best.window.FocusHistoryID {
		return item.window.FocusHistoryID > best.window.FocusHistoryID
	}
	if item.window.Hidden != best.window.Hidden {
		return !item.window.Hidden
	}
	return item.window.Address < best.window.Address
}
