package launcher

import (
	"fmt"

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
	foreground := snapshot.palette.queryText
	muted := snapshot.palette.resultSubtitle
	var identity woxwidget.Widget = woxwidget.Text{Value: "WOX", Style: woxui.TextStyle{Size: 22, Weight: woxui.FontWeightSemibold}, Color: foreground}
	if image := a.imageFor(snapshot.layout.Icon); image != nil {
		identity = woxwidget.Image{Source: image, Width: 28, Height: 28}
	}
	children := []woxwidget.Widget{identity}
	queryWidth := max(float32(140), width-300)
	glanceWidth := float32(0)
	if snapshot.glance != nil {
		glanceWidth = 160
		queryWidth = max(float32(140), queryWidth-glanceWidth-12)
	}
	if len(snapshot.refinements) > 0 {
		queryWidth = max(float32(140), queryWidth-140)
	}
	children = append(children, a.buildQuery(snapshot, queryWidth, 30))
	if len(snapshot.refinements) > 0 {
		children = append(children, a.buildRefinementToggle(snapshot))
	}
	if snapshot.glance != nil {
		children = append(children, a.buildGlance(*snapshot.glance, snapshot.hideGlanceIcon, snapshot.palette, glanceWidth))
	}
	children = append(children, woxwidget.Text{Value: "Alt + Space", Style: woxui.TextStyle{Size: 13}, Color: muted})
	return woxwidget.Container{
		Width: width, Height: height,
		Padding: woxwidget.Insets{Left: 20, Top: 18, Right: 20, Bottom: 18},
		Child: woxwidget.Container{
			Width: width - 40, Height: 52, Radius: 9, Color: snapshot.palette.queryBackground,
			Padding: woxwidget.Insets{Left: 16, Top: 11, Right: 16, Bottom: 11},
			Child:   woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: children},
		},
	}
}

func (a *App) buildQuery(snapshot viewSnapshot, width, height float32) woxwidget.Widget {
	style := woxui.TextStyle{Size: 16}
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
			if displayValue == "" {
				displayValue = "Start typing to search"
				color = snapshot.palette.resultSubtitle
			}
			if queryFocused && state.Composition == "" && state.Selection.Collapsed() && state.Selection.Focus == len(runes) && snapshot.completionHint != nil && snapshot.completionHint.InputPrefix == state.Text {
				valueMetrics, _ := a.window.MeasureText(state.Text, style)
				hintColor := snapshot.palette.queryText
				hintColor.A = 96
				displayList.DrawText(snapshot.completionHint.Suffix, woxui.Rect{X: bounds.X + valueMetrics.Size.Width, Y: bounds.Y, Width: max(float32(0), bounds.Width-valueMetrics.Size.Width), Height: bounds.Height}, style, hintColor)
			}

			prefixMetrics, _ := a.window.MeasureText(prefix, style)
			selectedMetrics, _ := a.window.MeasureText(selected, style)
			if queryFocused && state.Composition == "" && start != end {
				displayList.FillRoundedRect(woxui.Rect{X: bounds.X + prefixMetrics.Size.Width, Y: bounds.Y, Width: selectedMetrics.Size.Width, Height: 22}, 3, snapshot.palette.selectionBackground)
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
			displayList.FillRect(woxui.Rect{X: cursorX, Y: bounds.Y, Width: 1, Height: 22}, snapshot.palette.cursor)
			_ = a.window.SetTextInputState(woxui.TextInputState{Enabled: true, CursorRect: woxui.Rect{X: cursorX, Y: bounds.Y, Width: 1, Height: 24}})
			if state.Composition != "" {
				compositionMetrics, _ := a.window.MeasureText(state.Composition, style)
				displayList.FillRect(woxui.Rect{X: bounds.X + prefixMetrics.Size.Width, Y: bounds.Y + 23, Width: compositionMetrics.Size.Width, Height: 1}, snapshot.palette.cursor)
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
	ratio := float32(0.56)
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
		woxwidget.Painter{Width: 1, Height: height, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			displayList.FillRect(bounds, snapshot.palette.previewSplit)
		}},
		a.buildPreview(snapshot.results[snapshot.selected], snapshot.palette, width-splitX-1, height),
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
				Width: width - 28, Height: resultRowHeight, Padding: woxwidget.Insets{Left: 14, Top: 18},
				Child: woxwidget.Text{Value: result.Title, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: subtitle},
			})
			continue
		}
		var icon woxwidget.Widget = woxwidget.Container{Width: 32, Height: 32, Radius: 8, Color: resultColors[index%len(resultColors)]}
		if image := a.imageFor(result.Icon); image != nil {
			icon = woxwidget.Image{Source: image, Width: 32, Height: 32}
		}
		tailWidth := float32(0)
		gapWidth := float32(16)
		var tail woxwidget.Widget
		if len(result.Tails) > 0 {
			tail, tailWidth = a.buildResultTails(result.Tails, snapshot.palette, subtitle, width)
			gapWidth = 32
		}
		contentWidth := max(float32(0), width-56)
		labelWidth := max(float32(50), contentWidth-32-gapWidth-tailWidth)
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
				Width: width - 28, Height: resultRowHeight, Radius: 9, Color: background,
				Padding: woxwidget.Insets{Left: 14, Top: 7, Right: 14, Bottom: 7},
				Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 16, Children: []woxwidget.Widget{
					icon,
					woxwidget.Container{Width: labelWidth, Height: 44, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
						woxwidget.Text{Value: result.Title, Style: woxui.TextStyle{Size: 17, Weight: woxui.FontWeightSemibold}, Color: title},
						woxwidget.Text{Value: result.SubTitle, Style: woxui.TextStyle{Size: 13}, Color: subtitle},
					}}},
					tail,
				}},
			},
		})
	}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 14, Right: 14}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: resultRowGap, Children: rows}}
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
	muted := snapshot.palette.toolbarText
	leftLabel := "Wox core + Go GPU UI"
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
	actionLabel := "Enter to run"
	var toolbarAction *toolbarMessageAction
	if snapshot.selected >= 0 && snapshot.selected < len(snapshot.results) {
		if action, ok := defaultAction(snapshot.results[snapshot.selected].Actions); ok && action.Name != "" {
			actionLabel = a.translate(action.Name)
		}
	} else if snapshot.toolbarMsg != nil {
		if action, ok := defaultToolbarAction(snapshot.toolbarMsg.Actions); ok {
			actionLabel = a.translate(action.Name)
			copy := action
			toolbarAction = &copy
		}
	}
	var actionWidget woxwidget.Widget = woxwidget.Text{Value: actionLabel, Style: woxui.TextStyle{Size: 13}, Color: muted}
	if toolbarAction != nil {
		action := *toolbarAction
		actionWidget = woxwidget.Gesture{ID: "toolbar-action-" + action.ID, OnTap: func() { a.activateToolbarAction(action) }, Child: actionWidget}
	}
	contentWidth := max(float32(0), width-56)
	actionMetrics, _ := a.window.MeasureText(actionLabel, woxui.TextStyle{Size: 13})
	actionWidth := min(max(float32(90), actionMetrics.Size.Width+4), max(float32(90), contentWidth*0.45))
	leftWidth := max(float32(0), contentWidth-actionWidth-16)
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
	label := woxwidget.Container{Width: labelWidth, Height: 20, Child: woxwidget.Text{Value: leftLabel, Style: woxui.TextStyle{Size: 13}, Color: muted}}
	leftWidgets := make([]woxwidget.Widget, 0, 3)
	if leftIcon != nil {
		leftWidgets = append(leftWidgets, woxwidget.Image{Source: leftIcon, Width: 18, Height: 18})
	}
	leftWidgets = append(leftWidgets, label)
	if progressLabel != "" {
		leftWidgets = append(leftWidgets, woxwidget.Container{Width: progressWidth, Height: 20, Child: woxwidget.Text{Value: progressLabel, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.cursor}})
	}
	return woxwidget.Container{
		Width: width, Height: height, Color: snapshot.palette.toolbarBackground,
		Padding: woxwidget.Insets{Left: 28, Top: 13, Right: 28, Bottom: 10},
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Container{Width: leftWidth, Height: 20, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: leftWidgets}},
			woxwidget.Painter{Width: 16, Height: 1},
			woxwidget.Container{Width: actionWidth, Height: 20, Child: actionWidget},
		}},
	}
}
