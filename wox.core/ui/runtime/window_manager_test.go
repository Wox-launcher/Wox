package woxui

import (
	"testing"
	"time"
)

// TestManagedWindowCloseReentrant models a hover callback during native teardown.
func TestManagedWindowCloseReentrant(t *testing.T) {
	manager := NewWindowManager()
	managed := &ManagedWindow{manager: manager, id: "tooltip", closed: make(chan struct{})}
	manager.windows[managed.id] = managed
	closeCalls := 0
	managed.window = &Window{closeFn: func() error {
		closeCalls++
		if err := managed.Close(); err != nil {
			return err
		}
		managed.handleClosed()
		managed.signalClosed()
		return nil
	}}
	done := make(chan error, 1)
	go func() { done <- managed.Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("reentrant Close blocked native teardown")
	}
	if closeCalls != 1 || managed.Lifecycle() != WindowLifecycleClosed {
		t.Fatalf("close calls = %d, lifecycle = %d", closeCalls, managed.Lifecycle())
	}
	if _, exists := manager.Get(managed.id); exists {
		t.Fatal("closed window remains registered")
	}
}
