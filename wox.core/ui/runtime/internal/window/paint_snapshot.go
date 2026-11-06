package window

import (
	"slices"
	"wox/ui/runtime/internal/graphics"
)

// PaintSnapshot retains immutable drawing data, never widgets or their callbacks.
// Coordinates remain logical and relative to the enclosing snapshot scope. Images
// are immutable references; carets retain their captured phase without blinking.
type PaintSnapshot struct {
	commands  []displayCommand
	groups    []snapshotGroup
	clip      *displayClip
	region    *displayClip
	normalize AffineTransform
	bounds    Rect
}

// SnapshotCapture is an opaque, single-use marker paired with EndSnapshot.
type SnapshotCapture struct {
	list     *DisplayList
	start    int
	snapshot *PaintSnapshot
}

type snapshotScope struct {
	origin     Point
	normalize  AffineTransform
	groupDepth int
	valid      bool
}

// snapshotGroup retains the clip at group entry; a child's current clip alone
// cannot reconstruct the ancestor's opacity or directional fade silhouette.
type snapshotGroup struct {
	command  displayCommand
	clip     *displayClip
	identity *byte
}

func (s *PaintSnapshot) Bounds() Rect {
	if s == nil {
		return Rect{}
	}
	return s.bounds
}

// RetainedBytes is a conservative storage estimate for caller-owned budgets.
// Repeated image references within one snapshot count once.
func (s *PaintSnapshot) RetainedBytes() int {
	if s == nil {
		return 0
	}
	bytes := 256 * (cap(s.commands) + cap(s.groups) + 1)
	images := map[*Image]bool{}
	for _, command := range s.commands {
		bytes += len(command.text) + 8*cap(command.points)
		if command.image != nil && !images[command.image] {
			images[command.image] = true
			bytes += command.image.PixelBytes()
		}
	}
	for _, group := range s.groups {
		if group.clip != nil {
			bytes += 8 * cap(group.clip.points)
		}
	}
	if s.clip != nil {
		bytes += 8 * cap(s.clip.points)
	}
	if s.region != nil {
		bytes += 8 * cap(s.region.points)
	}
	return bytes
}

// BeginSnapshotScope fixes a local coordinate origin and the enclosing group
// depth. Parent groups remain live at replay, so snapshots must not copy them.
// Pair with EndSnapshotScope, including when painting an empty scope.
func (d *DisplayList) BeginSnapshotScope(origin Point) {
	inverse, valid := d.paintTransform().Inverse()
	d.snapshotScopes = append(d.snapshotScopes, snapshotScope{origin: origin,
		normalize:  inverse.Then(TranslationTransform(-origin.X, -origin.Y)),
		groupDepth: len(d.groups), valid: valid})
}

func (d *DisplayList) EndSnapshotScope() {
	if len(d.snapshotScopes) > 0 {
		d.snapshotScopes = d.snapshotScopes[:len(d.snapshotScopes)-1]
	}
}

// BeginSnapshot captures a balanced subtree inside the innermost scope. Damage
// culling is suspended until EndSnapshot so a partial frame still yields complete
// retained paint. The renderer's native damage scissor remains unchanged.
func (d *DisplayList) BeginSnapshot(bounds Rect) *SnapshotCapture {
	if len(d.snapshotScopes) == 0 {
		return nil
	}
	scope := d.snapshotScopes[len(d.snapshotScopes)-1]
	if !scope.valid || scope.groupDepth > len(d.groups) {
		return nil
	}
	d.snapshotDepth++
	return &SnapshotCapture{list: d, start: len(d.commands), snapshot: &PaintSnapshot{
		normalize: scope.normalize, bounds: d.paintTransform().Then(scope.normalize).Bounds(bounds),
		clip: d.snapshotClip(), groups: slices.Clone(d.groups[scope.groupDepth:]),
	}}
}

// BeginSnapshotRegion observes shared paint through a frozen region without
// changing the live clip. Several regions can observe the same drawing command.
func (d *DisplayList) BeginSnapshotRegion(bounds Rect) *SnapshotCapture {
	capture := d.BeginSnapshot(bounds)
	if capture != nil {
		region := d.transformedClip(bounds)
		capture.snapshot.region = &region
	}
	return capture
}

// EndSnapshot detaches commands from the frame, freezes carets, and excludes
// native embedded surfaces, which cannot be replayed from portable paint data.
func (d *DisplayList) EndSnapshot(capture *SnapshotCapture) *PaintSnapshot {
	if capture == nil || capture.list != d {
		return nil
	}
	capture.list = nil
	d.snapshotDepth--
	snapshot := capture.snapshot
	for _, command := range d.commands[capture.start:] {
		if command.kind == displayCommandBeginEmbeddedSurfaceOverlay {
			return nil
		}
		if command.caret && !command.caretVisible {
			continue
		}
		command.caret, command.caretVisible = false, false
		command.points = slices.Clone(command.points)
		snapshot.commands = append(snapshot.commands, command)
	}
	if len(snapshot.commands) == 0 {
		return nil
	}
	return snapshot
}

// DrawSnapshot replays in the innermost scope, through the current paint map and
// clip. It restores native clipping afterward without mutating portable stacks.
func (d *DisplayList) DrawSnapshot(snapshot *PaintSnapshot) {
	d.DrawSnapshots([]*PaintSnapshot{snapshot})
}

// DrawSnapshots composites ordered fragments under their shared captured groups
// once. Group identity distinguishes separate siblings with identical opacity.
func (d *DisplayList) DrawSnapshots(snapshots []*PaintSnapshot) {
	if len(d.snapshotScopes) == 0 {
		return
	}
	scope := d.snapshotScopes[len(d.snapshotScopes)-1]
	if !scope.valid {
		return
	}
	restore := d.snapshotClip()
	clip := func(original *displayClip, transform AffineTransform, parent *displayClip) {
		if original == nil {
			d.restoreSnapshotClip(parent)
			return
		}
		mapped := mapSnapshotClip(*original, transform)
		if parent != nil {
			mapped = intersectSnapshotClips(mapped, *parent)
		}
		d.appendClip(mapped)
	}
	var active []snapshotGroup
	var activeTransform AffineTransform
	closeGroups := func(keep int) {
		for i := len(active) - 1; i >= keep; i-- {
			group := active[i]
			if group.command.kind == displayCommandBeginOpacity {
				group.command.kind = displayCommandEndOpacity
			} else {
				group.command.kind = displayCommandEndEdgeFade
			}
			d.replaySnapshotCommand(group.command, activeTransform)
			clip(group.clip, activeTransform, restore)
		}
		active = active[:keep]
	}
	for _, snapshot := range snapshots {
		if snapshot == nil {
			continue
		}
		transform := snapshot.normalize.Then(TranslationTransform(scope.origin.X, scope.origin.Y)).Then(d.paintTransform())
		shared := 0
		if transform == activeTransform {
			for shared < len(active) && shared < len(snapshot.groups) && active[shared].identity == snapshot.groups[shared].identity {
				shared++
			}
		}
		closeGroups(shared)
		for _, group := range snapshot.groups[shared:] {
			clip(group.clip, transform, restore)
			d.replaySnapshotCommand(group.command, transform)
			clip(group.clip, transform, restore)
		}
		active, activeTransform = snapshot.groups, transform
		parent := restore
		if snapshot.region != nil {
			region := mapSnapshotClip(*snapshot.region, transform)
			if parent != nil {
				region = intersectSnapshotClips(region, *parent)
			}
			parent = &region
		}
		clip(snapshot.clip, transform, parent)
		for _, command := range snapshot.commands {
			switch command.kind {
			case displayCommandSetClipRect:
				clip(&displayClip{rect: command.rect, points: command.points}, transform, parent)
			case displayCommandClearClip:
				clip(nil, transform, parent)
			default:
				d.replaySnapshotCommand(command, transform)
			}
		}
		// Region masks belong to a fragment, not to the shared ancestor group.
		d.restoreSnapshotClip(restore)
	}
	closeGroups(0)
	d.restoreSnapshotClip(restore)
}

// replaySnapshotCommand composes onto recorded local geometry instead of scaling
// fonts, strokes or images twice. Material metadata follows the target envelope.
func (d *DisplayList) replaySnapshotCommand(command displayCommand, transform AffineTransform) {
	if command.transformed {
		transform = command.transform.Then(transform)
	}
	command.transform, command.transformed = transform, transform != IdentityTransform()
	d.appendRecordedCommand(command)
	if command.kind == displayCommandFloatingMaterial {
		d.floatingMaterials = append(d.floatingMaterials, transform.Bounds(command.rect))
		d.materialMargin = max(d.materialMargin, 3*command.material.Sigma)
	}
}

func (d *DisplayList) snapshotClip() *displayClip {
	if len(d.clipStack) == 0 {
		return nil
	}
	clip := d.clipStack[len(d.clipStack)-1]
	return &clip
}

func (d *DisplayList) restoreSnapshotClip(clip *displayClip) {
	if clip == nil {
		d.appendCommand(displayCommand{kind: displayCommandClearClip})
	} else {
		d.appendClip(*clip)
	}
}

// mapSnapshotClip preserves exact polygons when either the captured clip or the
// replay map is rotated, including singular maps that have an empty silhouette.
func mapSnapshotClip(clip displayClip, transform AffineTransform) displayClip {
	if _, valid := transform.Inverse(); !valid {
		return displayClip{}
	}
	if clip.points == nil && transform.B == 0 && transform.C == 0 {
		return displayClip{rect: transform.Bounds(clip.rect)}
	}
	points := clip.points
	if points == nil {
		points = IdentityTransform().RectPolygon(clip.rect)
	}
	mapped := make([]Point, len(points))
	for i, point := range points {
		mapped[i] = transform.Point(point)
	}
	return displayClip{rect: graphics.PolygonBounds(mapped), points: mapped}
}

// intersectSnapshotClips combines frozen clip geometry with the replay parent's
// current clip; replay must never clear or expand that parent's silhouette.
func intersectSnapshotClips(a, b displayClip) displayClip {
	if a.points == nil && b.points == nil {
		return displayClip{rect: intersectRects(a.rect, b.rect)}
	}
	pa, pb := a.points, b.points
	if pa == nil {
		pa = IdentityTransform().RectPolygon(a.rect)
	}
	if pb == nil {
		pb = IdentityTransform().RectPolygon(b.rect)
	}
	points := graphics.ClipConvexPolygon(pa, pb)
	return displayClip{rect: graphics.PolygonBounds(points), points: points}
}
