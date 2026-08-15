package launcher

import (
	"context"
	"log"
	"slices"
	"strings"
	"time"

	"wox/plugin"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

type glanceRef struct {
	PluginID string `json:"PluginId"`
	GlanceID string `json:"GlanceId"`
}

type glanceItem struct {
	PluginID string        `json:"PluginId"`
	ID       string        `json:"Id"`
	Text     string        `json:"Text"`
	Icon     woxImage      `json:"Icon"`
	Tooltip  string        `json:"Tooltip"`
	Action   *glanceAction `json:"Action"`
}

type glanceRefreshRequest struct {
	ref      glanceRef
	reason   string
	revision uint64
	queryID  string
}

type glanceAction struct {
	ID                     string            `json:"Id"`
	Name                   string            `json:"Name"`
	Icon                   woxImage          `json:"Icon"`
	PreventHideAfterAction bool              `json:"PreventHideAfterAction"`
	ContextData            map[string]string `json:"ContextData"`
}

type glanceCatalogItem struct {
	Ref               glanceRef
	PluginName        string
	Name              string
	Description       string
	Icon              woxImage
	Preview           *glanceItem
	RefreshIntervalMs int
}

// buildGlance resolves controller-owned image resources before delegating to the pure view.
func (a *App) buildGlance(item glanceItem, hideIcon bool, palette uiPalette, width, imageScale float32, densityMetrics launcherDensityMetrics) woxwidget.Widget {
	var icon *woxui.Image
	if !hideIcon && item.Icon.ImageData != "" {
		iconTint := palette.queryText
		iconTint.A = uint8(float32(iconTint.A) * 0.8 * 0.72)
		icon = a.imageForTint(item.Icon, &iconTint, physicalImageSize(int(densityMetrics.scaled(16)), imageScale))
	}
	return launcherview.GlanceBoundary(launcherview.GlanceProps{
		Text: item.Text, Tooltip: item.Tooltip, Width: width, Icon: icon, Theme: palette.componentTheme(), DensityScale: densityMetrics.scale,
		OnTap: a.activateGlance, OnHover: a.setGlanceHover,
	})
}

func (a *App) glanceEligibleLocked() bool {
	primary := a.generalSettings.Data().PrimaryGlance
	if !a.visible || !a.generalSettings.Data().EnableGlance || primary.PluginID == "" || primary.GlanceID == "" {
		return false
	}
	if a.query.QueryType != "input" || a.layout.Icon.ImageData != "" || len(a.layout.ScopeIcons) > 0 {
		return false
	}
	if len(a.query.QueryScope.Plugins) > 0 {
		return false
	}
	if a.query.QueryText == "" {
		return true
	}
	return a.queryContextKnown && a.queryContext.IsGlobalQuery
}

// refreshGlance loads the selected global accessory and rejects replies for superseded query sessions.
func (a *App) refreshGlance(reason, pluginID string, ids []string) {
	var request *glanceRefreshRequest
	if err := a.runOnUI("prepare glance refresh", func() {
		ref := a.generalSettings.Data().PrimaryGlance
		if pluginID != "" && (ref.PluginID != pluginID || (len(ids) > 0 && !slices.Contains(ids, ref.GlanceID))) {
			return
		}
		if !a.glanceEligibleLocked() {
			a.stopGlanceLocked(true)
			a.invalidateLauncherBoundary(launcherview.GlanceBoundaryKey)
			return
		}
		if a.glanceLoading && pluginID == "" && reason != "settingsChanged" {
			return
		}
		a.cancelGlanceTimerLocked()
		a.glanceRevision++
		a.glanceLoading = true
		request = &glanceRefreshRequest{ref: ref, reason: reason, revision: a.glanceRevision, queryID: a.query.QueryID}
	}); err != nil {
		log.Printf("dispatch glance refresh preparation: %v", err)
		return
	}
	if request == nil {
		return
	}

	util.Go(a.lifecycleCtx, "load glance catalog", a.loadGlanceCatalog)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	loaded, err := a.services.GlanceItems(ctx, a.sessionID, []plugin.GlanceKey{{PluginId: request.ref.PluginID, GlanceId: request.ref.GlanceID}}, plugin.GlanceRefreshReason(request.reason))
	cancel()
	items := make([]glanceItem, len(loaded))
	for index, item := range loaded {
		items[index] = glanceItemFromUI(item)
	}

	var selected *glanceItem
	if err == nil {
		for index := range items {
			if items[index].PluginID == request.ref.PluginID && items[index].ID == request.ref.GlanceID && strings.TrimSpace(items[index].Text) != "" {
				copy := items[index]
				selected = &copy
				break
			}
		}
	}
	if dispatchErr := a.runOnUI("apply glance refresh", func() {
		if request.revision != a.glanceRevision || request.queryID != a.query.QueryID || request.ref != a.generalSettings.Data().PrimaryGlance || !a.glanceEligibleLocked() {
			return
		}
		a.glanceLoading = false
		if err != nil {
			log.Printf("refresh glance: %v", err)
			a.glanceItem = nil
		} else {
			a.glanceItem = selected
			a.scheduleGlanceRefreshLocked(request.ref)
		}
		a.invalidateLauncherBoundary(launcherview.GlanceBoundaryKey)
	}); dispatchErr != nil {
		log.Printf("dispatch glance refresh result: %v", dispatchErr)
	}
}

// glanceItemFromUI converts the shared core response for launcher and picker previews.
func glanceItemFromUI(item plugin.GlanceItemUI) glanceItem {
	var action *glanceAction
	if item.Action != nil {
		action = &glanceAction{
			ID: item.Action.Id, Name: item.Action.Name,
			Icon:                   woxImage{ImageType: item.Action.Icon.ImageType, ImageData: item.Action.Icon.ImageData},
			PreventHideAfterAction: item.Action.PreventHideAfterAction, ContextData: map[string]string(item.Action.ContextData),
		}
	}
	return glanceItem{
		PluginID: item.PluginId, ID: item.Id, Text: item.Text,
		Icon: woxImage{ImageType: item.Icon.ImageType, ImageData: item.Icon.ImageData}, Tooltip: item.Tooltip, Action: action,
	}
}

func (a *App) cancelGlanceTimerLocked() {
	if a.glanceTimer != nil {
		a.glanceTimer.Stop()
		a.glanceTimer = nil
	}
}

func (a *App) stopGlanceLocked(clear bool) {
	a.glanceRevision++
	a.glanceLoading = false
	a.cancelGlanceTimerLocked()
	if clear {
		a.glanceItem = nil
	}
}

func (a *App) setGlanceHover(inside bool, text string, anchor woxui.Rect) {
	a.setNativeHoverTooltip(&a.glanceTooltipRevision, "go-ui-glance", "update glance tooltip", inside, text, anchor, "top", func() *woxui.Window { return a.window })
}

func (a *App) scheduleGlanceRefreshLocked(ref glanceRef) {
	interval := 60 * time.Second
	for _, item := range a.appearanceSettings.Snapshot().GlanceCatalog {
		if item.Ref == ref && item.RefreshIntervalMs > 0 {
			interval = time.Duration(item.RefreshIntervalMs) * time.Millisecond
			break
		}
	}
	if interval < time.Second {
		interval = time.Second
	}
	a.cancelGlanceTimerLocked()
	a.glanceTimer = time.AfterFunc(interval, func() {
		a.refreshGlance("interval", "", nil)
	})
}

// loadGlanceCatalog reads translated plugin metadata once for settings choices and provider refresh intervals.
// State lives on appearanceSettings; this wrapper supplies the onLoaded callback that reschedules the active
// glance refresh against the newly cached catalog without giving the controller a back-reference to *App.
func (a *App) loadGlanceCatalog() {
	a.appearanceSettings.ReloadGlanceCatalog(context.Background(), a.services, a.sessionID, func() {
		if a.glanceItem != nil {
			a.scheduleGlanceRefreshLocked(a.generalSettings.Data().PrimaryGlance)
		}
		if a.settingsOpen || a.onboardingOpen {
			util.Go(a.lifecycleCtx, "load glance picker previews", a.loadGlancePickerPreviews)
		}
	})
}

func (a *App) loadGlancePickerPreviews() {
	a.appearanceSettings.ReloadGlancePreviews(context.Background(), a.services, a.sessionID)
}

// activateGlance preserves plugin actions and refreshes informational items on demand.
func (a *App) activateGlance() {
	if a.glanceItem == nil {
		return
	}
	if a.glanceItem.Action != nil {
		a.executeGlanceAction()
		return
	}
	util.Go(a.lifecycleCtx, "refresh glance after click", func() {
		a.refreshGlance("manualRefresh", "", nil)
	})
}

func (a *App) executeGlanceAction() {
	item := a.glanceItem
	if item == nil || item.Action == nil {
		return
	}
	pluginID := item.PluginID
	glanceID := item.ID
	action := *item.Action
	util.Go(a.lifecycleCtx, "execute glance action", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		err := a.services.ExecuteGlanceAction(ctx, a.sessionID, pluginID, glanceID, action.ID)
		cancel()
		if err != nil {
			log.Printf("execute glance action: %v", err)
			return
		}
		if !action.PreventHideAfterAction {
			if err := a.hideWindow(true); err != nil {
				log.Printf("hide after glance action: %v", err)
			}
		}
	})
}

func (a *App) reloadGlanceCatalogFromCore() {
	a.appearanceSettings.ResetGlanceCatalog()
	a.loadGlanceCatalog()
	a.refreshGlance("settingsChanged", "", nil)
}
