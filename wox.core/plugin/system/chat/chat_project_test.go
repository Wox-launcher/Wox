package chat

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"wox/ai"
	"wox/common"
	"wox/database"
	"wox/util"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDefaultChatProjectPersistsAndIgnoresDeletedProjects(t *testing.T) {
	ctx := context.Background()
	r := newChatStorageTestPlugin(t)
	project, err := r.SaveChatProject(ctx, common.AIChatProject{Name: "Project", Directories: []string{t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetDefaultChatProject(ctx, project.Id); err != nil {
		t.Fatal(err)
	}
	restarted := &AIChatPlugin{store: r.store, api: r.api}
	if got := restarted.newChatData(ctx).ProjectId; got != project.Id {
		t.Fatalf("new chat project = %q, want %q", got, project.Id)
	}
	if err := r.SetDefaultChatProject(ctx, "missing"); err == nil {
		t.Fatal("accepted a missing project")
	}
	if got := restarted.defaultChatProject(ctx); got != project.Id {
		t.Fatalf("invalid selection changed default to %q", got)
	}
	if err := r.SetDefaultChatProject(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if got := restarted.newChatData(ctx).ProjectId; got != "" {
		t.Fatalf("no-project choice was not saved: %q", got)
	}
	if err := r.SetDefaultChatProject(ctx, project.Id); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteChatProject(ctx, project.Id); err != nil {
		t.Fatal(err)
	}
	if got := restarted.newChatData(ctx).ProjectId; got != "" {
		t.Fatalf("deleted project remained the default: %q", got)
	}
}

func TestChatProjectsPersistLocalDirectoriesAndPreserveChats(t *testing.T) {
	ctx := context.Background()
	r := newChatStorageTestPlugin(t)
	one, two := t.TempDir(), t.TempDir()
	p, err := r.SaveChatProject(ctx, common.AIChatProject{Name: "Example", Directories: []string{one, two, one}, PrimaryDirectory: two})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Directories) != 2 || p.PrimaryDirectory != two {
		t.Fatalf("%+v", p)
	}
	raw, err := r.store.ListByPrefix(chatProjectPrefix)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw[chatProjectPrefix+p.Id], one) || strings.Contains(raw[chatProjectPrefix+p.Id], "Directories") {
		t.Fatal("local paths leaked into synced project")
	}
	r.saveChat(ctx, common.AIChatData{Id: "chat", ProjectId: p.Id, Conversations: []common.Conversation{{Text: "keep"}}})
	if r.chatSummaries()[0].ProjectId != p.Id {
		t.Fatal("summary lost project")
	}
	restarted := &AIChatPlugin{store: r.store, api: r.api}
	projects, err := restarted.ListChatProjects(ctx)
	if err != nil || len(projects) != 1 || projects[0].PrimaryDirectory != two {
		t.Fatalf("%+v %v", projects, err)
	}
	if err := r.MoveChatToProject(ctx, "chat", ""); err != nil {
		t.Fatal(err)
	}
	// Late title/stream snapshots must not restore the previous project.
	r.saveChat(ctx, common.AIChatData{Id: "chat", ProjectId: p.Id, Conversations: []common.Conversation{{Text: "keep"}}})
	chat, _ := r.GetChat(ctx, "chat")
	if chat.ProjectId != "" {
		t.Fatal("old snapshot restored membership")
	}
	if err := r.MoveChatToProject(ctx, "chat", p.Id); err != nil {
		t.Fatal(err)
	}
	r.activeChatCancels.Store("chat", &activeAIChatCancel{cancel: func() {}})
	if err := r.DeleteChatProject(ctx, p.Id); err == nil {
		t.Fatal("deleted streaming project")
	}
	if err := r.MoveChatToProject(ctx, "chat", ""); err == nil {
		t.Fatal("moved streaming chat")
	}
	r.activeChatCancels.Delete("chat")
	if err := r.DeleteChatProject(ctx, p.Id); err != nil {
		t.Fatal(err)
	}
	chat, ok := r.GetChat(ctx, "chat")
	if !ok || chat.ProjectId != "" || len(chat.Conversations) != 1 || chat.Conversations[0].Text != "keep" {
		t.Fatalf("%+v", chat)
	}
	if _, err := os.Stat(one); err != nil {
		t.Fatal("deleted source directory")
	}
}

func TestProjectContextIsPerTurnAndMissingBindingsDoNotFallback(t *testing.T) {
	ctx := context.Background()
	r := newChatStorageTestPlugin(t)
	one, two := t.TempDir(), t.TempDir()
	p, err := r.SaveChatProject(ctx, common.AIChatProject{Name: "Example", Directories: []string{one, two}})
	if err != nil {
		t.Fatal(err)
	}
	chat := common.AIChatData{Id: "chat", ProjectId: p.Id}
	frozen, err := r.projectRequestContext(ctx, &chat)
	if err != nil {
		t.Fatal(err)
	}
	p.PrimaryDirectory = two
	if _, err := r.SaveChatProject(ctx, p); err != nil {
		t.Fatal(err)
	}
	next, err := r.projectRequestContext(ctx, &chat)
	if err != nil {
		t.Fatal(err)
	}
	if common.AIWorkingDirectory(frozen) != one || common.AIWorkingDirectory(next) != two {
		t.Fatal("turn context was not isolated")
	}
	prompt := projectRuntimePrompt(frozen)
	// JSON quoting of Windows backslashes matches the runtime prompt.
	for _, path := range []string{one, two} {
		quoted, _ := json.Marshal(path)
		if !strings.Contains(prompt, string(quoted)) {
			t.Fatalf("missing directory: %s", prompt)
		}
	}
	if err := r.store.DeleteWithSync(chatProjectLocalPrefix+p.Id, false); err != nil {
		t.Fatal(err)
	}
	if _, err := r.projectRequestContext(ctx, &chat); err == nil {
		t.Fatal("missing binding silently fell back")
	}
	if err := r.store.SetWithSync(chatProjectLocalPrefix+p.Id, chatProjectDirectories{[]string{one}, filepath.Join(one, "missing")}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := r.projectRequestContext(ctx, &chat); err == nil {
		t.Fatal("missing primary silently fell back")
	}
	chat.ProjectId = "deleted"
	if _, err := r.projectRequestContext(ctx, &chat); err != nil || chat.ProjectId != "" {
		t.Fatal("orphan chat was not kept as ordinary")
	}
}

func TestProjectSaveRejectsInvalidDirectoryWithoutPartialMetadata(t *testing.T) {
	r := newChatStorageTestPlugin(t)
	if _, err := r.SaveChatProject(context.Background(), common.AIChatProject{Name: "bad", Directories: []string{filepath.Join(t.TempDir(), "missing")}}); err == nil {
		t.Fatal("accepted missing directory")
	}
	values, err := r.store.ListByPrefix(chatProjectPrefix)
	if err != nil || len(values) != 0 {
		t.Fatalf("%v %v", values, err)
	}
}

// TestRemoteProjectDeletionCannotBeUndoneByStaleSaves models sync during a stream/editor.
func TestRemoteProjectDeletionCannotBeUndoneByStaleSaves(t *testing.T) {
	ctx := context.Background()
	r := newChatStorageTestPlugin(t)
	project, err := r.SaveChatProject(ctx, common.AIChatProject{Name: "Remote", Directories: []string{t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	chat := common.AIChatData{Id: "stream", ProjectId: project.Id, Conversations: []common.Conversation{{Text: "answer"}}}
	r.saveChat(ctx, chat)
	frozen, err := r.projectRequestContext(ctx, &chat)
	if err != nil {
		t.Fatal(err)
	}
	// Remote project and chat records can arrive independently and in either order.
	if err := r.store.Delete(chatProjectPrefix + project.Id); err != nil {
		t.Fatal(err)
	}
	if _, err := r.SaveChatProject(ctx, project); err == nil {
		t.Fatal("stale editor recreated remotely deleted project")
	}
	r.saveChat(ctx, chat)
	stored, ok := r.GetChat(ctx, chat.Id)
	if !ok || stored.ProjectId != "" || stored.Conversations[0].Text != "answer" {
		t.Fatalf("late stream save: %+v", stored)
	}
	if common.AIWorkingDirectory(frozen) != project.PrimaryDirectory || !strings.Contains(projectRuntimePrompt(frozen), "Remote") {
		t.Fatal("remote deletion changed the active turn snapshot")
	}
	projects, err := r.ListChatProjects(ctx)
	if err != nil || len(projects) != 0 {
		t.Fatalf("deleted project reappeared: %+v %v", projects, err)
	}
}

func TestProjectMembershipChangesPreserveUnknownChatFields(t *testing.T) {
	ctx := context.Background()
	r := newChatStorageTestPlugin(t)
	p, err := r.SaveChatProject(ctx, common.AIChatProject{Name: "Project", Directories: []string{t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.store.SetWithSync(chatSettingKey("future"), `{"Id":"future","FutureField":{"keep":true},"Conversations":[{"Text":"hello"}]}`, true); err != nil {
		t.Fatal(err)
	}
	if err := r.MoveChatToProject(ctx, "future", p.Id); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteChatProject(ctx, p.Id); err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := r.store.Get(chatSettingKey("future"), &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["FutureField"]) != `{"keep":true}` {
		t.Fatalf("lost unknown field: %s", fields["FutureField"])
	}
}

// TestProjectPromptSurvivesCompactionWithoutSharingOtherChats covers provider-facing context.
func TestProjectPromptSurvivesCompactionWithoutSharingOtherChats(t *testing.T) {
	// Runtime tool discovery uses global settings; isolate its database and handles.
	if os.Getenv("WOX_PROJECT_CONTEXT_TEST_CHILD") == "" {
		t.Setenv(util.TestWoxDataDirEnv, t.TempDir())
		t.Setenv(util.TestUserDataDirEnv, t.TempDir())
		t.Setenv("WOX_PROJECT_CONTEXT_TEST_CHILD", "1")
		command := exec.Command(os.Args[0], "-test.run=^TestProjectPromptSurvivesCompactionWithoutSharingOtherChats$")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("project runtime context: %v\n%s", err, output)
		}
		return
	}
	if err := util.GetLocation().Init(); err != nil {
		t.Fatal(err)
	}
	if err := database.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	r := newChatStorageTestPlugin(t)
	ctx := context.Background()
	p, err := r.SaveChatProject(ctx, common.AIChatProject{Name: "Example", Directories: []string{t.TempDir(), t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	r.saveChat(ctx, common.AIChatData{Id: "other", ProjectId: p.Id, Conversations: []common.Conversation{{Id: "secret", Role: common.ConversationRoleUser, Text: "other conversation must stay private"}}})
	chat := common.AIChatData{Id: "active", ProjectId: p.Id, Conversations: []common.Conversation{
		{Id: "old", Role: common.ConversationRoleUser, Text: "compacted question"},
		{Id: "new", Role: common.ConversationRoleUser, Text: "current question"},
	}}
	frozen, err := r.projectRequestContext(ctx, &chat)
	if err != nil {
		t.Fatal(err)
	}
	entry := &common.AIChatCompactionEntry{Summary: "Previous facts", FirstKeptConversationId: "new"}
	conversations := r.composeRuntimeConversations(frozen, chat, entry, false)
	foundProject, foundSummary, foundQuestion := false, false, false
	for _, c := range conversations {
		if strings.Contains(c.Text, "other conversation must stay private") || c.Text == "compacted question" {
			t.Fatalf("unexpected conversation content: %q", c.Text)
		}
		foundProject = foundProject || strings.Contains(c.Text, "Primary working directory:")
		foundSummary = foundSummary || strings.Contains(c.Text, "Previous facts")
		foundQuestion = foundQuestion || c.Text == "current question"
	}
	if !foundProject || !foundSummary || !foundQuestion {
		t.Fatalf("incomplete provider context: %+v", conversations)
	}
	if len(chat.Conversations) != 2 || chat.Conversations[0].Text != "compacted question" {
		t.Fatal("runtime prompt mutated persisted messages")
	}
}

func TestProjectResourcesLoadInstructionsAndSkillsPerTurn(t *testing.T) {
	root, other := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("Use project conventions"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "agents.md"), []byte("Use secondary conventions"), 0644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, ".agents", "skills", "local")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: local-project-skill\ndescription: Local project skill\n---\nUse local skill"), 0644); err != nil {
		t.Fatal(err)
	}
	plugin := &AIChatPlugin{}
	project := common.AIChatProject{Name: "Local", PrimaryDirectory: root, Directories: []string{root, other}}
	ctx := context.WithValue(context.Background(), chatProjectContextKey{}, project)
	prepared := plugin.prepareProjectResources(ctx)
	prompt := projectRuntimePrompt(prepared)
	for _, value := range []string{"Use project conventions", "Use secondary conventions", "user's explicit request take precedence", "closer AGENTS.md"} {
		if !strings.Contains(prompt, value) {
			t.Fatalf("missing %q in %q", value, prompt)
		}
	}
	skills := ai.SkillsForContext(prepared)
	var projectSkill common.Skill
	for _, skill := range skills {
		if skill.Name == "local-project-skill" && skill.Source == "project" {
			projectSkill = skill
		}
	}
	if projectSkill.Id == "" {
		t.Fatal("project skill missing from runtime context")
	}
	message := common.Conversation{Role: common.ConversationRoleUser, Text: "{skill:local-project-skill} review", SkillRefs: []common.AISkillRef{ai.SkillRefFromSkill(projectSkill)}}
	expanded := plugin.withMessageSkillReferences(prepared, message)
	if !strings.Contains(expanded.Text, "Use local skill") || strings.Contains(expanded.Text, "{skill:") {
		t.Fatalf("project skill not expanded: %s", expanded.Text)
	}
	noProject := plugin.prepareProjectResources(context.Background())
	if projectRuntimePrompt(noProject) != "" {
		t.Fatal("project instructions leaked to ordinary chat")
	}
	for _, skill := range ai.SkillsForContext(noProject) {
		if skill.Name == "local-project-skill" && skill.Source == "project" {
			t.Fatal("project skill leaked to ordinary chat")
		}
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("Updated conventions"), 0644); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(projectRuntimePrompt(plugin.prepareProjectResources(ctx)), "Updated conventions") || strings.Contains(projectRuntimePrompt(prepared), "Updated conventions") {
		t.Fatal("instructions not frozen per turn")
	}
}

func TestProjectSelectionUsagePersistsAndIsConcurrentSafe(t *testing.T) {
	ctx := context.Background()
	r := newChatStorageTestPlugin(t)
	p, err := r.SaveChatProject(ctx, common.AIChatProject{Name: "Usage", Directories: []string{t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	const selections = 12
	errors := make(chan error, selections)
	for i := 0; i < selections; i++ {
		go func() { errors <- r.RecordProjectSelection(ctx, p.Id) }()
	}
	for i := 0; i < selections; i++ {
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
	}
	restarted := &AIChatPlugin{store: r.store, api: r.api}
	projects, err := restarted.ListChatProjects(ctx)
	if err != nil || len(projects) != 1 || projects[0].SelectionCount != selections {
		t.Fatalf("selection counts: %+v, %v", projects, err)
	}
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var usage database.PluginSetting
	if err := db.Where("plugin_id = ? AND key = ?", common.AIChatPluginID, chatProjectUsagePrefix+p.Id).First(&usage).Error; err != nil || !usage.IsLocal {
		t.Fatalf("usage must remain local: %+v, %v", usage, err)
	}
	var oplogs int64
	if err := db.Model(&database.Oplog{}).Where("key = ?", chatProjectUsagePrefix+p.Id).Count(&oplogs).Error; err != nil || oplogs != 0 {
		t.Fatalf("usage unexpectedly entered sync log: %d, %v", oplogs, err)
	}
	if err := r.DeleteChatProject(ctx, p.Id); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := r.store.Get(chatProjectUsagePrefix+p.Id, &count); err == nil {
		t.Fatal("deleted project's usage was retained")
	}
	if err := r.RecordProjectSelection(ctx, p.Id); err == nil {
		t.Fatal("recorded selection of deleted project")
	}
}
