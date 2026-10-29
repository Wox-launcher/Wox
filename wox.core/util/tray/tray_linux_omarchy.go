//go:build linux

package tray

import (
	"embed"
	"os"
	"path/filepath"

	"wox/internal/assetfs"
	"wox/util"
	"wox/util/mouse"
)

//go:embed wox-tray-symbolic.svg
var rawOmarchyTrayIcon embed.FS

var omarchyTrayIcon = assetfs.New(rawOmarchyTrayIcon)

// isOmarchyTraySession avoids changing the icon in other sessions on an Omarchy installation.
func isOmarchyTraySession() bool {
	return SupportsQueryIcons()
}

// prepareOmarchyTrayIcon exposes a named symbolic icon for the Omarchy shell to tint.
func prepareOmarchyTrayIcon(appIcon []byte) ([]sniIconPixmap, string, string, string) {
	icon, err := omarchyTrayIcon.ReadFile("wox-tray-symbolic.svg")
	if err != nil {
		return prepareTrayIcon(appIcon)
	}
	iconDir := filepath.Join(util.GetLocation().GetCacheDirectory(), "tray-icons")
	if err := os.MkdirAll(iconDir, 0o700); err != nil {
		return prepareTrayIcon(appIcon)
	}
	iconFile := filepath.Join(iconDir, "wox-tray-symbolic.svg")
	if err := os.WriteFile(iconFile, icon, 0o600); err != nil {
		return prepareTrayIcon(appIcon)
	}
	// An empty pixmap makes the host resolve the symbolic name and retain its suffix.
	return []sniIconPixmap{}, "wox-tray-symbolic", iconDir, iconFile
}

// startOmarchyQueryTray gives each query its own StatusNotifierItem connection.
func startOmarchyQueryTray(item QueryIconItem) (*linuxTray, error) {
	return startLinuxTrayItem(item.Icon, "wox-query-"+item.Identifier, item.Tooltip, nil, func(x, y int32) {
		// The tray host may send zero coordinates; Hyprland reports logical desktop coordinates.
		rect := ClickRect{}
		if point, ok := mouse.CurrentPosition(); ok {
			rect = ClickRect{X: int(point.X), Y: int(point.Y), Width: 1, Height: 1}
		}
		if item.Callback != nil {
			item.Callback(rect)
		}
	}, []MenuItem{{Title: item.ContextMenuTitle, Callback: item.ContextMenuCallback}}, true)
}
