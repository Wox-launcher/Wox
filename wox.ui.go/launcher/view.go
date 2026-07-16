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
	actionEditing         woxui.TextEditingState
	actionIndices         []int
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
	var actionEditing woxui.TextEditingState
	var actionIndices []int
	if a.actionPanel && a.actionFilter != nil {
		actionEditing = a.actionFilter.State()
		if a.selected >= 0 && a.selected < len(a.results) {
			actionIndices = filteredActionIndices(a.results[a.selected].Actions, actionEditing.Text, a.translations)
		}
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
		actionEditing:         actionEditing,
		actionIndices:         actionIndices,
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
		queryHeight = queryBoxHeight + snapshot.palette.appPadding.Top
	}
	toolbarHeight := float32(0)
	if !snapshot.show.HideToolbar && !snapshot.chatFullscreen && (len(snapshot.results) > 0 || snapshot.toolbarMsg != nil) {
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
			rightOffset := snapshot.palette.appPadding.Right + 10
			bottomOffset := snapshot.palette.appPadding.Bottom + 10
			body = woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
				{Child: body},
				{Left: max(rightOffset, width-panelWidth-rightOffset), Top: max(queryChromeHeight+8, height-toolbarHeight-panelHeight-bottomOffset), Child: panel},
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
	const queryLeftPadding = float32(8)
	const accessoryGap = float32(12)
	horizontalPadding := snapshot.palette.appPadding.Left + snapshot.palette.appPadding.Right
	contentWidth := max(float32(0), width-horizontalPadding-queryLeftPadding-6)
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
	refinementWidth := float32(0)
	if len(snapshot.refinements) > 0 {
		refinementWidth = a.refinementToggleWidth(snapshot)
		queryWidth -= refinementWidth + accessoryGap
	}
	var queryIcon woxwidget.Widget
	if snapshot.glance == nil {
		if image := a.imageFor(snapshot.layout.Icon); image != nil {
			queryIcon = woxwidget.Container{Width: 30, Height: 34, Padding: woxwidget.Insets{Top: 2}, Child: woxwidget.Image{Source: image, Width: 30, Height: 30}}
			queryWidth -= 30 + accessoryGap
		}
	}
	queryWidth = max(float32(140), queryWidth)
	children := []woxwidget.Widget{woxwidget.Container{
		Width: queryWidth, Height: queryBoxHeight, Padding: woxwidget.Insets{Top: 4, Bottom: 17},
		Child: a.buildQuery(snapshot, queryWidth, 34),
	}}
	if len(snapshot.refinements) > 0 {
		children = append(children, woxwidget.Container{
			Width: refinementWidth, Height: queryBoxHeight, Padding: woxwidget.Insets{Top: 10.5, Bottom: 10.5},
			Child: a.buildRefinementToggle(snapshot),
		})
	}
	if snapshot.glance != nil {
		children = append(children, woxwidget.Container{
			Width: glanceWidth, Height: queryBoxHeight, Padding: woxwidget.Insets{Top: 12.5, Bottom: 12.5},
			Child: a.buildGlance(*snapshot.glance, snapshot.hideGlanceIcon, snapshot.palette, glanceWidth),
		})
	}
	if queryIcon != nil {
		children = append(children, woxwidget.Container{
			Width: 30, Height: queryBoxHeight, Padding: woxwidget.Insets{Top: 10.5, Bottom: 10.5},
			Child: queryIcon,
		})
	}
	return woxwidget.Container{
		Width: width, Height: height,
		Padding: woxwidget.Insets{Left: snapshot.palette.appPadding.Left, Top: snapshot.palette.appPadding.Top, Right: snapshot.palette.appPadding.Right},
		Child: woxwidget.Container{
			Width: width - horizontalPadding, Height: queryBoxHeight, Radius: snapshot.palette.queryRadius, Color: snapshot.palette.queryBackground,
			Padding: woxwidget.Insets{Left: queryLeftPadding, Right: 6},
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
			queryFocused := snapshot.form == nil && !snapshot.requirementFormActive && !snapshot.actionPanel
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
	rowHeight := resultRowHeightForPalette(snapshot.palette)
	containerPadding := snapshot.palette.resultContainerPadding
	containerPadding.Left += snapshot.palette.appPadding.Left
	containerPadding.Right += snapshot.palette.appPadding.Right
	containerPadding.Bottom += snapshot.palette.appPadding.Bottom
	rowWidth := max(float32(0), width-containerPadding.Left-containerPadding.Right)
	rowPadding := snapshot.palette.resultItemPadding
	rowPadding.Left += 5
	rowPadding.Right += 5
	innerRowWidth := max(float32(0), rowWidth-rowPadding.Left-rowPadding.Right)
	rows := make([]woxwidget.Widget, 0, len(snapshot.results))
	for index, result := range snapshot.results {
		index := index
		result := result
		selected := index == snapshot.selected
		background := woxui.Color{}
		title := snapshot.palette.resultTitle
		subtitle := snapshot.palette.resultSubtitle
		tailColor := snapshot.palette.resultTail
		if selected {
			background = snapshot.palette.selectedBackground
			title = snapshot.palette.selectedTitle
			subtitle = snapshot.palette.selectedSubtitle
			tailColor = snapshot.palette.selectedTail
		}
		if result.IsGroup {
			rows = append(rows, woxwidget.Container{
				Width: rowWidth, Height: rowHeight, Padding: woxwidget.Insets{Left: 8, Top: 18},
				Child: woxwidget.Text{Value: result.Title, Style: woxui.TextStyle{Size: 15}, Color: subtitle},
			})
			continue
		}
		var icon woxwidget.Widget = woxwidget.Painter{Width: 28, Height: 28}
		if image := a.imageFor(result.Icon); image != nil {
			icon = woxwidget.Image{Source: image, Width: 28, Height: 28}
		}
		tailWidth := float32(0)
		var tail woxwidget.Widget
		if len(result.Tails) > 0 {
			tail, tailWidth = a.buildResultTails(result.Tails, snapshot.palette, tailColor, width)
		}
		labelWidth := max(float32(50), innerRowWidth-28-20-tailWidth)
		labelChildren := []woxwidget.Widget{
			woxwidget.Text{Value: result.Title, Style: woxui.TextStyle{Size: 15}, Color: title},
		}
		labelTop := float32(7)
		labelGap := float32(0)
		if result.SubTitle != "" {
			labelChildren = append(labelChildren, woxwidget.Text{Value: result.SubTitle, Style: woxui.TextStyle{Size: 12}, Color: subtitle})
			labelGap = 2
		} else {
			metrics, _ := a.window.MeasureText(result.Title, woxui.TextStyle{Size: 15})
			labelTop = max(float32(0), (50-metrics.Size.Height)/2)
		}
		rows = append(rows, woxwidget.Gesture{
			ID: fmt.Sprintf("result-%s", result.ID),
			OnHover: func(inside bool) {
				if inside {
					a.selectResult(index)
				}
			},
			OnTap: func() { a.activateResult(index) },
			Child: woxwidget.Container{
				Width: rowWidth, Height: rowHeight, Radius: snapshot.palette.resultItemRadius, Color: background,
				Padding: rowPadding,
				Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
					woxwidget.Container{Width: 28, Height: 50, Padding: woxwidget.Insets{Top: 11}, Child: icon},
					woxwidget.Container{Width: labelWidth, Height: 50, Padding: woxwidget.Insets{Top: labelTop}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: labelGap, Children: labelChildren}},
					woxwidget.Container{Width: tailWidth, Height: 50, Padding: woxwidget.Insets{Top: 9}, Child: tail},
				}},
			},
		})
	}
	contentHeight := containerPadding.Top + containerPadding.Bottom + float32(len(rows))*rowHeight + float32(max(0, len(rows)-1)*resultRowGap)
	offset := a.configureResultScroll(snapshot.results, nil, snapshot.selected, width, height, contentHeight)
	content := woxwidget.Container{Width: width, Height: contentHeight, Padding: containerPadding, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: resultRowGap, Children: rows}}
	return a.buildResultScrollSurface(content, snapshot.palette, width, height, contentHeight, offset)
}

// configureResultScroll keeps the portable viewport geometry aligned with the current result layout.
func (a *App) configureResultScroll(results []queryResult, layout *gridLayout, selected int, width, viewport, content float32) float32 {
	a.mu.Lock()
	a.resultWidth = width
	a.resultViewport = viewport
	a.resultContent = content
	a.resultScroll = min(max(float32(0), a.resultScroll), max(float32(0), content-viewport))
	a.ensureResultIndexVisibleLocked(results, layout, selected)
	offset := a.resultScroll
	a.mu.Unlock()
	return offset
}

// ensureResultSelectionVisibleLocked follows keyboard selection without changing pointer-driven scrolling.
func (a *App) ensureResultSelectionVisibleLocked() {
	a.ensureResultIndexVisibleLocked(a.results, a.layout.GridLayout, a.selected)
}

func (a *App) ensureResultIndexVisibleLocked(results []queryResult, layout *gridLayout, selected int) {
	if selected < 0 || selected >= len(results) || a.resultViewport <= 0 || a.resultContent <= a.resultViewport {
		a.resultScroll = min(max(float32(0), a.resultScroll), max(float32(0), a.resultContent-a.resultViewport))
		return
	}
	rowHeight := resultRowHeightForPalette(a.palette)
	top := a.palette.resultContainerPadding.Top + float32(selected)*(rowHeight+resultRowGap)
	bottom := top + rowHeight
	if layout != nil {
		top, bottom = gridResultVerticalBounds(results, selected, a.resultWidth, layout)
	}
	if top < a.resultScroll {
		a.resultScroll = top
	} else if bottom > a.resultScroll+a.resultViewport {
		a.resultScroll = bottom - a.resultViewport
	}
	a.resultScroll = min(max(float32(0), a.resultScroll), max(float32(0), a.resultContent-a.resultViewport))
}

func (a *App) scrollResults(delta float32) {
	a.mu.Lock()
	a.resultScroll = min(max(float32(0), a.resultScroll+delta), max(float32(0), a.resultContent-a.resultViewport))
	a.mu.Unlock()
}

// buildResultScrollSurface overlays the portable thumb on the same clipped content used by list and grid results.
func (a *App) buildResultScrollSurface(content woxwidget.Widget, palette uiPalette, width, height, contentHeight, offset float32) woxwidget.Widget {
	children := []woxwidget.StackChild{{Child: woxwidget.ScrollView{Width: width, Height: height, ContentHeight: contentHeight, Offset: offset, Child: content}}}
	if contentHeight > height && height > 0 {
		thumbHeight := max(float32(24), height*height/contentHeight)
		thumbTop := (height - thumbHeight) * offset / (contentHeight - height)
		thumbColor := palette.resultSubtitle
		thumbColor.A = min(150, thumbColor.A)
		children = append(children, woxwidget.StackChild{Left: max(float32(0), width-5), Top: thumbTop, Child: woxwidget.Container{Width: 3, Height: thumbHeight, Radius: 2, Color: thumbColor}})
	}
	return woxwidget.Gesture{ID: "result-scroll", OnScroll: func(delta woxui.Point) { a.scrollResults(-delta.Y) }, Child: woxwidget.Stack{Width: width, Height: height, Children: children}}
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
	contentWidth := max(float32(0), width-snapshot.palette.toolbarPadding.Left-snapshot.palette.toolbarPadding.Right)
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
		Width: width, Height: height, Color: snapshot.palette.toolbarBackground, Padding: woxwidget.Insets{Left: snapshot.palette.toolbarPadding.Left, Top: 6, Right: snapshot.palette.toolbarPadding.Right, Bottom: 6},
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
	labelMetrics, _ := a.window.MeasureText(label, labelStyle)
	chip, chipWidth := a.buildHotkeyView(hotkey, palette.toolbarText, palette.toolbarBackground)
	width := labelMetrics.Size.Width + 8 + chipWidth
	content := woxwidget.Container{Width: width, Height: 28, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
		woxwidget.Container{Width: labelMetrics.Size.Width, Height: 28, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Text{Value: label, Style: labelStyle, Color: palette.toolbarText}},
		chip,
	}}}
	return woxwidget.Gesture{ID: id, OnTap: onTap, Child: content}, width
}

// buildHotkeyView mirrors Flutter's separate 22px keycaps while using the native system font.
func (a *App) buildHotkeyView(hotkey string, foreground, background woxui.Color) (woxwidget.Widget, float32) {
	style := woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}
	labels := formatHotkeyLabels(hotkey)
	children := make([]woxwidget.Widget, 0, len(labels))
	totalWidth := float32(0)
	for _, label := range labels {
		metrics, _ := a.window.MeasureText(label, style)
		width := max(float32(28), metrics.Size.Width+14)
		children = append(children, woxwidget.Stack{Width: width, Height: 22, Children: []woxwidget.StackChild{
			{Child: woxwidget.Container{Width: width, Height: 22, Radius: 4, Color: background}},
			{Child: woxwidget.Painter{Width: width, Height: 22, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
				displayList.StrokeRoundedRect(bounds, 4, 1, foreground)
			}}},
			{Left: max(float32(0), (width-metrics.Size.Width)/2), Top: max(float32(0), (float32(22)-metrics.Size.Height)/2), Child: woxwidget.Text{Value: label, Style: style, Color: foreground}},
		}})
		totalWidth += width
	}
	if len(children) > 1 {
		totalWidth += float32(len(children)-1) * 4
	}
	return woxwidget.Container{Width: totalWidth, Height: 28, Padding: woxwidget.Insets{Top: 3, Bottom: 3}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 4, Children: children}}, totalWidth
}
