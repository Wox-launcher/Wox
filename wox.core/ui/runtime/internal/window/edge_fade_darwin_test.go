//go:build darwin

package window

import "testing"

func TestDarwinEdgeFadePreservesGlass(t *testing.T) {
	checkEdgeFadePixels(t, testDarwinEdgeFade)
}
