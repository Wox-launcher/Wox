//go:build linux

package keyboard

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

type fakeKDEInputManager struct {
	dbus.BusObject
	devices func() []string
}

func (f fakeKDEInputManager) CallWithContext(context.Context, string, dbus.Flags, ...any) *dbus.Call {
	return &dbus.Call{Body: []any{dbus.MakeVariant(f.devices())}}
}

func TestKDEInputDeviceWaitsForExactEventNode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	checks := 0
	manager := fakeKDEInputManager{devices: func() []string {
		checks++
		if checks == 1 {
			return []string{"event3", "event90"}
		}
		return []string{"event3", "event90", "event9"}
	}}
	if err := waitForKDEInputDevice(ctx, manager, "event9"); err != nil {
		t.Fatal(err)
	}
	if checks != 2 {
		t.Fatalf("checked for keyboard %d times, want 2", checks)
	}
}

func TestKDEInputDeviceTimesOutBeforeRegistration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	manager := fakeKDEInputManager{devices: func() []string { return []string{"event3"} }}
	if err := waitForKDEInputDevice(ctx, manager, "event9"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected device registration timeout, got %v", err)
	}
}
