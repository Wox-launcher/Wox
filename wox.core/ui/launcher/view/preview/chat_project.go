package preview

import (
	"fmt"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// ChatProjectRow carries immutable display data and callbacks for one source or project.
type ChatProjectRow struct {
	ID, Label          string
	Selected           bool
	OnSelect, OnRemove func()
}

// ChatProjectDialogProps describes the directory editor shared by both chat hosts.
type ChatProjectDialogProps struct {
	DirectoryInput, DirectoryHint                                                       string
	OnDirectoryInput                                                                    func(string)
	Width, Height                                                                       float32
	Theme                                                                               woxcomponent.ControlTheme
	Icon                                                                                *woxui.Image
	Window                                                                              *woxui.Window
	Title, Name, NameLabel, PrimaryLabel, AddLabel, SaveLabel, CancelLabel, RemoveLabel string
	Error                                                                               string
	Busy                                                                                bool
	Rows                                                                                []ChatProjectRow
	OnName                                                                              func(string)
	OnAdd, OnSave, OnCancel                                                             func()
}

// ChatProjectDialog reuses modal focus trapping and scrolls long directory lists.
func ChatProjectDialog(p ChatProjectDialogProps) woxwidget.Widget {
	s := p.Theme.Scaled
	width := min(s(560), max(0, p.Width-s(24)))
	inner := max(0, width-s(32))
	height := min(s(480), max(0, p.Height-s(24)))
	children := []woxwidget.Widget{
		woxwidget.TextBlock{Value: p.Title, Height: s(32), LineHeight: s(32), MaxLines: 1, AlignmentY: 0.5, Style: woxui.TextStyle{Size: s(woxcomponent.SettingsPageTitleFontSize), Weight: woxui.FontWeightSemibold}, Color: p.Theme.Text},
	}
	// Use explicit line boxes; native CJK title metrics exceed the old 24-unit estimate.
	reserved := 4*s(32) + 4*s(12) + 2*s(16)
	children = append(children, woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: "chat-project-name", Label: p.NameLabel, Hint: p.NameLabel, Value: p.Name, Width: inner, Height: s(32),
		Window: p.Window, Theme: p.Theme, Disabled: p.Busy, OnChanged: p.OnName,
	}), woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: s(8), Children: []woxwidget.Widget{
		woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
			ID: "chat-project-directory-input", Label: p.DirectoryHint, Hint: p.DirectoryHint, Value: p.DirectoryInput,
			Width: max(0, inner-s(128)), Height: s(32), Theme: p.Theme, Window: p.Window, Disabled: p.Busy, OnChanged: p.OnDirectoryInput,
		}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: "chat-project-add-directory", Label: p.AddLabel, Icon: p.Icon, Width: s(120), Height: s(32), Theme: p.Theme, Disabled: p.Busy, OnTap: p.OnAdd,
		}),
	}})
	if p.Error != "" {
		children = append(children, woxwidget.TextBlock{Value: p.Error, Width: inner, Height: s(48), MaxLines: 3, LineHeight: s(16), Style: woxui.TextStyle{Size: s(12)}, Color: p.Theme.Error})
		reserved += s(48) + s(12)
	}
	rowHeight := s(36)
	actionAxis := woxwidget.Horizontal
	if inner < s(400) {
		actionAxis = woxwidget.Vertical
		reserved += s(40)
	}
	rows := make([]woxwidget.Widget, 0, len(p.Rows))
	for index, row := range p.Rows {
		label := row.Label
		if row.Selected {
			label = p.PrimaryLabel + ": " + label
		}
		color := p.Theme.Text
		if row.Selected {
			color = p.Theme.SelectionText
		}
		directoryContent := []woxwidget.Widget{
			woxwidget.Image{Source: p.Icon, Width: s(16), Height: s(16)},
			woxwidget.Expanded{Child: woxwidget.TextBlock{Value: row.Label, Height: s(20), LineHeight: s(20), MaxLines: 1, AlignmentY: 0.5, Style: woxui.TextStyle{Size: s(woxcomponent.SettingsControlFontSize)}, Color: color}},
		}
		if row.Selected {
			directoryContent = append(directoryContent, woxwidget.TextBlock{Value: p.PrimaryLabel, Height: s(20), LineHeight: s(20), MaxLines: 1, ShrinkWrap: true, AlignmentY: 0.5, Style: woxui.TextStyle{Size: s(woxcomponent.SettingsHelpFontSize)}, Color: color})
		}
		entry := woxcomponent.WoxListItem(woxcomponent.ListItemProps{
			ID: row.ID, Label: label, Width: inner, Height: s(32), Selected: row.Selected, Theme: p.Theme, Disabled: p.Busy, OnTap: row.OnSelect,
			Padding: woxwidget.Insets{Left: s(8), Right: s(40)},
			Child:   woxwidget.Align{Height: s(32), Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: s(8), CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: directoryContent}},
		})
		removeActiveColor := p.Theme.Error
		if p.Busy {
			removeActiveColor = color
		}
		remove := woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: fmt.Sprintf("chat-project-remove-%d", index), Label: p.RemoveLabel, Icon: woxcomponent.DeleteGlyph(s(16), removeActiveColor), IdleIcon: woxcomponent.DeleteGlyph(s(16), color),
			Width: s(28), Height: s(28), Radius: s(4), Disabled: p.Busy, OnTap: row.OnRemove, FocusRingColor: p.Theme.Focus,
		})
		rows = append(rows, woxwidget.Stack{Width: inner, Height: rowHeight, Children: []woxwidget.StackChild{
			{Child: entry}, {Right: s(2), AnchorRight: true, Child: woxwidget.Align{Width: s(28), Height: s(32), Vertical: 0.5, Child: remove}},
		}})
	}
	// A short form should not expand to the viewport just because the list can scroll.
	height = min(height, reserved+max(s(32), float32(len(rows))*rowHeight))
	listHeight := max(s(32), height-reserved)
	children = append(children, woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: "chat-project-list", Width: inner, Height: listHeight, ContentHeight: float32(len(rows)) * rowHeight, Theme: p.Theme,
		Content: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
	}))
	actions := []woxwidget.Widget{woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "chat-project-cancel", Label: p.CancelLabel, Height: s(32), Theme: p.Theme, Disabled: p.Busy, OnTap: p.OnCancel})}
	actions = append(actions, woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "chat-project-save", Label: p.SaveLabel, Height: s(32), Theme: p.Theme, Variant: woxcomponent.ButtonPrimary, Disabled: p.Busy, OnTap: p.OnSave}))
	actionHeight := s(32)
	if actionAxis == woxwidget.Vertical {
		actionHeight = float32(len(actions))*s(32) + float32(len(actions)-1)*s(8)
	}
	children = append(children, woxwidget.Align{Width: inner, Height: actionHeight, Horizontal: 1, Child: woxwidget.Flex{Axis: actionAxis, CrossAxisAlignment: woxwidget.CrossAxisEnd, Gap: s(8), Children: actions}})
	body := woxwidget.Widget(woxwidget.Flex{Axis: woxwidget.Vertical, Gap: s(12), Children: children})
	if height < reserved+s(32) {
		body = woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
			Key: "chat-project-dialog-scroll", Width: inner, Height: max(0, height-s(32)), ContentHeight: reserved,
			Theme: p.Theme, Content: body,
		})
	}
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "chat-project-dialog", Label: p.Title, Width: width, Height: height, OverlayWidth: p.Width, OverlayHeight: p.Height,
		BackdropID: "chat-project-backdrop", BackdropAlpha: 160, Theme: p.Theme, Padding: woxwidget.UniformInsets(s(16)),
		InitialFocus: "chat-project-name", OnEscape: p.OnCancel, OnBackdrop: p.OnCancel,
		Child: body,
	})
}

// ChatProjectDeleteDialog confirms project deletion separately from editing.
type ChatProjectDeleteDialogProps struct {
	Width, Height                                         float32
	Theme                                                 woxcomponent.ControlTheme
	Title, Name, Message, Error, CancelLabel, DeleteLabel string
	Busy                                                  bool
	OnCancel, OnDelete                                    func()
}

// ChatProjectDeleteDialog explains what happens to the project's conversations and files.
func ChatProjectDeleteDialog(p ChatProjectDeleteDialogProps) woxwidget.Widget {
	s := p.Theme.Scaled
	width := min(s(460), max(0, p.Width-s(24)))
	inner := max(0, width-s(32))
	contentHeight := 3*s(32) + s(20) + s(80) + 3*s(12)
	actionAxis, actionHeight := woxwidget.Horizontal, s(32)
	if inner < s(300) {
		actionAxis, actionHeight = woxwidget.Vertical, 2*s(32)+s(8)
		contentHeight += s(32) + s(8)
	}
	if p.Error != "" {
		contentHeight += s(32) + s(12)
	}
	height := min(contentHeight, max(0, p.Height-s(24)))
	content := []woxwidget.Widget{
		woxwidget.TextBlock{Value: p.Title, Height: s(32), LineHeight: s(32), MaxLines: 1, AlignmentY: 0.5, Style: woxui.TextStyle{Size: s(woxcomponent.SettingsPageTitleFontSize), Weight: woxui.FontWeightSemibold}, Color: p.Theme.Text},
		woxwidget.TextBlock{Value: p.Name, Width: inner, Height: s(20), LineHeight: s(20), MaxLines: 1, AlignmentY: 0.5, Style: woxui.TextStyle{Size: s(13), Weight: woxui.FontWeightSemibold}, Color: p.Theme.Text},
		woxwidget.TextBlock{Value: p.Message, Width: inner, Height: s(80), LineHeight: s(16), MaxLines: 5, Style: woxui.TextStyle{Size: s(12)}, Color: p.Theme.TextSecondary},
	}
	if p.Error != "" {
		content = append(content, woxwidget.TextBlock{Value: p.Error, Width: inner, Height: s(32), LineHeight: s(16), MaxLines: 2, Style: woxui.TextStyle{Size: s(12)}, Color: p.Theme.Error})
	}
	content = append(content, woxwidget.Align{Width: inner, Height: actionHeight, Horizontal: 1, Child: woxwidget.Flex{Axis: actionAxis, Gap: s(8), CrossAxisAlignment: woxwidget.CrossAxisEnd, Children: []woxwidget.Widget{
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "chat-project-delete-cancel", Label: p.CancelLabel, Height: s(32), Theme: p.Theme, Disabled: p.Busy, OnTap: p.OnCancel}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "chat-project-delete-confirm", Label: p.DeleteLabel, Height: s(32), Theme: p.Theme, Disabled: p.Busy, Variant: woxcomponent.ButtonPrimary, OnTap: p.OnDelete}),
	}}})
	body := woxwidget.Widget(woxwidget.Flex{Axis: woxwidget.Vertical, Gap: s(12), Children: content})
	if height < contentHeight {
		body = woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
			Key: "chat-project-delete-scroll", Width: inner, Height: max(0, height-s(32)), ContentHeight: contentHeight - s(32),
			Theme: p.Theme, Content: body,
		})
	}
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "chat-project-delete-dialog", Label: p.Title, Width: width, Height: height, OverlayWidth: p.Width, OverlayHeight: p.Height,
		BackdropID: "chat-project-delete-backdrop", BackdropAlpha: 160, Theme: p.Theme, Padding: woxwidget.UniformInsets(s(16)),
		InitialFocus: "chat-project-delete-cancel", OnEscape: p.OnCancel, OnBackdrop: p.OnCancel,
		Child: body,
	})
}

// ChatProjectToolbarExtra reserves a second toolbar line on narrow chat surfaces.
func ChatProjectToolbarExtra(props ChatInputProps, width float32) float32 {
	if props.ProjectVisible && width < ChatScaledSize(props.DensityScale, 480) {
		return ChatScaledSize(props.DensityScale, 28)
	}
	return 0
}

// ChatProjectSelector uses the catalog's monochrome folder and shared button semantics.
func ChatProjectSelector(p ChatInputProps, width float32) woxwidget.Widget {
	return chatComposerSelector(chatComposerSelectorProps{
		ID: "chat-project-selector-" + p.Key, Label: p.ProjectLabel, Icon: p.ProjectIcon,
		Width: width, DensityScale: p.DensityScale, Theme: p.Theme, Disabled: p.Sending || p.ProjectBusy,
		OnTap: p.OnProjects,
	})
}

// ChatProjectSelectorWidth measures the label so short project names stay next to the model.
func ChatProjectSelectorWidth(p ChatInputProps, available float32) float32 {
	metrics, _ := p.Window.MeasureText(p.ProjectLabel, woxui.TextStyle{Size: ChatScaledSize(p.DensityScale, 11)})
	return min(available, ChatScaledSize(p.DensityScale, 180), metrics.Size.Width+chatComposerSelectorChromeWidth(p.DensityScale))
}

// chatProjectHistoryGroup shares retained row hover with chat history and keeps actions keyboard reachable.
func chatProjectHistoryGroup(item ChatCatalogItemProps, width float32, theme woxcomponent.Theme, hovered, newFocused, editFocused, deleteFocused bool, onHover, onNewFocus, onEditFocus, onDeleteFocus func(bool)) woxwidget.Widget {
	s := func(n float32) float32 { return ChatScaledSize(item.DensityScale, n) }
	height := s(ChatHistoryRowHeight)
	controls := theme.Controls
	controls.DensityScale = item.DensityScale
	foreground := theme.PreviewText
	if item.Kind == "projects-section" {
		foreground = theme.ResultSubtitle
	}
	var arrow woxwidget.Widget
	if hovered {
		arrow = woxcomponent.DisclosureChevronGlyph(s(16), foreground, !item.Collapsed)
	}
	reserved := float32(0)
	if item.OnNew != nil {
		reserved += s(chatSidebarActionSize)
	}
	if item.OnEdit != nil {
		reserved += s(chatSidebarActionSize)
	}
	if item.OnDelete != nil {
		reserved += s(chatSidebarActionSize)
	}
	label := []woxwidget.Widget{}
	if item.Icon != nil {
		label = append(label, woxwidget.Image{Source: item.Icon, Width: s(chatSidebarActionGlyph), Height: s(chatSidebarActionGlyph)})
	}
	// The 18-unit slot matches the icon centerline. AlignmentY keeps CJK glyphs in that slot.
	title := woxwidget.TextBlock{Value: item.Title, Height: s(18), LineHeight: s(18), MaxLines: 1, ShrinkWrap: true, AlignmentY: 0.5, Style: woxui.TextStyle{Size: s(12)}, Color: foreground}
	if item.Kind == "projects-section" {
		title.Style.Weight = woxui.FontWeightSemibold
	}
	label = append(label, woxwidget.Flexible{Child: title}, woxwidget.Container{Width: s(16), Height: s(16), Child: arrow})
	labelActionsWidth := float32(0)
	if hovered || newFocused || editFocused || deleteFocused {
		labelActionsWidth = reserved
	}
	radius := s(6)
	background := woxui.Color{}
	if hovered {
		background = theme.SelectedBackground
	}
	row := woxcomponent.WoxListItem(woxcomponent.ListItemProps{
		ID: item.SelectID, Label: item.Title, OnTap: item.OnSelect,
		Width: width, Height: height, Radius: &radius, Background: &background, Theme: controls,
		Padding: woxwidget.Insets{Left: s(chatSidebarRowInset), Right: labelActionsWidth + s(chatSidebarRowInset)},
		// A shrink-wrapped flex sits on the row top. Align centers the icon, label, and chevron.
		Child: woxwidget.Align{Height: height, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: s(chatSidebarIconGap), CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: label}},
	})
	activeColor := theme.PreviewText
	if hovered {
		activeColor = theme.SelectedTitle
	}
	idleColor := theme.ResultSubtitle
	var newIcon, editIcon, deleteIcon, newIdle, editIdle, deleteIdle woxwidget.Widget
	if hovered || newFocused {
		newIcon = woxcomponent.AddGlyph(s(chatSidebarActionGlyph), activeColor)
		newIdle = woxcomponent.AddGlyph(s(chatSidebarActionGlyph), idleColor)
	}
	if hovered || editFocused {
		editIcon = woxcomponent.EditGlyph(s(chatSidebarActionGlyph), activeColor)
		editIdle = woxcomponent.EditGlyph(s(chatSidebarActionGlyph), idleColor)
	}
	if hovered || deleteFocused {
		deleteIcon = woxcomponent.DeleteGlyph(s(chatSidebarActionGlyph), theme.ErrorText)
		deleteIdle = woxcomponent.DeleteGlyph(s(chatSidebarActionGlyph), theme.PreviewText)
	}
	newID := item.SelectID + "-new"
	if item.Kind == "projects-section" {
		newID = "chat-projects-section-new"
	}
	var actions []woxwidget.Widget
	if item.OnNew != nil {
		actions = append(actions, woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: newID, Label: item.NewLabel, Icon: newIcon, IdleIcon: newIdle, Width: s(chatSidebarActionSize), Height: s(chatSidebarActionSize), OnTap: item.OnNew,
			OnFocusChange: onNewFocus, FocusRingColor: theme.Cursor,
		}))
	}
	if item.OnEdit != nil {
		actions = append(actions, woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: item.SelectID + "-edit", Label: item.EditLabel, Icon: editIcon, IdleIcon: editIdle, Width: s(chatSidebarActionSize), Height: s(chatSidebarActionSize), OnTap: item.OnEdit,
			OnFocusChange: onEditFocus, FocusRingColor: theme.Cursor,
		}))
	}
	if item.OnDelete != nil {
		actions = append(actions, woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: item.SelectID + "-delete", Label: item.DeleteLabel, Icon: deleteIcon, IdleIcon: deleteIdle, Width: s(chatSidebarActionSize), Height: s(chatSidebarActionSize), OnTap: item.OnDelete,
			OnFocusChange: onDeleteFocus, FocusRingColor: theme.Cursor,
		}))
	}
	return woxwidget.Semantics{AutomationID: item.SelectID + "-group", Role: woxui.AccessibilityRoleGroup, Label: item.Title, Expanded: !item.Collapsed, Hovered: hovered,
		Child: woxwidget.Gesture{ID: item.SelectID + "-hover", CoverHover: true, OnHover: onHover, Child: woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
			{Child: row},
			{Right: s(4), AnchorRight: true, Child: woxwidget.Align{Width: reserved, Height: height, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: actions}}},
		}}},
	}
}
