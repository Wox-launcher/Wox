package launcher

import (
	"context"
	"errors"
	"testing"
	"time"

	"wox/common"
	chatview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
)

type projectTestServices struct{ chatHistoryTestServices }

func (projectTestServices) ChatProjects(context.Context, string) ([]common.AIChatProject, error) {
	return nil, nil
}
func (projectTestServices) SaveChatProject(_ context.Context, _ string, p common.AIChatProject) (common.AIChatProject, error) {
	return p, nil
}
func (projectTestServices) DeleteChatProject(context.Context, string, string) error { return nil }
func (projectTestServices) MoveChatToProject(context.Context, string, string, string) error {
	return nil
}
func (projectTestServices) SetDefaultChatProject(context.Context, string, string) error { return nil }

type delayedProjectChoiceServices struct {
	projectTestServices
	writes  chan string
	release chan struct{}
}

func (s delayedProjectChoiceServices) SetDefaultChatProject(_ context.Context, _, id string) error {
	s.writes <- id
	<-s.release
	return nil
}

// TestChatProjectChoiceSerializesWrites covers rapid selection and reopening while a write is pending.
func TestChatProjectChoiceSerializesWrites(t *testing.T) {
	service := delayedProjectChoiceServices{writes: make(chan string, 2), release: make(chan struct{}, 2)}
	dispatched := make(chan func(), 2)
	app := New(false, nil)
	defer app.cancel()
	app.uiCall = nil
	preview := queryPreview{PreviewType: "chat", PreviewData: `{"ActiveChat":{"Id":"chat"}}`}
	result := queryResult{ID: "result", QueryID: "query"}
	if err := app.chat.activateChatPreview(result, preview); err != nil {
		t.Fatal(err)
	}
	app.chat.services = service
	app.chat.runOnUI = func(_ string, fn func()) error { dispatched <- fn; return nil }
	app.chat.chooseChatProject("first")
	if got := <-service.writes; got != "first" {
		t.Fatalf("first write = %q", got)
	}
	app.chat.chooseChatProject("second")
	app.chat.resetChatPreview()
	app.chat.services = nil
	if err := app.chat.activateChatPreview(result, preview); err != nil {
		t.Fatal(err)
	}
	app.chat.services = service
	if app.chat.chatPreview.chat.ProjectID != "second" {
		t.Fatal("reopened chat lost the pending selection")
	}
	service.release <- struct{}{}
	(<-dispatched)()
	if got := <-service.writes; got != "second" {
		t.Fatalf("last write = %q", got)
	}
	service.release <- struct{}{}
	(<-dispatched)()
	if app.chat.chatProjectChoiceSaving {
		t.Fatal("project selection remained pending")
	}
}

func TestChatProjectSwitchPreservesDraftAndModel(t *testing.T) {
	state := &chatPreviewState{
		chat:        chatData{ID: "draft", Model: aiModel{Name: "model"}},
		editor:      woxui.NewTextEditor("unsent question"),
		attachments: []common.AIChatAttachment{{ID: "file", Kind: common.AIChatAttachmentFile, URL: "example.txt"}},
		projects:    chatProjectUI{Loaded: true, Projects: []common.AIChatProject{{Id: "p", Name: "Project"}}},
	}
	app := bindTestApp(&App{services: projectTestServices{}, chat: chatController{chatPreview: state}})
	for _, id := range []string{"p", ""} {
		app.chat.chooseChatProject(id)
		if state.chat.ProjectID != id || state.editor.State().Text != "unsent question" || state.chat.Model.Name != "model" || len(state.attachments) != 1 {
			t.Fatalf("switch lost draft: %+v", state)
		}
		if state.panel == chatProjectPanel {
			t.Fatal("picker remained open")
		}
	}
	state.chat.IsStreaming = true
	app.chat.chooseChatProject("p")
	if state.chat.ProjectID != "" {
		t.Fatal("changed streaming membership")
	}
}

func TestChatProjectHistoryIncludesEmptyGroupsAndHidesCollapsedChats(t *testing.T) {
	ui := chatProjectUI{Projects: []common.AIChatProject{{Id: "empty", Name: "Empty"}, {Id: "p", Name: "Project"}}, Collapsed: map[string]bool{"p": true}}
	chats := []chatData{{ID: "chat", ProjectID: "p", IsSummary: true, UpdatedAt: 100}, {ID: "orphan", ProjectID: "deleted", IsSummary: true}, {ID: "recent", IsSummary: true}, {ID: "draft"}}
	rows := chatProjectHistoryRows(ui, chats)
	if len(rows) != 6 || rows[0].ProjectID != chatProjectsSection || rows[1].ProjectID != "p" || rows[1].ChatID != "" || rows[3].ProjectID != chatRecentsSection || !rows[3].Group || rows[4].ChatID != "orphan" || rows[5].ChatID != "recent" {
		t.Fatalf("%+v", rows)
	}
	ui.Collapsed["p"] = false
	if expanded := chatProjectHistoryRows(ui, chats); len(expanded) != 7 || expanded[2].ChatID != "chat" {
		t.Fatalf("%+v", expanded)
	}
	ui.Collapsed[chatProjectsSection] = true
	rows = chatProjectHistoryRows(ui, chats)
	if len(rows) != 4 || rows[1].ProjectID != chatRecentsSection || rows[2].ChatID != "orphan" || rows[3].ChatID != "recent" {
		t.Fatalf("collapsing projects must keep recents reachable: %+v", rows)
	}
	ui.Collapsed[chatProjectsSection] = false
	ui.Collapsed[chatRecentsSection] = true
	rows = chatProjectHistoryRows(ui, chats)
	if len(rows) != 5 || rows[2].ChatID != "chat" || rows[4].ProjectID != chatRecentsSection {
		t.Fatalf("collapsing recents must keep projects reachable: %+v", rows)
	}
}

func TestChatRecentsHistoryProps(t *testing.T) {
	state := &chatPreviewState{}
	app := bindTestApp(&App{
		chat:     chatController{chatPreview: state},
		language: languageCatalog{translations: map[string]string{"plugin_ai_chat_project_recents": "Recents"}},
	})
	snapshot := &chatPreviewSnapshot{chats: []chatData{{ID: "recent", IsSummary: true}}}
	props := app.chatProjectHistoryProps(snapshot, uiPalette{}, 240, 400)
	items := props.Items
	if props.ContentHeight != projectHistoryHeight(&chatPreviewState{chats: snapshot.chats}) {
		t.Fatalf("history content height = %v, want %v", props.ContentHeight, projectHistoryHeight(&chatPreviewState{chats: snapshot.chats}))
	}
	if len(items) != 3 || items[1].Title != "Recents" || items[1].Kind != "projects-section" || items[1].OnNew != nil || items[1].OnEdit != nil || items[1].SpacingBefore != chatview.ChatHistorySectionGap || !items[2].Indented {
		t.Fatalf("unexpected recents presentation: %+v", items)
	}
	items[1].OnSelect()
	if !state.projects.Collapsed[chatRecentsSection] {
		t.Fatal("selecting recents did not collapse it")
	}
}

func TestChatHistoryShowsActiveAndBackgroundStreams(t *testing.T) {
	app := New(false, nil)
	defer app.cancel()
	app.chat.chatPreview = &chatPreviewState{}
	snapshot := &chatPreviewSnapshot{
		projects: chatProjectUI{Projects: []common.AIChatProject{{Id: "p", Name: "Project"}}},
		chat:     chatData{ID: "active"}, sending: true,
		chats: []chatData{
			{ID: "active", IsSummary: true},
			{ID: "background", ProjectID: "p", IsSummary: true, IsStreaming: true},
			{ID: "done", IsSummary: true},
		},
	}
	check := func(want map[string]bool) {
		t.Helper()
		for _, item := range app.chatProjectHistoryProps(snapshot, uiPalette{}, 240, 400).Items {
			if expected, found := want[item.SelectID]; found && item.Streaming != expected {
				t.Fatalf("%s streaming = %v, want %v", item.SelectID, item.Streaming, expected)
			}
		}
	}
	check(map[string]bool{"chat-history-row-active": true, "chat-history-row-background": true, "chat-history-row-done": false})
	snapshot.sending = false
	snapshot.chats[1].IsStreaming = false
	check(map[string]bool{"chat-history-row-active": false, "chat-history-row-background": false, "chat-history-row-done": false})
}

type blockingStopChatServices struct {
	chatHistoryTestServices
	started chan struct{}
	release chan struct{}
}

func (s blockingStopChatServices) StopChat(context.Context, string, string) (bool, error) {
	close(s.started)
	<-s.release
	return true, nil
}

func TestStoppingBackgroundChatClearsHistoryStreaming(t *testing.T) {
	services := blockingStopChatServices{started: make(chan struct{}), release: make(chan struct{})}
	done := make(chan struct{})
	state := &chatPreviewState{
		chat:  chatData{ID: "running", IsStreaming: true},
		chats: []chatData{{ID: "running", IsSummary: true, IsStreaming: true}},
	}
	controller := &chatController{
		chatPreview: state, services: services, lifecycleCtx: context.Background(),
		invalidateChatSurfaces: func() {},
		runOnUI:                func(_ string, callback func()) error { callback(); close(done); return nil },
	}
	controller.stopChatMessage()
	<-services.started
	state.chat = chatData{ID: "another"}
	close(services.release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stop request did not complete")
	}
	if state.chats[0].IsStreaming {
		t.Fatal("stopped background conversation remains marked as streaming")
	}
}

func TestProjectHistoryKeyboardAccountsForSectionSpacing(t *testing.T) {
	state := &chatPreviewState{
		panelDensityScale: 1, panelViewport: chatview.ChatHistoryRowHeight,
		projects: chatProjectUI{Projects: []common.AIChatProject{{Id: "p", Name: "Project"}}},
		chats:    []chatData{{ID: "recent", IsSummary: true}},
	}
	rows := chatProjectHistoryRows(state.projects, state.chats)
	for _, selected := range []int{1, 2, 3} {
		state.panelSelected, state.panelScroll = selected, 0
		ensureChatPanelSelectionVisibleLocked(state, len(rows))
		want := float32(46) + float32(selected)*chatview.ChatHistoryRowHeight
		if selected >= 2 {
			want += chatview.ChatHistorySectionGap
		}
		if state.panelScroll != want {
			t.Fatalf("row %d: scroll = %v, want %v", selected, state.panelScroll, want)
		}
	}
}

// TestChatProjectPickerSharesCatalogNavigation preserves history, drafts and keyboard selection.
func TestChatProjectPickerSharesCatalogNavigation(t *testing.T) {
	state := &chatPreviewState{
		panel: "history", active: true, sidebarOpen: true,
		chat: chatData{ID: "draft", ProjectID: "a"}, editor: woxui.NewTextEditor("unsent"),
		projects: chatProjectUI{Projects: []common.AIChatProject{{Id: "a", Name: "Alpha"}, {Id: "b", Name: "Beta"}}},
	}
	app := bindTestApp(&App{services: projectTestServices{}, chat: chatController{chatPreview: state}})
	app.chat.openChatProjects()
	if state.panel != chatProjectPanel || !state.sidebarOpen || state.panelSelected != 1 || !chatOverlayPanel(state.panel) {
		t.Fatalf("picker did not preserve history or select the current project: %+v", state)
	}
	app.chat.setChatCatalogQuery("bet")
	items := app.chat.projectPaletteItems()
	if len(items) != 2 || items[1].title != "Beta" || items[0].sourceIndex != -1 {
		t.Fatalf("filtered projects must retain the no-project option: %+v", items)
	}
	app.chat.moveChatPanelSelection(1)
	app.chat.activateChatPanelSelection()
	if state.chat.ProjectID != "b" || state.panel != "history" || state.editor.State().Text != "unsent" {
		t.Fatalf("selection did not restore history and preserve draft: %+v", state)
	}
	app.chat.openChatProjects()
	app.chat.closeChatPanel()
	if state.panel != "history" || state.chat.ProjectID != "b" {
		t.Fatal("dismissal changed project membership or lost the history drawer")
	}
}

func TestChatProjectSnapshotDoesNotCaptureMutableDirectoryState(t *testing.T) {
	source := chatProjectUI{
		Projects:  []common.AIChatProject{{Id: "p", Directories: []string{"original"}}},
		Editor:    &common.AIChatProject{Directories: []string{"draft"}},
		Collapsed: map[string]bool{"p": false},
	}
	copy := cloneChatProjectUI(source)
	source.Projects[0].Directories[0] = "changed"
	source.Editor.Directories[0] = "changed"
	source.Collapsed["p"] = true
	if copy.Projects[0].Directories[0] != "original" || copy.Editor.Directories[0] != "draft" || copy.Collapsed["p"] {
		t.Fatal("snapshot shares mutable project state")
	}
}

func TestChatProjectContractPreservesMembership(t *testing.T) {
	source := common.AIChatData{Id: "chat", ProjectId: "p"}
	converted := fromCoreChatData(source)
	restored, err := chatDataToContract(converted)
	if err != nil || restored.ProjectId != "p" || chatSummary(converted).ProjectID != "p" {
		t.Fatalf("%+v %v", restored, err)
	}
}

func TestChatProjectDeleteConfirmationIsSeparateAndCancelable(t *testing.T) {
	state := &chatPreviewState{active: true, chat: chatData{ProjectID: "p"}, projects: chatProjectUI{Projects: []common.AIChatProject{{Id: "p", Name: "Project"}}}}
	app := bindTestApp(&App{services: projectTestServices{}, chat: chatController{chatPreview: state}})
	app.chat.confirmChatProjectDelete("p")
	if state.projects.DeleteID != "p" || state.projects.Editor != nil || len(state.projects.Projects) != 1 || state.chat.ProjectID != "p" {
		t.Fatal("opening confirmation changed project data or opened the editor")
	}
	if app.chat.handleChatKey(woxui.KeyEvent{Key: woxui.KeyEnter, Down: true}, true) {
		t.Fatal("composer consumed confirmation Enter")
	}
	if !app.chat.handleChatKey(woxui.KeyEvent{Key: woxui.KeyEscape, Down: true}, true) || state.projects.DeleteID != "" || len(state.projects.Projects) != 1 {
		t.Fatal("cancel must close confirmation without deleting the project")
	}
}

type deleteProjectTestServices struct {
	projectTestServices
	err error
}

func (s deleteProjectTestServices) DeleteChatProject(context.Context, string, string) error {
	return s.err
}

func TestChatProjectDeletePreservesChatsAndReportsFailures(t *testing.T) {
	for _, fail := range []bool{false, true} {
		var deleteErr error
		if fail {
			deleteErr = errors.New("delete failed")
		}
		state := &chatPreviewState{active: true, chat: chatData{ID: "chat", ProjectID: "p"}, chats: []chatData{{ID: "chat", ProjectID: "p"}}, projects: chatProjectUI{Projects: []common.AIChatProject{{Id: "p"}}, DeleteID: "p"}}
		applied := make(chan func(), 2)
		app := bindTestApp(&App{lifecycleCtx: context.Background(), services: deleteProjectTestServices{err: deleteErr}, uiCall: func(fn func()) error { applied <- fn; return nil }, chat: chatController{chatPreview: state}})
		app.chat.deleteChatProject()
		select {
		case apply := <-applied:
			apply()
		case <-time.After(5 * time.Second):
			t.Fatal("delete result was not dispatched")
		}
		if fail {
			if state.projects.DeleteID != "p" || state.projects.Error == "" || state.chat.ProjectID != "p" || state.chats[0].ProjectID != "p" {
				t.Fatal("failed deletion changed membership or dismissed confirmation")
			}
		} else if state.projects.DeleteID != "" || len(state.projects.Projects) != 0 || state.chat.ProjectID != "" || len(state.chats) != 1 || state.chats[0].ProjectID != "" {
			t.Fatal("successful deletion did not preserve and detach the conversation")
		}
	}
}

func TestChatProjectModalDoesNotSubmitComposerOnEnter(t *testing.T) {
	app := bindTestApp(&App{chat: chatController{chatPreview: &chatPreviewState{active: true, projects: chatProjectUI{Editor: &common.AIChatProject{}}}}})
	if app.chat.handleChatKey(woxui.KeyEvent{Key: woxui.KeyEnter, Down: true}, true) {
		t.Fatal("composer consumed modal Enter")
	}
}

func TestChatProjectNewChatInheritsCurrentMembership(t *testing.T) {
	state := &chatPreviewState{
		chat:   chatData{ID: "old", ProjectID: "p", Model: aiModel{Name: "model"}},
		editor: woxui.NewTextEditor("previous draft"),
	}
	app := bindTestApp(&App{lifecycleCtx: context.Background(), services: projectTestServices{}, chat: chatController{chatPreview: state}})
	app.chat.startNewChat()
	if state.chat.ID == "old" || state.chat.ProjectID != "p" || state.editor.State().Text != "" {
		t.Fatalf("new project chat: %+v", state.chat)
	}
	app.chat.chooseChatProject("")
	app.chat.startNewChat()
	if state.chat.ProjectID != "" {
		t.Fatal("ordinary new chat remembered a global project")
	}
}

// TestProjectHistoryKeyboardRevealsLastRow includes creation controls and density.
func TestProjectHistoryKeyboardRevealsLastRow(t *testing.T) {
	for _, scale := range []float32{0.9, 1, 1.1} {
		chats := make([]chatData, 18)
		for i := range chats {
			chats[i] = chatData{ID: string(rune('a' + i)), IsSummary: true}
		}
		state := &chatPreviewState{panelDensityScale: scale, panelViewport: 180, chats: chats}
		rows := chatProjectHistoryRows(state.projects, state.chats)
		state.panelSelected = len(rows) - 1
		ensureChatPanelSelectionVisibleLocked(state, len(rows))
		lastBottom := projectHistoryHeight(state)
		if state.panelScroll+state.panelViewport < lastBottom-0.01 {
			t.Fatalf("density %v: last row clipped at %v, viewport ends at %v", scale, lastBottom, state.panelScroll+state.panelViewport)
		}
	}
}

type delayedProjectSkillsServices struct {
	projectTestServices
	started chan string
	release map[string]chan struct{}
}

func (s delayedProjectSkillsServices) ChatProjectSkills(_ context.Context, _, id string) ([]common.Skill, error) {
	s.started <- id
	<-s.release[id]
	return []common.Skill{{Id: id, Name: id + "-skill", Source: "project", Enabled: true}}, nil
}

func TestChatProjectSkillDiscoveryDiscardsStaleResults(t *testing.T) {
	service := delayedProjectSkillsServices{started: make(chan string, 2), release: map[string]chan struct{}{"a": make(chan struct{}), "b": make(chan struct{})}}
	dispatched := make(chan func(), 2)
	app := New(false, service)
	defer app.cancel()
	app.uiCall = nil
	app.chat.runOnUI = func(_ string, fn func()) error { dispatched <- fn; return nil }
	state := &chatPreviewState{chat: chatData{ProjectID: "a"}}
	app.chat.chatPreview = state
	app.chat.refreshChatProjectSkills()
	if id := <-service.started; id != "a" {
		t.Fatalf("first discovery = %q", id)
	}
	state.chat.ProjectID = "b"
	app.chat.refreshChatProjectSkills()
	if id := <-service.started; id != "b" {
		t.Fatalf("second discovery = %q", id)
	}
	close(service.release["b"])
	(<-dispatched)()
	close(service.release["a"])
	(<-dispatched)()
	if len(state.projectSkills) != 1 || state.projectSkills[0].ID != "b" || state.projectSkillsLoading {
		t.Fatalf("stale discovery replaced project: %+v", state.projectSkills)
	}
}

type projectUsageTestServices struct {
	projectTestServices
	recorded chan string
}

func (s projectUsageTestServices) RecordChatProjectSelection(_ context.Context, _, id string) error {
	s.recorded <- id
	return nil
}

func TestRapidProjectSelectionsAllRecordUsage(t *testing.T) {
	service := projectUsageTestServices{recorded: make(chan string, 3)}
	app := New(false, service)
	defer app.cancel()
	app.uiCall = nil
	app.chat.runOnUI = func(string, func()) error { return nil }
	app.chat.chatPreview = &chatPreviewState{}
	app.chat.rememberChatProject("a")
	app.chat.rememberChatProject("b")
	app.chat.rememberChatProject("a")
	got := map[string]int{}
	for i := 0; i < 3; i++ {
		select {
		case id := <-service.recorded:
			got[id]++
		case <-time.After(5 * time.Second):
			t.Fatal("project selection count was lost")
		}
	}
	if got["a"] != 2 || got["b"] != 1 {
		t.Fatalf("recorded selections = %+v", got)
	}
}
