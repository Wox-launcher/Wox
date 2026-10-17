//go:build windows

package raycast

import (
	"os"
	"strings"
	"testing"
)

func TestAppxPackageIdentity(t *testing.T) {
	name := "Raycast.Raycast_2.8.0.0_x64__qypenmj9wpt2a"
	if !isRaycastAppxPackage(name) || isRaycastAppxPackage("Other.Raycast_1.0.0.0_x64__abc") {
		t.Fatalf("identity %s", name)
	}
	if got := appxPackageVersion(name); got != "2.8.0.0" {
		t.Fatalf("version = %s", got)
	}
	if !appxVersionNewer("2.10.0.0", "2.8.0.0") || appxVersionNewer("2.8.0.0", "2.8.0.0") {
		t.Fatal("version order")
	}
}

func TestStoreRaycastIsDetectedWhenRegistered(t *testing.T) {
	location, version := raycastFromAppx()
	if location == "" {
		t.Skip("no Microsoft Store Raycast package is registered")
	}
	info, err := os.Stat(location)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("location %s: %v", location, err)
	}
	if !strings.Contains(strings.ToLower(location), "raycast.exe") || version == "" {
		t.Fatalf("location=%s version=%s", location, version)
	}
}
