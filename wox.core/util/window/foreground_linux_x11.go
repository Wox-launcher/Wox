//go:build linux

package window

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// queryX11Foreground reads the EWMH active window. WM_CLASS is the application
// id X11 clients publish; the clipboard selection owner is a different window
// and is not what the other platforms record.
func queryX11Foreground(ctx context.Context) (linuxForegroundApp, error) {
	output, err := exec.CommandContext(ctx, "xprop", "-root", "_NET_ACTIVE_WINDOW").Output()
	if err != nil {
		return linuxForegroundApp{}, fmt.Errorf("xprop active window: %w", err)
	}
	windowID := parseX11ActiveWindowID(string(output))
	if windowID == "" {
		return linuxForegroundApp{}, errLinuxForegroundMissing
	}
	output, err = exec.CommandContext(ctx, "xprop", "-id", windowID, "WM_CLASS", "_NET_WM_PID", "_NET_WM_NAME").Output()
	if err != nil {
		return linuxForegroundApp{}, fmt.Errorf("xprop window %s: %w", windowID, err)
	}
	return parseX11ForegroundProperties(string(output))
}

// parseX11ActiveWindowID extracts the window id from `_NET_ACTIVE_WINDOW(WINDOW): window id # 0x123`.
func parseX11ActiveWindowID(output string) string {
	_, id, ok := strings.Cut(output, "#")
	if !ok {
		return ""
	}
	id = strings.TrimSpace(id)
	id = strings.TrimPrefix(id, "0x")
	id = strings.TrimPrefix(id, "0X")
	if id == "" || strings.ContainsAny(id, " \t\r\n") {
		return ""
	}
	value, err := strconv.ParseUint(id, 16, 32)
	if err != nil || value == 0 {
		return ""
	}
	return "0x" + id
}

func parseX11ForegroundProperties(output string) (linuxForegroundApp, error) {
	app := linuxForegroundApp{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "WM_CLASS"):
			quoted := quotedXPropStrings(line)
			if len(quoted) > 0 {
				app.AppID = quoted[0]
			}
			if len(quoted) > 1 {
				app.AltID = quoted[1]
			}
		case strings.HasPrefix(line, "_NET_WM_PID"):
			fields := strings.Fields(line)
			if len(fields) > 0 {
				pid, err := strconv.Atoi(fields[len(fields)-1])
				if err == nil {
					app.Pid = pid
				}
			}
		case strings.HasPrefix(line, "_NET_WM_NAME"):
			quoted := quotedXPropStrings(line)
			if len(quoted) > 0 {
				app.Title = quoted[0]
			}
		}
	}
	if normalizeLinuxAppID(app.AppID) == "" && normalizeLinuxAppID(app.AltID) == "" && app.Pid <= 0 {
		return linuxForegroundApp{}, errLinuxForegroundMissing
	}
	return app, nil
}

func quotedXPropStrings(line string) []string {
	var quoted []string
	for index := 0; index < len(line); index++ {
		if line[index] != '"' {
			continue
		}
		var value strings.Builder
		index++
		for index < len(line) {
			if line[index] == '\\' && index+1 < len(line) {
				value.WriteByte(line[index+1])
				index += 2
				continue
			}
			if line[index] == '"' {
				break
			}
			value.WriteByte(line[index])
			index++
		}
		quoted = append(quoted, value.String())
	}
	return quoted
}
