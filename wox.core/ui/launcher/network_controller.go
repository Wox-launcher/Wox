package launcher

import (
	"sync"
)

// networkSettingsSnapshot is the immutable Network tab state consumed by the view layer.
type networkSettingsSnapshot struct {
	ProxyEnabled bool
	ProxyURL     string
}

// networkSettingsController owns a synced mirror of the Network tab state
// (HTTP proxy enable flag and proxy URL). The source of truth remains
// settingsData (general domain) until Task 15 splits settingsData; this
// controller mirrors those two fields so subsequent tasks can switch
// settingItems to read from the snapshot without touching saveSetting.
type networkSettingsController struct {
	deps CommonDeps
	mu   sync.RWMutex

	proxyEnabled bool
	proxyURL     string
}

func newNetworkSettingsController(deps CommonDeps) *networkSettingsController {
	return &networkSettingsController{deps: deps}
}

// ApplyData syncs the controller's mirror of network settings from the loaded
// settingsData. Called by App.reloadSettings after the full settings payload
// is fetched, so the controller stays in sync on initial load and after every
// saveSetting round-trip (which calls reloadSettings).
func (c *networkSettingsController) ApplyData(enabled bool, url string) {
	c.mu.Lock()
	c.proxyEnabled = enabled
	c.proxyURL = url
	c.mu.Unlock()
}

// Set updates one network setting by key. Used by App.saveSetting dispatch in
// Task 15; accepted keys are HttpProxyEnabled (bool) and HttpProxyURL /
// HttpProxyUrl (string). Unknown keys are ignored so callers can blindly
// forward every network-tab key.
func (c *networkSettingsController) Set(key, value string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch key {
	case "HttpProxyEnabled":
		c.proxyEnabled = value == "true"
	case "HttpProxyURL", "HttpProxyUrl":
		c.proxyURL = value
	}
	return nil
}

// Snapshot returns a copy of the network state for the view layer.
func (c *networkSettingsController) Snapshot() networkSettingsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return networkSettingsSnapshot{
		ProxyEnabled: c.proxyEnabled,
		ProxyURL:     c.proxyURL,
	}
}
