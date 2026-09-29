package setting

import (
	"testing"
	"time"

	"wox/cloudsync"
)

type syncLockProbeStore struct {
	entered chan struct{}
}

func (s *syncLockProbeStore) Get(string, interface{}) error { return nil }
func (s *syncLockProbeStore) Set(string, interface{}) error { return nil }
func (s *syncLockProbeStore) Delete(string) error           { return nil }
func (s *syncLockProbeStore) DeleteWithSync(string, bool) error {
	return nil
}
func (s *syncLockProbeStore) SetWithSync(string, interface{}, bool) error {
	close(s.entered)
	return nil
}

func TestSettingSetTakesSyncLockBeforeValueMutex(t *testing.T) {
	release := make(chan struct{})
	holding := make(chan struct{})
	go func() {
		_ = cloudsync.WithLocalSyncMutation(func() error {
			close(holding)
			<-release
			return nil
		})
	}()
	<-holding

	store := &syncLockProbeStore{entered: make(chan struct{})}
	value := &SettingValue[string]{key: "LockOrder", settingStore: store, syncable: true}
	setDone := make(chan error, 1)
	go func() {
		setDone <- value.Set("local")
	}()

	select {
	case <-store.entered:
		close(release)
		t.Fatal("Set entered the store while another goroutine held the sync lock")
	case <-time.After(50 * time.Millisecond):
	}
	if !value.mu.TryLock() {
		close(release)
		t.Fatal("Set holds the value mutex while waiting for the sync lock")
	}
	value.mu.Unlock()

	close(release)
	select {
	case err := <-setDone:
		if err != nil {
			t.Fatalf("Set: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Set did not finish after the sync lock was released")
	}
}
