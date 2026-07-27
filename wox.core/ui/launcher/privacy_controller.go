package launcher

import (
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// privacySettingsSnapshot is the immutable Privacy tab state consumed by the view layer.
type privacySettingsSnapshot struct {
	Sample string
	Error  string
}

// privacySettingsController owns the Privacy tab state (sample payload and error).
// Unlike most tabs, privacy has no network reload: the sample is generated locally
// from a telemetry payload struct and copied to the clipboard on demand.
type privacySettingsController struct {
	deps   CommonDeps
	mu     sync.RWMutex
	sample string
	errMsg string
}

func newPrivacySettingsController(deps CommonDeps) *privacySettingsController {
	return &privacySettingsController{deps: deps}
}

// SampleVisible reports whether the privacy sample overlay is currently shown.
func (c *privacySettingsController) SampleVisible() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sample != ""
}

// ToggleSample either clears the visible sample or generates a fresh local payload.
// getVersion supplies the Wox version for the payload; it is injected so the
// controller does not depend on aboutSettingsController directly.
func (c *privacySettingsController) ToggleSample(getVersion func() string) {
	c.mu.Lock()
	if c.sample != "" {
		c.sample = ""
		c.errMsg = ""
		c.mu.Unlock()
		c.deps.Invalidate()
		return
	}
	version := getVersion()
	if version == "" {
		version = "current Wox version"
	}
	payload := privacySamplePayload{
		SchemaVersion: 1,
		InstallHash:   "sha256(install_id) - a 64-character hexadecimal string",
		OSFamily:      runtime.GOOS,
		WoxVersion:    version,
		SentAt:        time.Now().UnixMilli(),
	}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		c.errMsg = err.Error()
	} else {
		c.sample = string(encoded)
		c.errMsg = ""
	}
	c.mu.Unlock()
	c.deps.Invalidate()
}

// CopySample publishes the visible sample through the portable clipboard boundary.
// writeClipboard is injected so the controller does not depend on the native window.
func (c *privacySettingsController) CopySample(writeClipboard func(string) error) {
	c.mu.RLock()
	value := c.sample
	c.mu.RUnlock()
	if value == "" {
		return
	}
	err := writeClipboard(value)
	c.mu.Lock()
	if err != nil {
		c.errMsg = fmt.Sprintf("Could not copy sample: %v", err)
	} else {
		c.errMsg = ""
	}
	c.mu.Unlock()
	c.deps.Invalidate()
}

// SetError records an error from outside the toggle/copy paths.
func (c *privacySettingsController) SetError(msg string) {
	c.mu.Lock()
	c.errMsg = msg
	c.mu.Unlock()
	c.deps.Invalidate()
}

// Snapshot returns a copy of the Privacy state for the view layer.
func (c *privacySettingsController) Snapshot() privacySettingsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return privacySettingsSnapshot{Sample: c.sample, Error: c.errMsg}
}
