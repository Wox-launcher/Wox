package preview

import (
	"fmt"
	"strings"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// ShellOutputProps maps the shared Shell grid back to the stored output for selection and copy.
type ShellOutputProps struct {
	SessionID, Label, Text          string
	SelectionAnchor, SelectionFocus int
	CellOffset                      func(int) int
	OnSelectionStart                func(int, woxui.KeyModifiers)
	OnSelectionExtend               func(int)
	OnSelectWord, OnSelectLine      func(int)
	OnCopy, OnSelectAll             func()
	OnContextMenu                   func(woxui.Point)
	OnScroll                        func(float32)
}

// shellOutputGrid provides read-only output gestures on the same grid used by a live shell.
func shellOutputGrid(p ShellOutputProps, painter woxwidget.CaretPainter, cellAt func(woxui.Point) int) woxwidget.Widget {
	gesture := woxwidget.Gesture{
		ID: string(TerminalOutputKey(p.SessionID)), Child: painter, Cursor: woxui.PointerCursorText,
		OnScroll: func(delta woxui.Point) { p.OnScroll(-delta.Y) },
	}
	output := woxwidget.Semantics{
		Key: TerminalOutputKey(p.SessionID), AutomationID: "launcher.preview.terminal.output", Role: woxui.AccessibilityRoleText,
		Label: p.Label, Value: p.Text, ReadOnly: true,
	}
	if strings.TrimSpace(p.Text) != "" {
		offsetAt := func(point woxui.Point) int { return p.CellOffset(cellAt(point)) }
		gesture.OnSelectionStart = func(point woxui.Point, mods woxui.KeyModifiers) { p.OnSelectionStart(offsetAt(point), mods) }
		gesture.OnSelectionExtend = func(point woxui.Point) {
			if point.Y < 0 {
				p.OnScroll(point.Y)
			} else if point.Y > painter.Height {
				p.OnScroll(point.Y - painter.Height)
			}
			p.OnSelectionExtend(offsetAt(point))
		}
		gesture.OnDoubleTapAt = func(point woxui.Point) { p.OnSelectWord(offsetAt(point)) }
		gesture.OnTripleTapAt = func(point woxui.Point) { p.OnSelectLine(offsetAt(point)) }
		gesture.OnSecondaryTapDown = p.OnContextMenu
		output.HasTextSelection = true
		output.SelectionStart, output.SelectionEnd = terminalRuneSelection(p.Text, p.SelectionAnchor, p.SelectionFocus)
		output.Actions = []woxui.AccessibilityAction{woxui.AccessibilityActionSelectAll, woxui.AccessibilityActionCopy}
		output.OnAction = func(action woxui.AccessibilityAction, _ string) error {
			switch action {
			case woxui.AccessibilityActionSelectAll:
				p.OnSelectAll()
			case woxui.AccessibilityActionCopy:
				p.OnCopy()
			}
			return nil
		}
	}
	output.Child = gesture
	return output
}

// ShellOutputToolbarProps supplies the output status and preview-local find controls.
type ShellOutputToolbarProps struct {
	Width, RowHeight, SearchHeight         float32
	Theme                                  woxcomponent.Theme
	Window                                 *woxui.Window
	SessionID, Status, StatusLabel         string
	SearchHotkey, FullscreenHotkey         string
	SearchEditing                          woxui.TextEditingState
	SearchOpen, CaseSensitive, Fullscreen  bool
	MatchCount, MatchIndex                 int
	OnOpenSearch, OnCloseSearch            func()
	OnToggleFullscreen, OnToggleSearchCase func()
	OnSetSearch                            func(string) error
	OnSearchChanged                        func(string)
	OnSearchKey                            func(woxui.KeyEvent) bool
	OnMoveSearch                           func(int)
	OnTagHover                             func(bool, string, woxui.Rect)
}

// TerminalSearchInputKey keeps controller focus routing aligned with the retained search field.
func TerminalSearchInputKey(sessionID string) woxwidget.Key {
	return woxwidget.Key("terminal-search-input-" + sessionID)
}

// ShellOutputToolbar keeps status and actions below the terminal grid.
func ShellOutputToolbar(p ShellOutputToolbarProps) woxwidget.Widget {
	size := p.Theme.Controls.Scaled(28)
	button := func(id, label, hotkey string, icon woxwidget.Widget, action func()) woxwidget.Widget {
		return woxwidget.Align{Width: size, Height: p.RowHeight, Vertical: 0.5, Child: woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: id + p.SessionID, Label: label, Icon: icon, Width: size, Height: size, Radius: size / 2,
			HoverBackground: previewColorWithOpacity(p.Theme.PreviewText, 0.1), FocusRingColor: p.Theme.Cursor, OnTap: action,
			OnHoverAt: func(inside bool, bounds woxui.Rect) {
				if p.OnTagHover != nil {
					p.OnTagHover(inside, hotkey, bounds)
				}
			},
		})}
	}
	status := woxwidget.Semantics{
		AutomationID: "launcher.preview.terminal.status", Role: woxui.AccessibilityRoleText, Label: p.StatusLabel,
		Value: p.Status, LiveRegion: woxui.AccessibilityLiveRegionPolite,
		Child: woxwidget.Align{Height: p.RowHeight, Vertical: 0.5, Child: woxwidget.Text{
			Value: p.StatusLabel, Style: woxui.TextStyle{Size: p.Theme.Controls.Scaled(woxcomponent.ResultSubtitleFontSize)}, Color: p.Theme.Controls.TextSecondary,
		}},
	}
	glyphSize := p.Theme.Controls.Scaled(18)
	row := woxwidget.Stack{Width: p.Width, Height: p.RowHeight, Children: []woxwidget.StackChild{
		{Right: 2 * size, StretchWidth: true, Child: status},
		{Right: size, AnchorRight: true, Child: button("terminal-search-open-", "Find", p.SearchHotkey, woxcomponent.SearchGlyph(glyphSize, p.Theme.PreviewText), p.OnOpenSearch)},
		{AnchorRight: true, Child: button("terminal-fullscreen-", "Toggle fullscreen", p.FullscreenHotkey, woxcomponent.FullscreenGlyph(glyphSize, p.Theme.PreviewText, p.Fullscreen), p.OnToggleFullscreen)},
	}}
	if !p.SearchOpen {
		return row
	}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{shellOutputSearchBar(p, p.SearchHeight), row}}
}
func shellOutputSearchBar(props ShellOutputToolbarProps, height float32) woxwidget.Widget {
	innerWidth := max(float32(0), props.Width-20)
	actionWidth := float32(32)
	countWidth := float32(46)
	gap := float32(5)
	inputWidth := max(float32(90), innerWidth-countWidth-actionWidth*4-gap*5)
	input := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: string(TerminalSearchInputKey(props.SessionID)), Label: "Find in terminal output", Width: inputWidth, Height: 34,
		Radius: 7, Padding: woxwidget.Insets{Left: 10, Top: 7, Right: 10, Bottom: 5}, Background: previewColorWithOpacity(props.Theme.Background, 0.2),
		BorderColor: props.Theme.PreviewSplit, BorderWidth: 1, FocusRingColor: props.Theme.Cursor, Value: props.SearchEditing.Text,
		Focused: true, Autofocus: true, MaxLines: 1, Window: props.Window, Theme: props.Theme.Controls,
		OnChanged: props.OnSearchChanged, OnSetValue: props.OnSetSearch, OnKey: props.OnSearchKey,
	})
	count := "0/0"
	if props.MatchCount > 0 {
		count = fmt.Sprintf("%d/%d", props.MatchIndex+1, props.MatchCount)
	}
	hoverBackground := previewColorWithOpacity(props.Theme.PreviewText, 0.1)
	button := func(id, label string, selected bool, icon woxwidget.Widget, action func()) woxwidget.Widget {
		background := woxui.Color{}
		if selected {
			background = props.Theme.SelectedBackground
		}
		return woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: id + "-" + props.SessionID, Label: label, Icon: icon, Width: actionWidth, Height: 34, Radius: 17,
			Background: background, HoverBackground: hoverBackground, FocusRingColor: props.Theme.Cursor, OnTap: action,
		})
	}
	panel := woxwidget.Container{Width: props.Width, Height: 50, Radius: 8, Color: previewColorWithOpacity(props.Theme.PreviewText, 0.035), BorderColor: previewColorWithOpacity(props.Theme.PreviewSplit, 0.75), BorderWidth: 1, Padding: woxwidget.Insets{Left: 10, Top: 8, Right: 10, Bottom: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: gap, Children: []woxwidget.Widget{
		input,
		woxwidget.Semantics{AutomationID: "launcher.preview.terminal.search.match-count", Role: woxui.AccessibilityRoleText, Label: "Terminal search matches", Value: count, LiveRegion: woxui.AccessibilityLiveRegionPolite,
			Child: woxwidget.Container{Width: countWidth, Height: 34, Padding: woxwidget.Insets{Left: 5, Top: 10}, Child: woxwidget.Text{Value: count, Style: woxui.TextStyle{Size: 9, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultSubtitle}}},
		button("terminal-search-previous", "Previous match", false, woxcomponent.ChevronGlyph(16, props.Theme.PreviewText, true), func() {
			if props.OnMoveSearch != nil {
				props.OnMoveSearch(-1)
			}
		}),
		button("terminal-search-next", "Next match", false, woxcomponent.ChevronGlyph(16, props.Theme.PreviewText, false), func() {
			if props.OnMoveSearch != nil {
				props.OnMoveSearch(1)
			}
		}),
		button("terminal-search-case", "Match case", props.CaseSensitive, woxwidget.Text{Value: "Aa", Style: woxui.TextStyle{Size: 10, Weight: woxui.FontWeightSemibold}, Color: props.Theme.PreviewText}, props.OnToggleSearchCase),
		button("terminal-search-close", "Close find", false, woxcomponent.CloseGlyph(16, props.Theme.PreviewText), props.OnCloseSearch),
	}}}
	return woxwidget.Container{Width: props.Width, Height: height, Padding: woxwidget.Insets{Bottom: 8}, Child: panel}
}
