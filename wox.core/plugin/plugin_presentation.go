package plugin

import (
	"context"
	"fmt"
	"strings"

	"wox/common"
	"wox/network"
	"wox/util"

	"github.com/google/uuid"
	"github.com/samber/lo"
)

// pluginPresentation owns Glance/Toolbar callbacks and active query-session routing.
// It receives only instance lookup and UI capabilities, never a Manager reference.
type pluginPresentation struct {
	toolbarMsgActions    *util.HashMap[string, *toolbarMsgActionEntry]
	pluginToolbarMsgIds  *util.HashMap[string, string]
	glanceActions        *util.HashMap[string, GlanceAction]
	sessionPluginQueries *util.HashMap[string, *sessionPluginQueryState]
	lookup               func(string) *Instance
	currentUI            func() common.UI
}

// newPluginPresentation receives live capabilities without retaining a second UI reference.
func newPluginPresentation(lookup func(string) *Instance, currentUI func() common.UI) pluginPresentation {
	return pluginPresentation{
		lookup:               lookup,
		currentUI:            currentUI,
		toolbarMsgActions:    util.NewHashMap[string, *toolbarMsgActionEntry](),
		pluginToolbarMsgIds:  util.NewHashMap[string, string](),
		glanceActions:        util.NewHashMap[string, GlanceAction](),
		sessionPluginQueries: util.NewHashMap[string, *sessionPluginQueryState](),
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
	// Clear the visible message along with its callbacks, including persistent messages.
	p.clearCurrentPluginToolbarMsg(util.NewTraceContext(), pluginID)
	prefix := pluginID + "\x00"
	for _, key := range p.glanceActions.Keys() {
		if strings.HasPrefix(key, prefix) {
			p.glanceActions.Delete(key)
		}
	}
}

type toolbarMsgActionEntry struct {
	PluginId string
	Actions  map[string]ToolbarMsgAction
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
	PluginId string
	QueryId  string
}

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
	if msg.Id == "" {
		msg.Id = uuid.NewString()
	}

	normalized := p.normalizeToolbarMsg(resolvedCtx, pluginInstance, msg)
	p.clearCurrentPluginToolbarMsgAction(pluginInstance.Metadata.Id)

	actionEntry := &toolbarMsgActionEntry{
		PluginId: pluginInstance.Metadata.Id,
		Actions:  make(map[string]ToolbarMsgAction, len(normalized.Actions)),
	}
	for _, action := range normalized.Actions {
		actionEntry.Actions[action.Id] = action
	}

	p.toolbarMsgActions.Store(normalized.Id, actionEntry)
	p.pluginToolbarMsgIds.Store(pluginInstance.Metadata.Id, normalized.Id)
	p.currentUI().ShowToolbarMsg(resolvedCtx, normalized.toToolbarMsgUI())
}

// ClearToolbarMsg removes the toolbar msg action callbacks owned by the current plugin and asks UI
// to clear the matching toolbar msg if it is still visible.
func (p *pluginPresentation) ClearToolbarMsg(ctx context.Context, pluginInstance *Instance, toolbarMsgId string) {
	if pluginInstance == nil || toolbarMsgId == "" {
		return
	}
	resolvedCtx, ok := p.resolveActiveToolbarMsgContext(ctx, pluginInstance.Metadata.Id)
	if !ok {
		resolvedCtx = ctx
	}

	if currentToolbarMsgId, found := p.pluginToolbarMsgIds.Load(pluginInstance.Metadata.Id); found && currentToolbarMsgId == toolbarMsgId {
		p.pluginToolbarMsgIds.Delete(pluginInstance.Metadata.Id)
	}
	p.toolbarMsgActions.Delete(toolbarMsgId)
	p.currentUI().ClearToolbarMsg(resolvedCtx, toolbarMsgId)
}

// ExecuteToolbarMsgAction resolves the current toolbar msg action callback and invokes it.
func (p *pluginPresentation) ExecuteToolbarMsgAction(ctx context.Context, sessionId string, toolbarMsgId string, actionId string) error {
	entry, found := p.toolbarMsgActions.Load(toolbarMsgId)
	if !found {
		return fmt.Errorf("toolbar msg not found: %s", toolbarMsgId)
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
	action.Action(callbackCtx, actionCtx)
	return nil
}

// HasVisibleToolbarMsg reports whether backend currently tracks any persistent toolbar msg. This is
// an approximation used for routing Notify() calls without maintaining toolbar visibility policy.
func (p *pluginPresentation) HasVisibleToolbarMsg(ctx context.Context) bool {
	return p.pluginToolbarMsgIds.Len() > 0
}

// VisibleToolbarMsgOwners returns pluginId=toolbarMsgId pairs still tracked for toolbar routing.
func (p *pluginPresentation) VisibleToolbarMsgOwners() []string {
	if p == nil || p.pluginToolbarMsgIds == nil {
		return nil
	}
	owners := make([]string, 0, p.pluginToolbarMsgIds.Len())
	p.pluginToolbarMsgIds.Range(func(pluginID, msgID string) bool {
		owners = append(owners, pluginID+"="+msgID)
		return true
	})
	return owners
}

func (p *pluginPresentation) clearCurrentPluginToolbarMsgAction(pluginId string) {
	if pluginId == "" {
		return
	}

	currentToolbarMsgId, found := p.pluginToolbarMsgIds.Load(pluginId)
	if !found || currentToolbarMsgId == "" {
		return
	}

	p.pluginToolbarMsgIds.Delete(pluginId)
	p.toolbarMsgActions.Delete(currentToolbarMsgId)
}

func (p *pluginPresentation) clearCurrentPluginToolbarMsg(ctx context.Context, pluginId string) {
	if pluginId == "" {
		return
	}

	currentToolbarMsgId, found := p.pluginToolbarMsgIds.Load(pluginId)
	if !found || currentToolbarMsgId == "" {
		return
	}

	p.clearCurrentPluginToolbarMsgAction(pluginId)
	if ui := p.currentUI(); ui != nil {
		ui.ClearToolbarMsg(ctx, currentToolbarMsgId)
	}
}

func (p *pluginPresentation) resolveActiveToolbarMsgContext(ctx context.Context, pluginId string) (context.Context, bool) {
	sessionId := util.GetContextSessionId(ctx)
	if sessionId != "" {
		return ctx, p.isPluginActiveInSession(sessionId, pluginId)
	}

	activeSessionId, activeQueryId, found := p.findActivePluginQuery(pluginId)
	if !found {
		return nil, false
	}

	resolvedCtx := util.WithSessionContext(ctx, activeSessionId)
	if activeQueryId != "" {
		resolvedCtx = util.WithQueryIdContext(resolvedCtx, activeQueryId)
	}
	return resolvedCtx, true
}

// normalizeToolbarMsg translates user-facing text, normalizes UI-facing icons, clones context data,
// and backfills host proxies for external plugin action callbacks.
func (p *pluginPresentation) normalizeToolbarMsg(ctx context.Context, pluginInstance *Instance, msg ToolbarMsg) ToolbarMsg {
	icon := msg.Icon
	if icon.IsEmpty() {
		icon = common.ParseWoxImageOrDefault(pluginInstance.Metadata.Icon, common.WoxImage{})
	}
	normalizedIcon := pluginInstance.ConvertIcon(ctx, icon)

	normalized := ToolbarMsg{
		Id:            msg.Id,
		Title:         pluginInstance.translateMetadataText(ctx, common.I18nString(msg.Title)),
		Icon:          normalizedIcon,
		Progress:      msg.Progress,
		Indeterminate: msg.Indeterminate,
		Actions:       make([]ToolbarMsgAction, 0, len(msg.Actions)),
	}

	for _, action := range msg.Actions {
		if action.Id == "" {
			action.Id = uuid.NewString()
		}
		action.Name = pluginInstance.translateMetadataText(ctx, common.I18nString(action.Name))
		action.ContextData = common.ContextData(lo.Assign(map[string]string{}, action.ContextData))
		if !action.Icon.IsEmpty() {
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

// HandleQueryLifecycle updates the active plugin query for the session and fires enter/leave callbacks
// when the owning plugin changes. Leaving a plugin query also clears its toolbar msg snapshot.
func (p *pluginPresentation) HandleQueryLifecycle(ctx context.Context, query Query, pluginInstance *Instance) {
	sessionId := query.SessionId
	if sessionId == "" {
		return
	}

	nextPluginId := ""
	singleScopeOwner := query.HasScope() && len(query.Scope.Deduplicate().Plugins) == 1
	traditionalKeywordOwner := query.Type == QueryTypeInput && query.TriggerKeyword != ""
	if pluginInstance != nil && (singleScopeOwner || traditionalKeywordOwner) {
		nextPluginId = pluginInstance.Metadata.Id
	}

	prevState, hasPrev := p.sessionPluginQueries.Load(sessionId)
	prevPluginId := ""
	prevQueryId := ""
	if hasPrev {
		prevPluginId = prevState.PluginId
		prevQueryId = prevState.QueryId
	}

	if prevPluginId == nextPluginId {
		if nextPluginId == "" {
			p.sessionPluginQueries.Delete(sessionId)
		} else {
			p.sessionPluginQueries.Store(sessionId, &sessionPluginQueryState{PluginId: nextPluginId, QueryId: query.Id})
		}
		return
	}

	// Switch the active owner first so background plugin callbacks cannot re-resolve the
	// previous plugin after we leave its query context.
	if nextPluginId == "" {
		p.sessionPluginQueries.Delete(sessionId)
	} else {
		p.sessionPluginQueries.Store(sessionId, &sessionPluginQueryState{PluginId: nextPluginId, QueryId: query.Id})
	}

	if prevPluginId != "" {
		leaveCtx := util.WithQueryIdContext(util.WithSessionContext(ctx, sessionId), prevQueryId)
		p.clearCurrentPluginToolbarMsg(leaveCtx, prevPluginId)

		if prevInstance := p.activePlugin(prevPluginId); prevInstance != nil {
			for _, callback := range prevInstance.LeavePluginQueryCallbacks {
				util.Go(leaveCtx, fmt.Sprintf("[%s] leave plugin query callback", prevInstance.GetName(leaveCtx)), func() {
					callback(leaveCtx)
				})
			}
		}
	}

	if nextPluginId == "" {
		return
	}

	if nextInstance := p.activePlugin(nextPluginId); nextInstance != nil {
		enterCtx := util.WithQueryIdContext(util.WithSessionContext(ctx, sessionId), query.Id)
		for _, callback := range nextInstance.EnterPluginQueryCallbacks {
			util.Go(enterCtx, fmt.Sprintf("[%s] enter plugin query callback", nextInstance.GetName(enterCtx)), func() {
				callback(enterCtx)
			})
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
	p.sessionPluginQueries.Range(func(currentSessionId string, state *sessionPluginQueryState) bool {
		if state != nil && state.PluginId == pluginId {
			sessionId = currentSessionId
			queryId = state.QueryId
			return false
		}
		return true
	})

	return sessionId, queryId, sessionId != ""
}
