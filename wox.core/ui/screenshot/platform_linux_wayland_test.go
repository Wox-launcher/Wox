//go:build linux

package screenshot

import "testing"

func TestLinuxWaylandBackendPriority(t *testing.T) {
	backends := []linuxWaylandCaptureBackend{
		{name: "inactive", priority: 300, matches: func() bool { return false }},
		{name: "lower", priority: 10, matches: func() bool { return true }},
		{name: "higher", priority: 20, matches: func() bool { return true }},
	}
	selected := linuxMatchingWaylandCaptureBackend(backends)
	if selected == nil || selected.name != "higher" {
		t.Fatalf("selected backend = %#v", selected)
	}
}

func TestLinuxWaylandBackendUsesFallback(t *testing.T) {
	backends := []linuxWaylandCaptureBackend{
		{name: "fallback", priority: 0, matches: func() bool { return true }},
		{name: "desktop", priority: 100, matches: func() bool { return false }},
	}
	selected := linuxMatchingWaylandCaptureBackend(backends)
	if selected == nil || selected.name != "fallback" {
		t.Fatalf("selected backend = %#v", selected)
	}
}
