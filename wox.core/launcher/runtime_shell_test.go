package launcher

import (
	"context"
	"errors"
	"testing"
	"time"
	"wox/common"
	"wox/launcher/platform"
	"wox/plugin"
)

type fakeShellHost struct {
	startCalls []platform.StartOptions
	showCalls  []platform.ShowRequest
	hideCalls  int
	stopCalls  int
	visible    bool
	startErr   error
	showErr    error
	hideErr    error
}

type fakeTextInputHost struct {
	startCalls  int
	stopCalls   int
	focusCalls  int
	blurCalls   int
	updateCalls []platform.TextInputState
	parentCalls []uintptr
	changeFn    platform.TextInputChangeHandler
	navFn       platform.TextInputSelectionNavigationHandler
}

func (f *fakeShellHost) Start(ctx context.Context, options platform.StartOptions) error {
	f.startCalls = append(f.startCalls, options)
	return f.startErr
}

func (f *fakeShellHost) Stop(ctx context.Context) error {
	f.stopCalls++
	return nil
}

func (f *fakeShellHost) Show(ctx context.Context, request platform.ShowRequest) error {
	f.showCalls = append(f.showCalls, request)
	if f.showErr == nil {
		f.visible = true
	}
	return f.showErr
}

func (f *fakeShellHost) Hide(ctx context.Context) error {
	f.hideCalls++
	if f.hideErr == nil {
		f.visible = false
	}
	return f.hideErr
}

func (f *fakeShellHost) IsVisible(ctx context.Context) bool {
	return f.visible
}

func (f *fakeTextInputHost) Start(ctx context.Context) error {
	f.startCalls++
	return nil
}

func (f *fakeTextInputHost) Stop(ctx context.Context) error {
	f.stopCalls++
	return nil
}

func (f *fakeTextInputHost) UpdateState(ctx context.Context, state platform.TextInputState) error {
	f.updateCalls = append(f.updateCalls, state)
	return nil
}

func (f *fakeTextInputHost) Focus(ctx context.Context) error {
	f.focusCalls++
	return nil
}

func (f *fakeTextInputHost) Blur(ctx context.Context) error {
	f.blurCalls++
	return nil
}

func (f *fakeTextInputHost) SetParentWindow(ctx context.Context, windowHandle uintptr) error {
	f.parentCalls = append(f.parentCalls, windowHandle)
	return nil
}

func (f *fakeTextInputHost) SetChangeHandler(ctx context.Context, handler platform.TextInputChangeHandler) error {
	f.changeFn = handler
	return nil
}

func (f *fakeTextInputHost) EmitChange(state platform.TextInputState) {
	if f.changeFn != nil {
		f.changeFn(context.Background(), state)
	}
}

func (f *fakeTextInputHost) SetSelectionNavigationHandler(ctx context.Context, handler platform.TextInputSelectionNavigationHandler) error {
	f.navFn = handler
	return nil
}

func (f *fakeTextInputHost) EmitNavigation(delta int) {
	if f.navFn != nil {
		f.navFn(context.Background(), delta)
	}
}

var _ platform.Host = (*fakeShellHost)(nil)
var _ platform.TextInputHost = (*fakeTextInputHost)(nil)

func TestWindowShellRuntimeStartPassesDefaultShellAppearance(t *testing.T) {
	t.Parallel()

	host := &fakeShellHost{}
	runtime := NewWindowShellRuntime(host)

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	if len(host.startCalls) != 1 {
		t.Fatalf("expected one start call, got %d", len(host.startCalls))
	}

	start := host.startCalls[0]
	if !start.Appearance.Transparent {
		t.Fatal("launcher shell should start with transparency enabled")
	}

	if !start.Appearance.RoundedCorners {
		t.Fatal("launcher shell should start with rounded corners enabled")
	}
}

func TestWindowShellRuntimeShowHideAndToggleDelegateToHost(t *testing.T) {
	t.Parallel()

	host := &fakeShellHost{}
	runtime := NewWindowShellRuntime(host)

	showContext := common.ShowContext{HideToolbar: true}
	runtime.Show(context.Background(), showContext)
	if len(host.showCalls) != 1 || host.showCalls[0].ShowContext != showContext {
		t.Fatal("Show should delegate the show context to the shell host")
	}

	runtime.Toggle(context.Background(), common.ShowContext{HideOnBlur: true})
	if host.hideCalls != 1 {
		t.Fatal("Toggle should hide when host reports the window as visible")
	}

	runtime.Toggle(context.Background(), common.ShowContext{SelectAll: true})
	if len(host.showCalls) != 2 || !host.showCalls[1].ShowContext.SelectAll {
		t.Fatal("Toggle should show when host reports the window as hidden")
	}
}

func TestWindowShellRuntimeStartReturnsHostError(t *testing.T) {
	t.Parallel()

	host := &fakeShellHost{startErr: errors.New("boom")}
	runtime := NewWindowShellRuntime(host)

	if err := runtime.Start(context.Background()); err == nil {
		t.Fatal("Start should return host start errors")
	}
}

func TestWindowShellRuntimePropagatesQueryStateToHost(t *testing.T) {
	t.Parallel()

	host := &fakeShellHost{}
	runtime := NewWindowShellRuntime(host)

	initialQuery := common.PlainQuery{QueryText: "hello"}
	runtime.ChangeQuery(context.Background(), initialQuery)
	runtime.Show(context.Background(), common.ShowContext{SelectAll: true})

	if len(host.showCalls) != 1 {
		t.Fatalf("expected one show call, got %d", len(host.showCalls))
	}

	if host.showCalls[0].Query.QueryText != initialQuery.QueryText {
		t.Fatal("Show should pass the latest query state to the host")
	}

	if !host.showCalls[0].QueryBox.Visible {
		t.Fatal("Show should mark the query box as visible by default")
	}

	if host.showCalls[0].QueryBox.Text != initialQuery.QueryText {
		t.Fatal("Show should derive query-box text from the latest query state")
	}

	if host.showCalls[0].QueryBox.Placeholder == "" {
		t.Fatal("Show should provide a query-box placeholder")
	}

	updatedQuery := common.PlainQuery{QueryText: "hello world"}
	runtime.ChangeQuery(context.Background(), updatedQuery)

	if len(host.showCalls) != 2 {
		t.Fatalf("expected ChangeQuery to refresh the visible shell, got %d show calls", len(host.showCalls))
	}

	if host.showCalls[1].Query.QueryText != updatedQuery.QueryText {
		t.Fatal("ChangeQuery should refresh the host with the updated query state")
	}

	if host.showCalls[1].QueryBox.Text != updatedQuery.QueryText {
		t.Fatal("ChangeQuery should refresh the query-box text")
	}
}

func TestWindowShellRuntimeRefreshQueryReplaysVisibleState(t *testing.T) {
	t.Parallel()

	host := &fakeShellHost{}
	runtime := NewWindowShellRuntime(host)

	runtime.ChangeQuery(context.Background(), common.PlainQuery{QueryText: "refresh me"})
	runtime.Show(context.Background(), common.ShowContext{SelectAll: true})
	runtime.RefreshQuery(context.Background(), false)

	if len(host.showCalls) != 2 {
		t.Fatalf("expected RefreshQuery to replay the visible state, got %d show calls", len(host.showCalls))
	}

	if host.showCalls[1].QueryBox.Text != "refresh me" {
		t.Fatal("RefreshQuery should keep the latest query-box text")
	}
}

func TestWindowShellRuntimePropagatesMappedThemeToHost(t *testing.T) {
	t.Parallel()

	host := &fakeShellHost{}
	runtime := NewWindowShellRuntime(host)

	runtime.ChangeTheme(context.Background(), common.Theme{
		ThemeId:                              "theme-1",
		AppBackgroundColor:                   "#101010",
		QueryBoxBackgroundColor:              "#202020",
		QueryBoxFontColor:                    "#f5f5f5",
		QueryBoxCursorColor:                  "#ff0000",
		QueryBoxBorderRadius:                 16,
		QueryBoxTextSelectionBackgroundColor: "#333333",
		QueryBoxTextSelectionColor:           "#eeeeee",
	})
	runtime.Show(context.Background(), common.ShowContext{})

	if len(host.showCalls) != 1 {
		t.Fatalf("expected one show call, got %d", len(host.showCalls))
	}

	if host.showCalls[0].Theme.ThemeID != "theme-1" {
		t.Fatal("Show should pass the mapped launcher theme to the host")
	}

	if host.showCalls[0].Theme.QueryBox.BackgroundColor != "#202020" {
		t.Fatal("Show should pass the query-box background color from the mapped theme")
	}

	if host.showCalls[0].Theme.Window.BackgroundColor != "#101010" {
		t.Fatal("Show should pass the window background color from the mapped theme")
	}
}

func TestWindowShellRuntimeSyncsTextInputLifecycle(t *testing.T) {
	t.Parallel()

	host := &fakeShellHost{}
	textInput := &fakeTextInputHost{}
	runtime := NewWindowShellRuntimeWithBundle(platform.Bundle{
		Host:      host,
		TextInput: textInput,
	})

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	runtime.ChangeQuery(context.Background(), common.PlainQuery{QueryText: "hello"})
	runtime.Show(context.Background(), common.ShowContext{})

	if textInput.startCalls != 1 {
		t.Fatal("Start should start the text input host")
	}

	if textInput.focusCalls != 1 {
		t.Fatal("Show should focus the text input host when query box is visible")
	}

	if len(textInput.updateCalls) == 0 {
		t.Fatal("Show should push query-box state into the text input host")
	}

	last := textInput.updateCalls[len(textInput.updateCalls)-1]
	if last.QueryBox.Text != "hello" {
		t.Fatal("text input host should receive the latest query-box text")
	}

	if last.QueryBox.Frame.Width <= 0 || last.QueryBox.Frame.Height <= 0 {
		t.Fatal("Show should derive a visible query-box frame for the text input host")
	}

	if len(textInput.parentCalls) != 0 {
		t.Fatal("Show should not attach to a native parent when the shell host does not expose one")
	}

	runtime.Hide(context.Background())
	if textInput.blurCalls != 1 {
		t.Fatal("Hide should blur the text input host")
	}

	if err := runtime.Stop(context.Background()); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}

	if textInput.stopCalls != 1 {
		t.Fatal("Stop should stop the text input host")
	}
}

type fakeNativeShellHost struct {
	fakeShellHost
	windowHandle uintptr
}

func (f *fakeNativeShellHost) NativeWindowHandle(ctx context.Context) uintptr {
	return f.windowHandle
}

func TestWindowShellRuntimeAttachesTextInputToNativeShellWindow(t *testing.T) {
	t.Parallel()

	host := &fakeNativeShellHost{windowHandle: 0x1234}
	textInput := &fakeTextInputHost{}
	runtime := NewWindowShellRuntimeWithBundle(platform.Bundle{
		Host:      host,
		TextInput: textInput,
	})

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	runtime.Show(context.Background(), common.ShowContext{})

	if len(textInput.parentCalls) != 1 {
		t.Fatalf("expected one parent attachment, got %d", len(textInput.parentCalls))
	}

	if textInput.parentCalls[0] != 0x1234 {
		t.Fatalf("unexpected parent window handle: %#x", textInput.parentCalls[0])
	}
}

func TestWindowShellRuntimeDerivesQueryBoxFrameFromWindowPosition(t *testing.T) {
	t.Parallel()

	host := &fakeShellHost{}
	textInput := &fakeTextInputHost{}
	runtime := NewWindowShellRuntimeWithBundle(platform.Bundle{
		Host:      host,
		TextInput: textInput,
	})

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	runtime.ChangeQuery(context.Background(), common.PlainQuery{QueryText: "frame"})
	runtime.Show(context.Background(), common.ShowContext{
		WindowWidth: 800,
		WindowPosition: &common.WindowPosition{
			X: 100,
			Y: 120,
		},
	})

	last := textInput.updateCalls[len(textInput.updateCalls)-1]
	if last.QueryBox.Frame.X != 124 || last.QueryBox.Frame.Y != 140 {
		t.Fatalf("unexpected query-box origin: %+v", last.QueryBox.Frame)
	}

	if last.QueryBox.Frame.Width != 752 || last.QueryBox.Frame.Height != 48 {
		t.Fatalf("unexpected query-box size: %+v", last.QueryBox.Frame)
	}
}

func TestWindowShellRuntimeTracksNativeTextInputChanges(t *testing.T) {
	t.Parallel()

	host := &fakeShellHost{}
	textInput := &fakeTextInputHost{}
	var observed []common.PlainQuery
	observedCh := make(chan common.PlainQuery, 1)
	runtime := NewWindowShellRuntimeWithBundleAndOptions(platform.Bundle{
		Host:      host,
		TextInput: textInput,
	}, WindowShellRuntimeOptions{
		OnUserQueryChanged: func(ctx context.Context, query common.PlainQuery) error {
			observed = append(observed, query)
			observedCh <- query
			return nil
		},
	})

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	runtime.Show(context.Background(), common.ShowContext{})

	textInput.EmitChange(platform.TextInputState{
		QueryBox: platform.QueryBoxState{
			Visible: true,
			Text:    "typed from native",
		},
		SelectionStart: 17,
		SelectionEnd:   17,
	})

	if got := runtime.DebugSnapshot(context.Background()).Query.QueryText; got != "typed from native" {
		t.Fatalf("runtime should update query text from native input, got %q", got)
	}

	var observedQuery common.PlainQuery
	select {
	case observedQuery = <-observedCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for observed native query change")
	}

	if len(observed) != 1 {
		t.Fatalf("expected one observed native query change, got %d", len(observed))
	}

	if observedQuery.QueryText != "typed from native" {
		t.Fatalf("unexpected observed query text: %q", observedQuery.QueryText)
	}

	if observedQuery.QueryId == "" {
		t.Fatal("native query changes should allocate a query id")
	}

	if observedQuery.QueryType != "input" {
		t.Fatalf("native query changes should use input query type, got %q", observedQuery.QueryType)
	}
}

func TestWindowShellRuntimePushResultsUpdatesPreviewState(t *testing.T) {
	t.Parallel()

	host := &fakeShellHost{}
	runtime := NewWindowShellRuntime(host)

	runtime.ChangeQuery(context.Background(), common.PlainQuery{QueryId: "q1", QueryType: "input", QueryText: "hello"})
	runtime.Show(context.Background(), common.ShowContext{WindowWidth: 900})

	ok := runtime.PushResults(context.Background(), plugin.PushResultsPayload{
		QueryId: "q1",
		Results: []plugin.QueryResultUI{
			{
				QueryId:  "q1",
				Id:       "r1",
				Title:    "Result 1",
				SubTitle: "Subtitle 1",
				Preview: plugin.WoxPreview{
					PreviewType: plugin.WoxPreviewTypeText,
					PreviewData: "Preview body",
				},
			},
		},
	})

	if !ok {
		t.Fatal("PushResults should be accepted for the active query")
	}

	last := host.showCalls[len(host.showCalls)-1]
	if !last.Preview.Visible {
		t.Fatal("PushResults should make preview visible for previewable results")
	}
	if last.Preview.Title != "Result 1" {
		t.Fatalf("unexpected preview title: %q", last.Preview.Title)
	}
	if last.Preview.Body.Kind != platform.PreviewKindText || last.Preview.Body.Content != "Preview body" {
		t.Fatalf("unexpected preview body: %+v", last.Preview.Body)
	}
}

func TestWindowShellRuntimeAdjustsWindowHeightToVisibleResults(t *testing.T) {
	t.Parallel()

	host := &fakeShellHost{}
	runtime := NewWindowShellRuntime(host)

	runtime.ChangeTheme(context.Background(), common.Theme{
		AppPaddingBottom:             10,
		ResultContainerPaddingBottom: 0,
		ResultItemPaddingTop:         4,
		ResultItemPaddingBottom:      4,
	})
	runtime.ChangeQuery(context.Background(), common.PlainQuery{QueryId: "q1", QueryType: "input", QueryText: "term"})
	runtime.Show(context.Background(), common.ShowContext{WindowWidth: 900, MaxResultCount: 2})

	if got := host.showCalls[len(host.showCalls)-1].WindowHeight; got != 78 {
		t.Fatalf("expected query-only height 78, got %d", got)
	}

	if ok := runtime.PushResults(context.Background(), plugin.PushResultsPayload{
		QueryId: "q1",
		Results: []plugin.QueryResultUI{
			{QueryId: "q1", Id: "r1", Title: "One"},
		},
	}); !ok {
		t.Fatal("PushResults should accept single-result payload")
	}

	if got := host.showCalls[len(host.showCalls)-1].WindowHeight; got != 152 {
		t.Fatalf("expected single-result height 152, got %d", got)
	}

	if ok := runtime.PushResults(context.Background(), plugin.PushResultsPayload{
		QueryId: "q1",
		Results: []plugin.QueryResultUI{
			{QueryId: "q1", Id: "r1", Title: "One"},
			{QueryId: "q1", Id: "r2", Title: "Two"},
			{QueryId: "q1", Id: "r3", Title: "Three"},
		},
	}); !ok {
		t.Fatal("PushResults should accept multi-result payload")
	}

	if got := host.showCalls[len(host.showCalls)-1].WindowHeight; got != 216 {
		t.Fatalf("expected capped multi-result height 216, got %d", got)
	}

	if ok := runtime.PushResults(context.Background(), plugin.PushResultsPayload{
		QueryId: "q1",
		Results: []plugin.QueryResultUI{
			{
				QueryId: "q1",
				Id:      "r-preview",
				Title:   "Preview",
				Preview: plugin.WoxPreview{
					PreviewType: plugin.WoxPreviewTypeText,
					PreviewData: "body",
				},
			},
		},
	}); !ok {
		t.Fatal("PushResults should accept preview payload")
	}

	if got := host.showCalls[len(host.showCalls)-1].WindowHeight; got != 216 {
		t.Fatalf("expected preview height to stay at cap 216, got %d", got)
	}
}

func TestWindowShellRuntimeSelectionNavigationUpdatesPreviewState(t *testing.T) {
	t.Parallel()

	host := &fakeShellHost{}
	textInput := &fakeTextInputHost{}
	runtime := NewWindowShellRuntimeWithBundle(platform.Bundle{
		Host:      host,
		TextInput: textInput,
	})

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	runtime.ChangeQuery(context.Background(), common.PlainQuery{QueryId: "q1", QueryType: "input", QueryText: "hello"})
	runtime.Show(context.Background(), common.ShowContext{WindowWidth: 900})
	runtime.PushResults(context.Background(), plugin.PushResultsPayload{
		QueryId: "q1",
		Results: []plugin.QueryResultUI{
			{
				QueryId: "q1",
				Id:      "r1",
				Title:   "Result 1",
				Preview: plugin.WoxPreview{
					PreviewType: plugin.WoxPreviewTypeText,
					PreviewData: "Preview 1",
				},
				Actions: []plugin.QueryResultActionUI{{Id: "a1", IsDefault: true}},
			},
			{
				QueryId: "q1",
				Id:      "r2",
				Title:   "Result 2",
				Preview: plugin.WoxPreview{
					PreviewType: plugin.WoxPreviewTypeMarkdown,
					PreviewData: "Preview 2",
				},
				Actions: []plugin.QueryResultActionUI{{Id: "a2", IsDefault: true}},
			},
		},
	})

	textInput.EmitNavigation(1)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snapshot := runtime.DebugSnapshot(context.Background())
		if snapshot.Results.SelectedIndex == 1 && snapshot.Preview.Title == "Result 2" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	snapshot := runtime.DebugSnapshot(context.Background())
	t.Fatalf("unexpected post-navigation snapshot: %+v", snapshot)
}
