package launcher

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"strings"
	"unicode/utf8"

	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
	"wox/util/clipboard"
)

const (
	maxTerminalPreviewBytes = 2 * 1024 * 1024
	terminalHistoryBytes    = 64 * 1024
)

type terminalPreviewData struct {
	SessionID string `json:"session_id"`
	Command   string `json:"command"`
	Status    string `json:"status"`
}

type terminalChunk struct {
	SessionID   string `json:"SessionId"`
	CursorStart int64  `json:"CursorStart"`
	CursorEnd   int64  `json:"CursorEnd"`
	Content     string `json:"Content"`
	Truncated   bool   `json:"Truncated"`
}

type terminalSessionState struct {
	SessionID        string `json:"SessionId"`
	Command          string `json:"Command"`
	Interpreter      string `json:"Interpreter"`
	WorkingDirectory string `json:"WorkingDirectory"`
	Status           string `json:"Status"`
	ExitCode         int    `json:"ExitCode"`
	Error            string `json:"Error"`
}

type terminalPreviewState struct {
	SessionID           string
	Command             string
	Status              string
	Error               string
	Text                string
	BaseCursor          int64
	CurrentCursor       int64
	Scroll              float32
	AutoFollow          bool
	MaxScroll           float32
	LoadingHistory      bool
	LastHistoryCursor   int64
	HistoryAnchorBase   int64
	HistoryAnchorScroll float32
	SearchOpen          bool
	SearchEditor        *woxui.TextEditor
	CaseSensitive       bool
	Matches             []terminalMatch
	MatchIndex          int
	SelectionAnchor     int
	SelectionFocus      int
}

type terminalPreviewSnapshot struct {
	SessionID       string
	Command         string
	Status          string
	Error           string
	Text            string
	Scroll          float32
	LoadingHistory  bool
	SearchOpen      bool
	SearchEditing   woxui.TextEditingState
	CaseSensitive   bool
	MatchCount      int
	MatchIndex      int
	Matches         []terminalMatch
	SelectionAnchor int
	SelectionFocus  int
}

type terminalMatch struct {
	start int
	end   int
}

// buildTerminalPreview delegates presentation to the pure view while retaining controller-owned text layout caching.
func (a *App) buildTerminalPreview(snapshot terminalPreviewSnapshot, palette uiPalette, width, height, imageScale float32, tags []previewview.PreviewTag) woxwidget.Widget {
	font := ""
	if a.generalSettings != nil {
		font = a.generalSettings.Data().AppFontFamily
	}
	matches := make([]previewview.TerminalMatch, len(snapshot.Matches))
	for index, match := range snapshot.Matches {
		matches[index] = previewview.TerminalMatch{Start: match.start, End: match.end}
	}
	return previewview.TerminalPreviewView(previewview.TerminalPreviewProps{
		Width: width, Height: height, Theme: palette.componentTheme(), Window: a.window,
		SessionID: snapshot.SessionID, Command: snapshot.Command, Status: snapshot.Status, Error: snapshot.Error, Text: snapshot.Text,
		Scroll: snapshot.Scroll, LoadingHistory: snapshot.LoadingHistory, SearchOpen: snapshot.SearchOpen, SearchEditing: snapshot.SearchEditing,
		CaseSensitive: snapshot.CaseSensitive, MatchCount: snapshot.MatchCount, MatchIndex: snapshot.MatchIndex, Matches: matches,
		Fullscreen: a.terminalFullscreen, SearchHotkey: strings.Join(formatHotkeyLabels(primaryHotkey("shift+f")), "+"),
		FullscreenHotkey: strings.Join(formatHotkeyLabels(primaryHotkey("b")), "+"), Tags: tags,
		LayoutText: func(value string, style woxui.TextStyle, textWidth, lineHeight float32) woxwidget.TextBlockLayout {
			return a.terminalLayout.measure(value, textLayoutKey{
				session: snapshot.SessionID, font: font, window: a.window, width: textWidth,
				scale: imageScale, lineHeight: lineHeight, style: style,
			})
		},
		OnClampScroll: a.clampTerminalPreviewScroll, OnScroll: a.scrollTerminalPreview, OnOpenSearch: a.openTerminalSearch,
		OnSetSearch: a.setTerminalSearchQuery, OnSearchChanged: func(value string) { _ = a.setTerminalSearchQuery(value) },
		OnSearchKey:  a.onTerminalPreviewKey,
		OnMoveSearch: a.moveTerminalSearch, OnToggleSearchCase: a.toggleTerminalSearchCase, OnCloseSearch: a.closeTerminalSearch,
		OnToggleFullscreen: a.toggleTerminalFullscreen, OnTagHover: a.setPreviewTooltip,
		SelectionAnchor: snapshot.SelectionAnchor, SelectionFocus: snapshot.SelectionFocus,
		OnSelectionStart: a.startTerminalSelection, OnSelectionExtend: a.extendTerminalSelection,
		OnSelectWord: a.selectTerminalWord, OnSelectLine: a.selectTerminalLine,
		OnCopy: a.copyTerminalPreviewOutput, OnSelectAll: a.selectAllTerminalPreview,
		OnContextMenu: a.openTerminalContextMenu,
	})
}

func decodeTerminalPreviewData(value string) terminalPreviewData {
	var data terminalPreviewData
	if json.Unmarshal([]byte(value), &data) == nil {
		return data
	}
	return terminalPreviewData{SessionID: strings.TrimSpace(value)}
}

// activateTerminalPreview switches the single visible terminal subscription before rendering.
func (a *App) activateTerminalPreview(preview queryPreview) {
	data := decodeTerminalPreviewData(preview.PreviewData)
	state := a.terminalPreview
	oldSessionID := ""
	sessionChanged := state == nil || state.SessionID != data.SessionID
	if sessionChanged {
		if state != nil {
			oldSessionID = state.SessionID
		}
		state = &terminalPreviewState{SessionID: data.SessionID, Command: data.Command, Status: data.Status, AutoFollow: true, Scroll: float32(math.MaxFloat32), LastHistoryCursor: -1, MatchIndex: -1}
		a.terminalPreview = state
	} else {
		if data.Command != "" {
			state.Command = data.Command
		}
		if data.Status != "" {
			state.Status = data.Status
		}
	}
	newSessionID := state.SessionID
	if sessionChanged && (oldSessionID != "" || newSessionID != "") {
		a.scheduleTerminalSubscription(newSessionID)
	}
}

// terminalPreviewSnapshotFor returns the prepared terminal state for rendering.
func (a *App) terminalPreviewSnapshotFor(preview queryPreview) terminalPreviewSnapshot {
	data := decodeTerminalPreviewData(preview.PreviewData)
	if a.terminalPreview == nil || a.terminalPreview.SessionID != data.SessionID {
		return terminalPreviewSnapshot{SessionID: data.SessionID, Command: data.Command, Status: data.Status}
	}
	return snapshotTerminalPreview(a.terminalPreview)
}

func snapshotTerminalPreview(state *terminalPreviewState) terminalPreviewSnapshot {
	if state == nil {
		return terminalPreviewSnapshot{}
	}
	snapshot := terminalPreviewSnapshot{
		SessionID: state.SessionID, Command: state.Command, Status: state.Status, Error: state.Error, Text: state.Text, Scroll: state.Scroll,
		LoadingHistory: state.LoadingHistory, SearchOpen: state.SearchOpen, CaseSensitive: state.CaseSensitive, MatchCount: len(state.Matches), MatchIndex: state.MatchIndex,
		Matches: append([]terminalMatch(nil), state.Matches...), SelectionAnchor: state.SelectionAnchor, SelectionFocus: state.SelectionFocus,
	}
	if state.SearchEditor != nil {
		snapshot.SearchEditing = state.SearchEditor.State()
	}
	return snapshot
}

// reconcileTerminalSubscription serializes transport writes and converges on the latest visible session.
func (a *App) reconcileTerminalSubscription() {
	a.terminalSubscriptionMu.Lock()
	defer a.terminalSubscriptionMu.Unlock()

	desiredSessionID, _ := a.terminalDesired.Load().(string)
	if a.terminalSubscribed == desiredSessionID {
		return
	}
	if a.terminalSubscribed != "" {
		if err := a.services.UnsubscribeTerminal(context.Background(), a.sessionID, a.terminalSubscribed); err != nil {
			log.Printf("unsubscribe terminal session: %v", err)
			return
		}
		a.terminalSubscribed = ""
	}
	// The desired session may change while the unsubscribe write is in flight.
	desiredSessionID, _ = a.terminalDesired.Load().(string)
	if desiredSessionID == "" {
		return
	}
	if _, err := a.services.SubscribeTerminal(context.Background(), a.sessionID, desiredSessionID, -1); err != nil {
		log.Printf("subscribe terminal session: %v", err)
		return
	}
	a.terminalSubscribed = desiredSessionID
}

// scheduleTerminalSubscription publishes the latest desired session without blocking the UI thread on transport I/O.
func (a *App) scheduleTerminalSubscription(sessionID string) {
	a.terminalDesired.Store(sessionID)
	util.Go(a.lifecycleCtx, "reconcile terminal subscription", a.reconcileTerminalSubscription)
}

// deactivateTerminalPreview releases core output when the selected preview no longer uses it.
func (a *App) deactivateTerminalPreview() {
	a.terminalLayout = textLayoutCache{}
	oldSessionID := ""
	searchWasOpen := false
	if a.terminalPreview != nil {
		oldSessionID = a.terminalPreview.SessionID
		searchWasOpen = a.terminalPreview.SearchOpen
		a.closeTerminalContextMenu()
		a.terminalPreview = nil
	}
	a.terminalFullscreen = false
	if oldSessionID != "" {
		a.scheduleTerminalSubscription("")
	}
	if searchWasOpen {
		a.restoreQueryTextInput()
	}
}

// applyTerminalChunk merges byte-cursor updates so UTF-8 output follows the core ring buffer exactly.
func (a *App) applyTerminalChunk(chunk terminalChunk) {
	if chunk.SessionID == "" || chunk.Content == "" {
		return
	}
	state := a.terminalPreview
	if state == nil || state.SessionID != chunk.SessionID {
		return
	}
	oldBase := state.BaseCursor
	if state.Text == "" || chunk.Truncated || chunk.CursorStart < state.BaseCursor {
		state.BaseCursor = chunk.CursorStart
		state.Text = chunk.Content
	} else {
		offset := chunk.CursorStart - state.BaseCursor
		switch {
		case offset >= int64(len(state.Text)):
			state.Text += chunk.Content
		case offset >= 0:
			overwriteEnd := min(int(offset)+len(chunk.Content), len(state.Text))
			state.Text = state.Text[:int(offset)] + chunk.Content + state.Text[overwriteEnd:]
		default:
			state.BaseCursor = chunk.CursorStart
			state.Text = chunk.Content
		}
	}
	state.CurrentCursor = max(state.CurrentCursor, chunk.CursorEnd)
	if len(state.Text) > maxTerminalPreviewBytes {
		trim := len(state.Text) - maxTerminalPreviewBytes
		for trim < len(state.Text) && !utf8.RuneStart(state.Text[trim]) {
			trim++
		}
		state.Text = state.Text[trim:]
		state.BaseCursor += int64(trim)
	}
	remapTerminalSelection(state, oldBase)
	if state.HistoryAnchorBase > 0 && state.BaseCursor <= state.HistoryAnchorBase {
		prefixBytes := state.HistoryAnchorBase - state.BaseCursor
		if prefixBytes >= 0 && prefixBytes <= int64(len(state.Text)) {
			state.Scroll = state.HistoryAnchorScroll + float32(strings.Count(state.Text[:prefixBytes], "\n"))*18
			state.AutoFollow = false
			state.HistoryAnchorBase = 0
			state.LoadingHistory = false
		}
	}
	if state.SearchOpen {
		rebuildTerminalMatches(state, true)
	}
	if state.AutoFollow {
		state.Scroll = float32(math.MaxFloat32)
	}
	_ = a.window.Invalidate()
}

func (a *App) applyTerminalState(update terminalSessionState) {
	if update.SessionID == "" {
		return
	}
	if state := a.terminalPreview; state != nil && state.SessionID == update.SessionID {
		if update.Command != "" {
			state.Command = update.Command
		}
		state.Status = update.Status
		state.Error = update.Error
	}
	_ = a.window.Invalidate()
}

func (a *App) clampTerminalPreviewScroll(maxOffset float32) {
	if state := a.terminalPreview; state != nil {
		state.MaxScroll = max(float32(0), maxOffset)
		state.Scroll = min(max(float32(0), state.Scroll), maxOffset)
		state.AutoFollow = maxOffset-state.Scroll <= 24
	}
}

func (a *App) scrollTerminalPreview(delta, maxOffset float32) {
	if delta == 0 {
		return
	}
	requestSession := ""
	requestCursor := int64(-1)
	if state := a.terminalPreview; state != nil {
		state.MaxScroll = max(float32(0), maxOffset)
		state.Scroll = min(max(float32(0), state.Scroll+delta), maxOffset)
		state.AutoFollow = maxOffset-state.Scroll <= 24
		if state.Scroll <= 16 && !state.LoadingHistory && state.BaseCursor > 0 && len(state.Text) < maxTerminalPreviewBytes {
			target := max(int64(0), state.BaseCursor-terminalHistoryBytes)
			if target != state.LastHistoryCursor {
				state.LoadingHistory = true
				state.LastHistoryCursor = target
				state.HistoryAnchorBase = state.BaseCursor
				state.HistoryAnchorScroll = state.Scroll
				requestSession = state.SessionID
				requestCursor = target
			}
		}
	}
	_ = a.window.Invalidate()
	if requestSession != "" {
		util.Go(a.lifecycleCtx, "request terminal history", func() {
			a.requestTerminalHistory(requestSession, requestCursor)
		})
	}
}

// requestTerminalHistory resets the existing core subscription to an earlier byte cursor without racing selection changes.
func (a *App) requestTerminalHistory(sessionID string, cursor int64) {
	a.terminalSubscriptionMu.Lock()
	var err error
	sent := false
	if a.terminalSubscribed == sessionID {
		sent = true
		_, err = a.services.SubscribeTerminal(context.Background(), a.sessionID, sessionID, cursor)
	}
	a.terminalSubscriptionMu.Unlock()
	if dispatchErr := a.runOnUI("apply terminal history request", func() {
		if state := a.terminalPreview; state != nil && state.SessionID == sessionID && state.LastHistoryCursor == cursor {
			state.LoadingHistory = false
			if !sent || err != nil {
				if err != nil {
					state.Error = err.Error()
				}
				state.HistoryAnchorBase = 0
			}
		}
		_ = a.window.Invalidate()
	}); dispatchErr != nil {
		log.Printf("dispatch terminal history result: %v", dispatchErr)
	}
	if err != nil {
		log.Printf("load terminal history: %v", err)
	}
}

// rebuildTerminalMatches indexes the loaded UTF-8 window and optionally preserves the absolute current hit.
func rebuildTerminalMatches(state *terminalPreviewState, preserveCurrent bool) {
	if state == nil || state.SearchEditor == nil {
		return
	}
	keyword := strings.TrimSpace(state.SearchEditor.State().Text)
	if keyword == "" || state.Text == "" {
		state.Matches = nil
		state.MatchIndex = -1
		return
	}
	previousStart := int64(-1)
	if preserveCurrent && state.MatchIndex >= 0 && state.MatchIndex < len(state.Matches) {
		previousStart = state.BaseCursor + int64(state.Matches[state.MatchIndex].start)
	}
	source := state.Text
	query := keyword
	if !state.CaseSensitive {
		source = strings.ToLower(source)
		query = strings.ToLower(query)
	}
	matches := make([]terminalMatch, 0)
	for from := 0; from <= len(source); {
		index := strings.Index(source[from:], query)
		if index < 0 {
			break
		}
		start := from + index
		matches = append(matches, terminalMatch{start: start, end: start + len(query)})
		from = start + len(query)
	}
	state.Matches = matches
	if len(matches) == 0 {
		state.MatchIndex = -1
		return
	}
	state.MatchIndex = 0
	if previousStart >= state.BaseCursor {
		for index, match := range matches {
			absolute := state.BaseCursor + int64(match.start)
			if absolute >= previousStart {
				state.MatchIndex = index
				break
			}
		}
	}
}

// openTerminalSearch transfers keyboard and IME ownership from the query box to the preview-local editor.
func (a *App) openTerminalSearch() {
	state := a.terminalPreview
	if state == nil {
		return
	}
	state.SearchOpen = true
	if state.SearchEditor == nil {
		state.SearchEditor = woxui.NewTextEditor("")
	}
	rebuildTerminalMatches(state, false)
	if a.host != nil {
		a.host.RequestFocus(previewview.TerminalSearchInputKey(state.SessionID))
	}
	a.updateFormTextInput(true)
	_ = a.window.Invalidate()
}

// closeTerminalSearch returns text input ownership to the launcher query.
func (a *App) closeTerminalSearch() {
	if state := a.terminalPreview; state != nil {
		state.SearchOpen = false
		state.Matches = nil
		state.MatchIndex = -1
	}
	a.restoreQueryTextInput()
	_ = a.window.Invalidate()
}

// setTerminalSearchQuery replaces the local find value for accessibility set-value actions.
func (a *App) setTerminalSearchQuery(value string) error {
	if state := a.terminalPreview; state != nil && state.SearchOpen && state.SearchEditor != nil {
		state.SearchEditor.SetText(value, false)
		rebuildTerminalMatches(state, false)
	}
	_ = a.window.Invalidate()
	return nil
}

// moveTerminalSearch advances through loaded matches and scrolls to an approximate text-layout position.
func (a *App) moveTerminalSearch(delta int) {
	state := a.terminalPreview
	if state == nil || !state.SearchOpen || state.SearchEditor == nil {
		return
	}
	rebuildTerminalMatches(state, true)
	if len(state.Matches) > 0 {
		if state.MatchIndex < 0 || state.MatchIndex >= len(state.Matches) {
			state.MatchIndex = 0
		} else {
			state.MatchIndex = (state.MatchIndex + delta + len(state.Matches)) % len(state.Matches)
		}
		match := state.Matches[state.MatchIndex]
		prefixEnd := min(max(0, match.start), len(state.Text))
		line := strings.Count(state.Text[:prefixEnd], "\n")
		totalLines := max(1, strings.Count(state.Text, "\n")+1)
		ratio := float32(line) / float32(totalLines)
		state.Scroll = min(max(float32(0), ratio*state.MaxScroll), state.MaxScroll)
		state.AutoFollow = false
	}
	_ = a.window.Invalidate()
}

// toggleTerminalSearchCase rebuilds the loaded-window index without a core round trip.
func (a *App) toggleTerminalSearchCase() {
	if state := a.terminalPreview; state != nil && state.SearchOpen {
		state.CaseSensitive = !state.CaseSensitive
		rebuildTerminalMatches(state, false)
	}
	_ = a.window.Invalidate()
}

// remapTerminalSelection keeps an existing highlight attached to the same absolute output bytes.
func remapTerminalSelection(state *terminalPreviewState, oldBase int64) {
	if state == nil || state.SelectionAnchor == state.SelectionFocus {
		return
	}
	// Preserve the drag anchor even when the selection runs backwards.
	anchor := int(oldBase + int64(state.SelectionAnchor) - state.BaseCursor)
	focus := int(oldBase + int64(state.SelectionFocus) - state.BaseCursor)
	if max(anchor, focus) <= 0 || min(anchor, focus) >= len(state.Text) {
		state.SelectionAnchor, state.SelectionFocus = 0, 0
		return
	}
	state.SelectionAnchor = previewview.TerminalClampUTF8Offset(state.Text, anchor)
	state.SelectionFocus = previewview.TerminalClampUTF8Offset(state.Text, focus)
}

// terminalSelectedText returns the currently highlighted output, or empty when collapsed.
func terminalSelectedText(state *terminalPreviewState) string {
	if state == nil {
		return ""
	}
	start, end := min(state.SelectionAnchor, state.SelectionFocus), max(state.SelectionAnchor, state.SelectionFocus)
	start = previewview.TerminalClampUTF8Offset(state.Text, start)
	end = previewview.TerminalClampUTF8Offset(state.Text, end)
	if start >= end || end > len(state.Text) {
		return ""
	}
	return state.Text[start:end]
}

// focusQueryForTerminalSelection returns IME ownership to the query so Ctrl+C copies output, not find text.
func (a *App) focusQueryForTerminalSelection() {
	if a.host == nil || !a.queryCanFocus() {
		return
	}
	if a.terminalSearchFocused() {
		a.host.RequestFocus(launcherview.LauncherQueryInputKey)
		a.restoreQueryTextInput()
	}
}

// startTerminalSelection begins a drag or shift-click highlight at the given output byte offset.
func (a *App) startTerminalSelection(offset int, modifiers woxui.KeyModifiers) {
	state := a.terminalPreview
	if state == nil || strings.TrimSpace(state.Text) == "" {
		return
	}
	a.focusQueryForTerminalSelection()
	offset = previewview.TerminalClampUTF8Offset(state.Text, offset)
	if modifiers&woxui.KeyModifierShift != 0 {
		state.SelectionFocus = offset
	} else {
		state.SelectionAnchor = offset
		state.SelectionFocus = offset
	}
	a.refreshTerminalContextMenuIfStale()
	_ = a.window.Invalidate()
}

// extendTerminalSelection updates the highlight focus while the pointer is dragged.
func (a *App) extendTerminalSelection(offset int) {
	state := a.terminalPreview
	if state == nil || strings.TrimSpace(state.Text) == "" {
		return
	}
	state.SelectionFocus = previewview.TerminalClampUTF8Offset(state.Text, offset)
	a.refreshTerminalContextMenuIfStale()
	_ = a.window.Invalidate()
}

// selectTerminalWord selects the word under a double-click.
func (a *App) selectTerminalWord(offset int) {
	state := a.terminalPreview
	if state == nil || strings.TrimSpace(state.Text) == "" {
		return
	}
	a.focusQueryForTerminalSelection()
	start, end := previewview.TerminalWordByteRange(state.Text, offset)
	state.SelectionAnchor, state.SelectionFocus = start, end
	a.refreshTerminalContextMenuIfStale()
	_ = a.window.Invalidate()
}

// selectTerminalLine selects the newline-delimited line under a triple-click.
func (a *App) selectTerminalLine(offset int) {
	state := a.terminalPreview
	if state == nil || strings.TrimSpace(state.Text) == "" {
		return
	}
	a.focusQueryForTerminalSelection()
	start, end := previewview.TerminalLineByteRange(state.Text, offset)
	state.SelectionAnchor, state.SelectionFocus = start, end
	a.refreshTerminalContextMenuIfStale()
	_ = a.window.Invalidate()
}

// selectAllTerminalPreview highlights the complete loaded output window.
func (a *App) selectAllTerminalPreview() {
	state := a.terminalPreview
	if state == nil || state.Text == "" {
		return
	}
	state.SelectionAnchor, state.SelectionFocus = 0, len(state.Text)
	a.refreshTerminalContextMenuIfStale()
	_ = a.window.Invalidate()
}

// copyTerminalPreviewOutput copies the current highlight, or the full output when none is selected.
func (a *App) copyTerminalPreviewOutput() {
	state := a.terminalPreview
	if state == nil {
		return
	}
	text := terminalSelectedText(state)
	if text == "" {
		text = state.Text
	}
	if text == "" {
		return
	}
	_ = clipboard.WriteText(text)
}

// copyTerminalPreviewSelection copies a non-empty highlight and reports whether Ctrl+C should be consumed.
func (a *App) copyTerminalPreviewSelection() bool {
	text := terminalSelectedText(a.terminalPreview)
	if text == "" {
		return false
	}
	_ = clipboard.WriteText(text)
	return true
}

type terminalMenuEnablement struct {
	canCopy, canSelectAll bool
}

func computeTerminalMenuEnablement(state *terminalPreviewState) terminalMenuEnablement {
	if state == nil {
		return terminalMenuEnablement{}
	}
	return terminalMenuEnablement{canCopy: terminalSelectedText(state) != "", canSelectAll: state.Text != ""}
}

// openTerminalContextMenu shows Copy and Select All above the terminal output.
func (a *App) openTerminalContextMenu(windowPos woxui.Point) {
	if a == nil || a.host == nil {
		return
	}
	state := a.terminalPreview
	if state == nil || strings.TrimSpace(state.Text) == "" {
		return
	}
	a.focusQueryForTerminalSelection()
	en := computeTerminalMenuEnablement(state)
	theme := a.palette.componentTheme()
	owner := previewview.TerminalOutputKey(state.SessionID)
	var token uint64
	clear := func() {
		a.host.ClearOverlay(owner, token)
	}
	menu := woxcomponent.BuildTextEditContextMenu(woxcomponent.TextEditContextMenuProps{
		ID: "launcher.preview.terminal.menu", Theme: theme.Controls,
		CanCopy: en.canCopy, CanSelectAll: en.canSelectAll,
		OnAction: func(action woxcomponent.TextEditContextAction) {
			clear()
			live := computeTerminalMenuEnablement(a.terminalPreview)
			switch action {
			case woxcomponent.TextEditContextCopy:
				if live.canCopy {
					a.copyTerminalPreviewOutput()
				}
			case woxcomponent.TextEditContextSelectAll:
				if live.canSelectAll {
					a.selectAllTerminalPreview()
				}
			}
		},
	})
	token = a.host.SetOverlay(owner, woxcomponent.PlaceTextEditContextMenu(a.host.FrameSize(), windowPos, string(owner), menu, clear))
	a.terminalMenuAnchor = windowPos
	a.terminalMenuEnablement = en
}

// refreshTerminalContextMenuIfStale rebuilds an open menu when copy/select-all enablement changed.
func (a *App) refreshTerminalContextMenuIfStale() {
	state := a.terminalPreview
	if a == nil || a.host == nil || state == nil || a.host.OverlayOwner() != previewview.TerminalOutputKey(state.SessionID) {
		return
	}
	next := computeTerminalMenuEnablement(state)
	if next == a.terminalMenuEnablement {
		return
	}
	a.openTerminalContextMenu(a.terminalMenuAnchor)
}

// clearTerminalSelection drops an output highlight so later query copy shortcuts stay with the editor.
func (a *App) clearTerminalSelection() {
	state := a.terminalPreview
	if state == nil || state.SelectionAnchor == state.SelectionFocus {
		return
	}
	state.SelectionAnchor, state.SelectionFocus = 0, 0
	a.refreshTerminalContextMenuIfStale()
	if a.window != nil {
		_ = a.window.Invalidate()
	}
}

// closeTerminalContextMenu dismisses the output context menu for the visible session.
func (a *App) closeTerminalContextMenu() {
	if a == nil || a.host == nil || a.terminalPreview == nil {
		return
	}
	a.host.ClearOverlay(previewview.TerminalOutputKey(a.terminalPreview.SessionID), 0)
}

// onTerminalPreviewKey handles preview-local find and output copy before launcher navigation sees the keystroke.
func (a *App) onTerminalPreviewKey(event woxui.KeyEvent) bool {
	if !event.Down || event.Composing {
		return false
	}
	state := a.terminalPreview
	if state == nil {
		return false
	}
	if event.Key == woxui.KeyEscape && a.host != nil && a.host.HasOverlay() && a.host.OverlayOwner() == previewview.TerminalOutputKey(state.SessionID) {
		a.closeTerminalContextMenu()
		return true
	}
	if event.Modifiers.HasPrimary() && event.Key == woxui.Key("c") && !a.terminalSearchFocused() && a.copyTerminalPreviewSelection() {
		return true
	}
	if hotkeyMatches(primaryHotkey("shift+f"), event) {
		a.openTerminalSearch()
		return true
	}
	if hotkeyMatches(primaryHotkey("b"), event) {
		a.toggleTerminalFullscreen()
		return true
	}
	if !state.SearchOpen || !a.terminalSearchFocused() {
		return false
	}
	if event.Key == woxui.KeyEscape {
		a.closeTerminalSearch()
		return true
	}
	if event.Key == woxui.KeyEnter {
		delta := 1
		if event.Modifiers&woxui.KeyModifierShift != 0 {
			delta = -1
		}
		a.moveTerminalSearch(delta)
		return true
	}
	return false
}

// onTerminalPreviewTextInput commits native IME input only while terminal find owns focus.
func (a *App) onTerminalPreviewTextInput(_ woxui.TextInputEvent) bool {
	state := a.terminalPreview
	if state == nil || !state.SearchOpen || state.SearchEditor == nil {
		return false
	}
	return a.terminalSearchFocused()
}

func (a *App) terminalSearchFocused() bool {
	return a.host != nil && a.terminalPreview != nil && a.host.HasFocus(previewview.TerminalSearchInputKey(a.terminalPreview.SessionID))
}

// toggleTerminalFullscreen switches the terminal preview between split and preview-only layout.
func (a *App) toggleTerminalFullscreen() {
	if a.terminalPreview == nil {
		return
	}
	a.terminalFullscreen = !a.terminalFullscreen
	_ = a.applyWindowBounds()
	_ = a.window.Invalidate()
}
