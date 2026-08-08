package launcher

import (
	"crypto/sha256"
	"fmt"
	"log"
	"math"
	"runtime"
	"strings"
	"time"

	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

type launcherPreparedSectionProps struct {
	Signature string
	Width     float32
	Height    float32
	Child     woxwidget.Widget `boundary:"stable"`
}

// Equal compares the immutable signature and geometry of a prepared launcher section.
func (p launcherPreparedSectionProps) Equal(other launcherPreparedSectionProps) bool {
	return p.Signature == other.Signature && p.Width == other.Width && p.Height == other.Height
}

func launcherPreparedSection(key woxwidget.Key, label string, props launcherPreparedSectionProps) woxwidget.Widget {
	return woxwidget.Boundary[launcherPreparedSectionProps]{Key: key, Label: label, Props: props, Build: func(props launcherPreparedSectionProps) woxwidget.Widget { return props.Child }}
}

func launcherSectionSignature(values ...any) string {
	hash := sha256.New()
	for _, value := range values {
		_, _ = fmt.Fprintf(hash, "%#v\x00", value)
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

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
	resultsRevision       uint64
	resultsQueryID        string
	queryComplete         bool
	queryLoading          bool
	selected              int
	hoveredResult         int
	resultScroll          scrollController
	resultScrollDetached  bool
	layout                queryLayout
	refinements           []queryRefinement
	refinementsRevision   uint64
	refinementValues      map[string]string
	refinementOpen        bool
	completionHint        *queryCompletionHint
	toolbarMsg            *toolbarMessage
	glance                *glanceItem
	hideGlanceIcon        bool
	form                  *formSnapshot
	tableEditor           *formTableEditorSnapshot
	requirementFormActive bool
	queryFocused          bool
	queryEnabled          bool
	chatFullscreen        bool
	terminalFullscreen    bool
	actionPanel           bool
	actionSelected        int
	actionFilter          string
	actionEntries         []actionPanelEntry
	actionIndices         []int
	actionsRevision       uint64
	show                  showAppParams
	palette               uiPalette
	densityMetrics        launcherDensityMetrics
}

type actionSectionRevisionState struct {
	Open            bool
	Selected        int
	Filter          string
	ResultsRevision uint64
	ToolbarRevision uint64
}

// snapshot prepares one UI-thread-only render view; collection references are consumed before buildLauncher returns.
func (a *App) snapshot() viewSnapshot {
	var tableEditor *formTableEditorSnapshot
	if a.launcherTableEditor != nil && a.formTableTargetCurrentLocked(a.launcherTableEditor.target) {
		tableEditor = snapshotFormTableEditorLocked(a.launcherTableEditor)
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
	actionFilter := ""
	var actionEntries []actionPanelEntry
	var actionIndices []int
	if a.actionPanel && a.actionFilter != nil {
		actionFilter = a.actionFilter.State().Text
		actionEntries = unifiedActionPanelEntries(a.results, a.selected, a.toolbarMsg)
		actionIndices = filteredActionIndices(actionEntries, actionFilter, a.translationSnapshot(), a.usePinYin())
	}
	actionState := actionSectionRevisionState{Open: a.actionPanel, Selected: a.actionSelected, Filter: actionFilter, ResultsRevision: a.resultsSectionRevision, ToolbarRevision: a.toolbarRevision}
	if actionState != a.actionSectionState {
		a.actionSectionState = actionState
		a.actionsSectionRevision++
	}
	return viewSnapshot{
		editing:               a.editor.State(),
		results:               a.results,
		resultsRevision:       a.resultsSectionRevision,
		resultsQueryID:        a.resultsQueryID,
		queryComplete:         a.queryComplete,
		queryLoading:          a.queryLoading,
		selected:              a.selected,
		hoveredResult:         a.hoveredResult,
		resultScroll:          a.resultScroll,
		resultScrollDetached:  a.resultScrollDetached,
		layout:                a.layout,
		refinements:           a.refinements,
		refinementsRevision:   a.refinementsSectionRevision,
		refinementValues:      a.query.QueryRefinements,
		refinementOpen:        a.refinementOpen,
		completionHint:        completionHint,
		toolbarMsg:            toolbarMsg,
		glance:                glance,
		hideGlanceIcon:        a.generalSettings.Data().HideGlanceIcon,
		form:                  snapshotFormLocked(a.form),
		tableEditor:           tableEditor,
		requirementFormActive: a.requirementForm != nil && a.requirementForm.active,
		queryFocused:          a.host != nil && a.host.HasFocus(launcherview.LauncherQueryInputKey),
		queryEnabled:          a.queryCanFocus(),
		chatFullscreen:        a.chatFullscreen,
		terminalFullscreen:    a.terminalFullscreen,
		actionPanel:           a.actionPanel,
		actionSelected:        a.actionSelected,
		actionFilter:          actionFilter,
		actionEntries:         actionEntries,
		actionIndices:         actionIndices,
		actionsRevision:       a.actionsSectionRevision,
		show:                  a.show,
		palette:               a.palette,
		densityMetrics:        a.densityMetrics.normalized(),
	}
}

func (a *App) buildLauncher(frame woxui.FrameInfo) woxwidget.Widget {
	snapshotStart := time.Now()
	snapshot := a.snapshot()
	if a.host != nil {
		a.host.RecordSnapshotDuration(time.Since(snapshotStart))
	}
	width := frame.Size.Width
	height := frame.Size.Height
	queryHeight := float32(0)
	previewFullscreen := snapshot.chatFullscreen || snapshot.terminalFullscreen
	queryLineHeight := a.queryLineHeight(snapshot.densityMetrics)
	if !snapshot.show.HideQueryBox && !previewFullscreen {
		queryHeight = snapshot.densityMetrics.queryBoxHeightForText(snapshot.editing.Text, queryLineHeight) + snapshot.palette.appPadding.Top
	}
	toolbarHeight := float32(0)
	if !snapshot.show.HideToolbar && !previewFullscreen && (len(snapshot.results) > 0 || snapshot.toolbarMsg != nil) {
		toolbarHeight = snapshot.densityMetrics.toolbarHeight
	}
	refinementHeight := float32(0)
	if queryHeight > 0 && snapshot.refinementOpen && len(snapshot.refinements) > 0 {
		refinementHeight = snapshot.densityMetrics.refinementBarHeight
	}
	previewOnly := launcherPreviewOnly(snapshot)
	showTitleBar := launcherPreviewTitleBarVisible(snapshot)
	var titleBar woxwidget.Widget
	titleBarHeight := float32(0)
	if showTitleBar {
		titleBarHeight = launcherview.SettingsTitleBarHeight
		titleBar = a.buildPreviewTitleBar(snapshot, width)
	}
	contentHeight := max(0, height-queryHeight-refinementHeight-toolbarHeight-titleBarHeight)
	content := a.buildContent(snapshot, width, contentHeight, frame.Scale)
	var header woxwidget.Widget
	if queryHeight > 0 {
		header = a.buildHeader(snapshot, width, queryHeight, queryLineHeight, frame.Scale)
	}
	var refinements woxwidget.Widget
	if refinementHeight > 0 {
		refinements = a.buildRefinementBar(snapshot, width, refinementHeight, frame.Scale)
	}
	var footer woxwidget.Widget
	if toolbarHeight > 0 {
		footer = a.buildFooter(snapshot, width, toolbarHeight, frame.Scale)
	}
	var floating *launcherview.LauncherFloatingView
	if snapshot.form != nil {
		panel, panelWidth, _ := a.buildFormPanel(snapshot, width)
		panel = launcherPreparedSection("launcher-form-section", "form", launcherPreparedSectionProps{Signature: launcherSectionSignature(snapshot.form, snapshot.palette, snapshot.densityMetrics, panelWidth), Width: panelWidth, Height: height, Child: panel})
		floating = &launcherview.LauncherFloatingView{Child: panel, Left: max(float32(14), width-panelWidth-14), Bottom: toolbarHeight + 12, AnchorBottom: true}
	} else if snapshot.actionPanel {
		queryChromeHeight := queryHeight + refinementHeight
		panel, panelWidth, panelHeight := a.buildActionPanel(snapshot, width, height, queryChromeHeight, toolbarHeight, frame.Scale)
		if panel != nil {
			rightOffset := snapshot.palette.appPadding.Right + 10
			bottomOffset := snapshot.palette.appPadding.Bottom + 10
			floating = &launcherview.LauncherFloatingView{Child: panel, Left: max(rightOffset, width-panelWidth-rightOffset), Top: max(queryChromeHeight+8, height-toolbarHeight-panelHeight-bottomOffset)}
		}
	}
	var overlay woxwidget.Widget
	if snapshot.tableEditor != nil {
		overlay = a.buildFormTableOverlay(snapshot.tableEditor, snapshot.palette, width, height, frame.Scale)
		overlay = launcherPreparedSection("launcher-table-overlay-section", "table-overlay", launcherPreparedSectionProps{Signature: launcherSectionSignature(snapshot.tableEditor, snapshot.palette, width, height, frame.Scale), Width: width, Height: height, Child: overlay})
	}
	return launcherview.LauncherView(launcherview.LauncherViewProps{
		Width: width, Height: height, TitleBar: titleBar, Header: header, Refinements: refinements, Content: content, Footer: footer,
		QueryAtBottom: snapshot.show.QueryBoxAtBottom, Floating: floating, Overlay: overlay, Theme: snapshot.palette.componentTheme(),
		PreviewOnly: previewOnly, BorderWidth: snapshot.palette.appPadding.Top, OnDragStart: func() {
			if err := a.window.StartDragging(); err != nil {
				log.Printf("start preview-only window drag: %v", err)
			}
		},
	})
}

// buildPreviewTitleBar reuses the settings chrome so full previews expose consistent window actions.
func (a *App) buildPreviewTitleBar(snapshot viewSnapshot, width float32) woxwidget.Widget {
	title := "Wox"
	if snapshot.selected >= 0 && snapshot.selected < len(snapshot.results) {
		title = strings.TrimSpace(snapshot.results[snapshot.selected].Title)
		if title == "" {
			title = strings.TrimSpace(snapshot.results[snapshot.selected].SubTitle)
		}
		if title == "" {
			title = "Wox"
		}
	}
	titleStyle := woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}
	titleWidth := float32(160)
	if a.window != nil {
		if metrics, err := a.window.MeasureText(title, titleStyle); err == nil {
			titleWidth = metrics.Size.Width + 24
		}
	}
	return launcherview.SettingsTitleBar(launcherview.SettingsTitleBarProps{
		Width: width, CloseOnly: true, Title: title, TitleWidth: titleWidth, Platform: runtime.GOOS, AppIcon: a.appIcon,
		Theme: snapshot.palette.componentTheme(),
		OnDrag: func() {
			if a.window != nil {
				_ = a.window.StartDragging()
			}
		},
		OnMinimize: func() {
			if a.window != nil {
				_ = a.window.Minimize()
			}
		},
		OnClose: a.closePreviewWindow,
	})
}

// launcherPreviewOnly identifies the chrome-free layout that needs edge drag hit areas.
func launcherPreviewOnly(snapshot viewSnapshot) bool {
	if snapshot.selected < 0 || snapshot.selected >= len(snapshot.results) {
		return false
	}
	preview := snapshot.results[snapshot.selected].Preview
	return (launcherChromeHidden(snapshot.show, snapshot.chatFullscreen) || snapshot.terminalFullscreen) &&
		launcherPreviewVisible(snapshot.layout, preview) &&
		launcherPreviewRatio(snapshot.layout, snapshot.chatFullscreen || snapshot.terminalFullscreen) == 0
}

// launcherPreviewTitleBarVisible limits the opt-in title bar to chrome-free non-chat previews.
func launcherPreviewTitleBarVisible(snapshot viewSnapshot) bool {
	if !snapshot.show.ShowPreviewTitleBar || !launcherPreviewOnly(snapshot) {
		return false
	}
	return snapshot.results[snapshot.selected].Preview.PreviewType != "chat"
}

// queryLineHeight includes the configured font's native line box so glyphs are not clipped.
func (a *App) queryLineHeight(densityMetrics launcherDensityMetrics) float32 {
	style := woxui.TextStyle{Size: densityMetrics.scaled(woxcomponent.QueryFontSize)}
	metrics, _ := a.window.MeasureText("Ag国", style)
	return densityMetrics.queryLineHeight(metrics.Size.Height)
}

func (a *App) buildHeader(snapshot viewSnapshot, width, height, queryLineHeight, scale float32) woxwidget.Widget {
	queryLineCount := launcherQueryLineCount(snapshot.editing.Text)
	queryBoxHeight := snapshot.densityMetrics.queryBoxHeightForText(snapshot.editing.Text, queryLineHeight)
	queryEditorHeight := queryLineHeight + snapshot.densityMetrics.scaled(4) + float32(queryLineCount-1)*queryLineHeight
	queryLeftPadding := snapshot.densityMetrics.scaled(8)
	accessoryGap := snapshot.densityMetrics.scaled(12)
	horizontalPadding := snapshot.palette.appPadding.Left + snapshot.palette.appPadding.Right
	contentWidth := max(float32(0), width-horizontalPadding-queryLeftPadding-snapshot.densityMetrics.scaled(6))
	queryWidth := contentWidth
	glanceWidth := float32(0)
	if !snapshot.queryLoading && snapshot.glance != nil {
		metrics, _ := a.window.MeasureText(strings.TrimSpace(snapshot.glance.Text), woxui.TextStyle{Size: snapshot.densityMetrics.scaled(woxcomponent.GlanceFontSize)})
		glanceWidth = metrics.Size.Width + snapshot.densityMetrics.scaled(20)
		if !snapshot.hideGlanceIcon && snapshot.glance.Icon.ImageData != "" {
			glanceWidth += snapshot.densityMetrics.scaled(21)
		}
		glanceWidth = min(snapshot.densityMetrics.scaled(192), max(snapshot.densityMetrics.scaled(44), glanceWidth))
		queryWidth -= glanceWidth + accessoryGap
	}
	refinementWidth := float32(0)
	if !snapshot.queryLoading && len(snapshot.refinements) > 0 {
		refinementWidth = a.refinementToggleWidth(snapshot, scale)
		queryWidth -= refinementWidth + accessoryGap
	}
	var queryIcon *woxui.Image
	if !snapshot.queryLoading && snapshot.glance == nil {
		if image := a.imageForSize(snapshot.layout.Icon, physicalImageSize(int(snapshot.densityMetrics.scaled(32)), scale)); image != nil {
			queryIcon = image
			queryWidth -= snapshot.densityMetrics.scaled(49) + accessoryGap
		}
	}
	queryWidth = max(snapshot.densityMetrics.scaled(140), queryWidth)
	var refinement woxwidget.Widget
	if !snapshot.queryLoading && len(snapshot.refinements) > 0 {
		refinement = a.buildRefinementToggle(snapshot, scale)
	}
	var glance woxwidget.Widget
	if !snapshot.queryLoading && snapshot.glance != nil {
		glance = a.buildGlance(*snapshot.glance, snapshot.hideGlanceIcon, snapshot.palette, glanceWidth, scale, snapshot.densityMetrics)
	}
	loading := false
	loadingWidth := float32(0)
	loadingSize := float32(0)
	if snapshot.queryLoading {
		loading = true
		loadingWidth = snapshot.densityMetrics.scaled(49)
		loadingSize = snapshot.densityMetrics.scaled(20)
		queryWidth -= loadingWidth + accessoryGap
	}
	queryWidth = max(snapshot.densityMetrics.scaled(140), queryWidth)
	return launcherview.LauncherHeaderView(launcherview.LauncherHeaderProps{
		Width: width, Height: height, QueryBoxHeight: queryBoxHeight, QueryEditorHeight: queryEditorHeight, DensityScale: snapshot.densityMetrics.scale,
		QueryWidth: queryWidth, QueryRadius: snapshot.palette.queryRadius, AppPadding: snapshot.palette.appPadding, Theme: snapshot.palette.componentTheme(),
		Query: a.queryViewProps(snapshot, queryWidth, queryEditorHeight, queryLineHeight), Refinement: refinement, RefinementWidth: refinementWidth,
		Glance: glance, GlanceWidth: glanceWidth, Icon: queryIcon,
		Loading: loading, LoadingWidth: loadingWidth, LoadingSize: loadingSize, LoadingColor: snapshot.palette.cursor,
		OnDragStart: func() {
			if err := a.window.StartDragging(); err != nil {
				log.Printf("start launcher window drag: %v", err)
			}
		},
	})
}

// queryViewProps prepares text slices and measurements without exposing controller state to the view.
func (a *App) queryViewProps(snapshot viewSnapshot, width, height, lineHeight float32) launcherview.LauncherQueryProps {
	caretHeight := snapshot.densityMetrics.scaled(34)
	style := woxui.TextStyle{Size: snapshot.densityMetrics.scaled(woxcomponent.QueryFontSize)}
	queryFocused := snapshot.queryFocused
	state := snapshot.editing
	runes := []rune(state.Text)
	start := max(0, min(len(runes), state.Selection.Start()))
	end := max(start, min(len(runes), state.Selection.End()))
	focus := max(0, min(len(runes), state.Selection.Focus))
	displayRunes := runes
	displayStart, displayEnd, displayFocus := start, end, focus
	compositionStart := -1
	if state.Composition != "" {
		composition := []rune(state.Composition)
		displayRunes = append(append(append([]rune(nil), runes[:start]...), composition...), runes[end:]...)
		compositionStart = start
		displayStart = start + len(composition)
		displayEnd = displayStart
		displayFocus = displayStart
	}
	measure := func(value string) float32 {
		metrics, _ := a.window.MeasureText(value, style)
		return metrics.Size.Width
	}
	completionSuffix := ""
	if queryFocused && state.Composition == "" && state.Selection.Collapsed() && state.Selection.Focus == len(runes) && snapshot.completionHint != nil && snapshot.completionHint.InputPrefix == state.Text {
		completionSuffix = snapshot.completionHint.Suffix
	}
	focusQuery := func() {
		if a.host != nil {
			a.host.RequestFocus(launcherview.LauncherQueryInputKey)
		}
	}
	lines, caretLine, caretWidth, compositionLine, compositionX, compositionWidth, textWidth := queryDisplayLines(
		displayRunes, displayStart, displayEnd, displayFocus, compositionStart, len([]rune(state.Composition)), measure,
	)
	selectQueryAt := func(point woxui.Point, line bool) {
		a.hideActionPanel()
		a.deactivateRequirementForm()
		offset := a.queryOffsetAt(a.editor.State().Text, point, style, lineHeight)
		if line {
			a.editor.SelectLineAt(offset)
		} else {
			a.editor.SelectWordAt(offset)
		}
		_ = a.window.Invalidate()
	}
	return launcherview.LauncherQueryProps{
		Width: width, Height: height, LineHeight: lineHeight, Style: style, State: state, Lines: lines,
		CompletionSuffix: completionSuffix, CaretWidth: caretWidth, CaretLine: caretLine,
		CompositionWidth: compositionWidth, CompositionX: compositionX, CompositionLine: compositionLine, TextWidth: textWidth, CaretHeight: caretHeight,
		Focused: queryFocused, Enabled: snapshot.queryEnabled, Theme: snapshot.palette.componentTheme(), OnTapAt: func(point woxui.Point) { a.placeQueryCaret(point, style, lineHeight) },
		OnDoubleTapAt: func(point woxui.Point) { selectQueryAt(point, false) }, OnTripleTapAt: func(point woxui.Point) { selectQueryAt(point, true) },
		OnTapEnd: focusQuery, OnDragStart: func() {
			if err := a.window.StartDragging(); err != nil {
				log.Printf("start launcher window drag: %v", err)
			}
			focusQuery()
		},
		OnSelectionStart: func(point woxui.Point) {
			a.hideActionPanel()
			a.deactivateRequirementForm()
			text := a.editor.State().Text
			anchor := a.queryOffsetAt(text, point, style, lineHeight)
			a.selectionAnchor = anchor
			a.editor.SetCaret(anchor)
			_ = a.window.Invalidate()
		},
		OnSelectionExtend: func(point woxui.Point) {
			text := a.editor.State().Text
			focus := a.queryOffsetAt(text, point, style, lineHeight)
			a.editor.SetSelection(a.selectionAnchor, focus)
			_ = a.window.Invalidate()
		},
		OnKey: a.onKey, OnTextInput: func(event woxui.TextInputEvent) bool { a.onTextInput(event); return true }, OnFocusChange: a.onQueryFocusChanged, OnSetValue: a.setQueryText,
		OnTextInputState: func(state woxui.TextInputState) { _ = a.window.SetTextInputState(state) },
	}
}

// setQueryText applies an accessibility or automation value through the normal query pipeline.
func (a *App) setQueryText(value string) error {
	a.deactivateRequirementForm()
	previousText := a.editor.State().Text
	value = normalizeQueryNewlines(value)
	a.editor.SetText(value, false)
	a.applyQueryTextChangeLocked(value)
	a.reconcileSelectedPreview()
	_ = a.window.Invalidate()
	a.resizeLauncherForQueryLineChange(previousText)
	return a.sendCurrentQuery()
}

func (a *App) placeQueryCaret(point woxui.Point, style woxui.TextStyle, lineHeight float32) {
	a.hideActionPanel()
	a.deactivateRequirementForm()
	text := a.editor.State().Text
	offset := a.queryOffsetAt(text, point, style, lineHeight)
	a.editor.SetCaret(offset)
	_ = a.window.Invalidate()
}

// queryOffsetAt maps an editor position to a rune offset using line selection and per-rune midpoints.
func (a *App) queryOffsetAt(text string, point woxui.Point, style woxui.TextStyle, lineHeight float32) int {
	lines := queryRuneLines([]rune(text))
	lineIndex := min(len(lines)-1, max(0, int(point.Y/lineHeight)))
	line := lines[lineIndex]
	runes := line.runes
	offset := len(runes)
	previousWidth := float32(0)
	for index := 1; index <= len(runes); index++ {
		metrics, _ := a.window.MeasureText(string(runes[:index]), style)
		if point.X < (previousWidth+metrics.Size.Width)*0.5 {
			offset = index - 1
			break
		}
		previousWidth = metrics.Size.Width
	}
	return line.start + offset
}

type queryRuneLine struct {
	start int
	runes []rune
}

// queryRuneLines keeps absolute rune offsets while splitting explicit query newlines.
func queryRuneLines(runes []rune) []queryRuneLine {
	lines := make([]queryRuneLine, 0, 1+strings.Count(string(runes), "\n"))
	start := 0
	for index, current := range runes {
		if current == '\n' {
			lines = append(lines, queryRuneLine{start: start, runes: runes[start:index]})
			start = index + 1
		}
	}
	return append(lines, queryRuneLine{start: start, runes: runes[start:]})
}

// queryDisplayLines prepares complete line geometry for the retained scroll viewport.
func queryDisplayLines(runes []rune, selectionStart, selectionEnd, focus, compositionStart, compositionLength int, measure func(string) float32) ([]launcherview.LauncherQueryLine, int, float32, int, float32, float32, float32) {
	runeLines := queryRuneLines(runes)
	caretAbsoluteLine := len(runeLines) - 1
	for index, line := range runeLines {
		if focus <= line.start+len(line.runes) {
			caretAbsoluteLine = index
			break
		}
	}
	lines := make([]launcherview.LauncherQueryLine, 0, len(runeLines))
	caretLine, caretWidth := caretAbsoluteLine, float32(0)
	compositionLine, compositionX, compositionWidth := 0, float32(0), float32(0)
	textWidth := float32(0)
	for index, line := range runeLines {
		lineEnd := line.start + len(line.runes)
		selectedStart := max(line.start, selectionStart)
		selectedEnd := min(lineEnd, selectionEnd)
		selected := ""
		prefixWidth, selectedWidth := float32(0), float32(0)
		if selectedStart < selectedEnd {
			prefixWidth = measure(string(runes[line.start:selectedStart]))
			selected = string(runes[selectedStart:selectedEnd])
			selectedWidth = measure(selected)
		}
		width := measure(string(line.runes))
		lines = append(lines, launcherview.LauncherQueryLine{Text: string(line.runes), Selected: selected, PrefixWidth: prefixWidth, SelectedWidth: selectedWidth, TextWidth: width})
		textWidth = max(textWidth, width)
		if focus >= line.start && (focus <= lineEnd || index == len(runeLines)-1) {
			caretLine = index
			caretWidth = measure(string(runes[line.start:min(focus, lineEnd)]))
		}
		if compositionStart >= line.start && compositionStart <= lineEnd {
			compositionLine = index
			compositionX = measure(string(runes[line.start:compositionStart]))
			compositionWidth = measure(string(runes[compositionStart:min(len(runes), compositionStart+compositionLength)]))
		}
	}
	return lines, caretLine, caretWidth, compositionLine, compositionX, compositionWidth, textWidth
}

func (a *App) buildContent(snapshot viewSnapshot, width, height, imageScale float32) woxwidget.Widget {
	if len(snapshot.results) == 0 {
		return woxwidget.Container{Width: width, Height: height}
	}
	previewVisible := snapshot.selected >= 0 && snapshot.selected < len(snapshot.results) && launcherPreviewVisible(snapshot.layout, snapshot.results[snapshot.selected].Preview)
	if !previewVisible {
		return a.buildResults(snapshot, width, height, imageScale)
	}
	ratio := launcherPreviewRatio(snapshot.layout, snapshot.chatFullscreen || snapshot.terminalFullscreen)
	if ratio <= 0 {
		result := snapshot.results[snapshot.selected]
		preview := a.buildPreviewSection(result, snapshot, width, height, imageScale)
		if launcherChromeHidden(snapshot.show, snapshot.chatFullscreen) && a.resolvePreview(result.Preview).PreviewType != "chat" && !launcherPreviewTitleBarVisible(snapshot) {
			label := a.translate("i18n:ui_close")
			if strings.TrimSpace(label) == "" || label == "i18n:ui_close" {
				label = "Close"
			}
			return launcherview.PreviewHoverClose(launcherview.PreviewHoverCloseProps{
				Width: width, Height: height, Child: preview, Label: label, Theme: snapshot.palette.componentTheme(), OnTooltip: a.setPreviewTooltip,
				OnClose: a.closePreviewWindow,
			})
		}
		return preview
	}
	if ratio >= 1 {
		return a.buildResults(snapshot, width, height, imageScale)
	}
	splitX := width * ratio
	return launcherview.LauncherSplitContentView(
		a.buildResults(snapshot, splitX, height, imageScale),
		a.buildPreviewSection(snapshot.results[snapshot.selected], snapshot, width-splitX, height, imageScale),
	)
}

func (a *App) buildPreviewSection(result queryResult, snapshot viewSnapshot, width, height, imageScale float32) woxwidget.Widget {
	child := a.buildPreview(result, snapshot.palette, width, height, imageScale)
	resolved := a.resolvePreview(result.Preview)
	// Media owns smaller animation and live-data boundaries; an enclosing section boundary would promote every update to the full preview.
	if resolved.PreviewType == "media" {
		return child
	}
	state := []any{result, resolved, snapshot.palette, snapshot.show, snapshot.chatFullscreen, snapshot.terminalFullscreen, width, height, imageScale, a.translationsRevision.Load(), a.imagesRevision.Load()}
	switch resolved.PreviewType {
	case "query_requirement_settings":
		if a.requirementForm != nil {
			state = append(state, snapshotRequirementFormLocked(a.requirementForm, a.aiSettings.ModelsError()))
		}
	case "trigger_keyword_conflict":
		if a.triggerConflict != nil {
			state = append(state, snapshotTriggerConflictPreviewLocked(a.triggerConflict))
		}
	case "theme_edit":
		if editor := a.themeSettings.ThemeEditor(); editor != nil {
			state = append(state, snapshotThemeEditorPreviewLocked(editor))
		}
	case "chat":
		if a.chatPreview != nil {
			state = append(state, snapshotChatPreviewLocked(a.chatPreview))
		}
	case "terminal":
		state = append(state, snapshotTerminalPreview(a.terminalPreview))
	case "dictation_history":
		if a.dictationAudio != nil {
			state = append(state, a.dictationAudio.revision, a.dictationAudio.path, a.dictationAudio.snapshot)
		}
	case "file":
		filePreview := a.filePreviewFor(resolved.PreviewData)
		state = append(state, filePreview)
		// Native surfaces report initialization failures after paint; include their controller state so the retained section can replace its placeholder.
		if filePreview.Kind == "native_file" {
			state = append(state, a.nativeFilePreviewPath, a.nativeFilePreviewError)
		}
		if filePreview.Kind == "webview" {
			state = append(state, a.webViewPreviewData, a.webViewPreviewError)
		}
	}
	return launcherPreparedSection("launcher-preview-section", "preview", launcherPreparedSectionProps{Signature: launcherSectionSignature(state...), Width: width, Height: height, Child: child})
}

// launcherPreviewVisible mirrors Flutter's grid preview exceptions for system-owned guidance.
func launcherPreviewVisible(layout queryLayout, preview queryPreview) bool {
	if preview.PreviewData == "" {
		return false
	}
	if layout.GridLayout == nil {
		return true
	}
	switch preview.PreviewType {
	case "query_requirement_settings", "trigger_keyword_conflict", "theme_edit", "hotkey_overview":
		return true
	default:
		return false
	}
}

func launcherChromeHidden(show showAppParams, chatFullscreen bool) bool {
	return chatFullscreen || show.HideQueryBox && show.HideToolbar
}

// launcherPreviewRatio keeps chat query layout separate from explicit fullscreen input mode.
func launcherPreviewRatio(layout queryLayout, chatFullscreen bool) float32 {
	ratio := float32(0.4)
	if layout.ResultPreviewWidthRatio != nil && *layout.ResultPreviewWidthRatio >= 0 && *layout.ResultPreviewWidthRatio <= 1 {
		ratio = float32(*layout.ResultPreviewWidthRatio)
	}
	if layout.ChatMode || chatFullscreen {
		return 0
	}
	return ratio
}

func (a *App) buildResults(snapshot viewSnapshot, width, height, imageScale float32) woxwidget.Widget {
	if snapshot.layout.GridLayout != nil {
		return a.buildGridResults(snapshot, width, height, imageScale)
	}
	densityMetrics := snapshot.densityMetrics.normalized()
	rowHeight := densityMetrics.resultRowHeight(snapshot.palette)
	containerPadding := snapshot.palette.resultContainerPadding
	containerPadding.Left += snapshot.palette.appPadding.Left
	containerPadding.Right += snapshot.palette.appPadding.Right
	containerPadding.Bottom += snapshot.palette.appPadding.Bottom
	rowPadding := snapshot.palette.resultItemPadding
	rowPadding.Left += densityMetrics.scaled(5)
	rowPadding.Right += densityMetrics.scaled(5)
	tailLayoutWidth := max(float32(0), width-containerPadding.Left-containerPadding.Right-snapshot.palette.resultItemPadding.Left-snapshot.palette.resultItemPadding.Right)
	contentHeight := containerPadding.Top + containerPadding.Bottom + float32(len(snapshot.results))*rowHeight + float32(max(0, len(snapshot.results)-1)*resultRowGap)
	scroll := resolveResultScroll(snapshot.results, nil, snapshot.selected, width, height, contentHeight, snapshot.resultScroll, snapshot.resultScrollDetached, snapshot.palette, snapshot.densityMetrics)
	a.rememberResolvedResultScroll(snapshot, scroll)
	offset := scroll.offset
	start, end := visibleResultRange(len(snapshot.results), offset, height, containerPadding.Top, rowHeight, resultRowGap)
	items := make([]launcherview.LauncherResultItem, 0, end-start)
	for index := start; index < end; index++ {
		result := snapshot.results[index]
		if result.IsGroup {
			items = append(items, launcherview.LauncherResultItem{
				ID: result.ID, Title: result.Title, Group: true, Selected: index == snapshot.selected, Hovered: index == snapshot.hoveredResult,
			})
			continue
		}
		tails, tailWidth, tailHeight := a.resultTailViewProps(result.Tails, tailLayoutWidth, densityMetrics, imageScale)
		titleHeight := float32(0)
		if result.SubTitle == "" {
			metrics, _ := a.window.MeasureText(result.Title, woxui.TextStyle{Size: densityMetrics.scaled(woxcomponent.ResultTitleFontSize)})
			titleHeight = metrics.Size.Height
		}
		items = append(items, launcherview.LauncherResultItem{
			ID: result.ID, Title: result.Title, Subtitle: result.SubTitle, Selected: index == snapshot.selected, Hovered: index == snapshot.hoveredResult,
			Icon: a.imageForSize(result.Icon, physicalImageSize(int(densityMetrics.scaled(32)), imageScale)), TitleHeight: titleHeight, Tails: tails, TailWidth: tailWidth, TailHeight: tailHeight,
			OnHover: func(inside bool) { a.hoverResult(index, inside) }, OnSelect: func() { a.selectResult(index) }, OnActivate: func() { a.activateResult(index) },
		})
	}
	return launcherview.LauncherResultsView(launcherview.LauncherResultsProps{
		Width: width, Height: height, ContentHeight: contentHeight, Offset: offset, StartIndex: start, RowHeight: rowHeight, RowGap: resultRowGap,
		ContainerPadding: containerPadding, ItemPadding: rowPadding, ItemRadius: snapshot.palette.resultItemRadius,
		TailColor: snapshot.palette.resultTail, SelectedTailColor: snapshot.palette.selectedTail, Theme: snapshot.palette.componentTheme(), DensityScale: densityMetrics.scale, Items: items,
		Complete: snapshot.queryComplete, ScrollDetached: snapshot.resultScrollDetached,
		OnScroll: func(delta float32) { a.scrollResultsFrom(snapshot.resultScrollDetached, scroll, delta) },
	})
}

// resultTailViewProps resolves tail images and bounds their measured widths before rendering.
func (a *App) resultTailViewProps(tails []resultTail, rowWidth float32, densityMetrics launcherDensityMetrics, imageScale float32) ([]launcherview.LauncherResultTail, float32, float32) {
	tailOuterPadding := densityMetrics.scaled(15)
	tailItemPadding := densityMetrics.scaled(10)
	textPadding := densityMetrics.scaled(16)
	textHeight := densityMetrics.scaled(22)
	defaultImageSize := densityMetrics.scaled(20)
	style := woxui.TextStyle{Size: densityMetrics.scaled(woxcomponent.TailFontSize)}
	// Flutter's one-third cap includes the 10 px leading and 5 px trailing tail padding; the row owns those gaps in Go UI, so only the inner tail width is reserved here.
	maximum := max(float32(0), rowWidth/3-tailOuterPadding)
	maximumTextWidth := max(float32(0), maximum-tailItemPadding)
	items := make([]launcherview.LauncherResultTail, 0, len(tails))
	used := float32(0)
	height := float32(0)
	for _, tail := range tails {
		item := launcherview.LauncherResultTail{Text: tail.Text, TextCategory: tail.TextCategory}
		switch tail.Type {
		case "text":
			if maximumTextWidth <= 0 {
				continue
			}
			metrics, _ := a.window.MeasureText(tail.Text, style)
			item.Width = min(maximumTextWidth, metrics.Size.Width+textPadding)
			item.Height = textHeight
			layout := woxwidget.LayoutTextBlock(a.window, tail.Text, style, max(float32(0), item.Width-textPadding), 1, 0)
			if len(layout.Lines) > 0 {
				item.Text = layout.Lines[0]
			}
		case "image":
			item.Width = defaultImageSize
			item.Height = defaultImageSize
			if tail.ImageWidth != nil && *tail.ImageWidth > 0 {
				item.Width = float32(*tail.ImageWidth)
			}
			if tail.ImageHeight != nil && *tail.ImageHeight > 0 {
				item.Height = float32(*tail.ImageHeight)
			}
			item.Image = a.imageForDimensions(
				tail.Image,
				physicalImageSize(int(math.Ceil(float64(item.Width))), imageScale),
				physicalImageSize(int(math.Ceil(float64(item.Height))), imageScale),
			)
			if item.Image == nil {
				continue
			}
			if text, ok := centeredSVGText(tail.Image, item.Width, item.Height); ok {
				item.ImageText = text.Value
				item.ImageTextColor = text.Color
				item.ImageTextSize = text.Size
			}
		default:
			continue
		}
		used += tailItemPadding + item.Width
		height = max(height, item.Height)
		items = append(items, item)
	}
	return items, min(maximum, used), height
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

// resolveResultScroll follows keyboard selection until pointer scrolling takes ownership of the viewport.
func resolveResultScroll(results []queryResult, layout *gridLayout, selected int, width, viewport, content float32, current scrollController, detached bool, palette uiPalette, densityMetrics launcherDensityMetrics) scrollController {
	scroll := current.withGeometry(viewport, content)
	if detached || selected < 0 || selected >= len(results) || viewport <= 0 || content <= viewport {
		return scroll
	}
	rowHeight := densityMetrics.normalized().resultRowHeight(palette)
	top := palette.resultContainerPadding.Top + float32(selected)*(rowHeight+resultRowGap)
	bottom := top + rowHeight
	if layout != nil {
		top, bottom = gridResultVerticalBounds(results, selected, width, layout)
	} else {
		for index := selected - 1; index >= 0; index-- {
			if results[index].IsGroup {
				if selected-index <= 2 {
					top = palette.resultContainerPadding.Top + float32(index)*(rowHeight+resultRowGap)
				}
				break
			}
		}
	}
	scroll.ensureVisible(top, bottom)
	return scroll
}

// rememberResolvedResultScroll makes consecutive key moves start from the viewport that was actually rendered.
func (a *App) rememberResolvedResultScroll(snapshot viewSnapshot, scroll scrollController) {
	if scroll == snapshot.resultScroll {
		return
	}
	if a.resultsQueryID == snapshot.resultsQueryID && a.selected == snapshot.selected && a.resultScroll == snapshot.resultScroll && a.resultScrollDetached == snapshot.resultScrollDetached {
		a.resultScroll = scroll
	}
}

// scrollResultsFrom detaches pointer scrolling from selection-following until selection changes.
func (a *App) scrollResultsFrom(snapshotDetached bool, rendered scrollController, delta float32) {
	base := a.resultScroll
	if !snapshotDetached && !a.resultScrollDetached {
		base = rendered
	}
	base.scrollBy(delta)
	a.resultScroll = base
	a.resultScrollDetached = true
}

func (a *App) buildFooter(snapshot viewSnapshot, width, height, imageScale float32) woxwidget.Widget {
	leftLabel := ""
	var leftIcon *woxui.Image
	progress := 0
	hasProgress := false
	indeterminate := false
	if snapshot.toolbarMsg != nil {
		leftLabel = snapshot.toolbarMsg.displayText()
		if image := a.imageForSize(snapshot.toolbarMsg.Icon, physicalImageSize(18, imageScale)); image != nil {
			leftIcon = image
		}
		if snapshot.toolbarMsg.Progress != nil {
			progress = *snapshot.toolbarMsg.Progress
			hasProgress = true
		} else if snapshot.toolbarMsg.Indeterminate {
			indeterminate = true
		}
	}
	actions := make([]launcherview.LauncherToolbarAction, 0)
	entries := unifiedActionPanelEntries(snapshot.results, snapshot.selected, snapshot.toolbarMsg)
	for _, source := range []actionPanelSource{actionPanelSourceResult, actionPanelSourceToolbar} {
		for _, entry := range entries {
			if entry.Source != source || strings.TrimSpace(entry.Hotkey) == "" {
				continue
			}
			actions = append(actions, launcherview.LauncherToolbarAction{
				ID: "toolbar-action-" + entry.ID, Label: a.translate(entry.Name), HotkeyLabels: formatHotkeyLabels(entry.Hotkey), OnTap: func() {
					if entry.Source == actionPanelSourceToolbar {
						a.activateToolbarActionForMessage(entry.ToolbarMessageID, entry.ToolbarMessageAction)
						return
					}
					a.activateAction(entry.ResultIndex, entry.ActionIndex)
				},
			})
		}
	}
	if len(entries) > 0 {
		actions = append(actions, launcherview.LauncherToolbarAction{
			ID: "result-toolbar-more", Label: a.translate("i18n:toolbar_more_actions"), HotkeyLabels: formatHotkeyLabels(primaryHotkey("j")), OnTap: a.toggleActionPanel,
		})
	}
	return launcherview.LauncherToolbarBoundary(launcherview.LauncherToolbarProps{
		Width: width, Height: height, Padding: snapshot.palette.toolbarPadding, Theme: snapshot.palette.componentTheme(), Window: a.window, DensityScale: snapshot.densityMetrics.scale,
		Label: leftLabel, Icon: leftIcon, Progress: progress, HasProgress: hasProgress, Indeterminate: indeterminate, Actions: actions, OnDragStart: func() {
			if err := a.window.StartDragging(); err != nil {
				util.GetLogger().Error(util.NewTraceContext(), fmt.Sprintf("start launcher toolbar window drag: %v", err))
			}
		},
	})
}
