package ui

import (
	"bytes"
	"context"
	"testing"

	"wox/common"
)

func TestTrayQueryIconUsesPNGBytes(t *testing.T) {
	m := &Manager{}
	svgIcon := common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24"><rect width="24" height="24" fill="#ff0000"/></svg>`)

	iconBytes := m.toTrayIconBytes(context.Background(), svgIcon)
	pngSignature := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	if !bytes.HasPrefix(iconBytes, pngSignature) {
		t.Fatal("tray icon should be rasterized to PNG before entering the platform tray adapter")
	}
}
