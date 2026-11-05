package common

// ToolbarMsgActionUI is the callback-free action snapshot sent to the launcher.
type ToolbarMsgActionUI struct {
	Id                     string
	Name                   string
	Icon                   WoxImage
	Hotkey                 string
	IsDefault              bool
	PreventHideAfterAction bool
	ContextData            ContextData
}

// ToolbarMsgUI carries a plugin status and its internal snapshot validity guards.
type ToolbarMsgUI struct {
	Id            string
	Title         string
	Icon          WoxImage
	Progress      *int
	Indeterminate bool
	Actions       []ToolbarMsgActionUI
	IsCurrent     func() bool `json:"-"`
}
