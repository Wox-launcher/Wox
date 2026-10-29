package tray

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
