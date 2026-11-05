package plugin

import (
	"context"
	"wox/common"
)

// ToolbarMsg is the payload pushed to the launcher toolbar.
type ToolbarMsg struct {
	Id             string
	Title          string
	Icon           common.WoxImage
	HideIcon       bool // HideIcon suppresses both an explicit icon and the default plugin icon.
	Progress       *int // Progress is clamped to 0-100 when the work has a measurable percentage.
	Indeterminate  bool // Indeterminate takes precedence over Progress and shows a spinner.
	IsPersist      bool // IsPersist keeps the message until it is cleared or the plugin context is left.
	Actions        []ToolbarMsgAction
	HostSnapshotId string `json:"-"` // HostSnapshotId identifies a host-owned callback snapshot.
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

// ToolbarMsgActionUI and ToolbarMsgUI keep existing internal callers compatible.
type ToolbarMsgActionUI = common.ToolbarMsgActionUI
type ToolbarMsgUI = common.ToolbarMsgUI
