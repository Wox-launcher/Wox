package woxui

import (
	"errors"
	"strings"
	"sync"
)

// WindowID identifies one logical top-level surface across its native lifetime.
type WindowID string

// WindowLifecycle describes the state owned by one managed window instance.
type WindowLifecycle uint8

const (
	WindowLifecycleCreated WindowLifecycle = iota
	WindowLifecyclePresenting
	WindowLifecycleVisible
	WindowLifecycleHidden
	WindowLifecycleClosing
	WindowLifecycleClosed
)

// WindowLifecycleEvent reports one state transition from the managed registry.
type WindowLifecycleEvent struct {
	ID       WindowID
	Previous WindowLifecycle
	Current  WindowLifecycle
}

// WindowMessage carries application state changes between independently hosted windows.
type WindowMessage struct {
	Source  WindowID
	Target  WindowID
	Topic   string
	Payload any
}

type windowMessageSubscription struct {
	target  WindowID
	topic   string
	handler func(WindowMessage)
}

// WindowManager owns named native windows, lifecycle notifications, and in-process messages.
type WindowManager struct {
	mu sync.RWMutex

	windows              map[WindowID]*ManagedWindow
	nextSubscriptionID   uint64
	lifecycleSubscribers map[uint64]func(WindowLifecycleEvent)
	messageSubscribers   map[uint64]windowMessageSubscription
}

// ManagedWindow wraps one named native window and serializes its lifecycle transitions.
type ManagedWindow struct {
	manager *WindowManager
	id      WindowID
	window  *Window

	mu        sync.RWMutex
	lifecycle WindowLifecycle
	closed    chan struct{}
	closeOnce sync.Once
}

// NewWindowManager creates an empty process-local top-level window registry.
func NewWindowManager() *WindowManager {
	return &WindowManager{
		windows:              map[WindowID]*ManagedWindow{},
		lifecycleSubscribers: map[uint64]func(WindowLifecycleEvent){},
		messageSubscribers:   map[uint64]windowMessageSubscription{},
	}
}

// Open creates one hidden named window or returns the existing live instance.
// Native creation still follows Open's requirement to run on the UI thread.
func (m *WindowManager) Open(id WindowID, options WindowOptions) (*ManagedWindow, bool, error) {
	if m == nil {
		return nil, false, errors.New("window manager is not initialized")
	}
	if strings.TrimSpace(string(id)) == "" {
		return nil, false, errors.New("window id is required")
	}

	m.mu.Lock()
	if existing := m.windows[id]; existing != nil {
		switch existing.Lifecycle() {
		case WindowLifecycleClosed:
		case WindowLifecycleClosing:
			m.mu.Unlock()
			return nil, false, errors.New("window is still closing")
		default:
			m.mu.Unlock()
			return existing, false, nil
		}
	}

	managed := &ManagedWindow{manager: m, id: id, lifecycle: WindowLifecycleCreated, closed: make(chan struct{})}
	originalOnClosed := options.OnClosed
	options.OnClosed = func() {
		defer managed.signalClosed()
		defer managed.handleClosed()
		if originalOnClosed != nil {
			originalOnClosed()
		}
	}
	window, err := Open(options)
	if err != nil {
		m.mu.Unlock()
		return nil, false, err
	}
	managed.window = window
	m.windows[id] = managed
	m.mu.Unlock()
	m.emitLifecycle(WindowLifecycleEvent{ID: id, Previous: WindowLifecycleClosed, Current: WindowLifecycleCreated})
	return managed, true, nil
}

// Get returns a live named window without changing its lifecycle.
func (m *WindowManager) Get(id WindowID) (*ManagedWindow, bool) {
	if m == nil {
		return nil, false
	}
	m.mu.RLock()
	window := m.windows[id]
	m.mu.RUnlock()
	return window, window != nil && window.Lifecycle() != WindowLifecycleClosed
}

// CloseAll releases every currently registered native window.
func (m *WindowManager) CloseAll() error {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	windows := make([]*ManagedWindow, 0, len(m.windows))
	for _, window := range m.windows {
		windows = append(windows, window)
	}
	m.mu.RUnlock()

	var closeErr error
	for _, window := range windows {
		if err := window.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	}
	return closeErr
}

// SubscribeLifecycle observes future managed-window state changes.
func (m *WindowManager) SubscribeLifecycle(handler func(WindowLifecycleEvent)) func() {
	if m == nil || handler == nil {
		return func() {}
	}
	m.mu.Lock()
	m.nextSubscriptionID++
	id := m.nextSubscriptionID
	m.lifecycleSubscribers[id] = handler
	m.mu.Unlock()
	return func() {
		m.mu.Lock()
		delete(m.lifecycleSubscribers, id)
		m.mu.Unlock()
	}
}

// SubscribeMessages registers a topic listener. An empty target receives every matching message.
func (m *WindowManager) SubscribeMessages(target WindowID, topic string, handler func(WindowMessage)) func() {
	if m == nil || strings.TrimSpace(topic) == "" || handler == nil {
		return func() {}
	}
	m.mu.Lock()
	m.nextSubscriptionID++
	id := m.nextSubscriptionID
	m.messageSubscribers[id] = windowMessageSubscription{target: target, topic: topic, handler: handler}
	m.mu.Unlock()
	return func() {
		m.mu.Lock()
		delete(m.messageSubscribers, id)
		m.mu.Unlock()
	}
}

// Publish delivers one immutable message snapshot outside the registry lock.
func (m *WindowManager) Publish(message WindowMessage) error {
	if m == nil {
		return errors.New("window manager is not initialized")
	}
	if strings.TrimSpace(message.Topic) == "" {
		return errors.New("window message topic is required")
	}
	m.mu.RLock()
	handlers := make([]func(WindowMessage), 0, len(m.messageSubscribers))
	for _, subscription := range m.messageSubscribers {
		if subscription.topic != message.Topic {
			continue
		}
		if subscription.target != "" && message.Target != "" && subscription.target != message.Target {
			continue
		}
		handlers = append(handlers, subscription.handler)
	}
	m.mu.RUnlock()
	for _, handler := range handlers {
		handler(message)
	}
	return nil
}

func (m *WindowManager) emitLifecycle(event WindowLifecycleEvent) {
	m.mu.RLock()
	handlers := make([]func(WindowLifecycleEvent), 0, len(m.lifecycleSubscribers))
	for _, handler := range m.lifecycleSubscribers {
		handlers = append(handlers, handler)
	}
	m.mu.RUnlock()
	for _, handler := range handlers {
		handler(event)
	}
}

func (m *WindowManager) remove(id WindowID, window *ManagedWindow) {
	m.mu.Lock()
	if m.windows[id] == window {
		delete(m.windows, id)
	}
	m.mu.Unlock()
}

// ID returns the stable logical identifier for this native lifetime.
func (w *ManagedWindow) ID() WindowID {
	if w == nil {
		return ""
	}
	return w.id
}

// Window exposes the platform-neutral native window operations to its owner.
func (w *ManagedWindow) Window() *Window {
	if w == nil {
		return nil
	}
	return w.window
}

// Lifecycle returns the latest serialized state for this window instance.
func (w *ManagedWindow) Lifecycle() WindowLifecycle {
	if w == nil {
		return WindowLifecycleClosed
	}
	w.mu.RLock()
	lifecycle := w.lifecycle
	w.mu.RUnlock()
	return lifecycle
}

// Show presents and focuses this window while retaining the same native instance.
func (w *ManagedWindow) Show() (FocusEpoch, error) {
	if w == nil || w.window == nil {
		return 0, errors.New("managed window is not initialized")
	}
	previous, changed, err := w.beginTransition(WindowLifecyclePresenting)
	if err != nil {
		return 0, err
	}
	if changed {
		w.manager.emitLifecycle(WindowLifecycleEvent{ID: w.id, Previous: previous, Current: WindowLifecyclePresenting})
	}
	epoch, showErr := w.window.Show()
	if showErr != nil {
		w.finishTransition(previous)
		return 0, showErr
	}
	w.finishTransition(WindowLifecycleVisible)
	return epoch, nil
}

// Hide ends this window's current presentation without releasing native resources.
func (w *ManagedWindow) Hide() error {
	if w == nil || w.window == nil {
		return errors.New("managed window is not initialized")
	}
	previous := w.Lifecycle()
	if previous == WindowLifecycleHidden || previous == WindowLifecycleCreated {
		return nil
	}
	if previous == WindowLifecycleClosing || previous == WindowLifecycleClosed {
		return errors.New("managed window is closing")
	}
	if err := w.window.Hide(); err != nil {
		return err
	}
	w.finishTransition(WindowLifecycleHidden)
	return nil
}

// Close permanently releases this native instance and removes it from the registry.
func (w *ManagedWindow) Close() error {
	if w == nil || w.window == nil {
		return nil
	}
	current := w.Lifecycle()
	if current == WindowLifecycleClosing {
		<-w.closed
		return nil
	}
	if current == WindowLifecycleClosed {
		return nil
	}
	previous, changed, err := w.beginTransition(WindowLifecycleClosing)
	if err != nil {
		if w.Lifecycle() == WindowLifecycleClosing {
			<-w.closed
			return nil
		}
		return err
	}
	if !changed {
		return nil
	}
	w.manager.emitLifecycle(WindowLifecycleEvent{ID: w.id, Previous: previous, Current: WindowLifecycleClosing})
	if err := w.window.Close(); err != nil {
		w.finishTransition(previous)
		return err
	}
	<-w.closed
	return nil
}

func (w *ManagedWindow) beginTransition(next WindowLifecycle) (WindowLifecycle, bool, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	previous := w.lifecycle
	if previous == WindowLifecycleClosed || previous == WindowLifecycleClosing {
		return previous, false, errors.New("managed window is closed")
	}
	if previous == next {
		return previous, false, nil
	}
	w.lifecycle = next
	return previous, true, nil
}

func (w *ManagedWindow) finishTransition(next WindowLifecycle) {
	w.mu.Lock()
	previous := w.lifecycle
	if previous == WindowLifecycleClosed {
		w.mu.Unlock()
		return
	}
	w.lifecycle = next
	w.mu.Unlock()
	if previous != next {
		w.manager.emitLifecycle(WindowLifecycleEvent{ID: w.id, Previous: previous, Current: next})
	}
}

func (w *ManagedWindow) handleClosed() {
	w.mu.Lock()
	previous := w.lifecycle
	if previous == WindowLifecycleClosed {
		w.mu.Unlock()
		return
	}
	w.lifecycle = WindowLifecycleClosed
	w.mu.Unlock()
	w.manager.remove(w.id, w)
	w.manager.emitLifecycle(WindowLifecycleEvent{ID: w.id, Previous: previous, Current: WindowLifecycleClosed})

}

func (w *ManagedWindow) signalClosed() {
	w.closeOnce.Do(func() { close(w.closed) })
}
