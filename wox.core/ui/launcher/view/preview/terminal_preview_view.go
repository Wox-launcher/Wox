package preview

import (
	"fmt"
	"sort"
	"strings"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TerminalPreviewProps contains the immutable state and actions rendered by a terminal preview.
type TerminalPreviewProps struct {
	Width              float32
	Height             float32
	Theme              woxcomponent.Theme
	Window             *woxui.Window
	SessionID          string
	Command            string
	Status             string
	Error              string
	Text               string
	Scroll             float32
	LoadingHistory     bool
	SearchOpen         bool
	SearchEditing      woxui.TextEditingState
	CaseSensitive      bool
	MatchCount         int
	MatchIndex         int
	Matches            []TerminalMatch
	Fullscreen         bool
	SearchHotkey       string
	FullscreenHotkey   string
	Tags               []PreviewTag
	LayoutText         func(string, woxui.TextStyle, float32, float32) woxwidget.TextBlockLayout
	OnClampScroll      func(float32)
	OnScroll           func(float32, float32)
	OnOpenSearch       func()
	OnSetSearch        func(string) error
	OnSearchChanged    func(string)
	OnSearchKey        func(woxui.KeyEvent) bool
	OnMoveSearch       func(int)
	OnToggleSearchCase func()
	OnCloseSearch      func()
	OnToggleFullscreen func()
	OnTagHover         func(bool, string, woxui.Rect)
	SelectionAnchor    int
	SelectionFocus     int
	OnSelectionStart   func(int, woxui.KeyModifiers)
	OnSelectionExtend  func(int)
	OnSelectWord       func(int)
	OnSelectLine       func(int)
	OnCopy             func()
	OnSelectAll        func()
	OnContextMenu      func(woxui.Point)
}

// TerminalMatch identifies one byte range in terminal output.
type TerminalMatch struct {
	Start int
	End   int
}

// TerminalSearchInputKey keeps controller focus routing aligned with the retained search field.
func TerminalSearchInputKey(sessionID string) woxwidget.Key {
	return woxwidget.Key("terminal-search-input-" + sessionID)
}

// TerminalPreviewView builds the frameless terminal surface and optional metadata tags.
func TerminalPreviewView(props TerminalPreviewProps) woxwidget.Widget {
	innerWidth := max(float32(0), props.Width-22)
	innerHeight := max(float32(0), props.Height-20)
	previewHeight := innerHeight
	if len(props.Tags) > 0 {
		previewHeight = max(float32(0), previewHeight-36)
	}
	contentProps := props
	contentProps.Width = innerWidth
	contentProps.Height = previewHeight
	children := []woxwidget.StackChild{{Child: terminalPreviewContent(contentProps)}}
	if len(props.Tags) > 0 {
		children = append(children, woxwidget.StackChild{Top: previewHeight + 10, Child: PreviewTags(props.Tags, props.Theme, props.Window, max(float32(0), innerWidth-12), props.OnTagHover)})
	}
	return woxwidget.Container{
		Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: 10, Top: 10, Right: 12, Bottom: 10},
		Child: woxwidget.Stack{Width: innerWidth, Height: innerHeight, Children: children},
	}
}

func terminalPreviewContent(props TerminalPreviewProps) woxwidget.Widget {
	const statusHeight = float32(42)
	searchHeight := float32(0)
	if props.SearchOpen {
		searchHeight = 58
	}
	bodyHeight := max(float32(0), props.Height-statusHeight-searchHeight)
	innerWidth := props.Width
	innerHeight := bodyHeight
	value := props.Text
	if strings.TrimSpace(value) == "" {
		value = "Waiting for terminal output…"
	}
	if props.Error != "" {
		value += "\n\n" + props.Error
	}
	style := woxui.TextStyle{Size: 12}
	layout := woxwidget.TextBlockLayout{}
	if props.LayoutText != nil {
		layout = props.LayoutText(value, style, innerWidth, terminalOutputLineHeight)
	}
	contentHeight := max(innerHeight, layout.Size.Height)
	maxOffset := max(float32(0), contentHeight-innerHeight)
	offset := min(max(float32(0), props.Scroll), maxOffset)
	if props.OnClampScroll != nil {
		props.OnClampScroll(maxOffset)
	}
	header := terminalHeader(props)
	selectable := terminalOutputSelectable(props.Text)
	selectionStart, selectionEnd := min(props.SelectionAnchor, props.SelectionFocus), max(props.SelectionAnchor, props.SelectionFocus)
	runeStart, runeEnd := 0, 0
	if selectable {
		runeStart, runeEnd = terminalRuneSelection(props.Text, selectionStart, selectionEnd)
	}
	actions := []woxui.AccessibilityAction{}
	if selectable {
		actions = append(actions, woxui.AccessibilityActionSelectAll, woxui.AccessibilityActionCopy)
	}
	body := woxwidget.Container{Width: props.Width, Height: bodyHeight, Padding: woxwidget.Insets{Top: 2}, Child: woxwidget.Gesture{
		ID: "terminal-preview-scroll-" + props.SessionID,
		OnScroll: func(delta woxui.Point) {
			if props.OnScroll != nil {
				props.OnScroll(-delta.Y, maxOffset)
			}
		},
		Child: woxwidget.ScrollView{Width: innerWidth, Height: innerHeight, ContentHeight: contentHeight, Offset: offset, Child: woxwidget.Semantics{
			Key: TerminalOutputKey(props.SessionID), AutomationID: "launcher.preview.terminal.output", Role: woxui.AccessibilityRoleText, Label: "Terminal output", Value: value, ReadOnly: true,
			HasTextSelection: selectable, SelectionStart: runeStart, SelectionEnd: runeEnd, Actions: actions,
			OnAction: func(action woxui.AccessibilityAction, _ string) error {
				switch action {
				case woxui.AccessibilityActionSelectAll:
					if props.OnSelectAll != nil {
						props.OnSelectAll()
					}
				case woxui.AccessibilityActionCopy:
					if props.OnCopy != nil {
						props.OnCopy()
					}
				}
				return nil
			},
			Child: terminalOutputText(props, value, style, layout, innerWidth, contentHeight, innerHeight, maxOffset),
		}},
	}}
	children := []woxwidget.Widget{header}
	if props.SearchOpen {
		children = append(children, terminalSearchBar(props, searchHeight))
	}
	children = append(children, body)
	return woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}
}

func terminalHeader(props TerminalPreviewProps) woxwidget.Widget {
	statusColor := props.Theme.ResultSubtitle
	switch props.Status {
	case "running":
		statusColor = woxui.Color{R: 68, G: 196, B: 120, A: 255}
	case "failed", "killed":
		statusColor = props.Theme.ErrorText
	}
	command := props.Command
	if command == "" {
		command = "Terminal"
	}
	status := props.Status
	if status == "" {
		status = "idle"
	}
	loadingWidth := float32(0)
	if props.LoadingHistory {
		status = "history…"
		loadingWidth = 20
	}
	contentWidth := max(float32(0), props.Width-20)
	hoverBackground := previewColorWithOpacity(props.Theme.PreviewText, 0.1)
	children := []woxwidget.StackChild{
		{Child: woxwidget.Align{Width: 8, Height: 34, Vertical: 0.5, Child: woxwidget.Semantics{
			AutomationID: "launcher.preview.terminal.status", Role: woxui.AccessibilityRoleText, Label: "Terminal status", Value: status, LiveRegion: woxui.AccessibilityLiveRegionPolite,
			Child: woxwidget.Container{Width: 8, Height: 8, Radius: 4, Color: statusColor},
		}}},
		{Left: 17, Right: 79 + loadingWidth, StretchWidth: true, Child: woxwidget.Align{Height: 34, Vertical: 0.5, Child: woxwidget.Text{Value: command, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.Theme.PreviewText}}},
	}
	if props.LoadingHistory {
		children = append(children, woxwidget.StackChild{Right: 62, AnchorRight: true, Child: woxwidget.Align{Width: 20, Height: 34, Vertical: 0.5, Child: woxwidget.Text{Value: "…", Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: props.Theme.PreviewText}}})
	}
	children = append(children, woxwidget.StackChild{Right: 34, AnchorRight: true, Child: woxwidget.Align{Width: 28, Height: 34, Vertical: 0.5, Child: woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
		ID: "terminal-search-open-" + props.SessionID, Label: "Find", Icon: woxcomponent.SearchGlyph(18, props.Theme.PreviewText), Width: 28, Height: 28, Radius: 14,
		HoverBackground: hoverBackground, FocusRingColor: props.Theme.Cursor, OnTap: props.OnOpenSearch, OnHoverAt: func(inside bool, bounds woxui.Rect) {
			if props.OnTagHover != nil {
				props.OnTagHover(inside, props.SearchHotkey, bounds)
			}
		},
	})}}, woxwidget.StackChild{AnchorRight: true, Child: woxwidget.Align{Width: 28, Height: 34, Vertical: 0.5, Child: woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
		ID: "terminal-fullscreen-" + props.SessionID, Label: "Toggle fullscreen", Icon: woxcomponent.FullscreenGlyph(18, props.Theme.PreviewText, props.Fullscreen), Width: 28, Height: 28, Radius: 14,
		HoverBackground: hoverBackground, FocusRingColor: props.Theme.Cursor, OnTap: props.OnToggleFullscreen, OnHoverAt: func(inside bool, bounds woxui.Rect) {
			if props.OnTagHover != nil {
				props.OnTagHover(inside, props.FullscreenHotkey, bounds)
			}
		},
	})}})
	panel := woxwidget.Container{
		Width: props.Width, Height: 36, Radius: 8, Color: previewColorWithOpacity(props.Theme.PreviewText, 0.035),
		BorderColor: previewColorWithOpacity(props.Theme.PreviewSplit, 0.75), BorderWidth: 1, Padding: woxwidget.Insets{Left: 10, Right: 10},
		Child: woxwidget.Stack{Width: contentWidth, Height: 34, Children: children},
	}
	return woxwidget.Container{Width: props.Width, Height: 42, Padding: woxwidget.Insets{Bottom: 6}, Child: panel}
}

func terminalSearchBar(props TerminalPreviewProps, height float32) woxwidget.Widget {
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

type terminalHighlightSegment struct {
	line       int
	start      int
	end        int
	matchIndex int
}

// terminalOutputText overlays selection and match colors on the shared wrapped text layout.
func terminalOutputText(props TerminalPreviewProps, value string, style woxui.TextStyle, layout woxwidget.TextBlockLayout, width, height, viewportHeight, maxOffset float32) woxwidget.Widget {
	text := woxwidget.TextBlock{Value: value, Width: width, Height: height, Style: style, LineHeight: terminalOutputLineHeight, Color: props.Theme.PreviewText, Layout: &layout}
	children := []woxwidget.StackChild{}
	selectionStart, selectionEnd := min(props.SelectionAnchor, props.SelectionFocus), max(props.SelectionAnchor, props.SelectionFocus)
	selectionStart = TerminalClampUTF8Offset(props.Text, selectionStart)
	selectionEnd = TerminalClampUTF8Offset(props.Text, min(selectionEnd, len(props.Text)))
	if selectionStart < selectionEnd {
		selectionColor := props.Theme.Controls.TextSelectionBackground
		if selectionColor.A == 0 {
			selectionColor = woxui.Color{R: 57, G: 204, B: 183, A: 120}
		}
		children = append(children, woxwidget.StackChild{Child: terminalRangePainter(value, layout, style, props.Window, width, height, []TerminalMatch{{Start: selectionStart, End: selectionEnd}}, -1, selectionColor, false)})
	}
	children = append(children, woxwidget.StackChild{Child: text})
	if segments := terminalHighlightSegments(value, layout.Lines, props.Matches); len(segments) > 0 && props.Window != nil {
		children = append(children, woxwidget.StackChild{Child: terminalRangePainter(value, layout, style, props.Window, width, height, props.Matches, props.MatchIndex, woxui.Color{R: 255, G: 245, B: 157, A: 255}, true)})
	}
	content := woxwidget.Stack{Width: width, Height: height, Children: children}
	if !terminalOutputSelectable(props.Text) {
		return content
	}
	offsetAt := func(position woxui.Point) int {
		offset := terminalOffsetAt(value, layout, props.Window, style, position, terminalOutputLineHeight)
		if offset > len(props.Text) {
			return len(props.Text)
		}
		return offset
	}
	return woxwidget.Gesture{
		ID: string(TerminalOutputKey(props.SessionID)), Cursor: woxui.PointerCursorText,
		OnSelectionStart: func(position woxui.Point, modifiers woxui.KeyModifiers) {
			if props.OnSelectionStart != nil {
				props.OnSelectionStart(offsetAt(position), modifiers)
			}
		},
		OnSelectionExtend: func(position woxui.Point) {
			if props.OnScroll != nil {
				viewY := position.Y - props.Scroll
				if viewY < 0 {
					props.OnScroll(viewY, maxOffset)
				} else if viewY > viewportHeight {
					props.OnScroll(viewY-viewportHeight, maxOffset)
				}
			}
			if props.OnSelectionExtend != nil {
				props.OnSelectionExtend(offsetAt(position))
			}
		},
		OnDoubleTapAt: func(position woxui.Point) {
			if props.OnSelectWord != nil {
				props.OnSelectWord(offsetAt(position))
			}
		},
		OnTripleTapAt: func(position woxui.Point) {
			if props.OnSelectLine != nil {
				props.OnSelectLine(offsetAt(position))
			}
		},
		OnSecondaryTapDown: props.OnContextMenu,
		Child:              content,
	}
}

// terminalRangePainter draws selection washes or search match overlays using the shared wrapped lines.
func terminalRangePainter(value string, layout woxwidget.TextBlockLayout, style woxui.TextStyle, window *woxui.Window, width, height float32, matches []TerminalMatch, activeIndex int, background woxui.Color, overlayText bool) woxwidget.Widget {
	segments := terminalHighlightSegments(value, layout.Lines, matches)
	if len(segments) == 0 {
		return woxwidget.Container{Width: width, Height: height}
	}
	activeBackground := woxui.Color{R: 251, G: 192, B: 45, A: 255}
	return woxwidget.Painter{Width: width, Height: height, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		start, end := 0, len(segments)
		if clip, ok := displayList.ClipRect(); ok {
			first := max(0, int((clip.Y-bounds.Y)/terminalOutputLineHeight))
			last := max(first, int((clip.Y+clip.Height-bounds.Y)/terminalOutputLineHeight)+1)
			start = sort.Search(len(segments), func(i int) bool { return segments[i].line >= first })
			end = sort.Search(len(segments), func(i int) bool { return segments[i].line >= last })
		}
		for _, segment := range segments[start:end] {
			line := layout.Lines[segment.line]
			prefixWidth := terminalMeasureWidth(window, line[:segment.start], style)
			matchWidth := terminalMeasureWidth(window, line[segment.start:segment.end], style)
			y := bounds.Y + float32(segment.line)*terminalOutputLineHeight
			fill := background
			if overlayText && segment.matchIndex == activeIndex {
				fill = activeBackground
			}
			rect := woxui.Rect{X: bounds.X + prefixWidth, Y: y + 1, Width: matchWidth, Height: terminalOutputLineHeight - 2}
			displayList.FillRoundedRect(rect, 1, fill)
			if overlayText {
				displayList.DrawText(line[segment.start:segment.end], woxui.Rect{X: rect.X, Y: y, Width: rect.Width, Height: terminalOutputLineHeight}, style, woxui.Color{A: 242})
			}
		}
	}}
}

// terminalHighlightSegments maps absolute byte ranges onto the wrapped lines rendered by TextBlock.
func terminalHighlightSegments(value string, lines []string, matches []TerminalMatch) []terminalHighlightSegment {
	if len(matches) == 0 {
		return nil
	}
	ranges := terminalLineRanges(value, lines)
	segments := make([]terminalHighlightSegment, 0, len(matches))
	matchIndex := 0
	for lineIndex, line := range ranges {
		for matchIndex < len(matches) && matches[matchIndex].End <= line.start {
			matchIndex++
		}
		for index := matchIndex; index < len(matches) && matches[index].Start < line.end; index++ {
			match := matches[index]
			start := max(match.Start, line.start)
			end := min(match.End, line.end)
			if start < end {
				segments = append(segments, terminalHighlightSegment{line: lineIndex, start: start - line.start, end: end - line.start, matchIndex: index})
			}
		}
	}
	return segments
}
