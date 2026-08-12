//go:build linux

package screen

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const hyprlandIPCTimeout = 100 * time.Millisecond

type hyprlandCursorPosition struct {
	X *float64 `json:"x"`
	Y *float64 `json:"y"`
}

// HyprlandMonitor describes an output using compositor-owned physical mode and fractional scale data.
type HyprlandMonitor struct {
	Name      string  `json:"name"`
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	X         int     `json:"x"`
	Y         int     `json:"y"`
	Scale     float64 `json:"scale"`
	Transform int     `json:"transform"`
}

// LogicalSize converts the physical mode to the monitor's global logical geometry.
func (monitor HyprlandMonitor) LogicalSize() Size {
	scale := monitor.Scale
	if scale <= 0 {
		scale = 1
	}
	width := monitor.Width
	height := monitor.Height
	if monitor.Transform == 1 || monitor.Transform == 3 || monitor.Transform == 5 || monitor.Transform == 7 {
		width, height = height, width
	}
	return Size{X: monitor.X, Y: monitor.Y, Width: int(float64(width)/scale + 0.5), Height: int(float64(height)/scale + 0.5)}
}

// PixelSize returns the transformed physical dimensions used by compositor capture streams.
func (monitor HyprlandMonitor) PixelSize() Size {
	width := monitor.Width
	height := monitor.Height
	if monitor.Transform == 1 || monitor.Transform == 3 || monitor.Transform == 5 || monitor.Transform == 7 {
		width, height = height, width
	}
	return Size{Width: width, Height: height}
}

// GetHyprlandMonitors returns compositor-owned output geometry and fractional scale data.
func GetHyprlandMonitors() ([]HyprlandMonitor, error) {
	response, err := queryHyprlandIPC("j/monitors", 1024*1024)
	if err != nil {
		return nil, err
	}
	var monitors []HyprlandMonitor
	if err := json.Unmarshal(response, &monitors); err != nil {
		return nil, fmt.Errorf("parse Hyprland monitors: %w", err)
	}
	if len(monitors) == 0 {
		return nil, fmt.Errorf("parse Hyprland monitors: no monitors")
	}
	return monitors, nil
}

// getHyprlandCursorPosition reads compositor-owned cursor coordinates without spawning a process.
func getHyprlandCursorPosition() (int, int, error) {
	response, err := queryHyprlandIPC("j/cursorpos", 4096)
	if err != nil {
		return 0, 0, err
	}
	return parseHyprlandCursorPosition(response)
}

func queryHyprlandIPC(command string, responseLimit int64) ([]byte, error) {
	signature := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
	if signature == "" {
		return nil, fmt.Errorf("HYPRLAND_INSTANCE_SIGNATURE is empty")
	}

	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = os.TempDir()
	}
	socketPath := filepath.Join(runtimeDir, "hypr", signature, ".socket.sock")
	connection, err := net.DialTimeout("unix", socketPath, hyprlandIPCTimeout)
	if err != nil {
		return nil, fmt.Errorf("connect to Hyprland IPC: %w", err)
	}
	defer connection.Close()

	if err := connection.SetDeadline(time.Now().Add(hyprlandIPCTimeout)); err != nil {
		return nil, fmt.Errorf("set Hyprland IPC deadline: %w", err)
	}
	if _, err := connection.Write([]byte(command)); err != nil {
		return nil, fmt.Errorf("request Hyprland IPC %s: %w", command, err)
	}
	response, err := io.ReadAll(io.LimitReader(connection, responseLimit))
	if err != nil {
		return nil, fmt.Errorf("read Hyprland IPC %s: %w", command, err)
	}
	return response, nil
}

// parseHyprlandCursorPosition validates the compositor response before converting it to GTK coordinates.
func parseHyprlandCursorPosition(response []byte) (int, int, error) {
	var position hyprlandCursorPosition
	if err := json.Unmarshal(response, &position); err != nil {
		return 0, 0, fmt.Errorf("parse Hyprland cursor position: %w", err)
	}
	if position.X == nil || position.Y == nil {
		return 0, 0, fmt.Errorf("parse Hyprland cursor position: missing x or y")
	}
	return int(*position.X), int(*position.Y), nil
}

func isHyprlandSession() bool {
	return os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != "" || strings.Contains(strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP")), "hyprland")
}
