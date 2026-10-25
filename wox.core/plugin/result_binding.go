package plugin

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"strings"
	"time"

	"wox/common"
	"wox/common/icons"
	"wox/database"
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
		case "identitykey", "scorekey":
			if result.IdentityKey != "" {
				hashTitle = result.IdentityKey
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
// currentResultBindingsForTest replaces the saved binding list when tests have no settings database.
var currentResultBindingsForTest func() []setting.ResultBinding

func currentResultBindings() []setting.ResultBinding {
	if currentResultBindingsForTest != nil {
		return currentResultBindingsForTest()
	}
	if database.GetDB() == nil {
		return nil
	}
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

// ResultBindingQuery is the plugin query used to find one imported shortcut target.
type ResultBindingQuery struct {
	Search         string
	Command        string
	TriggerKeyword string
}

// PluginQuery builds the input query a system plugin already understands.
func (q ResultBindingQuery) PluginQuery() Query {
	search := strings.TrimSpace(q.Search)
	command := strings.TrimSpace(q.Command)
	trigger := strings.TrimSpace(q.TriggerKeyword)
	raw := search
	if trigger != "" {
		raw = trigger
		if search != "" {
			raw += " " + search
		}
	} else if raw == "" {
		raw = command
	}
	return Query{Type: QueryTypeInput, RawQuery: raw, Search: search, Command: command, TriggerKeyword: trigger}
}

// ResultBindingSelector picks one query result. Path and identity use the result's own fields.
type ResultBindingSelector struct {
	IdentityKey         string
	Path                string
	Title               string
	Keyword             string
	RequireSettingsPath bool
}

// QueryPlugin asks one loaded plugin for results. A missing plugin returns no rows.
func (m *Manager) QueryPlugin(ctx context.Context, pluginID string, query Query) (results []QueryResult, err error) {
	if m == nil {
		return nil, fmt.Errorf("plugin manager is unavailable")
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			results = nil
			err = fmt.Errorf("plugin query failed")
		}
	}()
	instance := m.getPluginInstance(pluginID)
	if instance == nil || instance.Plugin == nil {
		return nil, fmt.Errorf("plugin %s is unavailable", pluginID)
	}
	return instance.Plugin.Query(ctx, query).Results, nil
}

// SelectQueryResult returns the first result that matches the selector.
func SelectQueryResult(results []QueryResult, selector ResultBindingSelector) (QueryResult, bool) {
	for _, result := range results {
		path := resultContextValue(result, "path")
		if selector.IdentityKey != "" && sameResultBindingToken(result.IdentityKey, selector.IdentityKey) {
			return result, true
		}
		if selector.Path != "" && (sameResultBindingToken(path, selector.Path) || sameResultBindingToken(result.IdentityKey, selector.Path)) {
			return result, true
		}
		if selector.Keyword != "" && sameResultBindingToken(resultContextValue(result, "keyword"), selector.Keyword) {
			return result, true
		}
		if selector.Title != "" && strings.EqualFold(strings.TrimSpace(result.Title), strings.TrimSpace(selector.Title)) {
			if !selector.RequireSettingsPath || strings.HasPrefix(strings.ToLower(path), "ms-settings:") || strings.HasPrefix(strings.ToLower(result.IdentityKey), "ms-settings:") {
				return result, true
			}
		}
	}
	return QueryResult{}, false
}

func resultContextValue(result QueryResult, key string) string {
	action := defaultPluginAction(result)
	if action == nil || action.ContextData == nil {
		return ""
	}
	return strings.TrimSpace(action.ContextData[key])
}

func sameResultBindingToken(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	if strings.EqualFold(left, right) {
		return true
	}
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

// UpsertResultBinding stores an alias and hotkey on one query result and registers the hotkey.
// An empty alias or hotkey keeps the value already stored for that result.
func (m *Manager) UpsertResultBinding(ctx context.Context, pluginID string, query Query, result QueryResult, alias, hotkey string) (string, error) {
	status, bindings, err := m.stageResultBinding(ctx, pluginID, query, result, alias, hotkey)
	if err != nil || status == "exists" {
		return status, err
	}
	if m == nil || m.applyResultBindings == nil {
		return "failed", fmt.Errorf("result binding applier is not initialized")
	}
	if err = m.applyResultBindings(ctx, bindings); err != nil {
		if strings.Contains(err.Error(), "conflict") {
			return "conflict", err
		}
		return "failed", err
	}
	return "imported", nil
}

// PreviewResultBinding reports whether a binding can be saved without writing it.
func (m *Manager) PreviewResultBinding(ctx context.Context, pluginID string, query Query, result QueryResult, alias, hotkey string) (string, error) {
	status, _, err := m.stageResultBinding(ctx, pluginID, query, result, alias, hotkey)
	return status, err
}

// stageResultBinding merges one shortcut onto the current result-binding list.
func (m *Manager) stageResultBinding(ctx context.Context, pluginID string, query Query, result QueryResult, alias, hotkey string) (string, []setting.ResultBinding, error) {
	if m == nil {
		return "failed", nil, fmt.Errorf("plugin manager is unavailable")
	}
	instance := m.getPluginInstance(pluginID)
	if instance == nil {
		return "failed", nil, fmt.Errorf("plugin %s is unavailable", pluginID)
	}
	alias = strings.TrimSpace(alias)
	hotkey = strings.TrimSpace(hotkey)
	if alias == "" && hotkey == "" {
		return "unsupported", nil, fmt.Errorf("i18n:plugin_manager_result_binding_invalid")
	}
	if alias != "" && !setting.IsSingleWordResultAlias(alias) {
		return "unsupported", nil, fmt.Errorf("i18n:plugin_manager_result_alias_invalid")
	}
	if hotkey != "" && !canBindResultHotkey(instance, result) {
		return "unsupported", nil, fmt.Errorf("i18n:plugin_manager_result_hotkey_not_executable")
	}
	hash := m.resultBindingHash(ctx, instance, query, result, "")
	if hash == "" {
		return "unsupported", nil, fmt.Errorf("i18n:plugin_manager_result_binding_invalid")
	}
	bindings := setting.CloneResultBindings(currentResultBindings())
	previous := setting.ResultBinding{}
	existed := false
	index := -1
	for i, binding := range bindings {
		if binding.Hash == hash {
			index = i
			previous = binding
			existed = true
			break
		}
	}
	if !existed {
		contextData := common.ContextData{}
		if action := defaultPluginAction(result); action != nil {
			contextData = maps.Clone(action.ContextData)
		}
		bindings = append(bindings, setting.ResultBinding{
			Hash: hash, PluginID: instance.Metadata.Id,
			Title: m.translatePlugin(ctx, instance, result.Title), SubTitle: m.translatePlugin(ctx, instance, result.SubTitle),
			Icon: result.Icon, ContextData: contextData,
		})
		index = len(bindings) - 1
	}
	next := bindings[index]
	if alias != "" {
		next.Alias = alias
	}
	if hotkey != "" {
		next.Hotkey = hotkey
	}
	if existed && next.Alias == previous.Alias && next.Hotkey == previous.Hotkey {
		return "exists", nil, nil
	}
	bindings[index] = next
	if database.GetDB() != nil {
		if err := m.ValidateResultBindings(ctx, bindings); err != nil {
			if strings.Contains(err.Error(), "conflict") {
				return "conflict", nil, err
			}
			return "unsupported", nil, err
		}
	}
	return "ready", bindings, nil
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
	m.resultStore.startQuery(query)
	// Silent restores can still register image tokens; release them with the temporary results.
	defer m.resultStore.clearSession(sessionID)

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

// restoreResultAliases resolves and polishes alias matches independently of ordinary queries.
// Scheduling, channel delivery, and completion belong to queryScheduler.
func (m *Manager) restoreResultAliases(ctx context.Context, query Query) QueryResponse {
	if !query.IsGlobalQuery() || strings.TrimSpace(query.Search) == "" {
		return QueryResponse{}
	}

	var results []QueryResult
	for _, binding := range currentResultBindings() {
		match := aliasMatchForQuery(ctx, binding.Alias, query.Search)
		if match == aliasMatchNone {
			continue
		}
		if ctx.Err() != nil {
			return QueryResponse{}
		}

		pluginInstance := m.getPluginInstance(binding.PluginID)
		if pluginInstance == nil || pluginInstanceDisabled(pluginInstance) || !pluginInstance.Metadata.IsSupportFeature(MetadataFeatureMRU) || len(pluginInstance.MRURestoreCallbacks) == 0 {
			continue
		}

		restored := m.restoreResultBinding(ctx, pluginInstance, binding, query.Env)
		if ctx.Err() != nil {
			return QueryResponse{}
		}

		if restored == nil {
			util.GetLogger().Debug(ctx, fmt.Sprintf("result alias restore skipped: hash=%s plugin=%s", binding.Hash, binding.PluginID))
			continue
		}
		if restored.Id == "" {
			restored.Id = resultAliasResultIDPrefix + binding.Hash
		}
		restored.Actions = append(restored.Actions, m.newResultBindingActions(ctx, pluginInstance, query, *restored, binding.Hash)...)
		polished := m.polishResultWithSource(ctx, pluginInstance, query, QueryLayout{}, *restored, binding.Hash, match)
		results = append(results, polished)
	}
	if len(results) == 0 || ctx.Err() != nil {
		return QueryResponse{}
	}
	util.GetLogger().Debug(ctx, fmt.Sprintf("result alias restore injected %d results for %q", len(results), query.Search))
	return QueryResponse{Results: results}
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
