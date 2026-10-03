package screenshot

import (
	"context"
	"fmt"
	"runtime"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
	"wox/util/ffmpeg"
)

// recordingRuntimeDialog is UI-thread owned; workers publish progress only through dispatch.
type recordingRuntimeDialog struct {
	owner                             *recordingToolbarState
	window                            *ManagedWindow
	host                              *woxwidget.Host
	bytes                             int64
	unsupported, busy, failed, closed bool
	progress                          ffmpeg.Progress
	cancel                            context.CancelFunc
	install                           func(context.Context, func(ffmpeg.Progress)) (string, error)
	dispatch                          func(func()) error
}

// openRuntimeDialog requests consent without starting a download or a recording session.
func (state *recordingToolbarState) openRuntimeDialog() {
	state.mu.Lock()
	if state.cancelled || state.finishing || state.runtimeDialog != nil {
		state.mu.Unlock()
		return
	}
	size, err := ffmpeg.DownloadSize()
	dialog := &recordingRuntimeDialog{owner: state, bytes: size, unsupported: err != nil, install: ffmpeg.Install, dispatch: Call}
	state.runtimeDialog = dialog
	state.mu.Unlock()
	if err := dialog.open(); err != nil {
		util.GetLogger().Error(context.Background(), fmt.Sprintf("open recording runtime dialog: %v", err))
		state.closeRuntimeDialog()
		state.setFinishError(err)
	}
}

// open uses logical units so the native window handles monitor scaling independently of capture pixels.
func (dialog *recordingRuntimeDialog) open() error {
	options := dialog.owner.options
	manager := options.WindowManager
	if manager == nil {
		manager = NewWindowManager()
	}
	role := WindowRoleUtility
	if runtime.GOOS == "darwin" {
		role = WindowRoleScreenshot
	}
	size := Size{Width: options.Theme.Scaled(460), Height: options.Theme.Scaled(264)}
	dialog.host = woxwidget.NewHost(dialog.build)
	managed, _, err := manager.Open("wox.screenshot.recording.runtime", WindowOptions{
		Title: options.RecordingRuntime.Title, Size: size, Role: role, Topmost: true,
		OnFrame: dialog.host.Frame, OnPointer: dialog.host.Pointer, OnKey: dialog.host.Key,
		OnFocus:  func(event woxui.FocusEvent) { dialog.host.SetWindowFocused(event.Active) },
		OnClosed: func() { dialog.window = nil; dialog.close() },
	})
	if err != nil {
		return err
	}
	dialog.window = managed
	dialog.host.Attach(managed.Window())
	if err := managed.Window().SetFontFamily(options.FontFamily); err != nil {
		return err
	}
	if err := managed.Window().CenterOnMouseScreen(size); err != nil {
		return err
	}
	_, err = managed.Show()
	return err
}

// build keeps a stable footprint while the standard buttons change between consent, progress, and retry.
func (dialog *recordingRuntimeDialog) build(frame FrameInfo) woxwidget.Widget {
	labels, theme := dialog.owner.options.RecordingRuntime, dialog.owner.options.Theme
	description := fmt.Sprintf(labels.Description, float64(dialog.bytes)/(1024*1024))
	if dialog.unsupported {
		description = labels.Unsupported
	}
	status, color := "", theme.TextSecondary
	if dialog.failed {
		status, color = labels.Failed, theme.Error
	}
	if dialog.busy {
		status = fmt.Sprintf(labels.Downloading, dialog.progress.Percent)
		if dialog.progress.Stage == ffmpeg.StageInstalling {
			status = labels.Installing
		}
	}
	label := labels.Install
	if dialog.failed {
		label = labels.Retry
	}
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "recording.runtime", Label: labels.Title, Width: frame.Size.Width, Height: frame.Size.Height,
		Padding: woxwidget.UniformInsets(theme.Scaled(20)), Theme: theme, Solid: true,
		InitialFocus: "recording.runtime.cancel", OnEscape: dialog.close,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: theme.Scaled(12), Children: []woxwidget.Widget{
			woxwidget.TextBlock{Value: labels.Title, Height: theme.Scaled(24), MaxLines: 1, Style: TextStyle{Size: theme.Scaled(woxcomponent.SettingsLabelFontSize), Weight: FontWeightSemibold}, Color: theme.Text},
			woxwidget.TextBlock{Value: description, Height: theme.Scaled(72), LineHeight: theme.Scaled(24), MaxLines: 3, Style: TextStyle{Size: theme.Scaled(woxcomponent.SettingsLabelFontSize)}, Color: theme.TextSecondary},
			woxwidget.Expanded{Child: woxwidget.TextBlock{Value: status, LineHeight: theme.Scaled(20), MaxLines: 3, Style: TextStyle{Size: theme.Scaled(woxcomponent.SettingsHelpFontSize)}, Color: color}},
			woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: theme.Scaled(8), MainAxisAlignment: woxwidget.MainAxisEnd, Children: []woxwidget.Widget{
				woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "recording.runtime.cancel", Label: labels.Cancel, Theme: theme, Height: theme.Scaled(32), Radius: theme.Scaled(4), Padding: woxwidget.Insets{Left: theme.Scaled(12), Right: theme.Scaled(12)}, OnTap: dialog.close}),
				woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "recording.runtime.install", Label: label, Theme: theme, Height: theme.Scaled(32), Radius: theme.Scaled(4), Padding: woxwidget.Insets{Left: theme.Scaled(12), Right: theme.Scaled(12)}, Variant: woxcomponent.ButtonPrimary, Disabled: dialog.busy || dialog.unsupported, OnTap: dialog.confirm}),
			}},
		}},
	})
}

// confirm is the only path that starts a network request; repeated activations cannot start extra workers.
func (dialog *recordingRuntimeDialog) confirm() {
	if dialog.closed || dialog.busy || dialog.unsupported {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	dialog.cancel, dialog.busy, dialog.failed = cancel, true, false
	dialog.progress = ffmpeg.Progress{Stage: ffmpeg.StageDownloading}
	dialog.invalidate()
	go func() {
		defer cancel()
		_, err := dialog.install(ctx, func(progress ffmpeg.Progress) {
			_ = dialog.dispatch(func() {
				if !dialog.closed {
					dialog.progress = progress
					dialog.invalidate()
				}
			})
		})
		if err != nil && ctx.Err() == nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("install recording runtime: %v", err))
		}
		_ = dialog.dispatch(func() {
			if dialog.closed {
				return
			}
			dialog.busy, dialog.failed = false, err != nil
			if err != nil {
				dialog.invalidate()
				return
			}
			dialog.close()
			dialog.owner.mu.Lock()
			cancelled := dialog.owner.cancelled
			dialog.owner.mu.Unlock()
			if !cancelled {
				dialog.owner.start()
			}
		})
	}()
}

func (dialog *recordingRuntimeDialog) invalidate() {
	if dialog.window != nil {
		_ = dialog.window.Window().Invalidate()
	}
}

// close cancels transfer and detaches before native close callbacks can reenter.
func (dialog *recordingRuntimeDialog) close() {
	if dialog.closed {
		return
	}
	dialog.closed = true
	if dialog.cancel != nil {
		dialog.cancel()
	}
	dialog.owner.mu.Lock()
	if dialog.owner.runtimeDialog == dialog {
		dialog.owner.runtimeDialog = nil
	}
	dialog.owner.mu.Unlock()
	if dialog.window != nil {
		window := dialog.window
		dialog.window = nil
		_ = window.Close()
	}
	if dialog.host != nil {
		dialog.host.Dispose()
	}
}

// closeRuntimeDialog also handles capture teardown while a transfer is still running.
func (state *recordingToolbarState) closeRuntimeDialog() {
	state.mu.Lock()
	dialog := state.runtimeDialog
	state.mu.Unlock()
	if dialog != nil {
		dialog.close()
	}
}
