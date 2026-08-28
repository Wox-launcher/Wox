package screenshot

import (
	"context"
	"fmt"
	"image"
	"math"
	"runtime"
	"strconv"
	"strings"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

// screenshotSizeDialog owns the draft values; the capture selection changes only on Apply.
type screenshotSizeDialog struct {
	editor        *screenshotEditorOverlayState
	window        *ManagedWindow
	host          *woxwidget.Host
	width, height *woxwidget.TextEditingController
	maxWidth      int
	maxHeight     int
	invalid       bool
	lockRatio     bool
	aspectRatio   float64
}

// publishSizeLabel gives assistive technology the same entry point as clicking the capture dimensions.
func (state *screenshotEditorOverlayState) publishSizeLabel(bounds Rect, value string) {
	if state.window == nil {
		return
	}
	tree := woxui.AccessibilityTree{Generation: 1}
	if bounds.Width > 0 {
		tree.RootIDs = []woxui.AccessibilityNodeID{1}
		tree.Nodes = []woxui.AccessibilityNode{{
			ID: 1, AutomationID: "screenshot.size.open", Role: woxui.AccessibilityRoleButton,
			Label: state.sizeDialogOptions.SizeLabels.Title, Value: value, Bounds: bounds,
			Enabled: state.activeSizeDialog() == nil, Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
		}}
	}
	_ = state.window.UpdateAccessibility(tree, func(_ woxui.AccessibilityNodeID, action woxui.AccessibilityAction, _ string) error {
		if action == woxui.AccessibilityActionActivate {
			state.openSizeDialog()
		}
		return nil
	})
}

func (state *screenshotEditorOverlayState) activeSizeDialog() *screenshotSizeDialog {
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.sizeDialog
}

// openSizeDialog uses a separate native surface so ordinary controls retain logical sizing on mixed-DPI desktops.
func (state *screenshotEditorOverlayState) openSizeDialog() bool {
	state.mu.Lock()
	if !state.hasSelection || state.dragging || state.annotationDragging || state.editMode != screenshotEditorEditNone || state.textEditing || state.autoConfirm || state.scrolling || state.scrollingStarting || state.saving || state.sizeDialog != nil || state.image == nil {
		state.mu.Unlock()
		return false
	}
	pixels, err := screenshotEditorPixelSelection(image.Rect(0, 0, state.image.Width, state.image.Height), state.selection, state.frameSize)
	if err != nil {
		state.mu.Unlock()
		return false
	}
	dialog := &screenshotSizeDialog{
		editor: state, width: woxwidget.NewTextEditingController(strconv.Itoa(pixels.Dx())), height: woxwidget.NewTextEditingController(strconv.Itoa(pixels.Dy())),
		maxWidth: state.image.Width - pixels.Min.X, maxHeight: state.image.Height - pixels.Min.Y,
	}
	dialog.width.SelectAll()
	state.sizeDialog = dialog
	state.hasHoveredMark, state.hasHoveredTool, state.hasHoveredAction = false, false, false
	state.mu.Unlock()
	state.setTextInputEnabled(false)
	state.invalidate()
	if state.window == nil {
		return true
	}
	if err := dialog.open(); err != nil {
		util.GetLogger().Warn(context.Background(), fmt.Sprintf("failed to open screenshot size dialog: %s", err.Error()))
		state.closeSizeDialog(true)
	}
	return true
}

// open keeps the dialog above the capture surface without changing the screenshot's coordinate system.
func (dialog *screenshotSizeDialog) open() error {
	options := dialog.editor.sizeDialogOptions
	manager := options.WindowManager
	if manager == nil {
		manager = NewWindowManager()
	}
	role := WindowRoleUtility
	if runtime.GOOS == "darwin" {
		// macOS capture surfaces sit above normal topmost panels; this role shares their level and still uses logical points.
		role = WindowRoleScreenshot
	}
	size := dialog.size()
	dialog.host = woxwidget.NewHost(dialog.build)
	managed, _, err := manager.Open("wox.screenshot.size", WindowOptions{
		Title: options.SizeLabels.Title, Size: size, Role: role, Topmost: true,
		OnFrame: dialog.draw, OnPointer: dialog.host.Pointer, OnKey: dialog.key,
		OnTextInput: func(event TextInputEvent) { dialog.host.TextInput(event) },
		OnFocus:     func(event woxui.FocusEvent) { dialog.host.SetWindowFocused(event.Active) },
		OnClosed:    dialog.closed,
	})
	if err != nil {
		dialog.host.Dispose()
		return err
	}
	dialog.window = managed
	window := managed.Window()
	dialog.host.Attach(window)
	if err := window.SetFontFamily(options.FontFamily); err != nil {
		return err
	}
	if err := window.CenterOnMouseScreen(size); err != nil {
		return err
	}
	_, err = managed.Show()
	return err
}

// closed detaches first: WindowManager finishes Close only after this callback returns, so it must not be reentered.
func (dialog *screenshotSizeDialog) closed() {
	dialog.window = nil
	dialog.host.Dispose()
	if dialog.editor.activeSizeDialog() == dialog {
		dialog.editor.closeSizeDialog(true)
	}
}

// draw leaves Windows' outer corners to DWM instead of layering a second rounded outline over its backdrop.
func (dialog *screenshotSizeDialog) draw(displayList *DisplayList, frame FrameInfo) {
	dialog.host.Frame(displayList, frame)
	if runtime.GOOS == "windows" {
		background := dialog.editor.sizeDialogOptions.Theme.ActionBackground
		background.A = 255
		displayList.Clear(background)
	}
}

// size fits the form's fixed line slots, standard controls, gaps, and 20-unit outer padding.
func (dialog *screenshotSizeDialog) size() Size {
	height := float32(220)
	if dialog.invalid {
		height += 48 // Two error lines plus the same 12-unit gap used by settings dialogs.
	}
	return Size{Width: 360, Height: height}
}

// setInvalid expands only for visible validation, preserving the native window's logical desktop origin.
func (dialog *screenshotSizeDialog) setInvalid(invalid bool) {
	if dialog.invalid == invalid {
		return
	}
	dialog.invalid = invalid
	if dialog.window == nil {
		return
	}
	window := dialog.window.Window()
	bounds, err := window.Bounds()
	if err == nil {
		bounds.Height = dialog.size().Height
		err = window.SetBounds(bounds)
	}
	if err != nil {
		util.GetLogger().Warn(context.Background(), fmt.Sprintf("failed to resize screenshot size dialog: %s", err.Error()))
	}
	_ = window.Invalidate()
}

// build composes shared form controls in logical units; the native dialog owns DPI conversion.
func (dialog *screenshotSizeDialog) build(frame FrameInfo) woxwidget.Widget {
	options := dialog.editor.sizeDialogOptions
	labels, theme := options.SizeLabels, options.Theme
	theme.ActionBackground.A = 255
	var window *Window
	if dialog.window != nil {
		window = dialog.window.Window()
	}
	fields := make([]woxwidget.Widget, 0, 2)
	for index, field := range []struct {
		id, label  string
		controller *woxwidget.TextEditingController
	}{{"width", labels.Width, dialog.width}, {"height", labels.Height, dialog.height}} {
		fields = append(fields, woxwidget.Expanded{Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: []woxwidget.Widget{
			woxwidget.TextBlock{Value: field.label, Height: 20, MaxLines: 1, Style: TextStyle{Size: woxcomponent.SettingsLabelFontSize}, Color: theme.ResultTitle},
			// Text fields need their actual width for text clipping and pointer-to-caret mapping.
			woxwidget.LayoutBuilder{Build: func(size Size) woxwidget.Widget {
				return woxcomponent.WoxSettingTextField(woxcomponent.TextFieldProps{
					ID: "screenshot.size." + field.id, Label: field.label, Width: size.Width, Controller: field.controller, Window: window, Theme: theme,
					OnChanged: func(value string) { dialog.changeDimension(index == 0, value) },
					OnKey: func(event KeyEvent) bool {
						if event.Down && !event.Composing && event.Key == KeyEnter {
							dialog.apply()
							return true
						}
						return false
					},
				})
			}},
		}}})
	}
	swap := woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
		ID: "screenshot.size.swap", Label: labels.Swap, Width: 32, Height: 32, Radius: 4,
		HoverBackground: theme.QueryBackground, FocusRingColor: theme.Cursor, OnTap: dialog.swapDimensions,
		Icon: woxwidget.Painter{Width: 16, Height: 16, Paint: func(displayList *DisplayList, bounds Rect) {
			drawScreenshotEditorToolbarIconSized(displayList, "control.swap", bounds, theme.ResultTitle, 1, 16)
		}},
	})
	children := []woxwidget.Widget{
		woxwidget.TextBlock{Value: labels.Title, Height: 20, MaxLines: 1, Style: TextStyle{Size: woxcomponent.SettingsLabelFontSize, Weight: FontWeightSemibold}, Color: theme.ResultTitle},
		woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 16, CrossAxisAlignment: woxwidget.CrossAxisEnd, Children: []woxwidget.Widget{fields[0], swap, fields[1]}},
		woxwidget.Semantics{AutomationID: "screenshot.size.lock-row", Role: woxui.AccessibilityRoleGroup, Label: labels.LockAspectRatio,
			Child: woxwidget.Gesture{ID: "screenshot.size.lock-row", OnTap: func() { dialog.setRatioLocked(!dialog.lockRatio) }, Child: woxwidget.Flex{
				Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
					woxwidget.Align{Width: 32, Height: 32, Horizontal: 0.5, Vertical: 0.5, Child: woxcomponent.WoxCheckbox(woxcomponent.CheckboxProps{
						ID: "screenshot.size.lock", Label: labels.LockAspectRatio, Value: dialog.lockRatio, OnChange: dialog.setRatioLocked, Theme: theme,
					})},
					woxwidget.Text{Value: labels.LockAspectRatio, Style: TextStyle{Size: woxcomponent.SettingsLabelFontSize}, Color: theme.ResultTitle},
				},
			}}},
	}
	if dialog.invalid {
		errorText := fmt.Sprintf(labels.InvalidSize, dialog.maxWidth, dialog.maxHeight)
		children = append(children, woxwidget.Semantics{AutomationID: "screenshot.size.error", Role: woxui.AccessibilityRoleText, Label: errorText, LiveRegion: woxui.AccessibilityLiveRegionPolite,
			Child: woxwidget.TextBlock{Value: errorText, Height: 36, LineHeight: 18, MaxLines: 2, Style: TextStyle{Size: woxcomponent.SettingsHelpFontSize}, Color: theme.ErrorText}})
	}
	children = append(children, woxwidget.Flex{Axis: woxwidget.Horizontal, MainAxisAlignment: woxwidget.MainAxisEnd, Gap: 8, Children: []woxwidget.Widget{
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "screenshot.size.cancel", Label: labels.Cancel, Theme: theme, OnTap: func() { dialog.editor.closeSizeDialog(true) }}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "screenshot.size.apply", Label: labels.Apply, Theme: theme, Variant: woxcomponent.ButtonPrimary, OnTap: dialog.apply}),
	}})
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "screenshot.size", Label: labels.Title, Width: frame.Size.Width, Height: frame.Size.Height,
		Padding: woxwidget.UniformInsets(20), Theme: theme,
		InitialFocus: "screenshot.size.width",
		OnEscape:     func() { dialog.editor.closeSizeDialog(true) },
		Child:        woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: children},
	})
}

// setRatioLocked snapshots the capture's pixel ratio, not a possibly edited or invalid draft ratio.
func (dialog *screenshotSizeDialog) setRatioLocked(locked bool) {
	dialog.lockRatio = locked
	if !locked {
		return
	}
	state := dialog.editor
	state.mu.Lock()
	pixels, err := screenshotEditorPixelSelection(image.Rect(0, 0, state.image.Width, state.image.Height), state.selection, state.frameSize)
	state.mu.Unlock()
	if err != nil {
		dialog.lockRatio = false
		return
	}
	dialog.aspectRatio = float64(pixels.Dx()) / float64(pixels.Dy())
	dialog.changeDimension(true, dialog.width.Text())
}

// changeDimension rounds the linked dimension to a whole pixel without truncating out-of-range drafts.
func (dialog *screenshotSizeDialog) changeDimension(widthChanged bool, value string) {
	dialog.setInvalid(false)
	number, err := strconv.Atoi(strings.TrimSpace(value))
	if !dialog.lockRatio || err != nil || number < 1 {
		return
	}
	other, ratio := dialog.height, 1/dialog.aspectRatio
	if !widthChanged {
		other, ratio = dialog.width, dialog.aspectRatio
	}
	other.SetText(strconv.FormatFloat(max(1, math.Round(float64(number)*ratio)), 'f', 0, 64), false)
}

// swapDimensions rotates both the draft and its locked ratio, so subsequent edits keep the new orientation.
func (dialog *screenshotSizeDialog) swapDimensions() {
	width, height := dialog.width.Text(), dialog.height.Text()
	dialog.width.SetText(height, false)
	dialog.height.SetText(width, false)
	if dialog.lockRatio {
		dialog.aspectRatio = 1 / dialog.aspectRatio
	}
	dialog.setInvalid(false)
}

// key confines keyboard actions to the size form while it is open.
func (dialog *screenshotSizeDialog) key(event KeyEvent) bool {
	if dialog.host != nil && dialog.host.Key(event) {
		return true
	}
	if event.Down && !event.Composing && event.Key == KeyEscape {
		dialog.editor.closeSizeDialog(true)
		return true
	}
	return false
}

// apply validates both dimensions before touching the capture or its annotations.
func (dialog *screenshotSizeDialog) apply() {
	width, widthErr := strconv.Atoi(strings.TrimSpace(dialog.width.Text()))
	height, heightErr := strconv.Atoi(strings.TrimSpace(dialog.height.Text()))
	dialog.setInvalid(widthErr != nil || heightErr != nil || width < 1 || height < 1 || width > dialog.maxWidth || height > dialog.maxHeight)
	if dialog.invalid {
		return
	}
	state := dialog.editor
	state.mu.Lock()
	state.selection = screenshotEditorSelectionWithPixelSize(state.selection, state.frameSize, image.Pt(state.image.Width, state.image.Height), width, height)
	state.mu.Unlock()
	state.closeSizeDialog(true)
}

// screenshotEditorSelectionWithPixelSize preserves the upper-left corner and matches the export crop's outward rounding.
// Callers validate dimensions against the remaining captured pixels before applying them.
func screenshotEditorSelectionWithPixelSize(selection Rect, frame Size, pixels image.Point, width, height int) Rect {
	scaleX, scaleY := float32(pixels.X)/frame.Width, float32(pixels.Y)/frame.Height
	left, top := int(math.Floor(float64(selection.X*scaleX))), int(math.Floor(float64(selection.Y*scaleY)))
	right, bottom := float32(left+width)/scaleX, float32(top+height)/scaleY
	selection.Width, selection.Height = right-selection.X, bottom-selection.Y
	// Division and subtraction can round an exact pixel edge upward. Move only that edge inward by one float step.
	for math.Ceil(float64((selection.X+selection.Width)*scaleX)) > float64(left+width) {
		right = math.Nextafter32(right, float32(math.Inf(-1)))
		selection.Width = right - selection.X
	}
	for math.Ceil(float64((selection.Y+selection.Height)*scaleY)) > float64(top+height) {
		bottom = math.Nextafter32(bottom, float32(math.Inf(-1)))
		selection.Height = bottom - selection.Y
	}
	return selection
}

// closeSizeDialog disposes the draft surface before restoring the screenshot's keyboard focus.
func (state *screenshotEditorOverlayState) closeSizeDialog(restoreFocus bool) {
	state.mu.Lock()
	dialog := state.sizeDialog
	state.sizeDialog = nil
	state.mu.Unlock()
	if dialog == nil {
		return
	}
	if dialog.window != nil {
		_ = dialog.window.Close()
	}
	state.invalidate()
	if restoreFocus && state.window != nil {
		_, _ = state.window.Show()
	}
}
