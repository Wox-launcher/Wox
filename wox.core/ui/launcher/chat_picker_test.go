package launcher

import (
	"fmt"
	"testing"

	"wox/common"
	woxui "wox/ui/runtime"
)

func TestChatPickerGroupsConfiguredProvidersWithoutDroppingModels(t *testing.T) {
	models := []aiModel{
		{Name: "a", Provider: "deepseek"},
		{Name: "b", Provider: "openai", ProviderAlias: "work"},
		{Name: "c", Provider: "deepseek"},
		{Name: "d", Provider: "openai", ProviderAlias: "personal"},
	}
	for i := 0; i < 40; i++ {
		models = append(models, aiModel{Name: fmt.Sprintf("model-%d", i), Provider: "magpie"})
	}
	items := chatCommandPaletteItems(models, []chatSkill{{Name: "creator"}}, models[2], "", chatCommandPanel)
	if len(items) != 45 {
		t.Fatalf("complete catalog has %d items", len(items))
	}
	for i, source := range []int{0, 2, 1, 3} {
		if items[i].sourceIndex != source {
			t.Fatalf("row %d refers to source %d, want %d", i, items[i].sourceIndex, source)
		}
	}
	if !items[1].current || items[44].group != "skills" {
		t.Fatal("grouping lost the active model or skills")
	}
	filtered := chatCommandPaletteItems(models, nil, models[2], "personal", "models")
	if len(filtered) != 1 || filtered[0].sourceIndex != 3 {
		t.Fatalf("provider alias filtering = %+v", filtered)
	}
}

func TestChatSlashSearchIncludesProjects(t *testing.T) {
	projects := chatProjectUI{Loaded: true, Projects: []common.AIChatProject{{Id: "a", Name: "Wox"}, {Id: "b", Name: "Work"}}}
	models := []aiModel{{Name: "Wox model", Provider: "provider"}}
	skills := []chatSkill{{Name: "Wox skill"}}
	items := chatSlashPaletteItems(models, skills, aiModel{}, projects, "b", "No project", "wox", chatCommandPanel)
	if len(items) != 3 || items[0].group != chatProjectPanel || items[0].sourceIndex != 0 || items[1].group != "models" || items[2].group != "skills" {
		t.Fatalf("mixed search results = %+v", items)
	}
	items = chatSlashPaletteItems(models, skills, aiModel{}, projects, "b", "No project", "work", chatProjectPanel)
	if len(items) != 1 || items[0].sourceIndex != 1 || !items[0].current {
		t.Fatalf("project filter = %+v", items)
	}
	items = chatSlashPaletteItems(models, skills, aiModel{}, projects, "", "No project", "no project", chatProjectPanel)
	if len(items) != 1 || items[0].sourceIndex != -1 || !items[0].current {
		t.Fatalf("no-project filter = %+v", items)
	}
}

func TestChatSlashProjectSelectionPreservesDraft(t *testing.T) {
	for _, pointer := range []bool{false, true} {
		t.Run(fmt.Sprintf("pointer=%t", pointer), func(t *testing.T) {
			app := New(false, projectTestServices{})
			defer app.cancel()
			app.uiCall = nil
			editor := woxui.NewTextEditor("before /wox after")
			editor.SetCaret(len([]rune("before /wox")))
			state := &chatPreviewState{
				key: "chat", active: true, panel: chatCommandPanel, panelQuery: "wox", panelFilter: chatProjectPanel,
				sidebarOpen: true, editor: editor, projects: chatProjectUI{Loaded: true, Projects: []common.AIChatProject{{Id: "p", Name: "Wox"}}},
			}
			app.chat.chatPreview = state
			snapshot := snapshotChatPreviewLocked(state)
			app.chat.attachChatPreviewCatalogs(snapshot)
			props := app.chatCatalogProps(snapshot, defaultPalette(), 400, 300)
			if len(props.Filters) != 4 || !props.Filters[2].Selected || len(props.Items) != 1 || props.Items[0].Kind != chatProjectPanel {
				t.Fatalf("project picker = %+v", props)
			}
			if pointer {
				props.Items[0].OnSelect()
			} else {
				app.chat.activateChatPanelSelection()
			}
			if state.chat.ProjectID != "p" || editor.State().Text != "before  after" || state.panel != "history" || !state.sidebarOpen {
				t.Fatalf("project selection lost draft or sidebar: project=%q draft=%q panel=%q", state.chat.ProjectID, editor.State().Text, state.panel)
			}
		})
	}
}

func TestChatSlashNoProjectAndStreamingGuard(t *testing.T) {
	app := New(false, projectTestServices{})
	defer app.cancel()
	app.uiCall = nil
	state := &chatPreviewState{
		active: true, panel: chatCommandPanel, panelFilter: chatProjectPanel,
		editor: woxui.NewTextEditor("/"), chat: chatData{ProjectID: "p", IsStreaming: true},
		projects: chatProjectUI{Loaded: true, Projects: []common.AIChatProject{{Id: "p", Name: "Wox"}}},
	}
	app.chat.chatPreview = state
	app.chat.activateChatPanelSelection()
	if state.chat.ProjectID != "p" || state.editor.State().Text != "/" || state.panel != chatCommandPanel {
		t.Fatal("streaming chat allowed a project change")
	}
	state.chat.IsStreaming = false
	app.chat.activateChatPanelSelection()
	if state.chat.ProjectID != "" || state.editor.State().Text != "" || state.panel != "" {
		t.Fatalf("no-project selection failed: project=%q text=%q panel=%q", state.chat.ProjectID, state.editor.State().Text, state.panel)
	}
}

func TestChatPickerCategoryPreservesSlashAndUsesFilteredKeyboardSelection(t *testing.T) {
	ai := newAISettingsController(CommonDeps{Translate: func(s string) string { return s }})
	ai.SetModels([]aiModel{{Name: "flash", Provider: "deepseek"}})
	ai.SetSkills([]chatSkill{{Name: "creator"}, {Name: "reviewer"}})
	app := bindTestApp(&App{aiSettings: ai, chat: chatController{chatPreview: &chatPreviewState{
		key: "picker", editor: woxui.NewTextEditor(""), active: true,
	}}})
	app.chat.setChatText("/")
	app.chat.setChatCatalogFilter("skills")
	snapshot := snapshotChatPreviewLocked(app.chat.chatPreview)
	app.chat.attachChatPreviewCatalogs(snapshot)
	props := app.chatCatalogProps(snapshot, defaultPalette(), 500, 400)
	if len(props.Items) != 2 || len(props.Filters) != 4 || !props.Filters[3].Selected || props.Search != nil {
		t.Fatalf("skills filter = %+v", props)
	}
	if app.chat.chatPreview.editor.State().Text != "/" {
		t.Fatal("category selection changed the draft")
	}
	app.chat.moveChatPanelSelection(1)
	app.chat.activateChatPanelSelection()
	if got := app.chat.chatPreview.editor.State().Text; got != "{skill:reviewer} " {
		t.Fatalf("filtered Enter inserted %q", got)
	}
	app.chat.setChatText("/")
	if app.chat.chatPreview.panelFilter != "" {
		t.Fatal("a fresh slash should start in All")
	}
	app.chat.setChatCatalogFilter("models")
	app.chat.setChatText("/deep")
	if app.chat.chatPreview.panelFilter != "models" {
		t.Fatal("typing reset the active category")
	}
}

func TestChatPickerGeometryAndKeyboardRevealIncludeProviderHeadersAndSkillDescriptions(t *testing.T) {
	models := []aiModel{{Name: "a", Provider: "one"}, {Name: "b", Provider: "two"}}
	items := chatCommandPaletteItems(models, []chatSkill{{Name: "skill", Description: "description"}}, aiModel{}, "", chatCommandPanel)
	// Include section titles, provider headers, group spacing and the two-line skill.
	if got := chatCommandContentHeight(items); got != 262 {
		t.Fatalf("content height = %v", got)
	}
	state := &chatPreviewState{panel: chatCommandPanel, panelViewport: 120, panelSelected: 2}
	ensureChatCommandSelectionVisibleLocked(state, items)
	if state.panelScroll != 142 {
		t.Fatalf("last skill is clipped: offset %v", state.panelScroll)
	}
	state.panelSelected = 0
	ensureChatCommandSelectionVisibleLocked(state, items)
	if state.panelScroll != 0 {
		t.Fatalf("first provider row not revealed: offset %v", state.panelScroll)
	}
}

func TestChatModelSearchLeavesDraftUntouched(t *testing.T) {
	ai := newAISettingsController(CommonDeps{Translate: func(s string) string { return s }})
	ai.SetModels([]aiModel{{Name: "flash", Provider: "deepseek"}, {Name: "pro", Provider: "magpie"}})
	app := bindTestApp(&App{aiSettings: ai, chat: chatController{chatPreview: &chatPreviewState{
		panel: "models", editor: woxui.NewTextEditor("keep this draft"),
	}}})
	app.chat.setChatCatalogQuery("magpie")
	snapshot := snapshotChatPreviewLocked(app.chat.chatPreview)
	app.chat.attachChatPreviewCatalogs(snapshot)
	props := app.chatCatalogProps(snapshot, defaultPalette(), 500, 400)
	if props.Search == nil || len(props.Filters) != 0 || len(props.Items) != 1 || props.Items[0].Title != "pro" {
		t.Fatalf("model-only search = %+v", props)
	}
	if app.chat.chatPreview.editor.State().Text != "keep this draft" {
		t.Fatal("model search changed the message")
	}
}

func TestChatPickerKeyboardRevealRestoresGroupContext(t *testing.T) {
	items := chatCommandPaletteItems(
		[]aiModel{{Name: "flash", Provider: "deepseek"}, {Name: "pro", Provider: "deepseek"},
			{Name: "a", Provider: "magpie"}, {Name: "b", Provider: "magpie"}},
		[]chatSkill{{Name: "creator"}, {Name: "reviewer"}, {Name: "writer"}, {Name: "tester"}}, aiModel{}, "", chatCommandPanel)
	for _, scale := range []float32{0.9, 1, 1.1} {
		t.Run(fmt.Sprintf("density-%g", scale), func(t *testing.T) {
			header := float32(int(chatCatalogGroupHeaderHeight*scale + 0.5))
			for _, selected := range []int{4, 2, 0} {
				rowTop := chatCommandItemOffset(items, selected, scale)
				state := &chatPreviewState{
					panel: chatCommandPanel, panelDensityScale: scale,
					panelViewport: 160, panelSelected: selected, panelScroll: rowTop,
				}
				ensureChatCommandSelectionVisibleLocked(state, items)
				want := rowTop - header
				if selected == 0 {
					want = 0
				}
				if state.panelScroll != want {
					t.Fatalf("row %d: scroll %g, want header at %g", selected, state.panelScroll, want)
				}
			}
		})
	}
	// A constrained viewport must still expose the complete selected row.
	state := &chatPreviewState{panel: chatCommandPanel, panelViewport: 60, panelScroll: 100}
	ensureChatCommandSelectionVisibleLocked(state, items)
	if state.panelScroll != 34 {
		t.Fatalf("small viewport clipped the first row: scroll %g", state.panelScroll)
	}
}

func TestChatPickerTabCyclesCategories(t *testing.T) {
	for _, dedicated := range []bool{false, true} {
		t.Run(fmt.Sprintf("dedicated-%t", dedicated), func(t *testing.T) {
			state := &chatPreviewState{
				active: true, panel: chatCommandPanel, editor: woxui.NewTextEditor("/deep"),
				panelQuery: "deep",
			}
			app := bindTestApp(&App{chat: chatController{chatPreview: state}})
			for _, step := range []struct {
				shift bool
				want  string
			}{
				{false, "models"}, {false, chatProjectPanel}, {false, "skills"}, {false, ""},
				{true, "skills"}, {true, chatProjectPanel}, {true, "models"}, {true, ""},
			} {
				state.panelSelected, state.panelScroll = 3, 120
				event := woxui.KeyEvent{Key: woxui.KeyTab, Down: true}
				if step.shift {
					event.Modifiers = woxui.KeyModifierShift
				}
				if !app.chat.handleChatKey(event, dedicated) || state.panelFilter != step.want {
					t.Fatalf("Tab shift=%t: category %q, want %q", step.shift, state.panelFilter, step.want)
				}
				if state.panelSelected != 0 || state.panelScroll != 0 {
					t.Fatal("category switch did not reset list navigation")
				}
				if state.editor.State().Text != "/deep" || state.panelQuery != "deep" || state.panel != chatCommandPanel {
					t.Fatal("Tab changed the query or closed the picker")
				}
			}
			for _, event := range []woxui.KeyEvent{
				{Key: woxui.KeyTab},
				{Key: woxui.KeyTab, Down: true, Composing: true},
			} {
				if app.chat.handleChatKey(event, dedicated) || state.panelFilter != "" {
					t.Fatal("key release or IME composition cycled categories")
				}
			}
		})
	}
}

func TestMentionPickerSharesCategoryFilteringAndKeyboardCycle(t *testing.T) {
	ai := newAISettingsController(CommonDeps{})
	ai.SetPluginMentions([]chatPluginMention{{ID: "notes", Name: "Notes"}})
	state := &chatPreviewState{active: true, panel: chatMentionPanel, editor: woxui.NewTextEditor("@"), key: "mentions"}
	app := bindTestApp(&App{aiSettings: ai, chat: chatController{chatPreview: state}})
	for _, step := range []struct {
		shift bool
		want  string
	}{
		{false, chatMentionKindFiles}, {false, chatMentionKindPlugin}, {false, ""},
		{true, chatMentionKindPlugin}, {true, chatMentionKindFiles}, {true, ""},
	} {
		event := woxui.KeyEvent{Key: woxui.KeyTab, Down: true}
		if step.shift {
			event.Modifiers = woxui.KeyModifierShift
		}
		if !app.chat.handleChatKey(event, true) || state.panelFilter != step.want {
			t.Fatalf("mention Tab: got %q, want %q", state.panelFilter, step.want)
		}
		items := app.chat.filteredChatMentionItems(app.chat.chatMentionCatalog(), "")
		for _, item := range items {
			if step.want != "" && item.group != step.want {
				t.Fatalf("category %q contains %+v", step.want, item)
			}
		}
		if len(items) == 0 || state.editor.State().Text != "@" {
			t.Fatal("category switching lost items or changed the input")
		}
	}
	snapshot := snapshotChatPreviewLocked(state)
	filters := app.chatCatalogFilters(snapshot)
	filters[2].OnSelect()
	items := app.chat.filteredChatMentionItems(app.chat.chatMentionCatalog(), "")
	if len(items) != 1 || items[0].title != "Notes" || items[0].sourceIndex != 2 {
		t.Fatalf("plugin tab lost the original source index: %+v", items)
	}
	app.chat.setChatText("@no")
	if state.panelFilter != chatMentionKindPlugin {
		t.Fatal("typing reset the mention category")
	}
	app.chat.activateChatPanelSelection()
	if state.editor.State().Text == "@no" {
		t.Fatal("keyboard activation did not insert the filtered plugin")
	}
	app.chat.setChatText("@")
	if state.panelFilter != "" {
		t.Fatal("a fresh mention picker did not reset to All")
	}
}

func TestChatProjectSkillCatalogUsesCurrentProject(t *testing.T) {
	ai := newAISettingsController(CommonDeps{Translate: func(s string) string { return s }})
	ai.SetSkills([]chatSkill{{ID: "global", Name: "review", Source: "user"}, {ID: "global-2", Name: "other", Source: "user"}})
	state := &chatPreviewState{chat: chatData{ProjectID: "p"}, projectSkillsProjectID: "p", projectSkills: []chatSkill{{ID: "project", Name: "review", Source: "project", Enabled: true}}, editor: woxui.NewTextEditor("/")}
	app := bindTestApp(&App{aiSettings: ai, chat: chatController{chatPreview: state}})
	skills := app.chat.chatSkills()
	if len(skills) != 2 || skills[0].ID != "project" || skills[1].ID != "global-2" {
		t.Fatalf("merged skills = %+v", skills)
	}
	refs := chatSkillRefsFromText("{skill:review}", skills)
	if len(refs) != 1 || refs[0].ID != "project" || refs[0].Source != "project" {
		t.Fatalf("skill ref = %+v", refs)
	}
	state.chat.ProjectID = "other-project"
	skills = app.chat.chatSkills()
	if len(skills) != 2 || skills[0].ID != "global" {
		t.Fatalf("stale project skills leaked: %+v", skills)
	}
}

func TestChatSlashFuzzyMatchAndSelection(t *testing.T) {
	ai := newAISettingsController(CommonDeps{Translate: func(s string) string { return s }})
	ai.SetSkills([]chatSkill{{ID: "commit", Name: "wox-commit", Source: "project"}, {ID: "other", Name: "design"}})
	state := &chatPreviewState{key: "fuzzy", active: true, editor: woxui.NewTextEditor(""), projects: chatProjectUI{Loaded: true}}
	app := bindTestApp(&App{aiSettings: ai, chat: chatController{chatPreview: state}})
	app.chat.setChatText("/wc")
	snapshot := snapshotChatPreviewLocked(state)
	app.chat.attachChatPreviewCatalogs(snapshot)
	displayed := chatSnapshotCommandItems(snapshot)
	keyboard := app.chat.commandPaletteItems()
	if len(displayed) != 1 || displayed[0].title != "wox-commit" || len(keyboard) != 1 || keyboard[0].sourceIndex != displayed[0].sourceIndex {
		t.Fatalf("fuzzy results differ: displayed=%+v keyboard=%+v", displayed, keyboard)
	}
	app.chat.activateChatPanelSelection()
	if got := state.editor.State().Text; got != "{skill:wox-commit} " {
		t.Fatalf("selected text = %q", got)
	}
}

func TestChatCatalogFuzzyMatchingAcrossGroups(t *testing.T) {
	models := []aiModel{{Name: "gpt-6-astra", Provider: "openai"}}
	skills := []chatSkill{{Name: "微信技能"}}
	projects := chatProjectUI{Loaded: true, Projects: []common.AIChatProject{{Id: "p", Name: "微信项目"}}}
	cases := []struct {
		query, filter string
		pinyin        bool
		want          string
	}{
		{"g6a", "models", false, "gpt-6-astra"},
		{"wxxm", chatProjectPanel, true, "微信项目"},
		{"wxjn", "skills", true, "微信技能"},
		{"wxxm", chatProjectPanel, false, ""},
		{"np", chatProjectPanel, false, "No project"},
	}
	for _, tc := range cases {
		t.Run(tc.query+tc.filter+fmt.Sprint(tc.pinyin), func(t *testing.T) {
			items := chatSlashPaletteItems(models, skills, aiModel{}, projects, "", "No project", tc.query, tc.filter, tc.pinyin)
			if tc.want == "" && len(items) == 0 {
				return
			}
			if len(items) != 1 || items[0].title != tc.want {
				t.Fatalf("search results = %+v, want %q", items, tc.want)
			}
		})
	}
}

func TestChatSearchRanksAllGroupsAndProjectUsage(t *testing.T) {
	models := []aiModel{{Name: "commit-model", Provider: "provider"}}
	skills := []chatSkill{{Name: "commit"}, {Name: "commit-helper"}}
	projects := chatProjectUI{Loaded: true, Projects: []common.AIChatProject{{Id: "a", Name: "commit-project"}}}
	items := chatSlashPaletteItems(models, skills, aiModel{}, projects, "", "No project", "commit", chatCommandPanel)
	if len(items) != 4 || items[0].group != "skills" || items[0].title != "commit" {
		t.Fatalf("exact skill should lead mixed results: %+v", items)
	}
	for i := 1; i < len(items); i++ {
		if items[i-1].score < items[i].score {
			t.Fatalf("results out of score order: %+v", items)
		}
	}
	projects.Projects = []common.AIChatProject{{Id: "rare", Name: "wox-team"}, {Id: "popular", Name: "wox-team", SelectionCount: 10}, {Id: "exact", Name: "wox"}, {Id: "unrelated", Name: "other", SelectionCount: 1000000}}
	items = chatSlashPaletteItems(nil, nil, aiModel{}, projects, "", "No project", "wox", chatProjectPanel)
	if len(items) != 3 || items[0].title != "wox" || items[1].sourceIndex != 1 || items[2].sourceIndex != 0 {
		t.Fatalf("usage should break close matches without overwhelming exact results: %+v", items)
	}
	empty := chatProjectPaletteItems(projects.Projects, "", "", "No project")
	if len(empty) != 5 || empty[1].sourceIndex != 0 || empty[2].sourceIndex != 1 {
		t.Fatalf("empty search changed browsing order: %+v", empty)
	}
}

func TestChatRankedSectionsPreserveHeaderGeometry(t *testing.T) {
	items := []chatCommandPaletteItem{{group: "skills"}, {group: "models", subtitle: "first"}, {group: chatProjectPanel}, {group: "models", subtitle: "second"}}
	// Each transition starts a section; models also have a provider header.
	want := []float32{28, 28 + 50 + 12 + 56, 28 + 50 + 12 + 56 + 38 + 12 + 28, 28 + 50 + 12 + 56 + 38 + 12 + 28 + 38 + 12 + 56}
	for i := range items {
		if got := chatCommandItemOffset(items, i); got != want[i] {
			t.Fatalf("row %d offset = %g, want %g", i, got, want[i])
		}
	}
}

func TestChatRankedResultsMatchKeyboardSelection(t *testing.T) {
	ai := newAISettingsController(CommonDeps{Translate: func(s string) string { return s }})
	ai.SetModels([]aiModel{{Name: "commit-model", Provider: "provider"}})
	ai.SetSkills([]chatSkill{{Name: "commit", ID: "exact"}, {Name: "commit-helper", ID: "other"}})
	state := &chatPreviewState{key: "ranked", panel: chatCommandPanel, panelQuery: "commit", editor: woxui.NewTextEditor("/commit"), projects: chatProjectUI{Loaded: true, Projects: []common.AIChatProject{{Id: "project", Name: "commit-project"}}}}
	app := bindTestApp(&App{aiSettings: ai, chat: chatController{chatPreview: state}})
	snapshot := snapshotChatPreviewLocked(state)
	app.chat.attachChatPreviewCatalogs(snapshot)
	displayed := chatSnapshotCommandItems(snapshot)
	keyboard := app.chat.commandPaletteItems()
	if len(displayed) == 0 || len(keyboard) != len(displayed) || displayed[0].group != "skills" || displayed[0].sourceIndex != keyboard[0].sourceIndex {
		t.Fatalf("display/keyboard ranking mismatch: %+v / %+v", displayed, keyboard)
	}
	app.chat.activateChatPanelSelection()
	if got := state.editor.State().Text; got != "{skill:commit} " {
		t.Fatalf("selected ranked result = %q", got)
	}
}
