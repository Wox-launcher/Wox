package component

import (
	"fmt"
	"slices"
	"strings"

	"wox/common"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const noteEditorSegmentGap = float32(8)

// NoteEditorProps describes the Notes document surface hosted by the utility window.
type NoteEditorProps struct {
	ID                   string
	Document             common.NoteDocument
	Width                float32
	Height               float32
	Padding              woxwidget.Insets
	Style                woxui.TextStyle
	LineHeight           float32
	Zoom                 float32
	TextColor            woxui.Color
	Theme                Theme
	Window               *woxui.Window
	ReadOnly             bool
	Autofocus            bool
	Controller           *woxwidget.TextEditingController
	FocusNode            *woxwidget.FocusNode
	Focused              bool
	Selection            woxui.TextSelection
	Label                string
	OnChanged            func(segmentStart int, value string)
	OnSelectionChanged   func(woxui.TextSelection)
	OnTapOffset          func(int) bool
	OnTapBelowText       func() bool
	CursorAtOffset       func(int) woxui.PointerCursor
	OnKey                func(woxui.KeyEvent) bool
	OnUndo               func() bool
	OnRedo               func() bool
	OnPaste              func(string) bool
	TransformPaste       func(string) string
	OnTableChange        func(block int, table common.NoteTable)
	OnTableFocus         func(block, row, column int)
	OnTableKey           func(block, row, column int, event woxui.KeyEvent) bool
	OnTablePaste         func(block, row, column int, value string) bool
	OnTableInsertRow     func(block int)
	OnTableInsertColumn  func(block int)
	OnTableDeleteRow     func(block int)
	OnTableDeleteColumn  func(block int)
	OnTableDelete        func(block int)
	OnTableActionHover   func(inside bool, label string, bounds woxui.Rect)
	OnDeleteEmptySegment func(segmentStart int) bool
	TableActionLabels    NoteTableActionLabels
	FocusedTableBlock    int
	FocusedTableRow      int
	FocusedTableCol      int
	ResolveImage         func(common.NoteImage) *woxui.Image
	MissingImageLabel    string
	ActiveSegmentStart   int
	FocusedImageBlock    int
	OnImageFocus         func(block int)
	OnImageLeave         func(block int, after bool)
	OnImageScale         func(block, delta int)
	OnImageDelete        func(block int)
	OnImageActionHover   func(inside bool, label string, bounds woxui.Rect)
	OnTextFocus          func(segmentStart int)
	ImageActionLabels    NoteImageActionLabels
}

// WoxNoteEditor renders linear Notes text segments and structural table and image blocks.
func WoxNoteEditor(props NoteEditorProps) woxwidget.Widget {
	if props.FocusedTableBlock < 0 {
		props.FocusedTableBlock = -1
	}
	if props.FocusedImageBlock < 0 {
		props.FocusedImageBlock = -1
	}
	if props.ActiveSegmentStart < 0 {
		props.ActiveSegmentStart = -1
	}
	segments := noteDocumentSegments(props.Document)
	if len(segments) == 1 && !segments[0].Structural() {
		return noteEditorTextField(props, segments[0], props.ID, props.Height, true, props.Padding)
	}
	children := make([]woxwidget.Widget, 0, len(segments))
	textIndex := 0
	contentHeight := float32(0)
	for _, segment := range segments {
		if len(children) > 0 {
			contentHeight += noteEditorSegmentGap
		}
		if segment.Image {
			block := props.Document.Blocks[segment.Start]
			padding := noteEditorEdgePadding(props.Padding, len(children) == 0)
			width := max(float32(0), props.Width-props.Padding.Left-props.Padding.Right)
			child, height := noteEditorImage(props, segment.Start, block, width)
			children = append(children, woxwidget.Container{Width: props.Width, Padding: padding, Child: child})
			contentHeight += padding.Top + height
			continue
		}
		if segment.Table {
			block := props.Document.Blocks[segment.Start]
			table := common.NoteTable{}
			if block.Table != nil {
				table = *block.Table
			}
			height := noteTableBlockHeight(table, props.Zoom, !props.ReadOnly && props.OnTableInsertRow != nil)
			padding := noteEditorEdgePadding(props.Padding, len(children) == 0)
			children = append(children, woxwidget.Container{Width: props.Width, Padding: padding, Child: WoxNoteTable(NoteTableProps{
				ID: fmt.Sprintf("%s.table.%s", props.ID, block.ID), Block: segment.Start, Table: table,
				Width: max(float32(0), props.Width-props.Padding.Left-props.Padding.Right), ReadOnly: props.ReadOnly,
				Theme: props.Theme, Window: props.Window, Zoom: props.Zoom, Style: props.Style,
				Focused: props.FocusedTableBlock == segment.Start, FocusRow: props.FocusedTableRow, FocusCol: props.FocusedTableCol,
				OnChange: func(updated common.NoteTable) {
					if props.OnTableChange != nil {
						props.OnTableChange(segment.Start, updated)
					}
				},
				OnFocus: func(row, column int) {
					if props.OnTableFocus != nil {
						props.OnTableFocus(segment.Start, row, column)
					}
				},
				OnKey: func(row, column int, event woxui.KeyEvent) bool {
					if props.OnTableKey != nil {
						return props.OnTableKey(segment.Start, row, column, event)
					}
					return false
				},
				OnPaste: func(row, column int, value string) bool {
					if props.OnTablePaste != nil {
						return props.OnTablePaste(segment.Start, row, column, value)
					}
					return false
				},
				OnUndo: props.OnUndo, OnRedo: props.OnRedo,
				Actions: noteEditorTableActions(props, segment.Start),
			})})
			contentHeight += padding.Top + height
			continue
		}
		fieldID := props.ID
		if textIndex > 0 {
			fieldID = fmt.Sprintf("%s.%d", props.ID, textIndex)
		}
		projected, _, _ := ProjectNoteDocument(noteSegmentDocument(props.Document, segment), props.Style, props.Theme)
		lines := 1
		if projected != "" {
			lines = 1 + countNoteNewlines(projected)
		}
		padding := noteEditorEdgePadding(props.Padding, len(children) == 0)
		height := max(props.LineHeight, float32(lines)*props.LineHeight+padding.Top+padding.Bottom)
		if segment.End == len(props.Document.Blocks) {
			height = max(height, props.Height-contentHeight)
		}
		primary := textIndex == 0
		if props.ActiveSegmentStart >= 0 {
			primary = segment.Start == props.ActiveSegmentStart
		}
		children = append(children, noteEditorTextField(props, segment, fieldID, height, primary, padding))
		contentHeight += height
		textIndex++
	}
	// Omit ContentHeight so the shared scroller measures the document and can show a vertical thumb.
	var content woxwidget.Widget = woxwidget.Flex{Axis: woxwidget.Vertical, Gap: noteEditorSegmentGap, Children: children}
	if props.Padding.Bottom > 0 {
		// Tables sit outside the text field, so the document scroller must own the trailing inset.
		content = woxwidget.Container{Width: props.Width, Padding: woxwidget.Insets{Bottom: props.Padding.Bottom}, Child: content}
	}
	return WoxScrollView(ScrollViewProps{
		Key: "notes.editor.scroll", AutomationID: "notes.editor.scroll", Label: props.Label,
		Width: props.Width, Height: props.Height, Content: content,
		ThumbColor: props.Theme.ResultSubtitle,
	})
}

func noteEditorTextField(props NoteEditorProps, segment NoteDocumentSegment, id string, height float32, primary bool, padding woxwidget.Insets) woxwidget.Widget {
	value, runs, _ := ProjectNoteSegment(props.Document, segment, props.Style, props.Theme)
	controller := (*woxwidget.TextEditingController)(nil)
	focus := (*woxwidget.FocusNode)(nil)
	focused := false
	autofocus := false
	if primary {
		controller = props.Controller
		focus = props.FocusNode
		focused = props.Focused
		// Reclaiming focus after a structural selection would clear it through OnTextFocus.
		autofocus = props.Autofocus && props.FocusedImageBlock < 0 && props.FocusedTableBlock < 0
		if controller != nil {
			value = controller.Text()
		}
	}
	start := segment.Start
	empty := strings.TrimSpace(value) == ""
	var onTapBelowText func() bool
	if segment.End == len(props.Document.Blocks) {
		onTapBelowText = props.OnTapBelowText
	}
	return WoxTextField(TextFieldProps{
		ID: id, Label: props.Label, Width: props.Width, Height: height, Padding: padding,
		Transparent: true, DisableHover: true, Style: props.Style, RichRuns: NoteFieldRuns(runs),
		LineHeight: props.LineHeight, TextAlignmentY: 0.5, TextColor: props.TextColor, Value: value,
		Controller: controller, FocusNode: focus, Focused: focused, Autofocus: autofocus,
		ReadOnly: props.ReadOnly, MaxLines: 10000, Window: props.Window, Theme: props.Theme,
		OnChanged: func(text string) {
			if props.OnChanged != nil {
				props.OnChanged(start, text)
			}
		},
		OnSelectionChanged: props.OnSelectionChanged,
		OnFocusChange: func(focused bool) {
			if focused && props.OnTextFocus != nil {
				props.OnTextFocus(start)
			}
		},
		OnTapOffset:    props.OnTapOffset,
		OnTapBelowText: onTapBelowText,
		CursorAtOffset: props.CursorAtOffset,
		OnKey: func(event woxui.KeyEvent) bool {
			if noteEditorEmptyBoundaryKey(event) && empty && props.OnDeleteEmptySegment != nil && props.OnDeleteEmptySegment(start) {
				return true
			}
			if props.OnKey != nil {
				return props.OnKey(event)
			}
			return false
		},
		OnUndo:         props.OnUndo,
		OnRedo:         props.OnRedo,
		OnPaste:        props.OnPaste,
		TransformPaste: props.TransformPaste,
	})
}

const (
	NoteEditorImageMaxHeight         = float32(360)
	noteEditorImagePlaceholderHeight = float32(52)
	noteEditorImageToolbarHeight     = float32(28)
	noteEditorImageToolbarGap        = float32(4)
	noteEditorImageScaleStep         = 10
	noteEditorImageHighlightPad      = float32(2)
)

// NoteImageActionLabels holds already-translated image structure commands.
type NoteImageActionLabels struct {
	Smaller, Larger, Delete string
}

// DeleteNoteImage removes one image block and keeps a text block when the document would be empty.
func DeleteNoteImage(document common.NoteDocument, block int) common.NoteDocument {
	if block < 0 || block >= len(document.Blocks) || document.Blocks[block].Type != common.NoteBlockImage {
		return document
	}
	updated := CloneNoteDocument(document)
	updated.Blocks = slices.Delete(updated.Blocks, block, block+1)
	return ensureNoteDocumentHasTextBlock(updated)
}

// noteEditorImage renders one attachment with the same reserved toolbar slot as tables.
func noteEditorImage(props NoteEditorProps, blockIndex int, block common.NoteBlock, width float32) (woxwidget.Widget, float32) {
	focused := props.FocusedImageBlock == blockIndex
	picture, pictureHeight := noteEditorImagePicture(props, block, width, focused)
	drawWidth, drawHeight := noteEditorImageSize(nil, block.Image, width, props.Zoom)
	if image := noteEditorResolvedImage(props, block); image != nil {
		drawWidth, drawHeight = noteEditorImageSize(image, block.Image, width, props.Zoom)
	}
	if !props.ReadOnly && props.OnImageFocus != nil {
		id := fmt.Sprintf("%s.image.%s", props.ID, block.ID)
		picture = woxwidget.Semantics{
			Key: woxwidget.Key(id), AutomationID: id, Role: woxui.AccessibilityRoleImage,
			Label: noteEditorImageLabel(props, block), Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
			OnAction: func(action woxui.AccessibilityAction, _ string) error {
				if action == woxui.AccessibilityActionActivate && props.OnImageFocus != nil {
					props.OnImageFocus(blockIndex)
				}
				return nil
			},
			Child: woxwidget.Gesture{ID: id, OnTapAt: func(position woxui.Point) {
				if noteEditorImageTapHitsPicture(position, width, drawWidth, drawHeight) {
					if props.OnImageFocus != nil {
						props.OnImageFocus(blockIndex)
					}
					return
				}
				if props.OnImageLeave != nil {
					props.OnImageLeave(blockIndex, position.Y >= pictureHeight/2)
				}
			}, Child: picture},
		}
	}
	toolbar := noteEditorImageToolbar(props, blockIndex, width, focused)
	if toolbar != nil && !focused && !props.ReadOnly && props.OnImageLeave != nil {
		toolbar = woxwidget.Gesture{ID: fmt.Sprintf("%s.image.%s.chrome", props.ID, block.ID), OnTap: func() {
			props.OnImageLeave(blockIndex, false)
		}, Child: toolbar}
	}
	height := pictureHeight
	if toolbar != nil {
		height += noteEditorImageToolbarGap + noteEditorImageToolbarHeight
		return woxwidget.Flex{Axis: woxwidget.Vertical, Gap: noteEditorImageToolbarGap, Children: []woxwidget.Widget{toolbar, picture}}, height
	}
	return picture, height
}

func noteEditorResolvedImage(props NoteEditorProps, block common.NoteBlock) *woxui.Image {
	if props.ResolveImage == nil || block.Image == nil {
		return nil
	}
	return props.ResolveImage(*block.Image)
}

// noteEditorImageTapHitsPicture reports taps on the centered bitmap, not the full-width chrome.
func noteEditorImageTapHitsPicture(local woxui.Point, rowWidth, drawWidth, drawHeight float32) bool {
	left := max(float32(0), (rowWidth-drawWidth)/2)
	top := noteEditorImageHighlightPad
	return local.X >= left && local.X <= left+drawWidth && local.Y >= top && local.Y <= top+drawHeight
}

func noteEditorImagePicture(props NoteEditorProps, block common.NoteBlock, width float32, focused bool) (woxwidget.Widget, float32) {
	var image *woxui.Image
	if props.ResolveImage != nil && block.Image != nil {
		image = props.ResolveImage(*block.Image)
	}
	drawWidth, drawHeight := noteEditorImageSize(image, block.Image, width, props.Zoom)
	var child woxwidget.Widget
	if image == nil || image.Width <= 0 || image.Height <= 0 {
		child = woxwidget.Container{Width: drawWidth, Height: drawHeight, Color: withAlpha(props.Theme.PreviewText, 10)}
		if block.Image == nil || block.Image.Width <= 0 || block.Image.Height <= 0 {
			child = woxwidget.Container{
				Width: drawWidth, Height: drawHeight, Padding: woxwidget.UniformInsets(10),
				Color: withAlpha(props.Theme.PreviewText, 10),
				Child: woxwidget.TextBlock{
					Value: noteEditorImageMissingLabel(props), Width: max(float32(0), drawWidth-20), Height: 32, MaxLines: 2,
					Style: woxui.TextStyle{Size: 12}, Color: props.Theme.PreviewText,
				},
			}
		}
	} else {
		child = woxwidget.Image{Source: image, Width: drawWidth, Height: drawHeight, Fit: woxwidget.ImageFitContain}
	}
	border := woxui.Color{}
	borderWidth := float32(0)
	if focused {
		border = props.Theme.Cursor
		if border.A == 0 {
			border = props.Theme.PreviewText
		}
		borderWidth = 2
	}
	return woxwidget.Container{
		Width: width, Height: drawHeight + noteEditorImageHighlightPad*2, Padding: woxwidget.UniformInsets(noteEditorImageHighlightPad),
		BorderWidth: borderWidth, BorderColor: border, Radius: 6,
		Child: woxwidget.Align{Width: width - noteEditorImageHighlightPad*2, Height: drawHeight, Horizontal: 0.5, Child: child},
	}, drawHeight + noteEditorImageHighlightPad*2
}

func noteEditorImageLabel(props NoteEditorProps, block common.NoteBlock) string {
	if block.Image != nil {
		if name := strings.TrimSpace(block.Image.FileName); name != "" {
			return name
		}
		if id := strings.TrimSpace(block.Image.ID); id != "" {
			return id
		}
	}
	return noteEditorImageMissingLabel(props)
}

// noteEditorImageMissingLabel is shown only when the attachment cannot be sized or found.
func noteEditorImageMissingLabel(props NoteEditorProps) string {
	if props.MissingImageLabel != "" {
		return props.MissingImageLabel
	}
	return "Image"
}

func noteEditorImageToolbar(props NoteEditorProps, block int, width float32, focused bool) woxwidget.Widget {
	if props.ReadOnly || props.OnImageScale == nil || props.OnImageDelete == nil {
		return nil
	}
	if !focused {
		return woxwidget.Container{Width: width, Height: noteEditorImageToolbarHeight}
	}
	color := props.Theme.ResultSubtitle
	if color.A == 0 {
		color = props.Theme.PreviewText
	}
	id := fmt.Sprintf("%s.image.%s", props.ID, props.Document.Blocks[block].ID)
	button := func(kind, label string, action func()) woxwidget.Widget {
		return WoxIconButton(IconButtonProps{
			ID: id + "." + kind, Label: label, Icon: FormatGlyph(kind, 16, color),
			Width: noteEditorImageToolbarHeight, Height: noteEditorImageToolbarHeight, Radius: 6,
			HoverBackground: TitleBarAlpha(color, 20), FocusRingColor: props.Theme.Cursor, OnTap: action,
			OnHoverAt: func(inside bool, bounds woxui.Rect) {
				if props.OnImageActionHover != nil {
					props.OnImageActionHover(inside, label, bounds)
				}
			},
		})
	}
	labels := props.ImageActionLabels
	return woxwidget.Container{Width: width, Height: noteEditorImageToolbarHeight, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, MainAxisAlignment: woxwidget.MainAxisSpaceBetween, CrossAxisAlignment: woxwidget.CrossAxisCenter,
		Children: []woxwidget.Widget{
			woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 4, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
				button("image-smaller", labels.Smaller, func() { props.OnImageScale(block, -noteEditorImageScaleStep) }),
				button("image-larger", labels.Larger, func() { props.OnImageScale(block, noteEditorImageScaleStep) }),
			}},
			button("image-delete", labels.Delete, func() { props.OnImageDelete(block) }),
		},
	}}
}

func noteEditorImageSize(image *woxui.Image, meta *common.NoteImage, availableWidth, zoom float32) (float32, float32) {
	srcWidth, srcHeight := 0, 0
	if image != nil && image.Width > 0 && image.Height > 0 {
		srcWidth, srcHeight = image.Width, image.Height
	} else if meta != nil && meta.Width > 0 && meta.Height > 0 {
		srcWidth, srcHeight = meta.Width, meta.Height
	}
	scale := float32(1)
	if meta != nil {
		scale = float32(notespluginScale(meta.Scale)) / 100
	}
	width := availableWidth * scale
	if srcWidth <= 0 || srcHeight <= 0 || width <= 0 {
		return max(width, 80), noteEditorImagePlaceholderHeight
	}
	maxHeight := NoteEditorImageMaxHeight * max(zoom, 1)
	ratio := width / float32(srcWidth)
	drawWidth := width
	drawHeight := float32(srcHeight) * ratio
	if drawHeight > maxHeight {
		ratio = maxHeight / float32(srcHeight)
		drawWidth = float32(srcWidth) * ratio
		drawHeight = maxHeight
	}
	return drawWidth, drawHeight
}

func notespluginScale(scale int) int {
	if scale <= 0 {
		return 100
	}
	if scale < 20 {
		return 20
	}
	if scale > 100 {
		return 100
	}
	return scale
}

func noteEditorTableActions(props NoteEditorProps, block int) NoteTableActions {
	if props.ReadOnly || props.OnTableInsertRow == nil || props.OnTableInsertColumn == nil ||
		props.OnTableDeleteRow == nil || props.OnTableDeleteColumn == nil || props.OnTableDelete == nil {
		return NoteTableActions{}
	}
	return NoteTableActions{
		Labels:         props.TableActionLabels,
		OnInsertRow:    func() { props.OnTableInsertRow(block) },
		OnInsertColumn: func() { props.OnTableInsertColumn(block) },
		OnDeleteRow:    func() { props.OnTableDeleteRow(block) },
		OnDeleteColumn: func() { props.OnTableDeleteColumn(block) },
		OnDeleteTable:  func() { props.OnTableDelete(block) },
		OnHover:        props.OnTableActionHover,
	}
}

// noteEditorEdgePadding keeps the document top inset on the first segment only.
// The scroller owns the trailing inset so text-above-table does not inherit Bottom: 24.
func noteEditorEdgePadding(padding woxwidget.Insets, first bool) woxwidget.Insets {
	edge := woxwidget.Insets{Left: padding.Left, Right: padding.Right}
	if first {
		edge.Top = padding.Top
	}
	return edge
}

func noteEditorEmptyBoundaryKey(event woxui.KeyEvent) bool {
	return event.Down && !event.Composing && event.Modifiers == 0 && (event.Key == woxui.KeyBackspace || event.Key == woxui.KeyDelete)
}

func countNoteNewlines(value string) int {
	count := 0
	for _, letter := range value {
		if letter == '\n' {
			count++
		}
	}
	return count
}
