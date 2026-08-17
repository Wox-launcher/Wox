package woxui

// PaintFingerprint records the dynamic draw inputs a retained segment depends on.
type PaintFingerprint struct {
	UsesFocus     bool
	UsesFocusRing bool
	UsesCaret     bool
	Focused       AccessibilityNodeID
	FocusRing     AccessibilityNodeID
	CaretVisible  bool
}

// Matches reports whether this segment can be replayed for the current focus/caret frame.
func (f PaintFingerprint) Matches(focused, focusRing AccessibilityNodeID, caretVisible bool) bool {
	if f.UsesFocus && f.Focused != focused {
		return false
	}
	if f.UsesFocusRing && f.FocusRing != focusRing {
		return false
	}
	if f.UsesCaret && f.CaretVisible != caretVisible {
		return false
	}
	return true
}

// PaintSegment is an immutable local-coordinate command batch owned by one Boundary.
// Updates allocate a new version so a queued display list can keep the previous pointer.
type PaintSegment struct {
	Bounds                    Rect
	Commands                  []displayCommand
	CommandCount              int
	TextDrawCount             int
	ImageDrawCount            int
	Fingerprint               PaintFingerprint
	HasEmbeddedSurfaceOverlay bool
}

// paintCommandStats is the expanded-command aggregate cached on lists and segments.
type paintCommandStats struct {
	commands int
	texts    int
	images   int
	overlay  bool
}

func (s *paintCommandStats) add(command displayCommand) {
	if command.kind == displayCommandPaintSegment && command.segment != nil {
		s.commands += command.segment.CommandCount
		s.texts += command.segment.TextDrawCount
		s.images += command.segment.ImageDrawCount
		s.overlay = s.overlay || command.segment.HasEmbeddedSurfaceOverlay
		return
	}
	s.commands++
	switch command.kind {
	case displayCommandDrawText:
		s.texts++
	case displayCommandDrawImage:
		s.images++
	case displayCommandBeginEmbeddedSurfaceOverlay:
		s.overlay = true
	}
}

func statsFromCommands(commands []displayCommand) paintCommandStats {
	var stats paintCommandStats
	for _, command := range commands {
		stats.add(command)
	}
	return stats
}

func (s paintCommandStats) applyTo(segment *PaintSegment) {
	segment.CommandCount = s.commands
	segment.TextDrawCount = s.texts
	segment.ImageDrawCount = s.images
	segment.HasEmbeddedSurfaceOverlay = s.overlay
}

// CapturePaintSegment freezes the current local command stream into one reusable segment.
func CapturePaintSegment(bounds Rect, list *DisplayList, fingerprint PaintFingerprint) *PaintSegment {
	segment := &PaintSegment{Bounds: bounds, Fingerprint: fingerprint}
	if list != nil {
		segment.Commands = append([]displayCommand(nil), list.commands...)
		list.stats.applyTo(segment)
	}
	return segment
}

// RebasePaintSegment returns a new version with replaced nested segment pointers.
// Aggregates are rebuilt from direct commands plus each child's cached stats.
func RebasePaintSegment(segment *PaintSegment, replacements map[*PaintSegment]*PaintSegment) *PaintSegment {
	if segment == nil || len(replacements) == 0 {
		return segment
	}
	commands := append([]displayCommand(nil), segment.Commands...)
	replaced := false
	for index := range commands {
		if commands[index].segment == nil {
			continue
		}
		next, ok := replacements[commands[index].segment]
		if !ok || next == nil || next == commands[index].segment {
			continue
		}
		commands[index].segment = next
		replaced = true
	}
	if !replaced {
		return segment
	}
	out := *segment
	out.Commands = commands
	statsFromCommands(commands).applyTo(&out)
	return &out
}

// Freeze snapshots the top-level command slice so later appends cannot change a
// queued list. Nested PaintSegments are immutable and shared by pointer.
func (d *DisplayList) Freeze() {
	if d == nil || d.frozen {
		return
	}
	d.commands = append([]displayCommand(nil), d.commands...)
	d.frozen = true
}

func expandedCommandCount(commands []displayCommand) int {
	return statsFromCommands(commands).commands
}

func (d *DisplayList) flattenedCommands() []displayCommand {
	if d == nil {
		return nil
	}
	out := make([]displayCommand, 0, d.CommandCount())
	d.ForEachCommand(func(command displayCommand) bool {
		out = append(out, command)
		return true
	})
	return out
}

// AppendPaintSegment records one retained subtree for later origin-aware encoding.
func (d *DisplayList) AppendPaintSegment(segment *PaintSegment, origin Point) {
	if d == nil || segment == nil {
		return
	}
	bounds := Rect{X: origin.X, Y: origin.Y, Width: segment.Bounds.Width, Height: segment.Bounds.Height}
	if !d.shouldRecord(bounds) {
		return
	}
	d.appendCommand(displayCommand{
		kind:    displayCommandPaintSegment,
		rect:    bounds,
		segment: segment,
	})
}

// ForEachCommand visits expanded drawing commands in window space.
func (d *DisplayList) ForEachCommand(visit func(displayCommand) bool) {
	if d == nil || visit == nil {
		return
	}
	walkDisplayCommands(d.commands, Point{}, Rect{}, Rect{}, false, visit)
}

// ForEachVisibleCommand skips retained segments that do not intersect encode damage.
// Overlay state commands retarget every later draw, so a list that contains one
// is walked in full; culling the BeginEmbeddedSurfaceOverlay segment would paint
// later overlay controls behind the platform surface.
func (d *DisplayList) ForEachVisibleCommand(damage Rect, visit func(displayCommand) bool) {
	if d == nil || visit == nil {
		return
	}
	if d.stats.overlay {
		walkDisplayCommands(d.commands, Point{}, Rect{}, Rect{}, false, visit)
		return
	}
	walkDisplayCommands(d.commands, Point{}, damage, Rect{}, false, visit)
}

func walkDisplayCommands(commands []displayCommand, origin Point, damage, inheritedClip Rect, hasInheritedClip bool, visit func(displayCommand) bool) bool {
	// Paint segments record clip stacks independently, so scope their clip commands
	// to the clip active at insertion instead of letting a child clear an ancestor viewport.
	activeClip := inheritedClip
	hasActiveClip := hasInheritedClip
	for _, command := range commands {
		if command.kind == displayCommandPaintSegment && command.segment != nil {
			childOrigin := Point{X: origin.X + command.rect.X, Y: origin.Y + command.rect.Y}
			bounds := Rect{X: childOrigin.X, Y: childOrigin.Y, Width: command.rect.Width, Height: command.rect.Height}
			if damage.Width > 0 && damage.Height > 0 && !rectsOverlap(bounds, damage) {
				continue
			}
			if !walkDisplayCommands(command.segment.Commands, childOrigin, damage, activeClip, hasActiveClip, visit) {
				return false
			}
			continue
		}
		command = translateDisplayCommand(command, origin)
		switch command.kind {
		case displayCommandSetClipRect:
			if hasInheritedClip {
				command.rect = intersectRects(inheritedClip, command.rect)
			}
			activeClip = command.rect
			hasActiveClip = true
		case displayCommandClearClip:
			if hasInheritedClip {
				command.kind = displayCommandSetClipRect
				command.rect = inheritedClip
				activeClip = inheritedClip
				hasActiveClip = true
			} else {
				activeClip = Rect{}
				hasActiveClip = false
			}
		}
		if !visit(command) {
			return false
		}
	}
	return true
}

func translateDisplayCommand(command displayCommand, origin Point) displayCommand {
	if origin.X == 0 && origin.Y == 0 {
		return command
	}
	command.rect.X += origin.X
	command.rect.Y += origin.Y
	if len(command.points) > 0 {
		points := make([]Point, len(command.points))
		for index, point := range command.points {
			points[index] = Point{X: point.X + origin.X, Y: point.Y + origin.Y}
		}
		command.points = points
	}
	return command
}
