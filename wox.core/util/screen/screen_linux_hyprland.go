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

// getHyprlandCursorPosition reads compositor-owned cursor coordinates without spawning a process.
func getHyprlandCursorPosition() (int, int, error) {
	signature := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
	if signature == "" {
		return 0, 0, fmt.Errorf("HYPRLAND_INSTANCE_SIGNATURE is empty")
	}

	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = os.TempDir()
	}
	socketPath := filepath.Join(runtimeDir, "hypr", signature, ".socket.sock")
	connection, err := net.DialTimeout("unix", socketPath, hyprlandIPCTimeout)
	if err != nil {
		return 0, 0, fmt.Errorf("connect to Hyprland IPC: %w", err)
	}
	defer connection.Close()

	if err := connection.SetDeadline(time.Now().Add(hyprlandIPCTimeout)); err != nil {
		return 0, 0, fmt.Errorf("set Hyprland IPC deadline: %w", err)
	}
	if _, err := connection.Write([]byte("j/cursorpos")); err != nil {
		return 0, 0, fmt.Errorf("request Hyprland cursor position: %w", err)
	}
	response, err := io.ReadAll(io.LimitReader(connection, 4096))
	if err != nil {
		return 0, 0, fmt.Errorf("read Hyprland cursor position: %w", err)
	}

	return parseHyprlandCursorPosition(response)
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
