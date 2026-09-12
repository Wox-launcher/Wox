package component

import (
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

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
	start  int
	end    int
	text   string
	indent float32
}

type textFieldContextAction uint8

const (
	textFieldContextCut textFieldContextAction = iota
	textFieldContextCopy
	textFieldContextPaste
	textFieldContextSelectAll
)

// TextFieldRichRun applies inline presentation without changing the editor's plain-text value.
type TextFieldRichRun struct {
	Start      int
	End        int
	Style      woxui.TextStyle
	Color      woxui.Color
	Underline  bool
	Strike     bool
	Background woxui.Color
	// Advance, when greater than zero, replaces the measured width of this range.
	Advance float32
	// HideText skips drawing the backing characters so a custom Paint can replace them.
	HideText bool
	// Paint draws this range. Bounds width is the remaining line when Advance is 0.
	Paint func(displayList *woxui.DisplayList, bounds woxui.Rect)
	// Dismissible shows a trailing close affordance while this painted run is hovered.
	Dismissible bool
	// ChipEditable shows a trailing edit affordance before close while hovered.
	ChipEditable bool
	// ChipLabel is the painted token text used when a dismissible chip is hovered.
	ChipLabel string
	// LineGutter paints a leading decoration on every soft-wrapped line that intersects this run.
	LineGutter      bool
	LineGutterWidth float32
	PaintLineGutter func(displayList *woxui.DisplayList, bounds woxui.Rect)
	// HangingIndent keeps later soft-wrapped lines aligned with the first glyph after this run.
	HangingIndent bool
}

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
	RichRuns         []TextFieldRichRun
	// AtomicTokens are complete placeholders that move and delete as one caret unit.
	AtomicTokens []TextFieldTokenRange
	LineHeight   float32
	TextColor    woxui.Color
	// TextAlignmentY optically positions measured glyph bounds within each line without moving the caret.
	TextAlignmentY float32
	Value          string
	Focused        bool
	Autofocus      bool
	Controller     *woxwidget.TextEditingController
	FocusNode      *woxwidget.FocusNode
	Disabled       bool
	DisableHover   bool
	// OnHoverAt reports pointer enter/leave even when DisableHover skips the hover fill.
	OnHoverAt func(bool, woxui.Rect)
	ReadOnly  bool
	Protected bool
	MaxLines  int
	// ExposeVisualLines publishes soft-wrapped lines and hanging indent on the text-field node.
	ExposeVisualLines bool
	Window            *woxui.Window
	Theme             Theme
	OnKey             func(woxui.KeyEvent) bool
	OnUndo            func() bool
	OnRedo            func() bool
	// OnSelectAll replaces the default field select-all when it returns true.
	OnSelectAll func() bool
	// OnCopy replaces the default copy of the field selection when it returns true.
	OnCopy func() bool
	// OnCut replaces the default cut of the field selection when it returns true.
	OnCut func() bool
	// PaintSelection draws the current selection even when the field is not focused.
	PaintSelection bool
	// OnPaste receives raw clipboard text and, when it returns true, replaces the default insert.
	// Empty text is still delivered so callers can paste images or files from the same shortcut.
	OnPaste            func(string) bool
	TransformPaste     func(string) string
	OnFocusChange      func(bool)
	OnChanged          func(string)
	OnSelectionChanged func(woxui.TextSelection)
	OnTapOffset        func(int) bool
	// TrailingHandle shows a drag affordance in the left gutter of the Start-End block.
	TrailingHandle *TextFieldTrailingHandle
	// OnHoverOffset reports the rune under the pointer so a block handle can follow hover.
	OnHoverOffset func(offset int, inside bool)
	// OnDismissRun removes a dismissible rich run. Returning true consumes the tap.
	OnDismissRun func(start, end int) bool
	// OnEditRun expands a dismissible rich run for in-place editing. Returning true consumes the tap.
	OnEditRun func(start, end int) bool
	// OnTapBelowText optionally handles a click beneath the final rendered line of a multiline field.
	OnTapBelowText       func() bool
	CursorAtOffset       func(int) woxui.PointerCursor
	onDismissHover       func(start, end int, ok bool)
	dismissHoverStart    int
	dismissHoverProgress float32
	OnSetValue           func(string) error
	editingState         woxui.TextEditingState
	onCaret              func(int)
	onWordSelection      func(int)
	onLineSelection      func(int)
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
	// handleDragging and handleDragLocalY keep the reorder icon under the pointer
	// without moving the gesture target, so pan coordinates stay stable.
	handleDragging       bool
	handleDragLocalY     float32
	onTrailingHandleDrag func(localY float32, ended bool)
}

// TextFieldTrailingHandle is a left-gutter drag control owned by one rune range.
type TextFieldTrailingHandle struct {
	Start        int
	End          int
	Size         float32
	Label        string
	Preview      string
	Active       bool
	ShowDropLine bool
	DropLineY    float32
	OnPanStart   func()
	OnPanUpdate  func(contentY float32)
	OnPanEnd     func()
}

func textFieldVisualLines(value string, window *woxui.Window, style woxui.TextStyle, width float32, richRuns []TextFieldRichRun) []textFieldLine {
	if style.Size <= 0 {
		style = woxui.TextStyle{Size: SettingsControlFontSize}
	}
	var measurer textFieldMeasurer
	if window != nil {
		measurer = window
	}
	return textFieldRichLines(value, measurer, style, width, true, richRuns)
}

// TextFieldVisualLineCount returns wrapped lines using the same rules as WoxTextField.
func TextFieldVisualLineCount(value string, window *woxui.Window, style woxui.TextStyle, width float32, richRuns []TextFieldRichRun) int {
	return max(1, len(textFieldVisualLines(value, window, style, width, richRuns)))
}

// TextFieldVisualLineIndex returns the wrapped line that contains caret.
func TextFieldVisualLineIndex(value string, caret int, window *woxui.Window, style woxui.TextStyle, width float32, richRuns []TextFieldRichRun) int {
	return textFieldLineIndex(textFieldVisualLines(value, window, style, width, richRuns), caret)
}

// WoxTextField builds a retained text field with shared IME, selection, and accessibility behavior.
func WoxTextField(props TextFieldProps) woxwidget.Widget {
	if props.MaxLines <= 1 && props.TextAlignmentY == 0 {
		props.TextAlignmentY = 0.5
	}
	if props.LineHeight <= 0 {
		props.LineHeight = textFieldLineHeight
	}
	return woxwidget.Stateful{
		Key: woxwidget.Key(props.ID), Type: (*textFieldState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &textFieldState{} },
	}
}

type textFieldState struct {
	hovered             bool
	hasHoveredDismiss   bool
	hoveredDismissStart int
	controller          *woxwidget.TextEditingController
	internalController  *woxwidget.TextEditingController
	focusNode           *woxwidget.FocusNode
	internalFocusNode   *woxwidget.FocusNode
	focusAttachment     *woxwidget.FocusAttachment
	// selectionAnchor holds the rune offset captured at drag-selection start so extend updates only the focus.
	selectionAnchor  int
	verticalOffset   float32
	lastFocus        int
	hasLastFocus     bool
	innerHeight      float32
	innerWidth       float32
	style            woxui.TextStyle
	richRuns         []TextFieldRichRun
	atomicTokens     []TextFieldTokenRange
	lineHeight       float32
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
	handleDragging      bool
	handleDragLocalY    float32
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
	if props.PaintSelection && displayState.Selection.Collapsed() && displayState.Text != "" {
		displayState.Selection = woxui.TextSelection{Anchor: 0, Focus: utf8.RuneCountInString(displayState.Text)}
	}
	props.editingState = displayState
	props.caretActive = props.Focused
	props.Focused = s.focusNode.HasFocus()
	style := props.Style
	if style.Size <= 0 {
		style = woxui.TextStyle{Size: SettingsControlFontSize}
	}
	s.style = style
	s.richRuns = props.RichRuns
	s.atomicTokens = props.AtomicTokens
	s.lineHeight = props.LineHeight
	if s.lineHeight <= 0 {
		s.lineHeight = textFieldLineHeight
	}
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
	lines := textFieldRichLines(displayState.Text, props.Window, style, innerWidth, softWrap, props.RichRuns)
	caretLine := textFieldLineIndex(lines, displayState.Selection.Focus)
	maximumVerticalOffset := max(float32(0), float32(len(lines))*s.lineHeight-innerHeight)
	s.verticalOffset = min(s.verticalOffset, maximumVerticalOffset)
	if !s.hasLastFocus || s.lastFocus != displayState.Selection.Focus {
		caretTop := float32(caretLine) * s.lineHeight
		if caretTop < s.verticalOffset {
			s.verticalOffset = caretTop
		} else if caretTop+s.lineHeight > s.verticalOffset+innerHeight {
			s.verticalOffset = caretTop + s.lineHeight - innerHeight
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
		if err != nil {
			text = ""
		}
		if original.OnPaste != nil && original.OnPaste(text) {
			notifySelection()
			invalidate()
			return true
		}
		if text == "" {
			return true
		}
		if original.TransformPaste != nil {
			text = original.TransformPaste(text)
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
			if original := widget.(TextFieldProps); original.OnCut != nil && original.OnCut() {
				break
			}
			_ = cutSelection()
		case textFieldContextCopy:
			if !en.canCopy {
				return
			}
			if original := widget.(TextFieldProps); original.OnCopy != nil && original.OnCopy() {
				break
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
			if original := widget.(TextFieldProps); original.OnSelectAll != nil && original.OnSelectAll() {
				break
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
		offset = mapHitOffset(offset)
		if snapped, ok := snapTextFieldAtomicCaret(s.atomicTokens, offset); ok {
			offset = snapped
		}
		s.controller.SetCaret(offset)
		notifySelection()
		invalidate()
	}
	props.onWordSelection = func(offset int) {
		s.focusNode.RequestFocus()
		closeContextMenu()
		offset = mapHitOffset(offset)
		if token, ok := textFieldAtomicTokenContaining(s.atomicTokens, offset); ok {
			s.controller.SetSelection(token.Start, token.End)
			notifySelection()
			invalidate()
			return
		}
		if token, ok := textFieldAtomicTokenAfter(s.atomicTokens, offset); ok {
			s.controller.SetSelection(token.Start, token.End)
			notifySelection()
			invalidate()
			return
		}
		s.controller.SelectWordAt(offset)
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
		if snapped, ok := snapTextFieldAtomicCaret(s.atomicTokens, offset); ok {
			offset = snapped
		}
		if modifiers&woxui.KeyModifierShift != 0 {
			s.selectionAnchor = s.controller.State().Selection.Anchor
			s.controller.SetSelection(s.selectionAnchor, offset)
			applyExpandedTextFieldAtomicSelection(s.controller, s.atomicTokens, s.selectionAnchor, offset)
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
				if original.OnSelectAll != nil && original.OnSelectAll() {
					return true
				}
				s.controller.SelectAll()
				notifySelection()
				invalidate()
				return true
			case woxui.Key("c"):
				if original.OnCopy != nil && original.OnCopy() {
					return true
				}
				return copySelection()
			case woxui.Key("x"):
				if event.Modifiers&woxui.KeyModifierShift != 0 {
					break
				}
				if original.OnCut != nil && original.OnCut() {
					return true
				}
				if !allowsMutation || original.Protected {
					return true
				}
				return cutSelection()
			case woxui.Key("v"):
				if !allowsMutation {
					return true
				}
				return pasteClipboard()
			case woxui.Key("z"):
				if event.Modifiers&woxui.KeyModifierShift != 0 && original.OnRedo != nil && original.OnRedo() {
					notifySelection()
					invalidate()
					return true
				}
				if original.OnUndo != nil && original.OnUndo() {
					notifySelection()
					invalidate()
					return true
				}
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
			case woxui.Key("y"):
				if original.OnRedo != nil && original.OnRedo() {
					notifySelection()
					invalidate()
					return true
				}
				fallthrough
			case woxui.KeyBackspace:
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
		if handled, changed := handleTextFieldAtomicTokenKey(s.controller, s.atomicTokens, event); handled {
			if changed {
				notifyChanged()
			}
			notifySelection()
			invalidate()
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
	reportHover := widget.(TextFieldProps).OnHoverAt
	if hoverEnabled || reportHover != nil || props.OnDismissRun != nil {
		props.onHoverAt = func(inside bool, bounds woxui.Rect) {
			if hoverEnabled && inside != s.hovered {
				context.SetState(func() { s.hovered = inside })
			}
			if !inside && s.hasHoveredDismiss {
				context.SetState(func() { s.hasHoveredDismiss = false })
			}
			if reportHover != nil {
				reportHover(inside, bounds)
			}
		}
	}
	if props.OnDismissRun != nil {
		props.dismissHoverStart = s.hoveredDismissStart
		props.onDismissHover = func(start, _ int, ok bool) {
			if !ok {
				if s.hasHoveredDismiss {
					context.SetState(func() { s.hasHoveredDismiss = false })
				}
				return
			}
			if s.hasHoveredDismiss && start == s.hoveredDismissStart {
				return
			}
			context.SetState(func() {
				s.hasHoveredDismiss = true
				s.hoveredDismissStart = start
			})
		}
	}
	props.handleDragging = s.handleDragging
	props.handleDragLocalY = s.handleDragLocalY
	props.onTrailingHandleDrag = func(localY float32, ended bool) {
		context.SetState(func() {
			s.handleDragging = !ended
			if ended {
				s.handleDragLocalY = 0
			} else {
				s.handleDragLocalY = localY
			}
		})
	}
	if props.OnDismissRun == nil {
		return buildWoxTextField(props, realState, copySelection, cutSelection, pasteClipboard, runContextAction)
	}
	target := float32(0)
	if s.hasHoveredDismiss {
		target = 1
	}
	baseRuns := widget.(TextFieldProps).RichRuns
	return woxwidget.AnimatedFloat{
		Key: woxwidget.Key(props.ID + "-token-dismiss"), Target: target,
		Duration: tokenChipDismissMs * time.Millisecond, Curve: woxwidget.AnimationEaseInOutCubic,
		Builder: func(progress float32) woxwidget.Widget {
			animated := props
			animated.RichRuns = withDismissibleChipHover(baseRuns, s.hoveredDismissStart, animated.Theme, progress)
			animated.dismissHoverProgress = progress
			return buildWoxTextField(animated, realState, copySelection, cutSelection, pasteClipboard, runContextAction)
		},
	}
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
	step := s.lineHeight
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
	offset := textFieldOffsetAt(display, s.dragScrollWindow, s.style, s.richRuns, s.maxLines, s.lineHeight, s.verticalOffset, s.innerWidth, s.maxLines > 1, true, local)
	if s.dragProtected {
		offset = woxui.MapProtectedDisplayOffsetToRune(s.controller.Text(), offset)
	}
	s.controller.SetSelection(s.selectionAnchor, offset)
	applyExpandedTextFieldAtomicSelection(s.controller, s.atomicTokens, s.selectionAnchor, offset)
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
		return line.indent + float32(focus-line.start)
	}
	metrics, _ := window.MeasureText(prefix, style)
	return line.indent + metrics.Size.Width
}

// textFieldOffsetForX finds the grapheme-safe caret offset nearest to targetX on one visual line.
func textFieldOffsetForX(line textFieldLine, targetX float32, window textFieldMeasurer, style woxui.TextStyle) int {
	if line.end <= line.start {
		return line.start
	}
	targetX -= line.indent
	if targetX <= 0 {
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
	contentPoint := func(position woxui.Point) woxui.Point {
		return woxui.Point{X: position.X - padding.Left, Y: position.Y - padding.Top}
	}
	offsetAt := func(position woxui.Point) int {
		point := contentPoint(position)
		point.X = max(float32(0), point.X)
		point.Y = max(float32(0), point.Y)
		return textFieldOffsetAt(state, props.Window, style, props.RichRuns, maxLines, props.LineHeight, props.verticalOffset, innerWidth, softWrap, props.Focused, point)
	}
	glyphHitAt := func(position woxui.Point) (int, bool) {
		return textFieldGlyphHitAt(state, props.Window, style, props.RichRuns, maxLines, props.LineHeight, props.verticalOffset, innerWidth, softWrap, props.Focused, contentPoint(position))
	}
	dismissHit := func(position woxui.Point) (int, int, tokenChipAction, bool) {
		if (props.OnDismissRun == nil && props.OnEditRun == nil) || props.Disabled || props.ReadOnly {
			return 0, 0, tokenChipActionNone, false
		}
		return textFieldDismissibleHit(state, props.Window, style, props.RichRuns, maxLines, props.LineHeight, props.verticalOffset, innerWidth, softWrap, props.Focused, contentPoint(position), props.dismissHoverProgress, props.dismissHoverStart)
	}
	onHoverAt := props.onHoverAt
	if props.OnHoverOffset != nil {
		reportHover := props.onHoverAt
		onHoverAt = func(inside bool, bounds woxui.Rect) {
			if !inside {
				props.OnHoverOffset(0, false)
			}
			if reportHover != nil {
				reportHover(inside, bounds)
			}
		}
	}
	content := woxwidget.Gesture{ID: props.ID, CoverHover: props.OnHoverOffset != nil, Cursor: pointerCursor, CursorAt: func(position woxui.Point) woxui.PointerCursor {
		if props.OnHoverOffset != nil {
			props.OnHoverOffset(offsetAt(position), true)
		}
		start, end, action, ok := dismissHit(position)
		if props.onDismissHover != nil {
			props.onDismissHover(start, end, ok)
		}
		if ok && action.isButton() {
			return woxui.PointerCursorHand
		}
		if offset, hit := glyphHitAt(position); hit && props.CursorAtOffset != nil {
			return props.CursorAtOffset(offset)
		}
		return pointerCursor
	}, OnHoverAt: onHoverAt, OnScrollHandled: props.onScroll, OnTapAt: func(position woxui.Point) {
		if props.Disabled || props.Window == nil || props.onCaret == nil {
			return
		}
		if start, end, action, ok := dismissHit(position); ok && action.isButton() {
			if action == tokenChipActionClose && props.OnDismissRun != nil && props.OnDismissRun(start, end) {
				props.onCaret(start)
				return
			}
			if action == tokenChipActionEdit && props.OnEditRun != nil && props.OnEditRun(start, end) {
				return
			}
		}
		if !props.ReadOnly && maxLines > 1 && props.OnTapBelowText != nil {
			lines := textFieldRichLines(state.Text, props.Window, style, innerWidth, softWrap, props.RichRuns)
			if contentPoint(position).Y+props.verticalOffset >= float32(len(lines))*props.LineHeight && props.OnTapBelowText() {
				return
			}
		}
		if offset, hit := glyphHitAt(position); hit && props.OnTapOffset != nil && props.OnTapOffset(offset) {
			return
		}
		props.onCaret(offsetAt(position))
	}, OnDoubleTapAt: func(position woxui.Point) {
		if props.Disabled || props.Window == nil || props.onWordSelection == nil {
			return
		}
		if _, _, action, ok := dismissHit(position); ok && action.isButton() {
			return
		}
		if offset, hit := glyphHitAt(position); hit && props.OnTapOffset != nil && props.OnTapOffset(offset) {
			return
		}
		props.onWordSelection(offsetAt(position))
	}, OnTripleTapAt: func(position woxui.Point) {
		if props.Disabled || props.Window == nil || props.onLineSelection == nil {
			return
		}
		if _, _, action, ok := dismissHit(position); ok && action.isButton() {
			return
		}
		if offset, hit := glyphHitAt(position); hit && props.OnTapOffset != nil && props.OnTapOffset(offset) {
			return
		}
		props.onLineSelection(offsetAt(position))
	}, OnSelectionStart: func(position woxui.Point, modifiers woxui.KeyModifiers) {
		if props.Disabled || props.Window == nil || props.onSelectionStart == nil {
			return
		}
		if _, _, action, ok := dismissHit(position); ok && action.isButton() {
			return
		}
		if offset, hit := glyphHitAt(position); hit && props.CursorAtOffset != nil && props.CursorAtOffset(offset) != woxui.PointerCursorText {
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
	}, Child: textFieldInnerContent(props, state, style, textColor, padding, radius, background, innerWidth, innerHeight, height, maxLines, softWrap),
	}
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
		TextLines: textFieldAccessibilityLines(state, style, innerWidth, maxLines, props),
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
			return woxui.TextInputState{Enabled: true, CursorRect: textFieldCursorRect(state, style, props.RichRuns, maxLines, props.LineHeight, props.verticalOffset, innerBounds, softWrap, props.Window)}
		},
		Child: content,
	}
	return field
}

// textFieldInnerContent stacks an optional left-gutter handle over the padded field.
func textFieldInnerContent(props TextFieldProps, state woxui.TextEditingState, style woxui.TextStyle, textColor woxui.Color, padding woxwidget.Insets, radius float32, background woxui.Color, innerWidth, innerHeight, height float32, maxLines int, softWrap bool) woxwidget.Widget {
	painter := woxwidget.CaretPainter{Width: innerWidth, Height: innerHeight, Active: props.caretActive, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect, focused, caretVisible bool) {
		if state.Text == "" && state.Composition == "" && props.Hint != "" {
			displayList.DrawText(props.Hint, textFieldAlignedTextBounds(bounds, props.Hint, style, props.TextAlignmentY, props.Window), style, props.Theme.ResultSubtitle)
		}
		if props.Window != nil {
			drawTextFieldTrailingHandleHighlight(displayList, bounds, state, style, props, softWrap)
			drawTextField(displayList, bounds, state, style, props.RichRuns, textColor, props.Theme, focused, caretVisible, props.PaintSelection, maxLines, props.LineHeight, props.verticalOffset, props.TextAlignmentY, softWrap, props.Window)
			drawTextFieldTrailingHandleDropLine(displayList, bounds, props)
		}
	}}
	content := woxwidget.Container{
		Width: props.Width, Height: height, Radius: radius, Color: background, BorderColor: props.BorderColor, BorderWidth: props.BorderWidth, Padding: padding,
		Child: woxwidget.Clip{Width: innerWidth, Height: innerHeight, Child: painter},
	}
	handle := textFieldTrailingHandleWidget(props, state, style, innerWidth, innerHeight, padding.Left, maxLines, softWrap)
	if handle == nil {
		return content
	}
	handleLeft := padding.Left + handle.bounds.X
	handleTop := padding.Top + handle.bounds.Y
	fieldHandle := handle.bounds
	fieldHandle.X, fieldHandle.Y = handleLeft, handleTop
	children := []woxwidget.StackChild{
		{Child: content},
		{Left: handleLeft, Top: handleTop, Child: handle.widget},
	}
	if ghost, left, top, ok := textFieldReorderGhost(props, fieldHandle, props.Width, height, textColor, style); ok {
		children = append(children, woxwidget.StackChild{Left: left, Top: top, Child: ghost})
	}
	return woxwidget.Stack{Width: props.Width, Height: height, Children: children}
}

type textFieldHandleOverlay struct {
	bounds woxui.Rect
	widget woxwidget.Widget
}

// textFieldTrailingHandleWidget places the drag handle in the left gutter of the first line.
func textFieldTrailingHandleWidget(props TextFieldProps, state woxui.TextEditingState, style woxui.TextStyle, innerWidth, innerHeight, leadingPad float32, maxLines int, softWrap bool) *textFieldHandleOverlay {
	spec := props.TrailingHandle
	if spec == nil || props.ReadOnly || props.Disabled || props.Window == nil || spec.Start > spec.End {
		return nil
	}
	size := spec.Size
	if size <= 0 {
		size = 16
	}
	bounds, ok := textFieldTrailingHandleBounds(state, props.Window, style, props.RichRuns, maxLines, props.LineHeight, props.verticalOffset, innerWidth, innerHeight, leadingPad, spec.Start, spec.End, size, softWrap)
	if !ok {
		return nil
	}
	color := props.Theme.ResultSubtitle
	if color.A == 0 {
		color = props.Theme.PreviewText
	}
	id := props.ID + ".reorder"
	label := spec.Label
	if label == "" {
		label = "Reorder"
	}
	onPanStart, onPanUpdate, onPanEnd := spec.OnPanStart, spec.OnPanUpdate, spec.OnPanEnd
	onHandleDrag := props.onTrailingHandleDrag
	handleTop := bounds.Y
	verticalOffset := props.verticalOffset
	contentYAt := func(position woxui.Point) float32 {
		return handleTop + position.Y + verticalOffset
	}
	icon := woxwidget.Widget(woxwidget.Align{Width: size, Height: size})
	if !props.handleDragging {
		icon = woxwidget.Align{Width: size, Height: size, Horizontal: 0.5, Vertical: 0.5, Child: GripDotsGlyph(16, color)}
	}
	return &textFieldHandleOverlay{bounds: bounds, widget: woxwidget.Semantics{
		Key: woxwidget.Key(id), AutomationID: id, Role: woxui.AccessibilityRoleButton, Label: label,
		Child: woxwidget.Gesture{ID: id, Cursor: woxui.PointerCursorMove, OnPanStart: func(position woxui.Point) {
			if onPanStart != nil {
				onPanStart()
			}
			if onHandleDrag != nil {
				onHandleDrag(position.Y, false)
			}
			if onPanUpdate != nil {
				onPanUpdate(contentYAt(position))
			}
		}, OnPanUpdate: func(position woxui.Point) {
			if onHandleDrag != nil {
				onHandleDrag(position.Y, false)
			}
			if onPanUpdate != nil {
				onPanUpdate(contentYAt(position))
			}
		}, OnPanEnd: func() {
			if onHandleDrag != nil {
				onHandleDrag(0, true)
			}
			if onPanEnd != nil {
				onPanEnd()
			}
		}, Child: icon},
	}}
}

// textFieldTrailingHandleBounds returns the inner-content box for a left-gutter handle.
func textFieldTrailingHandleBounds(state woxui.TextEditingState, window textFieldMeasurer, style woxui.TextStyle, richRuns []TextFieldRichRun, maxLines int, lineHeight, verticalOffset, innerWidth, innerHeight, leadingPad float32, start, end int, size float32, softWrap bool) (woxui.Rect, bool) {
	_ = maxLines
	if lineHeight <= 0 || size <= 0 {
		return woxui.Rect{}, false
	}
	lines := textFieldRichLines(state.Text, window, style, innerWidth, softWrap, richRuns)
	if len(lines) == 0 {
		return woxui.Rect{}, false
	}
	lineIndex := -1
	for index, line := range lines {
		if line.end < start || line.start > end {
			continue
		}
		lineIndex = index
		break
	}
	if lineIndex < 0 {
		return woxui.Rect{}, false
	}
	x := -leadingPad + max(float32(0), (leadingPad-size)*0.5)
	y := float32(lineIndex)*lineHeight + max(float32(0), (lineHeight-size)*0.5) - verticalOffset
	if y+size <= 0 || y >= innerHeight {
		return woxui.Rect{}, false
	}
	return woxui.Rect{X: x, Y: y, Width: size, Height: size}, true
}

// drawTextFieldTrailingHandleHighlight washes the dragged item before its text is painted.
func drawTextFieldTrailingHandleHighlight(displayList *woxui.DisplayList, bounds woxui.Rect, state woxui.TextEditingState, style woxui.TextStyle, props TextFieldProps, softWrap bool) {
	spec := props.TrailingHandle
	if spec == nil || !spec.Active || props.Window == nil || props.LineHeight <= 0 {
		return
	}
	wash := props.Theme.SelectionBackground
	if wash.A == 0 {
		wash = withAlpha(props.Theme.Cursor, 36)
	} else {
		wash.A = min(wash.A, 48)
	}
	for index, line := range textFieldRichLines(state.Text, props.Window, style, bounds.Width, softWrap, props.RichRuns) {
		if line.end < spec.Start || line.start > spec.End {
			continue
		}
		y := bounds.Y + float32(index)*props.LineHeight - props.verticalOffset
		if y+props.LineHeight <= bounds.Y || y >= bounds.Y+bounds.Height {
			continue
		}
		displayList.FillRoundedRect(woxui.Rect{X: bounds.X, Y: y, Width: bounds.Width, Height: props.LineHeight}, 4, wash)
	}
}

// drawTextFieldTrailingHandleDropLine paints the insert gap on top of the document.
func drawTextFieldTrailingHandleDropLine(displayList *woxui.DisplayList, bounds woxui.Rect, props TextFieldProps) {
	spec := props.TrailingHandle
	if spec == nil || !spec.ShowDropLine {
		return
	}
	y := bounds.Y + spec.DropLineY - props.verticalOffset
	if y < bounds.Y-2 || y > bounds.Y+bounds.Height+2 {
		return
	}
	color := props.Theme.Cursor
	if color.A == 0 {
		color = props.Theme.PreviewSplit
	}
	displayList.FillRoundedRect(woxui.Rect{X: bounds.X, Y: y - 1, Width: bounds.Width, Height: 2}, 1, color)
}

// textFieldReorderGhost is the floating preview that follows the pointer during a task drag.
func textFieldReorderGhost(props TextFieldProps, handle woxui.Rect, innerWidth, innerHeight float32, textColor woxui.Color, style woxui.TextStyle) (woxwidget.Widget, float32, float32, bool) {
	spec := props.TrailingHandle
	if spec == nil || !props.handleDragging || handle.Width <= 0 {
		return nil, 0, 0, false
	}
	size := handle.Width
	y := handle.Y + props.handleDragLocalY - size/2
	if y < 0 {
		y = 0
	}
	if y+size > innerHeight {
		y = max(float32(0), innerHeight-size)
	}
	color := props.Theme.ResultSubtitle
	if color.A == 0 {
		color = props.Theme.PreviewText
	}
	icon := woxwidget.Align{Width: size, Height: size, Horizontal: 0.5, Vertical: 0.5, Child: GripDotsGlyph(16, color)}
	preview := spec.Preview
	if preview == "" || props.Window == nil {
		return icon, handle.X, y, true
	}
	metrics, _ := props.Window.MeasureText(preview, style)
	pad, gap := float32(8), float32(6)
	width := min(innerWidth*0.75, pad+metrics.Size.Width+gap+size+4)
	if width < size {
		width = size
	}
	x := max(float32(0), handle.X)
	background := props.Theme.QueryBackground
	if background.A == 0 {
		background = props.Theme.Background
	}
	background.A = 230
	return woxwidget.Container{
		Width: width, Height: size + 4, Radius: 6, Color: background,
		Padding: woxwidget.Insets{Left: 4, Right: pad, Top: 2, Bottom: 2},
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: gap, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			icon,
			woxwidget.Text{Value: preview, Style: style, Color: textColor},
		}},
	}, x, max(float32(0), y-2), true
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
				breakAt := textFieldWrapBreak(remaining, fit, 0)
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

func textFieldRichLines(value string, window textFieldMeasurer, style woxui.TextStyle, width float32, softWrap bool, richRuns []TextFieldRichRun) []textFieldLine {
	if len(richRuns) == 0 || !softWrap || window == nil || width <= 0 {
		return textFieldLines(value, window, style, width, softWrap)
	}
	runes := []rune(value)
	spans := woxui.GraphemeSpans(value)
	lines := make([]textFieldLine, 0, strings.Count(value, "\n")+1)
	paragraphStart := 0
	for index := 0; index <= len(runes); index++ {
		atEnd := index == len(runes)
		atBreak := !atEnd && runes[index] == '\n'
		if !atEnd && !atBreak {
			continue
		}
		remaining := graphemeSpansInRange(spans, paragraphStart, index)
		if len(remaining) == 0 {
			lines = append(lines, textFieldLine{start: paragraphStart, end: paragraphStart})
		} else {
			hangIndent, hangSkip := textFieldHangingIndent(window, runes, paragraphStart, index, style, richRuns)
			firstLine := true
			for len(remaining) > 0 {
				lineWidth, lineIndent := width, float32(0)
				skipBefore := hangSkip
				if !firstLine {
					lineWidth, lineIndent = textFieldContinuationWidth(width, hangIndent)
					skipBefore = remaining[0].Start
				}
				fit := fittingRichGraphemePrefix(window, runes, remaining, style, richRuns, lineWidth)
				if fit >= len(remaining) {
					end := remaining[len(remaining)-1].End
					lines = append(lines, textFieldLine{start: remaining[0].Start, end: end, text: string(runes[remaining[0].Start:end]), indent: lineIndent})
					break
				}
				breakAt := textFieldWrapBreak(remaining, fit, skipBefore)
				end := remaining[breakAt-1].End
				start := remaining[0].Start
				lines = append(lines, textFieldLine{start: start, end: end, text: string(runes[start:end]), indent: lineIndent})
				remaining = remaining[breakAt:]
				firstLine = false
			}
		}
		if atBreak {
			paragraphStart = index + 1
		}
	}
	return lines
}

func fittingRichGraphemePrefix(window textFieldMeasurer, runes []rune, spans []woxui.GraphemeSpan, style woxui.TextStyle, richRuns []TextFieldRichRun, width float32) int {
	if len(spans) == 0 {
		return 0
	}
	low, high := 1, len(spans)
	for low < high {
		mid := low + (high-low+1)/2
		measured := textFieldMeasureRange(window, runes, spans[0].Start, spans[mid-1].End, style, richRuns)
		if measured <= width {
			low = mid
		} else {
			high = mid - 1
		}
	}
	return max(1, low)
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

const textFieldMinWrapWidth = float32(8)

// textFieldWrapBreak prefers a nearby space unless that would leave CJK or a list
// marker alone on the first line. Mixed CJK can break between characters.
func textFieldWrapBreak(spans []woxui.GraphemeSpan, fit int, skipBefore int) int {
	if fit <= 0 {
		return 1
	}
	if fit >= len(spans) {
		return len(spans)
	}
	breakAt := fit
	for candidate := fit - 1; candidate > 0; candidate-- {
		if spans[candidate].Start < skipBefore {
			break
		}
		cluster := []rune(spans[candidate].Text)
		if len(cluster) == 0 || !isTextFieldWrapSpace(cluster[0]) {
			continue
		}
		if textFieldWrapHasCJK(spans[candidate+1 : fit]) {
			break
		}
		return candidate + 1
	}
	return max(1, breakAt)
}

func textFieldWrapHasCJK(spans []woxui.GraphemeSpan) bool {
	for _, span := range spans {
		for _, current := range span.Text {
			if unicode.In(current, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
				return true
			}
		}
	}
	return false
}

func textFieldHangingIndent(window textFieldMeasurer, runes []rune, paragraphStart, paragraphEnd int, style woxui.TextStyle, richRuns []TextFieldRichRun) (float32, int) {
	for _, run := range richRuns {
		if !run.HangingIndent || run.End <= paragraphStart || run.Start >= paragraphEnd {
			continue
		}
		end := run.End
		for end < paragraphEnd && end < len(runes) && isTextFieldWrapSpace(runes[end]) {
			end++
		}
		return textFieldMeasureRange(window, runes, paragraphStart, end, style, richRuns), end
	}
	return 0, 0
}

func textFieldContinuationWidth(width, indent float32) (float32, float32) {
	if indent <= 0 || indent > width-textFieldMinWrapWidth {
		return width, 0
	}
	return width - indent, indent
}

func textFieldLineOriginX(baseX float32, line textFieldLine) float32 {
	return baseX + line.indent
}

// TextFieldVisualHeight is the pixel height that shows every soft-wrapped line.
func TextFieldVisualHeight(value string, window *woxui.Window, style woxui.TextStyle, width, lineHeight float32, padding woxwidget.Insets, runs []TextFieldRichRun) float32 {
	if lineHeight <= 0 {
		lineHeight = textFieldLineHeight
	}
	innerWidth := max(float32(0), width-padding.Left-padding.Right)
	lines := 1
	if window != nil && innerWidth > 0 {
		lines = max(1, len(textFieldRichLines(value, window, style, innerWidth, true, runs)))
	} else if value != "" {
		lines = 1 + strings.Count(value, "\n")
	}
	return max(lineHeight, float32(lines)*lineHeight+padding.Top+padding.Bottom)
}

func textFieldAccessibilityLines(state woxui.TextEditingState, style woxui.TextStyle, width float32, maxLines int, props TextFieldProps) []woxui.AccessibilityTextLine {
	if !props.ExposeVisualLines {
		return nil
	}
	return accessibilityLinesFromTextField(textFieldRichLines(state.Text, props.Window, style, width, maxLines > 1, props.RichRuns))
}

func accessibilityLinesFromTextField(lines []textFieldLine) []woxui.AccessibilityTextLine {
	visual := make([]woxui.AccessibilityTextLine, 0, len(lines))
	for _, line := range lines {
		visual = append(visual, woxui.AccessibilityTextLine{Text: line.text, Indent: line.indent})
	}
	return visual
}

func textFieldLineIndex(lines []textFieldLine, offset int) int {
	for index, line := range lines {
		if offset <= line.end || index == len(lines)-1 {
			return index
		}
	}
	return 0
}

func textFieldOffsetAt(state woxui.TextEditingState, window *woxui.Window, style woxui.TextStyle, richRuns []TextFieldRichRun, maxLines int, lineHeight, verticalOffset, width float32, softWrap bool, focused bool, point woxui.Point) int {
	return textFieldOffsetOnLines(state, window, style, richRuns, maxLines, lineHeight, verticalOffset, width, softWrap, focused, point)
}

// TextFieldCaretLayout is the content-local caret box used when remapping across layouts.
type TextFieldCaretLayout struct {
	Point     woxui.Point
	LineWidth float32
	AtLineEnd bool
}

// TextFieldCaretLayoutAt measures the caret's visual line so view toggles can keep it on screen.
func TextFieldCaretLayoutAt(value string, offset int, window *woxui.Window, style woxui.TextStyle, width, lineHeight float32, runs []TextFieldRichRun) (TextFieldCaretLayout, bool) {
	if lineHeight <= 0 || width <= 0 {
		return TextFieldCaretLayout{}, false
	}
	lines := textFieldRichLines(value, window, style, width, true, runs)
	if len(lines) == 0 {
		return TextFieldCaretLayout{}, false
	}
	index := textFieldLineIndex(lines, offset)
	line := lines[index]
	x := textFieldCaretX(line, offset, window, style)
	y := float32(index)*lineHeight + lineHeight*0.5
	lineWidth := x
	if window != nil {
		lineWidth = line.indent + textFieldMeasureRange(window, []rune(value), line.start, line.end, style, runs)
	}
	return TextFieldCaretLayout{
		Point:     woxui.Point{X: x, Y: y},
		LineWidth: lineWidth,
		AtLineEnd: offset == line.end,
	}, true
}

// TextFieldOffsetAtContentPoint maps a content-local point onto a rune offset.
func TextFieldOffsetAtContentPoint(value string, point woxui.Point, window *woxui.Window, style woxui.TextStyle, width, lineHeight float32, runs []TextFieldRichRun) int {
	return textFieldOffsetAt(woxui.TextEditingState{Text: value}, window, style, runs, 10000, lineHeight, 0, width, true, true, point)
}

// textFieldGlyphHitAt reports whether a content-local point sits on a rendered glyph, not just a snapped caret.
func textFieldGlyphHitAt(state woxui.TextEditingState, window textFieldMeasurer, style woxui.TextStyle, richRuns []TextFieldRichRun, maxLines int, lineHeight, verticalOffset, width float32, softWrap bool, focused bool, point woxui.Point) (int, bool) {
	if window == nil || lineHeight <= 0 {
		return 0, false
	}
	lines := textFieldRichLines(state.Text, window, style, width, softWrap, richRuns)
	if len(lines) == 0 {
		return 0, false
	}
	y := point.Y + verticalOffset
	if y < 0 || y >= float32(len(lines))*lineHeight {
		return 0, false
	}
	if maxLines == 1 {
		point.X += textFieldHorizontalOffset(focused, []rune(state.Text), state.Selection.Focus, style, width, window)
	}
	if point.X < 0 {
		return 0, false
	}
	line := lines[min(len(lines)-1, int(y/lineHeight))]
	if point.X < line.indent {
		return line.start, false
	}
	lineWidth := textFieldMeasureRange(window, []rune(state.Text), line.start, line.end, style, richRuns)
	if point.X > line.indent+lineWidth {
		return line.end, false
	}
	return textFieldOffsetOnLines(state, window, style, richRuns, maxLines, lineHeight, verticalOffset, width, softWrap, focused, point), true
}

func textFieldOffsetOnLines(state woxui.TextEditingState, window textFieldMeasurer, style woxui.TextStyle, richRuns []TextFieldRichRun, maxLines int, lineHeight, verticalOffset, width float32, softWrap bool, focused bool, point woxui.Point) int {
	lines := textFieldRichLines(state.Text, window, style, width, softWrap, richRuns)
	if len(lines) == 0 {
		return 0
	}
	lineIndex := min(len(lines)-1, max(0, int((point.Y+verticalOffset)/lineHeight)))
	line := lines[lineIndex]
	if maxLines == 1 {
		point.X += textFieldHorizontalOffset(focused, []rune(state.Text), state.Selection.Focus, style, width, window)
	}
	point.X -= line.indent
	if point.X < 0 {
		return line.start
	}
	spans := woxui.GraphemeSpans(line.text)
	if len(spans) == 0 {
		return line.start
	}
	offset := line.end - line.start
	previousWidth := float32(0)
	for candidate := 1; candidate <= len(spans); candidate++ {
		candidateEnd := line.start + spans[candidate-1].End
		measured := textFieldMeasureRange(window, []rune(state.Text), line.start, candidateEnd, style, richRuns)
		if point.X < (previousWidth+measured)*0.5 {
			offset = spans[candidate-1].Start
			break
		}
		previousWidth = measured
		offset = spans[candidate-1].End
	}
	return line.start + offset
}

// textFieldHorizontalOffset follows the caret only while focused so idle overflow stays left-aligned.
func textFieldHorizontalOffset(focused bool, runes []rune, focus int, style woxui.TextStyle, width float32, window textFieldMeasurer) float32 {
	if !focused || window == nil {
		return 0
	}
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

func drawTextField(displayList *woxui.DisplayList, bounds woxui.Rect, state woxui.TextEditingState, style woxui.TextStyle, richRuns []TextFieldRichRun, textColor woxui.Color, theme Theme, focused, caretVisible, paintSelection bool, maxLines int, lineHeight, verticalOffset, textAlignmentY float32, softWrap bool, window *woxui.Window) {
	richRuns = textFieldCompositionRichRuns(state, richRuns)
	displayRunes, start, end, focus, compositionStart, compositionEnd := textFieldDisplayState(state)
	lines := textFieldRichLines(string(displayRunes), window, style, bounds.Width, softWrap, richRuns)
	caretLine := textFieldLineIndex(lines, focus)
	visibleLines := max(1, min(maxLines, int(bounds.Height/lineHeight)))
	firstLine := max(0, int(verticalOffset/lineHeight))
	lineOffset := verticalOffset - float32(firstLine)*lineHeight
	lastLine := min(len(lines), firstLine+visibleLines+1)
	horizontalOffset := float32(0)
	if visibleLines == 1 {
		horizontalOffset = textFieldHorizontalOffset(focused, displayRunes, focus, style, bounds.Width, window)
	}
	for lineIndex := firstLine; lineIndex < lastLine; lineIndex++ {
		line := lines[lineIndex]
		y := bounds.Y + float32(lineIndex-firstLine)*lineHeight - lineOffset
		lineX := textFieldLineOriginX(bounds.X-horizontalOffset, line)
		selectionStart := max(start, line.start)
		selectionEnd := min(end, line.end)
		if (focused || paintSelection) && selectionStart < selectionEnd {
			prefixWidth := textFieldMeasureRange(window, displayRunes, line.start, selectionStart, style, richRuns)
			selectedWidth := textFieldMeasureRange(window, displayRunes, selectionStart, selectionEnd, style, richRuns)
			displayList.FillRoundedRect(woxui.Rect{X: lineX + prefixWidth, Y: y, Width: selectedWidth, Height: lineHeight}, 3, theme.SelectionBackground)
		}
		if gutter, ok := textFieldLineGutter(line, richRuns); ok && gutter.PaintLineGutter != nil {
			width := gutter.LineGutterWidth
			if width <= 0 {
				width = documentQuoteWidth(gutter.Style.Size)
			}
			gutter.PaintLineGutter(displayList, woxui.Rect{X: bounds.X - horizontalOffset, Y: y, Width: width, Height: lineHeight})
		}
		if (focused || paintSelection) && selectionStart < selectionEnd {
			prefixWidth := textFieldMeasureRange(window, displayRunes, line.start, selectionStart, style, richRuns)
			selectedWidth := textFieldMeasureRange(window, displayRunes, selectionStart, selectionEnd, style, richRuns)
			left := max(bounds.X, min(bounds.X+bounds.Width, lineX+prefixWidth))
			right := max(left, min(bounds.X+bounds.Width, lineX+prefixWidth+selectedWidth))
			// Keep full-line shaping and font fallback identical in every region.
			// Repainting a selected substring over the original text shifts mixed-script
			// baselines and doubles antialiased edges, especially on transparent fields.
			for index, clip := range []woxui.Rect{
				{X: bounds.X, Y: y, Width: left - bounds.X, Height: lineHeight},
				{X: left, Y: y, Width: right - left, Height: lineHeight},
				{X: right, Y: y, Width: bounds.X + bounds.Width - right, Height: lineHeight},
			} {
				if clip.Width <= 0 {
					continue
				}
				color := textColor
				if index == 1 {
					color = theme.SelectionText
				}
				displayList.PushClipRect(clip)
				drawTextFieldRichRange(displayList, window, displayRunes, line.start, line.end, lineX, bounds.X+bounds.Width, y, lineHeight, style, richRuns, color, index != 1, textAlignmentY)
				displayList.PopClipRect()
			}
		} else {
			drawTextFieldRichRange(displayList, window, displayRunes, line.start, line.end, lineX, bounds.X+bounds.Width, y, lineHeight, style, richRuns, textColor, true, textAlignmentY)
		}
	}
	if !focused {
		return
	}
	line := lines[caretLine]
	lineX := textFieldLineOriginX(bounds.X-horizontalOffset, line)
	cursorX := lineX + textFieldMeasureRange(window, displayRunes, line.start, focus, style, richRuns)
	cursorY := bounds.Y + float32(caretLine-firstLine)*lineHeight - lineOffset
	if compositionStart >= line.start && compositionEnd <= line.end {
		prefixWidth := textFieldMeasureRange(window, displayRunes, line.start, compositionStart, style, nil)
		compositionWidth := textFieldMeasureRange(window, displayRunes, compositionStart, compositionEnd, style, nil)
		displayList.FillRect(woxui.Rect{X: lineX + prefixWidth, Y: cursorY + lineHeight - 2, Width: compositionWidth, Height: 1}, theme.Cursor)
	}
	if caretVisible && start == end {
		displayList.FillRect(woxui.Rect{X: cursorX, Y: cursorY, Width: textFieldCursorWidth, Height: lineHeight}, theme.Cursor)
	}
}

func textFieldCursorRect(state woxui.TextEditingState, style woxui.TextStyle, richRuns []TextFieldRichRun, maxLines int, lineHeight, verticalOffset float32, bounds woxui.Rect, softWrap bool, window *woxui.Window) woxui.Rect {
	richRuns = textFieldCompositionRichRuns(state, richRuns)
	displayRunes, _, _, focus, _, _ := textFieldDisplayState(state)
	lines := textFieldRichLines(string(displayRunes), window, style, bounds.Width, softWrap, richRuns)
	caretLine := textFieldLineIndex(lines, focus)
	visibleLines := max(1, min(maxLines, int(bounds.Height/lineHeight)))
	firstLine := max(0, int(verticalOffset/lineHeight))
	lineOffset := verticalOffset - float32(firstLine)*lineHeight
	horizontalOffset := float32(0)
	if visibleLines == 1 {
		horizontalOffset = textFieldHorizontalOffset(true, displayRunes, focus, style, bounds.Width, window)
	}
	line := lines[caretLine]
	return woxui.Rect{
		X:     textFieldLineOriginX(bounds.X-horizontalOffset, line) + textFieldMeasureRange(window, displayRunes, line.start, focus, style, richRuns),
		Y:     bounds.Y + float32(caretLine-firstLine)*lineHeight - lineOffset,
		Width: textFieldCursorWidth, Height: lineHeight,
	}
}

func textFieldMeasureRange(window textFieldMeasurer, runes []rune, start, end int, base woxui.TextStyle, richRuns []TextFieldRichRun) float32 {
	width := float32(0)
	for _, segment := range textFieldRichSegments(start, end, base, richRuns) {
		if segment.Advance > 0 {
			width += segment.Advance
			continue
		}
		if segment.LineGutter && segment.Paint == nil && !segment.HideText {
			width += lineGutterWidth(segment)
			continue
		}
		metrics, _ := window.MeasureText(string(runes[segment.Start:segment.End]), segment.Style)
		width += metrics.Size.Width
	}
	return width
}

func textFieldRichSegments(start, end int, base woxui.TextStyle, richRuns []TextFieldRichRun) []TextFieldRichRun {
	if start >= end {
		return nil
	}
	boundaries := []int{start, end}
	for _, run := range richRuns {
		if run.End > start && run.Start < end {
			boundaries = append(boundaries, max(start, run.Start), min(end, run.End))
		}
	}
	slices.Sort(boundaries)
	boundaries = slices.Compact(boundaries)
	segments := make([]TextFieldRichRun, 0, len(boundaries)-1)
	for index := 0; index+1 < len(boundaries); index++ {
		segment := TextFieldRichRun{Start: boundaries[index], End: boundaries[index+1], Style: base}
		for _, run := range richRuns {
			if run.Start <= segment.Start && run.End >= segment.End {
				segment = run
				segment.Start = boundaries[index]
				segment.End = boundaries[index+1]
				if segment.Style.Size <= 0 {
					segment.Style = base
				}
			}
		}
		segments = append(segments, segment)
	}
	return segments
}

func drawTextFieldRichRange(displayList *woxui.DisplayList, window *woxui.Window, runes []rune, start, end int, x, right, y, lineHeight float32, base woxui.TextStyle, richRuns []TextFieldRichRun, color woxui.Color, useRunColor bool, alignment float32) {
	for _, segment := range textFieldRichSegments(start, end, base, richRuns) {
		if segment.Advance > 0 && segment.Paint != nil {
			segment.Paint(displayList, woxui.Rect{X: x, Y: y, Width: segment.Advance, Height: lineHeight})
			x += segment.Advance
			continue
		}
		if segment.LineGutter && segment.Paint == nil && !segment.HideText {
			x += lineGutterWidth(segment)
			continue
		}
		text := string(runes[segment.Start:segment.End])
		metrics, _ := window.MeasureText(text, segment.Style)
		if segment.Paint != nil && segment.HideText {
			paintWidth := metrics.Size.Width
			if segment.Advance <= 0 {
				paintWidth = max(float32(0), right-x)
			}
			segment.Paint(displayList, woxui.Rect{X: x, Y: y, Width: paintWidth, Height: lineHeight})
			x += metrics.Size.Width
			continue
		}
		bounds := woxui.Rect{X: x, Y: y, Width: metrics.Size.Width, Height: lineHeight}
		if segment.Background.A > 0 {
			displayList.FillRoundedRect(bounds, 3, segment.Background)
		}
		segmentColor := color
		if useRunColor && segment.Color.A > 0 {
			segmentColor = segment.Color
		}
		displayList.DrawText(text, textFieldAlignedTextBounds(bounds, text, segment.Style, alignment, window), segment.Style, segmentColor)
		if segment.Underline {
			displayList.FillRect(woxui.Rect{X: x, Y: y + lineHeight - 2, Width: metrics.Size.Width, Height: 1}, segmentColor)
		}
		if segment.Strike {
			displayList.FillRect(woxui.Rect{X: x, Y: y + lineHeight*0.52, Width: metrics.Size.Width, Height: 1}, segmentColor)
		}
		x += metrics.Size.Width
	}
}

// textFieldLineGutter keeps the decoration active across every soft-wrapped line in its block range.
func textFieldLineGutter(line textFieldLine, richRuns []TextFieldRichRun) (TextFieldRichRun, bool) {
	for _, run := range richRuns {
		if run.LineGutter && line.start >= run.Start && line.start <= run.End {
			return run, true
		}
	}
	return TextFieldRichRun{}, false
}

func lineGutterWidth(run TextFieldRichRun) float32 {
	if run.LineGutterWidth > 0 {
		return run.LineGutterWidth
	}
	return documentQuoteWidth(run.Style.Size)
}

// textFieldDismissibleHit reports a painted token chip under a content-local point.
func textFieldDismissibleHit(state woxui.TextEditingState, window textFieldMeasurer, style woxui.TextStyle, richRuns []TextFieldRichRun, maxLines int, lineHeight, verticalOffset, width float32, softWrap, focused bool, point woxui.Point, progress float32, hoveredStart int) (int, int, tokenChipAction, bool) {
	if lineHeight <= 0 {
		return 0, 0, tokenChipActionNone, false
	}
	lines := textFieldRichLines(state.Text, window, style, width, softWrap, richRuns)
	if len(lines) == 0 {
		return 0, 0, tokenChipActionNone, false
	}
	y := point.Y + verticalOffset
	if y < 0 || y >= float32(len(lines))*lineHeight {
		return 0, 0, tokenChipActionNone, false
	}
	if maxLines == 1 {
		point.X += textFieldHorizontalOffset(focused, []rune(state.Text), state.Selection.Focus, style, width, window)
	}
	if point.X < 0 {
		return 0, 0, tokenChipActionNone, false
	}
	line := lines[min(len(lines)-1, int(y/lineHeight))]
	x := line.indent
	runes := []rune(state.Text)
	for _, segment := range textFieldRichSegments(line.start, line.end, style, richRuns) {
		segmentWidth := textFieldMeasureRange(window, runes, segment.Start, segment.End, style, []TextFieldRichRun{segment})
		if point.X < x || point.X >= x+segmentWidth {
			x += segmentWidth
			continue
		}
		if !segment.Dismissible {
			return 0, 0, tokenChipActionNone, false
		}
		action := tokenChipActionBody
		if progress >= tokenChipCloseHitMin && segment.Start == hoveredStart {
			closeWidth := tokenChipCloseSlot * progress
			editWidth := float32(0)
			if segment.ChipEditable {
				editWidth = tokenChipEditSlot * progress
			}
			if closeWidth > 0 && point.X >= x+segmentWidth-closeWidth {
				action = tokenChipActionClose
			} else if editWidth > 0 && point.X >= x+segmentWidth-closeWidth-editWidth {
				action = tokenChipActionEdit
			}
		}
		return segment.Start, segment.End, action, true
	}
	return 0, 0, tokenChipActionNone, false
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

// textFieldCompositionRichRuns preserves decorations outside the IME replacement range and shifts following runs to their display offsets.
func textFieldCompositionRichRuns(state woxui.TextEditingState, richRuns []TextFieldRichRun) []TextFieldRichRun {
	if state.Composition == "" || len(richRuns) == 0 {
		return richRuns
	}
	textLength := len([]rune(state.Text))
	start := max(0, min(textLength, state.Selection.Start()))
	end := max(start, min(textLength, state.Selection.End()))
	compositionEnd := start + len([]rune(state.Composition))
	delta := compositionEnd - end
	adjusted := make([]TextFieldRichRun, 0, len(richRuns)+1)
	for _, run := range richRuns {
		switch {
		case run.End <= start:
			adjusted = append(adjusted, run)
		case run.Start >= end:
			run.Start += delta
			run.End += delta
			adjusted = append(adjusted, run)
		default:
			if run.Start < start {
				left := run
				left.End = start
				adjusted = append(adjusted, left)
			}
			if run.End > end {
				right := run
				right.Start = compositionEnd
				right.End += delta
				adjusted = append(adjusted, right)
			}
		}
	}
	return adjusted
}
