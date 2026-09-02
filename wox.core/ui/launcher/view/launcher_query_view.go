package view

import (
	"strconv"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	LauncherQueryInputKey           woxwidget.Key = "launcher-query-input-key"
	LauncherQueryLoadingBoundaryKey woxwidget.Key = "launcher-query-loading-boundary"
)

// Keep short queries pointer-editable before the trailing window-drag region begins.
const launcherQueryMinimumEditableWidth = float32(300)

// LauncherQueryProps contains the prepared text and callbacks for the launcher query editor.
type LauncherQueryProps struct {
	Width            float32
	Height           float32
	LineHeight       float32
	Style            woxui.TextStyle
	State            woxui.TextEditingState
	Lines            []LauncherQueryLine
	CompletionSuffix string
	CaretWidth       float32
	CaretLine        int
	CompositionWidth float32
	CompositionX     float32
	CompositionLine  int
	TextWidth        float32
	CaretHeight      float32
	Focused          bool
	Enabled          bool
	Theme            woxcomponent.Theme
	OnTapAt          func(woxui.Point) `boundary:"stable"`
	OnDoubleTapAt    func(woxui.Point) `boundary:"stable"`
	OnTripleTapAt    func(woxui.Point) `boundary:"stable"`
	OnTapEnd         func()            `boundary:"stable"`
	OnDragStart      func()            `boundary:"stable"`
	// OnSelectionStart begins a drag selection at the given editor-local point.
	OnSelectionStart func(woxui.Point, woxui.KeyModifiers) `boundary:"stable"`
	// OnSelectionExtend updates the active drag selection focus to the given editor-local point.
	OnSelectionExtend func(woxui.Point) `boundary:"stable"`
	// OnSecondaryTapDown opens the query context menu at window/client coordinates.
	OnSecondaryTapDown func(woxui.Point)               `boundary:"stable"`
	OnKey              func(woxui.KeyEvent) bool       `boundary:"stable"`
	OnTextInput        func(woxui.TextInputEvent) bool `boundary:"stable"`
	OnFocusChange      func(bool)                      `boundary:"stable"`
	OnSetValue         func(string) error              `boundary:"stable"`
	OnTextInputState   func(woxui.TextInputState)      `boundary:"stable"`
	OnSelectAll        func() error                    `boundary:"stable"`
	OnCopy             func() error                    `boundary:"stable"`
	OnCut              func() error                    `boundary:"stable"`
	OnPaste            func() error                    `boundary:"stable"`
}

// Equal compares every prepared rendering dependency for the launcher query editor.
func (p LauncherQueryProps) Equal(other LauncherQueryProps) bool {
	if p.Width != other.Width || p.Height != other.Height || p.LineHeight != other.LineHeight || p.Style != other.Style || p.State != other.State || p.CompletionSuffix != other.CompletionSuffix || p.CaretWidth != other.CaretWidth || p.CaretLine != other.CaretLine || p.CompositionWidth != other.CompositionWidth || p.CompositionX != other.CompositionX || p.CompositionLine != other.CompositionLine || p.TextWidth != other.TextWidth || p.CaretHeight != other.CaretHeight || p.Focused != other.Focused || p.Enabled != other.Enabled || p.Theme != other.Theme || len(p.Lines) != len(other.Lines) {
		return false
	}
	for index := range p.Lines {
		if p.Lines[index] != other.Lines[index] {
			return false
		}
	}
	return true
}

// LauncherQueryBoundary retains the query editor while keeping scroll chrome outside the cache.
// Scroll offset and scrollbar animation are retained widgets, so caching them makes
// WOX_DEBUG_REPAINT=verify compare a live scrolled paint with a shadow rebuild at offset 0.
func LauncherQueryBoundary(props LauncherQueryProps) woxwidget.Widget {
	return launcherQueryScrollSurface(props, woxwidget.Boundary[LauncherQueryProps]{
		Key: "launcher-query-boundary", Label: "header:query", Props: props,
		Build: launcherQueryEditor,
	})
}

// LauncherQueryLine contains adapter-measured text slices for one query line.
type LauncherQueryLine struct {
	Text          string
	Selected      string
	PrefixWidth   float32
	SelectedWidth float32
	TextWidth     float32
}

// LauncherHeaderProps contains the query box and its optional accessories.
type LauncherHeaderProps struct {
	Width             float32
	Height            float32
	QueryBoxHeight    float32
	QueryEditorHeight float32
	DensityScale      float32
	QueryWidth        float32
	QueryRadius       float32
	AppPadding        woxwidget.Insets
	Theme             woxcomponent.Theme
	Query             LauncherQueryProps
	Refinement        woxwidget.Widget
	RefinementWidth   float32
	Glance            woxwidget.Widget
	GlanceWidth       float32
	Icon              *woxui.Image
	// Icons stacks Scope plugin icons (usually 1-3). When set, Icon is unused.
	Icons        []*woxui.Image
	Loading      bool
	LoadingWidth float32
	LoadingSize  float32
	LoadingColor woxui.Color
	// OnDragStart starts a window drag when the pointer presses the header's
	// empty padding around the query pill (e.g. above the input box).
	OnDragStart func()
}

type launcherQueryLoadingProps struct {
	Width  float32
	Height float32
	Size   float32
	Color  woxui.Color
}

// LauncherScopeIconsWidth returns the full accessory slot used by a stacked scope icon group.
func LauncherScopeIconsWidth(iconCount int, densityScale float32) float32 {
	if iconCount <= 0 {
		return 0
	}
	iconSize := scaledLauncherSize(30, densityScale)
	overlap := scaledLauncherSize(14, densityScale)
	rightPadding := scaledLauncherSize(19, densityScale)
	return iconSize + float32(iconCount-1)*overlap + rightPadding
}

func (p launcherQueryLoadingProps) Equal(other launcherQueryLoadingProps) bool {
	return p == other
}

// LauncherHeaderView builds the query box and prepared accessory views.
func LauncherHeaderView(props LauncherHeaderProps) woxwidget.Widget {
	queryLeftPadding := scaledLauncherSize(8, props.DensityScale)
	accessoryGap := scaledLauncherSize(12, props.DensityScale)
	children := []woxwidget.Widget{woxwidget.Expanded{Child: woxwidget.Align{
		Width: props.QueryWidth, Height: props.QueryBoxHeight, Vertical: 0.5,
		Child: LauncherQueryBoundary(props.Query),
	}}}
	if props.Refinement != nil {
		children = append(children, woxwidget.Align{
			Width: props.RefinementWidth, Height: props.QueryBoxHeight, Vertical: 0.5, Child: props.Refinement,
		})
	}
	if props.Glance != nil {
		children = append(children, woxwidget.Align{
			Width: props.GlanceWidth, Height: props.QueryBoxHeight, Vertical: 0.5, Child: props.Glance,
		})
	}
	if len(props.Icons) > 0 {
		iconSize := scaledLauncherSize(30, props.DensityScale)
		overlap := scaledLauncherSize(14, props.DensityScale)
		iconRightPadding := scaledLauncherSize(19, props.DensityScale)
		accessoryWidth := LauncherScopeIconsWidth(len(props.Icons), props.DensityScale)
		stackWidth := accessoryWidth - iconRightPadding
		iconContainerHeight := scaledLauncherSize(34, props.DensityScale)
		stackChildren := make([]woxwidget.StackChild, 0, len(props.Icons))
		for index, icon := range props.Icons {
			if icon == nil {
				continue
			}
			stackChildren = append(stackChildren, woxwidget.StackChild{
				Left: float32(index) * overlap, Top: 0,
				Child: woxwidget.Image{Source: icon, Width: iconSize, Height: iconSize},
			})
		}
		children = append(children, woxwidget.Semantics{
			Key: "launcher-query-scope-icons-key", AutomationID: "launcher.query.scope-icons", Role: woxui.AccessibilityRoleGroup,
			Label: "Scoped plugins", Value: strconv.Itoa(len(stackChildren)), ReadOnly: true,
			Child: woxwidget.Align{
				Width: accessoryWidth, Height: props.QueryBoxHeight, Vertical: 0.5,
				Child: woxwidget.Container{
					Width: stackWidth, Height: iconContainerHeight, Padding: woxwidget.Insets{Top: scaledLauncherSize(2, props.DensityScale)},
					Child: woxwidget.Stack{Width: stackWidth, Height: iconSize, Children: stackChildren},
				},
			},
		})
	} else if props.Icon != nil {
		iconSize := scaledLauncherSize(30, props.DensityScale)
		// Flutter centers the icon in a 68px accessory slot, leaving 19px after it.
		iconRightPadding := scaledLauncherSize(19, props.DensityScale)
		iconContainerHeight := scaledLauncherSize(34, props.DensityScale)
		children = append(children, woxwidget.Align{
			Width: iconSize + iconRightPadding, Height: props.QueryBoxHeight, Vertical: 0.5,
			Child: woxwidget.Container{Width: iconSize, Height: iconContainerHeight, Padding: woxwidget.Insets{Top: scaledLauncherSize(2, props.DensityScale)}, Child: woxwidget.Image{Source: props.Icon, Width: iconSize, Height: iconSize}},
		})
	}
	if props.Loading {
		loadingProps := launcherQueryLoadingProps{Width: props.LoadingWidth, Height: props.QueryBoxHeight, Size: props.LoadingSize, Color: props.LoadingColor}
		children = append(children, woxwidget.Semantics{
			Key: "launcher-query-loading-key", AutomationID: "launcher.query.loading", Role: woxui.AccessibilityRoleProgressBar,
			Label: "Search in progress", Value: "loading", ReadOnly: true,
			Child: woxwidget.Boundary[launcherQueryLoadingProps]{
				Key: LauncherQueryLoadingBoundaryKey, Label: "header:loading", Props: loadingProps,
				Build: func(props launcherQueryLoadingProps) woxwidget.Widget {
					return woxwidget.Align{
						Width: props.Width, Height: props.Height, Horizontal: 0.5, Vertical: 0.5,
						Child: woxcomponent.WoxLoadingIndicator(props.Size, props.Color),
					}
				},
			},
		})
	}
	header := woxwidget.Widget(woxwidget.Container{
		Width: props.Width, Height: props.Height,
		// Bottom padding is used when the query box is anchored at the bottom so
		// explorer/dialog overlays keep symmetric chrome instead of leaving the
		// theme inset as empty space above the query pill.
		Padding: woxwidget.Insets{Left: props.AppPadding.Left, Top: props.AppPadding.Top, Right: props.AppPadding.Right, Bottom: props.AppPadding.Bottom},
		Child: woxwidget.Constrained{FillWidth: true, Child: woxwidget.Container{
			Height: props.QueryBoxHeight, Radius: props.QueryRadius, Color: props.Theme.QueryBackground,
			Padding: woxwidget.Insets{Left: queryLeftPadding, Right: scaledLauncherSize(6, props.DensityScale)},
			Child:   woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: accessoryGap, Children: children},
		}},
	})
	if props.OnDragStart == nil {
		return header
	}
	// Nested gestures (query editor, buttons) still win the hit test over this
	// wrapper, so only the empty padding around the pill starts a window drag.
	return woxwidget.Gesture{ID: "launcher-header-drag", OnDragStart: props.OnDragStart, Child: header}
}

// LauncherQueryView builds the query editor from adapter-prepared text metrics.
func LauncherQueryView(props LauncherQueryProps) woxwidget.Widget {
	return launcherQueryScrollSurface(props, launcherQueryEditor(props))
}

// launcherQueryLineMetrics returns the painted line box, display lines, and scrollable content height.
func launcherQueryLineMetrics(props LauncherQueryProps) (lineHeight float32, lines []LauncherQueryLine, contentHeight float32) {
	lineHeight = props.LineHeight
	if lineHeight <= 0 {
		lineHeight = props.CaretHeight
	}
	lines = props.Lines
	if len(lines) == 0 {
		lines = []LauncherQueryLine{{}}
	}
	return lineHeight, lines, max(props.Height, float32(len(lines))*lineHeight)
}

// launcherQueryEditor builds the retained query text painter without scroll chrome.
func launcherQueryEditor(props LauncherQueryProps) woxwidget.Widget {
	const cursorWidth = float32(2)
	pointerCursor := woxui.PointerCursorText
	if !props.Enabled {
		pointerCursor = woxui.PointerCursorDefault
	}
	lineHeight, lines, contentHeight := launcherQueryLineMetrics(props)

	var editor woxwidget.Widget = woxwidget.Gesture{
		ID:     "query-editor",
		Cursor: pointerCursor,
		OnTapAt: func(position woxui.Point) {
			if props.OnTapAt != nil {
				props.OnTapAt(position)
			}
		},
		OnDoubleTapAt: func(position woxui.Point) {
			if props.OnDoubleTapAt != nil {
				props.OnDoubleTapAt(position)
			}
		},
		OnTripleTapAt: func(position woxui.Point) {
			if props.OnTripleTapAt != nil {
				props.OnTripleTapAt(position)
			}
		},
		OnSelectionStart: func(position woxui.Point, modifiers woxui.KeyModifiers) {
			if props.OnSelectionStart != nil {
				props.OnSelectionStart(position, modifiers)
			}
		},
		OnSelectionExtend: func(position woxui.Point) {
			if props.OnSelectionExtend != nil {
				props.OnSelectionExtend(position)
			}
		},
		OnSecondaryTapDown: func(position woxui.Point) {
			if props.OnSecondaryTapDown != nil {
				props.OnSecondaryTapDown(position)
			}
		},
		Child: woxwidget.CaretPainter{Width: props.Width, Height: contentHeight, Active: props.Focused, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect, focused, caretVisible bool) {
			textTop := bounds.Y + max(float32(0), bounds.Height-float32(len(lines))*lineHeight)/2
			lastLine := lines[len(lines)-1]
			if focused && props.State.Composition == "" && props.CompletionSuffix != "" {
				hintColor := props.Theme.QueryText
				hintColor.A = 96
				displayList.DrawText(props.CompletionSuffix, woxui.Rect{X: bounds.X + lastLine.TextWidth, Y: textTop + float32(len(lines)-1)*lineHeight, Width: max(float32(0), bounds.Width-lastLine.TextWidth), Height: lineHeight}, props.Style, hintColor)
			}
			for index, line := range lines {
				lineY := textTop + float32(index)*lineHeight
				if focused && props.State.Composition == "" && line.Selected != "" {
					displayList.FillRoundedRect(woxui.Rect{X: bounds.X + line.PrefixWidth, Y: lineY, Width: line.SelectedWidth, Height: props.CaretHeight}, 3, props.Theme.SelectionBackground)
				}
				displayList.DrawText(line.Text, woxui.Rect{X: bounds.X, Y: lineY, Width: bounds.Width, Height: lineHeight}, props.Style, props.Theme.QueryText)
				if focused && props.State.Composition == "" && line.Selected != "" {
					displayList.DrawText(line.Selected, woxui.Rect{X: bounds.X + line.PrefixWidth, Y: lineY, Width: line.SelectedWidth, Height: lineHeight}, props.Style, props.Theme.SelectionText)
				}
			}
			if !focused {
				return
			}

			cursorX := bounds.X + props.CaretWidth
			caretY := textTop + float32(props.CaretLine)*lineHeight
			// Native text fields hide the blinking caret once a range is selected.
			if caretVisible && props.State.Selection.Collapsed() {
				displayList.FillRect(woxui.Rect{X: cursorX, Y: caretY, Width: cursorWidth, Height: props.CaretHeight}, props.Theme.Cursor)
			}
			if props.OnTextInputState != nil {
				props.OnTextInputState(woxui.TextInputState{Enabled: true, CursorRect: woxui.Rect{X: cursorX, Y: caretY, Width: cursorWidth, Height: props.CaretHeight}})
			}
			if props.State.Composition != "" {
				compositionY := textTop + float32(props.CompositionLine)*lineHeight
				displayList.FillRect(woxui.Rect{X: bounds.X + props.CompositionX, Y: compositionY + props.CaretHeight - 1, Width: props.CompositionWidth, Height: 1}, props.Theme.Cursor)
			}
		}},
	}
	editor = woxwidget.EditableText{
		Key:              LauncherQueryInputKey,
		AutomationID:     "launcher.query.input",
		Label:            "Search Wox",
		Value:            props.State.Text,
		Autofocus:        true,
		Disabled:         !props.Enabled,
		OnKey:            props.OnKey,
		OnTextInput:      props.OnTextInput,
		OnFocusChange:    props.OnFocusChange,
		OnSetValue:       props.OnSetValue,
		HasTextSelection: true,
		SelectionStart:   props.State.Selection.Start(),
		SelectionEnd:     props.State.Selection.End(),
		OnSelectAll:      props.OnSelectAll,
		OnCopy:           props.OnCopy,
		OnCut:            props.OnCut,
		OnPaste:          props.OnPaste,
		TextInput: func(bounds woxui.Rect) woxui.TextInputState {
			textTop := max(float32(0), bounds.Height-float32(len(lines))*lineHeight) / 2
			return woxui.TextInputState{Enabled: true, CursorRect: woxui.Rect{X: bounds.X + props.CaretWidth, Y: bounds.Y + textTop + float32(props.CaretLine)*lineHeight, Width: cursorWidth, Height: props.CaretHeight}}
		},
		Child: editor,
	}
	if props.CompletionSuffix != "" {
		editor = woxwidget.Stack{Width: props.Width, Height: contentHeight, Children: []woxwidget.StackChild{
			{Child: editor},
			{Child: woxwidget.Semantics{
				Key: "launcher-query-completion-key", AutomationID: "launcher.query.completion", Role: woxui.AccessibilityRoleText,
				Label: "Query completion", Value: props.CompletionSuffix, ReadOnly: true, LiveRegion: woxui.AccessibilityLiveRegionPolite,
				Child: woxwidget.Container{},
			}},
		}}
	}
	return editor
}

// launcherQueryScrollSurface clips overflowing query lines and reserves the trailing window-drag region.
func launcherQueryScrollSurface(props LauncherQueryProps, editor woxwidget.Widget) woxwidget.Widget {
	lineHeight, _, contentHeight := launcherQueryLineMetrics(props)
	keepVisible := &woxwidget.ScrollRange{Start: float32(props.CaretLine) * lineHeight, End: float32(props.CaretLine)*lineHeight + props.CaretHeight}
	editor = woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: "launcher-query-scroll", Content: editor, Width: props.Width, Height: props.Height, ContentHeight: contentHeight,
		KeepVisible: keepVisible, ThumbColor: props.Theme.ResultTitle, AlwaysShowScrollbar: true,
		AutomationID: "launcher.query.scroll", Label: "Query scroll position",
	})
	dragLeft := min(props.Width, launcherQueryMinimumEditableWidth+props.TextWidth)
	dragRight := float32(0)
	if contentHeight > props.Height {
		dragRight = 14
	}
	if dragLeft >= props.Width-dragRight {
		return editor
	}
	return woxwidget.Stack{
		Width: props.Width, Height: props.Height,
		Children: []woxwidget.StackChild{
			{Child: editor},
			{Left: dragLeft, Child: woxwidget.Gesture{
				ID:          "query-drag-area",
				OnTap:       props.OnTapEnd,
				OnDragStart: props.OnDragStart,
				Child:       woxwidget.Container{Width: props.Width - dragLeft - dragRight, Height: props.Height},
			}},
		},
	}
}
