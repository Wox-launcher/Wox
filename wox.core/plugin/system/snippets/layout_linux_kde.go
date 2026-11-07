//go:build linux && cgo

package snippets

import (
	"context"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	kdeKeyboardService = "org.kde.keyboard"
	kdeKeyboardPath    = dbus.ObjectPath("/Layouts")
	kdeKeyboardIface   = "org.kde.KeyboardLayouts"
)

// kdeLayout tracks KWin's current XKB group; callbacks must invalidate matching before applying changes.
type kdeLayout struct {
	conn    *dbus.Conn
	signals chan *dbus.Signal
	refresh chan struct{}
	stop    chan struct{}
	done    chan struct{}
	once    sync.Once
}

// startKDELayout subscribes before reading the current group so changes cannot be silently missed.
func startKDELayout(changed func(uint32, bool)) (*kdeLayout, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	// The connection outlives the startup timeout; individual calls use bounded contexts.
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, errLayoutUnavailable
	}
	if err := conn.AddMatchSignal(dbus.WithMatchInterface(kdeKeyboardIface), dbus.WithMatchObjectPath(kdeKeyboardPath)); err != nil {
		_ = conn.Close()
		return nil, errLayoutUnavailable
	}
	source := &kdeLayout{conn: conn, signals: make(chan *dbus.Signal, 32), refresh: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{})}
	conn.Signal(source.signals)
	group, err := source.current(ctx)
	if err != nil {
		conn.RemoveSignal(source.signals)
		_ = conn.Close()
		return nil, errLayoutUnavailable
	}
	changed(group, true)
	go source.listen(changed)
	return source, nil
}

// current confirms the active group against KWin before a replacement.
func (s *kdeLayout) current(ctx context.Context) (uint32, error) {
	var group uint32
	err := s.conn.Object(kdeKeyboardService, kdeKeyboardPath).CallWithContext(ctx, kdeKeyboardIface+".getLayout", 0).Store(&group)
	if err != nil {
		return 0, errLayoutUnavailable
	}
	return group, nil
}

// requestRefresh is nonblocking because keymap notifications run inside the native input lock.
func (s *kdeLayout) requestRefresh() {
	select {
	case s.refresh <- struct{}{}:
	default:
	}
}

// listen invalidates the cached group when KWin changes or reconfigures layouts.
func (s *kdeLayout) listen(changed func(uint32, bool)) {
	defer close(s.done)
	for {
		select {
		case <-s.stop:
			return
		case signal, ok := <-s.signals:
			if !ok {
				changed(0, false)
				return
			}
			if signal == nil || signal.Path != kdeKeyboardPath {
				continue
			}
			switch signal.Name {
			case kdeKeyboardIface + ".layoutChanged":
				if len(signal.Body) == 1 {
					if group, ok := signal.Body[0].(uint32); ok {
						changed(group, true)
						continue
					}
				}
				changed(0, false)
			case kdeKeyboardIface + ".layoutListChanged":
				changed(0, false)
				s.requestRefresh()
			}
		case <-s.refresh:
			ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			group, err := s.current(ctx)
			cancel()
			changed(group, err == nil)
		}
	}
}

// close stops layout notifications before releasing the private bus connection.
func (s *kdeLayout) close() {
	if s == nil {
		return
	}
	s.once.Do(func() {
		close(s.stop)
		<-s.done
		s.conn.RemoveSignal(s.signals)
		_ = s.conn.Close()
	})
}
