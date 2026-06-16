package plugin

import (
	"context"
	"wox/common"
)

type GlanceRefreshReason string

const (
	GlanceRefreshReasonWindowShown     GlanceRefreshReason = "windowShown"
	GlanceRefreshReasonInterval        GlanceRefreshReason = "interval"
	GlanceRefreshReasonManualRefresh   GlanceRefreshReason = "manualRefresh"
	GlanceRefreshReasonSettingsChanged GlanceRefreshReason = "settingsChanged"
)

// GlanceKey is the persisted global identity for a Glance item. Plugin-local ids
// are intentionally kept scoped so different plugins can expose "time" or
// "status" without conflicting in settings.
type GlanceKey struct {
	PluginId string
	GlanceId string
}

type GlanceRequest struct {
	Ids    []string
	Reason GlanceRefreshReason
}

type GlanceResponse struct {
	Items []GlanceItem
}

type GlanceItem struct {
	Id      string
	Text    string
	Icon    common.WoxImage
	Tooltip string
	Action  *GlanceAction
}

type GlanceAction struct {
	Id                     string
	Name                   string
	Icon                   common.WoxImage
	PreventHideAfterAction bool
	ContextData            common.ContextData
	Action                 func(ctx context.Context, actionContext GlanceActionContext) `json:"-"`
}

type GlanceActionContext struct {
	PluginId    string
	GlanceId    string
	ActionId    string
	ContextData common.ContextData
}

type GlanceItemUI struct {
	PluginId string
	Id       string
	Text     string
	Icon     common.WoxImage
	Tooltip  string
	Action   *GlanceActionUI
}

type GlanceActionUI struct {
	Id                     string
	Name                   string
	Icon                   common.WoxImage
	PreventHideAfterAction bool
	ContextData            common.ContextData
}
