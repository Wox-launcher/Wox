package plugin

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"time"

	"wox/common"
	"wox/common/icons"
	"wox/i18n"
	"wox/setting"
	"wox/setting/definition"
	"wox/util"
	utilhotkey "wox/util/hotkey"
	"wox/util/notifier"

	"github.com/google/uuid"
)

const (
	resultBindingFormHotkeyKey = "hotkey"
	resultBindingFormAliasKey  = "alias"
	resultAliasRestoreTimeout  = 2 * time.Second
	resultAliasResultIDPrefix  = "result-alias:"
)

type aliasMatchKind int

const (
	aliasMatchNone aliasMatchKind = iota
	aliasMatchFuzzy
	aliasMatchExact
)

// ResultBindingApplier validates, registers, and persists the full binding list.
type ResultBindingApplier func(ctx context.Context, bindings []setting.ResultBinding) error

// SetResultBindingApplier injects the UI-owned register-then-persist path.
func (m *Manager) SetResultBindingApplier(fn ResultBindingApplier) {
	m.applyResultBindings = fn
}

// isResultBindingMaintenanceAction excludes configuration edits from usage history.
func isResultBindingMaintenanceAction(actionID string) bool {
	switch actionID {
	case systemActionResetRankingID, systemActionAddQueryAliasID,
		systemActionOpenPluginSettingID, systemActionRemoveFromMRUID,
		systemActionSetResultHotkeyID, systemActionSetResultAliasID:
		return true
	default:
		return false
	}
}

// mruIdentityHash shares the persisted identity rules with normal MRU writes.
func mruIdentityHash(meta Metadata, query Query, result QueryResult) string {
	hashTitle := result.Title
	hashSubTitle := result.SubTitle
	if params, err := meta.GetFeatureParamsForMRU(); err == nil {
		switch params.HashBy {
		case "rawquery":
			if query.RawQuery != "" {
				hashTitle = query.RawQuery
				hashSubTitle = ""
			}
		case "search":
			if query.Search != "" {
				hashTitle = query.Search
				hashSubTitle = ""
			}
		case "scorekey":
			if result.ScoreKey != "" {
				hashTitle = result.ScoreKey
				hashSubTitle = ""
			}
		}
	}
	return string(setting.NewResultHash(meta.Id, hashTitle, hashSubTitle))
}

// resolveResultSourceHash keeps restored identities independent of the current query.
func resolveResultSourceHash(pluginInstance *Instance, query Query, result QueryResult, sourceHash string) string {
	if strings.TrimSpace(sourceHash) != "" {
		return sourceHash
	}
	if pluginInstance == nil || !pluginInstance.Metadata.IsSupportFeature(MetadataFeatureMRU) {
		return ""
	}
	return mruIdentityHash(pluginInstance.Metadata, query, result)
}

// defaultPluginAction follows default-action selection without choosing core maintenance actions.
func defaultPluginAction(result QueryResult) *QueryResultAction {
	for i := range result.Actions {
		if result.Actions[i].IsSystemAction {
			continue
		}
		if result.Actions[i].IsDefault {
			return &result.Actions[i]
		}
	}
	for i := range result.Actions {
		if !result.Actions[i].IsSystemAction {
			return &result.Actions[i]
		}
	}
	return nil
}

func isFormResultAction(action QueryResultAction) bool {
	return action.Type == QueryResultActionTypeForm || len(action.Form) > 0 || action.OnSubmit != nil
}

func isDirectlyExecutableAction(action *QueryResultAction) bool {
	if action == nil || isFormResultAction(*action) {
		return false
	}
	return action.Action != nil
}

// canBindResultHotkey recognizes host actions before polish attaches their proxies.
func canBindResultHotkey(instance *Instance, result QueryResult) bool {
	action := defaultPluginAction(result)
	if action == nil || isFormResultAction(*action) || (action.Type != "" && action.Type != QueryResultActionTypeExecute) {
		return false
	}
	_, hasActionProxy := instance.Plugin.(ActionProxyCreator)
	return action.Action != nil || hasActionProxy
}

// currentResultBindings reads the active platform binding snapshot.
func currentResultBindings() []setting.ResultBinding {
	woxSetting := setting.GetSettingManager().GetWoxSetting(context.Background())
	if woxSetting == nil || woxSetting.ResultBindings == nil {
		return nil
	}
	return woxSetting.ResultBindings.Get()
}

// hasResultAliases avoids scheduling restores when no aliases exist.
func (m *Manager) hasResultAliases() bool {
	for _, binding := range currentResultBindings() {
		if strings.TrimSpace(binding.Alias) != "" {
			return true
		}
	}
	return false
}

// applyResultBindingTitleTags places alias and hotkey chips after the result title.
func (m *Manager) applyResultBindingTitleTags(ctx context.Context, pluginInstance *Instance, query Query, result *QueryResult, sourceHash string) {
	if result == nil {
		return
	}
	hash := resolveResultSourceHash(pluginInstance, query, *result, sourceHash)
	binding, found := setting.FindResultBinding(currentResultBindings(), hash)
	if !found {
		result.TitleTags = nil
		return
	}
	var tags []QueryResultTitleTag
	if alias := strings.TrimSpace(binding.Alias); alias != "" {
		tags = append(tags, QueryResultTitleTag{
			Text:    alias,
			Kind:    QueryResultTitleTagKindAlias,
			Tooltip: "i18n:plugin_manager_result_alias_tag_tooltip",
		})
	}
	if hotkey := strings.TrimSpace(binding.Hotkey); hotkey != "" {
		tags = append(tags, QueryResultTitleTag{
			Text:    hotkey,
			Kind:    QueryResultTitleTagKindHotkey,
			Tooltip: "i18n:plugin_manager_result_hotkey_tag_tooltip",
		})
	}
	if pluginInstance != nil {
		for index := range tags {
			tags[index].Tooltip = m.translatePlugin(ctx, pluginInstance, tags[index].Tooltip)
		}
	}
	result.TitleTags = tags
}

// newResultBindingActions exposes binding forms only for restorable plugin results.
func (m *Manager) newResultBindingActions(ctx context.Context, pluginInstance *Instance, query Query, result QueryResult, sourceHash string) []QueryResultAction {
	if pluginInstance == nil || !pluginInstance.Metadata.IsSupportFeature(MetadataFeatureMRU) {
		return nil
	}
	if len(pluginInstance.MRURestoreCallbacks) == 0 {
		return nil
	}

	hash := m.resultBindingHash(ctx, pluginInstance, query, result, sourceHash)
	if hash == "" {
		return nil
	}

	binding, found := setting.FindResultBinding(currentResultBindings(), hash)
	defaultAction := defaultPluginAction(result)
	canSetHotkey := canBindResultHotkey(pluginInstance, result) || strings.TrimSpace(binding.Hotkey) != ""
	if !found && defaultAction == nil {
		return nil
	}

	var actions []QueryResultAction
	if canSetHotkey {
		actions = append(actions, m.newSetResultHotkeyAction(pluginInstance, result, hash, binding))
	}
	actions = append(actions, m.newSetResultAliasAction(pluginInstance, result, hash, binding))
	return actions
}

// newSetResultHotkeyAction edits the hotkey without replacing the alias.
func (m *Manager) newSetResultHotkeyAction(pluginInstance *Instance, result QueryResult, hash string, binding setting.ResultBinding) QueryResultAction {
	action := QueryResultAction{
		Id:                     systemActionSetResultHotkeyID,
		Name:                   "i18n:plugin_manager_set_result_hotkey",
		Icon:                   icons.Get(icons.ActionHotkey),
		Type:                   QueryResultActionTypeForm,
		IsSystemAction:         true,
		PreventHideAfterAction: true,
		Form: definition.PluginSettingDefinitions{
			{
				Type: definition.PluginSettingDefinitionTypeHotkey,
				Value: &definition.PluginSettingValueHotkey{
					Key:          resultBindingFormHotkeyKey,
					Label:        "i18n:plugin_manager_set_result_hotkey_label",
					Tooltip:      "i18n:plugin_manager_set_result_hotkey_tooltip",
					DefaultValue: binding.Hotkey,
				},
			},
		},
		OnSubmit: func(ctx context.Context, actionContext FormActionContext) {
			m.submitResultBindingField(ctx, pluginInstance, result, hash, resultBindingFormHotkeyKey, actionContext.Values[resultBindingFormHotkeyKey], actionContext.ResultId)
		},
	}
	return action
}

// newSetResultAliasAction edits the alias without replacing the hotkey.
func (m *Manager) newSetResultAliasAction(pluginInstance *Instance, result QueryResult, hash string, binding setting.ResultBinding) QueryResultAction {
	return QueryResultAction{
		Id:                     systemActionSetResultAliasID,
		Name:                   "i18n:plugin_manager_set_result_alias",
		Icon:                   icons.Get(icons.ActionQueryAlias),
		Type:                   QueryResultActionTypeForm,
		IsSystemAction:         true,
		PreventHideAfterAction: true,
		Form: definition.PluginSettingDefinitions{
			{
				Type: definition.PluginSettingDefinitionTypeTextBox,
				Value: &definition.PluginSettingValueTextBox{
					Key:          resultBindingFormAliasKey,
					Label:        "i18n:plugin_manager_set_result_alias_label",
					Tooltip:      "i18n:plugin_manager_set_result_alias_tooltip",
					DefaultValue: binding.Alias,
				},
			},
		},
		OnSubmit: func(ctx context.Context, actionContext FormActionContext) {
			m.submitResultBindingField(ctx, pluginInstance, result, hash, resultBindingFormAliasKey, actionContext.Values[resultBindingFormAliasKey], actionContext.ResultId)
		},
	}
}

// submitResultBindingField merges a form edit with the latest saved binding.
func (m *Manager) submitResultBindingField(ctx context.Context, pluginInstance *Instance, result QueryResult, hash string, field string, rawValue string, resultID string) {
	api := NewAPI(pluginInstance)
	value := strings.TrimSpace(rawValue)
	if field == resultBindingFormHotkeyKey && value != "" && !canBindResultHotkey(pluginInstance, result) {
		api.Notify(ctx, "i18n:plugin_manager_result_hotkey_not_executable")
		return
	}
	if field == resultBindingFormAliasKey && value != "" && !setting.IsSingleWordResultAlias(value) {
		api.Notify(ctx, "i18n:plugin_manager_result_alias_invalid")
		return
	}

	bindings := setting.CloneResultBindings(currentResultBindings())
	index := -1
	for i, binding := range bindings {
		if binding.Hash == hash {
			index = i
			break
		}
	}

	if index < 0 {
		if value == "" {
			return
		}
		defaultAction := defaultPluginAction(result)
		contextData := common.ContextData{}
		if defaultAction != nil {
			contextData = maps.Clone(defaultAction.ContextData)
		}
		bindings = append(bindings, setting.ResultBinding{
			Hash:        hash,
			PluginID:    pluginInstance.Metadata.Id,
			Title:       result.Title,
			SubTitle:    result.SubTitle,
			Icon:        result.Icon,
			ContextData: contextData,
		})
		index = len(bindings) - 1
	}

	if field == resultBindingFormHotkeyKey {
		bindings[index].Hotkey = value
	} else {
		bindings[index].Alias = value
	}
	if strings.TrimSpace(bindings[index].Hotkey) == "" && strings.TrimSpace(bindings[index].Alias) == "" {
		bindings = append(bindings[:index], bindings[index+1:]...)
	}

	if m.applyResultBindings == nil {
		util.GetLogger().Error(ctx, "result binding applier is not initialized")
		api.Notify(ctx, "i18n:plugin_manager_result_binding_save_failed")
		return
	}
	if err := m.applyResultBindings(ctx, bindings); err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("save result binding: %v", err))
		message := err.Error()
		if !strings.HasPrefix(message, "i18n:") {
			message = "i18n:plugin_manager_result_binding_save_failed"
		}
		api.Notify(ctx, message)
		return
	}
	if resultID != "" {
		if updatable := api.GetUpdatableResult(ctx, resultID); updatable != nil {
			api.UpdateResult(ctx, *updatable)
		}
	}
	if field == resultBindingFormHotkeyKey {
		api.Notify(ctx, "i18n:plugin_manager_set_result_hotkey_success")
		return
	}
	api.Notify(ctx, "i18n:plugin_manager_set_result_alias_success")
}

// ValidateResultBindings checks result identities and alias conflicts before saving.
func (m *Manager) ValidateResultBindings(ctx context.Context, bindings []setting.ResultBinding) error {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	seenHotkeys := map[string]string{}
	seenAliases := map[string]string{}
	seenHashes := map[string]bool{}

	mainHotkey := normalizeHotkeyForCompare(woxSetting.MainHotkey.Get())
	selectionHotkey := normalizeHotkeyForCompare(woxSetting.SelectionHotkey.Get())
	queryHotkeys := map[string]bool{}
	for _, queryHotkey := range woxSetting.QueryHotkeys.Get() {
		if normalized := normalizeHotkeyForCompare(queryHotkey.Hotkey); normalized != "" {
			queryHotkeys[normalized] = true
		}
	}

	queryAliasWords := map[string]bool{}
	for _, queryAlias := range woxSetting.QueryAliases.Get() {
		if word := setting.NormalizeResultAlias(queryAlias.Alias); word != "" {
			queryAliasWords[word] = true
		}
	}

	triggerWords := map[string]bool{}
	if m != nil {
		for _, pluginInstance := range m.pluginInstancesSnapshot() {
			for _, keyword := range pluginInstance.GetTriggerKeywords() {
				if word := setting.NormalizeResultAlias(keyword); word != "" && word != "*" {
					triggerWords[word] = true
				}
			}
		}
	}

	for _, binding := range bindings {
		if strings.TrimSpace(binding.Hash) == "" || strings.TrimSpace(binding.PluginID) == "" || seenHashes[binding.Hash] {
			return fmt.Errorf("i18n:plugin_manager_result_binding_invalid")
		}
		seenHashes[binding.Hash] = true
		if hotkey := strings.TrimSpace(binding.Hotkey); hotkey != "" {
			normalized := normalizeHotkeyForCompare(hotkey)
			if normalized == "" {
				return fmt.Errorf("i18n:plugin_manager_result_hotkey_invalid")
			}
			if normalized == mainHotkey || normalized == selectionHotkey || queryHotkeys[normalized] {
				return fmt.Errorf("i18n:plugin_manager_result_hotkey_conflict")
			}
			if owner, exists := seenHotkeys[normalized]; exists && owner != binding.Hash {
				return fmt.Errorf("i18n:plugin_manager_result_hotkey_conflict")
			}
			seenHotkeys[normalized] = binding.Hash
		}
		if alias := strings.TrimSpace(binding.Alias); alias != "" {
			if !setting.IsSingleWordResultAlias(alias) {
				return fmt.Errorf("i18n:plugin_manager_result_alias_invalid")
			}
			normalized := setting.NormalizeResultAlias(alias)
			if queryAliasWords[normalized] || triggerWords[normalized] {
				return fmt.Errorf("i18n:plugin_manager_result_alias_conflict")
			}
			if owner, exists := seenAliases[normalized]; exists && owner != binding.Hash {
				return fmt.Errorf("i18n:plugin_manager_result_alias_conflict")
			}
			seenAliases[normalized] = binding.Hash
		}
	}
	return nil
}

// ValidateQueryHotkeysAgainstBindings rejects a query hotkey that collides with a result hotkey.
func ValidateQueryHotkeysAgainstBindings(queryHotkeys []setting.QueryHotkey, bindings []setting.ResultBinding) error {
	used := map[string]bool{}
	for _, binding := range bindings {
		if normalized := normalizeHotkeyForCompare(binding.Hotkey); normalized != "" {
			used[normalized] = true
		}
	}
	for _, queryHotkey := range queryHotkeys {
		if normalized := normalizeHotkeyForCompare(queryHotkey.Hotkey); normalized != "" && used[normalized] {
			return fmt.Errorf("i18n:plugin_manager_result_hotkey_conflict")
		}
	}
	return nil
}

// ValidateTriggerKeywordsAgainstAliases rejects a trigger keyword that collides with a result alias.
func ValidateTriggerKeywordsAgainstAliases(keywords []string, bindings []setting.ResultBinding) error {
	for _, keyword := range keywords {
		normalized := setting.NormalizeResultAlias(keyword)
		if normalized == "" || normalized == "*" {
			continue
		}
		for _, binding := range bindings {
			if setting.NormalizeResultAlias(binding.Alias) == normalized {
				return fmt.Errorf("i18n:plugin_manager_result_alias_conflict")
			}
		}
	}
	return nil
}

// ValidateQueryAliasAgainstResultAliases keeps query-alias expansion from shadowing result aliases.
func ValidateQueryAliasAgainstResultAliases(queryAlias setting.QueryAlias, bindings []setting.ResultBinding) error {
	normalized := setting.NormalizeResultAlias(queryAlias.Alias)
	if normalized == "" {
		return nil
	}
	for _, binding := range bindings {
		if setting.NormalizeResultAlias(binding.Alias) == normalized {
			return fmt.Errorf("i18n:plugin_manager_result_alias_conflict")
		}
	}
	return nil
}

// normalizeHotkeyForCompare uses the same parser as native registration.
func normalizeHotkeyForCompare(value string) string {
	key, _ := utilhotkey.BindingKey(value)
	return key
}

// resultBindingHash preserves the existing MRU convention of hashing translated titles.
// Actions are built before polish, so normalize their identity at the same boundary.
func (m *Manager) resultBindingHash(ctx context.Context, instance *Instance, query Query, result QueryResult, sourceHash string) string {
	if sourceHash != "" {
		return sourceHash
	}
	result.Title = m.translatePlugin(ctx, instance, result.Title)
	result.SubTitle = m.translatePlugin(ctx, instance, result.SubTitle)
	return resolveResultSourceHash(instance, query, result, "")
}

// resultBindingDisplayIdentity is the snapshot key used to collapse an alias
// restore and the same plugin row into one visible result.
func resultBindingDisplayIdentity(cache *QueryResultCache) string {
	if cache == nil {
		return ""
	}
	return resolveResultSourceHash(cache.PluginInstance, cache.Query, cache.Result, cache.SourceMRUHash)
}

// dedupeResultBindingCaches keeps the first cache for each binding identity.
func dedupeResultBindingCaches(caches []*QueryResultCache) []*QueryResultCache {
	if len(caches) < 2 {
		return caches
	}
	seen := make(map[string]struct{}, len(caches))
	deduped := caches[:0]
	for _, cache := range caches {
		identity := resultBindingDisplayIdentity(cache)
		if identity != "" {
			if _, exists := seen[identity]; exists {
				continue
			}
			seen[identity] = struct{}{}
		}
		deduped = append(deduped, cache)
	}
	return deduped
}

// dropResultCachesDuplicateOfAlias hides plugin rows already shown as alias restores.
func dropResultCachesDuplicateOfAlias(aliasResults, ordinaryResults []*QueryResultCache) []*QueryResultCache {
	if len(aliasResults) == 0 {
		return ordinaryResults
	}
	seen := make(map[string]bool, len(aliasResults))
	for _, cache := range aliasResults {
		if identity := resultBindingDisplayIdentity(cache); identity != "" {
			seen[identity] = true
		}
	}
	kept := ordinaryResults[:0]
	for _, cache := range ordinaryResults {
		if !seen[resultBindingDisplayIdentity(cache)] {
			kept = append(kept, cache)
		}
	}
	return kept
}

// ExecuteResultBindingHotkey uses an isolated query cache for the restored action.
func (m *Manager) ExecuteResultBindingHotkey(ctx context.Context, binding setting.ResultBinding) error {
	pluginInstance := m.getPluginInstance(binding.PluginID)
	if pluginInstance == nil || pluginInstanceDisabled(pluginInstance) || !pluginInstance.Metadata.IsSupportFeature(MetadataFeatureMRU) || len(pluginInstance.MRURestoreCallbacks) == 0 {
		notifyResultBindingFailure("i18n:plugin_manager_result_binding_unavailable")
		return fmt.Errorf("result binding plugin unavailable: %s", binding.PluginID)
	}

	sessionID := uuid.NewString()
	queryID := uuid.NewString()
	queryCtx := util.WithQueryIdContext(util.WithSessionContext(ctx, sessionID), queryID)
	query := Query{Id: queryID, SessionId: sessionID, Type: QueryTypeInput}
	m.fillResultBindingQueryEnv(queryCtx, &query)
	m.startSessionQueryCache(query)
	defer m.sessionQueryResultCache.Delete(sessionID)

	restored := m.restoreResultBinding(queryCtx, pluginInstance, binding, query.Env)
	if restored == nil {
		notifyResultBindingFailure("i18n:plugin_manager_result_binding_restore_failed")
		return fmt.Errorf("result binding restore failed: %s", binding.Hash)
	}
	polished := m.polishResultWithSource(queryCtx, pluginInstance, query, QueryLayout{}, *restored, binding.Hash, aliasMatchNone)

	action := defaultPluginAction(polished)
	if !isDirectlyExecutableAction(action) {
		notifyResultBindingFailure("i18n:plugin_manager_result_hotkey_not_executable")
		return fmt.Errorf("result binding default action is not executable: %s", binding.Hash)
	}
	if err := m.ExecuteAction(queryCtx, sessionID, queryID, polished.Id, action.Id); err != nil {
		notifyResultBindingFailure("i18n:plugin_manager_result_binding_execute_failed")
		return err
	}
	return nil
}

// fillResultBindingQueryEnv captures the current desktop context before plugin filtering.
func (m *Manager) fillResultBindingQueryEnv(ctx context.Context, query *Query) {
	if ui := m.GetUI(); ui != nil {
		snapshot := ui.GetActiveWindowSnapshot(ctx)
		query.Env.ActiveWindowTitle = snapshot.Name
		query.Env.ActiveWindowPid = snapshot.Pid
		query.Env.ActiveWindowId = snapshot.WindowId
		query.Env.ActiveWindowIcon = snapshot.Icon
		query.Env.ActiveWindowIsOpenSaveDialog = snapshot.IsOpenSaveDialog
		query.Env.ActiveWindowIsOpenSaveDialogSelectFolder = snapshot.IsOpenSaveDialogSelectFolder
		query.Env.ActiveBrowserUrl = m.getActiveBrowserUrl(ctx)
	}
}

// restoreResultBinding bounds the wait even when a plugin ignores cancellation.
func (m *Manager) restoreResultBinding(ctx context.Context, pluginInstance *Instance, binding setting.ResultBinding, env QueryEnv) *QueryResult {
	if ctx.Err() != nil {
		return nil
	}
	item := setting.MRUItem{
		Hash:        binding.Hash,
		PluginID:    binding.PluginID,
		Title:       binding.Title,
		SubTitle:    binding.SubTitle,
		Icon:        binding.Icon,
		ContextData: maps.Clone(binding.ContextData),
	}
	// Filter environment for both alias and hotkey restores, just like QueryMRU.
	pluginQuery := m.buildPluginQueryEnv(ctx, pluginInstance, Query{Env: env})
	// ponytail: skip overlapping restores for one binding; share in-flight results
	// if callers must wait. A stuck callback must not accumulate on every keystroke.
	restoreKey := binding.PluginID + "\x00" + binding.Hash
	if _, busy := m.resultBindingRestores.LoadOrStore(restoreKey, struct{}{}); busy {
		return nil
	}
	restoreCtx, cancel := context.WithTimeout(ctx, resultAliasRestoreTimeout)
	defer cancel()
	resultCh := make(chan *QueryResult, 1)
	util.Go(restoreCtx, "restore result binding", func() {
		defer m.resultBindingRestores.Delete(restoreKey)
		defer close(resultCh)
		restored := m.restoreFromMRU(restoreCtx, pluginInstance, item, pluginQuery.Env)
		resultCh <- restored
	})
	select {
	case <-restoreCtx.Done():
		util.GetLogger().Debug(ctx, fmt.Sprintf("result binding restore canceled: hash=%s: %v", binding.Hash, restoreCtx.Err()))
		return nil
	case restored := <-resultCh:
		if restoreCtx.Err() != nil || restored == nil {
			return nil
		}
		return restored
	}
}

// runResultAliasJob restores matches independently of ordinary plugin queries.
func (m *Manager) runResultAliasJob(e *queryExecution, job queryPluginJob) {
	util.Go(e.ctx, "result alias restore", func() {
		defer e.tracker.finishJob(job.blocksFallback)
		if !e.query.IsGlobalQuery() || strings.TrimSpace(e.query.Search) == "" {
			return
		}

		var results []QueryResult
		for _, binding := range currentResultBindings() {
			match := aliasMatchForQuery(e.ctx, binding.Alias, e.query.Search)
			if match == aliasMatchNone {
				continue
			}
			if e.ctx.Err() != nil {
				return
			}

			pluginInstance := m.getPluginInstance(binding.PluginID)
			if pluginInstance == nil || pluginInstanceDisabled(pluginInstance) || !pluginInstance.Metadata.IsSupportFeature(MetadataFeatureMRU) || len(pluginInstance.MRURestoreCallbacks) == 0 {
				continue
			}

			restored := m.restoreResultBinding(e.ctx, pluginInstance, binding, e.query.Env)
			if e.ctx.Err() != nil {
				return
			}

			if restored == nil {
				util.GetLogger().Debug(e.ctx, fmt.Sprintf("result alias restore skipped: hash=%s plugin=%s", binding.Hash, binding.PluginID))
				continue
			}
			if restored.Id == "" {
				restored.Id = resultAliasResultIDPrefix + binding.Hash
			}
			restored.Actions = append(restored.Actions, m.newResultBindingActions(e.ctx, pluginInstance, e.query, *restored, binding.Hash)...)
			polished := m.polishResultWithSource(e.ctx, pluginInstance, e.query, QueryLayout{}, *restored, binding.Hash, match)
			results = append(results, polished)
		}
		if len(results) == 0 || e.ctx.Err() != nil {
			return
		}
		util.GetLogger().Debug(e.ctx, fmt.Sprintf("result alias restore injected %d results for %q", len(results), e.query.Search))
		response := QueryResponse{Results: results}
		select {
		case e.resultsChan <- response.ToUI():
		case <-e.ctx.Done():
		}
	})
}

// aliasMatchForQuery distinguishes exact matches from fuzzy matches for display order.
func aliasMatchForQuery(ctx context.Context, alias string, search string) aliasMatchKind {
	alias = strings.TrimSpace(alias)
	search = strings.TrimSpace(search)
	if alias == "" || search == "" {
		return aliasMatchNone
	}
	if strings.EqualFold(alias, search) {
		return aliasMatchExact
	}
	if IsStringMatch(ctx, alias, search) {
		return aliasMatchFuzzy
	}
	return aliasMatchNone
}

// notifyResultBindingFailure reports explicit hotkey failures without opening Wox.
func notifyResultBindingFailure(message string) {
	woxIcon, _ := icons.Get(icons.BrandWox).ToImage()
	notifier.Notify(woxIcon, i18n.GetI18nManager().TranslateWox(context.Background(), strings.TrimPrefix(message, "i18n:")))
}
