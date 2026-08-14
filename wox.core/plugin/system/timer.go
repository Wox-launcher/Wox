package system

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
	"wox/util"
	"wox/util/notifier"
	"wox/util/overlay"
	"wox/util/overlay/timeroverlay"

	"github.com/google/uuid"
)

var timerPluginIcon = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="9" fill="#4f7cff"/><path fill="#fff" d="M12 7a1 1 0 0 1 1 1v3.586l2.207 2.207a1 1 0 1 1-1.414 1.414l-2.5-2.5A1 1 0 0 1 11 12V8a1 1 0 0 1 1-1z"/><path fill="#4f7cff" d="M11 2h2v2h-2z"/><path fill="#dbe6ff" d="M16.5 4.2 17.9 5.6 16.5 7 15.1 5.6z"/></svg>`)

var (
	timerPauseIcon  = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="9" fill="#f5c542"/><path fill="#fff" d="M9 7h2v10H9zm4 0h2v10h-2z"/></svg>`)
	timerResumeIcon = common.ExecuteRunIcon
	timerPinIcon    = common.UIIcon("screenshot.pin")
)

const (
	timerPluginID            = "a3f8c2e1-9b4d-4e7a-8c1f-2d5e6a7b8c9d"
	timerOverlayIDPrefix     = "wox_timer_overlay_"
	timerPersistedSettingKey = "activeTimers"
	timerResultScoreBase     = int64(1_000_000)
	timerStartResultScore    = int64(2_000_000)
	timerEmptyResultScore    = int64(1)
)

var timerDurationTokenRe = regexp.MustCompile(`(?i)(\d+)\s*([hms])`)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &TimerPlugin{})
}

// TimerPlugin is the built-in countdown timer plugin.
type TimerPlugin struct {
	api            plugin.API
	mu             sync.Mutex
	timers         map[string]*timerEntry
	trackedResults *util.HashMap[string, string] // resultId -> timerId
}

type timerEntry struct {
	ID             string
	DurationLabel  string // parsed duration text, e.g. "1m"
	Note           string // optional user description
	Duration       time.Duration
	Deadline       time.Time
	Remaining      time.Duration
	Paused         bool
	OverlayVisible bool
	OverlayPlaced  bool
}

// timerPersisted is the durable snapshot. Running timers store an absolute deadline so
// remaining time stays correct across restarts; paused timers store remaining duration.
// Pin/overlay state is intentionally not persisted.
type timerPersisted struct {
	ID             string `json:"id"`
	DurationLabel  string `json:"durationLabel"`
	Note           string `json:"note"`
	DurationMs     int64  `json:"durationMs"`
	DeadlineUnixMs int64  `json:"deadlineUnixMs"`
	RemainingMs    int64  `json:"remainingMs"`
	Paused         bool   `json:"paused"`
}

func (t *TimerPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            timerPluginID,
		Name:          "i18n:plugin_timer_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_timer_plugin_description",
		Icon:          timerPluginIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"timer",
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
	}
}

func (t *TimerPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	t.api = initParams.API
	t.timers = make(map[string]*timerEntry)
	t.trackedResults = util.NewHashMap[string, string]()
	t.loadTimers(ctx)

	util.Go(ctx, "refresh timers", func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			t.tick(util.NewTraceContext())
		}
	})
}

func (t *TimerPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	search := strings.TrimSpace(query.Search)
	results := []plugin.QueryResult{}

	if duration, durationLabel, note, ok := parseTimerQuery(search); ok {
		results = append(results, t.buildStartResult(ctx, duration, durationLabel, note))
	}

	for _, entry := range t.listTimersSorted() {
		results = append(results, t.buildTimerResult(ctx, entry))
	}

	if len(results) == 0 {
		results = append(results, plugin.QueryResult{
			Title:    "i18n:plugin_timer_no_timers",
			SubTitle: "i18n:plugin_timer_no_timers_subtitle",
			Icon:     timerPluginIcon,
			Score:    timerEmptyResultScore,
		})
	}

	return plugin.QueryResponse{Results: results}
}

// buildStartResult creates start actions for a parsed duration and optional note.
func (t *TimerPlugin) buildStartResult(ctx context.Context, duration time.Duration, durationLabel, note string) plugin.QueryResult {
	title := t.api.GetTranslation(ctx, "plugin_timer_start_title") + " " + durationLabel
	subTitle := "i18n:plugin_timer_start_subtitle"
	if note != "" {
		subTitle = note
	}

	return plugin.QueryResult{
		Title:    title,
		SubTitle: subTitle,
		Icon:     timerPluginIcon,
		Score:    timerStartResultScore,
		Actions: []plugin.QueryResultAction{
			{
				Name:      "i18n:plugin_timer_action_start_pinned",
				Icon:      timerPinIcon,
				IsDefault: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					t.startTimer(ctx, duration, durationLabel, note, true)
				},
			},
			{
				Name:   "i18n:plugin_timer_action_start_background",
				Icon:   timerResumeIcon,
				Hotkey: util.PrimaryHotkey("enter"),
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					t.startTimer(ctx, duration, durationLabel, note, false)
				},
			},
		},
	}
}

// buildTimerResult maps one active timer into a query result with live actions.
func (t *TimerPlugin) buildTimerResult(ctx context.Context, entry *timerEntry) plugin.QueryResult {
	remaining := t.remainingOf(entry)
	title := formatTimerRemaining(remaining)
	subTitle := t.timerSubtitle(ctx, entry)
	result := plugin.QueryResult{
		Id:       entry.ID,
		Title:    title,
		SubTitle: subTitle,
		Icon:     timerResultIcon(entry),
		Score:    timerResultScoreBase - int64(remaining/time.Second),
		ScoreKey: "timer:" + entry.ID,
		Tails:    t.timerTails(ctx, entry),
		Actions:  t.buildTimerActions(ctx, entry.ID),
	}
	t.trackResult(result.Id, entry.ID)
	return result
}

func (t *TimerPlugin) buildTimerActions(ctx context.Context, timerID string) []plugin.QueryResultAction {
	entry := t.getTimer(timerID)
	if entry == nil {
		return nil
	}

	actions := []plugin.QueryResultAction{}

	// Enter toggles pause/resume; Primary+Enter toggles desktop overlay visibility.
	if entry.Paused {
		actions = append(actions, plugin.QueryResultAction{
			Id:                     timerID + ":resume",
			Name:                   "i18n:plugin_timer_action_resume",
			Icon:                   timerResumeIcon,
			IsDefault:              true,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				t.resumeTimer(ctx, timerID)
				t.pushTimerResultUpdate(ctx, actionContext.ResultId, timerID)
			},
		})
	} else {
		actions = append(actions, plugin.QueryResultAction{
			Id:                     timerID + ":pause",
			Name:                   "i18n:plugin_timer_action_pause",
			Icon:                   timerPauseIcon,
			IsDefault:              true,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				t.pauseTimer(ctx, timerID)
				t.pushTimerResultUpdate(ctx, actionContext.ResultId, timerID)
			},
		})
	}

	pinName := "i18n:plugin_timer_action_pin_on_desktop"
	if entry.OverlayVisible {
		pinName = "i18n:plugin_timer_action_unpin_from_desktop"
	}
	actions = append(actions, plugin.QueryResultAction{
		Id:                     timerID + ":pin",
		Name:                   pinName,
		Icon:                   timerPinIcon,
		Hotkey:                 util.PrimaryHotkey("enter"),
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			t.togglePin(ctx, timerID)
			t.pushTimerResultUpdate(ctx, actionContext.ResultId, timerID)
		},
	})

	actions = append(actions, plugin.QueryResultAction{
		Id:                     timerID + ":edit_note",
		Name:                   "i18n:plugin_timer_action_edit_note",
		Icon:                   common.EditIcon,
		Type:                   plugin.QueryResultActionTypeForm,
		PreventHideAfterAction: true,
		Form: definition.PluginSettingDefinitions{
			{
				Type: definition.PluginSettingDefinitionTypeTextBox,
				Value: &definition.PluginSettingValueTextBox{
					Key:          "note",
					Label:        "i18n:plugin_timer_edit_note_label",
					DefaultValue: entry.Note,
					Tooltip:      "i18n:plugin_timer_edit_note_hint",
				},
			},
		},
		OnSubmit: func(ctx context.Context, actionContext plugin.FormActionContext) {
			t.updateNote(ctx, timerID, strings.TrimSpace(actionContext.Values["note"]))
			// Refresh the visible query so subtitle and form defaults stay in sync.
			t.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
		},
	})

	actions = append(actions, plugin.QueryResultAction{
		Id:                     timerID + ":delete",
		Name:                   "i18n:plugin_timer_action_delete",
		Icon:                   common.TrashIcon,
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			t.deleteTimer(ctx, timerID)
			t.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
		},
	})

	return actions
}

func (t *TimerPlugin) timerSubtitle(ctx context.Context, entry *timerEntry) string {
	parts := []string{}
	if entry.Note != "" {
		parts = append(parts, entry.Note)
	} else {
		parts = append(parts, entry.DurationLabel)
	}
	if entry.Paused {
		parts = append(parts, t.api.GetTranslation(ctx, "plugin_timer_status_paused"))
	}
	return strings.Join(parts, " · ")
}

// timerResultIcon uses a pause glyph while the timer is paused so running/paused rows are distinct.
func timerResultIcon(entry *timerEntry) common.WoxImage {
	if entry != nil && entry.Paused {
		return timerPauseIcon
	}
	return timerPluginIcon
}

// timerTails only keeps non-subtitle status chips (duration / pinned). Note and paused text stay in SubTitle.
func (t *TimerPlugin) timerTails(ctx context.Context, entry *timerEntry) []plugin.QueryResultTail {
	tails := []plugin.QueryResultTail{plugin.NewQueryResultTailText(entry.DurationLabel)}
	if entry.OverlayVisible {
		tails = append(tails, plugin.NewQueryResultTailText(t.api.GetTranslation(ctx, "plugin_timer_status_pinned")))
	}
	return tails
}

func (t *TimerPlugin) trackResult(resultID, timerID string) {
	if t.trackedResults == nil {
		t.trackedResults = util.NewHashMap[string, string]()
	}
	t.trackedResults.Store(resultID, timerID)
}

// startTimer creates a timer and optionally pins it on the desktop immediately.
func (t *TimerPlugin) startTimer(ctx context.Context, duration time.Duration, durationLabel, note string, pin bool) *timerEntry {
	entry := t.addTimer(ctx, duration, durationLabel, note)
	display := durationLabel
	if note != "" {
		display = note
	}
	t.api.Notify(ctx, fmt.Sprintf("%s %s", t.api.GetTranslation(ctx, "plugin_timer_started"), display))
	if pin {
		t.setPinned(ctx, entry.ID, true)
	}
	return entry
}

func (t *TimerPlugin) addTimer(ctx context.Context, duration time.Duration, durationLabel, note string) *timerEntry {
	entry := &timerEntry{
		ID:            uuid.NewString(),
		DurationLabel: durationLabel,
		Note:          note,
		Duration:      duration,
		Deadline:      time.Now().Add(duration),
		Remaining:     duration,
	}

	t.mu.Lock()
	if t.timers == nil {
		t.timers = make(map[string]*timerEntry)
	}
	t.timers[entry.ID] = entry
	t.mu.Unlock()

	t.saveTimers(ctx)
	t.logInfo(ctx, fmt.Sprintf("timer started: id=%s duration=%s note=%q", entry.ID, durationLabel, note))
	return entry
}

func (t *TimerPlugin) updateNote(ctx context.Context, id, note string) {
	t.mu.Lock()
	entry, ok := t.timers[id]
	if !ok {
		t.mu.Unlock()
		return
	}
	entry.Note = note
	pinned := entry.OverlayVisible
	t.mu.Unlock()

	t.saveTimers(ctx)
	t.logInfo(ctx, fmt.Sprintf("timer note updated: id=%s note=%q", id, note))
	if pinned {
		t.showOverlay(ctx, id, true)
	}
}

func (t *TimerPlugin) getTimer(id string) *timerEntry {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.timers == nil {
		return nil
	}
	entry, ok := t.timers[id]
	if !ok {
		return nil
	}
	copy := *entry
	return &copy
}

func (t *TimerPlugin) listTimersSorted() []*timerEntry {
	t.mu.Lock()
	defer t.mu.Unlock()

	entries := make([]*timerEntry, 0, len(t.timers))
	for _, entry := range t.timers {
		copy := *entry
		entries = append(entries, &copy)
	}
	sort.Slice(entries, func(i, j int) bool {
		return t.remainingOfLocked(entries[i]) < t.remainingOfLocked(entries[j])
	})
	return entries
}

func (t *TimerPlugin) remainingOf(entry *timerEntry) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.remainingOfLocked(entry)
}

func (t *TimerPlugin) remainingOfLocked(entry *timerEntry) time.Duration {
	if entry == nil {
		return 0
	}
	if entry.Paused {
		if entry.Remaining < 0 {
			return 0
		}
		return entry.Remaining
	}
	remaining := time.Until(entry.Deadline)
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (t *TimerPlugin) pauseTimer(ctx context.Context, id string) {
	t.mu.Lock()
	entry, ok := t.timers[id]
	if !ok || entry.Paused {
		t.mu.Unlock()
		return
	}
	entry.Remaining = time.Until(entry.Deadline)
	if entry.Remaining < 0 {
		entry.Remaining = 0
	}
	entry.Paused = true
	remaining := entry.Remaining
	t.mu.Unlock()
	t.saveTimers(ctx)
	t.logInfo(ctx, fmt.Sprintf("timer paused: id=%s remaining=%s", id, remaining))
}

func (t *TimerPlugin) resumeTimer(ctx context.Context, id string) {
	t.mu.Lock()
	entry, ok := t.timers[id]
	if !ok || !entry.Paused {
		t.mu.Unlock()
		return
	}
	entry.Deadline = time.Now().Add(entry.Remaining)
	entry.Paused = false
	remaining := entry.Remaining
	t.mu.Unlock()
	t.saveTimers(ctx)
	t.logInfo(ctx, fmt.Sprintf("timer resumed: id=%s remaining=%s", id, remaining))
}

func (t *TimerPlugin) deleteTimer(ctx context.Context, id string) {
	t.closeOverlay(id)
	t.mu.Lock()
	delete(t.timers, id)
	t.mu.Unlock()
	t.saveTimers(ctx)
	t.logInfo(ctx, fmt.Sprintf("timer deleted: id=%s", id))
}

func (t *TimerPlugin) togglePin(ctx context.Context, id string) {
	entry := t.getTimer(id)
	if entry == nil {
		return
	}
	t.setPinned(ctx, id, !entry.OverlayVisible)
}

func (t *TimerPlugin) setPinned(ctx context.Context, id string, pinned bool) {
	t.mu.Lock()
	entry, ok := t.timers[id]
	if !ok {
		t.mu.Unlock()
		return
	}
	entry.OverlayVisible = pinned
	if !pinned {
		entry.OverlayPlaced = false
	}
	t.mu.Unlock()

	if pinned {
		t.showOverlay(ctx, id, false)
	} else {
		t.closeOverlay(id)
	}
}

func (t *TimerPlugin) overlayID(timerID string) string {
	return timerOverlayIDPrefix + timerID
}

func (t *TimerPlugin) showOverlay(ctx context.Context, timerID string, preservePosition bool) {
	entry := t.getTimer(timerID)
	if entry == nil || !entry.OverlayVisible {
		return
	}

	countdown := formatTimerRemaining(t.remainingOf(entry))
	note := entry.Note
	if entry.Paused {
		paused := t.api.GetTranslation(ctx, "plugin_timer_status_paused")
		if note != "" {
			note = note + " · " + paused
		} else {
			note = paused
		}
	}

	timeroverlay.Show(timeroverlay.Options{
		Window: overlay.WindowOptions{
			ID:               t.overlayID(timerID),
			Anchor:           overlay.AnchorBottomCenter,
			OffsetY:          -80,
			Topmost:          true,
			Movable:          true,
			PreservePosition: preservePosition,
			OnClose: func() {
				t.mu.Lock()
				if current, ok := t.timers[timerID]; ok {
					current.OverlayVisible = false
					current.OverlayPlaced = false
				}
				t.mu.Unlock()
			},
		},
		Countdown: countdown,
		Note:      note,
		Closable:  true,
	})

	t.mu.Lock()
	if current, ok := t.timers[timerID]; ok && current.OverlayVisible {
		current.OverlayPlaced = true
	}
	t.mu.Unlock()
}

func (t *TimerPlugin) closeOverlay(timerID string) {
	overlay.Close(t.overlayID(timerID))
}

// tick advances timers, refreshes visible results, and updates pinned overlays.
func (t *TimerPlugin) tick(ctx context.Context) {
	finished := t.collectFinished(ctx)
	for _, entry := range finished {
		t.finishTimer(ctx, entry)
	}

	t.refreshOverlays(ctx)

	if t.api == nil || !t.api.IsVisible(context.Background()) || t.trackedResults == nil {
		return
	}

	var toRemove []string
	t.trackedResults.Range(func(resultID, timerID string) bool {
		if t.getTimer(timerID) == nil {
			toRemove = append(toRemove, resultID)
			return true
		}
		// The first tick can race with the query result reaching the UI. Keep the
		// active timer tracked so a transient miss is retried on the next tick.
		t.pushTimerResultUpdate(ctx, resultID, timerID)
		return true
	})
	for _, resultID := range toRemove {
		t.trackedResults.Delete(resultID)
	}
}

func (t *TimerPlugin) collectFinished(ctx context.Context) []*timerEntry {
	t.mu.Lock()
	defer t.mu.Unlock()

	finished := []*timerEntry{}
	now := time.Now()
	for _, entry := range t.timers {
		if entry.Paused {
			continue
		}
		if !entry.Deadline.After(now) {
			copy := *entry
			finished = append(finished, &copy)
		}
	}
	return finished
}

func (t *TimerPlugin) finishTimer(ctx context.Context, entry *timerEntry) {
	t.closeOverlay(entry.ID)
	t.mu.Lock()
	delete(t.timers, entry.ID)
	t.mu.Unlock()
	t.saveTimers(ctx)
	t.notifyTimerFinished(ctx, entry)
}

func (t *TimerPlugin) notifyTimerFinished(ctx context.Context, entry *timerEntry) {
	display := entry.DurationLabel
	if entry.Note != "" {
		display = entry.Note
	}
	message := fmt.Sprintf("%s %s", t.api.GetTranslation(ctx, "plugin_timer_finished"), display)
	if t.api != nil {
		t.api.Notify(ctx, message)
	}
	icon, _ := timerPluginIcon.ToImage()
	notifier.Notify(icon, message)
	t.logInfo(ctx, fmt.Sprintf("timer finished: id=%s duration=%s note=%q", entry.ID, entry.DurationLabel, entry.Note))
}

func (t *TimerPlugin) logInfo(ctx context.Context, msg string) {
	if t.api == nil {
		return
	}
	t.api.Log(ctx, plugin.LogLevelInfo, msg)
}

// loadTimers restores unfinished timers from plugin settings. Expired running timers
// notify once and are dropped; pin/overlay state always starts cleared.
func (t *TimerPlugin) loadTimers(ctx context.Context) {
	if t.api == nil {
		return
	}
	raw := strings.TrimSpace(t.api.GetSetting(ctx, timerPersistedSettingKey))
	if raw == "" {
		return
	}

	var items []timerPersisted
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		t.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to load timers: %s", err.Error()))
		return
	}

	now := time.Now()
	expired := make([]*timerEntry, 0)
	kept := 0
	t.mu.Lock()
	for _, item := range items {
		entry := timerEntryFromPersisted(item)
		if entry.ID == "" || entry.Duration <= 0 {
			continue
		}
		if !entry.Paused && !entry.Deadline.After(now) {
			expired = append(expired, entry)
			continue
		}
		t.timers[entry.ID] = entry
	}
	kept = len(t.timers)
	t.mu.Unlock()

	for _, entry := range expired {
		t.notifyTimerFinished(ctx, entry)
	}
	if len(expired) > 0 || kept != len(items) {
		t.saveTimers(ctx)
	}
	t.logInfo(ctx, fmt.Sprintf("loaded %d timers (%d expired while away)", kept, len(expired)))
}

func (t *TimerPlugin) saveTimers(ctx context.Context) {
	if t.api == nil {
		return
	}

	t.mu.Lock()
	items := make([]timerPersisted, 0, len(t.timers))
	for _, entry := range t.timers {
		items = append(items, timerEntryToPersisted(entry))
	}
	t.mu.Unlock()

	data, err := json.Marshal(items)
	if err != nil {
		t.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to serialize timers: %s", err.Error()))
		return
	}
	// Platform-specific: countdown deadlines belong to this device session.
	t.api.SaveSetting(ctx, timerPersistedSettingKey, string(data), true)
}

func timerEntryToPersisted(entry *timerEntry) timerPersisted {
	return timerPersisted{
		ID:             entry.ID,
		DurationLabel:  entry.DurationLabel,
		Note:           entry.Note,
		DurationMs:     entry.Duration.Milliseconds(),
		DeadlineUnixMs: entry.Deadline.UnixMilli(),
		RemainingMs:    entry.Remaining.Milliseconds(),
		Paused:         entry.Paused,
	}
}

func timerEntryFromPersisted(item timerPersisted) *timerEntry {
	return &timerEntry{
		ID:            item.ID,
		DurationLabel: item.DurationLabel,
		Note:          item.Note,
		Duration:      time.Duration(item.DurationMs) * time.Millisecond,
		Deadline:      time.UnixMilli(item.DeadlineUnixMs),
		Remaining:     time.Duration(item.RemainingMs) * time.Millisecond,
		Paused:        item.Paused,
	}
}

func (t *TimerPlugin) refreshOverlays(ctx context.Context) {
	t.mu.Lock()
	ids := make([]string, 0, len(t.timers))
	for id, entry := range t.timers {
		if entry.OverlayVisible {
			ids = append(ids, id)
		}
	}
	t.mu.Unlock()

	for _, id := range ids {
		entry := t.getTimer(id)
		if entry == nil {
			continue
		}
		t.showOverlay(ctx, id, entry.OverlayPlaced)
	}
}

// pushTimerResultUpdate refreshes one visible result from the current timer state.
func (t *TimerPlugin) pushTimerResultUpdate(ctx context.Context, resultID, timerID string) bool {
	entry := t.getTimer(timerID)
	if entry == nil {
		return false
	}

	remaining := t.remainingOf(entry)
	title := formatTimerRemaining(remaining)
	subTitle := t.timerSubtitle(ctx, entry)
	icon := timerResultIcon(entry)
	tails := t.timerTails(ctx, entry)
	actions := t.buildTimerActions(ctx, timerID)

	update := plugin.UpdatableResult{
		Id:       resultID,
		Title:    &title,
		SubTitle: &subTitle,
		Icon:     &icon,
		Tails:    &tails,
		Actions:  &actions,
	}
	return t.api.UpdateResult(ctx, update)
}

// parseTimerQuery parses "5m", "1h 15m cooking", and "1m这只是一个说明".
// Duration tokens must lead the input; any trailing text becomes the optional note.
func parseTimerQuery(input string) (time.Duration, string, string, bool) {
	input = strings.TrimSpace(input)
	if input == "" {
		return 0, "", "", false
	}

	loc := 0
	var total time.Duration
	parts := make([]string, 0, 3)
	for {
		rest := strings.TrimLeft(input[loc:], " \t")
		skipped := len(input[loc:]) - len(rest)
		indexes := timerDurationTokenRe.FindStringSubmatchIndex(rest)
		if indexes == nil || indexes[0] != 0 {
			break
		}

		value, err := strconv.Atoi(rest[indexes[2]:indexes[3]])
		if err != nil || value < 0 {
			return 0, "", "", false
		}
		unit := strings.ToLower(rest[indexes[4]:indexes[5]])
		switch unit {
		case "h":
			total += time.Duration(value) * time.Hour
		case "m":
			total += time.Duration(value) * time.Minute
		case "s":
			total += time.Duration(value) * time.Second
		default:
			return 0, "", "", false
		}
		parts = append(parts, fmt.Sprintf("%d%s", value, unit))
		loc += skipped + indexes[1]
	}

	if total <= 0 || len(parts) == 0 {
		return 0, "", "", false
	}

	note := strings.TrimSpace(input[loc:])
	return total, strings.Join(parts, " "), note, true
}

// formatTimerRemaining formats remaining time for result titles and overlays.
func formatTimerRemaining(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalSec := int((d + time.Second/2) / time.Second)
	hours := totalSec / 3600
	minutes := (totalSec % 3600) / 60
	seconds := totalSec % 60
	if hours > 0 {
		return fmt.Sprintf("%dh %02dm %02ds", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}
