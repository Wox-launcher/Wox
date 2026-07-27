package launcher

import (
	"testing"
)

func TestNetworkControllerApplyData(t *testing.T) {
	deps := CommonDeps{
		Invalidate: func() {},
		Translate:  func(s string) string { return s },
	}
	c := newNetworkSettingsController(deps)
	c.ApplyData(true, "http://proxy:8080")
	snap := c.Snapshot()
	if !snap.ProxyEnabled || snap.ProxyURL != "http://proxy:8080" {
		t.Fatalf("ApplyData did not mirror state: %+v", snap)
	}

	c.ApplyData(false, "")
	snap = c.Snapshot()
	if snap.ProxyEnabled || snap.ProxyURL != "" {
		t.Fatalf("ApplyData(false, empty) did not clear state: %+v", snap)
	}
}

func TestNetworkControllerSetEnabled(t *testing.T) {
	deps := CommonDeps{
		Invalidate: func() {},
		Translate:  func(s string) string { return s },
	}
	c := newNetworkSettingsController(deps)
	if err := c.Set("HttpProxyEnabled", "true"); err != nil {
		t.Fatalf("Set HttpProxyEnabled=true returned error: %v", err)
	}
	if snap := c.Snapshot(); !snap.ProxyEnabled {
		t.Fatalf("expected ProxyEnabled=true after Set true, got %+v", snap)
	}
	if err := c.Set("HttpProxyEnabled", "false"); err != nil {
		t.Fatalf("Set HttpProxyEnabled=false returned error: %v", err)
	}
	if snap := c.Snapshot(); snap.ProxyEnabled {
		t.Fatalf("expected ProxyEnabled=false after Set false, got %+v", snap)
	}
}

func TestNetworkControllerSetURL(t *testing.T) {
	deps := CommonDeps{
		Invalidate: func() {},
		Translate:  func(s string) string { return s },
	}
	c := newNetworkSettingsController(deps)
	if err := c.Set("HttpProxyURL", "http://new:8080"); err != nil {
		t.Fatalf("Set HttpProxyURL returned error: %v", err)
	}
	if snap := c.Snapshot(); snap.ProxyURL != "http://new:8080" {
		t.Fatalf("expected ProxyURL=http://new:8080, got %+v", snap)
	}
	// HttpProxyUrl is the JSON key used by settingsData.HttpProxyURL; Set accepts both.
	if err := c.Set("HttpProxyUrl", "http://alias:9090"); err != nil {
		t.Fatalf("Set HttpProxyUrl returned error: %v", err)
	}
	if snap := c.Snapshot(); snap.ProxyURL != "http://alias:9090" {
		t.Fatalf("expected ProxyURL=http://alias:9090 via HttpProxyUrl alias, got %+v", snap)
	}
}
