package launcher

import (
	"errors"
	"strings"
	"testing"
)

func TestPrivacyControllerToggleSampleGeneratesPayload(t *testing.T) {
	invalidateCalled := 0
	deps := CommonDeps{
		Invalidate: func() { invalidateCalled++ },
		Translate:  func(s string) string { return s },
	}
	c := newPrivacySettingsController(deps)
	c.ToggleSample(func() string { return "v1.0" })
	snap := c.Snapshot()
	if snap.Sample == "" {
		t.Fatalf("sample should be generated, got empty")
	}
	if !strings.Contains(snap.Sample, "wox_version") {
		t.Fatalf("sample should contain wox_version field, got: %s", snap.Sample)
	}
	if !strings.Contains(snap.Sample, "v1.0") {
		t.Fatalf("sample should contain version v1.0, got: %s", snap.Sample)
	}
	if snap.Error != "" {
		t.Fatalf("error should be empty on success, got: %s", snap.Error)
	}
	if invalidateCalled < 1 {
		t.Fatalf("Invalidate should be called, got %d", invalidateCalled)
	}
}

func TestPrivacyControllerToggleSampleClears(t *testing.T) {
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }}
	c := newPrivacySettingsController(deps)
	// First toggle generates the sample.
	c.ToggleSample(func() string { return "v1.0" })
	if c.Snapshot().Sample == "" {
		t.Fatalf("first toggle should generate sample")
	}
	// Second toggle clears it.
	c.ToggleSample(func() string { return "v1.0" })
	snap := c.Snapshot()
	if snap.Sample != "" {
		t.Fatalf("second toggle should clear sample, got: %s", snap.Sample)
	}
	if snap.Error != "" {
		t.Fatalf("second toggle should clear error, got: %s", snap.Error)
	}
}

func TestPrivacyControllerCopySampleSetsErrorOnFailure(t *testing.T) {
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }}
	c := newPrivacySettingsController(deps)
	c.ToggleSample(func() string { return "v1.0" })
	// Copy fails: error should be recorded with the expected prefix.
	c.CopySample(func(string) error { return errors.New("clipboard locked") })
	snap := c.Snapshot()
	if !strings.Contains(snap.Error, "Could not copy") {
		t.Fatalf("error should contain 'Could not copy', got: %s", snap.Error)
	}
}

func TestPrivacyControllerCopySampleClearsErrorOnSuccess(t *testing.T) {
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }}
	c := newPrivacySettingsController(deps)
	c.ToggleSample(func() string { return "v1.0" })
	// Copy succeeds: error should stay empty.
	c.CopySample(func(string) error { return nil })
	snap := c.Snapshot()
	if snap.Error != "" {
		t.Fatalf("error should be empty on successful copy, got: %s", snap.Error)
	}
	if snap.Sample == "" {
		t.Fatalf("sample should remain visible after copy")
	}
}

func TestPrivacyControllerSampleVisible(t *testing.T) {
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }}
	c := newPrivacySettingsController(deps)
	if c.SampleVisible() {
		t.Fatalf("sample should not be visible initially")
	}
	c.ToggleSample(func() string { return "v1.0" })
	if !c.SampleVisible() {
		t.Fatalf("sample should be visible after toggle")
	}
	c.ToggleSample(func() string { return "v1.0" })
	if c.SampleVisible() {
		t.Fatalf("sample should not be visible after clearing toggle")
	}
}

func TestPrivacyControllerSetError(t *testing.T) {
	invalidateCalled := 0
	deps := CommonDeps{
		Invalidate: func() { invalidateCalled++ },
		Translate:  func(s string) string { return s },
	}
	c := newPrivacySettingsController(deps)
	c.SetError("something went wrong")
	snap := c.Snapshot()
	if snap.Error != "something went wrong" {
		t.Fatalf("error should be recorded, got: %s", snap.Error)
	}
	if invalidateCalled < 1 {
		t.Fatalf("Invalidate should be called, got %d", invalidateCalled)
	}
}
