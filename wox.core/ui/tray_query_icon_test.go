package ui

import (
	"bytes"
	"context"
	"runtime"
	"testing"

	"wox/common"
)

func TestTrayQueryIconUsesPNGBytes(t *testing.T) {
	m := &Manager{}
	svgIcon := common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24"><rect width="24" height="24" fill="#ff0000"/></svg>`)

	iconBytes := m.toTrayIconBytes(context.Background(), svgIcon)
	if runtime.GOOS == "windows" {
		const icoHeaderSize = 22
		if len(iconBytes) < icoHeaderSize || !bytes.HasPrefix(iconBytes, []byte{0, 0, 1, 0, 1, 0}) {
			t.Fatal("Windows tray icon should contain one ICO image")
		}
		iconBytes = iconBytes[icoHeaderSize:]
	}
	pngSignature := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	if !bytes.HasPrefix(iconBytes, pngSignature) {
		t.Fatal("tray icon should be rasterized to PNG before entering the platform tray adapter")
	}
}
