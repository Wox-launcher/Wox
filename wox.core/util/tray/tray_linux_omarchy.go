package tray

import (
	"embed"
	"os"
	"path/filepath"
	"strings"

	"wox/internal/assetfs"
	"wox/util"
)

//go:embed wox-tray-symbolic.svg
var rawOmarchyTrayIcon embed.FS

var omarchyTrayIcon = assetfs.New(rawOmarchyTrayIcon)

// isOmarchyTraySession avoids changing the icon in other sessions on an Omarchy installation.
func isOmarchyTraySession() bool {
	return util.IsOmarchy() && util.IsHyprlandSession() && strings.EqualFold(strings.TrimSpace(os.Getenv("DESKTOP_SESSION")), "omarchy")
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
