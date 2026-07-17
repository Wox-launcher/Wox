package launcher

import (
	"fmt"
	"log"
	"math"
	"strings"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
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
	hoveredResult         int
	layout                queryLayout
	refinements           []queryRefinement
	refinementValues      map[string]string
	refinementOpen        bool
	completionHint        *queryCompletionHint
	toolbarMsg            *toolbarMessage
	glance                *glanceItem
	glanceHovered         bool
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
		hoveredResult:         a.hoveredResult,
		layout:                a.layout,
		refinements:           append([]queryRefinement(nil), a.refinements...),
		refinementValues:      refinementValues,
		refinementOpen:        a.refinementOpen,
		completionHint:        completionHint,
		toolbarMsg:            toolbarMsg,
		glance:                glance,
		glanceHovered:         a.glanceHovered,
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

func (a *App) buildLauncher(frame woxui.FrameInfo) woxwidget.Widget {
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
	var header woxwidget.Widget
	if queryHeight > 0 {
		header = a.buildHeader(snapshot, width, queryHeight)
	}
	var refinements woxwidget.Widget
	if refinementHeight > 0 {
		refinements = a.buildRefinementBar(snapshot, width, refinementHeight)
	}
	var footer woxwidget.Widget
	if toolbarHeight > 0 {
		footer = a.buildFooter(snapshot, width, toolbarHeight)
	}
	var floating *launcherview.LauncherFloatingView
	if snapshot.form != nil {
		queryChromeHeight := queryHeight + refinementHeight
		panel, panelWidth, panelHeight := a.buildFormPanel(snapshot, width)
		floating = &launcherview.LauncherFloatingView{Child: panel, Left: max(float32(14), width-panelWidth-14), Top: max(queryChromeHeight+8, height-toolbarHeight-panelHeight-12)}
	} else if snapshot.actionPanel {
		queryChromeHeight := queryHeight + refinementHeight
		panel, panelWidth, panelHeight := a.buildActionPanel(snapshot, width, height, queryChromeHeight, toolbarHeight)
		if panel != nil {
			rightOffset := snapshot.palette.appPadding.Right + 10
			bottomOffset := snapshot.palette.appPadding.Bottom + 10
			floating = &launcherview.LauncherFloatingView{Child: panel, Left: max(rightOffset, width-panelWidth-rightOffset), Top: max(queryChromeHeight+8, height-toolbarHeight-panelHeight-bottomOffset)}
		}
	}
	var overlay woxwidget.Widget
	if snapshot.tableEditor != nil {
		overlay = a.buildFormTableOverlay(snapshot.tableEditor, snapshot.palette, width, height)
	}
	return launcherview.LauncherView(launcherview.LauncherViewProps{
		Width: width, Height: height, Radius: appSurfaceRadius(), Header: header, Refinements: refinements, Content: content, Footer: footer,
		QueryAtBottom: snapshot.show.QueryBoxAtBottom, Floating: floating, Overlay: overlay, Theme: snapshot.palette.componentTheme(),
	})
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
	var queryIcon *woxui.Image
	if snapshot.glance == nil {
		if image := a.imageFor(snapshot.layout.Icon); image != nil {
			queryIcon = image
			queryWidth -= 30 + accessoryGap
		}
	}
	queryWidth = max(float32(140), queryWidth)
	var refinement woxwidget.Widget
	if len(snapshot.refinements) > 0 {
		refinement = a.buildRefinementToggle(snapshot)
	}
	var glance woxwidget.Widget
	if snapshot.glance != nil {
		glance = a.buildGlance(*snapshot.glance, snapshot.glanceHovered, snapshot.hideGlanceIcon, snapshot.palette, glanceWidth)
	}
	return launcherview.LauncherHeaderView(launcherview.LauncherHeaderProps{
		Width: width, Height: height, QueryBoxHeight: queryBoxHeight, QueryEditorHeight: queryEditorHeight,
		QueryWidth: queryWidth, QueryRadius: snapshot.palette.queryRadius, AppPadding: snapshot.palette.appPadding, Theme: snapshot.palette.componentTheme(),
		Query: a.queryViewProps(snapshot, queryWidth, queryEditorHeight), Refinement: refinement, RefinementWidth: refinementWidth,
		Glance: glance, GlanceWidth: glanceWidth, Icon: queryIcon,
	})
}

// queryViewProps prepares text slices and measurements without exposing controller state to the view.
func (a *App) queryViewProps(snapshot viewSnapshot, width, height float32) launcherview.LauncherQueryProps {
	const caretHeight = float32(34)
	style := woxui.TextStyle{Size: 28}
	queryFocused := snapshot.form == nil && snapshot.tableEditor == nil && !snapshot.requirementFormActive && !snapshot.actionPanel
	state := snapshot.editing
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
	caretPrefix := string(runes[:focus])
	if state.Composition != "" {
		caretPrefix = prefix + state.Composition
	}
	measure := func(value string) float32 {
		metrics, _ := a.window.MeasureText(value, style)
		return metrics.Size.Width
	}
	completionSuffix := ""
	if queryFocused && state.Composition == "" && state.Selection.Collapsed() && state.Selection.Focus == len(runes) && snapshot.completionHint != nil && snapshot.completionHint.InputPrefix == state.Text {
		completionSuffix = snapshot.completionHint.Suffix
	}
	return launcherview.LauncherQueryProps{
		Width: width, Height: height, Style: style, State: state, DisplayValue: displayValue, Selected: selected,
		CompletionSuffix: completionSuffix, PrefixWidth: measure(prefix), SelectedWidth: measure(selected), CaretWidth: measure(caretPrefix),
		CompositionWidth: measure(state.Composition), FocusWidth: measure(string(runes[:focus])), TextWidth: measure(state.Text), CaretHeight: caretHeight,
		Focused: queryFocused, Theme: snapshot.palette.componentTheme(), OnTapAt: func(x float32) { a.placeQueryCaret(x, style) },
		OnTapEnd: func() { a.placeQueryCaret(width, style) }, OnDragStart: func() {
			if err := a.window.StartDragging(); err != nil {
				log.Printf("start launcher window drag: %v", err)
			}
		},
		OnKey: a.onKey, OnTextInput: func(event woxui.TextInputEvent) bool { a.onTextInput(event); return true }, OnSetValue: a.setQueryText,
		OnTextInputState: func(state woxui.TextInputState) { _ = a.window.SetTextInputState(state) },
	}
}

// setQueryText applies an accessibility or automation value through the normal query pipeline.
func (a *App) setQueryText(value string) error {
	a.deactivateRequirementForm()
	a.mu.Lock()
	a.editor.SetText(value, false)
	a.applyQueryTextChangeLocked(value)
	a.mu.Unlock()
	a.deactivateTerminalPreview()
	_ = a.window.Invalidate()
	return a.sendCurrentQuery()
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
		return launcherview.LauncherEmptyResultsView(width, height, snapshot.pendingResults, "Type a query to search Wox plugins", snapshot.palette.resultSubtitle)
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
	return launcherview.LauncherSplitContentView(
		a.buildResults(snapshot, splitX, height),
		a.buildPreview(snapshot.results[snapshot.selected], snapshot.palette, width-splitX, height),
	)
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
	rowPadding := snapshot.palette.resultItemPadding
	rowPadding.Left += 5
	rowPadding.Right += 5
	contentHeight := containerPadding.Top + containerPadding.Bottom + float32(len(snapshot.results))*rowHeight + float32(max(0, len(snapshot.results)-1)*resultRowGap)
	offset := a.configureResultScroll(snapshot.results, nil, snapshot.selected, width, height, contentHeight)
	start, end := visibleResultRange(len(snapshot.results), offset, height, containerPadding.Top, rowHeight, resultRowGap)
	items := make([]launcherview.LauncherResultItem, 0, end-start)
	for index := start; index < end; index++ {
		index := index
		result := snapshot.results[index]
		if result.IsGroup {
			items = append(items, launcherview.LauncherResultItem{
				ID: result.ID, Title: result.Title, Group: true, Selected: index == snapshot.selected, Hovered: index == snapshot.hoveredResult,
			})
			continue
		}
		tails, tailWidth := a.resultTailViewProps(result.Tails, width)
		titleHeight := float32(0)
		if result.SubTitle == "" {
			metrics, _ := a.window.MeasureText(result.Title, woxui.TextStyle{Size: 15})
			titleHeight = metrics.Size.Height
		}
		items = append(items, launcherview.LauncherResultItem{
			ID: result.ID, Title: result.Title, Subtitle: result.SubTitle, Selected: index == snapshot.selected, Hovered: index == snapshot.hoveredResult,
			Icon: a.imageFor(result.Icon), TitleHeight: titleHeight, Tails: tails, TailWidth: tailWidth,
			OnHover: func(inside bool) { a.hoverResult(index, inside) }, OnSelect: func() { a.selectResult(index) }, OnActivate: func() { a.activateResult(index) },
			OnKey: func(event woxui.KeyEvent) bool {
				if !event.Down || event.Composing {
					return false
				}
				switch event.Key {
				case woxui.KeyEnter:
					a.selectResult(index)
					a.activateResult(index)
					return true
				case woxui.KeyArrowUp:
					a.moveSelection(-a.resultNavigationColumns())
					return true
				case woxui.KeyArrowDown:
					a.moveSelection(a.resultNavigationColumns())
					return true
				case woxui.KeyEscape:
					return a.onKey(event)
				default:
					return false
				}
			},
		})
	}
	return launcherview.LauncherResultsView(launcherview.LauncherResultsProps{
		Width: width, Height: height, ContentHeight: contentHeight, Offset: offset, StartIndex: start, RowHeight: rowHeight, RowGap: resultRowGap,
		ContainerPadding: containerPadding, ItemPadding: rowPadding, ItemRadius: snapshot.palette.resultItemRadius,
		TailColor: snapshot.palette.resultTail, SelectedTailColor: snapshot.palette.selectedTail, Theme: snapshot.palette.componentTheme(), Items: items,
		OnScroll: a.scrollResults,
	})
}

// resultTailViewProps resolves tail images and bounds their measured widths before rendering.
func (a *App) resultTailViewProps(tails []resultTail, rowWidth float32) ([]launcherview.LauncherResultTail, float32) {
	const gap = float32(5)
	style := woxui.TextStyle{Size: 11}
	maximum := min(float32(280), max(float32(88), rowWidth*0.4))
	items := make([]launcherview.LauncherResultTail, 0, len(tails))
	used := float32(0)
	for _, tail := range tails {
		itemWidth := float32(32)
		image := a.imageFor(tail.Image)
		if image == nil {
			if tail.Text == "" {
				continue
			}
			metrics, _ := a.window.MeasureText(tail.Text, style)
			itemWidth = min(float32(88), max(float32(30), metrics.Size.Width+14))
		}
		nextWidth := itemWidth
		if len(items) > 0 {
			nextWidth += gap
		}
		if used+nextWidth > maximum {
			break
		}
		if len(items) > 0 {
			used += gap
		}
		used += itemWidth
		items = append(items, launcherview.LauncherResultTail{Text: tail.Text, Image: image, Width: itemWidth})
	}
	return items, used
}

// visibleResultRange returns the viewport rows plus a small buffer for smooth scrolling.
func visibleResultRange(count int, offset, viewport, topPadding, rowHeight, gap float32) (int, int) {
	if count <= 0 || rowHeight <= 0 {
		return 0, 0
	}
	const overscan = 2
	stride := rowHeight + gap
	start := int(math.Floor(float64((offset-topPadding)/stride))) - overscan
	end := int(math.Ceil(float64((offset+viewport-topPadding)/stride))) + overscan
	start = max(0, min(count, start))
	end = max(start, min(count, end))
	return start, end
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
	} else {
		for index := selected - 1; index >= 0; index-- {
			if results[index].IsGroup {
				if selected-index <= 2 {
					top = a.palette.resultContainerPadding.Top + float32(index)*(rowHeight+resultRowGap)
				}
				break
			}
		}
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

func (a *App) buildFooter(snapshot viewSnapshot, width, height float32) woxwidget.Widget {
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
	actions := make([]launcherview.LauncherToolbarAction, 0)
	if snapshot.selected >= 0 && snapshot.selected < len(snapshot.results) {
		resultIndex := snapshot.selected
		for actionIndex, action := range snapshot.results[resultIndex].Actions {
			if strings.TrimSpace(action.Hotkey) == "" {
				continue
			}
			actionIndex := actionIndex
			action := action
			actions = append(actions, launcherview.LauncherToolbarAction{
				ID: fmt.Sprintf("result-toolbar-action-%d", actionIndex), Label: a.translate(action.Name), HotkeyLabels: formatHotkeyLabels(action.Hotkey),
				OnTap: func() { a.activateAction(resultIndex, actionIndex) },
			})
		}
		if len(snapshot.results[resultIndex].Actions) > 0 {
			actions = append(actions, launcherview.LauncherToolbarAction{
				ID: "result-toolbar-more", Label: a.translate("i18n:toolbar_more_actions"), HotkeyLabels: formatHotkeyLabels(primaryHotkey("j")), OnTap: a.toggleActionPanel,
			})
		}
	} else if snapshot.toolbarMsg != nil {
		for _, action := range snapshot.toolbarMsg.Actions {
			if strings.TrimSpace(action.Hotkey) == "" {
				continue
			}
			action := action
			actions = append(actions, launcherview.LauncherToolbarAction{
				ID: "toolbar-action-" + action.ID, Label: a.translate(action.Name), HotkeyLabels: formatHotkeyLabels(action.Hotkey), OnTap: func() { a.activateToolbarAction(action) },
			})
		}
	}
	return launcherview.LauncherToolbarView(launcherview.LauncherToolbarProps{
		Width: width, Height: height, Padding: snapshot.palette.toolbarPadding, Theme: snapshot.palette.componentTheme(), Window: a.window,
		Label: leftLabel, Icon: leftIcon, ProgressLabel: progressLabel, Actions: actions,
	})
}
