package launcher

import (
	"context"
	"math"
	"os"
	"slices"
	"sort"
	"strings"

	"wox/common"
	chatview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	"wox/util"
)

type chatProjectServices interface {
	ChatProjects(context.Context, string) ([]common.AIChatProject, error)
	SaveChatProject(context.Context, string, common.AIChatProject) (common.AIChatProject, error)
	DeleteChatProject(context.Context, string, string) error
	MoveChatToProject(context.Context, string, string, string) error
	SetDefaultChatProject(context.Context, string, string) error
}

type chatProjectSkillServices interface {
	ChatProjectSkills(context.Context, string, string) ([]common.Skill, error)
}

// refreshChatProjectSkills keeps discovery scoped to the active project and rejects stale results.
func (a *chatController) refreshChatProjectSkills() {
	state := a.chatPreview
	if state == nil {
		return
	}
	id := state.chat.ProjectID
	state.projectSkillsRevision++
	revision := state.projectSkillsRevision
	if state.projectSkillsProjectID != id {
		state.projectSkills = nil
		state.projectSkillsProjectID = id
	}
	state.projectSkillsLoading = false
	service, ok := a.services.(chatProjectSkillServices)
	if id == "" || !ok {
		return
	}
	state.projectSkillsLoading = true
	util.Go(a.lifecycleCtx, "discover project skills", func() {
		skills, err := service.ChatProjectSkills(a.lifecycleCtx, a.sessionID, id)
		if err != nil {
			util.GetLogger().Warn(a.lifecycleCtx, "discover project skills: "+err.Error())
		}
		a.runOnUI("apply project skills", func() {
			if a.chatPreview != state || state.chat.ProjectID != id || state.projectSkillsRevision != revision {
				return
			}
			state.projectSkillsLoading = false
			state.projectSkills = nil
			for _, skill := range skills {
				if skill.Enabled && strings.TrimSpace(skill.Name) != "" {
					state.projectSkills = append(state.projectSkills, chatSkill{ID: skill.Id, Name: skill.Name, Description: skill.Description, Path: skill.Path, ManifestPath: skill.ManifestPath, Source: skill.Source, SourceName: skill.SourceName, Error: skill.Error, Enabled: skill.Enabled})
				}
			}
			state.panelSelected = 0
			a.invalidateChatSurfaces()
		})
	})
}

// chatSkills combines the active project's catalog with global skills; project names win.
func (a *chatController) chatSkills() []chatSkill {
	var skills []chatSkill
	state := a.chatPreview
	if state != nil && state.chat.ProjectID != "" && state.projectSkillsProjectID == state.chat.ProjectID {
		skills = append(skills, state.projectSkills...)
	}
	projectSkills := skills
	for _, skill := range a.aiSettings().Skills() {
		duplicate := false
		for _, projectSkill := range projectSkills {
			if strings.EqualFold(projectSkill.Name, skill.Name) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			skills = append(skills, skill)
		}
	}
	return skills
}

// rememberChatProject serializes default writes while recording every explicit selection.
func (a *chatController) rememberChatProject(id string) {
	if service, ok := a.services.(interface {
		RecordChatProjectSelection(context.Context, string, string) error
	}); ok && id != "" {
		state := a.chatPreview
		util.Go(a.lifecycleCtx, "record chat project selection", func() {
			if err := service.RecordChatProjectSelection(context.Background(), a.sessionID, id); err != nil {
				util.GetLogger().Warn(context.Background(), "record chat project selection: "+err.Error())
				return
			}
			a.runOnUI("refresh project usage", func() {
				if a.chatPreview == state {
					a.loadChatProjects()
				}
			})
		})
	}
	a.chatProjectChoice = id
	a.chatProjectChoiceRevision++
	if a.chatProjectChoiceSaving {
		return
	}
	a.chatProjectChoiceSaving = true
	a.saveChatProjectChoice()
}

func (a *chatController) saveChatProjectChoice() {
	service, ok := a.services.(chatProjectServices)
	if !ok {
		a.chatProjectChoiceSaving = false
		return
	}
	id, revision := a.chatProjectChoice, a.chatProjectChoiceRevision
	util.Go(a.lifecycleCtx, "save default chat project", func() {
		err := service.SetDefaultChatProject(context.Background(), a.sessionID, id)
		if err != nil {
			util.GetLogger().Error(context.Background(), "save default chat project: "+err.Error())
		}
		if dispatchErr := a.runOnUI("finish saving default chat project", func() {
			if a.chatProjectChoiceRevision != revision {
				a.saveChatProjectChoice()
			} else {
				a.chatProjectChoiceSaving = false
			}
		}); dispatchErr != nil {
			util.GetLogger().Error(context.Background(), "dispatch default chat project: "+dispatchErr.Error())
		}
	})
}

// chatProjectUI owns device-local editor state; it is copied into each render snapshot.
type chatProjectUI struct {
	Projects       []common.AIChatProject
	Collapsed      map[string]bool
	Loaded         bool
	Loading        bool
	ReloadPending  bool
	Editor         *common.AIChatProject
	Error          string
	Busy           bool
	DeleteID       string
	DirectoryInput string
}

// cloneChatProjectUI isolates view props from asynchronous resource refreshes.
func cloneChatProjectUI(source chatProjectUI) chatProjectUI {
	result := source
	result.Projects = slices.Clone(source.Projects)
	for i := range result.Projects {
		result.Projects[i].Directories = slices.Clone(result.Projects[i].Directories)
	}
	result.Collapsed = make(map[string]bool, len(source.Collapsed))
	for key, value := range source.Collapsed {
		result.Collapsed[key] = value
	}
	if source.Editor != nil {
		editor := *source.Editor
		editor.Directories = slices.Clone(editor.Directories)
		result.Editor = &editor
	}
	return result
}

// loadChatProjects refreshes metadata independently of model/skill discovery.
func (a *chatController) loadChatProjects() {
	service, ok := a.services.(chatProjectServices)
	state := a.chatPreview
	if !ok || state == nil {
		return
	}
	if state.projects.Loading {
		state.projects.ReloadPending = true
		return
	}
	state.projects.Loading = true
	util.Go(a.lifecycleCtx, "load chat projects", func() {
		projects, err := service.ChatProjects(a.lifecycleCtx, a.sessionID)
		a.runOnUI("apply chat projects", func() {
			if a.chatPreview != state {
				return
			}
			state.projects.Loading = false
			if state.projects.ReloadPending {
				state.projects.ReloadPending = false
				a.loadChatProjects()
				return
			}
			if err != nil {
				state.error = err.Error()
			} else {
				state.projects.Projects, state.projects.Loaded = projects, true
				// Missing identities are displayed as ordinary chats without rewriting sync data.
				if !chatProjectExists(projects, state.chat.ProjectID) && !state.chat.IsStreaming {
					state.chat.ProjectID = ""
				}
				a.refreshChatProjectSkills()
			}
			a.invalidateChatSurfaces()
		})
	})
}

func chatProjectExists(projects []common.AIChatProject, id string) bool {
	return id == "" || slices.ContainsFunc(projects, func(p common.AIChatProject) bool { return p.Id == id })
}

// chooseChatProject preserves drafts and moves saved chats only after persistence succeeds.
func (a *chatController) chooseChatProject(id string) {
	state := a.chatPreview
	if state == nil || state.sending || state.chat.IsStreaming || state.projects.Busy {
		return
	}
	service, ok := a.services.(chatProjectServices)
	if !ok {
		return
	}
	if state.panel == chatCommandPanel {
		replaceChatSlashToken(state.editor, "")
		restoreChatHistoryPanelLocked(state)
	}
	if len(state.chat.Conversations) == 0 && !state.chat.IsSummary {
		state.chat.ProjectID = id
		a.refreshChatProjectSkills()
		a.rememberChatProject(id)
		a.dismissChatProject()
		return
	}
	chatID := state.chat.ID
	state.projects.Busy = true
	util.Go(a.lifecycleCtx, "move chat to project", func() {
		err := service.MoveChatToProject(a.lifecycleCtx, a.sessionID, chatID, id)
		a.runOnUI("apply chat membership", func() {
			if a.chatPreview != state {
				return
			}
			state.projects.Busy = false
			if err != nil {
				state.error = err.Error()
			} else {
				for i := range state.chats {
					if state.chats[i].ID == chatID {
						state.chats[i].ProjectID = id
					}
				}
				if state.chat.ID == chatID {
					state.chat.ProjectID = id
					a.refreshChatProjectSkills()
				}
				a.rememberChatProject(id)
				a.dismissChatProject()
			}
			a.invalidateChatSurfaces()
		})
	})
}

// editChatProject opens a detached draft so cancellation never mutates saved paths.
func (a *chatController) editChatProject(id string) {
	state := a.chatPreview
	if state == nil || state.projects.Busy {
		return
	}
	project := common.AIChatProject{}
	for _, candidate := range state.projects.Projects {
		if candidate.Id == id {
			project = candidate
			project.Directories = slices.Clone(candidate.Directories)
			break
		}
	}
	if state.panel == chatProjectPanel {
		restoreChatHistoryPanelLocked(state)
	}
	state.projects.Editor = &project
	state.projects.Error, state.projects.DeleteID, state.projects.DirectoryInput = "", "", ""
	a.updateChatTextInput(false)
	a.invalidateChatSurfaces()
}

// dismissChatProject restores composer ownership after the shared modal releases focus.
func (a *chatController) dismissChatProject() {
	if state := a.chatPreview; state != nil && !state.projects.Busy {
		state.projects.Editor = nil
		if state.panel == chatProjectPanel {
			restoreChatHistoryPanelLocked(state)
		}
		state.projects.Error, state.projects.DeleteID, state.projects.DirectoryInput = "", "", ""
		a.updateChatTextInput(true)
		a.invalidateChatSurfaces()
	}
}

func (a *chatController) openChatProjects() {
	if state := a.chatPreview; state != nil && !state.sending && !state.chat.IsStreaming && !state.projects.Busy {
		state.projects.Error = ""
		a.toggleChatPanel(chatProjectPanel)
	}
}

// pickChatProjectDirectory uses the active native host's directory picker.
func (a *chatController) pickChatProjectDirectory() {
	state := a.chatPreview
	if state == nil || state.projects.Editor == nil || state.projects.Busy {
		return
	}
	if path := strings.TrimSpace(state.projects.DirectoryInput); path != "" {
		editor := state.projects.Editor
		if !slices.Contains(editor.Directories, path) {
			editor.Directories = append(editor.Directories, path)
		}
		if editor.PrimaryDirectory == "" {
			editor.PrimaryDirectory = path
		}
		state.projects.DirectoryInput = ""
		a.invalidateChatSurfaces()
		return
	}
	window := a.chatTextInputWindow()
	if window == nil {
		return
	}
	path, err := window.PickFile(woxui.FileDialogOptions{Directory: true})
	if err != nil {
		state.projects.Error = err.Error()
	} else if path != "" {
		editor := state.projects.Editor
		if !slices.Contains(editor.Directories, path) {
			editor.Directories = append(editor.Directories, path)
		}
		if editor.PrimaryDirectory == "" {
			editor.PrimaryDirectory = path
		}
	}
	a.invalidateChatSurfaces()
}

// saveChatProject keeps the editor open with its input intact on a failed save.
func (a *chatController) saveChatProject() {
	state := a.chatPreview
	service, ok := a.services.(chatProjectServices)
	if !ok || state == nil || state.projects.Editor == nil || state.projects.Busy {
		return
	}
	project := *state.projects.Editor
	project.Directories = slices.Clone(project.Directories)
	state.projects.Busy, state.projects.Error = true, ""
	util.Go(a.lifecycleCtx, "save chat project", func() {
		project, err := service.SaveChatProject(a.lifecycleCtx, a.sessionID, project)
		a.runOnUI("apply saved chat project", func() {
			if a.chatPreview != state {
				return
			}
			state.projects.Busy = false
			if err != nil {
				state.projects.Error = err.Error()
			} else {
				index := slices.IndexFunc(state.projects.Projects, func(p common.AIChatProject) bool { return p.Id == project.Id })
				if index < 0 {
					state.projects.Projects = append(state.projects.Projects, project)
				} else {
					state.projects.Projects[index] = project
				}
				if state.projects.Editor.Id == "" && len(state.chat.Conversations) == 0 {
					state.chat.ProjectID = project.Id
					a.rememberChatProject(project.Id)
				}
				a.dismissChatProject()
				a.loadChatProjects()
			}
			a.invalidateChatSurfaces()
		})
	})
}

// confirmChatProjectDelete opens a separate confirmation without changing the editor draft.
func (a *chatController) confirmChatProjectDelete(id string) {
	state := a.chatPreview
	if state == nil || state.projects.Busy || id == "" || !chatProjectExists(state.projects.Projects, id) {
		return
	}
	state.projects.DeleteID, state.projects.Error = id, ""
	a.updateChatTextInput(false)
	a.invalidateChatSurfaces()
}

// deleteChatProject detaches conversations only after the project deletion succeeds.
func (a *chatController) deleteChatProject() {
	state := a.chatPreview
	service, ok := a.services.(chatProjectServices)
	if !ok || state == nil || state.projects.DeleteID == "" || state.projects.Busy {
		return
	}
	id := state.projects.DeleteID
	state.projects.Busy, state.projects.Error = true, ""
	util.Go(a.lifecycleCtx, "delete chat project", func() {
		err := service.DeleteChatProject(a.lifecycleCtx, a.sessionID, id)
		a.runOnUI("apply deleted chat project", func() {
			if a.chatPreview != state {
				return
			}
			state.projects.Busy = false
			if err != nil {
				state.projects.Error = err.Error()
			} else {
				state.projects.Projects = slices.DeleteFunc(state.projects.Projects, func(p common.AIChatProject) bool { return p.Id == id })
				for i := range state.chats {
					if state.chats[i].ProjectID == id {
						state.chats[i].ProjectID = ""
					}
				}
				if state.chat.ProjectID == id {
					state.chat.ProjectID = ""
				}
				a.dismissChatProject()
				a.loadChatProjects()
			}
			a.invalidateChatSurfaces()
		})
	})
}

// validateChatProjectDraft prevents losing the draft when this device has no usable primary directory.
func (a *chatController) validateChatProjectDraft() bool {
	state := a.chatPreview
	if state.projects.Busy {
		return false
	}
	if state.chat.ProjectID == "" {
		return true
	}
	for _, project := range state.projects.Projects {
		if project.Id == state.chat.ProjectID {
			if info, err := os.Stat(project.PrimaryDirectory); err == nil && info.IsDir() {
				return true
			}
			state.error = a.language.translate("i18n:plugin_ai_chat_project_bind_directories")
			a.invalidateChatSurfaces()
			return false
		}
	}
	if !state.projects.Loaded {
		return false
	}
	state.chat.ProjectID = ""
	return true
}

// chatProjectHistoryRows flattens only expanded groups so keyboard order matches rendering.
func chatProjectHistoryRows(state chatProjectUI, chats []chatData) []chatProjectHistoryRow {
	projects := slices.Clone(state.Projects)
	activity := make(map[string]int64)
	for _, p := range projects {
		activity[p.Id] = p.UpdatedAt
	}
	for _, chat := range chats {
		activity[chat.ProjectID] = max(activity[chat.ProjectID], chat.UpdatedAt)
	}
	sort.SliceStable(projects, func(i, j int) bool { return activity[projects[i].Id] > activity[projects[j].Id] })
	rows := []chatProjectHistoryRow{{ProjectID: chatProjectsSection, Group: true}}
	for _, project := range projects {
		if state.Collapsed[chatProjectsSection] {
			break
		}
		rows = append(rows, chatProjectHistoryRow{ProjectID: project.Id, Title: project.Name, Group: true})
		if state.Collapsed[project.Id] {
			continue
		}
		for _, chat := range chats {
			id := chat.ProjectID
			if !chatProjectExists(state.Projects, id) {
				id = ""
			}
			if id == project.Id && (chat.IsSummary || len(chat.Conversations) > 0) {
				rows = append(rows, chatProjectHistoryRow{ProjectID: id, ChatID: chat.ID, Title: chat.Title, Streaming: chat.IsStreaming})
			}
		}
	}
	rows = append(rows, chatProjectHistoryRow{ProjectID: chatRecentsSection, Group: true})
	if !state.Collapsed[chatRecentsSection] {
		for _, chat := range chats {
			if (chat.ProjectID == "" || !chatProjectExists(state.Projects, chat.ProjectID)) && (chat.IsSummary || len(chat.Conversations) > 0) {
				rows = append(rows, chatProjectHistoryRow{ProjectID: chatRecentsSection, ChatID: chat.ID, Title: chat.Title, Streaming: chat.IsStreaming})
			}
		}
	}
	return rows
}

type chatProjectHistoryRow struct {
	ProjectID, ChatID, Title string
	Group                    bool
	Streaming                bool
}

func (a *chatController) activateProjectHistoryRow(row chatProjectHistoryRow) {
	state := a.chatPreview
	if state == nil {
		return
	}
	if !row.Group {
		a.selectChatHistory(row.ChatID)
		return
	}
	if state.projects.Collapsed == nil {
		state.projects.Collapsed = make(map[string]bool)
	}
	state.projects.Collapsed[row.ProjectID] = !state.projects.Collapsed[row.ProjectID]
	a.invalidateChatSurfaces()
}

// projectHistoryHeight includes the new-chat entry, its gap and all visible history rows.
func projectHistoryHeight(state *chatPreviewState) float32 {
	return chatview.ChatScaledSize(state.panelDensityScale, 46) + float32(len(chatProjectHistoryRows(state.projects, state.chats)))*chatview.ChatScaledSize(state.panelDensityScale, chatview.ChatHistoryRowHeight) + chatview.ChatScaledSize(state.panelDensityScale, chatview.ChatHistorySectionGap)
}

const chatProjectPanel = "projects"
const chatProjectsSection = "$projects"
const chatRecentsSection = "$recents"

// chatProjectPaletteItems shares filtering and keyboard geometry with the model catalog.
func chatProjectPaletteItems(projects []common.AIChatProject, current, query, noneLabel string, pinyin ...bool) []chatCommandPaletteItem {
	items := []chatCommandPaletteItem{{group: chatProjectPanel, sourceIndex: -1, title: noneLabel, current: current == ""}}
	query = strings.TrimSpace(query)
	usePinYin := len(pinyin) > 0 && pinyin[0]
	for index, project := range projects {
		if score, ok := chatCatalogMatchScore(query, usePinYin, project.Name); ok {
			// A capped logarithmic bonus makes frequent choices useful without overwhelming an exact match.
			if query != "" {
				score += int64(min(30, math.Log2(float64(max(0, project.SelectionCount))+1)*6))
			}
			items = append(items, chatCommandPaletteItem{group: chatProjectPanel, sourceIndex: index, title: project.Name, current: project.Id == current, score: score})
		}
	}
	if query != "" {
		sortChatCatalogResults(items[1:])
	}
	return items
}

func (a *chatController) projectPaletteItems() []chatCommandPaletteItem {
	state := a.chatPreview
	return chatProjectPaletteItems(state.projects.Projects, state.chat.ProjectID, state.panelQuery, a.language.translate("i18n:plugin_ai_chat_project_none"), a.usePinYin())
}
