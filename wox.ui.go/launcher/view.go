package launcher

import (
	"fmt"
	"strings"

	woxui "github.com/Wox-launcher/wox.ui.go"
	woxwidget "github.com/Wox-launcher/wox.ui.go/widget"
)

var resultColors = []woxui.Color{
	{R: 61, G: 205, B: 175, A: 255},
	{R: 255, G: 119, B: 81, A: 255},
	{R: 177, G: 104, B: 255, A: 255},
	{R: 66, G: 153, B: 225, A: 255},
	{R: 238, G: 191, B: 64, A: 255},
}

type viewSnapshot struct {
	editing               woxui.TextEditingState
	results               []queryResult
	pendingResults        bool
	selected              int
	layout                queryLayout
	refinements           []queryRefinement
	refinementValues      map[string]string
	refinementOpen        bool
	completionHint        *queryCompletionHint
	toolbarMsg            *toolbarMessage
	glance                *glanceItem
	hideGlanceIcon        bool
	form                  *formSnapshot
	tableEditor           *formTableEditorSnapshot
	requirementFormActive bool
	chatFullscreen        bool
	actionPanel           bool
	actionSelected        int
	show                  showAppParams
	palette               uiPalette
}

func (a *App) snapshot() viewSnapshot {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var tableEditor *formTableEditorSnapshot
	if a.tableEditor != nil && a.formTableTargetCurrentLocked(a.tableEditor.target) {
		tableEditor = snapshotFormTableEditorLocked(a.tableEditor)
	}
	refinementValues := make(map[string]string, len(a.query.QueryRefinements))
	for key, value := range a.query.QueryRefinements {
		refinementValues[key] = value
	}
	var completionHint *queryCompletionHint
	if a.completionHint != nil {
		copy := *a.completionHint
		completionHint = &copy
	}
	var toolbarMsg *toolbarMessage
	if a.toolbarMsg != nil {
		copy := *a.toolbarMsg
		copy.Actions = append([]toolbarMessageAction(nil), a.toolbarMsg.Actions...)
		toolbarMsg = &copy
	}
	var glance *glanceItem
	if a.glanceItem != nil {
		copy := *a.glanceItem
		if a.glanceItem.Action != nil {
			action := *a.glanceItem.Action
			copy.Action = &action
		}
		glance = &copy
	}
	return viewSnapshot{
		editing:               a.editor.State(),
		results:               append([]queryResult(nil), a.results...),
		pendingResults:        a.pendingResults,
		selected:              a.selected,
		layout:                a.layout,
		refinements:           append([]queryRefinement(nil), a.refinements...),
		refinementValues:      refinementValues,
		refinementOpen:        a.refinementOpen,
		completionHint:        completionHint,
		toolbarMsg:            toolbarMsg,
		glance:                glance,
		hideGlanceIcon:        a.settings.HideGlanceIcon,
		form:                  snapshotFormLocked(a.form),
		tableEditor:           tableEditor,
		requirementFormActive: a.requirementForm != nil && a.requirementForm.active,
		chatFullscreen:        a.chatFullscreen,
		actionPanel:           a.actionPanel,
		actionSelected:        a.actionSelected,
		show:                  a.show,
		palette:               a.palette,
	}
}

func (a *App) build(frame woxui.FrameInfo) woxwidget.Widget {
	a.mu.RLock()
	mode := a.mode
	a.mu.RUnlock()
	if mode == viewSettings {
		return a.buildSettings(frame)
	}
	snapshot := a.snapshot()
	width := frame.Size.Width
	height := frame.Size.Height
	queryHeight := float32(0)
	if !snapshot.show.HideQueryBox && !snapshot.chatFullscreen {
		queryHeight = headerHeight
	}
	toolbarHeight := float32(0)
	if !snapshot.show.HideToolbar && !snapshot.chatFullscreen {
		toolbarHeight = footerHeight
	}
	refinementHeight := float32(0)
	if queryHeight > 0 && snapshot.refinementOpen && len(snapshot.refinements) > 0 {
		refinementHeight = refinementBarHeight
	}
	contentHeight := max(0, height-queryHeight-refinementHeight-toolbarHeight)
	content := a.buildContent(snapshot, width, contentHeight)
	children := make([]woxwidget.Widget, 0, 4)
	if queryHeight > 0 && !snapshot.show.QueryBoxAtBottom {
		children = append(children, a.buildHeader(snapshot, width, queryHeight))
	}
	if refinementHeight > 0 && !snapshot.show.QueryBoxAtBottom {
		children = append(children, a.buildRefinementBar(snapshot, width, refinementHeight))
	}
	children = append(children, content)
	if refinementHeight > 0 && snapshot.show.QueryBoxAtBottom {
		children = append(children, a.buildRefinementBar(snapshot, width, refinementHeight))
	}
	if queryHeight > 0 && snapshot.show.QueryBoxAtBottom {
		children = append(children, a.buildHeader(snapshot, width, queryHeight))
	}
	if toolbarHeight > 0 {
		children = append(children, a.buildFooter(snapshot, width, toolbarHeight))
	}
	var body woxwidget.Widget = woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}
	if snapshot.form != nil {
		queryChromeHeight := queryHeight + refinementHeight
		panel, panelWidth, panelHeight := a.buildFormPanel(snapshot, width)
		body = woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
			{Child: body},
			{Left: max(float32(14), width-panelWidth-14), Top: max(queryChromeHeight+8, height-toolbarHeight-panelHeight-12), Child: panel},
		}}
	} else if snapshot.actionPanel {
		queryChromeHeight := queryHeight + refinementHeight
		panel, panelWidth, panelHeight := a.buildActionPanel(snapshot, width, height, queryChromeHeight, toolbarHeight)
		if panel != nil {
			body = woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
				{Child: body},
				{Left: max(float32(12), width-panelWidth-14), Top: max(queryHeight+8, height-toolbarHeight-panelHeight-12), Child: panel},
			}}
		}
	}
	if snapshot.tableEditor != nil {
		body = woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
			{Child: body},
			{Child: a.buildFormTableOverlay(snapshot.tableEditor, snapshot.palette, width, height)},
		}}
	}
	return woxwidget.Container{
		Width:  width,
		Height: height,
		Color:  snapshot.palette.background,
		Radius: 14,
		Child:  body,
	}
}

func (a *App) buildHeader(snapshot viewSnapshot, width, height float32) woxwidget.Widget {
	const horizontalPadding = float32(10)
	const queryLeftPadding = float32(8)
	const accessoryGap = float32(12)
	contentWidth := max(float32(0), width-horizontalPadding*2-queryLeftPadding-6)
	queryWidth := contentWidth
	glanceWidth := float32(0)
	if snapshot.glance != nil {
		metrics, _ := a.window.MeasureText(strings.TrimSpace(snapshot.glance.Text), woxui.TextStyle{Size: 15})
		glanceWidth = metrics.Size.Width + 20
		if !snapshot.hideGlanceIcon && snapshot.glance.Icon.ImageData != "" {
			glanceWidth += 21
		}
		glanceWidth = min(float32(192), max(float32(44), glanceWidth))
		queryWidth -= glanceWidth + accessoryGap
	}
	if len(snapshot.refinements) > 0 {
		queryWidth -= a.refinementToggleWidth(snapshot) + accessoryGap
	}
	var queryIcon woxwidget.Widget
	if snapshot.glance == nil {
		if image := a.imageFor(snapshot.layout.Icon); image != nil {
			queryIcon = woxwidget.Container{Width: 30, Height: 34, Padding: woxwidget.Insets{Top: 2}, Child: woxwidget.Image{Source: image, Width: 30, Height: 30}}
			queryWidth -= 30 + accessoryGap
		}
	}
	queryWidth = max(float32(140), queryWidth)
	children := []woxwidget.Widget{a.buildQuery(snapshot, queryWidth, 34)}
	if len(snapshot.refinements) > 0 {
		children = append(children, a.buildRefinementToggle(snapshot))
	}
	if snapshot.glance != nil {
		children = append(children, a.buildGlance(*snapshot.glance, snapshot.hideGlanceIcon, snapshot.palette, glanceWidth))
	}
	if queryIcon != nil {
		children = append(children, queryIcon)
	}
	return woxwidget.Container{
		Width: width, Height: height,
		Padding: woxwidget.Insets{Left: horizontalPadding, Top: 10, Right: horizontalPadding, Bottom: 10},
		Child: woxwidget.Container{
			Width: width - horizontalPadding*2, Height: 55, Radius: 8, Color: snapshot.palette.queryBackground,
			Padding: woxwidget.Insets{Left: queryLeftPadding, Top: 4, Right: 6, Bottom: 17},
			Child:   woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: accessoryGap, Children: children},
		},
	}
}

func (a *App) buildQuery(snapshot viewSnapshot, width, height float32) woxwidget.Widget {
	style := woxui.TextStyle{Size: 28}
	return woxwidget.Gesture{
		ID: "query-editor",
		OnTapAt: func(position woxui.Point) {
			a.placeQueryCaret(position.X, style)
		},
		Child: woxwidget.Painter{Width: width, Height: height, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			state := snapshot.editing
			queryFocused := snapshot.form == nil && !snapshot.requirementFormActive
			runes := []rune(state.Text)
			start := max(0, min(len(runes), state.Selection.Start()))
			end := max(start, min(len(runes), state.Selection.End()))
			focus := max(0, min(len(runes), state.Selection.Focus))
			prefix := string(runes[:start])
			selected := string(runes[start:end])
			displayValue := state.Text
			if state.Composition != "" {
				displayValue = prefix + state.Composition + string(runes[end:])
			}
			color := snapshot.palette.queryText
			if queryFocused && state.Composition == "" && state.Selection.Collapsed() && state.Selection.Focus == len(runes) && snapshot.completionHint != nil && snapshot.completionHint.InputPrefix == state.Text {
				valueMetrics, _ := a.window.MeasureText(state.Text, style)
				hintColor := snapshot.palette.queryText
				hintColor.A = 96
				displayList.DrawText(snapshot.completionHint.Suffix, woxui.Rect{X: bounds.X + valueMetrics.Size.Width, Y: bounds.Y, Width: max(float32(0), bounds.Width-valueMetrics.Size.Width), Height: bounds.Height}, style, hintColor)
			}

			prefixMetrics, _ := a.window.MeasureText(prefix, style)
			selectedMetrics, _ := a.window.MeasureText(selected, style)
			if queryFocused && state.Composition == "" && start != end {
				displayList.FillRoundedRect(woxui.Rect{X: bounds.X + prefixMetrics.Size.Width, Y: bounds.Y, Width: selectedMetrics.Size.Width, Height: 34}, 3, snapshot.palette.selectionBackground)
			}
			displayList.DrawText(displayValue, bounds, style, color)
			if queryFocused && state.Composition == "" && selected != "" {
				displayList.DrawText(selected, woxui.Rect{X: bounds.X + prefixMetrics.Size.Width, Y: bounds.Y, Width: selectedMetrics.Size.Width, Height: bounds.Height}, style, snapshot.palette.selectionText)
			}
			if !queryFocused {
				return
			}

			caretPrefix := string(runes[:focus])
			if state.Composition != "" {
				caretPrefix = prefix + state.Composition
			}
			caretMetrics, _ := a.window.MeasureText(caretPrefix, style)
			cursorX := bounds.X + caretMetrics.Size.Width
			displayList.FillRect(woxui.Rect{X: cursorX, Y: bounds.Y, Width: 1, Height: 34}, snapshot.palette.cursor)
			_ = a.window.SetTextInputState(woxui.TextInputState{Enabled: true, CursorRect: woxui.Rect{X: cursorX, Y: bounds.Y, Width: 1, Height: 34}})
			if state.Composition != "" {
				compositionMetrics, _ := a.window.MeasureText(state.Composition, style)
				displayList.FillRect(woxui.Rect{X: bounds.X + prefixMetrics.Size.Width, Y: bounds.Y + 33, Width: compositionMetrics.Size.Width, Height: 1}, snapshot.palette.cursor)
			}
		}},
	}
}

func (a *App) placeQueryCaret(x float32, style woxui.TextStyle) {
	a.deactivateRequirementForm()
	a.mu.RLock()
	text := a.editor.State().Text
	a.mu.RUnlock()
	runes := []rune(text)
	offset := len(runes)
	previousWidth := float32(0)
	for index := 1; index <= len(runes); index++ {
		metrics, _ := a.window.MeasureText(string(runes[:index]), style)
		if x < (previousWidth+metrics.Size.Width)*0.5 {
			offset = index - 1
			break
		}
		previousWidth = metrics.Size.Width
	}
	a.mu.Lock()
	a.editor.SetCaret(offset)
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) buildContent(snapshot viewSnapshot, width, height float32) woxwidget.Widget {
	if len(snapshot.results) == 0 {
		if snapshot.pendingResults {
			return woxwidget.Container{Width: width, Height: height}
		}
		return woxwidget.Container{
			Width: width, Height: height, Padding: woxwidget.Insets{Left: 28, Top: 18},
			Child: woxwidget.Text{Value: "Type a query to search Wox plugins", Style: woxui.TextStyle{Size: 14}, Color: snapshot.palette.resultSubtitle},
		}
	}
	maxResults := snapshot.show.MaxResultCount
	if maxResults <= 0 {
		maxResults = defaultMaxResult
	}
	if len(snapshot.results) > maxResults {
		snapshot.results = snapshot.results[:maxResults]
	}
	previewVisible := snapshot.selected >= 0 && snapshot.selected < len(snapshot.results) && snapshot.results[snapshot.selected].Preview.PreviewData != ""
	if !previewVisible {
		a.deactivateTerminalPreview()
		a.deactivateWebViewPreview()
		a.deactivateTriggerConflictPreview()
		a.deactivateThemeEditorPreview()
		a.deactivateChatPreview()
		return a.buildResults(snapshot, width, height)
	}
	ratio := float32(0.4)
	if snapshot.layout.ResultPreviewWidthRatio != nil && *snapshot.layout.ResultPreviewWidthRatio >= 0 && *snapshot.layout.ResultPreviewWidthRatio <= 1 {
		ratio = float32(*snapshot.layout.ResultPreviewWidthRatio)
	}
	if snapshot.chatFullscreen {
		ratio = 0
	}
	if ratio <= 0 {
		return a.buildPreview(snapshot.results[snapshot.selected], snapshot.palette, width, height)
	}
	if ratio >= 1 {
		a.deactivateTerminalPreview()
		a.deactivateWebViewPreview()
		a.deactivateTriggerConflictPreview()
		a.deactivateThemeEditorPreview()
		a.deactivateChatPreview()
		return a.buildResults(snapshot, width, height)
	}
	splitX := width * ratio
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		a.buildResults(snapshot, splitX, height),
		a.buildPreview(snapshot.results[snapshot.selected], snapshot.palette, width-splitX, height),
	}}
}

func (a *App) buildResults(snapshot viewSnapshot, width, height float32) woxwidget.Widget {
	if snapshot.layout.GridLayout != nil {
		return a.buildGridResults(snapshot, width, height)
	}
	rows := make([]woxwidget.Widget, 0, len(snapshot.results))
	for index, result := range snapshot.results {
		index := index
		result := result
		selected := index == snapshot.selected
		background := woxui.Color{}
		title := snapshot.palette.resultTitle
		subtitle := snapshot.palette.resultSubtitle
		if selected {
			background = snapshot.palette.selectedBackground
			title = snapshot.palette.selectedTitle
			subtitle = snapshot.palette.selectedSubtitle
		}
		if result.IsGroup {
			rows = append(rows, woxwidget.Container{
				Width: width - 20, Height: resultRowHeight, Padding: woxwidget.Insets{Left: 8, Top: 18},
				Child: woxwidget.Text{Value: result.Title, Style: woxui.TextStyle{Size: 15}, Color: subtitle},
			})
			continue
		}
		var icon woxwidget.Widget = woxwidget.Container{Width: 28, Height: 28, Radius: 7, Color: resultColors[index%len(resultColors)]}
		if image := a.imageFor(result.Icon); image != nil {
			icon = woxwidget.Image{Source: image, Width: 28, Height: 28}
		}
		tailWidth := float32(0)
		var tail woxwidget.Widget
		if len(result.Tails) > 0 {
			tail, tailWidth = a.buildResultTails(result.Tails, snapshot.palette, subtitle, width)
		}
		contentWidth := max(float32(0), width-46)
		labelWidth := max(float32(50), contentWidth-28-20-tailWidth)
		rows = append(rows, woxwidget.Gesture{
			ID: fmt.Sprintf("result-%s", result.ID),
			OnHover: func(inside bool) {
				if inside {
					a.selectResult(index)
				}
			},
			OnTap: func() { a.activateResult(index) },
			OnScroll: func(delta woxui.Point) {
				if delta.Y > 0 {
					a.moveSelection(-1)
				} else if delta.Y < 0 {
					a.moveSelection(1)
				}
			},
			Child: woxwidget.Container{
				Width: width - 20, Height: resultRowHeight, Radius: 8, Color: background,
				Padding: woxwidget.Insets{Left: 13, Top: 3, Right: 13, Bottom: 3},
				Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
					woxwidget.Container{Width: 28, Height: 50, Padding: woxwidget.Insets{Top: 11}, Child: icon},
					woxwidget.Container{Width: labelWidth, Height: 50, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 2, Children: []woxwidget.Widget{
						woxwidget.Text{Value: result.Title, Style: woxui.TextStyle{Size: 15}, Color: title},
						woxwidget.Text{Value: result.SubTitle, Style: woxui.TextStyle{Size: 12}, Color: subtitle},
					}}},
					woxwidget.Container{Width: tailWidth, Height: 50, Padding: woxwidget.Insets{Top: 9}, Child: tail},
				}},
			},
		})
	}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 10, Top: resultListInset, Right: 10}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: resultRowGap, Children: rows}}
}

// buildResultTails keeps plugin and debug tails visible in one bounded row without stealing the title column.
func (a *App) buildResultTails(tails []resultTail, palette uiPalette, foreground woxui.Color, rowWidth float32) (woxwidget.Widget, float32) {
	const gap = float32(5)
	style := woxui.TextStyle{Size: 11}
	maximum := min(float32(280), max(float32(88), rowWidth*0.4))
	children := make([]woxwidget.Widget, 0, len(tails))
	used := float32(0)
	for _, item := range tails {
		itemWidth := float32(32)
		var content woxwidget.Widget
		if image := a.imageFor(item.Image); image != nil {
			content = woxwidget.Image{Source: image, Width: 20, Height: 20}
		} else if item.Text != "" {
			metrics, _ := a.window.MeasureText(item.Text, style)
			itemWidth = min(float32(88), max(float32(30), metrics.Size.Width+14))
			content = woxwidget.Clip{Width: itemWidth - 12, Height: 20, Child: woxwidget.Text{Value: item.Text, Style: style, Color: foreground}}
		} else {
			continue
		}
		nextWidth := itemWidth
		if len(children) > 0 {
			nextWidth += gap
		}
		if used+nextWidth > maximum {
			break
		}
		if len(children) > 0 {
			used += gap
		}
		used += itemWidth
		children = append(children, woxwidget.Container{Width: itemWidth, Height: 28, Radius: 6, Color: palette.toolbarBackground, Padding: woxwidget.Insets{Left: 6, Top: 5, Right: 6, Bottom: 3}, Child: content})
	}
	return woxwidget.Container{Width: used, Height: 32, Padding: woxwidget.Insets{Top: 2}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: gap, Children: children}}, used
}

func (a *App) buildFooter(snapshot viewSnapshot, width, height float32) woxwidget.Widget {
	type footerAction struct {
		id     string
		label  string
		hotkey string
		onTap  func()
	}
	type measuredFooterAction struct {
		widget woxwidget.Widget
		width  float32
	}

	leftLabel := ""
	var leftIcon *woxui.Image
	progressLabel := ""
	if snapshot.toolbarMsg != nil {
		leftLabel = snapshot.toolbarMsg.displayText()
		if image := a.imageFor(snapshot.toolbarMsg.Icon); image != nil {
			leftIcon = image
		}
		if snapshot.toolbarMsg.Progress != nil {
			progressLabel = fmt.Sprintf("%d%%", *snapshot.toolbarMsg.Progress)
		} else if snapshot.toolbarMsg.Indeterminate {
			progressLabel = "Working…"
		}
	}
	actions := make([]footerAction, 0)
	if snapshot.selected >= 0 && snapshot.selected < len(snapshot.results) {
		resultIndex := snapshot.selected
		for actionIndex, action := range snapshot.results[resultIndex].Actions {
			if strings.TrimSpace(action.Hotkey) == "" {
				continue
			}
			actionIndex := actionIndex
			action := action
			actions = append(actions, footerAction{
				id: fmt.Sprintf("result-toolbar-action-%d", actionIndex), label: a.translate(action.Name), hotkey: action.Hotkey,
				onTap: func() { a.activateAction(resultIndex, actionIndex) },
			})
		}
		if len(snapshot.results[resultIndex].Actions) > 0 {
			actions = append(actions, footerAction{id: "result-toolbar-more", label: a.translate("i18n:toolbar_more_actions"), hotkey: primaryHotkey("j"), onTap: a.toggleActionPanel})
		}
	} else if snapshot.toolbarMsg != nil {
		for _, action := range snapshot.toolbarMsg.Actions {
			if strings.TrimSpace(action.Hotkey) == "" {
				continue
			}
			action := action
			actions = append(actions, footerAction{id: "toolbar-action-" + action.ID, label: a.translate(action.Name), hotkey: action.Hotkey, onTap: func() { a.activateToolbarAction(action) }})
		}
	}
	contentWidth := max(float32(0), width-20)
	leftWidth := float32(0)
	if leftLabel != "" || leftIcon != nil || progressLabel != "" {
		leftWidth = min(contentWidth*0.42, float32(320))
	}
	rightAvailable := max(float32(0), contentWidth-leftWidth)
	if leftWidth > 0 && len(actions) > 0 {
		rightAvailable -= 16
	}
	measured := make([]measuredFooterAction, 0, len(actions))
	for _, action := range actions {
		widget, actionWidth := a.buildToolbarAction(action.id, action.label, action.hotkey, snapshot.palette, action.onTap)
		measured = append(measured, measuredFooterAction{widget: widget, width: actionWidth})
	}
	shown := make([]measuredFooterAction, 0, len(measured))
	rightWidth := float32(0)
	for index := len(measured) - 1; index >= 0; index-- {
		nextWidth := measured[index].width
		if len(shown) > 0 {
			nextWidth += 16
		}
		if rightWidth+nextWidth > rightAvailable {
			break
		}
		rightWidth += nextWidth
		shown = append([]measuredFooterAction{measured[index]}, shown...)
	}
	rightChildren := make([]woxwidget.Widget, 0, len(shown))
	for _, action := range shown {
		rightChildren = append(rightChildren, action.widget)
	}
	extraWidth := float32(0)
	if leftIcon != nil {
		extraWidth += 26
	}
	progressWidth := float32(0)
	if progressLabel != "" {
		progressMetrics, _ := a.window.MeasureText(progressLabel, woxui.TextStyle{Size: 12})
		progressWidth = min(float32(90), progressMetrics.Size.Width+4)
		extraWidth += progressWidth + 8
	}
	labelWidth := max(float32(0), leftWidth-extraWidth)
	label := woxwidget.Container{Width: labelWidth, Height: 28, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Text{Value: leftLabel, Style: woxui.TextStyle{Size: 12}, Color: snapshot.palette.toolbarText}}
	leftWidgets := make([]woxwidget.Widget, 0, 3)
	if leftIcon != nil {
		leftWidgets = append(leftWidgets, woxwidget.Container{Width: 18, Height: 28, Padding: woxwidget.Insets{Top: 5}, Child: woxwidget.Image{Source: leftIcon, Width: 18, Height: 18}})
	}
	leftWidgets = append(leftWidgets, label)
	if progressLabel != "" {
		leftWidgets = append(leftWidgets, woxwidget.Container{Width: progressWidth, Height: 28, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Text{Value: progressLabel, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.cursor}})
	}
	body := woxwidget.Container{
		Width: width, Height: height, Color: snapshot.palette.toolbarBackground, Padding: woxwidget.Insets{Left: 10, Top: 6, Right: 10, Bottom: 6},
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Container{Width: leftWidth, Height: 28, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: leftWidgets}},
			woxwidget.Painter{Width: max(float32(0), contentWidth-leftWidth-rightWidth), Height: 1},
			woxwidget.Container{Width: rightWidth, Height: 28, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 16, Children: rightChildren}},
		}},
	}
	border := snapshot.palette.toolbarText
	border.A = min(border.A, uint8(26))
	return woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
		{Child: body},
		{Child: woxwidget.Painter{Width: width, Height: 1, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) { displayList.FillRect(bounds, border) }}},
	}}
}

// buildToolbarAction renders the same label-and-keycap unit used by Flutter's launcher toolbar.
func (a *App) buildToolbarAction(id, label, hotkey string, palette uiPalette, onTap func()) (woxwidget.Widget, float32) {
	labelStyle := woxui.TextStyle{Size: 12}
	hotkeyStyle := woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}
	hotkeyLabel := formatHotkeyLabel(hotkey)
	labelMetrics, _ := a.window.MeasureText(label, labelStyle)
	hotkeyMetrics, _ := a.window.MeasureText(hotkeyLabel, hotkeyStyle)
	chipWidth := max(float32(28), hotkeyMetrics.Size.Width+12)
	width := labelMetrics.Size.Width + 8 + chipWidth
	border := palette.toolbarText
	border.A = min(border.A, uint8(110))
	chip := woxwidget.Container{Width: chipWidth, Height: 28, Radius: 5, Color: border, Padding: woxwidget.UniformInsets(1), Child: woxwidget.Container{
		Width: chipWidth - 2, Height: 26, Radius: 4, Color: palette.toolbarBackground,
		Padding: woxwidget.Insets{Left: 5, Top: 6, Right: 5, Bottom: 4}, Child: woxwidget.Text{Value: hotkeyLabel, Style: hotkeyStyle, Color: palette.toolbarText},
	}}
	content := woxwidget.Container{Width: width, Height: 28, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
		woxwidget.Container{Width: labelMetrics.Size.Width, Height: 28, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Text{Value: label, Style: labelStyle, Color: palette.toolbarText}},
		chip,
	}}}
	return woxwidget.Gesture{ID: id, OnTap: onTap, Child: content}, width
}
