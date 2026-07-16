package launcher

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"strings"
	"unicode/utf8"

	woxui "wox/ui/runtime"
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
}

type terminalPreviewSnapshot struct {
	SessionID      string
	Command        string
	Status         string
	Error          string
	Text           string
	Scroll         float32
	LoadingHistory bool
	SearchOpen     bool
	SearchEditing  woxui.TextEditingState
	CaseSensitive  bool
	MatchCount     int
	MatchIndex     int
}

type terminalMatch struct {
	start int
}

func decodeTerminalPreviewData(value string) terminalPreviewData {
	var data terminalPreviewData
	if json.Unmarshal([]byte(value), &data) == nil {
		return data
	}
	return terminalPreviewData{SessionID: strings.TrimSpace(value)}
}

// terminalPreviewFor switches the single visible subscription and returns an immutable render snapshot.
func (a *App) terminalPreviewFor(preview queryPreview) terminalPreviewSnapshot {
	data := decodeTerminalPreviewData(preview.PreviewData)
	a.mu.Lock()
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
	snapshot := snapshotTerminalPreviewLocked(state)
	newSessionID := state.SessionID
	a.mu.Unlock()
	if sessionChanged && (oldSessionID != "" || newSessionID != "") {
		go a.reconcileTerminalSubscription()
	}
	return snapshot
}

func snapshotTerminalPreviewLocked(state *terminalPreviewState) terminalPreviewSnapshot {
	if state == nil {
		return terminalPreviewSnapshot{}
	}
	snapshot := terminalPreviewSnapshot{
		SessionID: state.SessionID, Command: state.Command, Status: state.Status, Error: state.Error, Text: state.Text, Scroll: state.Scroll,
		LoadingHistory: state.LoadingHistory, SearchOpen: state.SearchOpen, CaseSensitive: state.CaseSensitive, MatchCount: len(state.Matches), MatchIndex: state.MatchIndex,
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

	a.mu.RLock()
	desiredSessionID := ""
	if a.terminalPreview != nil {
		desiredSessionID = a.terminalPreview.SessionID
	}
	a.mu.RUnlock()
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
	a.mu.RLock()
	desiredSessionID = ""
	if a.terminalPreview != nil {
		desiredSessionID = a.terminalPreview.SessionID
	}
	a.mu.RUnlock()
	if desiredSessionID == "" {
		return
	}
	if _, err := a.services.SubscribeTerminal(context.Background(), a.sessionID, desiredSessionID, -1); err != nil {
		log.Printf("subscribe terminal session: %v", err)
		return
	}
	a.terminalSubscribed = desiredSessionID
}

// deactivateTerminalPreview releases core output when the selected preview no longer uses it.
func (a *App) deactivateTerminalPreview() {
	a.mu.Lock()
	oldSessionID := ""
	searchWasOpen := false
	if a.terminalPreview != nil {
		oldSessionID = a.terminalPreview.SessionID
		searchWasOpen = a.terminalPreview.SearchOpen
		a.terminalPreview = nil
	}
	a.mu.Unlock()
	if oldSessionID != "" {
		go a.reconcileTerminalSubscription()
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
	a.mu.Lock()
	state := a.terminalPreview
	if state == nil || state.SessionID != chunk.SessionID {
		a.mu.Unlock()
		return
	}
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
		rebuildTerminalMatchesLocked(state, true)
	}
	if state.AutoFollow {
		state.Scroll = float32(math.MaxFloat32)
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) applyTerminalState(update terminalSessionState) {
	if update.SessionID == "" {
		return
	}
	a.mu.Lock()
	if state := a.terminalPreview; state != nil && state.SessionID == update.SessionID {
		if update.Command != "" {
			state.Command = update.Command
		}
		state.Status = update.Status
		state.Error = update.Error
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) clampTerminalPreviewScroll(maxOffset float32) {
	a.mu.Lock()
	if state := a.terminalPreview; state != nil {
		state.MaxScroll = max(float32(0), maxOffset)
		state.Scroll = min(max(float32(0), state.Scroll), maxOffset)
		state.AutoFollow = maxOffset-state.Scroll <= 24
	}
	a.mu.Unlock()
}

func (a *App) scrollTerminalPreview(delta, maxOffset float32) {
	if delta == 0 {
		return
	}
	requestSession := ""
	requestCursor := int64(-1)
	a.mu.Lock()
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
	a.mu.Unlock()
	_ = a.window.Invalidate()
	if requestSession != "" {
		go a.requestTerminalHistory(requestSession, requestCursor)
	}
}

// requestTerminalHistory resets the existing core subscription to an earlier byte cursor without racing selection changes.
func (a *App) requestTerminalHistory(sessionID string, cursor int64) {
	a.terminalSubscriptionMu.Lock()
	a.mu.RLock()
	current := a.terminalPreview != nil && a.terminalPreview.SessionID == sessionID
	a.mu.RUnlock()
	var err error
	sent := false
	if current && a.terminalSubscribed == sessionID {
		sent = true
		_, err = a.services.SubscribeTerminal(context.Background(), a.sessionID, sessionID, cursor)
	}
	a.terminalSubscriptionMu.Unlock()
	a.mu.Lock()
	if state := a.terminalPreview; state != nil && state.SessionID == sessionID && state.LastHistoryCursor == cursor {
		state.LoadingHistory = false
		if !sent || err != nil {
			if err != nil {
				state.Error = err.Error()
			}
			state.HistoryAnchorBase = 0
		}
	}
	a.mu.Unlock()
	if err != nil {
		log.Printf("load terminal history: %v", err)
	}
	_ = a.window.Invalidate()
}

// rebuildTerminalMatchesLocked indexes the loaded UTF-8 window and optionally preserves the absolute current hit.
func rebuildTerminalMatchesLocked(state *terminalPreviewState, preserveCurrent bool) {
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
		matches = append(matches, terminalMatch{start: start})
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
	a.mu.Lock()
	state := a.terminalPreview
	if state == nil {
		a.mu.Unlock()
		return
	}
	state.SearchOpen = true
	if state.SearchEditor == nil {
		state.SearchEditor = woxui.NewTextEditor("")
	}
	rebuildTerminalMatchesLocked(state, false)
	a.mu.Unlock()
	a.updateFormTextInput(true)
	_ = a.window.Invalidate()
}

// closeTerminalSearch returns text input ownership to the launcher query.
func (a *App) closeTerminalSearch() {
	a.mu.Lock()
	if state := a.terminalPreview; state != nil {
		state.SearchOpen = false
		state.Matches = nil
		state.MatchIndex = -1
	}
	a.mu.Unlock()
	a.restoreQueryTextInput()
	_ = a.window.Invalidate()
}

// setTerminalSearchCaret applies pointer hit testing from the shared editor widget.
func (a *App) setTerminalSearchCaret(offset int) {
	a.mu.Lock()
	if state := a.terminalPreview; state != nil && state.SearchOpen && state.SearchEditor != nil {
		state.SearchEditor.SetCaret(offset)
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

// moveTerminalSearch advances through loaded matches and scrolls to an approximate text-layout position.
func (a *App) moveTerminalSearch(delta int) {
	a.mu.Lock()
	state := a.terminalPreview
	if state == nil || !state.SearchOpen || state.SearchEditor == nil {
		a.mu.Unlock()
		return
	}
	rebuildTerminalMatchesLocked(state, true)
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
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

// toggleTerminalSearchCase rebuilds the loaded-window index without a core round trip.
func (a *App) toggleTerminalSearchCase() {
	a.mu.Lock()
	if state := a.terminalPreview; state != nil && state.SearchOpen {
		state.CaseSensitive = !state.CaseSensitive
		rebuildTerminalMatchesLocked(state, false)
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

// onTerminalPreviewKey handles preview-local find before launcher navigation sees the keystroke.
func (a *App) onTerminalPreviewKey(event woxui.KeyEvent) bool {
	a.mu.RLock()
	state := a.terminalPreview
	searchOpen := state != nil && state.SearchOpen
	a.mu.RUnlock()
	if event.Modifiers.HasPrimary() && event.Key == woxui.Key("f") {
		a.openTerminalSearch()
		return state != nil
	}
	if !searchOpen {
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
	a.mu.Lock()
	if state := a.terminalPreview; state != nil && state.SearchOpen && state.SearchEditor != nil {
		_, changed := state.SearchEditor.HandleKey(event)
		if changed {
			rebuildTerminalMatchesLocked(state, false)
		}
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
	return true
}

// onTerminalPreviewTextInput commits native IME input only while terminal find owns focus.
func (a *App) onTerminalPreviewTextInput(event woxui.TextInputEvent) bool {
	a.mu.Lock()
	state := a.terminalPreview
	if state == nil || !state.SearchOpen || state.SearchEditor == nil {
		a.mu.Unlock()
		return false
	}
	if state.SearchEditor.HandleTextInput(event) {
		rebuildTerminalMatchesLocked(state, false)
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
	return true
}
