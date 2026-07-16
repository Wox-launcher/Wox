package launcher

import (
	"context"
	"encoding/json"
	"log"
	"slices"
	"sort"
	"strings"
	"time"
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
	RefreshIntervalMs int
}

func (a *App) glanceEligibleLocked() bool {
	if !a.visible || a.mode != viewLauncher || !a.settings.EnableGlance || a.settings.PrimaryGlance.PluginID == "" || a.settings.PrimaryGlance.GlanceID == "" {
		return false
	}
	if a.query.QueryType != "input" || a.layout.Icon.ImageData != "" {
		return false
	}
	if a.query.QueryText == "" {
		return true
	}
	return a.queryContextKnown && a.queryContext.IsGlobalQuery
}

// handleRefreshGlance routes plugin push refreshes through the same stale-query guard as interval refreshes.
func (a *App) handleRefreshGlance(raw json.RawMessage) error {
	var request struct {
		PluginID string   `json:"PluginId"`
		IDs      []string `json:"Ids"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &request); err != nil {
			return err
		}
	}
	go a.refreshGlance("manualRefresh", request.PluginID, request.IDs)
	return nil
}

// refreshGlance loads the selected global accessory and rejects replies for superseded query sessions.
func (a *App) refreshGlance(reason, pluginID string, ids []string) {
	a.mu.Lock()
	ref := a.settings.PrimaryGlance
	if pluginID != "" && (ref.PluginID != pluginID || (len(ids) > 0 && !slices.Contains(ids, ref.GlanceID))) {
		a.mu.Unlock()
		return
	}
	if !a.glanceEligibleLocked() {
		a.stopGlanceLocked(true)
		a.mu.Unlock()
		_ = a.window.Invalidate()
		return
	}
	if a.glanceLoading && pluginID == "" && reason != "settingsChanged" {
		a.mu.Unlock()
		return
	}
	a.cancelGlanceTimerLocked()
	a.glanceRevision++
	a.glanceLoading = true
	revision := a.glanceRevision
	queryID := a.query.QueryID
	a.mu.Unlock()

	go a.loadGlanceCatalog()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	var items []glanceItem
	err := a.client.Post(ctx, "/glance", map[string]any{"Glances": []glanceRef{ref}, "Reason": reason}, &items)
	cancel()

	var selected *glanceItem
	if err == nil {
		for index := range items {
			if items[index].PluginID == ref.PluginID && items[index].ID == ref.GlanceID && strings.TrimSpace(items[index].Text) != "" {
				copy := items[index]
				selected = &copy
				break
			}
		}
	}
	a.mu.Lock()
	if revision != a.glanceRevision || queryID != a.query.QueryID || ref != a.settings.PrimaryGlance || !a.glanceEligibleLocked() {
		a.mu.Unlock()
		return
	}
	a.glanceLoading = false
	if err != nil {
		log.Printf("refresh glance: %v", err)
		a.glanceItem = nil
	} else {
		a.glanceItem = selected
		a.scheduleGlanceRefreshLocked(ref)
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
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

func (a *App) scheduleGlanceRefreshLocked(ref glanceRef) {
	interval := 60 * time.Second
	for _, item := range a.glanceCatalog {
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
func (a *App) loadGlanceCatalog() {
	a.mu.Lock()
	if a.glanceCatalogLoaded || a.glanceCatalogLoading {
		a.mu.Unlock()
		return
	}
	a.glanceCatalogLoading = true
	a.glanceCatalogError = ""
	a.mu.Unlock()

	var plugins []struct {
		ID      string         `json:"Id"`
		Name    string         `json:"Name"`
		Glances []pluginGlance `json:"Glances"`
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err := a.client.Post(ctx, "/plugin/installed", map[string]any{}, &plugins)
	cancel()
	catalog := make([]glanceCatalogItem, 0)
	if err == nil {
		for _, plugin := range plugins {
			for _, glance := range plugin.Glances {
				if strings.TrimSpace(plugin.ID) == "" || strings.TrimSpace(glance.ID) == "" {
					continue
				}
				catalog = append(catalog, glanceCatalogItem{
					Ref: glanceRef{PluginID: plugin.ID, GlanceID: glance.ID}, PluginName: plugin.Name, Name: glance.Name,
					Description: glance.Description, RefreshIntervalMs: glance.RefreshIntervalMs,
				})
			}
		}
		sort.SliceStable(catalog, func(i, j int) bool {
			return strings.ToLower(catalog[i].Name+catalog[i].PluginName) < strings.ToLower(catalog[j].Name+catalog[j].PluginName)
		})
	}
	a.mu.Lock()
	a.glanceCatalogLoading = false
	if err != nil {
		a.glanceCatalogError = err.Error()
	} else {
		a.glanceCatalog = catalog
		a.glanceCatalogLoaded = true
		a.glanceCatalogError = ""
		if a.glanceItem != nil {
			a.scheduleGlanceRefreshLocked(a.settings.PrimaryGlance)
		}
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) executeGlanceAction() {
	a.mu.RLock()
	item := a.glanceItem
	if item == nil || item.Action == nil {
		a.mu.RUnlock()
		return
	}
	pluginID := item.PluginID
	glanceID := item.ID
	action := *item.Action
	a.mu.RUnlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		err := a.client.Post(ctx, "/glance/action", map[string]string{"PluginId": pluginID, "GlanceId": glanceID, "ActionId": action.ID}, nil)
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
	}()
}

func (a *App) reloadGlanceCatalogFromCore() {
	a.mu.Lock()
	a.glanceCatalog = nil
	a.glanceCatalogLoaded = false
	a.glanceCatalogError = ""
	a.mu.Unlock()
	a.loadGlanceCatalog()
	a.mu.RLock()
	refresh := a.glanceEligibleLocked()
	a.mu.RUnlock()
	if refresh {
		a.refreshGlance("settingsChanged", "", nil)
	}
}
