package plugin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"wox/common"
	"wox/common/icons"
	"wox/network"
	"wox/util"
	"wox/util/notifier"

	"github.com/google/uuid"
	"github.com/samber/lo"
)

// pluginPresentation owns Glance/Toolbar callbacks and active query-session routing.
// It receives only instance lookup and UI capabilities, never a Manager reference.
type pluginPresentation struct {
	toolbarMsgActions     *util.HashMap[string, *toolbarMsgActionEntry]
	pluginToolbarMsgIds   *util.HashMap[string, *toolbarMsgActionEntry]
	glanceActions         *util.HashMap[string, GlanceAction]
	sessionPluginQueries  *util.HashMap[string, *sessionPluginQueryState]
	latestSessionQueries  *util.HashMap[string, string]
	lookup                func(string) *Instance
	currentUI             func() common.UI
	scheduleToolbarExpiry func(time.Duration, func())
	scheduleToolbarHold   func(time.Duration, func())
}

const toolbarMsgDefaultDuration = 10 * time.Second
const toolbarSelectionHoldDuration = 1200 * time.Millisecond

// newPluginPresentation receives live capabilities without retaining a second UI reference.
func newPluginPresentation(lookup func(string) *Instance, currentUI func() common.UI) pluginPresentation {
	return pluginPresentation{
		lookup:                lookup,
		currentUI:             currentUI,
		scheduleToolbarExpiry: func(delay time.Duration, expire func()) { time.AfterFunc(delay, expire) },
		scheduleToolbarHold:   func(delay time.Duration, expire func()) { time.AfterFunc(delay, expire) },
		toolbarMsgActions:     util.NewHashMap[string, *toolbarMsgActionEntry](),
		pluginToolbarMsgIds:   util.NewHashMap[string, *toolbarMsgActionEntry](),
		glanceActions:         util.NewHashMap[string, GlanceAction](),
		sessionPluginQueries:  util.NewHashMap[string, *sessionPluginQueryState](),
		latestSessionQueries:  util.NewHashMap[string, string](),
	}
}

// activePlugin preserves exact-ID matching for enter/leave notifications.
func (p *pluginPresentation) activePlugin(id string) *Instance {
	instance := p.lookup(id)
	if instance == nil || instance.Metadata.Id != id {
		return nil
	}
	return instance
}

// releasePlugin drops UI callbacks when their plugin runtime is no longer usable.
// Query-session routing stays intact so re-enabling the same plugin preserves it.
func (p *pluginPresentation) releasePlugin(pluginID string) {
	notifier.CancelPlugin(pluginID)
	if instance := p.lookup(pluginID); instance != nil {
		if api, ok := instance.API.(*APIImpl); ok {
			api.stopHotkeys()
			api.closeWindows()
		}
	}
	for _, sessionID := range p.sessionPluginQueries.Keys() {
		p.sessionPluginQueries.Compute(sessionID, func(state *sessionPluginQueryState, found bool) (*sessionPluginQueryState, bool) {
			if found && state.ToolbarHold != nil && state.ToolbarHold.queued != nil && state.ToolbarHold.queued.instance.Metadata.Id == pluginID {
				copyState, copyHold := *state, *state.ToolbarHold
				copyHold.queued = nil
				copyState.ToolbarHold = &copyHold
				return &copyState, true
			}
			return state, found
		})
	}
	// Clear the visible message along with its callbacks, including persistent messages.
	for _, key := range p.pluginToolbarMsgIds.Keys() {
		sessionID, ownerID, ok := strings.Cut(key, "\x00")
		if ok && ownerID == pluginID {
			p.clearCurrentPluginToolbarMsg(util.WithSessionContext(util.NewTraceContext(), sessionID), pluginID)
		}
	}
	prefix := pluginID + "\x00"
	for _, key := range p.glanceActions.Keys() {
		if strings.HasPrefix(key, prefix) {
			p.glanceActions.Delete(key)
		}
	}
}

type toolbarMsgActionEntry struct {
	PluginId      string
	MessageId     string
	QueryId       string
	RequireActive bool
	Actions       map[string]ToolbarMsgAction
}

func glanceActionCacheKey(pluginId, glanceId, actionId string) string {
	return pluginId + "\x00" + glanceId + "\x00" + actionId
}

func (p *pluginPresentation) clearGlanceActions(pluginId string, glanceIds []string) {
	// A refreshed glance may return no items, so stale callbacks must be removed
	// before the new response is normalized instead of relying on UI visibility.
	prefixes := make([]string, 0, len(glanceIds))
	for _, glanceId := range glanceIds {
		prefixes = append(prefixes, glanceActionCacheKey(pluginId, glanceId, ""))
	}

	for _, key := range p.glanceActions.Keys() {
		for _, prefix := range prefixes {
			if strings.HasPrefix(key, prefix) {
				p.glanceActions.Delete(key)
				break
			}
		}
	}
}

func (p *pluginPresentation) GetGlanceItems(ctx context.Context, keys []GlanceKey, reason GlanceRefreshReason) []GlanceItemUI {
	requestIdsByPlugin := map[string][]string{}
	requested := map[string]bool{}
	for _, key := range keys {
		if key.PluginId == "" || key.GlanceId == "" {
			continue
		}
		cacheKey := key.PluginId + "\x00" + key.GlanceId
		if requested[cacheKey] {
			continue
		}
		requested[cacheKey] = true
		requestIdsByPlugin[key.PluginId] = append(requestIdsByPlugin[key.PluginId], key.GlanceId)
	}

	var uiItems []GlanceItemUI
	for pluginId, ids := range requestIdsByPlugin {
		pluginInstance := p.lookup(pluginId)
		if pluginInstance == nil || pluginInstance.Setting.Disabled.Get() {
			continue
		}
		provider, ok := pluginInstance.Plugin.(GlanceProvider)
		if !ok {
			continue
		}

		declared := map[string]bool{}
		var declaredIds []string
		for _, id := range ids {
			if pluginInstance.Metadata.HasGlance(id) {
				declared[id] = true
				declaredIds = append(declaredIds, id)
			}
		}
		if len(declaredIds) == 0 {
			continue
		}

		// Global Glance is a pull-based API. Calling the plugin only for user-selected
		// ids keeps third-party providers from occupying the query-box accessory area
		// unless the setting explicitly points at them.
		p.clearGlanceActions(pluginId, declaredIds)
		response := provider.Glance(ctx, GlanceRequest{Ids: declaredIds, Reason: reason})
		for _, item := range response.Items {
			if !declared[item.Id] {
				continue
			}
			uiItems = append(uiItems, p.normalizeGlanceItem(ctx, pluginInstance, item))
		}
	}
	return uiItems
}

func (p *pluginPresentation) normalizeGlanceItem(ctx context.Context, pluginInstance *Instance, item GlanceItem) GlanceItemUI {
	uiItem := GlanceItemUI{
		PluginId: pluginInstance.Metadata.Id,
		Id:       item.Id,
		Text:     pluginInstance.translateMetadataText(ctx, common.I18nString(item.Text)),
		Icon:     pluginInstance.ConvertIcon(ctx, item.Icon),
		Tooltip:  pluginInstance.translateMetadataText(ctx, common.I18nString(item.Tooltip)),
	}

	if item.Action != nil {
		// Glance intentionally exposes only one action in v1. A single optional
		// callback keeps the query-box accessory glanceable instead of turning it
		// into a secondary action menu.
		action := *item.Action
		if action.Id == "" {
			action.Id = uuid.NewString()
		}
		action.Name = pluginInstance.translateMetadataText(ctx, common.I18nString(action.Name))
		action.ContextData = common.ContextData(lo.Assign(map[string]string{}, action.ContextData))
		if !action.Icon.IsEmpty() {
			action.Icon = pluginInstance.ConvertIcon(ctx, action.Icon)
		}
		p.glanceActions.Store(glanceActionCacheKey(pluginInstance.Metadata.Id, item.Id, action.Id), action)
		uiItem.Action = &GlanceActionUI{
			Id:                     action.Id,
			Name:                   action.Name,
			Icon:                   action.Icon,
			PreventHideAfterAction: action.PreventHideAfterAction,
			ContextData:            common.ContextData(lo.Assign(map[string]string{}, action.ContextData)),
		}
	}

	return uiItem
}

func (p *pluginPresentation) ExecuteGlanceAction(ctx context.Context, pluginId string, glanceId string, actionId string) error {
	if instance := p.lookup(pluginId); network.IsOffline() && instance != nil && instance.Host != nil {
		return network.ErrOffline
	}
	action, found := p.glanceActions.Load(glanceActionCacheKey(pluginId, glanceId, actionId))
	if !found {
		return fmt.Errorf("glance action not found: %s/%s/%s", pluginId, glanceId, actionId)
	}
	if action.Action == nil {
		return fmt.Errorf("glance action callback missing: %s", actionId)
	}
	action.Action(ctx, GlanceActionContext{
		PluginId:    pluginId,
		GlanceId:    glanceId,
		ActionId:    actionId,
		ContextData: common.ContextData(lo.Assign(map[string]string{}, action.ContextData)),
	})
	return nil
}

type sessionPluginQueryState struct {
	PluginId      string
	QueryId       string
	SelectedOwner bool
	ResultId      string
	ToolbarHold   *toolbarSelectionHold
}

// toolbarSelectionHold keeps the displayed owner stable while selection settles.
type toolbarSelectionHold struct {
	token  *toolbarHoldToken
	owner  string
	queued *queuedToolbarMessage
}

type toolbarHoldToken struct{ marker byte }

type queuedToolbarMessage struct {
	ctx      context.Context
	instance *Instance
	message  ToolbarMsg
}

func toolbarSessionKey(sessionID, id string) string { return sessionID + "\x00" + id }

// ShowToolbarMsg normalizes action callbacks, replaces the current toolbar msg owned by this plugin,
// and forwards the latest snapshot to UI.
func (p *pluginPresentation) ShowToolbarMsg(ctx context.Context, pluginInstance *Instance, msg ToolbarMsg) {
	if pluginInstance == nil {
		return
	}
	resolvedCtx, ok := p.resolveActiveToolbarMsgContext(ctx, pluginInstance.Metadata.Id)
	if !ok {
		return
	}
	p.showSessionToolbarMsg(resolvedCtx, pluginInstance, msg, true, false)
}

// showSessionToolbarMsg publishes a message after its session has been resolved.
func (p *pluginPresentation) showSessionToolbarMsg(resolvedCtx context.Context, pluginInstance *Instance, msg ToolbarMsg, requireActive, onlyIfAbsent bool) {
	if msg.Id == "" {
		msg.Id = uuid.NewString()
	}

	normalized := p.normalizeToolbarMsg(resolvedCtx, pluginInstance, msg)
	actionEntry := &toolbarMsgActionEntry{
		PluginId:      pluginInstance.Metadata.Id,
		MessageId:     normalized.Id,
		QueryId:       util.GetContextQueryId(resolvedCtx),
		RequireActive: requireActive,
		Actions:       make(map[string]ToolbarMsgAction, len(normalized.Actions)),
	}
	for _, action := range normalized.Actions {
		actionEntry.Actions[action.Id] = action
	}

	sessionID := util.GetContextSessionId(resolvedCtx)
	key := toolbarSessionKey(sessionID, pluginInstance.Metadata.Id)
	commit := func() {
		p.clearCurrentPluginToolbarMsgAction(util.WithSessionContext(context.Background(), sessionID), pluginInstance.Metadata.Id)
		p.toolbarMsgActions.Store(toolbarSessionKey(sessionID, normalized.Id), actionEntry)
		p.pluginToolbarMsgIds.Store(key, actionEntry)
	}
	committed := !requireActive
	if requireActive {
		// Commit under the same session state locks as a selection change so an
		// old asynchronous update cannot overwrite another plugin's current status.
		p.latestSessionQueries.Compute(sessionID, func(latest string, exists bool) (string, bool) {
			p.sessionPluginQueries.Compute(sessionID, func(state *sessionPluginQueryState, found bool) (*sessionPluginQueryState, bool) {
				if found && state.ToolbarHold != nil && state.PluginId == actionEntry.PluginId && state.ToolbarHold.owner != state.PluginId {
					copyState, copyHold := *state, *state.ToolbarHold
					copyHold.queued = &queuedToolbarMessage{ctx: resolvedCtx, instance: pluginInstance, message: normalized}
					copyState.ToolbarHold = &copyHold
					return &copyState, true
				}
				owner := found && (state.PluginId == actionEntry.PluginId && state.ToolbarHold == nil || state.ToolbarHold != nil && state.ToolbarHold.owner == actionEntry.PluginId)
				_, alreadyShown := p.pluginToolbarMsgIds.Load(key)
				if owner && (!onlyIfAbsent || !alreadyShown) {
					commit()
					committed = true
				}
				return state, found
			})
			return latest, exists
		})
	} else {
		commit()
	}
	if !committed {
		return
	}
	uiMsg := normalized.toToolbarMsgUI()
	uiMsg.IsCurrent = func() bool {
		current, ok := p.pluginToolbarMsgIds.Load(key)
		if !ok || current != actionEntry {
			return false
		}
		if !actionEntry.RequireActive {
			return true
		}
		state, active := p.sessionPluginQueries.Load(sessionID)
		if !active {
			return false
		}
		if state.ToolbarHold != nil {
			return state.ToolbarHold.owner == actionEntry.PluginId
		}
		return state.PluginId == actionEntry.PluginId
	}
	// Selection ownership, not the query that started an operation, controls visibility.
	uiMsg.CanShow = uiMsg.IsCurrent
	p.currentUI().ShowToolbarMsg(resolvedCtx, uiMsg)
	if !normalized.IsPersist {
		p.scheduleToolbarExpiry(toolbarMsgDefaultDuration, func() {
			ctx := util.WithSessionContext(util.NewTraceContext(), sessionID)
			if removed := p.removePluginToolbarMsgAction(ctx, actionEntry.PluginId, actionEntry); removed != nil {
				if ui := p.currentUI(); ui != nil {
					ui.ClearToolbarMsg(ctx, removed.MessageId)
				}
			}
		})
	}
}

// ClearToolbarMsg removes the toolbar msg action callbacks owned by the current plugin and asks UI
// to clear the matching toolbar msg if it is still visible.
func (p *pluginPresentation) ClearToolbarMsg(ctx context.Context, pluginInstance *Instance, toolbarMsgId string) {
	if pluginInstance == nil || toolbarMsgId == "" {
		return
	}
	if sessionID := util.GetContextSessionId(ctx); sessionID != "" {
		p.sessionPluginQueries.Compute(sessionID, func(state *sessionPluginQueryState, found bool) (*sessionPluginQueryState, bool) {
			if found && state.ToolbarHold != nil && state.ToolbarHold.queued != nil && state.ToolbarHold.queued.instance.Metadata.Id == pluginInstance.Metadata.Id && state.ToolbarHold.queued.message.Id == toolbarMsgId {
				copyState, copyHold := *state, *state.ToolbarHold
				copyHold.queued = nil
				copyState.ToolbarHold = &copyHold
				return &copyState, true
			}
			return state, found
		})
	}
	resolvedCtx, ok := p.resolveActiveToolbarMsgContext(ctx, pluginInstance.Metadata.Id)
	if !ok {
		if util.GetContextSessionId(ctx) == "" {
			return
		}
		resolvedCtx = ctx
	}
	key := toolbarSessionKey(util.GetContextSessionId(resolvedCtx), pluginInstance.Metadata.Id)
	entry, found := p.pluginToolbarMsgIds.Load(key)
	if !found || entry.MessageId != toolbarMsgId || (!ok && entry.RequireActive) {
		return
	}
	if removed := p.removePluginToolbarMsgAction(resolvedCtx, pluginInstance.Metadata.Id, entry); removed != nil {
		if ui := p.currentUI(); ui != nil {
			ui.ClearToolbarMsg(resolvedCtx, removed.MessageId)
		}
	}
}

// ExecuteToolbarMsgAction resolves the current toolbar msg action callback and invokes it.
func (p *pluginPresentation) ExecuteToolbarMsgAction(ctx context.Context, sessionId string, toolbarMsgId string, actionId string) error {
	entry, found := p.toolbarMsgActions.Load(toolbarSessionKey(sessionId, toolbarMsgId))
	if !found {
		return fmt.Errorf("toolbar msg not found: %s", toolbarMsgId)
	}
	queryID := entry.QueryId
	if entry.RequireActive {
		state, active := p.sessionPluginQueries.Load(sessionId)
		if !active || state.PluginId != entry.PluginId {
			return fmt.Errorf("toolbar msg is no longer active: %s", toolbarMsgId)
		}
		if latest, ok := p.latestSessionQueries.Load(sessionId); ok && latest != state.QueryId {
			return fmt.Errorf("toolbar msg is no longer active: %s", toolbarMsgId)
		}
		queryID = state.QueryId
	}

	if instance := p.lookup(entry.PluginId); network.IsOffline() && instance != nil && instance.Host != nil {
		return network.ErrOffline
	}
	action, found := entry.Actions[actionId]
	if !found {
		return fmt.Errorf("toolbar msg action not found: %s", actionId)
	}
	if action.Action == nil {
		return fmt.Errorf("toolbar msg action callback missing: %s", actionId)
	}

	actionCtx := ToolbarMsgActionContext{
		ToolbarMsgId:       toolbarMsgId,
		ToolbarMsgActionId: actionId,
		ContextData:        common.ContextData(lo.Assign(map[string]string{}, action.ContextData)),
	}

	callbackCtx := ctx
	if sessionId != "" {
		callbackCtx = util.WithSessionContext(callbackCtx, sessionId)
	}
	if queryID != "" {
		callbackCtx = util.WithQueryIdContext(callbackCtx, queryID)
	}
	action.Action(callbackCtx, actionCtx)
	return nil
}

func (p *pluginPresentation) clearCurrentPluginToolbarMsgAction(ctx context.Context, pluginId string) *toolbarMsgActionEntry {
	return p.removePluginToolbarMsgAction(ctx, pluginId, nil)
}

// removePluginToolbarMsgAction only expires the expected snapshot, so an old timer cannot remove its replacement.
func (p *pluginPresentation) removePluginToolbarMsgAction(ctx context.Context, pluginId string, expected *toolbarMsgActionEntry) *toolbarMsgActionEntry {
	if pluginId == "" {
		return nil
	}
	sessionID := util.GetContextSessionId(ctx)
	key := toolbarSessionKey(sessionID, pluginId)
	var removed *toolbarMsgActionEntry
	p.pluginToolbarMsgIds.Compute(key, func(entry *toolbarMsgActionEntry, found bool) (*toolbarMsgActionEntry, bool) {
		if !found || (expected != nil && entry != expected) {
			return entry, found
		}

		removed = entry
		return nil, false
	})
	if removed != nil {
		p.toolbarMsgActions.Compute(toolbarSessionKey(sessionID, removed.MessageId), func(entry *toolbarMsgActionEntry, found bool) (*toolbarMsgActionEntry, bool) {
			if found && entry == removed {
				return nil, false
			}
			return entry, found
		})
	}
	return removed
}

func (p *pluginPresentation) clearCurrentPluginToolbarMsg(ctx context.Context, pluginId string) {
	if removed := p.clearCurrentPluginToolbarMsgAction(ctx, pluginId); removed != nil {
		if ui := p.currentUI(); ui != nil {
			ui.ClearToolbarMsg(ctx, removed.MessageId)
		}
	}
}

func (p *pluginPresentation) resolveActiveToolbarMsgContext(ctx context.Context, pluginId string) (context.Context, bool) {
	sessionId := util.GetContextSessionId(ctx)
	if sessionId != "" {
		state, found := p.sessionPluginQueries.Load(sessionId)
		if !found || (state.PluginId != pluginId && (state.ToolbarHold == nil || state.ToolbarHold.owner != pluginId)) {
			return nil, false
		}
		if state.QueryId != "" {
			ctx = util.WithQueryIdContext(ctx, state.QueryId)
		}
		return ctx, true
	}

	activeSessionId, activeQueryId, found := p.findActivePluginQuery(pluginId)
	if !found {
		return nil, false
	}

	resolvedCtx := util.WithSessionContext(ctx, activeSessionId)
	if activeQueryId != "" {
		resolvedCtx = util.WithQueryIdContext(resolvedCtx, activeQueryId)
	}
	return p.resolveActiveToolbarMsgContext(resolvedCtx, pluginId)
}

// normalizeToolbarMsg translates user-facing text, normalizes UI-facing icons, clones context data,
// and backfills host proxies for external plugin action callbacks.
func (p *pluginPresentation) normalizeToolbarMsg(ctx context.Context, pluginInstance *Instance, msg ToolbarMsg) ToolbarMsg {
	var normalizedIcon common.WoxImage
	if !msg.HideIcon {
		icon := msg.Icon
		if icon.IsEmpty() {
			icon = common.ParseWoxImageOrDefault(pluginInstance.Metadata.Icon, common.WoxImage{})
		}
		normalizedIcon = pluginInstance.ConvertIcon(ctx, icon)
	}

	normalized := ToolbarMsg{
		Id:            msg.Id,
		Title:         pluginInstance.translateMetadataText(ctx, common.I18nString(msg.Title)),
		Icon:          normalizedIcon,
		HideIcon:      msg.HideIcon,
		Progress:      msg.Progress,
		Indeterminate: msg.Indeterminate,
		IsPersist:     msg.IsPersist,
		Actions:       make([]ToolbarMsgAction, 0, len(msg.Actions)),
	}

	for _, action := range msg.Actions {
		if action.Id == "" {
			action.Id = uuid.NewString()
		}
		action.Name = pluginInstance.translateMetadataText(ctx, common.I18nString(action.Name))
		action.ContextData = common.ContextData(lo.Assign(map[string]string{}, action.ContextData))
		if action.Icon.IsEmpty() {
			action.Icon = icons.Get(icons.ActionExecute)
		} else {
			// Action icons share the same toolbar payload path as the message icon. Normalize them
			// before storing callbacks so plugin-relative assets stay usable when actions are shown.
			action.Icon = pluginInstance.ConvertIcon(ctx, action.Icon)
		}

		if action.Action == nil {
			if proxyCreator, ok := pluginInstance.Plugin.(ToolbarMsgActionProxyCreator); ok {
				action.Action = proxyCreator.CreateToolbarMsgActionProxy(action.Id)
			}
		}

		normalized.Actions = append(normalized.Actions, action)
	}

	return normalized
}

// toToolbarMsgUI strips callbacks and returns a UI-safe snapshot.
func (m ToolbarMsg) toToolbarMsgUI() ToolbarMsgUI {
	uiMsg := ToolbarMsgUI{
		Id:            m.Id,
		Title:         m.Title,
		Icon:          m.Icon,
		Progress:      m.Progress,
		Indeterminate: m.Indeterminate,
		Actions:       make([]ToolbarMsgActionUI, 0, len(m.Actions)),
	}
	for _, action := range m.Actions {
		uiMsg.Actions = append(uiMsg.Actions, ToolbarMsgActionUI{
			Id:                     action.Id,
			Name:                   action.Name,
			Icon:                   action.Icon,
			Hotkey:                 action.Hotkey,
			IsDefault:              action.IsDefault,
			PreventHideAfterAction: action.PreventHideAfterAction,
			ContextData:            common.ContextData(lo.Assign(map[string]string{}, action.ContextData)),
		})
	}
	return uiMsg
}

// HandleQueryLifecycle updates the query owner and releases the previous context.
func (p *pluginPresentation) HandleQueryLifecycle(ctx context.Context, query Query, pluginInstance *Instance) {
	if query.SessionId == "" {
		return
	}
	owner := ""
	if pluginInstance != nil && ((query.HasScope() && len(query.Scope.Deduplicate().Plugins) == 1) || (query.Type == QueryTypeInput && query.TriggerKeyword != "")) {
		owner = pluginInstance.Metadata.Id
	}
	next := &sessionPluginQueryState{PluginId: owner, QueryId: query.Id, SelectedOwner: owner == ""}
	var previous *sessionPluginQueryState
	var removed *toolbarMsgActionEntry
	var started *toolbarSelectionHold
	accepted := false
	p.latestSessionQueries.Compute(query.SessionId, func(latest string, exists bool) (string, bool) {
		if exists && latest != query.Id {
			return latest, true
		}
		p.sessionPluginQueries.Compute(query.SessionId, func(state *sessionPluginQueryState, found bool) (*sessionPluginQueryState, bool) {
			if found {
				previous = state
				started, removed = p.transitionToolbarOwner(ctx, query.SessionId, state, next, query.Id != "" && (state.SelectedOwner || next.SelectedOwner))
			}
			return next, next.QueryId != "" || next.PluginId != ""
		})
		accepted = true
		return latest, exists
	})
	if accepted {
		p.dispatchSessionOwnerChange(ctx, query.SessionId, previous, next, removed)
		p.startToolbarSelectionHold(query.SessionId, started)
	}
}

// selectResultContext activates the selected result's plugin in an ownerless query.
func (p *pluginPresentation) selectResultContext(ctx context.Context, sessionID, queryID, resultID, pluginID string) {
	var previous, next *sessionPluginQueryState
	var removed *toolbarMsgActionEntry
	var started *toolbarSelectionHold
	p.latestSessionQueries.Compute(sessionID, func(latest string, exists bool) (string, bool) {
		if exists && latest != queryID {
			return latest, true
		}
		p.sessionPluginQueries.Compute(sessionID, func(state *sessionPluginQueryState, found bool) (*sessionPluginQueryState, bool) {
			if !found || !state.SelectedOwner || state.QueryId != queryID || (state.ResultId == resultID && state.PluginId == pluginID) {
				return state, found
			}
			previous = state
			next = &sessionPluginQueryState{PluginId: pluginID, QueryId: queryID, SelectedOwner: true, ResultId: resultID}
			started, removed = p.transitionToolbarOwner(ctx, sessionID, state, next, true)
			return next, true
		})
		return latest, exists
	})
	if next != nil {
		p.dispatchSessionOwnerChange(ctx, sessionID, previous, next, removed)
		p.startToolbarSelectionHold(sessionID, started)
	}
}

// transitionToolbarOwner delays only toolbar replacement; plugin enter and leave callbacks remain immediate.
func (p *pluginPresentation) transitionToolbarOwner(ctx context.Context, sessionID string, previous, next *sessionPluginQueryState, allowHold bool) (*toolbarSelectionHold, *toolbarMsgActionEntry) {
	hold := previous.ToolbarHold
	if hold != nil {
		if allowHold && next.PluginId != hold.owner {
			copyHold := *hold
			if next.PluginId != previous.PluginId {
				copyHold.queued = nil
			}
			next.ToolbarHold = &copyHold
			return nil, nil
		}
		if next.PluginId == hold.owner && allowHold {
			return nil, nil
		}
		return nil, p.clearCurrentPluginToolbarMsgAction(util.WithSessionContext(ctx, sessionID), hold.owner)
	}
	if previous.PluginId == "" || previous.PluginId == next.PluginId {
		return nil, nil
	}
	entry, exists := p.pluginToolbarMsgIds.Load(toolbarSessionKey(sessionID, previous.PluginId))
	if allowHold && exists && entry.RequireActive {
		hold = &toolbarSelectionHold{token: &toolbarHoldToken{}, owner: previous.PluginId}
		next.ToolbarHold = hold
		return hold, nil
	}
	return nil, p.clearCurrentPluginToolbarMsgAction(util.WithSessionContext(ctx, sessionID), previous.PluginId)
}

// startToolbarSelectionHold uses one deadline even if subsequent queries change selection again.
func (p *pluginPresentation) startToolbarSelectionHold(sessionID string, hold *toolbarSelectionHold) {
	if hold == nil {
		return
	}
	p.scheduleToolbarHold(toolbarSelectionHoldDuration, func() {
		ctx := util.WithSessionContext(util.NewTraceContext(), sessionID)
		var removed *toolbarMsgActionEntry
		var queued *queuedToolbarMessage
		p.sessionPluginQueries.Compute(sessionID, func(state *sessionPluginQueryState, found bool) (*sessionPluginQueryState, bool) {
			if !found || state.ToolbarHold == nil || state.ToolbarHold.token != hold.token {
				return state, found
			}
			removed = p.clearCurrentPluginToolbarMsgAction(ctx, state.ToolbarHold.owner)
			queued = state.ToolbarHold.queued
			next := *state
			next.ToolbarHold = nil
			return &next, true
		})
		if removed != nil && p.currentUI() != nil {
			p.currentUI().ClearToolbarMsg(ctx, removed.MessageId)
		}
		if queued != nil {
			resolvedCtx, ok := p.resolveActiveToolbarMsgContext(queued.ctx, queued.instance.Metadata.Id)
			if ok {
				// A newer status may have arrived after the hold was released.
				p.showSessionToolbarMsg(resolvedCtx, queued.instance, queued.message, true, true)
			}
		}
	})
}

// dispatchSessionOwnerChange clears only this session and invokes plugin callbacks off the UI thread.
func (p *pluginPresentation) dispatchSessionOwnerChange(ctx context.Context, sessionID string, previous, next *sessionPluginQueryState, removed *toolbarMsgActionEntry) {
	if removed != nil {
		if ui := p.currentUI(); ui != nil {
			ui.ClearToolbarMsg(util.WithSessionContext(ctx, sessionID), removed.MessageId)
		}
	}
	if previous != nil && previous.PluginId == next.PluginId {
		return
	}
	if previous != nil && previous.PluginId != "" {
		leaveCtx := util.WithQueryIdContext(util.WithSessionContext(ctx, sessionID), previous.QueryId)
		if instance := p.activePlugin(previous.PluginId); instance != nil {
			for _, callback := range instance.LeavePluginQueryCallbacks {
				util.Go(leaveCtx, fmt.Sprintf("[%s] leave plugin query callback", instance.GetName(leaveCtx)), func() { callback(leaveCtx) })
			}
		}
	}
	if next.PluginId == "" {
		return
	}
	if instance := p.activePlugin(next.PluginId); instance != nil {
		enterCtx := util.WithQueryIdContext(util.WithSessionContext(ctx, sessionID), next.QueryId)
		for _, callback := range instance.EnterPluginQueryCallbacks {
			util.Go(enterCtx, fmt.Sprintf("[%s] enter plugin query callback", instance.GetName(enterCtx)), func() { callback(enterCtx) })
		}
	}
}

// isPluginActiveInSession reports whether the given plugin currently owns the session query context.
func (p *pluginPresentation) isPluginActiveInSession(sessionId string, pluginId string) bool {
	if sessionId == "" || pluginId == "" {
		return false
	}

	state, found := p.sessionPluginQueries.Load(sessionId)
	return found && state.PluginId == pluginId
}

// findActivePluginQuery returns one active session/query pair for the plugin.
// Toolbar msg routing uses this to resolve the UI session when plugins fire status updates
// without carrying a session-bound context.
func (p *pluginPresentation) findActivePluginQuery(pluginId string) (string, string, bool) {
	if pluginId == "" {
		return "", "", false
	}

	var sessionId string
	var queryId string
	ambiguous := false
	p.sessionPluginQueries.Range(func(currentSessionId string, state *sessionPluginQueryState) bool {
		if state != nil && state.PluginId == pluginId {
			if sessionId != "" {
				ambiguous = true
				return false
			}
			sessionId = currentSessionId
			queryId = state.QueryId
		}
		return true
	})

	return sessionId, queryId, sessionId != "" && !ambiguous
}
