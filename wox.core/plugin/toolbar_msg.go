package plugin

import (
	"context"
	"wox/common"
)

// ToolbarMsg is the payload pushed to the launcher toolbar.
type ToolbarMsg struct {
	Id            string
	Title         string
	Icon          common.WoxImage
	Progress      *int // Progress is a 0-100 value when the work has a measurable percentage.
	Indeterminate bool // Indeterminate shows a spinner without percentage when progress cannot be measured yet.
	Actions       []ToolbarMsgAction
}

// ToolbarMsgAction describes one action rendered on the toolbar while the
// toolbar msg is visible.
type ToolbarMsgAction struct {
	Id                     string
	Name                   string
	Icon                   common.WoxImage
	Hotkey                 string
	IsDefault              bool
	PreventHideAfterAction bool
	ContextData            common.ContextData                                               // ContextData is round-tripped back to the action callback.
	Action                 func(ctx context.Context, actionContext ToolbarMsgActionContext) `json:"-"`
}

// ToolbarMsgActionContext identifies the toolbar msg action invocation.
type ToolbarMsgActionContext struct {
	ToolbarMsgId       string
	ToolbarMsgActionId string
	ContextData        common.ContextData
}

// ToolbarMsgActionUI is the UI-safe action snapshot sent to Flutter.
type ToolbarMsgActionUI struct {
	Id                     string
	Name                   string
	Icon                   common.WoxImage
	Hotkey                 string
	IsDefault              bool
	PreventHideAfterAction bool
	ContextData            common.ContextData
}

// ToolbarMsgUI is the UI-safe toolbar msg snapshot sent to Flutter.
type ToolbarMsgUI struct {
	Id            string
	Title         string
	Icon          common.WoxImage
	Progress      *int
	Indeterminate bool
	Actions       []ToolbarMsgActionUI
}
