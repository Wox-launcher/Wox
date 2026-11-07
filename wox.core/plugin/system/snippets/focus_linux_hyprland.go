//go:build linux && cgo

package snippets

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type hyprlandWindow struct {
	Address      string `json:"address"`
	Class        string `json:"class"`
	InitialClass string `json:"initialClass"`
}

// hyprlandFocus tracks top-level focus when an application has no accessibility bridge.
type hyprlandFocus struct {
	conn     net.Conn
	mu       sync.Mutex
	window   hyprlandWindow
	token    uint64
	sequence uint64
	onChange func()
	done     chan struct{}
}

// startHyprlandFocus subscribes before taking the initial snapshot so focus changes are not missed.
func startHyprlandFocus(onChange func()) (*hyprlandFocus, error) {
	runtimeDir, signature := os.Getenv("XDG_RUNTIME_DIR"), os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
	if runtimeDir == "" || signature == "" {
		return nil, errors.New("Hyprland event socket is unavailable")
	}
	conn, err := net.DialTimeout("unix", filepath.Join(runtimeDir, "hypr", signature, ".socket2.sock"), time.Second)
	if err != nil {
		return nil, err
	}
	f := &hyprlandFocus{conn: conn, onChange: onChange, done: make(chan struct{})}
	f.refresh("")
	go f.listen()
	return f, nil
}

// queryHyprlandSnippetWindow bounds compositor IPC outside the input callback.
func queryHyprlandSnippetWindow() (hyprlandWindow, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	output, err := exec.CommandContext(ctx, "hyprctl", "-j", "activewindow").Output()
	if err != nil {
		return hyprlandWindow{}, err
	}
	var window hyprlandWindow
	if err := json.Unmarshal(output, &window); err != nil {
		return hyprlandWindow{}, err
	}
	if window.Address == "" || window.Address == "0x0" {
		return hyprlandWindow{}, errors.New("no active window")
	}
	return window, nil
}

// refresh blocks matching until the compositor snapshot agrees with the focus event.
func (f *hyprlandFocus) refresh(address string) {
	f.mu.Lock()
	if address != "" && f.token != 0 && strings.EqualFold(strings.TrimPrefix(f.window.Address, "0x"), address) {
		f.mu.Unlock()
		return
	}
	f.sequence++
	sequence := f.sequence
	f.token = 0
	f.window = hyprlandWindow{}
	f.mu.Unlock()
	f.onChange()
	window, err := queryHyprlandSnippetWindow()
	if err != nil || (address != "" && !strings.EqualFold(strings.TrimPrefix(window.Address, "0x"), address)) {
		return
	}
	f.mu.Lock()
	if f.sequence == sequence {
		f.window = window
		f.token = sequence
	}
	f.mu.Unlock()
}

// listen drops the target on disconnect rather than trusting stale window state.
func (f *hyprlandFocus) listen() {
	defer close(f.done)
	defer func() {
		f.mu.Lock()
		f.sequence++
		f.token = 0
		f.mu.Unlock()
		f.onChange()
	}()
	scanner := bufio.NewScanner(f.conn)
	for scanner.Scan() {
		name, value, ok := strings.Cut(scanner.Text(), ">>")
		if ok && name == "activewindowv2" {
			if value == "" {
				f.mu.Lock()
				f.sequence++
				f.token = 0
				f.mu.Unlock()
				f.onChange()
				continue
			}
			f.refresh(value)
		}
	}
}

func (f *hyprlandFocus) fast() uint64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.token
}

// verifiedTarget rechecks the active window before injecting any input.
func (f *hyprlandFocus) verifiedTarget() uint64 {
	f.mu.Lock()
	token, address := f.token, f.window.Address
	f.mu.Unlock()
	if token == 0 {
		return 0
	}
	window, err := queryHyprlandSnippetWindow()
	if err != nil || window.Address != address || f.fast() != token {
		return 0
	}
	return token
}

// isTerminal selects the paste shortcut for standalone terminal applications.
func (f *hyprlandFocus) isTerminal(token uint64) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if token == 0 || f.token != token {
		return false
	}
	for _, class := range []string{f.window.Class, f.window.InitialClass} {
		switch strings.ToLower(class) {
		case "foot", "footclient", "kitty", "alacritty", "org.wezfurlong.wezterm", "wezterm", "com.mitchellh.ghostty", "ghostty", "org.gnome.terminal", "konsole", "org.kde.konsole", "xterm":
			return true
		}
	}
	return false
}

func (f *hyprlandFocus) close() {
	if f == nil {
		return
	}
	_ = f.conn.Close()
	<-f.done
}
