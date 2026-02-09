package tray

type MenuItem struct {
	Title    string
	Callback func()
}

type ClickRect struct {
	X      int
	Y      int
	Width  int
	Height int
}

type QueryIconItem struct {
	Icon     []byte
	Tooltip  string
	Callback func(ClickRect)
}
