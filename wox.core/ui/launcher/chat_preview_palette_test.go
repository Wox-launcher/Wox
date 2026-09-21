package launcher

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"wox/common/icons"
	"wox/ui/contract"
	woxcomponent "wox/ui/launcher/component"
	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
)

func TestChatHistoryGroupUsesLocalDayBoundaries(t *testing.T) {
	location := time.FixedZone("test", 8*60*60)
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, location)
	if group := chatHistoryGroup(time.Date(2026, 8, 5, 0, 0, 0, 0, location).UnixMilli(), now); group != "today" {
		t.Fatalf("today group = %q", group)
	}
	if group := chatHistoryGroup(time.Date(2026, 8, 4, 23, 59, 0, 0, location).UnixMilli(), now); group != "yesterday" {
		t.Fatalf("yesterday group = %q", group)
	}
	if group := chatHistoryGroup(time.Date(2026, 8, 3, 23, 59, 0, 0, location).UnixMilli(), now); group != "history" {
		t.Fatalf("history group = %q", group)
	}
}

func TestSetChatTextKeepsHistorySidebarOpen(t *testing.T) {
	ai := newAISettingsController(CommonDeps{Translate: func(s string) string { return s }})
	ai.SetModels([]aiModel{{Name: "deepseek-v4-flash", Provider: "deepseek"}})
	ai.SetSkills(nil)
	app := &App{
		aiSettings: ai,
		chatPreview: &chatPreviewState{
			panel:  "history",
			editor: woxui.NewTextEditor(""),
			chat:   chatData{ID: "1", Title: "Suzhou"},
		},
	}

	app.setChatText("/")
	if !app.chatPreview.sidebarOpen {
		t.Fatal("typing / hid the conversation sidebar")
	}
	if app.chatPreview.panel != chatCommandPanel {
		t.Fatalf("panel = %q, want %q", app.chatPreview.panel, chatCommandPanel)
	}

	app.setChatText("hello")
	if app.chatPreview.panel != "history" || !app.chatPreview.sidebarOpen {
		t.Fatalf("clearing / did not restore the sidebar: panel=%q open=%v", app.chatPreview.panel, app.chatPreview.sidebarOpen)
	}
}

func TestDedicatedChatSnapshotAttachesModelCatalog(t *testing.T) {
	ai := newAISettingsController(CommonDeps{Translate: func(s string) string { return s }})
	ai.SetModels([]aiModel{{Name: "deepseek-v4-flash", Provider: "deepseek"}})
	app := &App{
		aiSettings: ai,
		chatPreview: &chatPreviewState{
			panel: chatCommandPanel,
			chat:  chatData{Model: aiModel{Name: "deepseek-v4-flash", Provider: "deepseek"}},
		},
	}

	snapshot := snapshotChatPreviewLocked(app.chatPreview)
	if len(snapshot.models) != 0 {
		t.Fatal("raw snapshot unexpectedly included models before attach")
	}
	app.attachChatPreviewCatalogs(snapshot)
	items := chatCommandPaletteItems(snapshot.models, snapshot.skills, snapshot.chat.Model, snapshot.panelQuery, snapshot.panel)
	if len(items) != 1 || items[0].title != "deepseek-v4-flash" {
		t.Fatalf("attached command palette = %+v, want the loaded model", items)
	}

	props := app.chatCatalogProps(snapshot, defaultPalette(), 400, 120)
	if len(props.Items) != 1 || props.Items[0].Title != "deepseek-v4-flash" {
		t.Fatalf("command catalog items = %+v empty=%q, want the loaded model", props.Items, props.EmptyMessage)
	}
}

func TestChatSkillTagRangesUsesRuneOffsets(t *testing.T) {
	text := "前 {skill:wox-plugin-creator} 后"
	ranges := chatSkillTagRanges(text)
	if len(ranges) != 1 || ranges[0].name != "wox-plugin-creator" {
		t.Fatalf("skill ranges = %+v", ranges)
	}
	runes := []rune(text)
	tag := "{skill:wox-plugin-creator}"
	if ranges[0].start != 2 || ranges[0].end != 2+len([]rune(tag)) || string(runes[ranges[0].start:ranges[0].end]) != tag {
		t.Fatalf("skill range = %+v, want rune span around the complete tag", ranges[0])
	}
	if got := chatSkillTagRanges("incomplete {skill:wox-plugin-creator"); len(got) != 0 {
		t.Fatalf("incomplete tag ranges = %+v, want none", got)
	}
}

func TestFindChatSlashTokenUsesTokenAtCaret(t *testing.T) {
	text := "hello /wri"
	token, ok := findChatSlashToken(woxui.TextEditingState{Text: text, Selection: woxui.TextSelection{Anchor: len([]rune(text)), Focus: len([]rune(text))}})
	if !ok || token.query != "wri" || token.start != 6 || token.end != 10 {
		t.Fatalf("slash token = %+v, %v", token, ok)
	}
}

func TestChatCommandPaletteFiltersModelsAndSkills(t *testing.T) {
	models := []aiModel{{Name: "deepseek-v4-pro", Provider: "deepseek"}}
	skills := []chatSkill{{Name: "writing-plans", Description: "Create an implementation plan", Source: "remote"}}

	items := chatCommandPaletteItems(models, skills, aiModel{}, "deep", chatCommandPanel)
	if len(items) != 1 || items[0].group != "models" || items[0].sourceIndex != 0 {
		t.Fatalf("model filter = %+v", items)
	}
	items = chatCommandPaletteItems(models, skills, aiModel{}, "implementation", chatCommandPanel)
	if len(items) != 1 || items[0].group != "skills" || items[0].sourceIndex != 0 {
		t.Fatalf("skill filter = %+v", items)
	}
}

func TestChatCommandCatalogShowsModelLoadingWhileSkillsReady(t *testing.T) {
	ai := newAISettingsController(CommonDeps{Translate: func(s string) string { return s }})
	ai.SetSkills([]chatSkill{{Name: "wox-plugin-creator", Description: "Create plugins"}})
	ai.SetModelsLoading(true)
	app := &App{
		aiSettings:  ai,
		chatPreview: &chatPreviewState{panel: chatCommandPanel, key: "chat-1"},
	}

	snapshot := snapshotChatPreviewLocked(app.chatPreview)
	app.attachChatPreviewCatalogs(snapshot)
	props := app.chatCatalogProps(snapshot, defaultPalette(), 400, 160)
	if len(props.Items) != 2 {
		t.Fatalf("catalog items = %+v, want loading model + ready skill", props.Items)
	}
	if !props.Items[0].Placeholder || props.Items[0].Kind != "models" || props.Items[0].Title != "ui ai chat loading models" {
		t.Fatalf("model placeholder = %+v", props.Items[0])
	}
	if props.Items[1].Title != "wox-plugin-creator" || props.Items[1].Placeholder || props.Items[1].OnSelect == nil {
		t.Fatalf("skill item = %+v", props.Items[1])
	}

	ai.SetModels([]aiModel{{Name: "grok-4.6", Provider: "grok"}})
	snapshot = snapshotChatPreviewLocked(app.chatPreview)
	app.attachChatPreviewCatalogs(snapshot)
	props = app.chatCatalogProps(snapshot, defaultPalette(), 400, 160)
	if len(props.Items) != 2 || props.Items[0].Placeholder || props.Items[0].Title != "grok-4.6" || props.Items[1].Title != "wox-plugin-creator" {
		t.Fatalf("after models loaded = %+v", props.Items)
	}
}

func TestChatCommandPaletteHeightIncludesModelLoadingRow(t *testing.T) {
	snapshot := &chatPreviewSnapshot{
		panel:         chatCommandPanel,
		skills:        []chatSkill{{Name: "wox-plugin-creator"}},
		modelsLoading: true,
	}
	if height := chatCatalogPanelHeight(snapshot, 600); height != 146 {
		t.Fatalf("loading palette height = %.0f, want 146", height)
	}
}

func TestChatModelPaletteHeightShrinksToContentAndCaps(t *testing.T) {
	snapshot := &chatPreviewSnapshot{panel: "models", models: []aiModel{{Name: "flash"}, {Name: "pro"}}}
	if height := chatCatalogPanelHeight(snapshot, 600); height != 118 {
		t.Fatalf("two-model palette height = %.0f, want content height 118", height)
	}

	snapshot.models = make([]aiModel, 20)
	if height := chatCatalogPanelHeight(snapshot, 600); height != 310 {
		t.Fatalf("large model palette height = %.0f, want maximum 310", height)
	}
}

func TestChatModelPaletteHeightIncludesTitleWhileLoading(t *testing.T) {
	snapshot := &chatPreviewSnapshot{panel: "models", modelsLoading: true}
	if height := chatCatalogPanelHeight(snapshot, 600); height != 82 {
		t.Fatalf("loading model palette height = %.0f, want 82 so the title and placeholder stay in one panel", height)
	}

	snapshot.modelsLoading = false
	if height := chatCatalogPanelHeight(snapshot, 600); height != 82 {
		t.Fatalf("empty model palette height = %.0f, want 82 so the title and empty copy stay in one panel", height)
	}
}

func TestSetChatTextOpensPluginMentionPanel(t *testing.T) {
	ai := newAISettingsController(CommonDeps{Translate: func(s string) string { return s }})
	ai.SetPluginMentions([]chatPluginMention{{ID: "notes", Name: "Notes"}})
	app := &App{
		aiSettings: ai,
		chatPreview: &chatPreviewState{
			panel:  "history",
			editor: woxui.NewTextEditor(""),
			chat:   chatData{ID: "1", Title: "Suzhou"},
		},
	}

	app.setChatText("@no")
	if !app.chatPreview.sidebarOpen {
		t.Fatal("typing @ hid the conversation sidebar")
	}
	if app.chatPreview.panel != chatMentionPanel {
		t.Fatalf("panel = %q, want %q", app.chatPreview.panel, chatMentionPanel)
	}
	if app.chatPreview.panelQuery != "no" {
		t.Fatalf("plugin query = %q", app.chatPreview.panelQuery)
	}

	app.setChatText("hello")
	if app.chatPreview.panel != "history" || !app.chatPreview.sidebarOpen {
		t.Fatalf("clearing @ did not restore the sidebar: panel=%q open=%v", app.chatPreview.panel, app.chatPreview.sidebarOpen)
	}
}

func TestFindChatAtTokenUsesTokenAtCaret(t *testing.T) {
	text := "hello @no"
	token, ok := findChatAtToken(woxui.TextEditingState{Text: text, Selection: woxui.TextSelection{Anchor: len([]rune(text)), Focus: len([]rune(text))}})
	if !ok || token.query != "no" || token.start != 6 || token.end != 9 {
		t.Fatalf("at token = %+v, %v", token, ok)
	}
}

func TestChatMentionPaletteItemsFilterByName(t *testing.T) {
	mentions := chatMentionsFromPlugins([]chatPluginMention{{ID: "notes", Name: "Notes"}, {ID: "files", Name: "File Search", NameEn: "File Search"}})
	items := chatMentionPaletteItems(mentions, "file", false)
	if len(items) != 1 || items[0].title != "File Search" || items[0].group != chatMentionKindPlugin {
		t.Fatalf("mention filter = %+v", items)
	}
}

func TestChatMentionPaletteItemsMatchPinyinWhenEnabled(t *testing.T) {
	mentions := []chatMention{
		{Kind: chatMentionKindPlugin, ID: "media", Name: "媒体播放器", NameEn: "Media Player"},
		{Kind: chatMentionKindPlugin, ID: "notes", Name: "Notes", NameEn: "Notes"},
	}
	if items := chatMentionPaletteItems(mentions, "mtbfq", false); len(items) != 0 {
		t.Fatalf("pinyin disabled should not match initials, got %+v", items)
	}
	items := chatMentionPaletteItems(mentions, "mtbfq", true)
	if len(items) != 1 || items[0].title != "媒体播放器" {
		t.Fatalf("pinyin initials = %+v", items)
	}
	items = chatMentionPaletteItems(mentions, "meiti", true)
	if len(items) != 1 || items[0].title != "媒体播放器" {
		t.Fatalf("pinyin full = %+v", items)
	}
}

func TestChatMentionCatalogGroupsPlugins(t *testing.T) {
	ai := newAISettingsController(CommonDeps{Translate: func(s string) string { return s }})
	ai.SetPluginMentions([]chatPluginMention{{ID: "notes", Name: "Notes"}})
	app := &App{
		aiSettings:     ai,
		lifecycleCtx:   context.Background(),
		images:         map[string]*woxui.Image{},
		imageRequested: map[string]string{},
		imageLastUsed:  map[string]uint64{},
		imageErrors:    map[string]string{},
		chatPreview:    &chatPreviewState{panel: chatMentionPanel, key: "chat-1"},
	}
	snapshot := snapshotChatPreviewLocked(app.chatPreview)
	app.attachChatPreviewCatalogs(snapshot)
	props := app.chatCatalogProps(snapshot, defaultPalette(), 400, 160)
	if props.Label != "" {
		t.Fatalf("mention catalog should use group headers, label = %q", props.Label)
	}
	if len(props.Items) != 3 {
		t.Fatalf("mention catalog items = %+v", props.Items)
	}
	if props.Items[0].Kind != chatMentionKindFiles || props.Items[0].GroupLabel != "ui ai chat mention files" || props.Items[0].OnSelect == nil {
		t.Fatalf("file picker item = %+v", props.Items[0])
	}
	if props.Items[1].Kind != chatMentionKindFiles || props.Items[1].GroupLabel != "ui ai chat mention files" || props.Items[1].OnSelect == nil {
		t.Fatalf("folder picker item = %+v", props.Items[1])
	}
	if props.Items[2].Title != "Notes" || props.Items[2].Kind != chatMentionKindPlugin || props.Items[2].GroupLabel != "ui ai chat mention plugins" || props.Items[2].Subtitle != "" || props.Items[2].OnSelect == nil {
		t.Fatalf("plugin mention item = %+v", props.Items[2])
	}
}

func TestChatFileMentionsUseColoredCatalogIcons(t *testing.T) {
	mentions := chatFileMentions(nil)
	if len(mentions) != 2 {
		t.Fatalf("file mentions = %+v", mentions)
	}
	fileIcon := fromCoreImage(icons.Get(icons.ChatSelectFile))
	folderIcon := fromCoreImage(icons.Get(icons.ChatSelectFolder))
	pluginFile := fromCoreImage(icons.Get(icons.PluginFile))
	pluginFolder := fromCoreImage(icons.Get(icons.PluginFolder))
	if mentions[0].Name != "Select file" || mentions[0].Icon != fileIcon || mentions[0].Icon == pluginFile {
		t.Fatalf("select-file mention = %+v", mentions[0])
	}
	if mentions[1].Name != "Select folder" || mentions[1].Icon != folderIcon || mentions[1].Icon == pluginFolder {
		t.Fatalf("select-folder mention = %+v", mentions[1])
	}
}

func TestChatPickerIDsDoNotCapturePluginMentions(t *testing.T) {
	for _, id := range []string{chatMentionIDOpenFile, chatMentionIDOpenFolder} {
		ai := newAISettingsController(CommonDeps{Translate: func(s string) string { return s }})
		ai.SetPluginMentions([]chatPluginMention{{ID: id, Name: "Picker plugin"}})
		editor := woxui.NewTextEditor("@")
		editor.SetCaret(1)
		app := &App{aiSettings: ai, chatPreview: &chatPreviewState{panel: chatMentionPanel, editor: editor}}
		catalog := app.chatMentionCatalog()
		app.insertChatMention(chatCommandPaletteItem{sourceIndex: 2})
		if got := editor.State().Text; got != "{plugin:"+id+"} " {
			t.Fatalf("plugin %s inserted %q", id, got)
		}
		refs := chatMentionRefsFromText(editor.State().Text, catalog)
		if len(refs) != 1 || refs[0].ID != id || refs[0].Kind != chatMentionKindPlugin {
			t.Fatalf("plugin %s references = %+v", id, refs)
		}
		if directory, ok := chatMentionOpensPicker(chatMention{Kind: chatMentionKindFiles, ID: id}); !ok || directory != (id == chatMentionIDOpenFolder) {
			t.Fatalf("file action %s no longer opens its picker", id)
		}
	}
}

func TestChatMentionPluginErrorRemainsVisibleWithFileActions(t *testing.T) {
	for _, cached := range []bool{false, true} {
		ai := newAISettingsController(CommonDeps{Translate: func(s string) string { return s }})
		if cached {
			ai.SetPluginMentions([]chatPluginMention{{ID: "notes", Name: "Notes"}})
		}
		ai.SetPluginMentionsError("Plugin catalog unavailable")
		app := &App{
			aiSettings: ai, lifecycleCtx: context.Background(),
			images: map[string]*woxui.Image{}, imageRequested: map[string]string{},
			imageLastUsed: map[string]uint64{}, imageErrors: map[string]string{},
			chatPreview: &chatPreviewState{panel: chatMentionPanel, key: "error-test"},
		}
		for _, query := range []string{"", "no-match"} {
			app.chatPreview.panelQuery = query
			snapshot := snapshotChatPreviewLocked(app.chatPreview)
			app.attachChatPreviewCatalogs(snapshot)
			props := app.chatMentionCatalogProps(snapshot, defaultPalette(), 400, 100)
			if len(props.Items) == 0 {
				t.Fatal("missing plugin error row")
			}
			status := props.Items[len(props.Items)-1]
			if status.Title != ai.PluginMentionsError() || status.Kind != chatMentionKindPlugin || status.GroupLabel == "" || !status.Placeholder || status.OnSelect != nil || status.Selected {
				t.Fatalf("plugin error row = %+v", status)
			}
			// Measure actual rows and group transitions independently of the status-height helper.
			height := float32(0)
			group := ""
			for _, row := range props.Items {
				if row.GroupLabel != group {
					height += chatCatalogGroupHeaderHeight
					group = row.GroupLabel
				}
				height += chatCatalogRowHeight
			}
			if props.ContentHeight != height || chatCatalogPanelHeight(snapshot, 600) != previewview.ChatCatalogHeight(height, false, 600) {
				t.Fatal("plugin error row is missing from panel geometry")
			}
			app.scrollChatPanel(1000)
			if app.chatPreview.panelScroll != max(float32(0), height-app.chatPreview.panelViewport) {
				t.Fatal("plugin error row is outside the scroll range")
			}
		}
	}
}

func TestChatMentionPaletteItemsIncludeFileAndFolderPickers(t *testing.T) {
	mentions := append(chatFileMentions(nil), chatMentionsFromPlugins([]chatPluginMention{{ID: "notes", Name: "Notes"}})...)
	items := chatMentionPaletteItems(mentions, "", false)
	if len(items) != 3 || items[0].group != chatMentionKindFiles || items[1].group != chatMentionKindFiles || items[2].group != chatMentionKindPlugin {
		t.Fatalf("full mention catalog = %+v", items)
	}
	items = chatMentionPaletteItems(mentions, "file", false)
	if len(items) != 1 || items[0].sourceIndex != 0 || items[0].group != chatMentionKindFiles {
		t.Fatalf("file query = %+v", items)
	}
	items = chatMentionPaletteItems(mentions, "folder", false)
	if len(items) != 1 || items[0].sourceIndex != 1 || items[0].group != chatMentionKindFiles {
		t.Fatalf("folder query = %+v", items)
	}
}

func TestInsertChatFileMentionClearsAtToken(t *testing.T) {
	ai := newAISettingsController(CommonDeps{Translate: func(s string) string { return s }})
	editor := woxui.NewTextEditor("see @fi")
	editor.SetCaret(len([]rune("see @fi")))
	app := &App{
		aiSettings: ai,
		chatPreview: &chatPreviewState{
			panel:  chatMentionPanel,
			editor: editor,
			chat:   chatData{ID: "1"},
		},
	}
	items := app.filteredChatMentionItems(app.chatMentionCatalog(), "fi")
	if len(items) == 0 || items[0].sourceIndex != 0 {
		t.Fatalf("file mention items = %+v", items)
	}
	app.insertChatMention(items[0])
	if got := app.chatPreview.editor.State().Text; got != "see " {
		t.Fatalf("composer text = %q, want the @ token removed", got)
	}
	if app.chatPreview.panel == chatMentionPanel {
		t.Fatal("file picker should close the mention overlay")
	}
	if strings.Contains(app.chatPreview.editor.State().Text, "{") {
		t.Fatal("file picker must not insert a mention tag")
	}
}

func TestReplaceChatAtTokenWithPluginTag(t *testing.T) {
	editor := woxui.NewTextEditor("use @no now")
	editor.SetCaret(7)
	replaceChatAtToken(editor, "{plugin:Notes}")
	state := editor.State()
	if state.Text != "use {plugin:Notes} now" || state.Selection.Focus != 18 {
		t.Fatalf("replaced editor = %+v", state)
	}
}

func TestChatMentionChipLabelKeepsAtPrefix(t *testing.T) {
	if got := chatMentionChipLabel("媒体播放器"); got != "@媒体播放器" {
		t.Fatalf("mention chip label = %q", got)
	}
	if got := chatMentionChipLabel(""); got != "@" {
		t.Fatalf("empty mention chip label = %q", got)
	}
}

func TestLookupChatMentionMatchesNameAndID(t *testing.T) {
	mentions := chatMentionsFromPlugins([]chatPluginMention{{ID: "notes-id", Name: "笔记", NameEn: "Notes"}})
	if got, ok := lookupChatMention(chatMentionKindPlugin, "Notes", mentions); !ok || got.ID != "notes-id" {
		t.Fatalf("english lookup = %+v %v", got, ok)
	}
	if got, ok := lookupChatMention(chatMentionKindPlugin, "笔记", mentions); !ok || got.ID != "notes-id" {
		t.Fatalf("localized lookup = %+v %v", got, ok)
	}
	if _, ok := lookupChatMention(chatMentionKindPlugin, "missing", mentions); ok {
		t.Fatal("missing mention should not match")
	}
	if _, ok := lookupChatMention("file", "Notes", mentions); ok {
		t.Fatal("other mention kinds must not match plugin catalog entries")
	}
}

func TestChatInputPluginChipUsesLocalizedCatalogName(t *testing.T) {
	ai := newAISettingsController(CommonDeps{Translate: func(s string) string { return s }})
	ai.SetPluginMentions([]chatPluginMention{{ID: "notes", Name: "Media Player", NameEn: "Media Player"}})
	app := &App{aiSettings: ai, chatPreview: &chatPreviewState{key: "chat-1", editor: woxui.NewTextEditor("{plugin:notes} hello")}}
	snapshot := snapshotChatPreviewLocked(app.chatPreview)
	app.attachChatPreviewCatalogs(snapshot)
	props := app.chatInputProps(snapshot, defaultPalette(), 400, 80, nil, nil)
	if len(props.RichRuns) != 1 || props.RichRuns[0].ChipLabel != "@Media Player" {
		t.Fatalf("localized chip = %#v", props.RichRuns)
	}
}

func TestChatMessagePropsRendersPluginMentionAsChip(t *testing.T) {
	ai := newAISettingsController(CommonDeps{Translate: func(s string) string { return s }})
	ai.SetPluginMentions([]chatPluginMention{{ID: "notes", Name: "笔记", NameEn: "Notes", Icon: woxImage{ImageType: "emoji", ImageData: "📝"}}})
	app := &App{
		aiSettings:     ai,
		lifecycleCtx:   context.Background(),
		images:         map[string]*woxui.Image{},
		imageRequested: map[string]string{},
		imageLastUsed:  map[string]uint64{},
		imageErrors:    map[string]string{},
		previewLayouts: map[string]*textLayoutCache{},
	}
	props := app.chatMessageProps("chat", 0, chatConversation{
		Role: "user", Text: "{plugin:notes} 现在有几个笔记?",
		Mentions: []chatMentionRef{{Kind: chatMentionKindPlugin, ID: "notes", Name: "笔记"}},
	}, defaultPalette(), 400, false, false, 1)
	if props.Skills != "" {
		t.Fatalf("inline mention chips should not add a skills footer, got %q", props.Skills)
	}
	if len(props.RichRuns) != 1 || props.RichRuns[0].ChipLabel != "@笔记" {
		t.Fatalf("mention chip = %#v", props.RichRuns)
	}
	plain := woxcomponent.MeasureTokenChip(nil, "@笔记")
	if props.RichRuns[0].Advance < plain {
		t.Fatalf("plugin chip advance = %.0f, want at least the text chip width", props.RichRuns[0].Advance)
	}
}

func TestChatMessagePropsRendersSkillTagAsChip(t *testing.T) {
	app := &App{aiSettings: newAISettingsController(CommonDeps{Translate: func(s string) string { return s }}), previewLayouts: map[string]*textLayoutCache{}}
	props := app.chatMessageProps("chat", 0, chatConversation{
		Role: "user", Text: "{skill:review} please",
		SkillRefs: []chatSkillRef{{ID: "review", Name: "review"}},
	}, defaultPalette(), 400, false, false, 1)
	if props.Skills != "" {
		t.Fatalf("inline skill chips should not add a skills footer, got %q", props.Skills)
	}
	if len(props.RichRuns) != 1 || props.RichRuns[0].ChipLabel != "review" {
		t.Fatalf("skill chip = %#v", props.RichRuns)
	}
}

func TestChatMessagePropsKeepsSkillsFooterWithoutInlineTags(t *testing.T) {
	app := &App{aiSettings: newAISettingsController(CommonDeps{Translate: func(s string) string { return s }}), previewLayouts: map[string]*textLayoutCache{}}
	props := app.chatMessageProps("chat", 0, chatConversation{
		Role: "user", Text: "hello",
		Mentions: []chatMentionRef{{Kind: chatMentionKindPlugin, ID: "notes", Name: "笔记"}},
	}, defaultPalette(), 400, false, false, 1)
	if props.Skills != "@笔记" {
		t.Fatalf("skills footer = %q", props.Skills)
	}
	if len(props.RichRuns) != 0 {
		t.Fatalf("rich runs = %#v", props.RichRuns)
	}
}

func TestChatInputPluginChipUsesCatalogIcon(t *testing.T) {
	ai := newAISettingsController(CommonDeps{Translate: func(s string) string { return s }})
	ai.SetPluginMentions([]chatPluginMention{{ID: "notes", Name: "Notes", Icon: woxImage{ImageType: "emoji", ImageData: "📝"}}})
	app := &App{
		aiSettings:     ai,
		lifecycleCtx:   context.Background(),
		images:         map[string]*woxui.Image{},
		imageRequested: map[string]string{},
		imageLastUsed:  map[string]uint64{},
		imageErrors:    map[string]string{},
		chatPreview:    &chatPreviewState{key: "chat-1", editor: woxui.NewTextEditor("{plugin:Notes} hello")},
	}
	snapshot := snapshotChatPreviewLocked(app.chatPreview)
	app.attachChatPreviewCatalogs(snapshot)
	props := app.chatInputProps(snapshot, defaultPalette(), 400, 80, nil, nil)
	if len(props.RichRuns) != 1 || props.RichRuns[0].ChipLabel != "@Notes" {
		t.Fatalf("plugin chip = %#v", props.RichRuns)
	}
	plain := woxcomponent.MeasureTokenChip(nil, "@Notes")
	if props.RichRuns[0].Advance < plain {
		t.Fatalf("plugin chip advance = %.0f, want at least the text chip width", props.RichRuns[0].Advance)
	}
}

func TestChatMentionTagRangesUsesRuneOffsetsAndIgnoresSkills(t *testing.T) {
	text := "前 {plugin:Notes} {skill:Review} 后"
	ranges := chatMentionTagRanges(text)
	if len(ranges) != 1 || ranges[0].name != "Notes" || ranges[0].kind != chatMentionKindPlugin {
		t.Fatalf("mention ranges = %+v", ranges)
	}
	runes := []rune(text)
	tag := "{plugin:Notes}"
	if ranges[0].start != 2 || ranges[0].end != 2+len([]rune(tag)) || string(runes[ranges[0].start:ranges[0].end]) != tag {
		t.Fatalf("mention range = %+v", ranges[0])
	}
}

func TestReplaceChatSlashTokenWithSkillTag(t *testing.T) {
	editor := woxui.NewTextEditor("use /wri now")
	editor.SetCaret(8)
	replaceChatSlashToken(editor, "{skill:writing-plans}")
	state := editor.State()
	if state.Text != "use {skill:writing-plans} now" || state.Selection.Focus != 25 {
		t.Fatalf("replaced editor = %+v", state)
	}
}

func TestChatPaletteIgnoresKeyRelease(t *testing.T) {
	app := &App{chatPreview: &chatPreviewState{active: true, panel: chatCommandPanel, panelSelected: 2}}
	if app.onChatPreviewKey(woxui.KeyEvent{Key: woxui.KeyArrowDown}) {
		t.Fatal("key release was handled")
	}
	if app.chatPreview.panelSelected != 2 {
		t.Fatalf("selection moved to %d on key release", app.chatPreview.panelSelected)
	}
}

func TestPrimaryChatEscapeReturnsToQuery(t *testing.T) {
	app := &App{
		isPrimary:      true,
		chatFullscreen: true,
		chatPreview:    &chatPreviewState{active: true},
		editor:         woxui.NewTextEditor("chat "),
	}

	if !app.onChatPreviewKey(woxui.KeyEvent{Key: woxui.KeyEscape, Down: true}) {
		t.Fatal("Escape was not handled")
	}
	if app.chatFullscreen || app.chatPreview == nil || app.chatPreview.active {
		t.Fatalf("chat mode state = fullscreen:%v preview:%+v", app.chatFullscreen, app.chatPreview)
	}
	if selection := app.editor.State().Selection; selection.Anchor != 0 || selection.Focus != len([]rune("chat ")) {
		t.Fatalf("query selection after escape = %+v, want full text selected", selection)
	}
}

func TestPrimaryChatShortcutTogglesHistorySidebar(t *testing.T) {
	app := &App{chatPreview: &chatPreviewState{active: true}}

	primaryModifier := woxui.KeyModifierControl
	if strings.HasPrefix(primaryHotkey("b"), "command+") {
		primaryModifier = woxui.KeyModifierMeta
	}
	event := woxui.KeyEvent{Key: woxui.Key("b"), Modifiers: primaryModifier, Down: true}
	if !app.onChatPreviewKey(event) {
		t.Fatal("primary+B was not handled")
	}
	if app.chatPreview.panel != "history" {
		t.Fatalf("history panel = %q after primary+B, want open", app.chatPreview.panel)
	}
	if !app.onChatPreviewKey(event) {
		t.Fatal("second primary+B was not handled")
	}
	if app.chatPreview.panel != "" {
		t.Fatalf("history panel = %q after second primary+B, want closed", app.chatPreview.panel)
	}
}

func TestHistoryDrawerLeavesComposerKeysToTheEditor(t *testing.T) {
	app := &App{chatPreview: &chatPreviewState{active: true, panel: "history", sidebarOpen: true, editor: woxui.NewTextEditor("")}}
	for _, event := range []woxui.KeyEvent{
		{Key: woxui.KeyEnter, Down: true, Modifiers: woxui.KeyModifierShift},
		{Key: woxui.KeyDelete, Down: true},
		{Key: woxui.KeyArrowUp, Down: true},
		{Key: woxui.KeyArrowDown, Down: true},
		{Key: woxui.KeyTab, Down: true},
	} {
		if app.onChatPreviewKey(event) {
			t.Fatalf("history drawer intercepted composer key %+v", event)
		}
	}
	// Empty-submit validation proves Enter reached send rather than history activation.
	if !app.onChatPreviewKey(woxui.KeyEvent{Key: woxui.KeyEnter, Down: true}) || app.chatPreview.error == "" {
		t.Fatal("Enter did not reach composer submission")
	}
	if !app.onChatPreviewKey(woxui.KeyEvent{Key: woxui.KeyEscape, Down: true}) || app.chatPreview.panel != "" || !app.chatPreview.active {
		t.Fatal("Escape should dismiss the drawer while keeping the composer active")
	}
}

func TestChatHistoryContentHeightIncludesGroupsAndRows(t *testing.T) {
	location := time.FixedZone("test", 8*60*60)
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, location)
	chat := func(id string, updatedAt int64) chatData {
		return chatData{ID: id, UpdatedAt: updatedAt, Conversations: []chatConversation{{Role: "user", Text: "hi"}}}
	}
	chats := []chatData{
		chat("today-1", now.UnixMilli()),
		chat("today-2", now.UnixMilli()),
		chat("old", now.AddDate(0, 0, -5).UnixMilli()),
	}
	if got := chatHistoryContentHeight(chats, now); got != 46+2*32+3*38 {
		t.Fatalf("history content height = %.0f, want %d", got, 46+2*32+3*38)
	}
	if got := chatHistoryContentHeight(nil, now); got != 46 {
		t.Fatalf("empty history content height = %.0f, want 46", got)
	}
}

func TestChatMentionWheelScrollUsesMentionContentHeight(t *testing.T) {
	ai := newAISettingsController(CommonDeps{Translate: func(s string) string { return s }})
	plugins := make([]chatPluginMention, 20)
	for i := range plugins {
		plugins[i] = chatPluginMention{ID: fmt.Sprintf("p%d", i), Name: fmt.Sprintf("Plugin %d", i)}
	}
	ai.SetPluginMentions(plugins)
	app := &App{aiSettings: ai, chatPreview: &chatPreviewState{panel: chatMentionPanel, panelViewport: 160}}
	contentHeight := chatCommandContentHeight(app.filteredChatMentionItems(app.chatMentionCatalog(), ""))
	if contentHeight <= 160 {
		t.Fatalf("mention catalog too short to overflow: %.0f", contentHeight)
	}
	app.scrollChatPanel(80)
	if app.chatPreview.panelScroll != 80 {
		t.Fatalf("mention wheel scroll = %.0f, want 80 (content %.0f)", app.chatPreview.panelScroll, contentHeight)
	}
}

func TestChatHistoryWheelScrollUsesDrawerContentHeight(t *testing.T) {
	now := time.Now()
	chats := make([]chatData, 14)
	for i := range chats {
		chats[i] = chatData{ID: string(rune('a' + i)), UpdatedAt: now.UnixMilli(), Conversations: []chatConversation{{Role: "user", Text: "hi"}}}
	}
	// The old len*38 estimate kept maxOffset at 0 for moderate histories, so wheel scroll never moved.
	contentHeight := chatHistoryContentHeight(chats, time.Now())
	viewport := float32(576)
	app := &App{chatPreview: &chatPreviewState{panel: "history", chats: chats, panelViewport: viewport}}

	app.scrollChatPanel(120)
	want := min(float32(120), max(float32(0), contentHeight-viewport))
	if app.chatPreview.panelScroll != want {
		t.Fatalf("history panel scroll = %.0f, want %.0f (content %.0f, viewport %.0f)", app.chatPreview.panelScroll, want, contentHeight, viewport)
	}
	if app.chatPreview.panelScroll == 0 {
		t.Fatal("history wheel scroll stayed at 0 with overflow")
	}
}

func TestChatDebugGeometryClampsControlledScroll(t *testing.T) {
	app := &App{chatPreview: &chatPreviewState{panel: "debug", panelScroll: 500}}

	app.setChatDebugGeometry(100, 260)

	if app.chatPreview.panelMaxScroll != 160 || app.chatPreview.panelScroll != 160 {
		t.Fatalf("debug geometry = max %.0f offset %.0f, want 160/160", app.chatPreview.panelMaxScroll, app.chatPreview.panelScroll)
	}
}

func TestChatHistoryViewportUpdateKeepsWheelScroll(t *testing.T) {
	now := time.Now()
	chats := make([]chatData, 14)
	for i := range chats {
		chats[i] = chatData{ID: string(rune('a' + i)), UpdatedAt: now.UnixMilli(), Conversations: []chatConversation{{Role: "user", Text: "hi"}}}
	}
	app := &App{chatPreview: &chatPreviewState{panel: "history", chats: chats, panelViewport: 576}}

	app.scrollChatPanel(120)
	scrolled := app.chatPreview.panelScroll
	if scrolled == 0 {
		t.Fatal("wheel scroll did not move")
	}
	// The next build re-records the viewport and must not clamp the offset back to zero.
	app.setChatPanelViewport(576)
	if app.chatPreview.panelScroll != scrolled {
		t.Fatalf("viewport update changed scroll from %.0f to %.0f", scrolled, app.chatPreview.panelScroll)
	}
}

type chatCatalogPrefetchServices struct {
	contract.Services
	models []contract.AIModel
	skills []contract.AISkill

	modelsStarted chan struct{}
	skillsStarted chan struct{}
	release       chan struct{}

	modelCalls atomic.Int32
	skillCalls atomic.Int32
}

func (s *chatCatalogPrefetchServices) AIModels(_ context.Context, _ string) ([]contract.AIModel, error) {
	s.modelCalls.Add(1)
	if s.modelsStarted != nil {
		s.modelsStarted <- struct{}{}
	}
	if s.release != nil {
		<-s.release
	}
	return append([]contract.AIModel(nil), s.models...), nil
}

func (s *chatCatalogPrefetchServices) ChatPluginMentions(_ context.Context, _ string) ([]contract.AIPluginMention, error) {
	return nil, nil
}

func (s *chatCatalogPrefetchServices) AISkills(_ context.Context, _ string) ([]contract.AISkill, error) {
	s.skillCalls.Add(1)
	if s.skillsStarted != nil {
		s.skillsStarted <- struct{}{}
	}
	if s.release != nil {
		<-s.release
	}
	return append([]contract.AISkill(nil), s.skills...), nil
}

func waitChatCatalogPrefetch(t *testing.T, started chan struct{}) {
	t.Helper()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("chat catalog prefetch did not start")
	}
}

func TestPrefetchChatCatalogsLoadsOnceWhenUncached(t *testing.T) {
	service := &chatCatalogPrefetchServices{
		models:        []contract.AIModel{{Name: "flash", Provider: "deepseek"}},
		skills:        []contract.AISkill{{ID: "s1", Name: "creator", Enabled: true}},
		modelsStarted: make(chan struct{}, 1),
		skillsStarted: make(chan struct{}, 1),
		release:       make(chan struct{}),
	}
	ai := newAISettingsController(CommonDeps{Translate: func(s string) string { return s }})
	app := &App{lifecycleCtx: t.Context(), aiSettings: ai, services: service}

	app.prefetchChatCatalogs()
	waitChatCatalogPrefetch(t, service.modelsStarted)
	waitChatCatalogPrefetch(t, service.skillsStarted)
	if !ai.ModelsLoading() || !ai.SkillsLoading() {
		t.Fatal("uncached chat entry should start model and skill loads")
	}

	app.prefetchChatCatalogs()
	if service.modelCalls.Load() != 1 || service.skillCalls.Load() != 1 {
		t.Fatalf("in-flight prefetch started another load: models=%d skills=%d", service.modelCalls.Load(), service.skillCalls.Load())
	}
	close(service.release)
}

func TestPrefetchChatCatalogsSkipsCachedCatalogs(t *testing.T) {
	service := &chatCatalogPrefetchServices{
		modelsStarted: make(chan struct{}, 1),
		skillsStarted: make(chan struct{}, 1),
	}
	ai := newAISettingsController(CommonDeps{Translate: func(s string) string { return s }})
	ai.SetModels([]aiModel{{Name: "flash", Provider: "deepseek"}})
	ai.SetSkills([]chatSkill{{ID: "s1", Name: "creator"}})
	app := &App{lifecycleCtx: t.Context(), aiSettings: ai, services: service}

	app.prefetchChatCatalogs()
	if ai.ModelsLoading() || ai.SkillsLoading() {
		t.Fatal("cached catalogs should not start another load")
	}
	if service.modelCalls.Load() != 0 || service.skillCalls.Load() != 0 {
		t.Fatalf("cached catalogs were fetched again: models=%d skills=%d", service.modelCalls.Load(), service.skillCalls.Load())
	}
}

func TestEnterChatModePrefetchesUncachedCatalogs(t *testing.T) {
	service := &chatCatalogPrefetchServices{
		modelsStarted: make(chan struct{}, 1),
		skillsStarted: make(chan struct{}, 1),
		release:       make(chan struct{}),
	}
	ai := newAISettingsController(CommonDeps{Translate: func(s string) string { return s }})
	app := &App{
		lifecycleCtx: t.Context(),
		aiSettings:   ai,
		services:     service,
		editor:       woxui.NewTextEditor(""),
		chatPreview:  &chatPreviewState{editor: woxui.NewTextEditor("")},
	}

	app.enterChatMode()
	waitChatCatalogPrefetch(t, service.modelsStarted)
	waitChatCatalogPrefetch(t, service.skillsStarted)
	if !ai.ModelsLoading() || !ai.SkillsLoading() {
		t.Fatal("entering chat should prefetch catalogs when the cache is empty")
	}
	close(service.release)
}

func TestActivateChatPreviewPrefetchesUncachedCatalogs(t *testing.T) {
	service := &chatCatalogPrefetchServices{
		modelsStarted: make(chan struct{}, 1),
		skillsStarted: make(chan struct{}, 1),
		release:       make(chan struct{}),
	}
	ai := newAISettingsController(CommonDeps{Translate: func(s string) string { return s }})
	app := &App{lifecycleCtx: t.Context(), aiSettings: ai, services: service}

	if err := app.activateChatPreview(queryResult{ID: "result", QueryID: "query"}, queryPreview{
		PreviewType: "chat",
		PreviewData: `{"ActiveChat":{"Id":"chat"}}`,
	}); err != nil {
		t.Fatal(err)
	}
	waitChatCatalogPrefetch(t, service.modelsStarted)
	waitChatCatalogPrefetch(t, service.skillsStarted)
	if !ai.ModelsLoading() || !ai.SkillsLoading() {
		t.Fatal("showing chat should prefetch catalogs when the cache is empty")
	}
	close(service.release)
}
