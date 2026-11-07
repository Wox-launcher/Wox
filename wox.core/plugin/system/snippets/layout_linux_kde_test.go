//go:build linux && cgo

package snippets

import (
	"bufio"
	"context"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// TestKDELayoutTracksSignalsAndQueries exercises KWin's D-Bus contract on a private bus.
func TestKDELayoutTracksSignalsAndQueries(t *testing.T) {
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
	if reply, err := server.RequestName(kdeKeyboardService, dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("request keyboard service: %v %v", reply, err)
	}
	var active atomic.Uint32
	if err := server.ExportMethodTable(map[string]any{"getLayout": func() (uint32, *dbus.Error) { return active.Load(), nil }}, kdeKeyboardPath, kdeKeyboardIface); err != nil {
		t.Fatal(err)
	}
	type change struct {
		group uint32
		known bool
	}
	changes := make(chan change, 4)
	source, err := startKDELayout(func(group uint32, known bool) { changes <- change{group, known} })
	if err != nil {
		t.Fatal(err)
	}
	defer source.close()
	expect := func(want change) {
		t.Helper()
		select {
		case got := <-changes:
			if got != want {
				t.Fatalf("layout update = %#v, want %#v", got, want)
			}
		case <-time.After(time.Second):
			t.Fatal("layout update not received")
		}
	}
	expect(change{0, true})
	active.Store(1)
	if err := server.Emit(kdeKeyboardPath, kdeKeyboardIface+".layoutChanged", uint32(1)); err != nil {
		t.Fatal(err)
	}
	expect(change{1, true})
	if group, err := source.current(context.Background()); err != nil || group != 1 {
		t.Fatalf("current layout = %d, %v", group, err)
	}
	if err := server.Emit(kdeKeyboardPath, kdeKeyboardIface+".layoutListChanged"); err != nil {
		t.Fatal(err)
	}
	expect(change{0, false})
	expect(change{1, true})
}
