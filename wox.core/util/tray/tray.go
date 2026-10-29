package tray

import (
	"os"
	"strings"
	"wox/util"
)

// SupportsQueryIcons reports whether this session can display query tray icons.
func SupportsQueryIcons() bool {
	if !util.IsLinux() {
		return true
	}
	return util.IsOmarchy() && util.IsHyprlandSession() && strings.EqualFold(strings.TrimSpace(os.Getenv("DESKTOP_SESSION")), "omarchy")
}

type MenuItem struct {
	Title    string
	Callback func()
	// SeparatorBefore starts a native menu group without adding an action.
	SeparatorBefore bool
}

type ClickRect struct {
	DisplayID string
	X         int
	Y         int
	Width     int
	Height    int
}

type QueryIconItem struct {
	Identifier          string
	Icon                []byte
	Tooltip             string
	Callback            func(ClickRect)
	ContextMenuTitle    string
	ContextMenuCallback func()
}
