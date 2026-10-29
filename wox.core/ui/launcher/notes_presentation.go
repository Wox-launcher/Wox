package launcher

import (
	"fmt"
	"runtime"
	"strings"
	"unicode/utf8"

	"wox/common"
	notesplugin "wox/plugin/system/notes"
	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

// buildNotes derives this window's complete widget tree from controller snapshot state.
func (c *notesWindowController) buildNotes(frame woxui.FrameInfo) woxwidget.Widget {
	a := c.owner
	theme := a.palette().componentTheme()
	findQuery, findTarget := c.currentFind()
	toolbar := c.buildToolbar(frame.Size.Width, frame.WindowFocused, theme)
	formatHeight := float32(0)
	var formatBar woxwidget.Widget
	if c.formatVisible {
		formatHeight = launcherview.NotesFormatBarHeight
		formatBar = c.buildFormatBar(frame.Size.Width, theme)
	}
	statusHeight := float32(0)
	var status woxwidget.Widget
	if c.errorText != "" || c.saving {
		statusHeight = launcherview.NotesStatusHeight
		status = c.buildStatus(frame.Size.Width, theme)
	}
	editorHeight := max(float32(0), frame.Size.Height-launcherview.NotesToolbarHeight-formatHeight-statusHeight)
	var editor woxwidget.Widget
	if c.markdownView {
		editor = c.buildMarkdownEditor(frame.Size.Width, editorHeight, theme)
	} else {
		editor = woxcomponent.WoxNoteEditor(woxcomponent.NoteEditorProps{
			EdgeFade: 24, FindQuery: findQuery, FindTarget: findTarget, FindReveal: c.findReveal,
			ID: "notes.editor", Label: a.language.translate("i18n:notes_editor"), Document: c.document,
			Width: frame.Size.Width, Height: editorHeight, Padding: c.findEditorPadding(),
			Style: c.editorStyle(), LineHeight: notesEditorLineHeight * c.zoom, Zoom: c.zoom, TextColor: theme.PreviewText, Theme: theme.Controls,
			Window: c.managed.Window(), ReadOnly: c.record.DeletedAt > 0, Autofocus: true, Controller: c.editor,
			FocusNode: c.editorFocus, Focused: (c.editorFocus.HasFocus() || c.requestTextFocus) && c.focusedTableBlock < 0 && c.focusedImageBlock < 0, Selection: c.selection,
			RevealCaret:        c.revealEditorCaret,
			ActiveSegmentStart: c.activeTextSegment.Start, OnTextFocus: c.focusNoteText,
			OnChanged: c.onSegmentChanged, OnSelectionChanged: c.onEditorSelectionChanged,
			OnSelectAll: c.selectEntireDocument, OnCopy: c.copyDocumentSelection, OnCut: c.cutDocumentSelection,
			DocumentSelected: c.documentSelected,
			OnTapOffset:      c.handleBlockTap, OnTapBelowText: c.appendParagraphBelowText, CursorAtOffset: c.editorCursorAt, OnKey: c.onKey,
			OnUndo: c.undoDocument, OnRedo: c.redoDocument, OnPaste: c.pasteFromClipboard,
			TransformPaste: func(value string) string {
				converted := notesplugin.ToMarkdown(notesplugin.ParseMarkdown(value))
				if strings.TrimSpace(converted) == "" && strings.TrimSpace(value) != "" {
					return value
				}
				return converted
			},
			OnTableChange: c.replaceTable, OnTableFocus: c.focusTableCell, OnTableKey: c.onTableKey, OnTablePaste: c.pasteTableClipboard,
			OnDeleteEmptySegment: c.deleteEmptyTextSegment,
			OnTableInsertRow:     c.tableInsertRow, OnTableInsertColumn: c.tableInsertColumn, OnTableDeleteRow: c.tableDeleteRow,
			OnTableDeleteColumn: c.tableDeleteColumn, OnTableDelete: c.tableDelete, OnTableActionHover: c.updateTableActionTooltip,
			TableActionLabels: woxcomponent.NoteTableActionLabels{
				InsertRow: c.owner.language.translate("i18n:notes_table_insert_row"), InsertColumn: c.owner.language.translate("i18n:notes_table_insert_column"),
				DeleteRow: c.owner.language.translate("i18n:notes_table_delete_row"), DeleteColumn: c.owner.language.translate("i18n:notes_table_delete_column"),
				DeleteTable: c.owner.language.translate("i18n:notes_table_delete"),
			},
			FocusedTableBlock: c.focusedTableBlock, FocusedTableRow: c.focusedTableRow, FocusedTableCol: c.focusedTableCol,
			ResolveImage: c.resolveNoteImage, ReleaseImage: c.releaseNoteImage, MissingImageLabel: a.language.translate("i18n:notes_image_missing"),
			FocusedImageBlock: c.focusedImageBlock, OnImageFocus: c.focusImage, OnImageLeave: c.focusTextBesideImage, OnImageScale: c.scaleImage, OnImageDelete: c.deleteImage,
			OnImageActionHover: c.updateTableActionTooltip,
			ImageActionLabels: woxcomponent.NoteImageActionLabels{
				Smaller: c.owner.language.translate("i18n:notes_image_smaller"), Larger: c.owner.language.translate("i18n:notes_image_larger"),
				Delete: c.owner.language.translate("i18n:notes_image_delete"),
			},
			ReorderTaskLabel: a.language.translate("i18n:notes_reorder_task"), ReorderingTask: c.taskReorderFrom, HoveredTask: c.taskHover, ReorderDest: c.taskReorderTo,
			OnHoverTask: c.hoverTask, OnReorderTaskStart: c.startTaskReorder, OnReorderTaskDrag: c.dragTaskReorder, OnReorderTaskEnd: c.endTaskReorder,
		})
	}
	var overlay woxwidget.Widget
	if c.linkOpen {
		overlay = c.buildLinkOverlay(frame.Size, theme)
	} else if c.searchOpen {
		overlay = c.buildSearchOverlay(frame.Size, theme)
	} else if c.moreOpen {
		overlay = c.buildMoreOverlay(frame.Size, theme)
	}
	if c.findOpen {
		overlay = c.buildFindOverlay(frame.Size, theme, overlay)
	}
	overlay = c.composeLinuxInlineTooltip(frame.Size, theme, overlay)
	return launcherview.NotesWindow(launcherview.NotesWindowProps{
		Width: frame.Size.Width, Height: frame.Size.Height, Label: a.language.translate("i18n:notes_title"), Toolbar: toolbar, Editor: editor,
		FormatBar: formatBar, Status: status, Overlay: overlay, Theme: theme,
	})
}

// buildMarkdownEditor hosts the raw Markdown source instead of the rich preview.
func (c *notesWindowController) buildMarkdownEditor(width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
	query, target := c.currentFind()
	active := -1
	if target != nil {
		active = target.Offset
	}
	padding := c.findEditorPadding()
	// Source and preview share the UI font so toggling views does not change typeface.
	style := c.editorStyle()
	lineHeight := notesEditorLineHeight * c.zoom
	fieldHeight := max(height, woxcomponent.TextFieldVisualHeight(c.editor.Text(), c.managed.Window(), style, width, lineHeight, padding, nil))
	return woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		EdgeFade: 24,
		Key:      "notes.editor.scroll", AutomationID: "notes.editor.scroll", Label: c.owner.language.translate("i18n:notes_editor"),
		Width: width, Height: height, KeepVisible: c.noteMarkdownCaretVisible(width, lineHeight, padding),
		Content: woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
			ID: "notes.editor.markdown", Label: c.owner.language.translate("i18n:notes_editor"), Width: width, Height: fieldHeight,
			Padding: padding, Transparent: true, DisableHover: true,
			Style: style, LineHeight: lineHeight,
			TextAlignmentY: 0.5, TextColor: theme.PreviewText, Value: c.editor.Text(), Controller: c.editor,
			FocusNode: c.editorFocus, Focused: c.editorFocus.HasFocus(), Autofocus: true,
			ReadOnly: c.record.DeletedAt > 0, MaxLines: 10000, Window: c.managed.Window(), Theme: theme.Controls,
			Find:      &woxcomponent.TextFieldFind{Query: query, Active: active},
			OnChanged: c.onMarkdownChanged, OnKey: c.onKey, OnUndo: c.undoDocument, OnRedo: c.redoDocument,
			OnPaste: c.pasteFromClipboard,
		}),
		Theme: theme.Controls, ThumbColor: theme.ResultSubtitle,
	})
}

// noteMarkdownCaretVisible is the source-editor scroller interval that contains the caret.
func (c *notesWindowController) noteMarkdownCaretVisible(width, lineHeight float32, padding woxwidget.Insets) *woxwidget.ScrollRange {
	_, target := c.currentFind()
	if (!c.revealEditorCaret && (!c.findReveal || target == nil)) || lineHeight <= 0 {
		return nil
	}
	innerWidth := max(float32(0), width-padding.Left-padding.Right)
	offset := c.editor.State().Selection.Focus
	if target != nil && c.findReveal {
		offset = target.Offset
	}
	line := woxcomponent.TextFieldVisualLineIndex(c.editor.Text(), offset, c.noteMeasureWindow(), c.editorStyle(), innerWidth, nil)
	top := padding.Top + float32(line)*lineHeight
	start := top
	if target != nil {
		start = max(float32(0), top-c.controlTheme().Scaled(44))
	}
	return &woxwidget.ScrollRange{Start: start, End: top + lineHeight}
}

// buildToolbar prepares labels and native actions for the pure toolbar view.
func (c *notesWindowController) buildToolbar(width float32, active bool, theme woxcomponent.Theme) woxwidget.Widget {
	title := c.owner.language.translate("i18n:notes_untitled")
	if custom := notesplugin.CustomNoteTitle(c.document); custom != "" {
		title = custom
	}
	searchLabel := c.owner.language.translate("i18n:notes_search") + " (" + strings.Join(formatHotkeyLabels(primaryHotkey("p")), "+") + ")"
	pinHotkey := "(" + strings.Join(formatHotkeyLabels(primaryHotkey("shift+p")), "+") + ")"
	pinLabel := c.owner.language.translate("i18n:notes_pin_window") + " " + pinHotkey
	if c.windowPinned {
		pinLabel = c.owner.language.translate("i18n:notes_unpin_window") + " " + pinHotkey
	}
	newLabel := c.owner.language.translate("i18n:notes_new") + " (" + strings.Join(formatHotkeyLabels(primaryHotkey("n")), "+") + ")"
	return launcherview.NotesToolbar(launcherview.NotesToolbarProps{
		Width: width, Active: active, Platform: runtime.GOOS, Pinned: c.windowPinned,
		Maximized: c.windowMaximized, FormatVisible: c.formatVisible, Title: title,
		Label: c.owner.language.translate("i18n:notes_title"), SearchLabel: searchLabel, PinLabel: pinLabel, NewLabel: newLabel,
		FormatLabel: c.owner.language.translate("i18n:notes_format"), MoreLabel: c.owner.language.translate("i18n:notes_more"),
		Icon: notesWindowIcon(), Theme: theme, OnHover: c.updateToolbarTooltip,
		OnSearch: c.toggleSearch, OnPin: c.toggleWindowPin,
		OnFormat: func() { c.formatVisible = !c.formatVisible; c.invalidate() },
		OnNew:    func() { c.runAction(c.owner.openNewNoteWindow) },
		OnMore:   func() { c.moreOpen = !c.moreOpen; c.formatMore = false; c.searchOpen = false; c.invalidate() },
		OnDrag: func() {
			if c.windowMaximized {
				c.restoreFromMaximize()
			}
			if c.managed != nil {
				_ = c.managed.Window().StartDragging()
			}
		},
		OnMaximize: c.toggleMaximize, OnMinimize: c.minimizeWindow, OnClose: c.requestClose,
	})
}

// notesEditorPadding insets the rich and Markdown editors from the window frame.
func notesEditorPadding() woxwidget.Insets {
	return woxwidget.Insets{Left: notesEditorPaddingLeft, Top: notesEditorPaddingTop, Right: notesEditorPaddingRight, Bottom: notesEditorPaddingBottom}
}

// updateToolbarTooltip keeps Notes chrome hints in the same hover path as settings.
// Linux cannot position a native overlay window, so the owner window paints the
// same in-window tooltip settings already uses. Other platforms keep the native overlay.
func (c *notesWindowController) updateToolbarTooltip(inside bool, text string, anchor woxui.Rect) {
	if util.IsLinux() {
		c.owner.tooltips.scheduleLinuxInlineTooltip(linuxInlineTooltipTarget{
			revision: &c.tooltipRev, state: &c.inlineTooltip,
			open: true, invalidate: c.invalidate, job: "show Notes inline tooltip",
		}, inside, text, anchor, "top")
		return
	}
	c.owner.tooltips.setNativeHoverTooltip(&c.tooltipRev, "go-ui-notes-titlebar", "update Notes chrome tooltip", inside, text, anchor, "top", func() *woxui.Window {
		if c.managed == nil {
			return nil
		}
		return c.managed.Window()
	})
}

// composeLinuxInlineTooltip stacks the in-window Linux tooltip above search, link, or more overlays.
func (c *notesWindowController) composeLinuxInlineTooltip(size woxui.Size, theme woxcomponent.Theme, overlay woxwidget.Widget) woxwidget.Widget {
	if c.inlineTooltip == nil {
		return overlay
	}
	var notesWindow *woxui.Window
	if c.managed != nil {
		notesWindow = c.managed.Window()
	}
	tooltip, left, top := launcherview.SettingsInlineTooltipOverlay(launcherview.SettingsInlineTooltipProps{
		Width: size.Width, Height: size.Height, Anchor: c.inlineTooltip.Anchor, Message: c.inlineTooltip.Text, Side: c.inlineTooltip.Side, Theme: theme.Controls,
		Window: notesWindow,
	})
	return launcherview.NotesTooltipOverlay(size, overlay, tooltip, left, top)
}

// buildFormatBar snapshots active marks and prepares document commands.
func (c *notesWindowController) buildFormatBar(width float32, theme woxcomponent.Theme) woxwidget.Widget {
	formats := noteActiveFormats(c.document, c.blockRanges, c.selection)
	if c.focusedTableBlock >= 0 {
		formats = noteActiveFormatsForTable(c.document, c.focusedTableBlock, c.focusedTableRow, c.focusedTableCol)
	}
	item := func(id string, action func()) launcherview.NotesFormatAction {
		return launcherview.NotesFormatAction{ID: id, Label: c.owner.language.translate("i18n:notes_format_" + id), Active: formats[id], OnTap: action}
	}
	items := []launcherview.NotesFormatAction{
		// Heading cycles the current block through paragraph, H1-H3, and code.
		item("block", func() { c.cycleBlock() }),
		item("bold", func() { c.toggleInline("bold") }),
		item("italic", func() { c.toggleInline("italic") }),
		item("underline", func() { c.toggleInline("underline") }),
		item("strike", func() { c.toggleInline("strike") }),
		item("code", func() { c.toggleInline("code") }),
		item("link", c.openLink),
		item("bullet", func() { c.setBlock(common.NoteBlockBullet) }),
		item("ordered", func() { c.setBlock(common.NoteBlockOrdered) }),
		item("task", func() { c.setBlock(common.NoteBlockTask) }),
		item("quote", func() { c.setBlock(common.NoteBlockQuote) }),
		item("divider", func() { c.setBlock(common.NoteBlockDivider) }),
		item("table", c.insertTable),
	}
	stats := c.noteCharacterCountLabel()
	return launcherview.NotesFormatBar(launcherview.NotesFormatBarProps{
		Width: width, Stats: stats, StatsWidth: c.measureFormatBarStats(stats), Markdown: c.markdownView,
		ReadOnly: c.record.DeletedAt > 0, Actions: items, Theme: theme, OnHover: c.updateToolbarTooltip,
		More: item("more", func() { c.moreOpen, c.formatMore = true, true; c.invalidate() }),
	})
}

// measureFormatBarStats returns the character-count label width, with a conservative
// fallback before the native window can measure.
func (c *notesWindowController) measureFormatBarStats(stats string) float32 {
	style := woxui.TextStyle{Size: 11}
	if c != nil && c.managed != nil {
		if metrics, err := c.managed.Window().MeasureText(stats, style); err == nil && metrics.Size.Width > 0 {
			return metrics.Size.Width
		}
	}
	return float32(utf8.RuneCountInString(stats)) * 6
}

// noteCharacterCountLabel is the live document length shown on the format bar.
func (c *notesWindowController) noteCharacterCountLabel() string {
	return fmt.Sprintf(c.owner.language.translate("i18n:notes_character_count"), notesplugin.DocumentCharacterCount(c.document))
}

func (c *notesWindowController) buildStatus(width float32, theme woxcomponent.Theme) woxwidget.Widget {
	text, color := c.errorText, theme.ErrorText
	if c.saving {
		text, color = c.owner.language.translate("i18n:notes_saving"), theme.ResultSubtitle
	}
	var retry func()
	if c.errorText != "" && c.dirty {
		retry = func() { c.runAction(c.flush) }
	}
	return launcherview.NotesStatus(width, text, c.owner.language.translate("i18n:notes_retry"), color, theme, retry)
}

// buildSearchOverlay snapshots list order and labels before composing the browser.
func (c *notesWindowController) buildSearchOverlay(size woxui.Size, theme woxcomponent.Theme) woxwidget.Widget {
	items := c.searchItems()
	rows := make([]launcherview.NotesSearchItem, 0, len(items))
	for _, summary := range items {
		rows = append(rows, c.searchRowProps(summary))
	}
	return launcherview.NotesSearchOverlay(launcherview.NotesSearchProps{
		Size: size, Items: rows, Selected: c.searchIndex, Theme: theme,
		Label: c.owner.language.translate("i18n:notes_search"), Hint: c.owner.language.translate("i18n:notes_search_placeholder"),
		EmptyLabel: c.owner.language.translate("i18n:notes_no_results"), CloseLabel: c.owner.language.translate("i18n:close"),
		Value: c.searchEditor.Text(), Editor: c.searchEditor, Focus: c.searchFocus, Window: c.noteMeasureWindow(),
		OnChanged: func(string) { _ = c.reloadSummaries(); c.searchIndex = 0; c.invalidate() }, OnKey: c.onSearchKey,
		OnDismiss: func() { c.searchOpen = false; c.invalidate() },
	})
}

// searchRowProps resolves domain labels and captures the selected record for activation.
func (c *notesWindowController) searchRowProps(summary common.NoteSummary) launcherview.NotesSearchItem {
	label := summary.Title
	if summary.DeletedAt > 0 {
		label += " · " + c.owner.language.translate("i18n:notes_restore")
	}
	return launcherview.NotesSearchItem{ID: summary.ID, Label: label, Updated: util.FormatTimestamp(summary.UpdatedAt),
		Section: c.searchSectionLabel(summary), OnTap: func() { c.openSearchItem(summary) }}
}

func (c *notesWindowController) searchSectionLabel(item common.NoteSummary) string {
	switch {
	case item.DeletedAt > 0:
		return c.owner.language.translate("i18n:notes_deleted")
	case item.PinnedAt > 0:
		return c.owner.language.translate("i18n:notes_favorites")
	default:
		return c.owner.language.translate("i18n:notes_recent")
	}
}

func (c *notesWindowController) buildMoreOverlay(size woxui.Size, theme woxcomponent.Theme) woxwidget.Widget {
	width := float32(240)
	if c.formatMore {
		rows := []woxwidget.Widget{
			c.menuRow("format-bullet", c.owner.language.translate("i18n:notes_format_bullet"), width, theme, func() { c.setBlock(common.NoteBlockBullet) }),
			c.menuRow("format-ordered", c.owner.language.translate("i18n:notes_format_ordered"), width, theme, func() { c.setBlock(common.NoteBlockOrdered) }),
			c.menuRow("format-task", c.owner.language.translate("i18n:notes_format_task"), width, theme, func() { c.setBlock(common.NoteBlockTask) }),
			c.menuRow("format-quote", c.owner.language.translate("i18n:notes_format_quote"), width, theme, func() { c.setBlock(common.NoteBlockQuote) }),
			c.menuRow("format-divider", c.owner.language.translate("i18n:notes_format_divider"), width, theme, func() { c.setBlock(common.NoteBlockDivider) }),
			c.menuRow("format-table", c.owner.language.translate("i18n:notes_format_table"), width, theme, c.insertTable),
		}
		if c.focusedTableBlock >= 0 {
			rows = append(rows,
				c.menuRow("table-insert-row", c.owner.language.translate("i18n:notes_table_insert_row"), width, theme, c.insertTableRow),
				c.menuRow("table-insert-column", c.owner.language.translate("i18n:notes_table_insert_column"), width, theme, c.insertTableColumn),
				c.menuRow("table-delete-row", c.owner.language.translate("i18n:notes_table_delete_row"), width, theme, c.deleteTableRow),
				c.menuRow("table-delete-column", c.owner.language.translate("i18n:notes_table_delete_column"), width, theme, c.deleteTableColumn),
				c.menuRow("table-delete", c.owner.language.translate("i18n:notes_table_delete"), width, theme, c.deleteFocusedTable),
			)
		}
		return c.moreOverlay(size, width, rows, theme)
	}
	rows := []woxwidget.Widget{
		c.menuRow("view", c.viewModeMenuLabel(), width, theme, c.toggleMarkdownView),
		c.menuRow("copy-link", c.owner.language.translate("i18n:notes_copy_link"), width, theme, c.copyLink),
		c.menuRow("export-md", c.owner.language.translate("i18n:notes_export_markdown"), width, theme, func() { c.export("md") }),
		c.menuRow("export-txt", c.owner.language.translate("i18n:notes_export_text"), width, theme, func() { c.export("txt") }),
		c.menuRow("export-html", c.owner.language.translate("i18n:notes_export_html"), width, theme, func() { c.export("html") }),
	}
	if c.record.DeletedAt > 0 {
		rows = append(rows, c.menuRow("restore", c.owner.language.translate("i18n:notes_restore"), width, theme, c.restore))
	} else {
		rows = append(rows, c.menuRow("delete", c.owner.language.translate("i18n:notes_delete"), width, theme, c.delete))
	}
	return c.moreOverlay(size, width, rows, theme)
}

func (c *notesWindowController) moreOverlay(size woxui.Size, width float32, rows []woxwidget.Widget, theme woxcomponent.Theme) woxwidget.Widget {
	return launcherview.NotesMenuOverlay(size, width, rows, theme, c.owner.language.translate("i18n:close"), func() { c.moreOpen, c.formatMore = false, false; c.invalidate() })
}

func (c *notesWindowController) menuRow(id, label string, width float32, theme woxcomponent.Theme, action func()) woxwidget.Widget {
	return launcherview.NotesMenuRow(id, label, width, theme, func() { c.moreOpen = false; action(); c.invalidate() })
}

// buildLinkOverlay supplies the link draft and event handlers to its view.
func (c *notesWindowController) buildLinkOverlay(size woxui.Size, theme woxcomponent.Theme) woxwidget.Widget {
	return launcherview.NotesLinkOverlay(launcherview.NotesLinkProps{
		Size: size, Theme: theme, Label: c.owner.language.translate("i18n:notes_link"), CancelLabel: c.owner.language.translate("i18n:cancel"),
		ApplyLabel: c.owner.language.translate("i18n:notes_apply"), CloseLabel: c.owner.language.translate("i18n:close"),
		Value: c.linkEditor.Text(), Editor: c.linkEditor, Focus: c.linkFocus, Window: c.noteMeasureWindow(), OnKey: c.onLinkKey,
		OnDismiss: func() { c.linkOpen = false; c.invalidate() }, OnApply: c.applyLink,
	})
}

// viewModeMenuLabel names the Markdown/preview toggle with its platform shortcut.
func (c *notesWindowController) viewModeMenuLabel() string {
	key := "i18n:notes_view_markdown"
	if c.markdownView {
		key = "i18n:notes_view_preview"
	}
	return c.owner.language.translate(key) + " (" + strings.Join(formatHotkeyLabels(primaryHotkey("e")), "+") + ")"
}

func (c *notesWindowController) updateTableActionTooltip(inside bool, label string, bounds woxui.Rect) {
	c.updateToolbarTooltip(inside, label, bounds)
}
