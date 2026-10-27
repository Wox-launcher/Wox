//go:build linux

package app

import (
	"context"
	"testing"
	"time"
)

func TestWaitForLinuxLauncherHideSkipsWhenHidden(t *testing.T) {
	start := time.Now()
	waitForLinuxLauncherHide(context.Background(), func() bool { return false })
	if time.Since(start) > 30*time.Millisecond {
		t.Fatalf("waited %s for an already hidden launcher", time.Since(start))
	}
}
