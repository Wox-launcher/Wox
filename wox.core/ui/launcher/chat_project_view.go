package launcher

import (
	"fmt"
	"slices"
	"strings"

	"wox/common/icons"
	woxcomponent "wox/ui/launcher/component"
	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func (a *App) chatProjectLabel(snapshot *chatPreviewSnapshot) string {
	for _, p := range snapshot.projects.Projects {
		if p.Id == snapshot.chat.ProjectID {
			return p.Name
		}
	}
	return a.language.translate("i18n:plugin_ai_chat_project_none")
}

// chatProjectOverlay mounts the same modal in launcher and dedicated chat windows.
func (a *App) chatProjectOverlay(snapshot *chatPreviewSnapshot, palette uiPalette, width, height float32, window *woxui.Window, body woxwidget.Widget) woxwidget.Widget {
	ui := snapshot.projects
	if ui.Editor == nil && ui.DeleteID == "" {
		return body
	}
	tr := func(key string) string { return a.language.translate("i18n:plugin_ai_chat_project_" + key) }
	theme := palette.componentTheme().Controls
	theme.DensityScale = a.chatDensityScale()
	if ui.DeleteID != "" {
		name := ""
		for _, project := range ui.Projects {
			if project.Id == ui.DeleteID {
				name = project.Name
				break
			}
		}
		dialog := previewview.ChatProjectDeleteDialog(previewview.ChatProjectDeleteDialogProps{
			Width: width, Height: height, Theme: theme, Title: tr("delete"), Name: name, Message: tr("delete_confirm"),
			Error: ui.Error, Busy: ui.Busy, CancelLabel: tr("cancel"), DeleteLabel: tr("delete"),
			OnCancel: a.chat.dismissChatProject, OnDelete: a.chat.deleteChatProject,
		})
		return woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{{Child: body}, {Child: dialog}}}
	}
	props := previewview.ChatProjectDialogProps{
		Width: width, Height: height, Theme: theme, Window: window,
		Icon:  a.imageForSurface(fromCoreImage(icons.Get(icons.ControlFolderOpen)), 64, palette.background),
		Title: tr("new"), NameLabel: tr("name"), PrimaryLabel: tr("primary"), AddLabel: tr("add_directory"),
		SaveLabel: tr("save"), CancelLabel: tr("cancel"), RemoveLabel: tr("remove"),
		Error: ui.Error, Busy: ui.Busy,
		DirectoryInput: ui.DirectoryInput, DirectoryHint: tr("directory_hint"),
		OnDirectoryInput: func(value string) {
			if state := a.chat.chatPreview; state != nil && !state.projects.Busy {
				state.projects.DirectoryInput = value
				a.invalidateChatSurfaces()
			}
		},
		OnCancel: a.chat.dismissChatProject, OnAdd: a.chat.pickChatProjectDirectory,
		OnSave: a.chat.saveChatProject,
	}
	props.Name = ui.Editor.Name
	if ui.Editor.Id != "" {
		props.Title = tr("edit")
	}
	props.OnName = func(name string) {
		if state := a.chat.chatPreview; state != nil && state.projects.Editor != nil && !state.projects.Busy {
			state.projects.Editor.Name = name
			a.invalidateChatSurfaces()
		}
	}
	for i, directory := range ui.Editor.Directories {
		props.Rows = append(props.Rows, previewview.ChatProjectRow{
			ID: fmt.Sprintf("chat-project-directory-%d", i), Label: directory, Selected: directory == ui.Editor.PrimaryDirectory,
			OnSelect: func() {
				if state := a.chat.chatPreview; state != nil && state.projects.Editor != nil && !state.projects.Busy {
					state.projects.Editor.PrimaryDirectory = directory
					a.invalidateChatSurfaces()
				}
			},
			OnRemove: func() {
				if state := a.chat.chatPreview; state != nil && state.projects.Editor != nil && !state.projects.Busy {
					editor := state.projects.Editor
					editor.Directories = slices.DeleteFunc(editor.Directories, func(path string) bool { return path == directory })
					if editor.PrimaryDirectory == directory {
						editor.PrimaryDirectory = ""
						if len(editor.Directories) > 0 {
							editor.PrimaryDirectory = editor.Directories[0]
						}
					}
					a.invalidateChatSurfaces()
				}
			},
		})
	}
	return woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{{Child: body}, {Child: previewview.ChatProjectDialog(props)}}}
}

// chatProjectHistoryProps renders the same flattened order used by keyboard navigation.
func (a *App) chatProjectHistoryProps(snapshot *chatPreviewSnapshot, palette uiPalette, width, height float32) previewview.ChatCatalogProps {
	rows := chatProjectHistoryRows(snapshot.projects, snapshot.chats)
	items := make([]previewview.ChatCatalogItemProps, 0, len(rows))
	tr := func(key string) string { return a.language.translate("i18n:plugin_ai_chat_project_" + key) }
	for _, row := range rows {
		title := strings.TrimSpace(row.Title)
		if title == "" {
			title = a.language.translate("i18n:ui_ai_chat_new_chat")
		}
		item := previewview.ChatCatalogItemProps{
			SelectID: "chat-history-row-" + row.ChatID, Kind: "history", Title: title,
			Streaming: row.Streaming || row.ChatID != "" && row.ChatID == snapshot.chat.ID && (snapshot.chat.IsStreaming || snapshot.sending),
			Selected:  row.ChatID != "" && row.ChatID == snapshot.chat.ID, Indented: !row.Group && row.ProjectID != "", OnSelect: func() { a.chat.activateProjectHistoryRow(row) },
			DeleteID: "chat-history-delete-" + row.ChatID, DeleteLabel: a.language.translate("i18n:ui_ai_chat_delete_chat"),
			ConfirmDeleteLabel: a.language.translate("i18n:ui_delete_row_confirm"),
			OnDelete:           func() { a.chat.deleteChatHistory(row.ChatID) },
		}
		if row.Group {
			item.Kind, item.OnDelete, item.Collapsed = "project-group", nil, snapshot.projects.Collapsed[row.ProjectID]
			item.Selected = false
			item.SelectID = "chat-project-group-" + row.ProjectID
			if row.ProjectID == chatProjectsSection {
				item.Kind, item.Title = "projects-section", tr("section")
				item.SelectID = "chat-projects-section"
				item.NewLabel, item.OnNew = tr("new"), func() { a.chat.editChatProject("") }
			} else if row.ProjectID == chatRecentsSection {
				item.Kind, item.Title = "projects-section", tr("recents")
				item.SelectID = "chat-recents-section"
				item.SpacingBefore = previewview.ChatHistorySectionGap
			} else {
				item.Icon = a.imageForSurface(fromCoreImage(icons.Get(icons.ControlFolderOpen)), 64, palette.background)
				item.EditLabel = tr("edit")
				item.OnEdit = func() { a.chat.editChatProject(row.ProjectID) }
				item.DeleteLabel = tr("delete")
				item.OnDelete = func() { a.chat.confirmChatProjectDelete(row.ProjectID) }
				item.NewLabel = a.language.translate("i18n:ui_ai_chat_new_chat")
				item.OnNew = func() {
					a.chat.startNewChat()
					if state := a.chat.chatPreview; state != nil {
						state.chat.ProjectID = row.ProjectID
						a.chat.rememberChatProject(row.ProjectID)
					}
					a.invalidateChatSurfaces()
				}
			}
		}
		items = append(items, item)
	}
	viewport := max(float32(1), height-24)
	scale := a.chatDensityScale()
	content := previewview.ChatScaledSize(scale, 46) + float32(len(rows))*previewview.ChatScaledSize(scale, previewview.ChatHistoryRowHeight) + previewview.ChatScaledSize(scale, previewview.ChatHistorySectionGap)
	offset := snapshot.sidebarScroll
	if snapshot.panel == "history" {
		offset = snapshot.panelScroll
	}
	if state := a.chat.chatPreview; state != nil {
		state.panelDensityScale = scale
		if snapshot.panel == "history" {
			state.panelViewport = viewport
		} else {
			state.sidebarViewport = viewport
		}
	}
	return previewview.ChatCatalogProps{
		Width: width, Height: height, DensityScale: a.chatDensityScale(), Key: snapshot.key, Items: items,
		Scroll: min(max(0, offset), max(0, content-viewport)), ContentHeight: content,
		ShowNew: true, NewLabel: a.language.translate("i18n:ui_ai_chat_new_chat"), OnNew: a.chat.startNewChat,
		Theme: palette.componentTheme(), OnScroll: a.chat.scrollChatHistoryDrawer,
	}
}

// chatProjectPickerProps uses the model picker surface, search field and keyboard behavior.
func (a *App) chatProjectPickerProps(snapshot *chatPreviewSnapshot, palette uiPalette, width, height float32) previewview.ChatCatalogProps {
	tr := func(key string) string { return a.language.translate("i18n:plugin_ai_chat_project_" + key) }
	commands := chatProjectPaletteItems(snapshot.projects.Projects, snapshot.chat.ProjectID, snapshot.panelQuery, tr("none"), snapshot.usePinYin)
	scale := a.chatDensityScale()
	content := chatCommandContentHeight(commands, scale)
	height = previewview.ChatPickerHeight(content, scale, height)
	if state := a.chat.chatPreview; state != nil {
		state.panelDensityScale, state.panelLeadingHeight, state.panelLoadingHeight = scale, 0, 0
	}
	a.chat.setChatPanelViewport(max(0, height-previewview.ChatPickerChromeHeight(scale)))
	offset := snapshot.panelScroll
	if state := a.chat.chatPreview; state != nil {
		offset = state.panelScroll
	}
	props := previewview.ChatCatalogProps{
		Key: snapshot.key, Width: width, Height: height, DensityScale: scale, Picker: true,
		Scroll: offset, ContentHeight: content, Theme: palette.componentTheme(), OnScroll: a.chat.scrollChatPanel,
	}
	for index, command := range commands {
		id, automationID := "", "chat-project-none"
		if command.sourceIndex >= 0 {
			id = snapshot.projects.Projects[command.sourceIndex].Id
			automationID = "chat-project-choice-" + id
		}
		props.Items = append(props.Items, previewview.ChatCatalogItemProps{
			SelectID: automationID, Kind: chatProjectPanel, Picker: true, SectionLabel: tr("section"),
			Title: command.title, RowHeight: chatCatalogRowHeight, Current: command.current, Selected: index == snapshot.panelSelected,
			Icon:     a.imageForSurface(fromCoreImage(icons.Get(icons.ControlFolderOpen)), 64, palette.background),
			OnSelect: func() { a.chat.chooseChatProject(id); a.focusChatCatalogComposer(snapshot.key) },
		})
	}
	props.Search = &woxcomponent.TextFieldProps{
		ID: "chat-project-search-" + snapshot.key, Value: snapshot.panelQuery, Hint: tr("search"),
		Theme: props.Theme.Controls, Window: a.chat.chatTextInputWindow(), Focused: true, OnChanged: a.chat.setChatCatalogQuery,
		OnKey: func(event woxui.KeyEvent) bool {
			return previewview.ChatCatalogKey(event, a.chat.moveChatPanelSelection, a.chat.activateChatPanelSelection, a.chat.closeChatPanel)
		},
	}
	return props
}
