//go:build linux && cgo

package snippets

import (
	"bufio"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

func TestAccessibleTargetAllowed(t *testing.T) {
	focused := uint32(1 << atspiStateFocused)
	editable := uint32(1 << atspiStateEditable)
	for _, test := range []struct {
		name   string
		role   atspiRole
		states []uint32
		want   bool
	}{
		{"editable", 79, []uint32{focused | editable, 0}, true},
		{"terminal", atspiRoleTerminal, []uint32{focused, 0}, true},
		{"password", atspiRolePassword, []uint32{focused | editable, 0}, false},
		{"not focused", 79, []uint32{editable, 0}, false},
		{"not editable", 79, []uint32{focused, 0}, false},
		{"defunct", 79, []uint32{focused | editable | 1<<6, 0}, false},
		{"invalid response", 79, []uint32{focused | editable}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := accessibleTargetAllowed(test.role, test.states); got != test.want {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}

type fakeAccessibilityBus struct{ address string }

func (s *fakeAccessibilityBus) GetAddress() (string, *dbus.Error) { return s.address, nil }

type fakeAccessibilityRegistry struct{}

func (fakeAccessibilityRegistry) RegisterEvent(string, []string, string) *dbus.Error { return nil }

type fakeAccessible struct {
	role   atomic.Uint32
	states atomic.Uint32
}

func (s *fakeAccessible) GetRole() (uint32, *dbus.Error)    { return s.role.Load(), nil }
func (s *fakeAccessible) GetState() ([]uint32, *dbus.Error) { return []uint32{s.states.Load(), 0}, nil }

// TestWaylandFocusTracksRealDBusSignals exercises the focus protocol on a private session bus.
func TestWaylandFocusTracksRealDBusSignals(t *testing.T) {
	path, err := exec.LookPath("dbus-daemon")
	if err != nil {
		t.Skip("dbus-daemon is not installed")
	}
	daemon := exec.Command(path, "--session", "--nofork", "--print-address=1")
	output, err := daemon.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := daemon.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = daemon.Process.Kill(); _ = daemon.Wait() })
	address, err := bufio.NewReader(output).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	address = strings.TrimSpace(address)
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", address)
	server, err := dbus.Connect(address)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	for _, name := range []string{"org.a11y.Bus", atspiRegistry} {
		if reply, err := server.RequestName(name, dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
			t.Fatalf("request %s: %v %v", name, reply, err)
		}
	}
	if err := server.Export(&fakeAccessibilityBus{address: address}, "/org/a11y/bus", "org.a11y.Bus"); err != nil {
		t.Fatal(err)
	}
	if err := server.Export(fakeAccessibilityRegistry{}, "/org/a11y/atspi/registry", "org.a11y.atspi.Registry"); err != nil {
		t.Fatal(err)
	}
	item := &fakeAccessible{}
	item.role.Store(79)
	item.states.Store(1<<atspiStateFocused | 1<<atspiStateEditable)
	objectPath := dbus.ObjectPath("/org/a11y/atspi/accessible/1")
	if err := server.Export(item, objectPath, "org.a11y.atspi.Accessible"); err != nil {
		t.Fatal(err)
	}
	changed := make(chan struct{}, 4)
	focus, err := startAccessibilityFocus(func() { changed <- struct{}{} })
	if err != nil {
		t.Fatal(err)
	}
	defer focus.close()
	emit := func(enabled int32) {
		t.Helper()
		if err := server.Emit(objectPath, "org.a11y.atspi.Event.Object.StateChanged", "focused", enabled, int32(0), dbus.MakeVariant(""), map[string]dbus.Variant{}); err != nil {
			t.Fatal(err)
		}
		select {
		case <-changed:
		case <-time.After(time.Second):
			t.Fatal("focus signal was not received")
		}
	}
	emit(1)
	deadline := time.Now().Add(time.Second)
	for focus.fast() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := focus.verifiedTarget(); got == 0 {
		t.Fatal("focused editable control was not accepted")
	}
	item.states.Store(0)
	if got := focus.verifiedTarget(); got != 0 {
		t.Fatal("stale focus was accepted")
	}
	emit(0)
	if got := focus.fast(); got != 0 {
		t.Fatal("focus loss did not clear target")
	}
	item.role.Store(uint32(atspiRolePassword))
	item.states.Store(1<<atspiStateFocused | 1<<atspiStateEditable)
	emit(1)
	time.Sleep(50 * time.Millisecond)
	if got := focus.fast(); got != 0 {
		t.Fatal("password control was accepted")
	}
	if !focus.blocked.Load() {
		t.Fatal("password control did not block window fallback")
	}
	focus.reset()
	if focus.blocked.Load() {
		t.Fatal("window change did not discard the previous control")
	}
	if os.Getenv("DISPLAY") != "" {
		backend := newX11ExpansionBackend()
		stop, err := backend.start(func(inputEvent) {})
		if err != nil {
			t.Fatalf("start X11 listener with accessibility bus: %v", err)
		}
		if err := stop(); err != nil {
			t.Fatalf("stop X11 listener: %v", err)
		}
		if got := backend.target(); got != 0 {
			t.Fatalf("closed X11 target = %d", got)
		}
	}
}
