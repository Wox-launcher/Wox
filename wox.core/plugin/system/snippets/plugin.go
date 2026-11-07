package snippets

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"wox/common/icons"
	"wox/database"
	"wox/i18n"
	"wox/plugin"
	"wox/plugin/system"
	"wox/plugin/system/migrateapply"
	"wox/setting"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/util"
	"wox/util/clipboard"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &Plugin{})
	migrateapply.ImportSnippets = importMigratedSnippets
}

// Plugin exposes the synchronized snippets table in the launcher.
type Plugin struct {
	api         plugin.API
	repository  *Repository
	expansionMu sync.Mutex
	expansion   *expansion
	unavailable expansionIssue
	closed      bool
}

func (p *Plugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id: pluginID, Name: "i18n:plugin_snippets_plugin_name", Description: "i18n:plugin_snippets_plugin_description",
		Author: "Wox Launcher", Version: "1.0.0", MinWoxVersion: "2.0.0", Runtime: "Go",
		Icon: icons.Get(icons.PluginSnippets).String(), TriggerKeywords: []string{"snip"},
		SupportedOS: []string{"Windows", "Macos", "Linux"},
		SettingDefinitions: definition.PluginSettingDefinitions{
			{Type: definition.PluginSettingDefinitionTypeCheckBox, Value: &definition.PluginSettingValueCheckBox{
				Key: autoExpandSetting, Label: "i18n:plugin_snippets_enable", Tooltip: "i18n:plugin_snippets_enable_help", DefaultValue: "false",
			}},
			{Type: definition.PluginSettingDefinitionTypeDynamic, Value: &definition.PluginSettingValueDynamic{Key: expansionStatusSetting}},
			{Type: definition.PluginSettingDefinitionTypeDynamic, Value: &definition.PluginSettingValueDynamic{Key: conflictStatusSetting}},
			{Type: definition.PluginSettingDefinitionTypeTable, Value: &definition.PluginSettingValueTable{
				Key: snippetsSetting, Title: "i18n:plugin_snippets_plugin_name", InlineTable: true, EnableSearch: true, SearchColumnKey: "name",
				Columns: []definition.PluginSettingValueTableColumn{
					{Key: "name", Label: "i18n:plugin_snippets_name", Type: definition.PluginSettingValueTableColumnTypeText, Width: 140,
						Validators: []validator.PluginSettingValidator{{Type: validator.PluginSettingValidatorTypeNotEmpty, Value: &validator.PluginSettingValidatorNotEmpty{}}}},
					{Key: "keyword", Label: "i18n:plugin_snippets_keyword", Tooltip: "i18n:plugin_snippets_keyword_tooltip", Type: definition.PluginSettingValueTableColumnTypeText, Width: 120},
					{Key: "text", Label: "i18n:plugin_snippets_text", Tooltip: "i18n:plugin_snippets_text_tooltip", Type: definition.PluginSettingValueTableColumnTypeText, TextMaxLines: 8, VariableOptions: []definition.PluginSettingVariableOption{
						{Value: "{wox:date}", Label: "i18n:plugin_snippets_variable_date", Description: "i18n:plugin_snippets_variable_date_help", Icon: "clock"},
						{Value: "{wox:date?format=yyyy-MM-dd}", Label: "i18n:plugin_snippets_variable_date_format", Description: "i18n:plugin_snippets_variable_date_format_help", Icon: "clock"},
						{Value: "{wox:clipboard_text}", Label: "i18n:plugin_snippets_variable_clipboard", Description: "i18n:plugin_snippets_variable_clipboard_help", Icon: "copy"},
						{Value: "{wox:uuid}", Label: "i18n:plugin_snippets_variable_uuid", Description: "i18n:plugin_snippets_variable_uuid_help", Icon: "code"},
					}},
				},
			}},
		},
	}
}

func (p *Plugin) Init(ctx context.Context, params plugin.InitParams) {
	p.expansionMu.Lock()
	p.api = params.API
	p.repository = NewRepository(setting.NewPluginSettingStore(database.GetDB(), pluginID))
	p.closed = false
	p.expansionMu.Unlock()
	p.api.OnSettingChanged(ctx, p.onSettingChanged)
	p.api.OnUnload(ctx, func(context.Context) { p.stopExpansion() })
	p.api.OnGetDynamicSetting(ctx, p.expansionStatus)
	p.reconcileExpansion(ctx)
}

// onSettingChanged refreshes the matching snapshot after local or remote changes.
func (p *Plugin) onSettingChanged(ctx context.Context, key, _ string) {
	switch {
	case key == autoExpandSetting:
		p.reconcileExpansion(ctx)
	case key == snippetsSetting:
		p.refreshExpansion(ctx)
	}
}

func (p *Plugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	records, err := p.repository.List()
	if err != nil {
		p.reportError(ctx, err)
		return plugin.NewQueryResponse(nil)
	}
	results := make([]plugin.QueryResult, 0, len(records))
	search := strings.TrimSpace(query.Search)
	keywordCounts := make(map[string]int, len(records))
	for _, record := range records {
		if record.Keyword != "" {
			keywordCounts[record.Keyword]++
		}
	}
	for _, record := range records {
		if search != "" {
			nameMatch, _ := plugin.IsStringMatchScore(ctx, record.Name, search)
			keywordMatch, _ := plugin.IsStringMatchScore(ctx, record.Keyword, search)
			textMatch, _ := plugin.IsStringMatchScore(ctx, record.Text, search)
			if !nameMatch && !keywordMatch && !textMatch {
				continue
			}
		}
		results = append(results, p.result(ctx, query, record, keywordCounts[record.Keyword] > 1))
	}
	return plugin.NewQueryResponse(results)
}

// result exposes the expanded snippet through copy and paste actions.
func (p *Plugin) result(ctx context.Context, query plugin.Query, record Record, conflict bool) plugin.QueryResult {
	prepare := func() (string, error) { return expandTemplate(record.Text, time.Now(), clipboard.ReadTextRequired) }
	copyAction := plugin.QueryResultAction{
		Id: "copy", Name: "i18n:plugin_snippets_copy", Icon: icons.Get(icons.ActionCopy), IsDefault: true,
		Action: func(ctx context.Context, _ plugin.ActionContext) {
			text, err := prepare()
			if err == nil {
				err = clipboard.WriteText(text)
			}
			if err != nil {
				p.reportError(ctx, err)
			}
		},
	}
	actions := []plugin.QueryResultAction{copyAction}
	paste, err := system.GetPasteToActiveWindowAction(ctx, p.api, query.Env.ActiveWindowTitle, query.Env.ActiveWindowPid, query.Env.ActiveWindowIcon, func(context.Context) error {
		text, err := prepare()
		if err != nil {
			return err
		}
		return clipboard.WriteText(text)
	})
	if err == nil {
		paste.Id = "paste"
		paste.IsDefault = false
		actions = append(actions, paste)
	}
	subtitle := record.Keyword
	if conflict {
		subtitle += " · " + i18n.GetI18nManager().TranslateWox(ctx, "plugin_snippets_keyword_conflict_paused")
	}
	return plugin.QueryResult{
		Id: record.ID, IdentityKey: "snippet:" + record.ID, Title: record.Name, SubTitle: subtitle,
		Icon: icons.Get(icons.PluginSnippets), Score: 1, Actions: actions,
		Preview: plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeText, PreviewData: record.Text},
	}
}

const (
	autoExpandSetting      = "autoExpandLocal"
	snippetsSetting        = "snippetsTable"
	expansionStatusSetting = "expansionStatus"
	conflictStatusSetting  = "conflictStatus"
)

// reconcileExpansion starts or stops monitoring according to the device-only preference.
func (p *Plugin) reconcileExpansion(ctx context.Context) {
	p.expansionMu.Lock()
	defer p.expansionMu.Unlock()
	if p.closed {
		return
	}
	enabled := p.api.GetSetting(ctx, autoExpandSetting) == "true"
	if !enabled {
		if p.expansion != nil {
			p.expansion.close()
			p.expansion = nil
		}
		p.unavailable = expansionIssueNone
		return
	}
	if p.expansion != nil {
		return
	}
	records, err := p.repository.List()
	if err == nil {
		p.expansion, err = startExpansion(util.NewTraceContext(), newExpansionBackend(), records)
	}
	p.unavailable = classifyExpansionIssue(err)
	if err != nil {
		p.api.Log(ctx, plugin.LogLevelWarning, "snippet monitoring unavailable: "+err.Error())
	}
}

// refreshExpansion invalidates in-flight work before replacing the synchronized keyword index.
func (p *Plugin) refreshExpansion(ctx context.Context) {
	p.expansionMu.Lock()
	defer p.expansionMu.Unlock()
	if p.expansion == nil {
		return
	}
	p.expansion.engine.invalidate()
	records, err := p.repository.List()
	if err != nil {
		p.expansion.engine.reload(nil)
		p.api.Log(ctx, plugin.LogLevelError, "could not refresh snippet records")
		return
	}
	p.expansion.engine.reload(records)
}

func (p *Plugin) stopExpansion() {
	p.expansionMu.Lock()
	defer p.expansionMu.Unlock()
	p.closed = true
	if p.expansion != nil {
		p.expansion.close()
		p.expansion = nil
	}
}

// expansionStatus exposes platform errors next to the local monitoring switch.
func (p *Plugin) expansionStatus(_ context.Context, key string) definition.PluginSettingDefinitionItem {
	if key == conflictStatusSetting {
		repository := p.settingsRepository()
		if repository == nil {
			return definition.PluginSettingDefinitionItem{}
		}
		records, err := repository.List()
		if err != nil {
			return definition.PluginSettingDefinitionItem{}
		}
		keywords := make(map[string]bool, len(records))
		for _, record := range records {
			if record.Keyword != "" && keywords[record.Keyword] {
				return definition.PluginSettingDefinitionItem{Type: definition.PluginSettingDefinitionTypeLabel, Value: &definition.PluginSettingValueLabel{Content: "i18n:plugin_snippets_keyword_conflict_paused"}}
			}
			keywords[record.Keyword] = true
		}
		return definition.PluginSettingDefinitionItem{}
	}
	if key != expansionStatusSetting {
		return definition.PluginSettingDefinitionItem{}
	}
	p.expansionMu.Lock()
	issue := p.unavailable
	if p.expansion != nil {
		if backend, ok := p.expansion.backend.(interface{ health() error }); ok {
			issue = classifyExpansionIssue(backend.health())
		}
	}
	p.expansionMu.Unlock()
	if issue == expansionIssueNone {
		return definition.PluginSettingDefinitionItem{}
	}
	return definition.PluginSettingDefinitionItem{Type: definition.PluginSettingDefinitionTypeLabel, Value: &definition.PluginSettingValueLabel{Content: "i18n:plugin_snippets_unavailable_" + string(issue)}}
}

func (p *Plugin) reportError(ctx context.Context, err error) {
	p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("snippets: %v", err))
	key := "plugin_snippets_error"
	if errors.Is(err, ErrKeywordConflict) {
		key = "plugin_snippets_keyword_conflict"
	}
	p.api.ShowToastMsg(ctx, plugin.ShowToastMsgOption{Message: i18n.GetI18nManager().TranslateWox(ctx, key)})
}
