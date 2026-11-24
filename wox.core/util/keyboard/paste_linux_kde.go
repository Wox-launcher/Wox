//go:build linux

package keyboard

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/godbus/dbus/v5"
)

// SimulateKDEPasteAfterFocus waits for KWin to receive the keyboard before
// restoring focus, so device startup cannot delay a confirmed paste.
func SimulateKDEPasteAfterFocus(ctx context.Context, shift bool, focus func() error) error {
	return simulateCtrlKeyCombo(evKeyCodeV, "paste", shift, func(fd int) error {
		eventName, err := uinputEventName(fd)
		if err != nil {
			return err
		}
		if err := waitForKDEKeyboard(ctx, eventName); err != nil {
			return err
		}
		return focus()
	})
}

// uinputEventName resolves the event node for this file descriptor. Multiple
// Wox processes can create keyboards with the same display name.
func uinputEventName(fd int) (string, error) {
	var sysname [64]byte
	const uiGetSysname = (2 << 30) | (uintptr(len(sysname)) << 16) | ('U' << 8) | 44 // UI_GET_SYSNAME(len)
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uiGetSysname, uintptr(unsafe.Pointer(&sysname[0])))
	if errno != 0 {
		return "", fmt.Errorf("UI_GET_SYSNAME: %w", errno)
	}
	name, _, _ := strings.Cut(string(sysname[:]), "\x00")
	entries, err := os.ReadDir(filepath.Join("/sys/class/input", name))
	if err != nil {
		return "", fmt.Errorf("find event node for uinput device %s: %w", name, err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "event") {
			return entry.Name(), nil
		}
	}
	return "", fmt.Errorf("uinput device %s has no event node", name)
}

// waitForKDEKeyboard bounds the wait for KWin's asynchronous device discovery.
func waitForKDEKeyboard(ctx context.Context, eventName string) error {
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	conn, err := dbus.ConnectSessionBus(dbus.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("KWin input device bus: %w", err)
	}
	defer conn.Close()
	manager := conn.Object("org.kde.KWin", "/org/kde/KWin/InputDevice")
	return waitForKDEInputDevice(ctx, manager, eventName)
}

// waitForKDEInputDevice checks immediately for a keyboard already in use and
// lets cold starts proceed as soon as KWin reports the exact event node.
func waitForKDEInputDevice(ctx context.Context, manager dbus.BusObject, eventName string) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var devices []string
		err := manager.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0,
			"org.kde.KWin.InputDeviceManager", "devicesSysNames").Store(&devices)
		if err != nil {
			return fmt.Errorf("read KWin input devices: %w", err)
		}
		if slices.Contains(devices, eventName) {
			return ctx.Err()
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("KWin did not register uinput device %s: %w", eventName, ctx.Err())
		case <-ticker.C:
		}
	}
}
