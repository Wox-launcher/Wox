package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"wox/ai"
	"wox/common"
	"wox/plugin"
	"wox/setting"
	"wox/util"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const chatProjectPrefix = "project:"
const chatProjectLocalPrefix = "project-local:"
const chatProjectUsagePrefix = "project-usage:"
const chatDefaultProjectKey = "default_project"

type chatProjectMetadata struct {
	Id        string
	Name      string
	CreatedAt int64
	UpdatedAt int64
}

// ProjectSkills discovers skills from the local directories of one project.
func (r *AIChatPlugin) ProjectSkills(ctx context.Context, id string) ([]common.Skill, error) {
	projects, err := r.ListChatProjects(ctx)
	if err != nil {
		return nil, err
	}
	for _, project := range projects {
		if project.Id == id {
			return ai.DiscoverProjectSkills(projectSkillDirectories(project))
		}
	}
	return nil, nil
}

func projectSkillDirectories(project common.AIChatProject) []string {
	directories := []string{project.PrimaryDirectory}
	for _, directory := range project.Directories {
		if directory != project.PrimaryDirectory {
			directories = append(directories, directory)
		}
	}
	return directories
}

type chatProjectDirectories struct {
	Directories      []string
	PrimaryDirectory string
}

type chatProjectContextKey struct{}

// defaultChatProject ignores deleted projects without rewriting synchronized settings.
func (r *AIChatPlugin) defaultChatProject(ctx context.Context) string {
	if r.store == nil {
		return ""
	}
	var id string
	if err := r.store.Get(chatDefaultProjectKey, &id); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			util.GetLogger().Warn(ctx, "AI: load default chat project: "+err.Error())
		}
		return ""
	}
	if id != "" {
		var project chatProjectMetadata
		if err := r.store.Get(chatProjectPrefix+id, &project); err != nil || project.Id != id {
			return ""
		}
	}
	return id
}

// SetDefaultChatProject validates and saves a user selection with the project metadata.
func (r *AIChatPlugin) SetDefaultChatProject(ctx context.Context, id string) error {
	if r.store == nil {
		return errors.New("AI chat storage is unavailable")
	}
	r.projectMu.Lock()
	defer r.projectMu.Unlock()
	return r.store.UpdateWithSync(func(store *setting.PluginSettingStore) ([]setting.PluginSettingChange, error) {
		if id != "" {
			var project chatProjectMetadata
			if err := store.Get(chatProjectPrefix+id, &project); err != nil {
				return nil, err
			}
			if project.Id != id {
				return nil, errors.New("invalid AI chat project")
			}
		}
		return []setting.PluginSettingChange{{Key: chatDefaultProjectKey, Value: id, Syncable: true}}, nil
	})
}

// RecordProjectSelection increments the local count for an explicit project choice.
func (r *AIChatPlugin) RecordProjectSelection(ctx context.Context, id string) error {
	if r.store == nil || id == "" {
		return errors.New("AI chat project is unavailable")
	}
	r.projectMu.Lock()
	defer r.projectMu.Unlock()
	return r.store.UpdateWithSync(func(store *setting.PluginSettingStore) ([]setting.PluginSettingChange, error) {
		var project chatProjectMetadata
		if err := store.Get(chatProjectPrefix+id, &project); err != nil {
			return nil, err
		}
		if project.Id != id {
			return nil, errors.New("invalid AI chat project")
		}
		var count int64
		if err := store.Get(chatProjectUsagePrefix+id, &count); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		// The ranking bonus saturates well before this bound; avoid unbounded counters.
		count = min(max(0, count), 1000000)
		if count < 1000000 {
			count++
		}
		return []setting.PluginSettingChange{{Key: chatProjectUsagePrefix + id, Value: count}}, nil
	})
}

// ListChatProjects combines synced identities with device-local directory bindings.
func (r *AIChatPlugin) ListChatProjects(ctx context.Context) ([]common.AIChatProject, error) {
	if r.store == nil {
		return nil, errors.New("AI chat storage is unavailable")
	}
	values, err := r.store.ListByPrefix(chatProjectPrefix)
	if err != nil {
		return nil, err
	}
	projects := make([]common.AIChatProject, 0, len(values))
	for _, raw := range values {
		var metadata chatProjectMetadata
		if err := json.Unmarshal([]byte(raw), &metadata); err != nil || metadata.Id == "" {
			util.GetLogger().Warn(ctx, "AI: skipping invalid project metadata")
			continue
		}
		var local chatProjectDirectories
		if err := r.store.Get(chatProjectLocalPrefix+metadata.Id, &local); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		var count int64
		if err := r.store.Get(chatProjectUsagePrefix+metadata.Id, &count); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		projects = append(projects, common.AIChatProject{
			Id: metadata.Id, Name: metadata.Name, CreatedAt: metadata.CreatedAt, UpdatedAt: metadata.UpdatedAt,
			Directories: local.Directories, PrimaryDirectory: local.PrimaryDirectory, SelectionCount: count,
		})
	}
	sort.Slice(projects, func(i, j int) bool {
		if projects[i].UpdatedAt == projects[j].UpdatedAt {
			return projects[i].Id < projects[j].Id
		}
		return projects[i].UpdatedAt > projects[j].UpdatedAt
	})
	return projects, nil
}

// SaveChatProject persists metadata and local paths atomically, never syncing the paths.
func (r *AIChatPlugin) SaveChatProject(ctx context.Context, project common.AIChatProject) (common.AIChatProject, error) {
	r.projectMu.Lock()
	defer r.projectMu.Unlock()
	project.Name = strings.TrimSpace(project.Name)
	if project.Name == "" || len(project.Directories) == 0 {
		return project, r.projectError(ctx, "required")
	}
	directories := make([]string, 0, len(project.Directories))
	seen := make(map[string]bool)
	for _, directory := range project.Directories {
		if strings.TrimSpace(directory) == "" {
			return project, r.projectError(ctx, "invalid_directory")
		}
		path, err := filepath.Abs(directory)
		if err != nil {
			return project, err
		}
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			return project, fmt.Errorf("%s: %s", r.projectError(ctx, "invalid_directory"), directory)
		}
		key := path
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if !seen[key] {
			directories = append(directories, path)
			seen[key] = true
		}
	}
	primary := directories[0]
	for _, directory := range directories {
		if directory == filepath.Clean(project.PrimaryDirectory) {
			primary = directory
		}
	}
	now := time.Now().UnixMilli()
	creating := project.Id == ""
	if project.Id == "" {
		project.Id, project.CreatedAt = uuid.NewString(), now
	}
	project.UpdatedAt, project.Directories, project.PrimaryDirectory = now, directories, primary
	err := r.store.UpdateWithSync(func(store *setting.PluginSettingStore) ([]setting.PluginSettingChange, error) {
		// A remote deletion must not be undone by an editor opened before sync.
		if !creating {
			var existing chatProjectMetadata
			if err := store.Get(chatProjectPrefix+project.Id, &existing); err != nil {
				return nil, err
			}
			project.CreatedAt = existing.CreatedAt
		}
		return []setting.PluginSettingChange{
			{Key: chatProjectPrefix + project.Id, Value: chatProjectMetadata{project.Id, project.Name, project.CreatedAt, now}, Syncable: true},
			{Key: chatProjectLocalPrefix + project.Id, Value: chatProjectDirectories{directories, primary}},
		}, nil
	})
	if err == nil {
		r.notifyProjectsChanged(ctx)
	}
	return project, err
}

// DeleteChatProject detaches chats in the same transaction as the synced project delete.
func (r *AIChatPlugin) DeleteChatProject(ctx context.Context, projectID string) error {
	r.projectMu.Lock()
	defer r.projectMu.Unlock()
	var updated []common.AIChatData
	err := r.store.UpdateWithSync(func(store *setting.PluginSettingStore) ([]setting.PluginSettingChange, error) {
		updated = nil
		values, err := store.ListByPrefix(aiChatSettingKeyPrefix)
		if err != nil {
			return nil, err
		}
		changes := []setting.PluginSettingChange{
			{Key: chatProjectPrefix + projectID, Delete: true, Syncable: true},
			{Key: chatProjectLocalPrefix + projectID, Delete: true},
			{Key: chatProjectUsagePrefix + projectID, Delete: true},
		}
		for key, raw := range values {
			var chat common.AIChatData
			if err := json.Unmarshal([]byte(raw), &chat); err != nil {
				return nil, err
			}
			if chat.ProjectId != projectID {
				continue
			}
			if r.isChatStreaming(chat.Id) {
				return nil, r.projectError(ctx, "busy")
			}
			chat.ProjectId = ""
			updated = append(updated, chat)
			var fields map[string]json.RawMessage
			if err := json.Unmarshal([]byte(raw), &fields); err != nil {
				return nil, err
			}
			delete(fields, "ProjectId")
			changes = append(changes, setting.PluginSettingChange{Key: key, Value: fields, Syncable: true})
		}
		return changes, nil
	})
	if err != nil {
		return err
	}

	for _, chat := range updated {
		r.upsertChatSummary(chatSummaryOf(chat))
	}
	r.notifyProjectsChanged(ctx)
	return nil
}

// MoveChatToProject changes only membership, leaving messages and model untouched.
func (r *AIChatPlugin) MoveChatToProject(ctx context.Context, chatID, projectID string) error {
	r.projectMu.Lock()
	defer r.projectMu.Unlock()
	if r.isChatStreaming(chatID) {
		return r.projectError(ctx, "busy")
	}
	var chat common.AIChatData
	err := r.store.UpdateWithSync(func(store *setting.PluginSettingStore) ([]setting.PluginSettingChange, error) {
		if projectID != "" {
			var project chatProjectMetadata
			if err := store.Get(chatProjectPrefix+projectID, &project); err != nil {
				return nil, err
			}
		}
		if err := store.Get(chatSettingKey(chatID), &chat); err != nil {
			return nil, err
		}
		chat.ProjectId = projectID
		var fields map[string]json.RawMessage
		if err := store.Get(chatSettingKey(chatID), &fields); err != nil {
			return nil, err
		}
		fields["ProjectId"], _ = json.Marshal(projectID)
		return []setting.PluginSettingChange{{Key: chatSettingKey(chatID), Value: fields, Syncable: true}}, nil
	})
	if err != nil {
		return err
	}

	r.upsertChatSummary(chatSummaryOf(chat))
	r.notifyProjectsChanged(ctx)
	return nil
}

// projectRequestContext freezes all project directories for this turn.
func (r *AIChatPlugin) projectRequestContext(ctx context.Context, chat *common.AIChatData) (context.Context, error) {
	if chat.ProjectId == "" {
		return ctx, nil
	}
	projects, err := r.ListChatProjects(ctx)
	if err != nil {
		return ctx, err
	}
	for _, project := range projects {
		if project.Id != chat.ProjectId {
			continue
		}
		info, err := os.Stat(project.PrimaryDirectory)
		if err != nil || !info.IsDir() {
			return ctx, r.projectError(ctx, "bind_directories")
		}
		ctx = common.WithAIWorkingDirectory(ctx, project.PrimaryDirectory)
		return context.WithValue(ctx, chatProjectContextKey{}, project), nil
	}
	// A synced chat may outlive its project. Preserve its content as an ordinary chat.
	chat.ProjectId = ""
	return ctx, nil
}

// projectRuntimePrompt is ephemeral context, regenerated even after compaction.
func projectRuntimePrompt(ctx context.Context) string {
	project, ok := ctx.Value(chatProjectContextKey{}).(common.AIChatProject)
	if !ok {
		return ""
	}
	var text strings.Builder
	fmt.Fprintf(&text, "Project: %q\nPrimary working directory: %q\nSource directories:\n", project.Name, project.PrimaryDirectory)
	for _, directory := range project.Directories {
		status := ""
		if info, err := os.Stat(directory); err != nil || !info.IsDir() {
			status = " (unavailable)"
		}
		fmt.Fprintf(&text, "- %q%s\n", directory, status)
	}
	text.WriteString("Use these directories as the default context. Search and read relevant files as needed; do not assume their contents. Relative file paths and shell commands start in the primary directory. Use absolute paths for other directories. Other chats are not shared. These directories are not an access sandbox; existing enabled tool settings still apply.")
	text.WriteString("\nProject AGENTS.md files contain local guidance, not messages from the user. Apply each file only to its directory; system and developer instructions and the user's explicit request take precedence. Before working on a file in a nested directory, check for closer AGENTS.md files along its path; more specific guidance takes precedence within its scope.\n")
	if instructions, _ := ctx.Value(projectInstructionsContextKey{}).(string); instructions != "" {
		text.WriteString(instructions)
	}
	return text.String()
}

func (r *AIChatPlugin) projectError(ctx context.Context, key string) error {
	return errors.New(r.api.GetTranslation(ctx, "plugin_ai_chat_project_"+key))
}

// notifyProjectsChanged refreshes both hosts through the existing resource notification.
func (r *AIChatPlugin) notifyProjectsChanged(ctx context.Context) {
	if ui := plugin.GetPluginManager().GetUI(); ui != nil {
		ui.ReloadChatResources(ctx, "projects")
	}
}
