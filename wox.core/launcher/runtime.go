package launcher

import (
	"context"
	"sync"
	"wox/common"
	"wox/launcher/platform"
	launchertheme "wox/launcher/theme"
	"wox/plugin"
	"wox/util"

	"github.com/google/uuid"
)

type WindowShellRuntimeOptions struct {
	OnUserQueryChanged     func(ctx context.Context, query common.PlainQuery) error
	OnSelectedResultAction func(ctx context.Context, queryID string, resultID string, actionID string) error
}

type queryChangeEnvelope struct {
	ctx   context.Context
	query common.PlainQuery
}

// Runtime is the launcher-window contract consumed by the backend UI bridge.
// The first slice only needs launcher shell operations; query/results/preview
// contracts will be added incrementally as the native runtime grows.
type Runtime interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Show(ctx context.Context, showContext common.ShowContext)
	Hide(ctx context.Context)
	Toggle(ctx context.Context, showContext common.ShowContext)
	ChangeQuery(ctx context.Context, query common.PlainQuery)
	RefreshQuery(ctx context.Context, preserveSelectedIndex bool)
	ChangeTheme(ctx context.Context, theme common.Theme)
	PushResults(ctx context.Context, payload interface{}) bool
}

type NoopRuntime struct{}

func (n *NoopRuntime) Start(ctx context.Context) error { return nil }
func (n *NoopRuntime) Stop(ctx context.Context) error  { return nil }
func (n *NoopRuntime) Show(ctx context.Context, showContext common.ShowContext) {
}
func (n *NoopRuntime) Hide(ctx context.Context) {}
func (n *NoopRuntime) Toggle(ctx context.Context, showContext common.ShowContext) {
}
func (n *NoopRuntime) ChangeQuery(ctx context.Context, query common.PlainQuery) {}
func (n *NoopRuntime) RefreshQuery(ctx context.Context, preserveSelectedIndex bool) {
}
func (n *NoopRuntime) ChangeTheme(ctx context.Context, theme common.Theme)       {}
func (n *NoopRuntime) PushResults(ctx context.Context, payload interface{}) bool { return false }

type WindowShellRuntime struct {
	host      platform.Host
	textInput platform.TextInputHost
	mu        sync.RWMutex

	lastShowContext common.ShowContext
	query           common.PlainQuery
	results         platform.ResultListState
	paintTheme      launchertheme.PaintTheme
	sessionID       string

	onUserQueryChanged func(ctx context.Context, query common.PlainQuery) error
	onSelectedAction   func(ctx context.Context, queryID string, resultID string, actionID string) error
}

type DebugSnapshot struct {
	Host      platform.HostDebugSnapshot
	TextInput platform.TextInputDebugSnapshot
	Query     common.PlainQuery
	Results   platform.ResultListState
}

const (
	defaultQueryBoxFrameX      = 24
	defaultQueryBoxFrameY      = 20
	defaultQueryBoxFrameHeight = 48
	defaultResultListFrameY    = 84
	defaultShellPaddingX       = 24
	defaultShellHeight         = 420
	defaultShellWidth          = 760
)

func NewWindowShellRuntime(host platform.Host) *WindowShellRuntime {
	return NewWindowShellRuntimeWithBundleAndOptions(platform.Bundle{
		Host:      host,
		TextInput: &platform.NoopTextInputHost{},
	}, WindowShellRuntimeOptions{})
}

func NewWindowShellRuntimeWithBundle(bundle platform.Bundle) *WindowShellRuntime {
	return NewWindowShellRuntimeWithBundleAndOptions(bundle, WindowShellRuntimeOptions{})
}

func NewWindowShellRuntimeWithBundleAndOptions(bundle platform.Bundle, options WindowShellRuntimeOptions) *WindowShellRuntime {
	textInput := bundle.TextInput
	if textInput == nil {
		textInput = &platform.NoopTextInputHost{}
	}

	return &WindowShellRuntime{
		host:               bundle.Host,
		textInput:          textInput,
		paintTheme:         launchertheme.DefaultPaintTheme(),
		sessionID:          "core-" + uuid.NewString(),
		onUserQueryChanged: options.OnUserQueryChanged,
		onSelectedAction:   options.OnSelectedResultAction,
	}
}

func (r *WindowShellRuntime) Start(ctx context.Context) error {
	if r.host != nil {
		if err := r.host.Start(ctx, platform.StartOptions{
			Appearance: platform.WindowAppearance{
				Transparent:    true,
				Acrylic:        true,
				RoundedCorners: true,
			},
		}); err != nil {
			return err
		}
	}

	if r.textInput != nil {
		if err := r.textInput.Start(ctx); err != nil {
			if r.host != nil {
				_ = r.host.Stop(ctx)
			}
			return err
		}

		if observable, ok := r.textInput.(platform.ObservableTextInputHost); ok {
			if err := observable.SetChangeHandler(ctx, r.handleTextInputChanged); err != nil {
				_ = r.textInput.Stop(ctx)
				if r.host != nil {
					_ = r.host.Stop(ctx)
				}
				return err
			}
		}
		if navigable, ok := r.textInput.(platform.NavigableTextInputHost); ok {
			if err := navigable.SetSelectionNavigationHandler(ctx, r.handleSelectionNavigation); err != nil {
				_ = r.textInput.Stop(ctx)
				if r.host != nil {
					_ = r.host.Stop(ctx)
				}
				return err
			}
		}
		if submitCapable, ok := r.textInput.(platform.SubmitCapableTextInputHost); ok {
			if err := submitCapable.SetSubmitHandler(ctx, r.handleSelectedResultSubmit); err != nil {
				_ = r.textInput.Stop(ctx)
				if r.host != nil {
					_ = r.host.Stop(ctx)
				}
				return err
			}
		}
	}

	return nil
}

func (r *WindowShellRuntime) Stop(ctx context.Context) error {
	if r.textInput != nil {
		if err := r.textInput.Stop(ctx); err != nil {
			return err
		}
	}

	if r.host == nil {
		return nil
	}

	return r.host.Stop(ctx)
}

func (r *WindowShellRuntime) Show(ctx context.Context, showContext common.ShowContext) {
	if r.host == nil {
		return
	}

	r.mu.Lock()
	r.lastShowContext = showContext
	query := r.query
	queryBox := buildQueryBoxState(showContext, query)
	results := buildResultListState(showContext, r.results.Items)
	paintTheme := r.paintTheme
	r.mu.Unlock()

	if err := r.host.Show(ctx, platform.ShowRequest{
		ShowContext: showContext,
		Query:       query,
		QueryBox:    queryBox,
		Results:     results,
		Theme:       paintTheme,
	}); err != nil {
		util.GetLogger().Error(ctx, "launcher shell show failed: "+err.Error())
	}

	r.syncTextInputState(ctx, queryBox)
}

func (r *WindowShellRuntime) Hide(ctx context.Context) {
	if r.textInput != nil {
		if err := r.textInput.Blur(ctx); err != nil {
			util.GetLogger().Error(ctx, "launcher text input blur failed: "+err.Error())
		}
	}

	if r.host == nil {
		return
	}

	if err := r.host.Hide(ctx); err != nil {
		util.GetLogger().Error(ctx, "launcher shell hide failed: "+err.Error())
	}
}

func (r *WindowShellRuntime) Toggle(ctx context.Context, showContext common.ShowContext) {
	if r.host == nil {
		return
	}

	if r.host.IsVisible(ctx) {
		r.Hide(ctx)
		return
	}

	r.Show(ctx, showContext)
}

func (r *WindowShellRuntime) ChangeQuery(ctx context.Context, query common.PlainQuery) {
	if r.host == nil {
		return
	}

	r.mu.Lock()
	r.query = query
	r.results = buildResultListState(r.lastShowContext, nil)
	showContext := r.lastShowContext
	queryBox := buildQueryBoxState(showContext, query)
	results := r.results
	paintTheme := r.paintTheme
	r.mu.Unlock()

	if !r.host.IsVisible(ctx) {
		return
	}

	if err := r.host.Show(ctx, platform.ShowRequest{
		ShowContext: showContext,
		Query:       query,
		QueryBox:    queryBox,
		Results:     results,
		Theme:       paintTheme,
	}); err != nil {
		util.GetLogger().Error(ctx, "launcher shell query refresh failed: "+err.Error())
	}

	r.syncTextInputState(ctx, queryBox)
}

func (r *WindowShellRuntime) RefreshQuery(ctx context.Context, preserveSelectedIndex bool) {
	if r.host == nil {
		return
	}

	r.mu.RLock()
	showContext := r.lastShowContext
	query := r.query
	queryBox := buildQueryBoxState(showContext, query)
	results := r.results
	paintTheme := r.paintTheme
	r.mu.RUnlock()

	if !r.host.IsVisible(ctx) {
		return
	}

	if err := r.host.Show(ctx, platform.ShowRequest{
		ShowContext: showContext,
		Query:       query,
		QueryBox:    queryBox,
		Results:     results,
		Theme:       paintTheme,
	}); err != nil {
		util.GetLogger().Error(ctx, "launcher shell refresh failed: "+err.Error())
	}

	r.syncTextInputState(ctx, queryBox)
}

func (r *WindowShellRuntime) ChangeTheme(ctx context.Context, theme common.Theme) {
	r.mu.Lock()
	r.paintTheme = launchertheme.MapCommonTheme(theme)
	showContext := r.lastShowContext
	query := r.query
	queryBox := buildQueryBoxState(showContext, query)
	results := r.results
	paintTheme := r.paintTheme
	r.mu.Unlock()

	if r.host == nil || !r.host.IsVisible(ctx) {
		return
	}

	if err := r.host.Show(ctx, platform.ShowRequest{
		ShowContext: showContext,
		Query:       query,
		QueryBox:    queryBox,
		Results:     results,
		Theme:       paintTheme,
	}); err != nil {
		util.GetLogger().Error(ctx, "launcher shell theme refresh failed: "+err.Error())
	}

	r.syncTextInputState(ctx, queryBox)
}

func (r *WindowShellRuntime) PushResults(ctx context.Context, payload interface{}) bool {
	pushPayload, ok := payload.(plugin.PushResultsPayload)
	if !ok {
		return false
	}

	r.mu.Lock()
	if pushPayload.QueryId != r.query.QueryId {
		r.mu.Unlock()
		return false
	}

	r.results = buildResultListState(r.lastShowContext, mapQueryResults(pushPayload.Results))
	showContext := r.lastShowContext
	query := r.query
	queryBox := buildQueryBoxState(showContext, query)
	results := r.results
	paintTheme := r.paintTheme
	r.mu.Unlock()

	if r.host == nil || !r.host.IsVisible(ctx) {
		return true
	}

	if err := r.host.Show(ctx, platform.ShowRequest{
		ShowContext: showContext,
		Query:       query,
		QueryBox:    queryBox,
		Results:     results,
		Theme:       paintTheme,
	}); err != nil {
		util.GetLogger().Error(ctx, "launcher shell results refresh failed: "+err.Error())
		return false
	}

	r.syncTextInputState(ctx, queryBox)
	return true
}

func buildQueryBoxState(showContext common.ShowContext, query common.PlainQuery) platform.QueryBoxState {
	visible := !showContext.HideQueryBox
	return platform.QueryBoxState{
		Visible:      visible,
		Text:         query.String(),
		Placeholder:  platform.DefaultQueryBoxPlaceholder,
		HasFocus:     visible,
		CaretVisible: visible,
		Frame:        buildQueryBoxFrame(showContext),
	}
}

func buildQueryBoxFrame(showContext common.ShowContext) platform.Rect {
	width := showContext.WindowWidth
	if width <= 0 {
		width = defaultShellWidth
	}

	frame := platform.Rect{
		X:      defaultShellPaddingX,
		Y:      defaultQueryBoxFrameY,
		Width:  width - (defaultShellPaddingX * 2),
		Height: defaultQueryBoxFrameHeight,
	}

	if showContext.WindowPosition != nil {
		frame.X = showContext.WindowPosition.X + defaultShellPaddingX
		frame.Y = showContext.WindowPosition.Y + defaultQueryBoxFrameY
	}

	if frame.Width < 0 {
		frame.Width = 0
	}

	return frame
}

func buildResultListState(showContext common.ShowContext, items []platform.ResultListItem) platform.ResultListState {
	frame := buildResultListFrame(showContext)
	state := platform.ResultListState{
		Visible: frame.Height > 0 && len(items) > 0,
		Frame:   frame,
		Items:   append([]platform.ResultListItem(nil), items...),
	}
	if len(state.Items) > 0 {
		state.SelectedIndex = firstSelectableIndex(state.Items)
	}
	return state
}

func buildResultListFrame(showContext common.ShowContext) platform.Rect {
	width := showContext.WindowWidth
	if width <= 0 {
		width = defaultShellWidth
	}

	frame := platform.Rect{
		X:      defaultShellPaddingX,
		Y:      defaultResultListFrameY,
		Width:  width - (defaultShellPaddingX * 2),
		Height: defaultShellHeight - defaultResultListFrameY - defaultShellPaddingX,
	}

	if showContext.WindowPosition != nil {
		frame.X = showContext.WindowPosition.X + defaultShellPaddingX
		frame.Y = showContext.WindowPosition.Y + defaultResultListFrameY
	}

	if frame.Width < 0 {
		frame.Width = 0
	}
	if frame.Height < 0 {
		frame.Height = 0
	}

	return frame
}

func mapQueryResults(results []plugin.QueryResultUI) []platform.ResultListItem {
	mapped := make([]platform.ResultListItem, 0, len(results))
	for _, result := range results {
		mapped = append(mapped, platform.ResultListItem{
			QueryID:  result.QueryId,
			ResultID: result.Id,
			ActionID: resolveDefaultActionID(result),
			Title:    result.Title,
			Subtitle: result.SubTitle,
			IsGroup:  result.IsGroup,
		})
	}
	return mapped
}

func firstSelectableIndex(items []platform.ResultListItem) int {
	for index, item := range items {
		if !item.IsGroup {
			return index
		}
	}
	return 0
}

func moveSelectableIndex(items []platform.ResultListItem, current int, delta int) int {
	if len(items) == 0 || delta == 0 {
		return current
	}

	step := 1
	if delta < 0 {
		step = -1
	}

	index := current
	if index < 0 || index >= len(items) {
		index = firstSelectableIndex(items)
	}

	for remaining := abs(delta); remaining > 0; {
		next := index + step
		if next < 0 || next >= len(items) {
			return index
		}
		index = next
		if items[index].IsGroup {
			continue
		}
		remaining--
	}

	return index
}

func resolveDefaultActionID(result plugin.QueryResultUI) string {
	for _, action := range result.Actions {
		if action.IsDefault {
			return action.Id
		}
	}
	if len(result.Actions) > 0 {
		return result.Actions[0].Id
	}
	return ""
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func (r *WindowShellRuntime) syncTextInputState(ctx context.Context, queryBox platform.QueryBoxState) {
	if r.textInput == nil {
		return
	}

	if parentHost, ok := r.textInput.(platform.ParentWindowHost); ok {
		allowEmbedded := true
		if support, ok := r.host.(platform.EmbeddedTextInputSupport); ok {
			allowEmbedded = support.SupportsEmbeddedTextInput(ctx)
		}

		if allowEmbedded {
			if provider, ok := r.host.(platform.NativeWindowProvider); ok {
				if err := parentHost.SetParentWindow(ctx, provider.NativeWindowHandle(ctx)); err != nil {
					util.GetLogger().Error(ctx, "launcher text input parent attach failed: "+err.Error())
				}
			}
		}
	}

	state := platform.TextInputState{
		QueryBox:       queryBox,
		SelectionStart: len(queryBox.Text),
		SelectionEnd:   len(queryBox.Text),
	}

	if err := r.textInput.UpdateState(ctx, state); err != nil {
		util.GetLogger().Error(ctx, "launcher text input update failed: "+err.Error())
		return
	}

	if queryBox.Visible && queryBox.HasFocus {
		if err := r.textInput.Focus(ctx); err != nil {
			util.GetLogger().Error(ctx, "launcher text input focus failed: "+err.Error())
		}
		return
	}

	if err := r.textInput.Blur(ctx); err != nil {
		util.GetLogger().Error(ctx, "launcher text input blur failed: "+err.Error())
	}
}

func (r *WindowShellRuntime) handleTextInputChanged(_ context.Context, state platform.TextInputState) {
	if !state.QueryBox.Visible {
		return
	}

	envelope, shouldNotify := r.updateQueryFromTextInput(state)
	if !shouldNotify || r.onUserQueryChanged == nil {
		return
	}

	util.Go(envelope.ctx, "handle native launcher query change", func() {
		if err := r.onUserQueryChanged(envelope.ctx, envelope.query); err != nil {
			util.GetLogger().Error(envelope.ctx, "launcher native query handling failed: "+err.Error())
		}
	})
}

func (r *WindowShellRuntime) handleSelectionNavigation(ctx context.Context, delta int) {
	if delta == 0 {
		return
	}

	util.Go(ctx, "handle native launcher selection navigation", func() {
		r.applySelectionNavigation(ctx, delta)
	})
}

func (r *WindowShellRuntime) applySelectionNavigation(ctx context.Context, delta int) {
	r.mu.Lock()
	if len(r.results.Items) == 0 {
		r.mu.Unlock()
		return
	}

	nextIndex := moveSelectableIndex(r.results.Items, r.results.SelectedIndex, delta)
	if nextIndex == r.results.SelectedIndex {
		r.mu.Unlock()
		return
	}

	r.results.SelectedIndex = nextIndex
	showContext := r.lastShowContext
	query := r.query
	queryBox := buildQueryBoxState(showContext, query)
	results := r.results
	paintTheme := r.paintTheme
	r.mu.Unlock()

	if r.host == nil || !r.host.IsVisible(ctx) {
		return
	}

	if err := r.host.Show(ctx, platform.ShowRequest{
		ShowContext: showContext,
		Query:       query,
		QueryBox:    queryBox,
		Results:     results,
		Theme:       paintTheme,
	}); err != nil {
		util.GetLogger().Error(ctx, "launcher shell selection refresh failed: "+err.Error())
		return
	}

	r.syncTextInputState(ctx, queryBox)
}

func (r *WindowShellRuntime) handleSelectedResultSubmit(ctx context.Context) {
	if r.onSelectedAction == nil {
		return
	}

	util.Go(ctx, "handle native launcher selected result action", func() {
		r.applySelectedResultSubmit(ctx)
	})
}

func (r *WindowShellRuntime) applySelectedResultSubmit(ctx context.Context) {
	r.mu.RLock()
	if len(r.results.Items) == 0 || r.results.SelectedIndex < 0 || r.results.SelectedIndex >= len(r.results.Items) {
		r.mu.RUnlock()
		return
	}
	item := r.results.Items[r.results.SelectedIndex]
	r.mu.RUnlock()

	if item.IsGroup || item.ResultID == "" || item.ActionID == "" {
		return
	}

	if err := r.onSelectedAction(ctx, item.QueryID, item.ResultID, item.ActionID); err != nil {
		util.GetLogger().Error(ctx, "launcher selected result action failed: "+err.Error())
	}
}

func (r *WindowShellRuntime) updateQueryFromTextInput(state platform.TextInputState) (queryChangeEnvelope, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	next := r.query
	next.QueryType = string(plugin.QueryTypeInput)
	next.QuerySelection = common.PlainQuery{}.QuerySelection

	if next.QueryText == state.QueryBox.Text && next.QueryType == r.query.QueryType {
		r.query = next
		return queryChangeEnvelope{}, false
	}

	next.QueryText = state.QueryBox.Text
	next.QueryId = uuid.NewString()
	r.query = next

	queryCtx := util.WithShowSourceContext(
		util.WithQueryIdContext(
			util.WithSessionContext(util.NewTraceContext(), r.sessionID),
			next.QueryId,
		),
		string(r.lastShowContext.ShowSource),
	)

	return queryChangeEnvelope{
		ctx:   queryCtx,
		query: next,
	}, true
}

func (r *WindowShellRuntime) DebugSnapshot(ctx context.Context) DebugSnapshot {
	r.mu.RLock()
	query := r.query
	results := r.results
	r.mu.RUnlock()

	snapshot := DebugSnapshot{Query: query, Results: results}
	if host, ok := r.host.(platform.DebugHost); ok {
		snapshot.Host = host.DebugSnapshot(ctx)
	}
	if textInput, ok := r.textInput.(platform.DebugTextInputHost); ok {
		snapshot.TextInput = textInput.DebugSnapshot(ctx)
	}
	return snapshot
}
