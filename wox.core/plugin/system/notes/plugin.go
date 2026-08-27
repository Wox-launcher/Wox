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

const (
	trashRetention     = 60 * 24 * time.Hour
	notesMRUContextKey = "noteId"
	notesMRUNewID      = "new"

	PluginCommandCreateNote = "create_note"
	PluginCommandDataText   = "text"
	PluginCommandDataPath   = "path"
	PluginCommandDataTitle  = "title"
	createNoteMaxFileBytes  = 256 * 1024
)

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
		Name:          "i18n:plugin_notes_plugin_name",
		Author:        "Wox Launcher",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_notes_plugin_description",
		Icon:          common.PluginNotesIcon.String(),
		TriggerKeywords: []string{
			"note",
		},
		Commands: []plugin.MetadataCommand{
			{Command: "new", Description: "i18n:plugin_notes_command_new"},
			{Command: "search", Description: "i18n:plugin_notes_command_search"},
			{Command: "deleted", Description: "i18n:plugin_notes_command_deleted"},
		},
		Features: []plugin.MetadataFeature{
			{Name: plugin.MetadataFeatureDeepLink},
			{Name: plugin.MetadataFeatureIgnoreAutoScore},
			{Name: plugin.MetadataFeatureMRU, Params: map[string]any{"HashBy": "scoreKey"}},
		},
		SupportedOS: []string{"Windows", "Macos", "Linux"},
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
	p.api.OnMRURestore(ctx, p.handleMRURestore)
	p.api.OnHandlePluginCommand(ctx, p.handlePluginCommand)
	p.api.OnSettingChanged(ctx, func(callbackCtx context.Context, key string, _ string) {
		if strings.HasPrefix(key, noteSettingPrefix) {
			p.repository.ExternalChanged(strings.TrimPrefix(key, noteSettingPrefix))
		}
	})
	p.purgeEmpty(ctx)
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
		Id: "notes:new", Title: "i18n:plugin_notes_new", SubTitle: "i18n:plugin_notes_new_subtitle", Icon: common.PluginNotesIcon, Score: 1_000_000, ScoreKey: "note:new",
		Actions: []plugin.QueryResultAction{{Id: "new", Name: "i18n:plugin_notes_action_new", IsDefault: true, Icon: common.PluginNotesIcon, ContextData: noteActionContext(notesMRUNewID), Action: func(ctx context.Context, _ plugin.ActionContext) {
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
		if (record.DeletedAt > 0) != deleted || DocumentIsEmpty(record.Document) {
			continue
		}
		title := NoteTitle(record.Document)
		score := int64(1)
		if search != "" {
			titleMatch, titleScore := plugin.IsStringMatchScore(ctx, title, search)
			bodyMatch, bodyScore := plugin.IsStringMatchScore(ctx, ToPlainText(record.Document), search)
			if !titleMatch && !bodyMatch {
				continue
			}
			score = max(titleScore*2, bodyScore)
		} else if record.PinnedAt > 0 {
			score = record.PinnedAt
		} else {
			score = record.UpdatedAt
		}
		result := p.noteResult(record)
		result.Score = score
		results = append(results, result)
	}
	if len(results) == 0 {
		results = append(results, plugin.QueryResult{Title: "i18n:plugin_notes_no_results", SubTitle: "i18n:plugin_notes_no_results_subtitle", Icon: common.SearchIcon})
	}
	return results
}

// noteResult builds the shared launcher row used by query results and MRU restore.
func (p *Plugin) noteResult(record common.NoteRecord) plugin.QueryResult {
	group, groupScore := noteResultGroup(record)
	return plugin.QueryResult{
		Id: record.ID, Title: NoteTitle(record.Document), Icon: common.PluginNotesIcon, ScoreKey: "note:" + record.ID,
		Group: group, GroupScore: groupScore,
		Tails:   []plugin.QueryResultTail{plugin.NewQueryResultTailText(util.FormatTimestamp(record.UpdatedAt))},
		Actions: p.noteActions(record),
	}
}

func (p *Plugin) noteActions(record common.NoteRecord) []plugin.QueryResultAction {
	contextData := noteActionContext(record.ID)
	if record.DeletedAt > 0 {
		return []plugin.QueryResultAction{{Id: "restore", Name: "i18n:plugin_notes_action_restore", IsDefault: true, Icon: common.UpdateIcon, ContextData: contextData, Action: func(ctx context.Context, _ plugin.ActionContext) {
			if _, err := p.repository.Restore(record.ID); err != nil {
				p.notifyError(ctx, err)
				return
			}
			p.openWindow(ctx, common.NotesWindowRequest{Action: common.NotesWindowOpen, NoteID: record.ID})
		}}}
	}
	// Plugin actions are not tinted by the action panel, so use colored icons
	// instead of monochrome UIIcon glyphs that disappear on the panel.
	pinName, pinIcon := "i18n:plugin_notes_action_pin", common.PinIcon
	if record.PinnedAt > 0 {
		pinName, pinIcon = "i18n:plugin_notes_action_unpin", common.UnpinIcon
	}
	return []plugin.QueryResultAction{
		{Id: "open", Name: "i18n:plugin_notes_action_open", IsDefault: true, Icon: common.OpenIcon, ContextData: contextData, Action: func(ctx context.Context, _ plugin.ActionContext) {
			p.openWindow(ctx, common.NotesWindowRequest{Action: common.NotesWindowOpen, NoteID: record.ID})
		}},
		{Id: "pin", Name: pinName, Icon: pinIcon, PreventHideAfterAction: true, ContextData: contextData, Action: func(ctx context.Context, _ plugin.ActionContext) {
			if _, err := p.repository.SetPinned(record.ID, record.PinnedAt == 0); err != nil {
				p.notifyError(ctx, err)
			}
			p.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
		}},
		{Id: "copy-link", Name: "i18n:plugin_notes_action_copy_link", Icon: common.CopyIcon, ContextData: contextData, Action: func(ctx context.Context, _ plugin.ActionContext) {
			if err := clipboard.WriteText(noteDeepLink(record.ID)); err != nil {
				p.notifyError(ctx, err)
			}
		}},
		{Id: "export-markdown", Name: "i18n:plugin_notes_action_export_markdown", Icon: common.InstallIcon, ContextData: contextData, Action: func(ctx context.Context, _ plugin.ActionContext) {
			p.openWindow(ctx, common.NotesWindowRequest{Action: common.NotesWindowOpen, NoteID: record.ID, ExportFormat: "md"})
		}},
		{Id: "export-text", Name: "i18n:plugin_notes_action_export_text", Icon: common.TextIcon, ContextData: contextData, Action: func(ctx context.Context, _ plugin.ActionContext) {
			p.openWindow(ctx, common.NotesWindowRequest{Action: common.NotesWindowOpen, NoteID: record.ID, ExportFormat: "txt"})
		}},
		{Id: "export-html", Name: "i18n:plugin_notes_action_export_html", Icon: common.InstallIcon, ContextData: contextData, Action: func(ctx context.Context, _ plugin.ActionContext) {
			p.openWindow(ctx, common.NotesWindowRequest{Action: common.NotesWindowOpen, NoteID: record.ID, ExportFormat: "html"})
		}},
		{Id: "delete", Name: "i18n:plugin_notes_action_delete", Icon: common.TrashIcon, PreventHideAfterAction: true, ContextData: contextData, Action: func(ctx context.Context, _ plugin.ActionContext) {
			if _, err := p.repository.Delete(record.ID); err != nil {
				p.notifyError(ctx, err)
			}
			p.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
		}},
	}
}

// noteResultGroup mirrors clipboard: pinned notes first, then today / yesterday / history.
func noteResultGroup(record common.NoteRecord) (string, int64) {
	if record.DeletedAt == 0 && record.PinnedAt > 0 {
		return "i18n:plugin_notes_group_pinned", 100
	}
	elapsed := util.GetSystemTimestamp() - record.UpdatedAt
	if elapsed < 1000*60*60*24 {
		return "i18n:plugin_notes_group_today", 90
	}
	if elapsed < 1000*60*60*24*2 {
		return "i18n:plugin_notes_group_yesterday", 80
	}
	return "i18n:plugin_notes_group_history", 10
}

// createAndOpen opens a new draft window. The draft is not persisted until the user types.
func (p *Plugin) createAndOpen(ctx context.Context) {
	p.openWindow(ctx, common.NotesWindowRequest{Action: common.NotesWindowNew})
}

// handlePluginCommand creates a note from another plugin's text, file, or image path.
func (p *Plugin) handlePluginCommand(ctx context.Context, request plugin.PluginCommandRequest) plugin.PluginCommandResult {
	if request.Command != PluginCommandCreateNote {
		return plugin.PluginCommandResult{Handled: false}
	}

	title := strings.TrimSpace(request.Data[PluginCommandDataTitle])
	text := request.Data[PluginCommandDataText]
	path := strings.TrimSpace(request.Data[PluginCommandDataPath])
	document, err := documentFromPluginCommand(title, text, path)
	if err != nil {
		return plugin.PluginCommandResult{Handled: true, Message: err.Error()}
	}
	if DocumentIsEmpty(document) {
		return plugin.PluginCommandResult{Handled: true, Message: "note content is empty"}
	}

	record, err := p.repository.Create()
	if err != nil {
		return plugin.PluginCommandResult{Handled: true, Message: err.Error()}
	}
	saved, _, err := p.repository.Save(record.ID, record.Revision, document)
	if err != nil {
		return plugin.PluginCommandResult{Handled: true, Message: err.Error()}
	}

	p.openWindow(ctx, common.NotesWindowRequest{Action: common.NotesWindowOpen, NoteID: saved.ID})
	return plugin.PluginCommandResult{Handled: true}
}

// CreateNoteAction asks Notes to persist and open a note built from the caller's context.
func CreateNoteAction(api plugin.API, title string, text string, path string) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Name: "i18n:plugin_notes_action_save",
		Icon: common.PluginNotesIcon,
		Action: func(ctx context.Context, _ plugin.ActionContext) {
			plugin.InvokePluginCommandAndNotify(ctx, api, plugin.PluginCommandRequest{
				PluginId: common.NotesPluginID,
				Command:  PluginCommandCreateNote,
				Data: common.ContextData{
					PluginCommandDataTitle: title,
					PluginCommandDataText:  text,
					PluginCommandDataPath:  path,
				},
			})
		},
	}
}

// handleMRURestore rebuilds a homepage result from the note id recorded by a previous action.
func (p *Plugin) handleMRURestore(ctx context.Context, mruData plugin.MRUData) (*plugin.QueryResult, error) {
	noteID := strings.TrimSpace(mruData.ContextData[notesMRUContextKey])
	if noteID == "" {
		return nil, fmt.Errorf("empty note id in context data")
	}
	if noteID == notesMRUNewID {
		result := p.newResult()
		return &result, nil
	}
	record, err := p.repository.Get(noteID)
	if err != nil {
		return nil, err
	}
	if record.DeletedAt > 0 || DocumentIsEmpty(record.Document) {
		return nil, fmt.Errorf("note is no longer available: %s", noteID)
	}
	result := p.noteResult(record)
	result.Group = ""
	result.GroupScore = 0
	return &result, nil
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

// purgeEmpty removes leftover untitled notes that were persisted without content.
func (p *Plugin) purgeEmpty(ctx context.Context) {
	if _, err := p.repository.PurgeEmpty(); err != nil {
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to purge empty notes: %v", err))
	}
}

func (p *Plugin) notifyError(ctx context.Context, err error) {
	p.api.Log(ctx, plugin.LogLevelError, err.Error())
	p.api.Notify(ctx, err.Error())
}

func noteDeepLink(id string) string {
	return fmt.Sprintf("wox://plugin/%s?action=open&id=%s", common.NotesPluginID, id)
}

func noteActionContext(id string) common.ContextData {
	return common.ContextData{notesMRUContextKey: id}
}

// NotesList returns lightweight summaries for the native browser overlay.
func (p *Plugin) NotesList(ctx context.Context, search string, includeDeleted bool) ([]common.NoteSummary, error) {
	records, err := p.repository.List(includeDeleted)
	if err != nil {
		return nil, err
	}
	summaries := make([]common.NoteSummary, 0, len(records))
	for _, record := range records {
		if (!includeDeleted && record.DeletedAt > 0) || DocumentIsEmpty(record.Document) {
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

// NotesDiscard permanently removes a note so empty drafts never enter trash.
func (p *Plugin) NotesDiscard(_ context.Context, id string) error {
	return p.repository.Discard(id)
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
