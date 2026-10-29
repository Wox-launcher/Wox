package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"unicode/utf8"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/screen"
)

type notesWindowBounds struct {
	X, Y, Width, Height float32
}

// frame handles window and one-shot focus transitions outside widget composition.
func (c *notesWindowController) frame(host *woxwidget.Host, displayList *woxui.DisplayList, frame woxui.FrameInfo) {
	if c.sidebar.minimumWidth != c.sidebarMinSize().Width {
		c.syncSidebarMinSize()
	}
	c.observeFrame(frame.Size)
	host.Frame(displayList, frame)
	if c.sidebar.menuFocusPending && c.sidebar.dialog == "menu" {
		c.sidebar.menuFocusPending = false
		first := "notes.organize.pin"
		if c.sidebar.target.Group {
			first = "notes.organize.new-note"
		}
		host.RequestFocus(woxwidget.Key(first))
	}
	c.revealEditorCaret = false
	c.findReveal = false
	if !c.markdownView {
		c.requestTextFocus = false
	}
}

// sidebarMinSize reserves the original editor width when the tree is visible.
func (c *notesWindowController) sidebarMinSize() woxui.Size {
	minimum := notesNativeMinSize()
	minimum.Width = woxcomponent.SidebarMinimumWidth(minimum.Width, c.controlTheme().Scaled(256), c.sidebar.open)
	return minimum
}

// syncSidebarMinSize updates native tracking limits and expands an undersized restored window.
func (c *notesWindowController) syncSidebarMinSize() {
	if c.managed == nil {
		return
	}
	minimum := c.sidebarMinSize()
	if c.restoreFrame.Width > 0 && c.restoreFrame.Width < minimum.Width {
		c.restoreFrame.Width = minimum.Width
	}
	// Windows can synchronously render during SetBounds; publish the applied target before resizing.
	previous := c.sidebar.minimumWidth
	c.sidebar.minimumWidth = minimum.Width
	err := syncSidebarWindow(c.managed.Window(), minimum, c.windowMaximized, clampNotesBounds, func(size woxui.Size) { c.requestedSize = size })
	if err != nil {
		c.sidebar.minimumWidth = previous
		c.fail(err)
		return
	}
}

// observeFrame distinguishes an acknowledged programmatic resize from a manual one.
func (c *notesWindowController) observeFrame(size woxui.Size) {
	if c.lastFrame.Width > 0 && (abs32(size.Width-c.lastFrame.Width) > 1 || abs32(size.Height-c.lastFrame.Height) > 1) {
		requested := c.requestedSize.Width > 0 && abs32(size.Width-c.requestedSize.Width) <= 1 && abs32(size.Height-c.requestedSize.Height) <= 1
		if !requested {
			c.manualSize = true
			if c.windowMaximized {
				c.syncMaximizedFromFrame(size)
			}
		}
	}
	c.lastFrame = size
	if c.requestedSize.Width > 0 && abs32(size.Width-c.requestedSize.Width) <= 1 && abs32(size.Height-c.requestedSize.Height) <= 1 {
		c.requestedSize = woxui.Size{}
	}
}

// persistBounds stores logical coordinates and zoom as device-local preferences.
func (c *notesWindowController) persistBounds() {
	c.persistSidebarScroll()
	if c.managed == nil {
		return
	}
	bounds, err := c.managed.Window().Bounds()
	if err != nil {
		return
	}
	saved := notesWindowBounds{X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: bounds.Height}
	if c.windowMaximized && c.restoreFrame.Width > 0 && c.restoreFrame.Height > 0 {
		saved = notesWindowBounds{X: c.restoreFrame.X, Y: c.restoreFrame.Y, Width: c.restoreFrame.Width, Height: c.restoreFrame.Height}
	}
	clamped := clampNotesBounds(woxui.Rect{X: saved.X, Y: saved.Y, Width: saved.Width, Height: saved.Height})
	saved = notesWindowBounds{X: clamped.X, Y: clamped.Y, Width: clamped.Width, Height: clamped.Height}
	encoded, _ := json.Marshal(saved)
	_ = c.owner.services.NotesSetLocal(context.Background(), c.preferenceKey("windowBounds"), string(encoded))
	_ = c.owner.services.NotesSetLocal(context.Background(), c.preferenceKey("zoom"), fmt.Sprintf("%.2f", c.zoom))
	_ = c.owner.services.NotesSetLocal(context.Background(), c.preferenceKey("windowPinned"), map[bool]string{true: "1", false: "0"}[c.windowPinned])
	_ = c.owner.services.NotesSetLocal(context.Background(), c.preferenceKey("windowMaximized"), map[bool]string{true: "1", false: "0"}[c.windowMaximized])
}

// restoreBounds clamps saved logical geometry into a currently connected work area.
func (c *notesWindowController) restoreBounds(window *woxui.Window) bool {
	raw := c.localPreference("windowBounds")
	var saved notesWindowBounds
	if zoom, err := strconv.ParseFloat(c.localPreference("zoom"), 32); err == nil {
		c.zoom = max(float32(.75), min(float32(2), float32(zoom)))
	}
	if raw == "" || json.Unmarshal([]byte(raw), &saved) != nil || saved.Width <= 0 || saved.Height <= 0 {
		return false
	}
	c.manualSize = true
	bounds := clampNotesBounds(woxui.Rect{X: saved.X, Y: saved.Y, Width: saved.Width, Height: saved.Height})
	bounds.Width = max(bounds.Width, c.sidebarMinSize().Width)
	c.restoreFrame = bounds
	return window.SetBounds(bounds) == nil
}

func clampNotesBounds(bounds woxui.Rect) woxui.Rect {
	bounds.Width = max(notesMinimumWidth, bounds.Width)
	bounds.Height = max(notesMinimumHeight, min(notesMaximumHeight, bounds.Height))
	displays, err := screen.ListDisplays()
	if err != nil || len(displays) == 0 {
		return bounds
	}
	return clampNotesBoundsToDisplays(bounds, displays)
}

// clampNotesBoundsToDisplays chooses the largest logical overlap, including negative origins.
func clampNotesBoundsToDisplays(bounds woxui.Rect, displays []screen.Display) woxui.Rect {
	bounds.Width = max(notesMinimumWidth, bounds.Width)
	bounds.Height = max(notesMinimumHeight, min(notesMaximumHeight, bounds.Height))
	best := displays[0].WorkArea
	bestArea := -1
	for _, display := range displays {
		work := display.WorkArea
		left, top := max(int(bounds.X), work.X), max(int(bounds.Y), work.Y)
		right, bottom := min(int(bounds.X+bounds.Width), work.Right()), min(int(bounds.Y+bounds.Height), work.Bottom())
		area := max(0, right-left) * max(0, bottom-top)
		if area > bestArea || (area == bestArea && display.Primary) {
			best, bestArea = work, area
		}
	}
	bounds.Width = min(bounds.Width, float32(best.Width))
	bounds.Height = min(bounds.Height, float32(best.Height))
	bounds.X = max(float32(best.X), min(bounds.X, float32(best.Right())-bounds.Width))
	bounds.Y = max(float32(best.Y), min(bounds.Y, float32(best.Bottom())-bounds.Height))
	return bounds
}

// resizeForDocument auto-fits content until the user manually resizes this window lifetime.
func (c *notesWindowController) resizeForDocument() {
	if c.manualSize || c.windowMaximized || c.managed == nil || len(c.document.Blocks) == 0 {
		return
	}
	lineCount := 0
	for _, block := range c.document.Blocks {
		lineCount += max(1, (utf8.RuneCountInString(block.Text)+35)/36)
	}
	height := max(notesMinimumHeight, min(notesMaximumHeight, 98+float32(lineCount)*notesEditorLineHeight))
	bounds, err := c.managed.Window().Bounds()
	if err != nil {
		return
	}
	if bounds.Width <= 0 {
		bounds.Width = notesDefaultWidth
	}
	bounds.Height = height
	bounds = clampNotesBounds(bounds)
	c.requestedSize = woxui.Size{Width: bounds.Width, Height: bounds.Height}
	_ = c.managed.Window().SetBounds(bounds)
}

func abs32(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}

func (c *notesWindowController) local(key string) (string, error) {
	return c.owner.services.NotesGetLocal(context.Background(), key)
}

func (c *notesWindowController) preferenceKey(key string) string {
	return key + ":" + c.record.ID
}

// localPreference reads per-note state and migrates the former singleton preference for the last active note.
func (c *notesWindowController) localPreference(key string) string {
	if value, _ := c.local(c.preferenceKey(key)); value != "" {
		return value
	}
	currentNoteID, _ := c.local("currentNoteId")
	if currentNoteID == c.record.ID {
		value, _ := c.local(key)
		return value
	}
	return ""
}

// readWindowPinned loads the per-note always-on-top preference. New notes default to pinned.
func (c *notesWindowController) readWindowPinned() bool {
	if c == nil || c.owner == nil || c.owner.services == nil || c.record.ID == "" {
		return false
	}
	value := c.localPreference("windowPinned")
	return value == "" || value == "1"
}

// applyWindowTopmost mirrors the persisted pin onto the live native window.
// On macOS, pin also joins every Space; unpin returns the note to the current
// Space and normal app stacking.
func (c *notesWindowController) applyWindowTopmost() {
	if c.managed == nil {
		return
	}
	_ = c.managed.Window().SetTopmost(c.windowPinned)
}

// readWindowMaximized loads the per-note maximized preference. Missing values stay restored.
func (c *notesWindowController) readWindowMaximized() bool {
	if c == nil || c.owner == nil || c.owner.services == nil || c.record.ID == "" {
		return false
	}
	return c.localPreference("windowMaximized") == "1"
}

// applyRestoredMaximize expands a just-restored note onto its work area when that state was persisted.
func (c *notesWindowController) applyRestoredMaximize(window *woxui.Window) {
	if !c.windowMaximized || window == nil {
		return
	}
	c.maximizeWindowOn(window)
}

// minimizeWindow sends this note to the taskbar or dock.
func (c *notesWindowController) minimizeWindow() {
	if c == nil || c.managed == nil {
		return
	}
	_ = c.managed.Window().Minimize()
}

// toggleMaximize switches between the last restored frame and the current display work area.
func (c *notesWindowController) toggleMaximize() {
	if c.windowMaximized {
		c.restoreFromMaximize()
		return
	}
	c.maximizeWindow()
}

func (c *notesWindowController) maximizeWindow() {
	if c.managed == nil {
		return
	}
	c.maximizeWindowOn(c.managed.Window())
}

func (c *notesWindowController) maximizeWindowOn(window *woxui.Window) {
	if window == nil {
		return
	}
	bounds, err := window.Bounds()
	if err != nil {
		return
	}
	if !c.windowMaximized || c.restoreFrame.Width <= 0 || c.restoreFrame.Height <= 0 {
		c.restoreFrame = bounds
	}
	target := notesMaximizeBounds(bounds)
	c.requestedSize = woxui.Size{Width: target.Width, Height: target.Height}
	if err := window.SetBounds(target); err != nil {
		return
	}
	c.windowMaximized = true
	c.manualSize = true
	c.persistMaximizePreference()
	c.persistBounds()
	c.invalidate()
}

func (c *notesWindowController) restoreFromMaximize() {
	if c.managed == nil {
		return
	}
	target := c.restoreFrame
	if target.Width <= 0 || target.Height <= 0 {
		if bounds, err := c.managed.Window().Bounds(); err == nil {
			target = woxui.Rect{X: bounds.X, Y: bounds.Y, Width: notesDefaultWidth, Height: notesDefaultHeight}
		} else {
			target = woxui.Rect{Width: notesDefaultWidth, Height: notesDefaultHeight}
		}
	}
	target = clampNotesBounds(target)
	target.Width = max(target.Width, c.sidebarMinSize().Width)
	c.requestedSize = woxui.Size{Width: target.Width, Height: target.Height}
	if err := c.managed.Window().SetBounds(target); err != nil {
		return
	}
	c.windowMaximized = false
	c.manualSize = true
	c.persistMaximizePreference()
	c.persistBounds()
	c.invalidate()
}

// syncMaximizedFromFrame clears maximize after an interactive resize leaves the work area.
func (c *notesWindowController) syncMaximizedFromFrame(size woxui.Size) {
	if !c.windowMaximized {
		return
	}
	var current woxui.Rect
	if c.managed != nil {
		if bounds, err := c.managed.Window().Bounds(); err == nil {
			current = bounds
		}
	}
	if current.Width <= 0 || current.Height <= 0 {
		current.Width, current.Height = size.Width, size.Height
	}
	target := notesMaximizeBounds(current)
	if abs32(size.Width-target.Width) <= 4 && abs32(size.Height-target.Height) <= 4 {
		return
	}
	c.windowMaximized = false
	c.restoreFrame = current
	c.persistMaximizePreference()
}

func (c *notesWindowController) persistMaximizePreference() {
	if c.owner == nil || c.owner.services == nil || c.record.ID == "" {
		return
	}
	_ = c.owner.services.NotesSetLocal(context.Background(), c.preferenceKey("windowMaximized"), map[bool]string{true: "1", false: "0"}[c.windowMaximized])
}

// notesMaximizeBounds fills the work area that contains most of the current frame.
func notesMaximizeBounds(current woxui.Rect) woxui.Rect {
	displays, err := screen.ListDisplays()
	if err != nil || len(displays) == 0 {
		return current
	}
	return notesMaximizeBoundsToDisplays(current, displays)
}

func notesMaximizeBoundsToDisplays(current woxui.Rect, displays []screen.Display) woxui.Rect {
	if len(displays) == 0 {
		return current
	}
	best := displays[0].WorkArea
	bestArea := -1
	for _, display := range displays {
		work := display.WorkArea
		left, top := max(int(current.X), work.X), max(int(current.Y), work.Y)
		right, bottom := min(int(current.X+current.Width), work.Right()), min(int(current.Y+current.Height), work.Bottom())
		area := max(0, right-left) * max(0, bottom-top)
		if area > bestArea || (area == bestArea && display.Primary) {
			best, bestArea = work, area
		}
	}
	return woxui.Rect{X: float32(best.X), Y: float32(best.Y), Width: float32(best.Width), Height: float32(best.Height)}
}

// toggleWindowPin keeps this note above other applications and on every Space.
func (c *notesWindowController) toggleWindowPin() {
	c.windowPinned = !c.windowPinned
	if c.owner != nil && c.owner.services != nil && c.record.ID != "" {
		_ = c.owner.services.NotesSetLocal(context.Background(), c.preferenceKey("windowPinned"), map[bool]string{true: "1", false: "0"}[c.windowPinned])
	}
	c.applyWindowTopmost()
	c.invalidate()
}
