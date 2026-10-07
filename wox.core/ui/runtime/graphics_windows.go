//go:build windows

package woxui

import (
	graphics "wox/ui/runtime/internal/graphics"
)

// PackedBGRA exposes an immutable Windows desktop buffer without swapping every pixel before preview.
type PackedBGRA = graphics.PackedBGRA
