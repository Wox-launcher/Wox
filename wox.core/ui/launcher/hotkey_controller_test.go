package launcher

import (
	"context"
	"errors"
	"testing"

	"wox/ui/contract"
)

type hotkeyFakeService struct {
	apps    []contract.HotkeyApp
	err     error
	started chan struct{}
	release chan struct{}
}

func (f *hotkeyFakeService) HotkeyAppCandidates(_ context.Context, _ string) ([]contract.HotkeyApp, error) {
	if f.started != nil {
		f.started <- struct{}{}
	}
	if f.release != nil {
		<-f.release
	}
	return append([]contract.HotkeyApp(nil), f.apps...), f.err
}

func newHotkeyDeps() (CommonDeps, *int) {
	calls := 0
	deps := CommonDeps{
		Invalidate: func() { calls++ },
		Translate:  func(s string) string { return s },
	}
	return deps, &calls
}

func TestHotkeyControllerForm(t *testing.T) {
	deps, _ := newHotkeyDeps()
	c := newHotkeySettingsController(deps)
	if c.Form() != nil {
		t.Fatalf("Form should be nil initially")
	}
	form := newFormFieldsState(
		[]formDefinition{{Type: "hotkey", Value: formDefinitionValue{Key: "MainHotkey", Label: "Main"}}},
		map[string]string{"MainHotkey": "alt+space"}, true,
	)
	c.SetForm(&form)
	got := c.Form()
	if got == nil || got.values["MainHotkey"] != "alt+space" {
		t.Fatalf("Form() should return the installed form, got %+v", got)
	}
	c.SetForm(nil)
	if c.Form() != nil {
		t.Fatalf("Form should be nil after SetForm(nil)")
	}
}

func TestHotkeyControllerRecording(t *testing.T) {
	deps, _ := newHotkeyDeps()
	c := newHotkeySettingsController(deps)
	if c.Recording() != nil {
		t.Fatalf("Recording should be nil initially")
	}
	state := &hotkeyRecordingState{idPrefix: "hotkey-settings", status: "Press a hotkey…"}
	c.SetRecording(state)
	if c.Recording() != state {
		t.Fatalf("Recording() should return the installed state")
	}
	c.ClearRecording()
	if c.Recording() != nil {
		t.Fatalf("Recording should be nil after ClearRecording")
	}
}

func TestHotkeyControllerFocused(t *testing.T) {
	deps, _ := newHotkeyDeps()
	c := newHotkeySettingsController(deps)
	if c.Focused() {
		t.Fatalf("Focused should be false initially")
	}
	c.SetFocused(true)
	if !c.Focused() {
		t.Fatalf("Focused should be true after SetFocused(true)")
	}
	c.SetFocused(false)
	if c.Focused() {
		t.Fatalf("Focused should be false after SetFocused(false)")
	}
}

func TestHotkeyControllerAppCandidates(t *testing.T) {
	deps, _ := newHotkeyDeps()
	c := newHotkeySettingsController(deps)
	candidates := c.AppCandidates()
	if len(candidates) != 0 {
		t.Fatalf("AppCandidates should be empty initially, got %d", len(candidates))
	}
	c.SetAppCandidates([]ignoredHotkeyApp{{Name: "Finder", Identity: "com.apple.finder"}})
	got := c.AppCandidates()
	if len(got) != 1 || got[0].Name != "Finder" {
		t.Fatalf("AppCandidates should return 1 entry, got %+v", got)
	}
	// Mutating the returned slice must not affect the controller state.
	got[0].Name = "Mutated"
	if c.AppCandidates()[0].Name != "Finder" {
		t.Fatalf("AppCandidates should return a copy, but mutating it affected the controller")
	}
}

func TestHotkeyControllerReloadAppCandidatesSuccess(t *testing.T) {
	deps, invalidateCalls := newHotkeyDeps()
	c := newHotkeySettingsController(deps)
	service := &hotkeyFakeService{apps: []contract.HotkeyApp{
		{Name: "Finder", Identity: "com.apple.finder"},
		{Name: "Safari", Identity: "com.apple.Safari"},
	}}
	c.ReloadAppCandidates(context.Background(), service, "session")
	snap := c.Snapshot()
	if !snap.AppsLoaded || snap.AppsError != "" || len(snap.AppCandidates) != 2 {
		t.Fatalf("after reload: %+v", snap)
	}
	if snap.AppsLoading {
		t.Fatalf("AppsLoading should be false after reload completes")
	}
	if *invalidateCalls < 2 {
		t.Fatalf("Invalidate should be called at least twice, got %d", *invalidateCalls)
	}
}

func TestHotkeyControllerReloadAppCandidatesError(t *testing.T) {
	deps, _ := newHotkeyDeps()
	c := newHotkeySettingsController(deps)
	service := &hotkeyFakeService{err: errors.New("network down")}
	c.ReloadAppCandidates(context.Background(), service, "session")
	snap := c.Snapshot()
	if snap.AppsLoaded || snap.AppsError == "" {
		t.Fatalf("error should be recorded: %+v", snap)
	}
}

func TestHotkeyControllerReleaseWindowMemoryInvalidatesReload(t *testing.T) {
	deps, _ := newHotkeyDeps()
	c := newHotkeySettingsController(deps)
	form := newFormFieldsState(nil, nil, true)
	c.SetForm(&form)
	service := &hotkeyFakeService{
		apps:    []contract.HotkeyApp{{Name: "Finder", Identity: "com.apple.finder"}},
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	done := make(chan struct{})
	go func() {
		c.ReloadAppCandidates(context.Background(), service, "session")
		close(done)
	}()
	<-service.started
	c.ReleaseWindowMemory()
	close(service.release)
	<-done
	if c.Form() != nil {
		t.Fatal("settings form should be released")
	}
	snapshot := c.Snapshot()
	if snapshot.AppsLoaded || snapshot.AppsLoading || len(snapshot.AppCandidates) != 0 {
		t.Fatalf("released app candidates were repopulated: %+v", snapshot)
	}
}
