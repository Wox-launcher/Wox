package launcher

import (
	"fmt"
	"strings"

	woxui "github.com/Wox-launcher/wox.ui.go"
	woxwidget "github.com/Wox-launcher/wox.ui.go/widget"
)

// buildTerminalPreview renders the streaming model entirely through the portable text/display-list stack.
func (a *App) buildTerminalPreview(snapshot terminalPreviewSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	const statusHeight = float32(38)
	searchHeight := float32(0)
	if snapshot.SearchOpen {
		searchHeight = 50
	}
	bodyHeight := max(float32(0), height-statusHeight-searchHeight)
	innerWidth := max(float32(0), width-24)
	innerHeight := max(float32(0), bodyHeight-20)
	value := snapshot.Text
	if strings.TrimSpace(value) == "" {
		value = "Waiting for terminal output…"
	}
	if snapshot.Error != "" {
		value += "\n\n" + snapshot.Error
	}
	style := woxui.TextStyle{Size: 12}
	key := "terminal\x00" + snapshot.SessionID
	layout := a.previewTextLayout(key, value, style, innerWidth, 18)
	contentHeight := max(innerHeight, layout.Size.Height)
	maxOffset := max(float32(0), contentHeight-innerHeight)
	offset := min(max(float32(0), snapshot.Scroll), maxOffset)
	a.clampTerminalPreviewScroll(maxOffset)
	statusColor := palette.resultSubtitle
	switch snapshot.Status {
	case "running":
		statusColor = woxui.Color{R: 68, G: 196, B: 120, A: 255}
	case "failed", "killed":
		statusColor = woxui.Color{R: 232, G: 95, B: 95, A: 255}
	}
	command := snapshot.Command
	if command == "" {
		command = "Terminal"
	}
	status := snapshot.Status
	if status == "" {
		status = "idle"
	}
	if snapshot.LoadingHistory {
		status = "history…"
	}
	contentWidth := max(float32(0), width-24)
	searchWidth := float32(50)
	statusWidth := float32(64)
	commandWidth := max(float32(40), contentWidth-searchWidth-statusWidth-34)
	header := woxwidget.Container{Width: width, Height: statusHeight, Radius: 8, Color: palette.queryBackground, Padding: woxwidget.Insets{Left: 12, Top: 8, Right: 12}, Child: woxwidget.Stack{Width: contentWidth, Height: 22, Children: []woxwidget.StackChild{
		{Top: 7, Child: woxwidget.Container{Width: 8, Height: 8, Radius: 4, Color: statusColor}},
		{Left: 17, Top: 3, Child: woxwidget.Container{Width: commandWidth, Height: 18, Child: woxwidget.Text{Value: command, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: palette.previewText}}},
		{Left: contentWidth - searchWidth - statusWidth - 8, Top: 4, Child: woxwidget.Container{Width: statusWidth, Height: 18, Child: woxwidget.Text{Value: status, Style: woxui.TextStyle{Size: 10}, Color: statusColor}}},
		{Left: contentWidth - searchWidth, Child: woxwidget.Gesture{ID: "terminal-search-open-" + snapshot.SessionID, OnTap: a.openTerminalSearch, Child: woxwidget.Container{
			Width: searchWidth, Height: 22, Radius: 6, Color: palette.actionBackground, Padding: woxwidget.Insets{Left: 9, Top: 6}, Child: woxwidget.Text{Value: "Find", Style: woxui.TextStyle{Size: 9, Weight: woxui.FontWeightSemibold}, Color: palette.previewText},
		}}},
	}}}
	body := woxwidget.Container{Width: width, Height: bodyHeight, Radius: 8, Color: palette.queryBackground, Padding: woxwidget.Insets{Left: 12, Top: 10, Right: 12, Bottom: 10}, Child: woxwidget.Gesture{
		ID: "terminal-preview-scroll-" + snapshot.SessionID,
		OnScroll: func(delta woxui.Point) {
			a.scrollTerminalPreview(-delta.Y, maxOffset)
		},
		Child: woxwidget.ScrollView{Width: innerWidth, Height: innerHeight, ContentHeight: contentHeight, Offset: offset, Child: woxwidget.TextBlock{
			Value: value, Width: innerWidth, Height: contentHeight, Style: style, LineHeight: 18, Color: palette.previewText, Layout: &layout,
		}},
	}}
	children := []woxwidget.Widget{header}
	if snapshot.SearchOpen {
		children = append(children, a.buildTerminalSearchBar(snapshot, palette, width, searchHeight))
	}
	children = append(children, body)
	return woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}
}

// buildTerminalSearchBar reuses the shared text editor and keeps find behavior identical across native shells.
func (a *App) buildTerminalSearchBar(snapshot terminalPreviewSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-20)
	actionWidth := float32(32)
	countWidth := float32(46)
	gap := float32(5)
	inputWidth := max(float32(90), innerWidth-countWidth-actionWidth*4-gap*5)
	style := woxui.TextStyle{Size: 12}
	input := woxwidget.Gesture{ID: "terminal-search-input-" + snapshot.SessionID, OnTapAt: func(position woxui.Point) {
		offset := formTextOffsetAt(snapshot.SearchEditing, a.window, style, 1, inputWidth-20, woxui.Point{X: max(float32(0), position.X-10), Y: max(float32(0), position.Y-7)})
		a.setTerminalSearchCaret(offset)
	}, Child: woxwidget.Container{Width: inputWidth, Height: 34, Radius: 7, Color: palette.queryBackground, Padding: woxwidget.Insets{Left: 10, Top: 7, Right: 10, Bottom: 5}, Child: woxwidget.Clip{
		Width: inputWidth - 20, Height: 22, Child: woxwidget.Painter{Width: inputWidth - 20, Height: 22, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			drawFormEditor(displayList, bounds, snapshot.SearchEditing, style, palette, true, 1, a.window)
		}},
	}}}
	count := "0/0"
	if snapshot.MatchCount > 0 {
		count = fmt.Sprintf("%d/%d", snapshot.MatchIndex+1, snapshot.MatchCount)
	}
	button := func(id, label string, selected bool, action func()) woxwidget.Widget {
		background := palette.queryBackground
		if selected {
			background = palette.selectedBackground
		}
		return woxwidget.Gesture{ID: id + "-" + snapshot.SessionID, OnTap: action, Child: woxwidget.Container{Width: actionWidth, Height: 34, Radius: 7, Color: background, Padding: woxwidget.Insets{Left: 9, Top: 9}, Child: woxwidget.Text{
			Value: label, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: palette.previewText,
		}}}
	}
	return woxwidget.Container{Width: width, Height: height, Radius: 8, Color: palette.actionBackground, Padding: woxwidget.Insets{Left: 10, Top: 8, Right: 10, Bottom: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: gap, Children: []woxwidget.Widget{
		input,
		woxwidget.Container{Width: countWidth, Height: 34, Padding: woxwidget.Insets{Left: 5, Top: 10}, Child: woxwidget.Text{Value: count, Style: woxui.TextStyle{Size: 9, Weight: woxui.FontWeightSemibold}, Color: palette.resultSubtitle}},
		button("terminal-search-previous", "↑", false, func() { a.moveTerminalSearch(-1) }),
		button("terminal-search-next", "↓", false, func() { a.moveTerminalSearch(1) }),
		button("terminal-search-case", "Aa", snapshot.CaseSensitive, a.toggleTerminalSearchCase),
		button("terminal-search-close", "×", false, a.closeTerminalSearch),
	}}}
}
