//go:build darwin

package woxui

import "testing"

func TestDarwinEdgeFadePreservesGlass(t *testing.T) {
	checkEdgeFadePixels(t, testDarwinEdgeFade)
}
