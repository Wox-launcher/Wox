package window_manager

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/util/window"
)

const (
	windowManagerSettingGap    = "gap"
	windowManagerSettingGroups = "windowGroups"
	windowManagerCommandGroup  = "group"
	windowManagerDefaultGap    = 0
	windowManagerMaxGap        = 64
	windowManagerMoveTolerance = 2
)

type windowOperation string

const (
	operationLeftHalf           windowOperation = "left-half"
	operationRightHalf          windowOperation = "right-half"
	operationTopHalf            windowOperation = "top-half"
	operationBottomHalf         windowOperation = "bottom-half"
	operationTopLeftQuarter     windowOperation = "top-left-quarter"
	operationTopRightQuarter    windowOperation = "top-right-quarter"
	operationBottomLeftQuarter  windowOperation = "bottom-left-quarter"
	operationBottomRightQuarter windowOperation = "bottom-right-quarter"
	operationFirstThird         windowOperation = "first-third"
	operationCenterThird        windowOperation = "center-third"
	operationLastThird          windowOperation = "last-third"
	operationFirstTwoThirds     windowOperation = "first-two-thirds"
	operationLastTwoThirds      windowOperation = "last-two-thirds"
	operationMaximize           windowOperation = "maximize"
	operationAlmostMaximize     windowOperation = "almost-maximize"
	operationCenter             windowOperation = "center"
	operationReasonableSize     windowOperation = "reasonable-size"
	operationMaximizeHeight     windowOperation = "maximize-height"
	operationMaximizeWidth      windowOperation = "maximize-width"
	operationNextDisplay        windowOperation = "next-display"
	operationPreviousDisplay    windowOperation = "previous-display"
	operationMinimize           windowOperation = "minimize"
	operationRestore            windowOperation = "restore"
)

type windowManagerCommand struct {
	Command  string
	TitleKey string
	Aliases  []string
	Op       windowOperation
}

var windowManagerIcon = common.PluginWindowManagerIcon
var windowManagerCommands = []windowManagerCommand{
	{Command: "left-half", TitleKey: "plugin_window_manager_command_left_half", Aliases: []string{"left", "left half", "左半屏"}, Op: operationLeftHalf},
	{Command: "right-half", TitleKey: "plugin_window_manager_command_right_half", Aliases: []string{"right", "right half", "右半屏"}, Op: operationRightHalf},
	{Command: "top-half", TitleKey: "plugin_window_manager_command_top_half", Aliases: []string{"top", "top half", "上半屏"}, Op: operationTopHalf},
	{Command: "bottom-half", TitleKey: "plugin_window_manager_command_bottom_half", Aliases: []string{"bottom", "bottom half", "下半屏"}, Op: operationBottomHalf},
	{Command: "top-left-quarter", TitleKey: "plugin_window_manager_command_top_left_quarter", Aliases: []string{"top left", "upper left", "左上"}, Op: operationTopLeftQuarter},
	{Command: "top-right-quarter", TitleKey: "plugin_window_manager_command_top_right_quarter", Aliases: []string{"top right", "upper right", "右上"}, Op: operationTopRightQuarter},
	{Command: "bottom-left-quarter", TitleKey: "plugin_window_manager_command_bottom_left_quarter", Aliases: []string{"bottom left", "lower left", "左下"}, Op: operationBottomLeftQuarter},
	{Command: "bottom-right-quarter", TitleKey: "plugin_window_manager_command_bottom_right_quarter", Aliases: []string{"bottom right", "lower right", "右下"}, Op: operationBottomRightQuarter},
	{Command: "first-third", TitleKey: "plugin_window_manager_command_first_third", Aliases: []string{"left third", "first third", "左三分之一"}, Op: operationFirstThird},
	{Command: "center-third", TitleKey: "plugin_window_manager_command_center_third", Aliases: []string{"middle third", "center third", "中间三分之一"}, Op: operationCenterThird},
	{Command: "last-third", TitleKey: "plugin_window_manager_command_last_third", Aliases: []string{"right third", "last third", "右三分之一"}, Op: operationLastThird},
	{Command: "first-two-thirds", TitleKey: "plugin_window_manager_command_first_two_thirds", Aliases: []string{"left two thirds", "前两列", "左三分之二"}, Op: operationFirstTwoThirds},
	{Command: "last-two-thirds", TitleKey: "plugin_window_manager_command_last_two_thirds", Aliases: []string{"right two thirds", "后两列", "右三分之二"}, Op: operationLastTwoThirds},
	{Command: "maximize", TitleKey: "plugin_window_manager_command_maximize", Aliases: []string{"full screen", "maximize window", "最大化"}, Op: operationMaximize},
	{Command: "almost-maximize", TitleKey: "plugin_window_manager_command_almost_maximize", Aliases: []string{"almost full", "nearly maximize", "接近最大化"}, Op: operationAlmostMaximize},
	{Command: "center", TitleKey: "plugin_window_manager_command_center", Aliases: []string{"center window", "居中"}, Op: operationCenter},
	{Command: "reasonable-size", TitleKey: "plugin_window_manager_command_reasonable_size", Aliases: []string{"reasonable", "default size", "合适大小"}, Op: operationReasonableSize},
	{Command: "maximize-height", TitleKey: "plugin_window_manager_command_maximize_height", Aliases: []string{"full height", "最大高度"}, Op: operationMaximizeHeight},
	{Command: "maximize-width", TitleKey: "plugin_window_manager_command_maximize_width", Aliases: []string{"full width", "最大宽度"}, Op: operationMaximizeWidth},
	{Command: "next-display", TitleKey: "plugin_window_manager_command_next_display", Aliases: []string{"next monitor", "next screen", "下一个显示器"}, Op: operationNextDisplay},
	{Command: "previous-display", TitleKey: "plugin_window_manager_command_previous_display", Aliases: []string{"previous monitor", "previous screen", "上一个显示器"}, Op: operationPreviousDisplay},
	{Command: "minimize", TitleKey: "plugin_window_manager_command_minimize", Aliases: []string{"minimize window", "minimise", "最小化"}, Op: operationMinimize},
	{Command: "restore", TitleKey: "plugin_window_manager_command_restore", Aliases: []string{"restore window", "还原"}, Op: operationRestore},
}

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &WindowManagerPlugin{})
}

type WindowManagerPlugin struct {
	api      plugin.API
	restoreM sync.Mutex
	restore  map[string]window.WindowRect
}

func (p *WindowManagerPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "5b7d9f22-4d87-4c0f-a2c1-8e2b50c8bca0",
		Name:          "i18n:plugin_window_manager_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_window_manager_plugin_description",
		Icon:          windowManagerIcon.String(),
		TriggerKeywords: []string{
			"window",
			"*",
		},
		Commands: windowManagerMetadataCommands(),
		SupportedOS: []string{
			"Windows",
			"Macos",
		},
		SettingDefinitions: definition.PluginSettingDefinitions{
			{
				Type: definition.PluginSettingDefinitionTypeTextBox,
				Value: &definition.PluginSettingValueTextBox{
					Key:          windowManagerSettingGap,
					Label:        "i18n:plugin_window_manager_setting_gap",
					Tooltip:      "i18n:plugin_window_manager_setting_gap_tooltip",
					Suffix:       "i18n:plugin_window_manager_setting_gap_suffix",
					DefaultValue: strconv.Itoa(windowManagerDefaultGap),
					Validators: []validator.PluginSettingValidator{
						{
							Type:  validator.PluginSettingValidatorTypeNotEmpty,
							Value: &validator.PluginSettingValidatorNotEmpty{},
						},
						{
							Type: validator.PluginSettingValidatorTypeIsNumber,
							Value: &validator.PluginSettingValidatorIsNumber{
								IsInteger: true,
							},
						},
					},
				},
			},
			{
				Type:               definition.PluginSettingDefinitionTypeTable,
				IsPlatformSpecific: true,
				Value: &definition.PluginSettingValueTable{
					Key:          windowManagerSettingGroups,
					Title:        "i18n:plugin_window_manager_setting_groups",
					Tooltip:      "i18n:plugin_window_manager_setting_groups_tooltip",
					DefaultValue: "[]",
				},
			},
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureQueryEnv,
				Params: map[string]any{
					"requireActiveWindowName": true,
					"requireActiveWindowPid":  true,
					"requireActiveWindowId":   true,
					"requireActiveWindowIcon": true,
				},
			},
		},
	}
}

func (p *WindowManagerPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	p.api = initParams.API
	p.restore = make(map[string]window.WindowRect)
}

// Query lists available window layout commands or returns the explicitly parsed command.
func (p *WindowManagerPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	if strings.EqualFold(query.Command, windowManagerCommandGroup) {
		return p.queryWindowGroups(ctx, query)
	}

	var groupResults []plugin.QueryResult
	if query.Command == "" {
		groupResults = p.matchingWindowGroupResults(ctx, query.Search, !query.IsGlobalQuery())
	}

	if !hasActiveWindow(query.Env) {
		if len(groupResults) > 0 {
			return plugin.NewQueryResponse(groupResults)
		}
		if p.shouldShowNoActiveWindowResult(ctx, query) {
			return plugin.NewQueryResponse([]plugin.QueryResult{p.noActiveWindowResult()})
		}
		return plugin.QueryResponse{}
	}

	if query.Command != "" {
		command, ok := findWindowManagerCommand(query.Command)
		if !ok {
			return plugin.QueryResponse{}
		}
		return plugin.NewQueryResponse([]plugin.QueryResult{p.commandResult(ctx, query, command, 1000)})
	}

	results := make([]plugin.QueryResult, 0, len(windowManagerCommands))
	for _, command := range windowManagerCommands {
		if matched, score := p.commandMatches(ctx, command, query.Search); matched {
			results = append(results, p.commandResult(ctx, query, command, score))
		}
	}
	results = append(results, groupResults...)
	return plugin.NewQueryResponse(results)
}

// shouldShowNoActiveWindowResult keeps the explanatory row scoped to window-manager queries.
func (p *WindowManagerPlugin) shouldShowNoActiveWindowResult(ctx context.Context, query plugin.Query) bool {
	if query.Command != "" {
		return true
	}

	search := strings.TrimSpace(query.Search)
	if search == "" {
		return !query.IsGlobalQuery()
	}

	for _, command := range windowManagerCommands {
		if matched, _ := p.commandMatches(ctx, command, search); matched {
			return true
		}
	}
	return false
}

// noActiveWindowResult explains why the plugin cannot act on the current query.
func (p *WindowManagerPlugin) noActiveWindowResult() plugin.QueryResult {
	return plugin.QueryResult{
		Title:    "i18n:plugin_window_manager_no_active_window_title",
		SubTitle: "i18n:plugin_window_manager_no_active_window_subtitle",
		Icon:     windowManagerIcon,
	}
}

// commandResult captures the active-window query env so delayed actions still target the original window.
func (p *WindowManagerPlugin) commandResult(ctx context.Context, query plugin.Query, command windowManagerCommand, score int64) plugin.QueryResult {
	targetName := strings.TrimSpace(query.Env.ActiveWindowTitle)
	if targetName == "" {
		targetName = strconv.Itoa(query.Env.ActiveWindowPid)
	}
	subtitle := fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_window_manager_target_subtitle"), targetName)

	capturedEnv := query.Env
	capturedCommand := command
	return plugin.QueryResult{
		Title:    "i18n:" + command.TitleKey,
		SubTitle: subtitle,
		Icon:     windowManagerCommandIcon(command.Op),
		Score:    score,
		Tails:    targetWindowIconTail(query.Env.ActiveWindowIcon),
		Actions: []plugin.QueryResultAction{
			{
				Name:                   "i18n:plugin_window_manager_action_apply",
				IsDefault:              true,
				PreventHideAfterAction: true,
				Action: func(actionCtx context.Context, actionContext plugin.ActionContext) {
					p.applyCommand(actionCtx, capturedCommand, capturedEnv)
				},
			},
		},
	}
}

// commandMatches lets users search by translated title, command id, or common aliases.
func (p *WindowManagerPlugin) commandMatches(ctx context.Context, command windowManagerCommand, search string) (bool, int64) {
	search = strings.TrimSpace(search)
	if search == "" {
		return true, 100
	}

	candidates := append([]string{command.Command, i18n.GetI18nManager().TranslateWox(ctx, command.TitleKey)}, command.Aliases...)
	var bestScore int64
	for _, candidate := range candidates {
		matched, score := plugin.IsStringMatchScore(ctx, candidate, search)
		if matched && score > bestScore {
			bestScore = score
		}
	}
	return bestScore > 0, bestScore
}

const (
	layoutIconFill   = "#2563EB"
	layoutIconEmpty  = "#F8FAFC"
	layoutIconStroke = "#64748B"
)

type layoutIconRect struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// windowManagerCommandIcon renders the target layout as a compact SVG preview.
func windowManagerCommandIcon(operation windowOperation) common.WoxImage {
	switch operation {
	case operationLeftHalf:
		return layoutSummaryIcon(layoutIconRect{X: 8, Y: 8, Width: 16, Height: 32})
	case operationRightHalf:
		return layoutSummaryIcon(layoutIconRect{X: 24, Y: 8, Width: 16, Height: 32})
	case operationTopHalf:
		return layoutSummaryIcon(layoutIconRect{X: 8, Y: 8, Width: 32, Height: 16})
	case operationBottomHalf:
		return layoutSummaryIcon(layoutIconRect{X: 8, Y: 24, Width: 32, Height: 16})
	case operationTopLeftQuarter:
		return layoutSummaryIcon(layoutIconRect{X: 8, Y: 8, Width: 16, Height: 16})
	case operationTopRightQuarter:
		return layoutSummaryIcon(layoutIconRect{X: 24, Y: 8, Width: 16, Height: 16})
	case operationBottomLeftQuarter:
		return layoutSummaryIcon(layoutIconRect{X: 8, Y: 24, Width: 16, Height: 16})
	case operationBottomRightQuarter:
		return layoutSummaryIcon(layoutIconRect{X: 24, Y: 24, Width: 16, Height: 16})
	case operationFirstThird:
		return layoutSummaryIcon(layoutIconRect{X: 8, Y: 8, Width: 11, Height: 32})
	case operationCenterThird:
		return layoutSummaryIcon(layoutIconRect{X: 19, Y: 8, Width: 10, Height: 32})
	case operationLastThird:
		return layoutSummaryIcon(layoutIconRect{X: 29, Y: 8, Width: 11, Height: 32})
	case operationFirstTwoThirds:
		return layoutSummaryIcon(layoutIconRect{X: 8, Y: 8, Width: 21, Height: 32})
	case operationLastTwoThirds:
		return layoutSummaryIcon(layoutIconRect{X: 19, Y: 8, Width: 21, Height: 32})
	case operationMaximize:
		return layoutSummaryIcon(layoutIconRect{X: 8, Y: 8, Width: 32, Height: 32})
	case operationAlmostMaximize:
		return layoutSummaryIcon(layoutIconRect{X: 10, Y: 10, Width: 28, Height: 28})
	case operationCenter:
		return layoutSummaryIcon(layoutIconRect{X: 14, Y: 12, Width: 20, Height: 24})
	case operationReasonableSize:
		return layoutSummaryIcon(layoutIconRect{X: 12, Y: 12, Width: 24, Height: 24})
	case operationMaximizeHeight:
		return layoutSummaryIcon(layoutIconRect{X: 18, Y: 8, Width: 12, Height: 32})
	case operationMaximizeWidth:
		return layoutSummaryIcon(layoutIconRect{X: 8, Y: 18, Width: 32, Height: 12})
	case operationNextDisplay:
		return displayMoveIcon(true)
	case operationPreviousDisplay:
		return displayMoveIcon(false)
	case operationMinimize:
		return layoutSummaryIcon(layoutIconRect{X: 13, Y: 35, Width: 22, Height: 4})
	case operationRestore:
		return layoutSummaryIcon(layoutIconRect{X: 14, Y: 14, Width: 20, Height: 20})
	default:
		return windowManagerIcon
	}
}

// layoutSummaryIcon draws an empty desktop frame and fills the target area.
func layoutSummaryIcon(filledRects ...layoutIconRect) common.WoxImage {
	var filled strings.Builder
	for _, rect := range filledRects {
		filled.WriteString(layoutIconRectSvg(rect))
	}

	return common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48"><rect x="8" y="8" width="32" height="32" rx="5" fill="` + layoutIconEmpty + `"/>` + filled.String() + `<rect x="8" y="8" width="32" height="32" rx="5" fill="none" stroke="` + layoutIconStroke + `" stroke-width="2.5"/></svg>`)
}

// displayMoveIcon uses two frames because display movement is not a single-display layout.
func displayMoveIcon(next bool) common.WoxImage {
	targetX := 29.0
	arrowPath := "M20 24H28M25 21L28 24L25 27"
	if !next {
		targetX = 9
		arrowPath = "M28 24H20M23 21L20 24L23 27"
	}

	return common.NewWoxImageSvg(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48"><rect x="6" y="13" width="16" height="22" rx="3" fill="%s" stroke="%s" stroke-width="2.3"/><rect x="26" y="13" width="16" height="22" rx="3" fill="%s" stroke="%s" stroke-width="2.3"/><rect x="%s" y="19" width="10" height="10" rx="1.8" fill="%s"/><path d="%s" fill="none" stroke="%s" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"/></svg>`, layoutIconEmpty, layoutIconStroke, layoutIconEmpty, layoutIconStroke, svgNumber(targetX), layoutIconFill, arrowPath, layoutIconFill))
}

// layoutIconRectSvg converts a logical filled area into SVG markup.
func layoutIconRectSvg(rect layoutIconRect) string {
	return fmt.Sprintf(`<rect x="%s" y="%s" width="%s" height="%s" rx="1.8" fill="%s"/>`, svgNumber(rect.X), svgNumber(rect.Y), svgNumber(rect.Width), svgNumber(rect.Height), layoutIconFill)
}

func svgNumber(value float64) string {
	if value == math.Trunc(value) {
		return strconv.Itoa(int(value))
	}
	return strconv.FormatFloat(value, 'f', 1, 64)
}

// targetWindowIconTail shows which captured window will be adjusted without taking over the main layout icon.
func targetWindowIconTail(icon common.WoxImage) []plugin.QueryResultTail {
	if icon.IsEmpty() {
		return nil
	}

	size := 18.0
	return []plugin.QueryResultTail{
		{
			Type:        plugin.QueryResultTailTypeImage,
			Image:       icon,
			ImageWidth:  &size,
			ImageHeight: &size,
		},
	}
}

// applyCommand hides Wox before moving the captured window so the launcher never becomes the target.
func (p *WindowManagerPlugin) applyCommand(ctx context.Context, command windowManagerCommand, env plugin.QueryEnv) {
	p.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("window manager apply command: command=%s op=%s activeWindowId=%s activeWindowPid=%d activeWindowTitle=%q", command.Command, command.Op, env.ActiveWindowId, env.ActiveWindowPid, env.ActiveWindowTitle))

	p.api.HideApp(ctx)
	time.Sleep(120 * time.Millisecond)

	managedWindow, err := window.GetManagedWindow(env.ActiveWindowId, env.ActiveWindowPid, env.ActiveWindowTitle)
	if err != nil {
		p.notifyFailure(ctx, err)
		return
	}
	p.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("window manager resolved target: id=%s pid=%d title=%q bounds=%+v displayId=%s workArea=%+v", managedWindow.Id, managedWindow.Pid, managedWindow.Title, managedWindow.Bounds, managedWindow.Display.Id, managedWindow.Display.WorkArea))

	if command.Op == operationRestore {
		p.restoreWindow(ctx, managedWindow)
		return
	}

	if command.Op == operationMinimize {
		p.storeRestoreRect(managedWindow, managedWindow.Bounds)
		if err := window.MinimizeWindow(managedWindow); err != nil {
			p.notifyFailure(ctx, err)
		}
		return
	}

	if command.Op == operationMaximize {
		p.storeRestoreRect(managedWindow, managedWindow.Bounds)
		if err := window.MaximizeWindow(managedWindow); err == nil {
			return
		} else if !errors.Is(err, window.ErrWindowManagementUnsupported) {
			p.notifyFailure(ctx, err)
			return
		}
	}

	gap := p.getGap(ctx)
	targetRect, err := p.targetRect(ctx, command.Op, managedWindow, gap)
	if err != nil {
		p.notifyFailure(ctx, err)
		return
	}
	p.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("window manager target rect: command=%s gap=%d target=%+v", command.Command, gap, targetRect))

	p.storeRestoreRect(managedWindow, managedWindow.Bounds)
	if err := window.MoveResizeWindow(managedWindow, targetRect); err != nil {
		p.notifyFailure(ctx, err)
		return
	}

	updatedWindow, err := window.GetManagedWindow(managedWindow.Id, managedWindow.Pid, managedWindow.Title)
	if err != nil {
		p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("window manager failed to verify moved window: id=%s pid=%d err=%s", managedWindow.Id, managedWindow.Pid, err.Error()))
		return
	}
	p.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("window manager applied rect: command=%s before=%+v target=%+v after=%+v", command.Command, managedWindow.Bounds, targetRect, updatedWindow.Bounds))
	if !windowRectApproximatelyEqual(updatedWindow.Bounds, targetRect) {
		p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("window manager target mismatch after move: command=%s target=%+v after=%+v", command.Command, targetRect, updatedWindow.Bounds))
	}
}

// restoreWindow swaps the saved frame with the current frame so Restore can toggle once a frame exists.
func (p *WindowManagerPlugin) restoreWindow(ctx context.Context, managedWindow window.ManagedWindow) {
	key := restoreKey(managedWindow)

	p.restoreM.Lock()
	rect, ok := p.restore[key]
	if ok {
		p.restore[key] = managedWindow.Bounds
	}
	p.restoreM.Unlock()

	if !ok {
		p.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_window_manager_restore_missing"))
		return
	}

	if err := window.MoveResizeWindow(managedWindow, rect); err != nil {
		p.notifyFailure(ctx, err)
	}
}

// storeRestoreRect keeps restore state process-local by design; it is not persisted across Wox restarts.
func (p *WindowManagerPlugin) storeRestoreRect(managedWindow window.ManagedWindow, rect window.WindowRect) {
	p.restoreM.Lock()
	defer p.restoreM.Unlock()
	p.restore[restoreKey(managedWindow)] = rect
}

// getGap applies a runtime clamp because settings validators only enforce numeric input.
func (p *WindowManagerPlugin) getGap(ctx context.Context) int {
	value := strings.TrimSpace(p.api.GetSetting(ctx, windowManagerSettingGap))
	gap, err := strconv.Atoi(value)
	if err != nil {
		return windowManagerDefaultGap
	}
	return clamp(gap, 0, windowManagerMaxGap)
}

// notifyFailure maps platform errors to user-facing messages while keeping the detailed error in logs.
func (p *WindowManagerPlugin) notifyFailure(ctx context.Context, err error) {
	switch {
	case errors.Is(err, window.ErrWindowManagementPermissionDenied):
		p.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_window_manager_permission_denied"))
	case errors.Is(err, window.ErrWindowManagementNoAdjacentDisplay):
		p.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_window_manager_no_adjacent_display"))
	default:
		p.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_window_manager_action_failed"), err.Error()))
	}
	p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("window manager command failed: %s", err.Error()))
}

// targetRect converts a semantic command into a concrete desktop rectangle.
func (p *WindowManagerPlugin) targetRect(ctx context.Context, operation windowOperation, managedWindow window.ManagedWindow, gap int) (window.WindowRect, error) {
	workArea := managedWindow.Display.WorkArea
	if workArea.Width <= 0 || workArea.Height <= 0 {
		return window.WindowRect{}, window.ErrWindowManagementDisplayNotFound
	}

	switch operation {
	case operationLeftHalf:
		return gridRect(workArea, 2, 1, 0, 0, 1, 1, gap), nil
	case operationRightHalf:
		return gridRect(workArea, 2, 1, 1, 0, 1, 1, gap), nil
	case operationTopHalf:
		return gridRect(workArea, 1, 2, 0, 0, 1, 1, gap), nil
	case operationBottomHalf:
		return gridRect(workArea, 1, 2, 0, 1, 1, 1, gap), nil
	case operationTopLeftQuarter:
		return gridRect(workArea, 2, 2, 0, 0, 1, 1, gap), nil
	case operationTopRightQuarter:
		return gridRect(workArea, 2, 2, 1, 0, 1, 1, gap), nil
	case operationBottomLeftQuarter:
		return gridRect(workArea, 2, 2, 0, 1, 1, 1, gap), nil
	case operationBottomRightQuarter:
		return gridRect(workArea, 2, 2, 1, 1, 1, 1, gap), nil
	case operationFirstThird:
		return gridRect(workArea, 3, 1, 0, 0, 1, 1, gap), nil
	case operationCenterThird:
		return gridRect(workArea, 3, 1, 1, 0, 1, 1, gap), nil
	case operationLastThird:
		return gridRect(workArea, 3, 1, 2, 0, 1, 1, gap), nil
	case operationFirstTwoThirds:
		return gridRect(workArea, 3, 1, 0, 0, 2, 1, gap), nil
	case operationLastTwoThirds:
		return gridRect(workArea, 3, 1, 1, 0, 2, 1, gap), nil
	case operationMaximize:
		return insetRect(workArea, gap), nil
	case operationAlmostMaximize:
		return insetRect(workArea, max(gap, 24)), nil
	case operationCenter:
		return centerCurrentRect(managedWindow.Bounds, insetRect(workArea, gap)), nil
	case operationReasonableSize:
		return centeredRatioRect(insetRect(workArea, gap), 0.7, 0.75), nil
	case operationMaximizeHeight:
		return maximizeHeightRect(managedWindow.Bounds, insetRect(workArea, gap)), nil
	case operationMaximizeWidth:
		return maximizeWidthRect(managedWindow.Bounds, insetRect(workArea, gap)), nil
	case operationNextDisplay:
		return adjacentDisplayRect(managedWindow, gap, true)
	case operationPreviousDisplay:
		return adjacentDisplayRect(managedWindow, gap, false)
	default:
		return window.WindowRect{}, fmt.Errorf("unsupported window operation: %s", operation)
	}
}

// findWindowManagerCommand resolves command metadata parsed by the Wox query splitter.
func findWindowManagerCommand(commandName string) (windowManagerCommand, bool) {
	for _, command := range windowManagerCommands {
		if strings.EqualFold(command.Command, commandName) {
			return command, true
		}
	}
	return windowManagerCommand{}, false
}

// hasActiveWindow accepts either exact window id or process id for best-effort fallback.
func hasActiveWindow(env plugin.QueryEnv) bool {
	return strings.TrimSpace(env.ActiveWindowId) != "" || env.ActiveWindowPid > 0
}

// windowRectApproximatelyEqual allows small macOS Accessibility rounding differences.
func windowRectApproximatelyEqual(actual window.WindowRect, expected window.WindowRect) bool {
	return absInt(actual.X-expected.X) <= windowManagerMoveTolerance &&
		absInt(actual.Y-expected.Y) <= windowManagerMoveTolerance &&
		absInt(actual.Width-expected.Width) <= windowManagerMoveTolerance &&
		absInt(actual.Height-expected.Height) <= windowManagerMoveTolerance
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// restoreKey prefers the platform window id so multiple windows from one app do not share restore state.
func restoreKey(managedWindow window.ManagedWindow) string {
	if strings.TrimSpace(managedWindow.Id) != "" {
		return managedWindow.Id
	}
	return fmt.Sprintf("%d:%s", managedWindow.Pid, managedWindow.Title)
}

// gridRect divides a work area into a gap-aware grid and returns the requested cell span.
func gridRect(area window.WindowRect, cols int, rows int, col int, row int, colSpan int, rowSpan int, gap int) window.WindowRect {
	area = insetRect(area, gap)
	if cols <= 0 || rows <= 0 {
		return area
	}

	usableWidth := max(1, area.Width-gap*(cols-1))
	usableHeight := max(1, area.Height-gap*(rows-1))
	startX := roundToInt(float64(usableWidth) * float64(col) / float64(cols))
	endX := roundToInt(float64(usableWidth) * float64(col+colSpan) / float64(cols))
	startY := roundToInt(float64(usableHeight) * float64(row) / float64(rows))
	endY := roundToInt(float64(usableHeight) * float64(row+rowSpan) / float64(rows))

	return window.WindowRect{
		X:      area.X + startX + gap*col,
		Y:      area.Y + startY + gap*row,
		Width:  max(1, endX-startX+gap*(colSpan-1)),
		Height: max(1, endY-startY+gap*(rowSpan-1)),
	}
}

// insetRect applies the user gap while preserving at least one desktop unit in each dimension.
func insetRect(rect window.WindowRect, gap int) window.WindowRect {
	gap = clamp(gap, 0, max(0, min(rect.Width-1, rect.Height-1)/2))
	return window.WindowRect{
		X:      rect.X + gap,
		Y:      rect.Y + gap,
		Width:  max(1, rect.Width-gap*2),
		Height: max(1, rect.Height-gap*2),
	}
}

// centerCurrentRect keeps the current size when possible and centers it in the target area.
func centerCurrentRect(current window.WindowRect, area window.WindowRect) window.WindowRect {
	width := min(max(1, current.Width), area.Width)
	height := min(max(1, current.Height), area.Height)
	return window.WindowRect{
		X:      area.X + (area.Width-width)/2,
		Y:      area.Y + (area.Height-height)/2,
		Width:  width,
		Height: height,
	}
}

// centeredRatioRect creates centered preset sizes such as Reasonable Size.
func centeredRatioRect(area window.WindowRect, widthRatio float64, heightRatio float64) window.WindowRect {
	width := clamp(roundToInt(float64(area.Width)*widthRatio), 1, area.Width)
	height := clamp(roundToInt(float64(area.Height)*heightRatio), 1, area.Height)
	return window.WindowRect{
		X:      area.X + (area.Width-width)/2,
		Y:      area.Y + (area.Height-height)/2,
		Width:  width,
		Height: height,
	}
}

// maximizeHeightRect stretches vertically while preserving horizontal placement as much as possible.
func maximizeHeightRect(current window.WindowRect, area window.WindowRect) window.WindowRect {
	width := min(max(1, current.Width), area.Width)
	return window.WindowRect{
		X:      clamp(current.X, area.X, area.X+area.Width-width),
		Y:      area.Y,
		Width:  width,
		Height: area.Height,
	}
}

// maximizeWidthRect stretches horizontally while preserving vertical placement as much as possible.
func maximizeWidthRect(current window.WindowRect, area window.WindowRect) window.WindowRect {
	height := min(max(1, current.Height), area.Height)
	return window.WindowRect{
		X:      area.X,
		Y:      clamp(current.Y, area.Y, area.Y+area.Height-height),
		Width:  area.Width,
		Height: height,
	}
}

// adjacentDisplayRect preserves the window's relative frame when moving between displays.
func adjacentDisplayRect(managedWindow window.ManagedWindow, gap int, next bool) (window.WindowRect, error) {
	displays, err := window.ListDisplays()
	if err != nil {
		return window.WindowRect{}, err
	}
	if len(displays) < 2 {
		return window.WindowRect{}, window.ErrWindowManagementNoAdjacentDisplay
	}

	currentIndex := findCurrentDisplayIndex(displays, managedWindow)
	targetIndex := currentIndex - 1
	if next {
		targetIndex = currentIndex + 1
	}
	if targetIndex < 0 {
		targetIndex = len(displays) - 1
	}
	if targetIndex >= len(displays) {
		targetIndex = 0
	}

	source := managedWindow.Display.WorkArea
	target := insetRect(displays[targetIndex].WorkArea, gap)
	if source.Width <= 0 || source.Height <= 0 {
		return centerCurrentRect(managedWindow.Bounds, target), nil
	}

	targetRect := window.WindowRect{
		X:      target.X + roundToInt(float64(managedWindow.Bounds.X-source.X)*float64(target.Width)/float64(source.Width)),
		Y:      target.Y + roundToInt(float64(managedWindow.Bounds.Y-source.Y)*float64(target.Height)/float64(source.Height)),
		Width:  roundToInt(float64(managedWindow.Bounds.Width) * float64(target.Width) / float64(source.Width)),
		Height: roundToInt(float64(managedWindow.Bounds.Height) * float64(target.Height) / float64(source.Height)),
	}
	return clampRectToArea(targetRect, target), nil
}

// findCurrentDisplayIndex falls back to the window center when platform display ids differ.
func findCurrentDisplayIndex(displays []window.DisplayInfo, managedWindow window.ManagedWindow) int {
	for index, display := range displays {
		if display.Id != "" && display.Id == managedWindow.Display.Id {
			return index
		}
	}

	centerX := managedWindow.Bounds.X + managedWindow.Bounds.Width/2
	centerY := managedWindow.Bounds.Y + managedWindow.Bounds.Height/2
	for index, display := range displays {
		if centerX >= display.WorkArea.X && centerX < display.WorkArea.X+display.WorkArea.Width &&
			centerY >= display.WorkArea.Y && centerY < display.WorkArea.Y+display.WorkArea.Height {
			return index
		}
	}
	return 0
}

// clampRectToArea keeps platform move requests inside the target display work area.
func clampRectToArea(rect window.WindowRect, area window.WindowRect) window.WindowRect {
	rect.Width = min(max(1, rect.Width), area.Width)
	rect.Height = min(max(1, rect.Height), area.Height)
	rect.X = clamp(rect.X, area.X, area.X+area.Width-rect.Width)
	rect.Y = clamp(rect.Y, area.Y, area.Y+area.Height-rect.Height)
	return rect
}

func roundToInt(value float64) int {
	return int(math.Round(value))
}

func clamp(value int, minValue int, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
