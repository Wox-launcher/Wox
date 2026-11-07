//go:build linux && cgo

package snippets

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/godbus/dbus/v5"
)

type atspiRole uint32
type atspiState uint

const (
	atspiRegistry                 = "org.a11y.atspi.Registry"
	atspiFocusEvent               = "object:state-changed:focused"
	atspiRolePassword  atspiRole  = 40
	atspiRoleTerminal  atspiRole  = 60
	atspiStateDefunct  atspiState = 6
	atspiStateEditable atspiState = 7
	atspiStateFocused  atspiState = 12
)

type accessibleTarget struct {
	bus      string
	path     dbus.ObjectPath
	token    uint64
	terminal bool
}

// accessibilityFocus tracks AT-SPI focus changes without retaining application text.
// No focus is trusted until the application reports an editable, non-password object.
type accessibilityFocus struct {
	conn     *dbus.Conn
	mu       sync.Mutex
	current  accessibleTarget
	sequence uint64
	token    atomic.Uint64
	signals  chan *dbus.Signal
	cancel   context.CancelFunc
	done     chan struct{}
	once     sync.Once
}

// startAccessibilityFocus listens for focused-object changes on the accessibility bus.
func startAccessibilityFocus(onChange func()) (*accessibilityFocus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	session, err := dbus.ConnectSessionBus(dbus.WithContext(ctx))
	if err != nil {
		return nil, errFocusUnavailable
	}
	var address string
	err = session.Object("org.a11y.Bus", "/org/a11y/bus").CallWithContext(ctx, "org.a11y.Bus.GetAddress", 0).Store(&address)
	_ = session.Close()
	if err != nil || address == "" {
		return nil, errFocusUnavailable
	}
	conn, err := dbus.Connect(address)
	if err != nil {
		return nil, errFocusUnavailable
	}
	if err := conn.AddMatchSignal(dbus.WithMatchInterface("org.a11y.atspi.Event.Object"), dbus.WithMatchMember("StateChanged")); err != nil {
		_ = conn.Close()
		return nil, errFocusUnavailable
	}
	registry := conn.Object(atspiRegistry, "/org/a11y/atspi/registry")
	if call := registry.CallWithContext(ctx, "org.a11y.atspi.Registry.RegisterEvent", 0, atspiFocusEvent, []string{}, ""); call.Err != nil {
		_ = conn.Close()
		return nil, errFocusUnavailable
	}
	loopCtx, loopCancel := context.WithCancel(context.Background())
	focus := &accessibilityFocus{conn: conn, signals: make(chan *dbus.Signal, 64), cancel: loopCancel, done: make(chan struct{})}
	conn.Signal(focus.signals)
	go focus.listen(loopCtx, onChange)
	return focus, nil
}

// listen clears the previous target before making any potentially slow accessibility requests.
func (f *accessibilityFocus) listen(ctx context.Context, onChange func()) {
	defer close(f.done)
	for {
		select {
		case <-ctx.Done():
			return
		case signal, ok := <-f.signals:
			if !ok {
				return
			}
			if signal == nil || len(signal.Body) < 2 {
				continue
			}
			state, stateOK := signal.Body[0].(string)
			enabled, enabledOK := signal.Body[1].(int32)
			if !stateOK || !enabledOK || state != "focused" {
				continue
			}
			f.mu.Lock()
			f.sequence++
			sequence := f.sequence
			f.current = accessibleTarget{}
			f.token.Store(0)
			f.mu.Unlock()
			onChange()
			if enabled == 0 || signal.Sender == "" || !signal.Path.IsValid() {
				continue
			}
			candidate := accessibleTarget{bus: signal.Sender, path: signal.Path, token: sequence}
			go f.confirmCandidate(ctx, candidate)
		}
	}
}

// confirmCandidate accepts only a still-focused editable control and discards stale replies.
func (f *accessibilityFocus) confirmCandidate(parent context.Context, candidate accessibleTarget) {
	ctx, cancel := context.WithTimeout(parent, 250*time.Millisecond)
	defer cancel()
	role, ok := f.check(ctx, candidate)
	if !ok {
		return
	}
	candidate.terminal = role == atspiRoleTerminal
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sequence == candidate.token && parent.Err() == nil {
		f.current = candidate
		f.token.Store(candidate.token)
	}
}

// check re-reads focus before a replacement so delayed accessibility events fail closed.
func (f *accessibilityFocus) check(ctx context.Context, candidate accessibleTarget) (atspiRole, bool) {
	object := f.conn.Object(candidate.bus, candidate.path)
	var role uint32
	if object.CallWithContext(ctx, "org.a11y.atspi.Accessible.GetRole", 0).Store(&role) != nil || atspiRole(role) == atspiRolePassword {
		return 0, false
	}
	var states []uint32
	if object.CallWithContext(ctx, "org.a11y.atspi.Accessible.GetState", 0).Store(&states) != nil || len(states) != 2 {
		return 0, false
	}
	if !accessibleTargetAllowed(atspiRole(role), states) {
		return 0, false
	}
	return atspiRole(role), true
}

// accessibleTargetAllowed rejects passwords, defunct focus and non-editable controls.
func accessibleTargetAllowed(role atspiRole, states []uint32) bool {
	return role != atspiRolePassword && len(states) == 2 &&
		states[0]&(1<<atspiStateFocused) != 0 && states[0]&(1<<atspiStateDefunct) == 0 &&
		(role == atspiRoleTerminal || states[0]&(1<<atspiStateEditable) != 0)
}

func (f *accessibilityFocus) fast() uint64 { return f.token.Load() }

// verifiedTarget checks the accessible object and its generation outside the keyboard callback.
func (f *accessibilityFocus) verifiedTarget() uint64 {
	f.mu.Lock()
	candidate := f.current
	f.mu.Unlock()
	if candidate.token == 0 || candidate.token != f.fast() {
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_, ok := f.check(ctx, candidate)
	if !ok || candidate.token != f.fast() {
		return 0
	}
	return candidate.token
}

func (f *accessibilityFocus) isTerminal(target uint64) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.current.token == target && f.current.terminal
}

func (f *accessibilityFocus) close() {
	if f == nil {
		return
	}
	f.once.Do(func() {
		f.token.Store(0)
		f.cancel()
		f.conn.RemoveSignal(f.signals)
		_ = f.conn.Close()
		<-f.done
	})
}
