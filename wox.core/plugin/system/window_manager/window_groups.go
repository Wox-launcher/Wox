package window_manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/setting"
	"wox/util/shell"
	"wox/util/window"
)

const (
	windowGroupLaunchWaitTimeout  = 6 * time.Second
	windowGroupLaunchPollInterval = 250 * time.Millisecond
)

const (
	windowGroupLayoutFull             = "full"
	windowGroupLayoutHalvesHorizontal = "halves-horizontal"
	windowGroupLayoutHalvesVertical   = "halves-vertical"
	windowGroupLayoutThreeLeftMain    = "three-left-main"
	windowGroupLayoutThreeRightMain   = "three-right-main"
	windowGroupLayoutThreeTopMain     = "three-top-main"
	windowGroupLayoutThreeBottomMain  = "three-bottom-main"
	windowGroupLayoutQuarters         = "quarters"
)

type windowManagerWindowGroup struct {
	Id      string
	Name    string
	Screens []windowManagerWindowGroupScreen
}

type windowManagerWindowGroupScreen struct {
	DisplayId    string
	DisplayIndex int
	Layout       string
	Assignments  []windowManagerWindowGroupAssignment
}

type windowManagerWindowGroupAssignment struct {
	Slot string
	App  setting.IgnoredHotkeyApp
}

type windowGroupLayoutDefinition struct {
	Id    string
	Slots []windowGroupLayoutSlot
}

type windowGroupLayoutSlot struct {
	Id      string
	Cols    int
	Rows    int
	Col     int
	Row     int
	ColSpan int
	RowSpan int
}

type windowGroupPlacement struct {
	GroupName string
	Identity  string
	AppName   string
	AppPath   string
	Window    window.ManagedWindow
	Rect      window.WindowRect
}

type windowGroupApplySummary struct {
	Moved          int
	Launched       int
	MissingApps    []string
	FailedApps     []string
	PermissionApps []string
	LaunchFailures []string
}

var windowGroupLayoutDefinitions = map[string]windowGroupLayoutDefinition{
	windowGroupLayoutFull: {
		Id:    windowGroupLayoutFull,
		Slots: []windowGroupLayoutSlot{{Id: "full", Cols: 1, Rows: 1, Col: 0, Row: 0, ColSpan: 1, RowSpan: 1}},
	},
	windowGroupLayoutHalvesHorizontal: {
		Id: windowGroupLayoutHalvesHorizontal,
		Slots: []windowGroupLayoutSlot{
			{Id: "left", Cols: 2, Rows: 1, Col: 0, Row: 0, ColSpan: 1, RowSpan: 1},
			{Id: "right", Cols: 2, Rows: 1, Col: 1, Row: 0, ColSpan: 1, RowSpan: 1},
		},
	},
	windowGroupLayoutHalvesVertical: {
		Id: windowGroupLayoutHalvesVertical,
		Slots: []windowGroupLayoutSlot{
			{Id: "top", Cols: 1, Rows: 2, Col: 0, Row: 0, ColSpan: 1, RowSpan: 1},
			{Id: "bottom", Cols: 1, Rows: 2, Col: 0, Row: 1, ColSpan: 1, RowSpan: 1},
		},
	},
	windowGroupLayoutThreeLeftMain: {
		Id: windowGroupLayoutThreeLeftMain,
		Slots: []windowGroupLayoutSlot{
			{Id: "left", Cols: 2, Rows: 2, Col: 0, Row: 0, ColSpan: 1, RowSpan: 2},
			{Id: "rightTop", Cols: 2, Rows: 2, Col: 1, Row: 0, ColSpan: 1, RowSpan: 1},
			{Id: "rightBottom", Cols: 2, Rows: 2, Col: 1, Row: 1, ColSpan: 1, RowSpan: 1},
		},
	},
	windowGroupLayoutThreeRightMain: {
		Id: windowGroupLayoutThreeRightMain,
		Slots: []windowGroupLayoutSlot{
			{Id: "leftTop", Cols: 2, Rows: 2, Col: 0, Row: 0, ColSpan: 1, RowSpan: 1},
			{Id: "leftBottom", Cols: 2, Rows: 2, Col: 0, Row: 1, ColSpan: 1, RowSpan: 1},
			{Id: "right", Cols: 2, Rows: 2, Col: 1, Row: 0, ColSpan: 1, RowSpan: 2},
		},
	},
	windowGroupLayoutThreeTopMain: {
		Id: windowGroupLayoutThreeTopMain,
		Slots: []windowGroupLayoutSlot{
			{Id: "top", Cols: 2, Rows: 2, Col: 0, Row: 0, ColSpan: 2, RowSpan: 1},
			{Id: "bottomLeft", Cols: 2, Rows: 2, Col: 0, Row: 1, ColSpan: 1, RowSpan: 1},
			{Id: "bottomRight", Cols: 2, Rows: 2, Col: 1, Row: 1, ColSpan: 1, RowSpan: 1},
		},
	},
	windowGroupLayoutThreeBottomMain: {
		Id: windowGroupLayoutThreeBottomMain,
		Slots: []windowGroupLayoutSlot{
			{Id: "topLeft", Cols: 2, Rows: 2, Col: 0, Row: 0, ColSpan: 1, RowSpan: 1},
			{Id: "topRight", Cols: 2, Rows: 2, Col: 1, Row: 0, ColSpan: 1, RowSpan: 1},
			{Id: "bottom", Cols: 2, Rows: 2, Col: 0, Row: 1, ColSpan: 2, RowSpan: 1},
		},
	},
	windowGroupLayoutQuarters: {
		Id: windowGroupLayoutQuarters,
		Slots: []windowGroupLayoutSlot{
			{Id: "topLeft", Cols: 2, Rows: 2, Col: 0, Row: 0, ColSpan: 1, RowSpan: 1},
			{Id: "topRight", Cols: 2, Rows: 2, Col: 1, Row: 0, ColSpan: 1, RowSpan: 1},
			{Id: "bottomLeft", Cols: 2, Rows: 2, Col: 0, Row: 1, ColSpan: 1, RowSpan: 1},
			{Id: "bottomRight", Cols: 2, Rows: 2, Col: 1, Row: 1, ColSpan: 1, RowSpan: 1},
		},
	},
}

// windowManagerMetadataCommands registers normal window commands plus the group command used by query hotkeys.
func windowManagerMetadataCommands() []plugin.MetadataCommand {
	commands := make([]plugin.MetadataCommand, 0, len(windowManagerCommands)+1)
	for _, command := range windowManagerCommands {
		commands = append(commands, plugin.MetadataCommand{
			Command:     command.Command,
			Description: common.I18nString("i18n:" + command.TitleKey),
		})
	}
	commands = append(commands, plugin.MetadataCommand{
		Command:     windowManagerCommandGroup,
		Description: common.I18nString("i18n:plugin_window_manager_command_group"),
	})
	return commands
}

// queryWindowGroups resolves the exact group id used by silent hotkeys or lists groups for interactive queries.
func (p *WindowManagerPlugin) queryWindowGroups(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	groups := p.loadWindowGroups(ctx)
	search := strings.TrimSpace(query.Search)
	if len(groups) == 0 {
		if search == "" {
			return plugin.NewQueryResponse([]plugin.QueryResult{{
				Title:    "i18n:plugin_window_manager_group_empty_title",
				SubTitle: "i18n:plugin_window_manager_group_empty_subtitle",
				Icon:     windowManagerIcon,
			}})
		}
		return plugin.QueryResponse{}
	}

	return plugin.NewQueryResponse(p.windowGroupResults(ctx, groups, search, true))
}

// matchingWindowGroupResults lets normal and global searches apply saved groups without requiring an active window.
func (p *WindowManagerPlugin) matchingWindowGroupResults(ctx context.Context, search string, includeEmptySearch bool) []plugin.QueryResult {
	search = strings.TrimSpace(search)
	if search == "" && !includeEmptySearch {
		return nil
	}

	groups := p.loadWindowGroups(ctx)
	if len(groups) == 0 {
		return nil
	}
	return p.windowGroupResults(ctx, groups, search, includeEmptySearch)
}

func (p *WindowManagerPlugin) windowGroupResults(ctx context.Context, groups []windowManagerWindowGroup, search string, includeEmptySearch bool) []plugin.QueryResult {
	search = strings.TrimSpace(search)
	if search == "" && !includeEmptySearch {
		return nil
	}
	if search != "" {
		if group, ok := findExactWindowGroup(groups, search); ok {
			return []plugin.QueryResult{p.windowGroupResult(ctx, group, 1000)}
		}
	}

	results := make([]plugin.QueryResult, 0, len(groups))
	for _, group := range groups {
		if matched, score := windowGroupMatches(ctx, group, search); matched {
			results = append(results, p.windowGroupResult(ctx, group, score))
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return strings.ToLower(results[i].Title) < strings.ToLower(results[j].Title)
		}
		return results[i].Score > results[j].Score
	})
	return results
}

// loadWindowGroups decodes the platform-specific group setting and ignores incomplete rows.
func (p *WindowManagerPlugin) loadWindowGroups(ctx context.Context) []windowManagerWindowGroup {
	raw := strings.TrimSpace(p.api.GetSetting(ctx, windowManagerSettingGroups))
	if raw == "" {
		return nil
	}

	var groups []windowManagerWindowGroup
	if err := json.Unmarshal([]byte(raw), &groups); err != nil {
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to unmarshal window groups: %s", err.Error()))
		return nil
	}

	normalized := make([]windowManagerWindowGroup, 0, len(groups))
	for _, group := range groups {
		group.Id = strings.TrimSpace(group.Id)
		group.Name = strings.TrimSpace(group.Name)
		if group.Id == "" {
			continue
		}
		if group.Name == "" {
			group.Name = group.Id
		}
		normalized = append(normalized, group)
	}
	return normalized
}

// windowGroupResult creates the executable query result for one group.
func (p *WindowManagerPlugin) windowGroupResult(ctx context.Context, group windowManagerWindowGroup, score int64) plugin.QueryResult {
	appCount := countWindowGroupApps(group)
	subtitle := fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_window_manager_group_subtitle"), appCount, len(group.Screens))
	capturedGroup := group

	return plugin.QueryResult{
		Id:       "window-group-" + group.Id,
		Title:    group.Name,
		SubTitle: subtitle,
		Icon:     windowManagerIcon,
		Score:    score,
		ScoreKey: "window-group:" + group.Id,
		Actions: []plugin.QueryResultAction{
			{
				Name:                   "i18n:plugin_window_manager_group_action_apply",
				IsDefault:              true,
				PreventHideAfterAction: true,
				Action: func(actionCtx context.Context, actionContext plugin.ActionContext) {
					p.applyWindowGroup(actionCtx, capturedGroup)
				},
			},
		},
	}
}

// applyWindowGroup launches missing apps when possible, then moves matching windows into configured slots.
func (p *WindowManagerPlugin) applyWindowGroup(ctx context.Context, group windowManagerWindowGroup) {
	p.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("window manager apply group: id=%s name=%q screens=%d", group.Id, group.Name, len(group.Screens)))
	p.api.HideApp(ctx)
	time.Sleep(120 * time.Millisecond)

	summary, err := p.arrangeWindowGroup(ctx, group)
	if err != nil {
		p.notifyFailure(ctx, err)
		return
	}
	p.notifyWindowGroupSummary(ctx, group, summary)
}

// arrangeWindowGroup resolves screens, apps, windows, and applies all valid placements.
func (p *WindowManagerPlugin) arrangeWindowGroup(ctx context.Context, group windowManagerWindowGroup) (windowGroupApplySummary, error) {
	var summary windowGroupApplySummary

	displays, err := window.ListDisplays()
	if err != nil {
		return summary, err
	}

	placements, err := p.buildWindowGroupPlacements(ctx, group, displays, p.getGap(ctx))
	if err != nil {
		return summary, err
	}
	if len(placements) == 0 {
		return summary, nil
	}

	windows, err := window.ListManagedWindows()
	if err != nil {
		return summary, err
	}

	windowsByIdentity := indexManagedWindowsByIdentity(windows)
	missingBeforeLaunch := missingPlacementIdentities(placements, windowsByIdentity)
	if len(missingBeforeLaunch) > 0 {
		for _, placement := range placements {
			if !missingBeforeLaunch[placement.Identity] || strings.TrimSpace(placement.AppPath) == "" {
				continue
			}

			if err := shell.Open(placement.AppPath); err != nil {
				summary.LaunchFailures = appendUniqueString(summary.LaunchFailures, placement.AppName)
				p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("window manager failed to launch app for group: group=%s app=%s identity=%s path=%s err=%s", group.Id, placement.AppName, placement.Identity, placement.AppPath, err.Error()))
				continue
			}
			summary.Launched++
		}

		if summary.Launched > 0 {
			windows, err = waitForWindowGroupWindows(placements)
			if err != nil {
				return summary, err
			}
			windowsByIdentity = indexManagedWindowsByIdentity(windows)
		}
	}

	for _, placement := range placements {
		candidates := windowsByIdentity[placement.Identity]
		if len(candidates) == 0 {
			summary.MissingApps = appendUniqueString(summary.MissingApps, placement.AppName)
			continue
		}

		placement.Window = candidates[0]
		windowsByIdentity[placement.Identity] = candidates[1:]
		p.storeRestoreRect(placement.Window, placement.Window.Bounds)
		if err := window.MoveResizeWindow(placement.Window, placement.Rect); err != nil {
			if errors.Is(err, window.ErrWindowManagementPermissionDenied) {
				summary.PermissionApps = appendUniqueString(summary.PermissionApps, placement.AppName)
			} else {
				summary.FailedApps = appendUniqueString(summary.FailedApps, placement.AppName)
			}
			p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("window manager failed to move group window: group=%s app=%s identity=%s windowId=%s err=%s", group.Id, placement.AppName, placement.Identity, placement.Window.Id, err.Error()))
			continue
		}
		updatedWindow, err := window.GetManagedWindow(placement.Window.Id, placement.Window.Pid, placement.Window.Title)
		if err != nil {
			p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("window manager failed to verify moved group window: group=%s app=%s identity=%s windowId=%s pid=%d err=%s", group.Id, placement.AppName, placement.Identity, placement.Window.Id, placement.Window.Pid, err.Error()))
		} else {
			p.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("window manager applied group rect: group=%s app=%s identity=%s before=%+v target=%+v after=%+v", group.Id, placement.AppName, placement.Identity, placement.Window.Bounds, placement.Rect, updatedWindow.Bounds))
			if !windowRectApproximatelyEqual(updatedWindow.Bounds, placement.Rect) {
				p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("window manager group target mismatch after move: group=%s app=%s identity=%s target=%+v after=%+v", group.Id, placement.AppName, placement.Identity, placement.Rect, updatedWindow.Bounds))
				// Windows can scale the first cross-DPI move before the window updates its target monitor DPI.
				if retryErr := window.MoveResizeWindow(updatedWindow, placement.Rect); retryErr != nil {
					p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("window manager failed to retry group window move: group=%s app=%s identity=%s windowId=%s err=%s", group.Id, placement.AppName, placement.Identity, placement.Window.Id, retryErr.Error()))
				} else if retriedWindow, retryVerifyErr := window.GetManagedWindow(updatedWindow.Id, updatedWindow.Pid, updatedWindow.Title); retryVerifyErr != nil {
					p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("window manager failed to verify retried group window move: group=%s app=%s identity=%s windowId=%s pid=%d err=%s", group.Id, placement.AppName, placement.Identity, placement.Window.Id, placement.Window.Pid, retryVerifyErr.Error()))
				} else {
					p.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("window manager retried group rect: group=%s app=%s identity=%s target=%+v after=%+v", group.Id, placement.AppName, placement.Identity, placement.Rect, retriedWindow.Bounds))
					if !windowRectApproximatelyEqual(retriedWindow.Bounds, placement.Rect) {
						p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("window manager group target mismatch after retry: group=%s app=%s identity=%s target=%+v after=%+v", group.Id, placement.AppName, placement.Identity, placement.Rect, retriedWindow.Bounds))
					}
				}
			}
		}
		if !window.ActivateWindowByPid(placement.Window.Pid) {
			p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("window manager failed to activate group window: group=%s app=%s identity=%s windowId=%s pid=%d", group.Id, placement.AppName, placement.Identity, placement.Window.Id, placement.Window.Pid))
		}
		summary.Moved++
	}

	return summary, nil
}

// buildWindowGroupPlacements converts configured screen slots into concrete desktop rectangles.
func (p *WindowManagerPlugin) buildWindowGroupPlacements(ctx context.Context, group windowManagerWindowGroup, displays []window.DisplayInfo, gap int) ([]windowGroupPlacement, error) {
	placements := []windowGroupPlacement{}
	seenIdentities := map[string]bool{}

	for _, screen := range group.Screens {
		display, ok := resolveWindowGroupDisplay(displays, screen)
		if !ok {
			p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("window manager group screen skipped, display not found: group=%s displayId=%s displayIndex=%d", group.Id, screen.DisplayId, screen.DisplayIndex))
			continue
		}

		layoutId := strings.TrimSpace(screen.Layout)
		if layoutId == "" {
			layoutId = windowGroupLayoutFull
		}

		for _, assignment := range screen.Assignments {
			identity := normalizeWindowGroupIdentity(assignment.App.Identity)
			if identity == "" {
				continue
			}
			if seenIdentities[identity] {
				p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("window manager group duplicate app skipped: group=%s identity=%s", group.Id, identity))
				continue
			}

			rect, ok := windowGroupSlotRect(layoutId, assignment.Slot, display.WorkArea, gap)
			if !ok {
				p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("window manager group slot skipped, layout not found: group=%s layout=%s slot=%s", group.Id, layoutId, assignment.Slot))
				continue
			}

			seenIdentities[identity] = true
			placements = append(placements, windowGroupPlacement{
				GroupName: group.Name,
				Identity:  identity,
				AppName:   windowGroupAppName(assignment.App),
				AppPath:   strings.TrimSpace(assignment.App.Path),
				Rect:      rect,
			})
		}
	}

	return placements, nil
}

// windowGroupSlotRect maps one stable layout slot id to a grid rectangle.
func windowGroupSlotRect(layoutId string, slotId string, area window.WindowRect, gap int) (window.WindowRect, bool) {
	layout, ok := windowGroupLayoutDefinitions[layoutId]
	if !ok {
		return window.WindowRect{}, false
	}

	slotId = strings.TrimSpace(slotId)
	if slotId == "" && len(layout.Slots) == 1 {
		slotId = layout.Slots[0].Id
	}

	for _, slot := range layout.Slots {
		if slot.Id == slotId {
			return gridRect(area, slot.Cols, slot.Rows, slot.Col, slot.Row, slot.ColSpan, slot.RowSpan, gap), true
		}
	}
	return window.WindowRect{}, false
}

// resolveWindowGroupDisplay uses display id first and falls back to the sorted display index.
func resolveWindowGroupDisplay(displays []window.DisplayInfo, screen windowManagerWindowGroupScreen) (window.DisplayInfo, bool) {
	displayId := strings.TrimSpace(screen.DisplayId)
	if displayId != "" {
		for _, display := range displays {
			if display.Id == displayId {
				return display, true
			}
		}
	}

	if screen.DisplayIndex >= 0 && screen.DisplayIndex < len(displays) {
		return displays[screen.DisplayIndex], true
	}
	return window.DisplayInfo{}, false
}

func findExactWindowGroup(groups []windowManagerWindowGroup, search string) (windowManagerWindowGroup, bool) {
	normalizedSearch := strings.ToLower(strings.TrimSpace(search))
	for _, group := range groups {
		if strings.ToLower(group.Id) == normalizedSearch || strings.ToLower(group.Name) == normalizedSearch {
			return group, true
		}
	}
	return windowManagerWindowGroup{}, false
}

func windowGroupMatches(ctx context.Context, group windowManagerWindowGroup, search string) (bool, int64) {
	search = strings.TrimSpace(search)
	if search == "" {
		return true, 100
	}

	candidates := []string{group.Name, group.Id}
	var bestScore int64
	for _, candidate := range candidates {
		matched, score := plugin.IsStringMatchScore(ctx, candidate, search)
		if matched && score > bestScore {
			bestScore = score
		}
	}
	return bestScore > 0, bestScore
}

func countWindowGroupApps(group windowManagerWindowGroup) int {
	count := 0
	for _, screen := range group.Screens {
		for _, assignment := range screen.Assignments {
			if normalizeWindowGroupIdentity(assignment.App.Identity) != "" {
				count++
			}
		}
	}
	return count
}

func indexManagedWindowsByIdentity(windows []window.ManagedWindow) map[string][]window.ManagedWindow {
	windowsByIdentity := map[string][]window.ManagedWindow{}
	for _, managedWindow := range windows {
		identity := normalizeWindowGroupIdentity(managedWindow.AppIdentity)
		if identity == "" {
			identity = normalizeWindowGroupIdentity(window.GetProcessIdentity(managedWindow.Pid))
		}
		if identity == "" {
			continue
		}
		windowsByIdentity[identity] = append(windowsByIdentity[identity], managedWindow)
	}
	return windowsByIdentity
}

func missingPlacementIdentities(placements []windowGroupPlacement, windowsByIdentity map[string][]window.ManagedWindow) map[string]bool {
	missing := map[string]bool{}
	for _, placement := range placements {
		if len(windowsByIdentity[placement.Identity]) == 0 {
			missing[placement.Identity] = true
		}
	}
	return missing
}

func waitForWindowGroupWindows(placements []windowGroupPlacement) ([]window.ManagedWindow, error) {
	deadline := time.Now().Add(windowGroupLaunchWaitTimeout)
	var lastWindows []window.ManagedWindow
	for {
		windows, err := window.ListManagedWindows()
		if err != nil {
			return nil, err
		}
		lastWindows = windows

		if len(missingPlacementIdentities(placements, indexManagedWindowsByIdentity(windows))) == 0 || time.Now().After(deadline) {
			return lastWindows, nil
		}
		time.Sleep(windowGroupLaunchPollInterval)
	}
}

func (p *WindowManagerPlugin) notifyWindowGroupSummary(ctx context.Context, group windowManagerWindowGroup, summary windowGroupApplySummary) {
	groupName := group.Name
	if strings.TrimSpace(groupName) == "" {
		groupName = group.Id
	}

	issues := []string{}
	if len(summary.MissingApps) > 0 {
		issues = append(issues, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_window_manager_group_missing_apps"), strings.Join(summary.MissingApps, ", ")))
	}
	if len(summary.LaunchFailures) > 0 {
		issues = append(issues, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_window_manager_group_launch_failed_apps"), strings.Join(summary.LaunchFailures, ", ")))
	}
	if len(summary.FailedApps) > 0 {
		issues = append(issues, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_window_manager_group_failed_apps"), strings.Join(summary.FailedApps, ", ")))
	}
	if len(summary.PermissionApps) > 0 {
		issues = append(issues, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_window_manager_group_permission_apps"), strings.Join(summary.PermissionApps, ", ")))
	}

	if summary.Moved == 0 && len(issues) == 0 {
		p.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_window_manager_group_no_apps"), groupName))
		return
	}

	if len(issues) > 0 {
		p.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_window_manager_group_partial"), summary.Moved, groupName, strings.Join(issues, "; ")))
		return
	}

	p.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_window_manager_group_applied"), groupName))
}

func normalizeWindowGroupIdentity(identity string) string {
	return strings.ToLower(strings.TrimSpace(identity))
}

func windowGroupAppName(app setting.IgnoredHotkeyApp) string {
	if name := strings.TrimSpace(app.Name); name != "" {
		return name
	}
	if identity := strings.TrimSpace(app.Identity); identity != "" {
		return identity
	}
	return strings.TrimSpace(app.Path)
}

func appendUniqueString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if strings.EqualFold(existing, value) {
			return values
		}
	}
	return append(values, value)
}
