package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"wox/common"
	"wox/i18n"
)

// TriggerKeywordConflict describes one non-global trigger keyword used by more than one enabled plugin.
// The conflict is kept in core because both Doctor and query dispatch need the same ownership rule.
type TriggerKeywordConflict struct {
	Keyword         string
	PluginInstances []*Instance
}

type TriggerKeywordConflictError struct {
	Conflict TriggerKeywordConflict
}

func (e *TriggerKeywordConflictError) Error() string {
	return fmt.Sprintf("trigger keyword conflict: %s", e.Conflict.Keyword)
}

func AsTriggerKeywordConflictError(err error) (*TriggerKeywordConflictError, bool) {
	var conflictErr *TriggerKeywordConflictError
	if errors.As(err, &conflictErr) {
		return conflictErr, true
	}
	return nil, false
}

type triggerKeywordConflictPreviewPlugin struct {
	PluginId   string
	PluginName string
	// The preview now renders each conflicted plugin with its real icon. Carrying
	// the normalized WoxImage here keeps Flutter on the same image contract as
	// plugin settings instead of inventing a preview-only icon fallback.
	Icon            common.WoxImage
	TriggerKeywords []string
}

type triggerKeywordConflictPreviewData struct {
	Keyword string
	Title   string
	Message string
	Plugins []triggerKeywordConflictPreviewPlugin
}

func (m *Manager) findTriggerKeywordConflict(keyword string) (TriggerKeywordConflict, bool) {
	conflicts := m.findTriggerKeywordConflicts(keyword)
	if len(conflicts) == 0 {
		return TriggerKeywordConflict{}, false
	}
	return conflicts[0], true
}

func (m *Manager) findTriggerKeywordConflicts(targetKeyword string) []TriggerKeywordConflict {
	ownersByKeyword := map[string][]*Instance{}

	for _, pluginInstance := range m.instances {
		if pluginInstance == nil || pluginInstance.Setting == nil || pluginInstance.Setting.Disabled.Get() {
			continue
		}

		seenInPlugin := map[string]struct{}{}
		for _, triggerKeyword := range pluginInstance.GetTriggerKeywords() {
			keyword := strings.TrimSpace(triggerKeyword)
			// Global "*" triggers intentionally overlap. Only concrete trigger keywords
			// can make an input like "color " ambiguous, so Doctor and query dispatch ignore "*".
			if keyword == "" || keyword == "*" {
				continue
			}
			if targetKeyword != "" && keyword != targetKeyword {
				continue
			}
			if _, exists := seenInPlugin[keyword]; exists {
				continue
			}
			seenInPlugin[keyword] = struct{}{}
			ownersByKeyword[keyword] = append(ownersByKeyword[keyword], pluginInstance)
		}
	}

	conflicts := make([]TriggerKeywordConflict, 0)
	for keyword, pluginInstances := range ownersByKeyword {
		if len(pluginInstances) <= 1 {
			continue
		}
		conflicts = append(conflicts, TriggerKeywordConflict{
			Keyword:         keyword,
			PluginInstances: pluginInstances,
		})
	}

	sort.Slice(conflicts, func(i, j int) bool {
		return conflicts[i].Keyword < conflicts[j].Keyword
	})
	return conflicts
}

func formatTriggerKeywordConflictDetails(ctx context.Context, conflicts []TriggerKeywordConflict) string {
	items := make([]string, 0, len(conflicts))
	for _, conflict := range conflicts {
		items = append(items, fmt.Sprintf("%s: %s", conflict.Keyword, strings.Join(triggerKeywordConflictPluginNames(ctx, conflict.PluginInstances), ", ")))
	}
	return strings.Join(items, "; ")
}

func triggerKeywordConflictPluginNames(ctx context.Context, pluginInstances []*Instance) []string {
	names := make([]string, 0, len(pluginInstances))
	for _, pluginInstance := range pluginInstances {
		if pluginInstance == nil {
			continue
		}
		names = append(names, pluginInstance.GetName(ctx))
	}
	return names
}

func (m *Manager) buildTriggerKeywordConflictResponse(ctx context.Context, query Query, conflict TriggerKeywordConflict) QueryResponseUI {
	ownerPlugin := conflict.PluginInstances[0]
	pluginNames := triggerKeywordConflictPluginNames(ctx, conflict.PluginInstances)
	pluginNamesText := strings.Join(pluginNames, ", ")

	title := fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_manager_trigger_keyword_conflict_title"), conflict.Keyword)
	subtitle := fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_manager_trigger_keyword_conflict_subtitle"), pluginNamesText)

	previewPlugins := make([]triggerKeywordConflictPreviewPlugin, 0, len(conflict.PluginInstances))
	for _, pluginInstance := range conflict.PluginInstances {
		icon := pluginInstance.Metadata.GetIconOrDefault(pluginInstance.PluginDirectory, common.WoxIcon)
		previewPlugins = append(previewPlugins, triggerKeywordConflictPreviewPlugin{
			PluginId:        pluginInstance.Metadata.Id,
			PluginName:      pluginInstance.GetName(ctx),
			Icon:            common.ConvertIcon(ctx, icon, pluginInstance.PluginDirectory),
			TriggerKeywords: append([]string{}, pluginInstance.GetTriggerKeywords()...),
		})
	}
	previewData, marshalErr := json.Marshal(triggerKeywordConflictPreviewData{
		Keyword: conflict.Keyword,
		Title:   title,
		Message: subtitle,
		Plugins: previewPlugins,
	})
	if marshalErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to marshal trigger keyword conflict preview: %s", marshalErr.Error()))
		previewData = []byte(subtitle)
	}

	actions := make([]QueryResultAction, 0, len(conflict.PluginInstances))
	for _, pluginInstance := range conflict.PluginInstances {
		actions = append(actions, m.newTriggerKeywordConflictOpenPluginSettingAction(ctx, pluginInstance))
	}

	// Query-time blocking prevents two plugins from handling the same concrete trigger.
	// Returning a core-owned warning row is clearer than letting whichever plugin responds
	// first define the visible state while the other plugin also runs in the background.
	result := m.PolishResult(ctx, ownerPlugin, query, QueryResult{
		Title:    title,
		SubTitle: subtitle,
		Icon:     common.NewWoxImageEmoji("⚠️"),
		Preview: WoxPreview{
			PreviewType: WoxPreviewTypeTriggerKeywordConflict,
			PreviewData: string(previewData),
		},
		Actions: actions,
		Score:   1000000,
	})

	response := QueryResponse{
		Results: []QueryResult{result},
		Context: BuildQueryContext(query, nil),
	}
	responseUI := response.ToUI()
	for i := range responseUI.Results {
		responseUI.Results[i].QueryId = query.Id
	}
	return responseUI
}

func (m *Manager) BuildTriggerKeywordConflictResponse(ctx context.Context, query Query, conflict TriggerKeywordConflict) QueryResponseUI {
	// Error callers bypass the normal query pipeline, so create the result cache
	// here before polishing. That keeps result actions executable while still
	// avoiding plugin dispatch for the ambiguous trigger.
	m.startSessionQueryCache(query)
	return m.buildTriggerKeywordConflictResponse(ctx, query, conflict)
}

func (m *Manager) newTriggerKeywordConflictOpenPluginSettingAction(ctx context.Context, pluginInstance *Instance) QueryResultAction {
	return QueryResultAction{
		Id:                     fmt.Sprintf("%s_%s", systemActionOpenPluginSettingID, pluginInstance.Metadata.Id),
		Name:                   fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_sys_open_plugin_settings"), pluginInstance.GetName(ctx)),
		Icon:                   common.SettingIcon,
		IsSystemAction:         true,
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext ActionContext) {
			m.ui.OpenSettingWindow(ctx, common.SettingWindowContext{
				Path:  "/plugin/setting",
				Param: pluginInstance.Metadata.Id,
			})
		},
	}
}

func (m *Manager) newTriggerKeywordConflictErrorIfNeeded(ctx context.Context, query Query) error {
	if query.TriggerKeyword == "" {
		return nil
	}

	conflict, found := m.findTriggerKeywordConflict(query.TriggerKeyword)
	if !found {
		return nil
	}

	// Trigger-keyword conflicts are parse-time errors. Returning a typed error from
	// NewQuery keeps lifecycle and dispatch from temporarily selecting the first
	// matching plugin as the owner of an ambiguous query.
	logger.Warn(ctx, fmt.Sprintf("trigger keyword conflict detected during query parse: keyword=%s plugins=%s", conflict.Keyword, formatTriggerKeywordConflictDetails(ctx, []TriggerKeywordConflict{conflict})))
	return &TriggerKeywordConflictError{Conflict: conflict}
}
