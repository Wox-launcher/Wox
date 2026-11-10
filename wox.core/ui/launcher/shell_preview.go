package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"runtime"
	"strings"
	"unicode/utf8"

	"wox/common"
	"wox/plugin"
	"wox/plugin/system/shell/interactive"
	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
	"wox/util/clipboard"
)

const shellPreviewFocusKey woxwidget.Key = "launcher-shell-preview-input"

type shellPreviewServices interface {
	StartInteractiveShell(context.Context, string, interactive.Options, interactive.Size) (*interactive.Session, error)
}

// The plugin preview and each launch request have separate identities so stale work cannot reopen a shell.
type shellPreviewState struct {
	queryID, resultID string
	run               *shellPreviewRun
	size              interactive.Size
	focusPending      bool
	markedText        string
	pasteTooLarge     bool
	selection         previewview.ShellSelection
}

type shellPreviewRun struct {
	cancel           context.CancelFunc
	session          *interactive.Session
	frame            *interactive.Frame
	err              error
	historyPublished bool
}

// displayedPluginPreview changes the content of the Shell pane without replacing its live session.
func (a *previewController) displayedPluginPreview() (queryResult, bool) {
	result, ok := a.pluginPreview.result()
	if ok {
		switch a.shellOutput.Preview.PreviewType {
		case plugin.WoxPreviewTypeShell:
			result.ID = a.shellOutput.ID
			result.Preview = a.shellOutput.Preview
		case plugin.WoxPreviewTypeTerminal:
			result.Preview = a.shellOutput.Preview
		}
	}
	return result, ok
}

// pinShellOutput remembers the activated row independently of the selected result.
func (a *App) pinShellOutput(result queryResult) bool {
	preview := &a.previews.pluginPreview
	if preview.pluginID != common.ShellPluginID || preview.preview == nil || result.QueryID != a.querySession.query.QueryID {
		return false
	}
	a.previews.shellOutput = queryResult{ID: result.ID, Preview: result.Preview}
	return true
}

// showLiveShell closes the transcript and restores the live preview and query focus.
func (a *App) showLiveShell() {
	a.previews.shellOutput = queryResult{}
	a.previews.reconcileSelectedPreview()
	_ = a.applyWindowBounds()
	a.returnShellFocus()
}

// reconcileShellPreview starts a newly visible shell while leaving the query available for typing a command.
func (a *App) reconcileShellPreview(result queryResult) {
	state := &a.shellPreview
	if state.queryID == result.QueryID && state.resultID == result.ID {
		return
	}
	if state.run != nil {
		state.run.cancel()
	}
	*state = shellPreviewState{queryID: result.QueryID, resultID: result.ID, size: state.size}
	if a.host.HasFocus(shellPreviewFocusKey) {
		a.returnShellFocus()
	}
	if result.ID != "" {
		a.startShellPreview(result, false)
	}
}

// openShellPreview focuses the plugin's shell or restarts it after exit.
func (a *App) openShellPreview(result queryResult) {
	selected, preview, visible := a.previews.selectedPreviewForLifecycle()
	if !visible || selected.QueryID != result.QueryID || selected.ID != result.ID || preview.PreviewType != plugin.WoxPreviewTypeShell {
		return
	}
	a.startShellPreview(result, true)
}

// startShellPreview reuses a running session and serializes process replacements.
func (a *App) startShellPreview(result queryResult, focus bool) {
	service, ok := a.services.(shellPreviewServices)
	if !ok {
		return
	}
	state := &a.shellPreview
	if state.queryID != result.QueryID || state.resultID != result.ID {
		return
	}
	state.focusPending = focus
	if run := state.run; run != nil && run.err == nil && (run.frame == nil || !run.frame.Exited) {
		a.invalidateShellPreview()
		return
	}
	if state.run != nil {
		state.run.cancel()
	}
	ctx, cancel := context.WithCancel(a.lifecycleCtx)
	request := &shellPreviewRun{cancel: cancel}
	state.run = request
	state.markedText = ""
	state.pasteTooLarge = false
	state.selection = previewview.ShellSelection{}
	size := state.size
	if size.Columns == 0 {
		size = interactive.Size{Columns: 80, Rows: 24, CellWidthPixels: 8, CellHeightPixels: 18}
	}
	a.invalidateShellPreview()
	util.Go(ctx, "start interactive shell", func() {
		// A replacement cannot start while the previous PTY is still tearing down.
		a.shellRunMu.Lock()
		defer a.shellRunMu.Unlock()
		if ctx.Err() != nil {
			return
		}
		var options interactive.Options
		err := json.Unmarshal([]byte(result.Preview.PreviewData), &options)
		var session *interactive.Session
		if err == nil {
			session, err = service.StartInteractiveShell(ctx, a.sessionID, options, size)
		}
		if session != nil {
			defer func() {
				session.Close()
				<-session.Done()
			}()
		}
		if err != nil && ctx.Err() == nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("start interactive shell: %v", err))
		}
		dispatchErr := a.runOnUI("attach interactive shell", func() {
			if state.run != request || ctx.Err() != nil {
				return
			}
			request.session, request.err = session, err
			if err != nil {
				state.focusPending = false
			}
			if session != nil && state.size.Columns > 0 {
				session.Resize(state.size)
			}
			a.invalidateShellPreview()
		})
		if dispatchErr != nil || ctx.Err() != nil || session == nil {
			cancel()
			return
		}
		a.watchShellPreview(ctx, request, session)
	})
}

// watchShellPreview consumes latest-frame notifications without blocking PTY reads or writes.
func (a *App) watchShellPreview(ctx context.Context, request *shellPreviewRun, session *interactive.Session) {
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-session.Changes():
			frame := session.Frame()
			if frame != nil {
				err := a.runOnUI("publish interactive shell frame", func() {
					state := &a.shellPreview
					if state.run != request || ctx.Err() != nil || request.frame == frame {
						return
					}
					var previousHistory interactive.HistoryEntry
					if request.frame != nil {
						previousHistory = request.frame.History
					}
					state.selection = previewview.ShellSelection{}
					request.frame = frame
					if previousHistory != frame.History {
						a.syncShellSessionHistory()
					}
					if a.previews.shellOutput.Preview.PreviewType != plugin.WoxPreviewTypeTerminal {
						a.invalidateShellPreview()
					}
				})
				if err != nil {
					session.Close()
					return
				}
			}
			if !ok {
				return
			}
		}
	}
}

func (a *App) invalidateShellPreview() {
	if a.window != nil {
		_ = a.window.Invalidate()
	}
}

func (a *App) returnShellFocus() {
	a.shellPreview.focusPending = false
	a.shellPreview.markedText = ""
	if a.host != nil {
		a.host.RequestFocus(launcherview.LauncherQueryInputKey)
	}
	a.invalidateShellPreview()
}

// shellViewportProps shares logical drawing metrics and physical terminal dimensions.
func (a *App) shellViewportProps(theme woxcomponent.Theme, width, height, scale, barHeight float32) (previewview.ShellPreviewProps, interactive.Size) {
	style := woxui.TextStyle{Size: theme.Controls.Scaled(woxcomponent.PreviewCodeFontSize), Family: woxui.FontFamilyMonospace}
	if strings.TrimSpace(a.generalSettings.Data().AppFontFamily) != "" {
		style.Family = woxui.FontFamilyUI
	}
	var measure func(string, woxui.TextStyle) (woxui.TextMetrics, error)
	if a.window != nil {
		measure = a.window.MeasureText
	}
	cellWidth, cellHeight := shellCellMetrics(style, measure)
	padding := theme.Controls.Scaled(8)
	size, cellWidth, cellHeight := shellGridMetrics(width-2*padding, height-barHeight-padding, cellWidth, cellHeight, scale)
	return previewview.ShellPreviewProps{
		Width: width, Height: height, Padding: padding, BarHeight: barHeight, Theme: theme,
		CellWidth: cellWidth, CellHeight: cellHeight, Style: style, FocusKey: shellPreviewFocusKey,
	}, size
}

// shellCellMetrics keeps terminal columns fixed even when the configured font is proportional.
func shellCellMetrics(style woxui.TextStyle, measure func(string, woxui.TextStyle) (woxui.TextMetrics, error)) (width, height float32) {
	width, height = style.Size*0.61, style.Size*1.35
	if measure == nil {
		return width, height
	}
	samples := []struct {
		text    string
		columns float32
	}{{"M", 1}, {"W", 1}, {"@", 1}, {"界", 2}}
	if style.Family == woxui.FontFamilyMonospace {
		samples = samples[:1]
	}
	var measuredWidth float32
	for _, sample := range samples {
		if metrics, err := measure(sample.text, style); err == nil {
			measuredWidth = max(measuredWidth, metrics.Size.Width/sample.columns)
			height = max(height, metrics.Size.Height)
		}
	}
	if measuredWidth > 0 {
		width = max(1, measuredWidth)
	}
	return width, height
}

// buildShellPreview paints the interactive shell and connects the viewport to its PTY.
func (a *App) buildShellPreview(result queryResult, theme woxcomponent.Theme, width, height, scale float32) woxwidget.Widget {
	state := &a.shellPreview
	props, size := a.shellViewportProps(theme, width, height, scale, theme.Controls.Scaled(34))
	props.Open = a.language.translate("i18n:plugin_shell_interactive_open")
	props.Reopen = a.language.translate("i18n:plugin_shell_interactive_reopen")
	props.Help = a.language.translate("i18n:plugin_shell_interactive_keys")
	props.OnOpen = func() { a.openShellPreview(result) }
	if state.queryID != result.QueryID || state.resultID != result.ID {
		return previewview.ShellPreviewView(props)
	}
	if state.size != size {
		state.size = size
		if state.run != nil && state.run.session != nil {
			state.run.session.Resize(size)
		}
	}
	if run := state.run; run != nil {
		props.Frame = run.frame
		if run.err != nil {
			props.Error = a.language.translate("i18n:plugin_shell_interactive_failed")
		} else if run.frame != nil && run.frame.Err != nil {
			props.Error = a.language.translate("i18n:plugin_shell_interactive_exited")
		}
		if run.frame != nil && run.frame.PasteBlocked {
			props.Help = a.language.translate("i18n:plugin_shell_interactive_paste_blocked")
		}
	}
	if state.pasteTooLarge {
		props.Help = a.language.translate("i18n:plugin_shell_interactive_paste_large")
	}
	props.MarkedText, props.Selection = state.markedText, state.selection
	props.OnKey = a.onShellPreviewKey
	props.OnText = a.onShellPreviewText
	props.OnFocus = func() {
		if a.host != nil {
			a.host.RequestFocus(shellPreviewFocusKey)
		}
	}
	props.OnBlur = func() { state.markedText = ""; a.invalidateShellPreview() }
	props.OnSelect = func(index int, extend bool) {
		if !extend {
			state.selection.Anchor = index
		}
		state.selection.End = index
		a.invalidateShellPreview()
	}
	props.OnScroll = func(rows int) {
		state.selection = previewview.ShellSelection{}
		if state.run != nil && state.run.session != nil {
			state.run.session.Scroll(rows)
		}
		a.invalidateShellPreview()
	}
	return previewview.ShellPreviewView(props)
}

// shellGridMetrics snaps cells to device pixels and converts the snapped dimensions back for layout.
func shellGridMetrics(width, height, cellWidth, cellHeight, scale float32) (interactive.Size, float32, float32) {
	if scale <= 0 {
		scale = 1
	}
	physicalWidth := max(1, int(math.Ceil(float64(cellWidth*scale))))
	physicalHeight := max(1, int(math.Ceil(float64(cellHeight*scale))))
	cellWidth, cellHeight = float32(physicalWidth)/scale, float32(physicalHeight)/scale
	size := interactive.Size{
		Columns:         min(interactive.MaxColumns, max(1, int(width/cellWidth))),
		Rows:            min(interactive.MaxRows, max(1, int(height/cellHeight))),
		CellWidthPixels: physicalWidth, CellHeightPixels: physicalHeight,
	}
	return size, cellWidth, cellHeight
}

// onShellPreviewKey lets the Launcher handle its action-panel shortcut before terminal input.
func (a *App) onShellPreviewKey(event woxui.KeyEvent) bool {
	state := &a.shellPreview
	if !event.Down || event.Composing {
		return false
	}
	if hotkeyMatches(a.actionPanelHotkey(), event) {
		return a.actions.onActionKey(event)
	}
	if event.Key == woxui.KeyEscape && event.Modifiers == 0 {
		a.returnShellFocus()
		return true
	}
	run := state.run
	if run == nil || run.session == nil {
		return false
	}
	ctrl := event.Modifiers&woxui.KeyModifierControl != 0
	alt := event.Modifiers&woxui.KeyModifierAlt != 0
	shift := event.Modifiers&woxui.KeyModifierShift != 0
	meta := event.Modifiers&woxui.KeyModifierMeta != 0
	clipboardShortcut := !alt && (runtime.GOOS == "darwin" && meta && !ctrl || runtime.GOOS != "darwin" && ctrl && shift && !meta)
	if clipboardShortcut && event.Key == "c" {
		if text := previewview.ShellSelectedText(run.frame, state.selection); text != "" {
			_ = clipboard.WriteText(text)
		}
		return true
	}
	if run.frame != nil && run.frame.Exited {
		return false
	}
	if clipboardShortcut && event.Key == "v" {
		if text, err := clipboard.ReadText(); err == nil && text != "" {
			state.pasteTooLarge = len(text) > interactive.MaxPaste
			a.invalidateShellPreview()
			if state.pasteTooLarge {
				return true
			}
			a.sendShellInput(run.session.Paste(text))
		}
		return true
	}
	if event.Key == woxui.KeyEscape && event.Modifiers == woxui.KeyModifierShift {
		a.sendShellInput(run.session.Key(interactive.Key{Name: "escape", Repeat: event.Repeat}))
		return true
	}
	name := string(event.Key)
	if name == "" || meta || ctrl && alt || !ctrl && !alt && (utf8.RuneCountInString(name) == 1 || event.Key == woxui.KeySpace) {
		return false
	}
	a.sendShellInput(run.session.Key(interactive.Key{Name: name, Ctrl: ctrl, Alt: alt, Shift: shift, Repeat: event.Repeat}))
	return true
}

func (a *App) onShellPreviewText(event woxui.TextInputEvent) bool {
	state := &a.shellPreview
	if event.Kind == woxui.TextInputCompose {
		state.markedText = event.Text
	} else {
		state.markedText = ""
		state.pasteTooLarge = false
		if run := state.run; run != nil && run.session != nil && event.Text != "" && (run.frame == nil || !run.frame.Exited) {
			a.sendShellInput(run.session.Text(event.Text))
		}
	}
	a.invalidateShellPreview()
	return true
}

// sendShellInput ends a saturated session instead of silently dropping part of a command.
func (a *App) sendShellInput(accepted bool) {
	if accepted {
		return
	}
	if run := a.shellPreview.run; run != nil {
		run.err = fmt.Errorf("shell input was not accepted")
		run.cancel()
		a.invalidateShellPreview()
	}
}
