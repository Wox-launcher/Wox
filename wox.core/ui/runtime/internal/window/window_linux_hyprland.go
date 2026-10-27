//go:build linux

package window

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"wox/util"
)

const hyprlandLayerNoAnimRule = "wox-no-layer-anim"

// linuxHyprlandWindowPolicy uses layer-shell for utility placement and focus.
func linuxHyprlandWindowPolicy() linuxWindowPolicy {
	return linuxWindowPolicy{name: "hyprland", layerShell: true}
}

func isLinuxHyprlandWindowSession(desktop string) bool {
	return strings.Contains(desktop, "hyprland")
}

var hyprlandLayerRule struct {
	mu       sync.Mutex
	ready    bool
	watching bool
	logOnce  sync.Once
}

// ensureHyprlandLayerNoAnim stops Hyprland's layer geometry animation for Wox.
// Omarchy enables the layers animation, so each launcher height change eases
// for several hundred milliseconds. Theme switches run hyprctl reload, which
// drops this runtime rule, so the event socket installs it again.
func ensureHyprlandLayerNoAnim() {
	if !util.IsHyprlandSession() {
		return
	}
	startHyprlandLayerRuleWatch()
	if err := ensureHyprlandLayerNoAnimWith(hyprctlEval); err != nil {
		hyprlandLayerRule.logOnce.Do(func() {
			util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("failed to disable Hyprland layer animation: %s", err.Error()))
		})
	}
}

func ensureHyprlandLayerNoAnimWith(eval func(script string) error) error {
	hyprlandLayerRule.mu.Lock()
	defer hyprlandLayerRule.mu.Unlock()
	if hyprlandLayerRule.ready {
		return nil
	}
	if err := eval(hyprlandLayerNoAnimScript(util.LinuxDesktopAppID)); err != nil {
		return err
	}
	hyprlandLayerRule.ready = true
	return nil
}

// restoreHyprlandLayerNoAnim installs the rule again after Hyprland drops it.
func restoreHyprlandLayerNoAnim(eval func(script string) error) error {
	hyprlandLayerRule.mu.Lock()
	hyprlandLayerRule.ready = false
	hyprlandLayerRule.mu.Unlock()
	return ensureHyprlandLayerNoAnimWith(eval)
}

// startHyprlandLayerRuleWatch follows Hyprland's event socket for config reloads.
func startHyprlandLayerRuleWatch() {
	hyprlandLayerRule.mu.Lock()
	if hyprlandLayerRule.watching {
		hyprlandLayerRule.mu.Unlock()
		return
	}
	hyprlandLayerRule.watching = true
	hyprlandLayerRule.mu.Unlock()

	util.Go(util.NewTraceContext(), "watch hyprland layer animation", watchHyprlandConfigReloads)
}

// watchHyprlandConfigReloads reinstalls the layer rule after each Hyprland reload.
// Omarchy's theme switch runs `hyprctl reload`, and that reload removes rules
// created with hyprctl eval.
func watchHyprlandConfigReloads() {
	ctx := util.NewTraceContext()
	var logged bool
	for {
		err := readHyprlandConfigReloads(func(reason string) {
			if restoreErr := restoreHyprlandLayerNoAnim(hyprctlEval); restoreErr != nil {
				util.GetLogger().Warn(ctx, fmt.Sprintf("failed to restore Hyprland layer animation rule: %s", restoreErr.Error()))
				return
			}
			if reason == "config reload" {
				util.GetLogger().Info(ctx, "restored Hyprland layer animation rule after config reload")
			}
		})
		if err != nil && !logged {
			util.GetLogger().Warn(ctx, fmt.Sprintf("Hyprland event socket: %s", err.Error()))
			logged = true
		}
		if err == nil {
			logged = false
		}
		time.Sleep(time.Second)
	}
}

// readHyprlandConfigReloads keeps one event-socket connection. The rule is
// installed when the socket connects, because a reload may have dropped it
// while the socket was down, and again on each configreloaded event.
func readHyprlandConfigReloads(restore func(reason string)) error {
	socketPath, err := hyprlandEventSocketPath()
	if err != nil {
		return err
	}
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return err
	}
	defer conn.Close()
	restore("socket connected")

	reader := bufio.NewReader(conn)
	for {
		line, readErr := reader.ReadString('\n')
		if hyprlandConfigReloaded(line) {
			restore("config reload")
		}
		if readErr != nil {
			return readErr
		}
	}
}

// hyprlandEventSocketPath is Hyprland's runtime event socket for this session.
func hyprlandEventSocketPath() (string, error) {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	signature := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
	if runtimeDir == "" || signature == "" {
		return "", fmt.Errorf("Hyprland event socket is not available")
	}
	return filepath.Join(runtimeDir, "hypr", signature, ".socket2.sock"), nil
}

// hyprlandConfigReloaded reports Hyprland's configreloaded event.
func hyprlandConfigReloaded(line string) bool {
	name, _, found := strings.Cut(strings.TrimSpace(line), ">>")
	return found && name == "configreloaded"
}

// hyprlandLayerNoAnimScript installs one compositor-side layer rule. The Lua
// global keeps later Wox processes from appending another no_anim effect.
// A Hyprland reload drops the global and the rule together.
func hyprlandLayerNoAnimScript(namespace string) string {
	return fmt.Sprintf(`if wox_layer_no_anim ~= nil then return "exists" end
wox_layer_no_anim = hl.layer_rule({ name = %q, match = { namespace = [[%s]] }, no_anim = true })
return "installed"`, hyprlandLayerNoAnimRule, hyprlandNamespacePattern(namespace))
}

// hyprlandNamespacePattern anchors a layer namespace for Hyprland's RE2 match.
func hyprlandNamespacePattern(namespace string) string {
	var pattern strings.Builder
	pattern.WriteByte('^')
	for _, r := range namespace {
		if strings.ContainsRune(`\.+*?()|[]{}^$`, r) {
			pattern.WriteByte('\\')
		}
		pattern.WriteRune(r)
	}
	pattern.WriteByte('$')
	return pattern.String()
}

func hyprctlEval(script string) error {
	output, err := exec.Command("hyprctl", "eval", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, bytes.TrimSpace(output))
	}
	return nil
}
