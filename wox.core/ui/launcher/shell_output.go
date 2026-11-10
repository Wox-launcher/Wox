package launcher

import (
	"sort"
	"strings"

	"wox/plugin/system/shell/interactive"
	woxcomponent "wox/ui/launcher/component"
	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"

	"github.com/rivo/uniseg"
)

// shellOutputLayout indexes wrapped rows but materializes cells only for the visible viewport.
type shellOutputLayout struct {
	command, text, failure string
	value                  string
	prefix, columns        int
	rows                   []int
	cellHeight             float32
}

// prepare keeps stable rows on append and reparses the final row for partial graphemes and CRLF.
func (l *shellOutputLayout) prepare(command, text, failure string, columns int) {
	if l.columns == columns && l.command == command && l.text == text && l.failure == failure {
		return
	}
	prefix := ""
	if command != "" {
		prefix = "> " + command + "\n"
	}
	value := prefix + text
	if failure != "" {
		value += "\n" + failure
	}
	rows := l.rows[:0]
	offset := 0
	if l.columns == columns && l.command == command && l.failure == "" && len(l.rows) > 0 && strings.HasPrefix(text, l.text) {
		lastStableRow := max(0, len(l.rows)-2)
		offset = l.rows[lastStableRow]
		rows = l.rows[:lastStableRow]
	}
	rows = append(rows, offset)
	column := 0
	for rest := value[offset:]; len(rest) > 0; {
		cluster, next, width, _ := uniseg.FirstGraphemeClusterInString(rest, -1)
		switch cluster {
		case "\r", "\n", "\r\n":
			rows = append(rows, offset+len(cluster))
			column = 0
		default:
			if cluster == "\t" {
				width = max(1, min(8-column%8, columns-column))
			}
			width = min(width, columns)
			if column+width > columns {
				rows = append(rows, offset)
				column = 0
				if cluster == "\t" {
					width = min(8, columns)
				}
			}
			column += width
		}
		offset += len(cluster)
		rest = next
	}
	l.command, l.text, l.failure = command, text, failure
	l.value, l.prefix, l.columns, l.rows = value, len(prefix), columns, rows
}

func (l *shellOutputLayout) rowAt(offset int) int {
	return max(0, sort.Search(len(l.rows), func(i int) bool { return l.rows[i] > offset })-1)
}

// frame displays saved text as cells without feeding escape sequences or historical input to the PTY.
func (l *shellOutputLayout) frame(size interactive.Size, top int, output terminalPreviewSnapshot, theme woxcomponent.Theme) (*interactive.Frame, []int) {
	cells := make([]interactive.Cell, size.Rows*size.Columns)
	offsets := make([]int, len(cells))
	first, last := min(output.SelectionAnchor, output.SelectionFocus), max(output.SelectionAnchor, output.SelectionFocus)
	match := sort.Search(len(output.Matches), func(i int) bool { return output.Matches[i].end > l.rows[top]-l.prefix })
	for row := range size.Rows {
		start, end := len(l.value), len(l.value)
		if top+row < len(l.rows) {
			start = l.rows[top+row]
			if top+row+1 < len(l.rows) {
				end = l.rows[top+row+1]
			}
		}
		column := 0
		offset := start
		for rest := l.value[start:end]; len(rest) > 0 && column < size.Columns; {
			cluster, next, width, _ := uniseg.FirstGraphemeClusterInString(rest, -1)
			rest = next
			startByte := offset - l.prefix
			offset += len(cluster)
			endByte := offset - l.prefix
			if cluster == "\n" || cluster == "\r" || cluster == "\r\n" {
				offset -= len(cluster)
				break
			}
			cell := interactive.Cell{Text: cluster}
			if cluster == "\t" {
				cell.Text = ""
				width = 8 - column%8
			}
			width = min(width, size.Columns-column)
			if width <= 0 {
				continue
			}
			for match < len(output.Matches) && output.Matches[match].end <= startByte {
				match++
			}
			if match < len(output.Matches) && output.Matches[match].start < endByte {
				cell.Background = shellOutputRGBA(theme.Controls.TextSelectionBackground)
				if match == output.MatchIndex {
					cell.Background = shellOutputRGBA(theme.Controls.Focus)
				}
				cell.Foreground = shellOutputRGBA(theme.Controls.TextSelectionText)
			}
			if first < last && startByte < last && endByte > first {
				cell.Background = shellOutputRGBA(theme.Controls.TextSelectionBackground)
				cell.Foreground = shellOutputRGBA(theme.Controls.TextSelectionText)
			}
			cell.Wide = cell.Text != "" && width == 2
			index := row*size.Columns + column
			cells[index] = cell
			for i := range width {
				offsets[index+i] = min(len(output.Text), max(0, startByte))
				if i > 0 {
					cells[index+i] = interactive.Cell{Continuation: cell.Wide, Background: cell.Background}
				}
			}
			column += width
		}
		for ; column < size.Columns; column++ {
			offsets[row*size.Columns+column] = min(len(output.Text), max(0, offset-l.prefix))
		}
	}
	return &interactive.Frame{Size: size, Cells: cells}, offsets
}

func shellOutputRGBA(color woxui.Color) uint32 {
	return uint32(color.R)<<24 | uint32(color.G)<<16 | uint32(color.B)<<8 | uint32(color.A)
}

// buildShellOutputPreview uses the live Shell viewport for historical and one-shot output.
func (a *App) buildShellOutputPreview(output terminalPreviewSnapshot, theme woxcomponent.Theme, width, height, scale float32) woxwidget.Widget {
	controls := theme.Controls
	rowHeight := controls.Scaled(34)
	searchHeight := float32(0)
	if output.SearchOpen {
		searchHeight = 58
	}
	props, size := a.shellViewportProps(theme, width, height, scale, rowHeight+searchHeight)
	layout := &a.terminal.terminalLayout
	layout.prepare(output.Command, output.Text, output.Error, size.Columns)
	layout.cellHeight = props.CellHeight
	maxTop := max(0, len(layout.rows)-size.Rows)
	maxScroll := float32(maxTop) * props.CellHeight
	top := int(min(float32(maxTop), max(0, output.Scroll/props.CellHeight)))
	a.terminal.clampTerminalPreviewScroll(maxScroll)
	frame, offsets := layout.frame(size, top, output, theme)
	props.Frame = frame
	label := output.Command
	if label == "" {
		label = a.language.translate("i18n:plugin_shell_output")
	}
	props.Output = &previewview.ShellOutputProps{
		SessionID: output.SessionID, Label: label, Text: output.Text,
		SelectionAnchor: output.SelectionAnchor, SelectionFocus: output.SelectionFocus,
		CellOffset:       func(index int) int { return offsets[index] },
		OnSelectionStart: a.terminal.startTerminalSelection, OnSelectionExtend: a.terminal.extendTerminalSelection,
		OnSelectWord: a.terminal.selectTerminalWord, OnSelectLine: a.terminal.selectTerminalLine,
		OnCopy: a.terminal.copyTerminalPreviewOutput, OnSelectAll: a.terminal.selectAllTerminalPreview,
		OnContextMenu: a.terminal.openTerminalContextMenu,
		OnScroll:      func(delta float32) { a.terminal.scrollTerminalPreview(delta, maxScroll) },
	}
	status := output.Status
	if status == "" {
		status = "idle"
	}
	statusLabel := ""
	switch status {
	case "running":
		statusLabel = a.language.translate("i18n:plugin_shell_status_running")
	case "completed":
		statusLabel = a.language.translate("i18n:plugin_shell_status_success")
	case "failed":
		statusLabel = a.language.translate("i18n:plugin_shell_status_failed")
	case "killed":
		statusLabel = a.language.translate("i18n:plugin_shell_status_killed")
	}
	if output.LoadingHistory {
		statusLabel = "…"
	}
	props.Footer = previewview.ShellOutputToolbar(previewview.ShellOutputToolbarProps{
		Width: max(0, width-2*props.Padding), RowHeight: rowHeight, SearchHeight: searchHeight, Theme: theme, Window: a.window,
		SessionID: output.SessionID, Status: status, StatusLabel: statusLabel, SearchOpen: output.SearchOpen,
		SearchEditing: output.SearchEditing, CaseSensitive: output.CaseSensitive, MatchCount: output.MatchCount, MatchIndex: output.MatchIndex,
		Fullscreen: a.terminal.terminalFullscreen, SearchHotkey: strings.Join(formatHotkeyLabels(primaryHotkey("shift+f")), "+"),
		FullscreenHotkey: strings.Join(formatHotkeyLabels(primaryHotkey("b")), "+"), OnTagHover: a.previews.setPreviewTooltip,
		OnOpenSearch: a.terminal.openTerminalSearch, OnCloseSearch: a.terminal.closeTerminalSearch,
		OnSetSearch: a.terminal.setTerminalSearchQuery, OnSearchChanged: func(value string) { _ = a.terminal.setTerminalSearchQuery(value) },
		OnSearchKey: a.terminal.onTerminalPreviewKey, OnMoveSearch: a.terminal.moveTerminalSearch,
		OnToggleSearchCase: a.terminal.toggleTerminalSearchCase, OnToggleFullscreen: a.terminal.toggleTerminalFullscreen,
	})
	return previewview.ShellPreviewView(props)
}
