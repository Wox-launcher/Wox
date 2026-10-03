package screenshot

import (
	"strings"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// recordingFormatMenu owns the transient popup; the selected export format lives with the take.
type recordingFormatMenu struct {
	owner    *recordingToolbarState
	window   *ManagedWindow
	host     *woxwidget.Host
	size     Size
	scale    float32
	selected recordingExportFormat
}

// drawRecordingFormatTrigger keeps the badge inside the same hit target as neighboring icons.
func drawRecordingFormatTrigger(list *DisplayList, window woxwidget.HostServices, bounds Rect, format recordingExportFormat, disabled, active bool, scale float32) {
	foreground := Color{R: 255, G: 255, B: 255, A: 255}
	if disabled {
		foreground.A = 100
	}
	if active && !disabled {
		list.FillRoundedRect(bounds, 8*scale, Color{R: 255, G: 255, B: 255, A: 24})
	}
	woxwidget.PaintStateless(window, woxwidget.Align{Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Container{
		Width: 34 * scale, Height: 24 * scale, Radius: 4 * scale, BorderWidth: 1 * scale, BorderColor: foreground,
		Child: woxwidget.TextBlock{Value: strings.ToUpper(format.extension()), Height: 24 * scale, LineHeight: 24 * scale, MaxLines: 1, Centered: true, AlignmentY: 0.5, Style: TextStyle{Size: 10 * scale, Weight: FontWeightSemibold}, Color: foreground},
	}}, list, bounds)
}

// recordingFormatMenuBounds uses capture-local coordinates; the platform adapter converts at the window boundary.
func recordingFormatMenuBounds(frame Size, toolbar, anchor Rect, scale float32) Rect {
	margin := 8 * scale
	width := min(280*scale, max(float32(1), frame.Width-2*margin))
	height := min(112*scale, max(float32(1), frame.Height-2*margin))
	x := min(max(margin, toolbar.X+anchor.X), max(margin, frame.Width-width-margin))
	y := toolbar.Y + toolbar.Height + margin
	if y+height > frame.Height-margin {
		y = toolbar.Y - height - margin
	}
	y = min(max(margin, y), max(margin, frame.Height-height-margin))
	return Rect{X: x, Y: y, Width: width, Height: height}
}

// openFormatMenu uses a separate screenshot-level surface so the menu can extend beyond the toolbar.
func (state *recordingToolbarState) openFormatMenu() {
	state.mu.Lock()
	if state.finishing || state.cancelled || state.session == nil || state.session.currentState() != recordingStateSave {
		state.mu.Unlock()
		return
	}
	scale := state.toolbarChromeScale()
	bounds := recordingFormatMenuBounds(state.frameSize, state.expandedBounds, state.formatRect, scale)
	menu := &recordingFormatMenu{owner: state, scale: scale, size: Size{Width: bounds.Width, Height: bounds.Height}, selected: state.exportFormat}
	state.formatMenu = menu
	state.hoverTooltip = ""
	state.mu.Unlock()
	if state.window != nil {
		_ = state.window.Invalidate()
	}
	menu.host = woxwidget.NewHost(menu.build)
	manager := state.options.WindowManager
	if manager == nil {
		manager = NewWindowManager()
	}
	managed, _, err := manager.Open("wox.screenshot.recording.format", WindowOptions{
		Title: state.options.RecordingTooltips.Format, Size: menu.size, Role: WindowRoleScreenshot, Topmost: true,
		OnFrame: menu.host.Frame, OnPointer: menu.host.Pointer, OnKey: menu.key,
		OnFocus: menu.focusChanged,
		OnClosed: func() {
			menu.window = nil
			menu.host.Dispose()
			state.mu.Lock()
			if state.formatMenu == menu {
				state.formatMenu = nil
			}
			state.mu.Unlock()
		},
	})
	if err == nil {
		menu.window = managed
		menu.host.Attach(managed.Window())
		err = managed.Window().SetFontFamily(state.options.FontFamily)
	}
	if err == nil {
		err = setScreenshotScrollingWindowBounds(managed.Window(), state.platform, bounds, state.frameSize)
	}
	if err == nil {
		_, err = managed.Show()
	}
	if err != nil {
		state.closeFormatMenu()
		state.setFinishError(err)
	}
}

// focusChanged leaves trigger clicks to toolbarPointer: native blur arrives before its pointer-down.
func (menu *recordingFormatMenu) focusChanged(event woxui.FocusEvent) {
	if menu.host != nil {
		menu.host.SetWindowFocused(event.Active)
	}
	if event.Active {
		return
	}
	state := menu.owner
	state.mu.Lock()
	current := state.formatMenu == menu
	state.mu.Unlock()
	if !current {
		return
	}
	if state.platform.cursorPosition != nil {
		if pointer := state.platform.cursorPosition(); pointer != nil {
			state.mu.Lock()
			local := Point{X: pointer.X - state.expandedBounds.X, Y: pointer.Y - state.expandedBounds.Y}
			onTrigger := screenshotEditorRectContains(state.formatRect, local)
			state.mu.Unlock()
			if onTrigger {
				return
			}
		}
	}
	state.closeFormatMenu()
}

// closeFormatMenu detaches before closing native resources to allow focus and close callbacks to reenter.
func (state *recordingToolbarState) closeFormatMenu() {
	state.mu.Lock()
	menu := state.formatMenu
	state.formatMenu = nil
	state.mu.Unlock()
	if menu == nil {
		return
	}
	if menu.window != nil {
		_ = menu.window.Close()
	} else if menu.host != nil {
		menu.host.Dispose()
	}
	if state.window != nil {
		_ = state.window.Invalidate()
	}
}

// build keeps popup hover, focus, activation, and labels on the shared button component.
func (menu *recordingFormatMenu) build(_ FrameInfo) woxwidget.Widget {
	labels := menu.owner.options.RecordingTooltips
	theme := menu.owner.options.Theme
	theme.DensityScale = menu.scale
	theme.Surface = Color{R: 30, G: 26, B: 24, A: 255}
	theme.Text = Color{R: 255, G: 255, B: 255, A: 255}
	theme.SelectionBackground = Color{R: 47, G: 128, B: 237, A: 255}
	theme.SelectionText = theme.Text
	children := make([]woxwidget.Widget, 0, 3)
	for index, label := range []string{labels.FormatMP4, labels.FormatGIF, labels.FormatWebP} {
		format := recordingExportFormat(index)
		variant := woxcomponent.ButtonText
		if format == menu.selected {
			variant = woxcomponent.ButtonSelected
		}
		children = append(children, woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: "recording.format." + format.extension(), Label: label,
			Width: menu.size.Width - 16*menu.scale, Height: 32 * menu.scale,
			Padding: woxwidget.Insets{Left: 12 * menu.scale, Right: 12 * menu.scale}, Radius: 4 * menu.scale,
			AlignLeading: true, Variant: variant, Theme: theme, OnTap: func() { menu.choose(format) },
		}))
	}
	return woxwidget.Container{
		Width: menu.size.Width, Height: menu.size.Height, Color: theme.Surface, Radius: 8 * menu.scale,
		Padding: woxwidget.Insets{Left: 8 * menu.scale, Right: 8 * menu.scale, Top: 8 * menu.scale, Bottom: 8 * menu.scale},
		Child:   woxwidget.Flex{Axis: woxwidget.Vertical, Children: children},
	}
}

// key supports arrow selection, Enter to commit, and Escape to dismiss without cancelling the recording.
func (menu *recordingFormatMenu) key(event KeyEvent) bool {
	if event.Down {
		switch event.Key {
		case KeyEscape:
			menu.owner.closeFormatMenu()
			return true
		case KeyArrowUp, KeyArrowDown:
			delta := 1
			if event.Key == KeyArrowUp {
				delta = 2
			}
			menu.selected = recordingExportFormat((int(menu.selected) + delta) % 3)
			menu.host.RequestFocus(woxwidget.Key("recording.format." + menu.selected.extension()))
			if menu.window != nil {
				_ = menu.window.Window().Invalidate()
			}
			return true
		case KeyEnter:
			if menu.host.FocusedKey() != "" {
				return menu.host.Key(event)
			}
			menu.choose(menu.selected)
			return true
		}
	}
	return menu.host.Key(event)
}

// choose commits only while the finalized take still accepts export options.
func (menu *recordingFormatMenu) choose(format recordingExportFormat) {
	state := menu.owner
	state.mu.Lock()
	if format.valid() && !state.finishing && !state.cancelled && state.session != nil && state.session.currentState() == recordingStateSave {
		state.exportFormat = format
	}
	state.mu.Unlock()
	state.closeFormatMenu()
	if state.window != nil {
		_ = state.window.Invalidate()
	}
}
