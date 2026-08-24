package notes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"wox/common"
	"wox/database"
	"wox/plugin"
	"wox/setting"
	"wox/util"
	"wox/util/clipboard"
)

const trashRetention = 60 * 24 * time.Hour

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &Plugin{})
}

// Plugin exposes the built-in Notes repository through Wox query and deep-link flows.
type Plugin struct {
	api        plugin.API
	repository *Repository
}

func (p *Plugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            common.NotesPluginID,
		Name:          "i18n:plugin_notes_name",
		Author:        "Wox Launcher",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_notes_description",
		Icon:          common.PluginNotesIcon.String(),
		TriggerKeywords: []string{
			"notes", "note", "笔记", "便签",
		},
		Commands: []plugin.MetadataCommand{
			{Command: "new", Description: "i18n:plugin_notes_command_new"},
			{Command: "search", Description: "i18n:plugin_notes_command_search"},
			{Command: "deleted", Description: "i18n:plugin_notes_command_deleted"},
		},
		Features: []plugin.MetadataFeature{
			{Name: plugin.MetadataFeatureDeepLink},
			{Name: plugin.MetadataFeatureIgnoreAutoScore},
		},
		SupportedOS: []string{"Windows", "Macos", "Linux"},
		I18n: map[string]map[string]string{
			"en_US": notesEnglishTranslations,
			"zh_CN": notesChineseTranslations,
		},
	}
}

func (p *Plugin) Init(ctx context.Context, initParams plugin.InitParams) {
	p.api = initParams.API
	p.repository = NewRepository(setting.NewPluginSettingStore(database.GetDB(), common.NotesPluginID))
	p.repository.Observe(func(id string) {
		if ui := plugin.GetPluginManager().GetUI(); ui != nil {
			ui.RefreshNotesWindow(util.NewTraceContext(), id)
		}
	})
	p.api.OnDeepLink(ctx, p.handleDeepLink)
	p.api.OnSettingChanged(ctx, func(callbackCtx context.Context, key string, _ string) {
		if strings.HasPrefix(key, noteSettingPrefix) {
			p.repository.ExternalChanged(strings.TrimPrefix(key, noteSettingPrefix))
		}
	})
	p.purgeExpired(ctx)
	util.Go(ctx, "purge expired Notes trash", func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.purgeExpired(util.NewTraceContext())
			}
		}
	})
}

func (p *Plugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	command := strings.ToLower(strings.TrimSpace(query.Command))
	search := strings.TrimSpace(query.Search)
	switch command {
	case "new":
		return plugin.NewQueryResponse([]plugin.QueryResult{p.newResult()})
	case "deleted":
		return plugin.NewQueryResponse(p.noteResults(ctx, search, true))
	case "search":
		return plugin.NewQueryResponse(p.noteResults(ctx, search, false))
	}
	return plugin.NewQueryResponse(p.noteResults(ctx, search, false))
}

func (p *Plugin) newResult() plugin.QueryResult {
	return plugin.QueryResult{
		Id: "notes:new", Title: "i18n:plugin_notes_new", SubTitle: "i18n:plugin_notes_new_subtitle", Icon: common.UIIcon("control.add"), Score: 1_000_000,
		Actions: []plugin.QueryResultAction{{Id: "new", Name: "i18n:plugin_notes_action_new", IsDefault: true, Icon: common.UIIcon("control.add"), Action: func(ctx context.Context, _ plugin.ActionContext) {
			p.createAndOpen(ctx)
		}}},
	}
}

func (p *Plugin) noteResults(ctx context.Context, search string, deleted bool) []plugin.QueryResult {
	records, err := p.repository.List(true)
	if err != nil {
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to list notes: %v", err))
		return nil
	}
	results := make([]plugin.QueryResult, 0, len(records))
	for _, record := range records {
		if (record.DeletedAt > 0) != deleted {
			continue
		}
		title, preview := NoteTitle(record.Document), NotePreview(record.Document)
		score := int64(1)
		if search != "" {
			titleMatch, titleScore := plugin.IsStringMatchScore(ctx, title, search)
			bodyMatch, bodyScore := plugin.IsStringMatchScore(ctx, ToPlainText(record.Document), search)
			if !titleMatch && !bodyMatch {
				continue
			}
			score = max(titleScore*2, bodyScore)
		} else {
			score = record.UpdatedAt
		}
		results = append(results, plugin.QueryResult{
			Id: record.ID, Title: title, SubTitle: preview, Icon: common.PluginNotesIcon, Score: score, ScoreKey: "note:" + record.ID,
			Actions: p.noteActions(record),
		})
	}
	if len(results) == 0 {
		results = append(results, plugin.QueryResult{Title: "i18n:plugin_notes_no_results", SubTitle: "i18n:plugin_notes_no_results_subtitle", Icon: common.UIIcon("control.search")})
	}
	return results
}

func (p *Plugin) noteActions(record common.NoteRecord) []plugin.QueryResultAction {
	if record.DeletedAt > 0 {
		return []plugin.QueryResultAction{{Id: "restore", Name: "i18n:plugin_notes_action_restore", IsDefault: true, Icon: common.UIIcon("control.undo"), Action: func(ctx context.Context, _ plugin.ActionContext) {
			if _, err := p.repository.Restore(record.ID); err != nil {
				p.notifyError(ctx, err)
				return
			}
			p.openWindow(ctx, common.NotesWindowRequest{Action: common.NotesWindowOpen, NoteID: record.ID})
		}}}
	}
	pinName, pinIcon := "i18n:plugin_notes_action_pin", common.PinIcon
	if record.PinnedAt > 0 {
		pinName, pinIcon = "i18n:plugin_notes_action_unpin", common.UnpinIcon
	}
	return []plugin.QueryResultAction{
		{Id: "open", Name: "i18n:plugin_notes_action_open", IsDefault: true, Icon: common.PluginNotesIcon, Action: func(ctx context.Context, _ plugin.ActionContext) {
			p.openWindow(ctx, common.NotesWindowRequest{Action: common.NotesWindowOpen, NoteID: record.ID})
		}},
		{Id: "pin", Name: pinName, Icon: pinIcon, PreventHideAfterAction: true, Action: func(ctx context.Context, _ plugin.ActionContext) {
			if _, err := p.repository.SetPinned(record.ID, record.PinnedAt == 0); err != nil {
				p.notifyError(ctx, err)
			}
			p.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
		}},
		{Id: "copy-link", Name: "i18n:plugin_notes_action_copy_link", Icon: common.UIIcon("control.copy"), Action: func(ctx context.Context, _ plugin.ActionContext) {
			if err := clipboard.WriteText(noteDeepLink(record.ID)); err != nil {
				p.notifyError(ctx, err)
			}
		}},
		{Id: "export-markdown", Name: "i18n:plugin_notes_action_export_markdown", Icon: common.UIIcon("control.download"), Action: func(ctx context.Context, _ plugin.ActionContext) {
			p.openWindow(ctx, common.NotesWindowRequest{Action: common.NotesWindowOpen, NoteID: record.ID, ExportFormat: "md"})
		}},
		{Id: "export-text", Name: "i18n:plugin_notes_action_export_text", Icon: common.UIIcon("control.download"), Action: func(ctx context.Context, _ plugin.ActionContext) {
			p.openWindow(ctx, common.NotesWindowRequest{Action: common.NotesWindowOpen, NoteID: record.ID, ExportFormat: "txt"})
		}},
		{Id: "export-html", Name: "i18n:plugin_notes_action_export_html", Icon: common.UIIcon("control.download"), Action: func(ctx context.Context, _ plugin.ActionContext) {
			p.openWindow(ctx, common.NotesWindowRequest{Action: common.NotesWindowOpen, NoteID: record.ID, ExportFormat: "html"})
		}},
		{Id: "delete", Name: "i18n:plugin_notes_action_delete", Icon: common.UIIcon("control.delete"), PreventHideAfterAction: true, Action: func(ctx context.Context, _ plugin.ActionContext) {
			if _, err := p.repository.Delete(record.ID); err != nil {
				p.notifyError(ctx, err)
			}
			p.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
		}},
	}
}

func (p *Plugin) createAndOpen(ctx context.Context) {
	record, err := p.repository.Create()
	if err != nil {
		p.notifyError(ctx, err)
		return
	}
	p.openWindow(ctx, common.NotesWindowRequest{Action: common.NotesWindowOpen, NoteID: record.ID})
}

func (p *Plugin) handleDeepLink(ctx context.Context, arguments map[string]string) {
	switch strings.ToLower(strings.TrimSpace(arguments["action"])) {
	case "new":
		p.createAndOpen(ctx)
	case "open":
		p.openWindow(ctx, common.NotesWindowRequest{Action: common.NotesWindowOpen, NoteID: strings.TrimSpace(arguments["id"])})
	default:
		p.openWindow(ctx, common.NotesWindowRequest{Action: common.NotesWindowToggle})
	}
}

func (p *Plugin) openWindow(ctx context.Context, request common.NotesWindowRequest) {
	if ui := plugin.GetPluginManager().GetUI(); ui != nil {
		ui.OpenNotesWindow(ctx, request)
	}
}

func (p *Plugin) purgeExpired(ctx context.Context) {
	if _, err := p.repository.PurgeDeletedBefore(time.Now().Add(-trashRetention)); err != nil {
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to purge Notes trash: %v", err))
	}
}

func (p *Plugin) notifyError(ctx context.Context, err error) {
	p.api.Log(ctx, plugin.LogLevelError, err.Error())
	p.api.Notify(ctx, err.Error())
}

func noteDeepLink(id string) string {
	return fmt.Sprintf("wox://plugin/%s?action=open&id=%s", common.NotesPluginID, id)
}

// NotesList returns lightweight summaries for the native browser overlay.
func (p *Plugin) NotesList(ctx context.Context, search string, includeDeleted bool) ([]common.NoteSummary, error) {
	records, err := p.repository.List(includeDeleted)
	if err != nil {
		return nil, err
	}
	summaries := make([]common.NoteSummary, 0, len(records))
	for _, record := range records {
		if !includeDeleted && record.DeletedAt > 0 {
			continue
		}
		title, preview := NoteTitle(record.Document), NotePreview(record.Document)
		if search != "" {
			titleMatch, _ := plugin.IsStringMatchScore(ctx, title, search)
			bodyMatch, _ := plugin.IsStringMatchScore(ctx, ToPlainText(record.Document), search)
			if !titleMatch && !bodyMatch {
				continue
			}
		}
		summaries = append(summaries, common.NoteSummary{
			ID: record.ID, Title: title, Preview: preview, UpdatedAt: record.UpdatedAt,
			PinnedAt: record.PinnedAt, DeletedAt: record.DeletedAt,
		})
	}
	return summaries, nil
}

func (p *Plugin) NotesGet(_ context.Context, id string) (common.NoteRecord, error) {
	return p.repository.Get(id)
}

func (p *Plugin) NotesCreate(_ context.Context) (common.NoteRecord, error) {
	return p.repository.Create()
}

func (p *Plugin) NotesSave(_ context.Context, id, expectedRevision string, document common.NoteDocument) (common.NoteSaveResult, error) {
	record, conflict, err := p.repository.Save(id, expectedRevision, document)
	return common.NoteSaveResult{Record: record, Conflict: conflict}, err
}

func (p *Plugin) NotesSetPinned(_ context.Context, id string, pinned bool) (common.NoteRecord, error) {
	return p.repository.SetPinned(id, pinned)
}

func (p *Plugin) NotesDelete(_ context.Context, id string) (common.NoteRecord, error) {
	return p.repository.Delete(id)
}

func (p *Plugin) NotesRestore(_ context.Context, id string) (common.NoteRecord, error) {
	return p.repository.Restore(id)
}

func (p *Plugin) NotesExport(_ context.Context, id, format string) (common.NoteExport, error) {
	record, err := p.repository.Get(id)
	if err != nil {
		return common.NoteExport{}, err
	}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "txt", "text":
		return common.NoteExport{Content: ToPlainText(record.Document), Extension: "txt"}, nil
	case "html":
		return common.NoteExport{Content: ToHTML(record.Document), Extension: "html"}, nil
	case "md", "markdown", "":
		return common.NoteExport{Content: ToMarkdown(record.Document), Extension: "md"}, nil
	default:
		return common.NoteExport{}, fmt.Errorf("unsupported Notes export format %q", format)
	}
}

func (p *Plugin) NotesGetLocal(_ context.Context, key string) (string, error) {
	return p.repository.GetLocal(key), nil
}

func (p *Plugin) NotesSetLocal(_ context.Context, key, value string) error {
	return p.repository.SetLocal(key, value)
}

var notesEnglishTranslations = map[string]string{
	"plugin_notes_name": "Notes", "plugin_notes_description": "Fast floating notes with rich text and Cloud Sync",
	"plugin_notes_command_new": "Create a new note", "plugin_notes_command_search": "Search notes", "plugin_notes_command_deleted": "Recently deleted notes",
	"plugin_notes_new": "New Note", "plugin_notes_new_subtitle": "Create and open an empty note",
	"plugin_notes_no_results": "No notes found", "plugin_notes_no_results_subtitle": "Try another search or create a new note",
	"plugin_notes_action_new": "New Note", "plugin_notes_action_open": "Open Note", "plugin_notes_action_pin": "Pin Note", "plugin_notes_action_unpin": "Unpin Note",
	"plugin_notes_action_copy_link": "Copy Note Link", "plugin_notes_action_export_markdown": "Export as Markdown", "plugin_notes_action_export_text": "Export as Plain Text", "plugin_notes_action_export_html": "Export as HTML", "plugin_notes_action_delete": "Delete Note", "plugin_notes_action_restore": "Restore Note",
}

var notesChineseTranslations = map[string]string{
	"plugin_notes_name": "笔记", "plugin_notes_description": "支持富文本和云同步的轻量浮动笔记",
	"plugin_notes_command_new": "新建笔记", "plugin_notes_command_search": "搜索笔记", "plugin_notes_command_deleted": "最近删除的笔记",
	"plugin_notes_new": "新建笔记", "plugin_notes_new_subtitle": "创建并打开一篇空白笔记",
	"plugin_notes_no_results": "没有找到笔记", "plugin_notes_no_results_subtitle": "尝试其他关键词或新建一篇笔记",
	"plugin_notes_action_new": "新建笔记", "plugin_notes_action_open": "打开笔记", "plugin_notes_action_pin": "置顶笔记", "plugin_notes_action_unpin": "取消置顶",
	"plugin_notes_action_copy_link": "复制笔记链接", "plugin_notes_action_export_markdown": "导出为 Markdown", "plugin_notes_action_export_text": "导出为纯文本", "plugin_notes_action_export_html": "导出为 HTML", "plugin_notes_action_delete": "删除笔记", "plugin_notes_action_restore": "恢复笔记",
}
