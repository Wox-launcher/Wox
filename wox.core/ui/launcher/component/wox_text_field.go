package component

import (
	"strings"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	textFieldLineHeight       = float32(20)
	textFieldCursorWidth      = float32(2)
	textFieldContextMenuWidth = float32(140)
	textFieldContextMenuRowH  = float32(28)
)

type textFieldLine struct {
	start int
	end   int
	text  string
}

type textFieldContextAction uint8

const (
	textFieldContextCut textFieldContextAction = iota
	textFieldContextCopy
	textFieldContextPaste
	textFieldContextSelectAll
)

// textFieldMenuEnablement is the Cut/Copy/Paste/Select All snapshot shown in the context menu.
type textFieldMenuEnablement struct {
	canCut, canCopy, canPaste, canSelectAll bool
}

// computeTextFieldMenuEnablement derives menu enablement from the live editor and field props.
func computeTextFieldMenuEnablement(props TextFieldProps, editing woxui.TextEditingState) textFieldMenuEnablement {
	hasSelection := !editing.Selection.Collapsed()
	canMutate := !props.ReadOnly && !props.Disabled
	canCopy := hasSelection && !props.Protected
	return textFieldMenuEnablement{
		canCut: canCopy && canMutate, canCopy: canCopy, canPaste: canMutate, canSelectAll: editing.Text != "",
	}
}

// TextFieldProps describes a retained Wox text field and its business-value callbacks.
type TextFieldProps struct {
	ID               string
	Label            string
	Hint             string
	Width            float32
	Height           float32
	Radius           float32
	Padding          woxwidget.Insets
	Background       woxui.Color
	Transparent      bool
	BorderColor      woxui.Color
	BorderWidth      float32
	FocusRingColor   woxui.Color
	FocusRingOutsets woxwidget.Insets
	Style            woxui.TextStyle
	TextColor        woxui.Color
	// TextAlignmentY optically positions measured glyph bounds within each line without moving the caret.
	TextAlignmentY     float32
	Value              string
	Focused            bool
	Autofocus          bool
	Controller         *woxwidget.TextEditingController
	FocusNode          *woxwidget.FocusNode
	Disabled           bool
	DisableHover       bool
	ReadOnly           bool
	Protected          bool
	MaxLines           int
	Window             *woxui.Window
	Theme              Theme
	OnKey              func(woxui.KeyEvent) bool
	OnFocusChange      func(bool)
	OnChanged          func(string)
	OnSelectionChanged func(woxui.TextSelection)
	OnSetValue         func(string) error
	editingState       woxui.TextEditingState
	onCaret            func(int)
	onWordSelection    func(int)
	onLineSelection    func(int)
	// onSelectionStart begins a drag selection anchored at the given rune offset.
	onSelectionStart func(int, woxui.KeyModifiers)
	// onSelectionExtendAt updates selection from a local pointer point, including edge auto-scroll.
	onSelectionExtendAt func(woxui.Point)
	// onSelectionEnd stops continuous edge auto-scroll when the drag selection finishes.
	onSelectionEnd func()
	onSecondaryTap func(woxui.Point) // window/client coordinates for the context menu anchor
	onHoverAt      func(bool, woxui.Rect)
	hovered        bool
	onScroll       func(woxui.Point) bool
	onTextInput    func(woxui.TextInputEvent) bool
	verticalOffset float32
	// caretActive is the declared Focused value. CaretPainter uses it so Boundary
	// verify shadows (a fresh unfocused FocusNode) match the live tree.
	caretActive bool
}

// WoxTextField builds a retained text field with shared IME, selection, and accessibility behavior.
func WoxTextField(props TextFieldProps) woxwidget.Widget {
	if props.MaxLines <= 1 && props.TextAlignmentY == 0 {
		props.TextAlignmentY = 0.5
	}
	return woxwidget.Stateful{
		Key: woxwidget.Key(props.ID), Type: (*textFieldState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &textFieldState{} },
	}
}

type textFieldState struct {
	hovered            bool
	controller         *woxwidget.TextEditingController
	internalController *woxwidget.TextEditingController
	focusNode          *woxwidget.FocusNode
	internalFocusNode  *woxwidget.FocusNode
	focusAttachment    *woxwidget.FocusAttachment
	// selectionAnchor holds the rune offset captured at drag-selection start so extend updates only the focus.
	selectionAnchor  int
	verticalOffset   float32
	lastFocus        int
	hasLastFocus     bool
	innerHeight      float32
	innerWidth       float32
	style            woxui.TextStyle
	maxLines         int
	overlayOwner     woxwidget.Key
	overlayToken     uint64
	menuAnchor       woxui.Point
	menuEnablement   textFieldMenuEnablement
	menuActionRunner func(textFieldContextAction)
	// dragScrollEdge is the signed overflow distance while drag-selecting past the viewport edge.
	// Scroll steps chain through PostFrame callbacks started on first edge overflow.
	dragScrollEdge      float32
	dragScrollLocal     woxui.Point
	dragScrollMaxOff    float32
	dragScrollWindow    *woxui.Window
	dragProtected       bool
	dragInvalidate      func()
	dragNotifySelect    func()
	dragScrollScheduled bool
	dragPostFrame       func(func())
}

// InitState creates fallback controller and focus objects when the caller does not supply them.
func (s *textFieldState) InitState(context woxwidget.StateContext, widget any) {
	props := widget.(TextFieldProps)
	s.updateBindings(context, props)
	if props.Focused {
		context.PostFrame(func() { s.focusNode.RequestFocus() })
	}
}

// DidUpdateWidget applies programmatic value and focus changes without replacing local selection or composition.
func (s *textFieldState) DidUpdateWidget(context woxwidget.StateContext, oldWidget, newWidget any) {
	oldProps := oldWidget.(TextFieldProps)
	newProps := newWidget.(TextFieldProps)
	s.updateBindings(context, newProps)
	if newProps.Controller == nil && (oldProps.Controller != nil || oldProps.Value != newProps.Value) && s.controller.Text() != newProps.Value {
		s.controller.SetText(newProps.Value, false)
	}
	if newProps.Focused != oldProps.Focused {
		if newProps.Focused {
			context.PostFrame(func() { s.focusNode.RequestFocus() })
		} else {
			context.PostFrame(func() { s.focusNode.Unfocus() })
		}
	}
	if s.overlayToken != 0 && (newProps.ReadOnly != oldProps.ReadOnly || newProps.Protected != oldProps.Protected || newProps.Disabled != oldProps.Disabled) {
		context.PostFrame(func() {
			s.refreshContextMenuIfStale(context, newProps, s.overlayOwner, s.menuActionRunner)
		})
	}
}

// Build connects retained editor state to the Host's single EditableText focus and IME path.
func (s *textFieldState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(TextFieldProps)
	s.updateBindings(context, props)
	s.dragPostFrame = func(callback func()) { context.PostFrame(callback) }
	realState := s.controller.State()
	displayState := realState
	if props.Protected {
		displayState.Text = woxui.MaskProtectedText(realState.Text)
		displayState.Composition = woxui.MaskProtectedText(realState.Composition)
		displayState.Selection = woxui.MapSelectionToProtectedDisplay(realState.Text, realState.Selection)
	}
	props.editingState = displayState
	props.caretActive = props.Focused
	props.Focused = s.focusNode.HasFocus()
	style := props.Style
	if style.Size <= 0 {
		style = woxui.TextStyle{Size: SettingsControlFontSize}
	}
	s.style = style
	s.maxLines = max(1, props.MaxLines)
	padding := props.Padding
	if padding == (woxwidget.Insets{}) {
		padding = woxwidget.Insets{Left: 12, Top: 9, Right: 12, Bottom: 7}
	}
	height := props.Height
	if height <= 0 {
		height = 40
	}
	innerWidth := max(float32(0), props.Width-padding.Left-padding.Right)
	innerHeight := max(float32(0), height-padding.Top-padding.Bottom)
	s.innerWidth = innerWidth
	s.innerHeight = innerHeight
	softWrap := s.maxLines > 1
	lines := textFieldLines(displayState.Text, props.Window, style, innerWidth, softWrap)
	caretLine := textFieldLineIndex(lines, displayState.Selection.Focus)
	maximumVerticalOffset := max(float32(0), float32(len(lines))*textFieldLineHeight-innerHeight)
	s.verticalOffset = min(s.verticalOffset, maximumVerticalOffset)
	if !s.hasLastFocus || s.lastFocus != displayState.Selection.Focus {
		caretTop := float32(caretLine) * textFieldLineHeight
		if caretTop < s.verticalOffset {
			s.verticalOffset = caretTop
		} else if caretTop+textFieldLineHeight > s.verticalOffset+innerHeight {
			s.verticalOffset = caretTop + textFieldLineHeight - innerHeight
		}
	}
	s.lastFocus = displayState.Selection.Focus
	s.hasLastFocus = true
	props.verticalOffset = s.verticalOffset
	props.Controller = nil
	props.FocusNode = nil
	invalidate := func() { context.Invalidate() }
	ownerKey := woxwidget.Key(props.ID)
	s.overlayOwner = ownerKey
	closeContextMenu := func() {
		context.ClearHostOverlay(ownerKey, s.overlayToken)
		s.overlayToken = 0
	}
	var notifyChanged, notifySelection func()
	var runContextAction func(textFieldContextAction)
	notifyChanged = func() {
		notifyTextFieldChanged(widget.(TextFieldProps), s.controller.Text())
		s.refreshContextMenuIfStale(context, widget.(TextFieldProps), ownerKey, runContextAction)
	}
	notifySelection = func() {
		notifyTextFieldSelectionChanged(widget.(TextFieldProps), s.controller.State().Selection)
		s.refreshContextMenuIfStale(context, widget.(TextFieldProps), ownerKey, runContextAction)
	}
	mapHitOffset := func(displayOffset int) int {
		original := widget.(TextFieldProps)
		if !original.Protected {
			return displayOffset
		}
		return woxui.MapProtectedDisplayOffsetToRune(s.controller.Text(), displayOffset)
	}
	copySelection := func() bool {
		original := widget.(TextFieldProps)
		if original.Protected {
			return true
		}
		provider := currentClipboard()
		if provider == nil {
			return false
		}
		if selected := s.controller.SelectedText(); selected != "" {
			if err := provider.WriteText(selected); err != nil {
				return false
			}
		}
		return true
	}
	cutSelection := func() bool {
		original := widget.(TextFieldProps)
		if original.Protected || original.ReadOnly {
			return true
		}
		provider := currentClipboard()
		if provider == nil {
			return false
		}
		if selected := s.controller.SelectedText(); selected != "" {
			if err := provider.WriteText(selected); err != nil {
				return false
			}
			if s.controller.DeleteSelection() {
				notifyChanged()
			}
			notifySelection()
			invalidate()
		}
		return true
	}
	pasteClipboard := func() bool {
		original := widget.(TextFieldProps)
		if original.ReadOnly {
			return true
		}
		provider := currentClipboard()
		if provider == nil {
			return false
		}
		text, err := provider.ReadText()
		if err != nil || text == "" {
			return true
		}
		if max(1, original.MaxLines) <= 1 {
			text = woxui.FilterSingleLineNewlines(text)
		}
		if text == "" {
			return true
		}
		if s.controller.InsertTextSeparate(text) {
			notifyChanged()
		}
		notifySelection()
		invalidate()
		return true
	}
	runContextAction = func(action textFieldContextAction) {
		closeContextMenu()
		en := computeTextFieldMenuEnablement(widget.(TextFieldProps), s.controller.State())
		switch action {
		case textFieldContextCut:
			if !en.canCut {
				return
			}
			_ = cutSelection()
		case textFieldContextCopy:
			if !en.canCopy {
				return
			}
			_ = copySelection()
		case textFieldContextPaste:
			if !en.canPaste {
				return
			}
			_ = pasteClipboard()
		case textFieldContextSelectAll:
			if !en.canSelectAll {
				return
			}
			s.controller.SelectAll()
			notifySelection()
		}
		invalidate()
	}
	s.menuActionRunner = runContextAction
	props.onCaret = func(offset int) {
		s.focusNode.RequestFocus()
		closeContextMenu()
		s.controller.SetCaret(mapHitOffset(offset))
		notifySelection()
		invalidate()
	}
	props.onWordSelection = func(offset int) {
		s.focusNode.RequestFocus()
		closeContextMenu()
		s.controller.SelectWordAt(mapHitOffset(offset))
		notifySelection()
		invalidate()
	}
	props.onLineSelection = func(offset int) {
		s.focusNode.RequestFocus()
		closeContextMenu()
		s.controller.SelectLineAt(mapHitOffset(offset))
		notifySelection()
		invalidate()
	}
	props.onSelectionStart = func(offset int, modifiers woxui.KeyModifiers) {
		s.focusNode.RequestFocus()
		closeContextMenu()
		offset = mapHitOffset(offset)
		if modifiers&woxui.KeyModifierShift != 0 {
			s.selectionAnchor = s.controller.State().Selection.Anchor
			s.controller.SetSelection(s.selectionAnchor, offset)
		} else {
			s.selectionAnchor = offset
			s.controller.SetCaret(offset)
		}
		notifySelection()
		invalidate()
	}
	props.onSecondaryTap = func(windowPos woxui.Point) {
		original := widget.(TextFieldProps)
		if original.Disabled {
			return
		}
		s.focusNode.RequestFocus()
		openTextFieldContextMenu(context, original, s, ownerKey, windowPos, runContextAction)
	}
	props.onScroll = func(delta woxui.Point) bool {
		if s.maxLines <= 1 || maximumVerticalOffset <= 0 || delta.Y == 0 {
			return false
		}
		next, changed := textFieldScrolledOffset(s.verticalOffset, maximumVerticalOffset, delta.Y)
		if !changed {
			return false
		}
		s.verticalOffset = next
		invalidate()
		return true
	}
	// Live drag extend keeps scroll and hit-testing on the retained vertical offset.
	props.onSelectionExtendAt = func(position woxui.Point) {
		s.focusNode.RequestFocus()
		original := widget.(TextFieldProps)
		fieldPadding := original.Padding
		if fieldPadding == (woxwidget.Insets{}) {
			fieldPadding = woxwidget.Insets{Left: 12, Top: 9, Right: 12, Bottom: 7}
		}
		local := woxui.Point{X: max(float32(0), position.X-fieldPadding.Left), Y: position.Y - fieldPadding.Top}
		s.dragScrollWindow = original.Window
		s.dragProtected = original.Protected
		s.dragInvalidate = invalidate
		s.dragNotifySelect = notifySelection
		s.dragScrollMaxOff = maximumVerticalOffset
		if s.maxLines > 1 {
			if local.Y < 0 {
				s.dragScrollEdge = local.Y
			} else if local.Y > s.innerHeight {
				s.dragScrollEdge = local.Y - s.innerHeight
			} else {
				s.dragScrollEdge = 0
				s.stopDragScroll()
			}
			s.dragScrollLocal = local
			if s.dragScrollEdge != 0 {
				s.scheduleDragScroll(context)
			}
			local.Y = max(float32(0), min(s.innerHeight, local.Y))
		} else {
			s.dragScrollEdge = 0
			s.stopDragScroll()
		}
		s.applyDragSelectionAt(local)
	}
	props.onSelectionEnd = func() { s.stopDragScroll() }
	props.OnKey = func(event woxui.KeyEvent) bool {
		original := widget.(TextFieldProps)
		if original.Disabled {
			return false
		}
		if event.Down && event.Key == woxui.KeyEscape && context.HasHostOverlay() && context.HostOverlayOwner() == ownerKey {
			closeContextMenu()
			return true
		}
		allowsMutation := !original.ReadOnly
		if event.Down && !event.Composing && event.Modifiers.HasPrimary() {
			switch event.Key {
			case woxui.Key("a"):
				s.controller.SelectAll()
				notifySelection()
				invalidate()
				return true
			case woxui.Key("c"):
				return copySelection()
			case woxui.Key("x"):
				if !allowsMutation || original.Protected {
					return true
				}
				return cutSelection()
			case woxui.Key("v"):
				if !allowsMutation {
					return true
				}
				return pasteClipboard()
			case woxui.Key("z"), woxui.Key("y"), woxui.KeyBackspace:
				if !allowsMutation {
					return true
				}
				handled, changed := s.controller.HandleKey(event)
				if changed {
					notifyChanged()
				}
				if handled {
					notifySelection()
					invalidate()
				}
				return handled
			}
		}
		if !allowsMutation {
			// Read-only fields still own navigation, selection, and copy shortcuts.
			if event.Down && !event.Composing {
				switch event.Key {
				case woxui.KeyArrowLeft, woxui.KeyArrowRight, woxui.KeyArrowUp, woxui.KeyArrowDown,
					woxui.KeyHome, woxui.KeyEnd, woxui.KeyPageUp, woxui.KeyPageDown:
					handled, _ := handleTextFieldControllerKey(s.controller, s.maxLines, lines, s.innerHeight, original.Window, s.style, s.innerWidth, s.maxLines > 1, event)
					if handled {
						notifySelection()
						invalidate()
					}
					return handled
				case woxui.KeyBackspace, woxui.KeyDelete, woxui.KeyEnter:
					return true
				}
			}
			if original.OnKey != nil && original.OnKey(event) {
				return true
			}
			return false
		}
		if original.OnKey != nil && original.OnKey(event) {
			return true
		}
		handled, changed := handleTextFieldControllerKey(s.controller, s.maxLines, lines, s.innerHeight, original.Window, s.style, s.innerWidth, s.maxLines > 1, event)
		if handled {
			if changed {
				notifyChanged()
			}
			notifySelection()
			invalidate()
		}
		return handled
	}
	props.onTextInput = func(event woxui.TextInputEvent) bool {
		original := widget.(TextFieldProps)
		if original.Disabled || original.ReadOnly {
			return false
		}
		if event.Kind == woxui.TextInputCommit && max(1, original.MaxLines) <= 1 {
			event.Text = woxui.FilterSingleLineNewlines(event.Text)
			if event.Text == "" {
				s.controller.HandleTextInput(woxui.TextInputEvent{Kind: woxui.TextInputCommit})
				notifySelection()
				invalidate()
				return true
			}
		}
		changed := s.controller.HandleTextInput(event)
		if changed {
			notifyChanged()
		}
		notifySelection()
		invalidate()
		return true
	}
	props.OnFocusChange = func(focused bool) {
		s.focusNode.UpdateFocus(focused)
		if !focused {
			closeContextMenu()
		}
		if original := widget.(TextFieldProps).OnFocusChange; original != nil {
			original(focused)
		}
		invalidate()
	}
	props.OnSetValue = func(value string) error {
		original := widget.(TextFieldProps)
		if max(1, original.MaxLines) <= 1 {
			value = woxui.FilterSingleLineNewlines(value)
		}
		s.controller.SetText(value, false)
		notifyChanged()
		notifySelection()
		invalidate()
		if original.OnSetValue != nil {
			return original.OnSetValue(value)
		}
		return nil
	}
	hoverEnabled := !props.Disabled && !props.DisableHover
	props.hovered = s.hovered && hoverEnabled && !props.Focused
	if hoverEnabled {
		props.onHoverAt = func(inside bool, _ woxui.Rect) {
			if inside != s.hovered {
				context.SetState(func() { s.hovered = inside })
			}
		}
	}
	field := buildWoxTextField(props, realState, copySelection, cutSelection, pasteClipboard, runContextAction)
	return field
}

// Dispose detaches the state-owned focus binding from its window Host.
func (s *textFieldState) Dispose() {
	s.stopDragScroll()
	if s.focusAttachment != nil {
		s.focusAttachment.Detach()
		s.focusAttachment = nil
	}
}

func (s *textFieldState) stopDragScroll() {
	s.dragScrollEdge = 0
}

// scheduleDragScroll queues one PostFrame scroll step and re-schedules the next while past the edge.
func (s *textFieldState) scheduleDragScroll(_ woxwidget.StateContext) {
	if s.dragScrollScheduled || s.dragScrollEdge == 0 || s.maxLines <= 1 || s.dragPostFrame == nil {
		return
	}
	s.dragScrollScheduled = true
	s.dragPostFrame(func() {
		s.dragScrollScheduled = false
		if s.dragScrollEdge == 0 || s.maxLines <= 1 {
			return
		}
		s.applyDragScrollStep()
		// Chain the next step from this PostFrame so Build stays free of scroll scheduling.
		s.scheduleDragScroll(woxwidget.StateContext{})
	})
}

// applyDragScrollStep advances vertical offset while a drag selection is held past the viewport edge.
func (s *textFieldState) applyDragScrollStep() {
	if s.dragScrollEdge == 0 || s.maxLines <= 1 {
		return
	}
	step := textFieldLineHeight
	if s.dragScrollEdge < 0 {
		s.verticalOffset = max(float32(0), s.verticalOffset-step)
	} else {
		s.verticalOffset = max(float32(0), min(s.dragScrollMaxOff, s.verticalOffset+step))
	}
	local := s.dragScrollLocal
	local.Y = max(float32(0), min(s.innerHeight, local.Y))
	s.applyDragSelectionAt(local)
}

func (s *textFieldState) applyDragSelectionAt(local woxui.Point) {
	if s.controller == nil {
		return
	}
	display := s.controller.State()
	if s.dragProtected {
		real := display
		display.Text = woxui.MaskProtectedText(real.Text)
		display.Composition = woxui.MaskProtectedText(real.Composition)
		display.Selection = woxui.MapSelectionToProtectedDisplay(real.Text, real.Selection)
	}
	offset := textFieldOffsetAt(display, s.dragScrollWindow, s.style, s.maxLines, s.verticalOffset, s.innerWidth, s.maxLines > 1, local)
	if s.dragProtected {
		offset = woxui.MapProtectedDisplayOffsetToRune(s.controller.Text(), offset)
	}
	s.controller.SetSelection(s.selectionAnchor, offset)
	if s.dragNotifySelect != nil {
		s.dragNotifySelect()
	}
	if s.dragInvalidate != nil {
		s.dragInvalidate()
	}
}

// updateBindings keeps externally replaceable controller objects attached to the retained field state.
func (s *textFieldState) updateBindings(context woxwidget.StateContext, props TextFieldProps) {
	controller := props.Controller
	if controller == nil {
		if s.internalController == nil {
			s.internalController = woxwidget.NewTextEditingController(props.Value)
		}
		controller = s.internalController
	}
	s.controller = controller
	focusNode := props.FocusNode
	if focusNode == nil {
		if s.internalFocusNode == nil {
			s.internalFocusNode = woxwidget.NewFocusNode()
		}
		focusNode = s.internalFocusNode
	}
	if s.focusNode != focusNode || s.focusAttachment == nil {
		if s.focusAttachment != nil {
			s.focusAttachment.Detach()
		}
		s.focusNode = focusNode
		s.focusAttachment = context.BindFocusNode(focusNode, woxwidget.Key(props.ID))
	}
}

func notifyTextFieldChanged(props TextFieldProps, value string) {
	if props.OnChanged != nil {
		props.OnChanged(value)
	}
}

func notifyTextFieldSelectionChanged(props TextFieldProps, selection woxui.TextSelection) {
	if props.OnSelectionChanged != nil {
		props.OnSelectionChanged(selection)
	}
}

// handleTextFieldControllerKey adds multiline navigation around the shared editor key handling.
func handleTextFieldControllerKey(controller *woxwidget.TextEditingController, maxLines int, lines []textFieldLine, viewportHeight float32, window textFieldMeasurer, style woxui.TextStyle, width float32, softWrap bool, event woxui.KeyEvent) (bool, bool) {
	if controller == nil {
		return false, false
	}
	if maxLines <= 1 {
		return controller.HandleKey(event)
	}
	if !event.Down || event.Composing {
		return false, false
	}
	state := controller.State()
	if len(lines) == 0 {
		lines = textFieldLines(state.Text, window, style, width, softWrap)
	}
	lineIndex := textFieldLineIndex(lines, state.Selection.Focus)
	line := lines[lineIndex]
	extend := event.Modifiers&woxui.KeyModifierShift != 0
	setFocus := func(offset int) {
		if extend {
			controller.SetSelection(state.Selection.Anchor, offset)
		} else {
			controller.SetCaret(offset)
		}
	}
	moveVertical := func(deltaLines int) (bool, bool) {
		targetX, ok := controller.PreferredX()
		if !ok {
			targetX = textFieldCaretX(line, state.Selection.Focus, window, style)
			controller.SetPreferredX(targetX)
		}
		target := lineIndex + deltaLines
		if target < 0 {
			setFocus(textFieldOffsetForX(lines[0], targetX, window, style))
			controller.SetPreferredX(targetX)
			return true, false
		}
		if target >= len(lines) {
			setFocus(textFieldOffsetForX(lines[len(lines)-1], targetX, window, style))
			controller.SetPreferredX(targetX)
			return true, false
		}
		setFocus(textFieldOffsetForX(lines[target], targetX, window, style))
		controller.SetPreferredX(targetX)
		return true, false
	}
	pageLines := max(1, int(viewportHeight/textFieldLineHeight))
	switch event.Key {
	case woxui.KeyEnter:
		return true, controller.InsertText("\n")
	case woxui.KeyArrowUp:
		return moveVertical(-1)
	case woxui.KeyArrowDown:
		return moveVertical(1)
	case woxui.KeyPageUp:
		return moveVertical(-pageLines)
	case woxui.KeyPageDown:
		return moveVertical(pageLines)
	case woxui.KeyHome:
		setFocus(line.start)
		return true, false
	case woxui.KeyEnd:
		setFocus(line.end)
		return true, false
	default:
		return controller.HandleKey(event)
	}
}

// textFieldCaretX measures the visual X of a caret within one visual line.
func textFieldCaretX(line textFieldLine, focus int, window textFieldMeasurer, style woxui.TextStyle) float32 {
	focus = max(line.start, min(line.end, focus))
	prefix := string([]rune(line.text)[:focus-line.start])
	if window == nil {
		return float32(focus - line.start)
	}
	metrics, _ := window.MeasureText(prefix, style)
	return metrics.Size.Width
}

// textFieldOffsetForX finds the grapheme-safe caret offset nearest to targetX on one visual line.
func textFieldOffsetForX(line textFieldLine, targetX float32, window textFieldMeasurer, style woxui.TextStyle) int {
	if line.end <= line.start {
		return line.start
	}
	spans := woxui.GraphemeSpans(line.text)
	if len(spans) == 0 {
		return line.start
	}
	if window == nil {
		col := max(0, min(line.end-line.start, int(targetX+0.5)))
		return line.start + col
	}
	bestOffset := line.start
	bestDist := float32(1e9)
	previousWidth := float32(0)
	consider := func(offset int, width float32) {
		dist := width - targetX
		if dist < 0 {
			dist = -dist
		}
		if dist < bestDist {
			bestDist = dist
			bestOffset = line.start + offset
		}
	}
	consider(0, 0)
	for index := 1; index <= len(spans); index++ {
		metrics, _ := window.MeasureText(joinGraphemeSpans(spans[:index]), style)
		consider(spans[index-1].End, metrics.Size.Width)
		previousWidth = metrics.Size.Width
	}
	_ = previousWidth
	return bestOffset
}

func buildWoxTextField(props TextFieldProps, realState woxui.TextEditingState, copySelection, cutSelection, pasteClipboard func() bool, runContextAction func(textFieldContextAction)) woxwidget.Widget {
	height := props.Height
	if height <= 0 {
		height = 40
	}
	radius := props.Radius
	if radius <= 0 {
		radius = 8
	}
	padding := props.Padding
	if padding == (woxwidget.Insets{}) {
		padding = woxwidget.Insets{Left: 12, Top: 9, Right: 12, Bottom: 7}
	}
	background := props.Background
	if background.A == 0 && !props.Transparent {
		background = props.Theme.QueryBackground
	}
	if props.hovered {
		background = controlHoverColor(background, props.Theme.ResultTitle)
	}
	style := props.Style
	if style.Size <= 0 {
		style = woxui.TextStyle{Size: SettingsControlFontSize}
	}
	textColor := props.TextColor
	if textColor.A == 0 {
		textColor = props.Theme.ActionText
	}
	maxLines := max(1, props.MaxLines)
	innerWidth := max(float32(0), props.Width-padding.Left-padding.Right)
	innerHeight := max(float32(0), height-padding.Top-padding.Bottom)
	state := props.editingState
	softWrap := maxLines > 1
	pointerCursor := woxui.PointerCursorText
	if props.Disabled {
		pointerCursor = woxui.PointerCursorDefault
	}
	offsetAt := func(position woxui.Point) int {
		point := woxui.Point{X: max(float32(0), position.X-padding.Left), Y: max(float32(0), position.Y-padding.Top)}
		return textFieldOffsetAt(state, props.Window, style, maxLines, props.verticalOffset, innerWidth, softWrap, point)
	}
	content := woxwidget.Gesture{ID: props.ID, Cursor: pointerCursor, OnHoverAt: props.onHoverAt, OnScrollHandled: props.onScroll, OnTapAt: func(position woxui.Point) {
		if props.Disabled || props.Window == nil || props.onCaret == nil {
			return
		}
		props.onCaret(offsetAt(position))
	}, OnDoubleTapAt: func(position woxui.Point) {
		if props.Disabled || props.Window == nil || props.onWordSelection == nil {
			return
		}
		props.onWordSelection(offsetAt(position))
	}, OnTripleTapAt: func(position woxui.Point) {
		if props.Disabled || props.Window == nil || props.onLineSelection == nil {
			return
		}
		props.onLineSelection(offsetAt(position))
	}, OnSelectionStart: func(position woxui.Point, modifiers woxui.KeyModifiers) {
		if props.Disabled || props.Window == nil || props.onSelectionStart == nil {
			return
		}
		props.onSelectionStart(offsetAt(position), modifiers)
	}, OnSelectionExtend: func(position woxui.Point) {
		if props.Disabled || props.Window == nil || props.onSelectionExtendAt == nil {
			return
		}
		props.onSelectionExtendAt(position)
	}, OnSelectionEnd: func() {
		if props.onSelectionEnd != nil {
			props.onSelectionEnd()
		}
	}, OnSecondaryTapDown: func(windowPos woxui.Point) {
		if props.onSecondaryTap == nil {
			return
		}
		props.onSecondaryTap(windowPos)
	}, Child: woxwidget.Container{
		Width: props.Width, Height: height, Radius: radius, Color: background, BorderColor: props.BorderColor, BorderWidth: props.BorderWidth, Padding: padding,
		Child: woxwidget.Clip{Width: innerWidth, Height: innerHeight, Child: woxwidget.CaretPainter{Width: innerWidth, Height: innerHeight, Active: props.caretActive, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect, focused, caretVisible bool) {
			if state.Text == "" && state.Composition == "" && props.Hint != "" {
				displayList.DrawText(props.Hint, textFieldAlignedTextBounds(bounds, props.Hint, style, props.TextAlignmentY, props.Window), style, props.Theme.ResultSubtitle)
			}
			if props.Window != nil {
				drawTextField(displayList, bounds, state, style, textColor, props.Theme, focused, caretVisible, maxLines, props.verticalOffset, props.TextAlignmentY, softWrap, props.Window)
			}
		}},
		}}}
	key := woxwidget.Key(props.ID)
	focusRingColor := props.FocusRingColor
	if focusRingColor.A == 0 && props.BorderWidth > 0 {
		focusRingColor = props.Theme.Cursor
	}
	selection := realState.Selection
	value := realState.Text
	if props.Protected {
		value = ""
	}
	field := woxwidget.EditableText{
		Key: key, AutomationID: props.ID, Label: props.Label, Value: value, ReadOnly: props.ReadOnly, Protected: props.Protected,
		Autofocus: props.Autofocus, Disabled: props.Disabled, OnKey: props.OnKey, OnTextInput: props.onTextInput,
		FocusRingColor: focusRingColor, FocusRingRadius: radius, FocusRingOutsets: props.FocusRingOutsets,
		OnFocusChange: props.OnFocusChange, OnSetValue: props.OnSetValue,
		HasTextSelection: true, SelectionStart: selection.Start(), SelectionEnd: selection.End(),
		OnSelectAll: func() error {
			if props.Disabled {
				return nil
			}
			runContextAction(textFieldContextSelectAll)
			return nil
		},
		OnCopy: func() error {
			_ = copySelection()
			return nil
		},
		OnCut: func() error {
			_ = cutSelection()
			return nil
		},
		OnPaste: func() error {
			_ = pasteClipboard()
			return nil
		},
		TextInput: func(bounds woxui.Rect) woxui.TextInputState {
			if !props.Focused || props.Window == nil {
				return woxui.TextInputState{}
			}
			innerBounds := woxui.Rect{X: bounds.X + padding.Left, Y: bounds.Y + padding.Top, Width: innerWidth, Height: innerHeight}
			return woxui.TextInputState{Enabled: true, CursorRect: textFieldCursorRect(state, style, maxLines, props.verticalOffset, innerBounds, softWrap, props.Window)}
		},
		Child: content,
	}
	return field
}

// openTextFieldContextMenu places Cut/Copy/Paste/Select All in a Host overlay so it is not clipped by the field.
func openTextFieldContextMenu(context woxwidget.StateContext, props TextFieldProps, state *textFieldState, ownerKey woxwidget.Key, windowPos woxui.Point, runContextAction func(textFieldContextAction)) {
	frame := context.HostFrameSize()
	menu := buildTextFieldContextMenu(props, state, runContextAction)
	var token uint64
	token = context.SetHostOverlay(ownerKey, PlaceTextEditContextMenu(frame, windowPos, props.ID, menu, func() {
		context.ClearHostOverlay(ownerKey, token)
		if state != nil {
			state.overlayToken = 0
		}
	}))
	if state != nil {
		state.overlayToken = token
		state.menuAnchor = windowPos
		state.menuEnablement = computeTextFieldMenuEnablement(props, state.controller.State())
		state.menuActionRunner = runContextAction
	}
}

// refreshContextMenuIfStale rebuilds an open menu when selection or editability enablement changed.
func (s *textFieldState) refreshContextMenuIfStale(context woxwidget.StateContext, props TextFieldProps, ownerKey woxwidget.Key, runContextAction func(textFieldContextAction)) {
	if s == nil || s.overlayToken == 0 || runContextAction == nil {
		return
	}
	if !context.HasHostOverlay() || context.HostOverlayOwner() != ownerKey {
		s.overlayToken = 0
		return
	}
	next := computeTextFieldMenuEnablement(props, s.controller.State())
	if next == s.menuEnablement {
		return
	}
	openTextFieldContextMenu(context, props, s, ownerKey, s.menuAnchor, runContextAction)
}

func buildTextFieldContextMenu(props TextFieldProps, state *textFieldState, runContextAction func(textFieldContextAction)) woxwidget.Widget {
	editing := props.editingState
	if state != nil && state.controller != nil {
		editing = state.controller.State()
	}
	en := computeTextFieldMenuEnablement(props, editing)
	return BuildTextEditContextMenu(TextEditContextMenuProps{
		ID: props.ID + ".menu", Theme: props.Theme,
		CanCut: en.canCut, CanCopy: en.canCopy, CanPaste: en.canPaste, CanSelectAll: en.canSelectAll,
		OnAction: func(action TextEditContextAction) {
			runContextAction(textFieldContextAction(action))
		},
	})
}

// textFieldLines splits text into visual lines. Soft wrap uses measured grapheme widths when enabled.
func textFieldLines(value string, window textFieldMeasurer, style woxui.TextStyle, width float32, softWrap bool) []textFieldLine {
	runes := []rune(value)
	if !softWrap || window == nil || width <= 0 {
		lines := make([]textFieldLine, 0, strings.Count(value, "\n")+1)
		start := 0
		for index, current := range runes {
			if current == '\n' {
				lines = append(lines, textFieldLine{start: start, end: index, text: string(runes[start:index])})
				start = index + 1
			}
		}
		lines = append(lines, textFieldLine{start: start, end: len(runes), text: string(runes[start:])})
		return lines
	}
	lines := make([]textFieldLine, 0, strings.Count(value, "\n")+1)
	spans := woxui.GraphemeSpans(value)
	paragraphStart := 0
	for index := 0; index <= len(runes); index++ {
		atEnd := index == len(runes)
		atBreak := !atEnd && runes[index] == '\n'
		if !atEnd && !atBreak {
			continue
		}
		paragraphSpans := graphemeSpansInRange(spans, paragraphStart, index)
		if len(paragraphSpans) == 0 {
			lines = append(lines, textFieldLine{start: paragraphStart, end: paragraphStart, text: ""})
		} else {
			offset := paragraphStart
			remaining := paragraphSpans
			for len(remaining) > 0 {
				fit := fittingGraphemePrefix(window, remaining, style, width)
				if fit >= len(remaining) {
					end := remaining[len(remaining)-1].End
					lines = append(lines, textFieldLine{start: offset, end: end, text: string(runes[offset:end])})
					break
				}
				breakAt := fit
				for candidate := fit - 1; candidate > 0; candidate-- {
					cluster := []rune(remaining[candidate].Text)
					if len(cluster) > 0 && isTextFieldWrapSpace(cluster[0]) {
						breakAt = candidate + 1
						break
					}
				}
				if breakAt <= 0 {
					breakAt = max(1, fit)
				}
				end := remaining[breakAt-1].End
				lines = append(lines, textFieldLine{start: offset, end: end, text: string(runes[offset:end])})
				offset = end
				remaining = remaining[breakAt:]
			}
		}
		if atBreak {
			paragraphStart = index + 1
		}
	}
	if len(lines) == 0 {
		lines = append(lines, textFieldLine{start: 0, end: 0, text: ""})
	}
	return lines
}

type textFieldMeasurer interface {
	MeasureText(text string, style woxui.TextStyle) (woxui.TextMetrics, error)
}

func graphemeSpansInRange(spans []woxui.GraphemeSpan, start, end int) []woxui.GraphemeSpan {
	filtered := make([]woxui.GraphemeSpan, 0, len(spans))
	for _, span := range spans {
		if span.Start >= start && span.End <= end {
			filtered = append(filtered, span)
		}
	}
	return filtered
}

func fittingGraphemePrefix(window textFieldMeasurer, spans []woxui.GraphemeSpan, style woxui.TextStyle, width float32) int {
	if len(spans) == 0 {
		return 0
	}
	if width <= 0 {
		return 1
	}
	low, high := 1, len(spans)
	for low < high {
		mid := low + (high-low+1)/2
		text := joinGraphemeSpans(spans[:mid])
		metrics, _ := window.MeasureText(text, style)
		if metrics.Size.Width <= width {
			low = mid
		} else {
			high = mid - 1
		}
	}
	return max(1, low)
}

func joinGraphemeSpans(spans []woxui.GraphemeSpan) string {
	if len(spans) == 0 {
		return ""
	}
	total := 0
	for _, span := range spans {
		total += len(span.Text)
	}
	buf := make([]byte, 0, total)
	for _, span := range spans {
		buf = append(buf, span.Text...)
	}
	return string(buf)
}

func isTextFieldWrapSpace(current rune) bool {
	return current == ' ' || current == '\t'
}

func textFieldLineIndex(lines []textFieldLine, offset int) int {
	for index, line := range lines {
		if offset <= line.end || index == len(lines)-1 {
			return index
		}
	}
	return 0
}

func textFieldOffsetAt(state woxui.TextEditingState, window *woxui.Window, style woxui.TextStyle, maxLines int, verticalOffset, width float32, softWrap bool, point woxui.Point) int {
	lines := textFieldLines(state.Text, window, style, width, softWrap)
	lineIndex := min(len(lines)-1, max(0, int((point.Y+verticalOffset)/textFieldLineHeight)))
	line := lines[lineIndex]
	if maxLines == 1 {
		point.X += textFieldHorizontalOffset([]rune(state.Text), state.Selection.Focus, style, width, window)
	}
	spans := woxui.GraphemeSpans(line.text)
	if len(spans) == 0 {
		return line.start
	}
	offset := line.end - line.start
	previousWidth := float32(0)
	for candidate := 1; candidate <= len(spans); candidate++ {
		metrics, _ := window.MeasureText(joinGraphemeSpans(spans[:candidate]), style)
		if point.X < (previousWidth+metrics.Size.Width)*0.5 {
			offset = spans[candidate-1].Start
			break
		}
		previousWidth = metrics.Size.Width
		offset = spans[candidate-1].End
	}
	return line.start + offset
}

func textFieldHorizontalOffset(runes []rune, focus int, style woxui.TextStyle, width float32, window *woxui.Window) float32 {
	focus = max(0, min(len(runes), focus))
	metrics, _ := window.MeasureText(string(runes[:focus]), style)
	return max(float32(0), metrics.Size.Width-max(float32(0), width-4))
}

// textFieldAlignedTextBounds aligns measured glyphs while preserving the line box used by editing geometry.
func textFieldAlignedTextBounds(bounds woxui.Rect, value string, style woxui.TextStyle, alignment float32, window *woxui.Window) woxui.Rect {
	if alignment <= 0 || value == "" || window == nil {
		return bounds
	}
	metrics, err := window.MeasureText(value, style)
	if err != nil || metrics.Size.Height <= 0 {
		return bounds
	}
	height := min(bounds.Height, metrics.Size.Height)
	bounds.Y += max(float32(0), bounds.Height-height) * min(alignment, float32(1))
	bounds.Height = height
	return bounds
}

func drawTextField(displayList *woxui.DisplayList, bounds woxui.Rect, state woxui.TextEditingState, style woxui.TextStyle, textColor woxui.Color, theme Theme, focused, caretVisible bool, maxLines int, verticalOffset, textAlignmentY float32, softWrap bool, window *woxui.Window) {
	displayRunes, start, end, focus, compositionStart, compositionEnd := textFieldDisplayState(state)
	lines := textFieldLines(string(displayRunes), window, style, bounds.Width, softWrap)
	caretLine := textFieldLineIndex(lines, focus)
	visibleLines := max(1, min(maxLines, int(bounds.Height/textFieldLineHeight)))
	firstLine := max(0, int(verticalOffset/textFieldLineHeight))
	lineOffset := verticalOffset - float32(firstLine)*textFieldLineHeight
	lastLine := min(len(lines), firstLine+visibleLines+1)
	horizontalOffset := float32(0)
	if visibleLines == 1 {
		horizontalOffset = textFieldHorizontalOffset(displayRunes, focus, style, bounds.Width, window)
	}
	for lineIndex := firstLine; lineIndex < lastLine; lineIndex++ {
		line := lines[lineIndex]
		y := bounds.Y + float32(lineIndex-firstLine)*textFieldLineHeight - lineOffset
		textBounds := textFieldAlignedTextBounds(woxui.Rect{X: bounds.X - horizontalOffset, Y: y, Width: bounds.Width + horizontalOffset, Height: textFieldLineHeight}, line.text, style, textAlignmentY, window)
		selectionStart := max(start, line.start)
		selectionEnd := min(end, line.end)
		if focused && selectionStart < selectionEnd {
			prefixMetrics, _ := window.MeasureText(string(displayRunes[line.start:selectionStart]), style)
			selectedMetrics, _ := window.MeasureText(string(displayRunes[selectionStart:selectionEnd]), style)
			displayList.FillRoundedRect(woxui.Rect{X: bounds.X - horizontalOffset + prefixMetrics.Size.Width, Y: y, Width: selectedMetrics.Size.Width, Height: textFieldLineHeight}, 3, theme.SelectionBackground)
		}
		displayList.DrawText(line.text, textBounds, style, textColor)
		if focused && selectionStart < selectionEnd {
			prefixMetrics, _ := window.MeasureText(string(displayRunes[line.start:selectionStart]), style)
			selectedText := string(displayRunes[selectionStart:selectionEnd])
			selectedMetrics, _ := window.MeasureText(selectedText, style)
			selectedBounds := textBounds
			selectedBounds.X = bounds.X - horizontalOffset + prefixMetrics.Size.Width
			selectedBounds.Width = selectedMetrics.Size.Width
			displayList.DrawText(selectedText, selectedBounds, style, theme.SelectionText)
		}
	}
	if !focused {
		return
	}
	line := lines[caretLine]
	caretMetrics, _ := window.MeasureText(string(displayRunes[line.start:focus]), style)
	cursorX := bounds.X - horizontalOffset + caretMetrics.Size.Width
	cursorY := bounds.Y + float32(caretLine-firstLine)*textFieldLineHeight - lineOffset
	if compositionStart >= line.start && compositionEnd <= line.end {
		prefixMetrics, _ := window.MeasureText(string(displayRunes[line.start:compositionStart]), style)
		compositionMetrics, _ := window.MeasureText(string(displayRunes[compositionStart:compositionEnd]), style)
		displayList.FillRect(woxui.Rect{X: bounds.X - horizontalOffset + prefixMetrics.Size.Width, Y: cursorY + 19, Width: compositionMetrics.Size.Width, Height: 1}, theme.Cursor)
	}
	if caretVisible {
		displayList.FillRect(woxui.Rect{X: cursorX, Y: cursorY, Width: textFieldCursorWidth, Height: textFieldLineHeight}, theme.Cursor)
	}
}

func textFieldCursorRect(state woxui.TextEditingState, style woxui.TextStyle, maxLines int, verticalOffset float32, bounds woxui.Rect, softWrap bool, window *woxui.Window) woxui.Rect {
	displayRunes, _, _, focus, _, _ := textFieldDisplayState(state)
	lines := textFieldLines(string(displayRunes), window, style, bounds.Width, softWrap)
	caretLine := textFieldLineIndex(lines, focus)
	visibleLines := max(1, min(maxLines, int(bounds.Height/textFieldLineHeight)))
	firstLine := max(0, int(verticalOffset/textFieldLineHeight))
	lineOffset := verticalOffset - float32(firstLine)*textFieldLineHeight
	horizontalOffset := float32(0)
	if visibleLines == 1 {
		horizontalOffset = textFieldHorizontalOffset(displayRunes, focus, style, bounds.Width, window)
	}
	line := lines[caretLine]
	metrics, _ := window.MeasureText(string(displayRunes[line.start:focus]), style)
	return woxui.Rect{
		X:     bounds.X - horizontalOffset + metrics.Size.Width,
		Y:     bounds.Y + float32(caretLine-firstLine)*textFieldLineHeight - lineOffset,
		Width: textFieldCursorWidth, Height: 22,
	}
}

func textFieldScrolledOffset(offset, maximumOffset, deltaY float32) (float32, bool) {
	next := max(float32(0), min(maximumOffset, offset-deltaY))
	return next, next != offset
}

func textFieldDisplayState(state woxui.TextEditingState) ([]rune, int, int, int, int, int) {
	runes := []rune(state.Text)
	start := max(0, min(len(runes), state.Selection.Start()))
	end := max(start, min(len(runes), state.Selection.End()))
	focus := max(0, min(len(runes), state.Selection.Focus))
	displayValue := state.Text
	compositionStart := -1
	compositionEnd := -1
	if state.Composition != "" {
		displayValue = string(runes[:start]) + state.Composition + string(runes[end:])
		compositionStart = start
		compositionEnd = start + len([]rune(state.Composition))
		start = compositionEnd
		end = compositionEnd
		focus = compositionEnd
	}
	return []rune(displayValue), start, end, focus, compositionStart, compositionEnd
}
