//go:build linux

package window

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"sync"

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
	mu      sync.Mutex
	ready   bool
	logOnce sync.Once
}

// ensureHyprlandLayerNoAnim stops Hyprland's layer geometry animation for Wox.
// Omarchy enables the layers animation, so each launcher height change eases
// for several hundred milliseconds.
func ensureHyprlandLayerNoAnim() {
	if !util.IsHyprlandSession() {
		return
	}
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
