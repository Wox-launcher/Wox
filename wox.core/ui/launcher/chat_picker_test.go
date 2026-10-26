package launcher

import (
	"fmt"
	"testing"

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
	if len(props.Items) != 2 || len(props.Filters) != 3 || !props.Filters[2].Selected || props.Search != nil {
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
				{false, "models"}, {false, "skills"}, {false, ""},
				{true, "skills"}, {true, "models"}, {true, ""},
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
