//go:build darwin

package window

import (
	"testing"
)

func TestDarwinOpacityGroupPixels(t *testing.T) {
	checkOpacityPixels(t, testDarwinOpacity)
}
