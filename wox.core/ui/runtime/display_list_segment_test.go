package woxui

import "testing"

func TestPaintSegmentCompareFlattensTranslatedCommands(t *testing.T) {
	local := &DisplayList{}
	local.FillRect(Rect{Width: 10, Height: 10}, Color{R: 255, A: 255})
	segment := CapturePaintSegment(Rect{Width: 10, Height: 10}, local, PaintFingerprint{})

	retained := &DisplayList{}
	retained.AppendPaintSegment(segment, Point{X: 5, Y: 7})
	flat := &DisplayList{}
	flat.FillRect(Rect{X: 5, Y: 7, Width: 10, Height: 10}, Color{R: 255, A: 255})
	if err := retained.Compare(flat); err != nil {
		t.Fatalf("flattened segment compare: %v", err)
	}
	if retained.CommandCount() != 1 {
		t.Fatalf("expanded command count = %d, want 1", retained.CommandCount())
	}
}

func TestForEachVisibleCommandSkipsNonIntersectingSegments(t *testing.T) {
	local := &DisplayList{}
	local.FillRect(Rect{Width: 10, Height: 10}, Color{A: 255})
	segment := CapturePaintSegment(Rect{Width: 10, Height: 10}, local, PaintFingerprint{})
	displayList := &DisplayList{}
	displayList.AppendPaintSegment(segment, Point{X: 0, Y: 0})
	displayList.AppendPaintSegment(segment, Point{X: 50, Y: 50})

	var count int
	displayList.ForEachVisibleCommand(Rect{X: 48, Y: 48, Width: 10, Height: 10}, func(displayCommand) bool {
		count++
		return true
	})
	if count != 1 {
		t.Fatalf("visible command count = %d, want 1 intersecting segment", count)
	}
}

func TestPaintSegmentClipRestoresInheritedClip(t *testing.T) {
	local := &DisplayList{}
	local.PushClipRect(Rect{Width: 30, Height: 30})
	local.FillRect(Rect{Width: 30, Height: 30}, Color{A: 255})
	local.PopClipRect()
	segment := CapturePaintSegment(Rect{Width: 30, Height: 30}, local, PaintFingerprint{})

	displayList := &DisplayList{}
	displayList.PushClipRect(Rect{Width: 10, Height: 10})
	displayList.AppendPaintSegment(segment, Point{X: 5, Y: 5})
	displayList.FillRect(Rect{Width: 10, Height: 10}, Color{A: 255})
	displayList.PopClipRect()

	var commands []displayCommand
	displayList.ForEachCommand(func(command displayCommand) bool {
		commands = append(commands, command)
		return true
	})
	if len(commands) != 6 {
		t.Fatalf("flattened command count = %d, want 6", len(commands))
	}
	if commands[1].kind != displayCommandSetClipRect || commands[1].rect != (Rect{X: 5, Y: 5, Width: 5, Height: 5}) {
		t.Fatalf("segment clip = %+v, want intersection with inherited clip", commands[1])
	}
	if commands[3].kind != displayCommandSetClipRect || commands[3].rect != (Rect{Width: 10, Height: 10}) {
		t.Fatalf("segment clip restore = %+v, want inherited clip", commands[3])
	}
}

func TestCapturePaintSegmentPublishesANewImmutableVersion(t *testing.T) {
	red := &DisplayList{}
	red.FillRect(Rect{Width: 10, Height: 10}, Color{R: 255, A: 255})
	published := CapturePaintSegment(Rect{Width: 10, Height: 10}, red, PaintFingerprint{})

	queued := &DisplayList{}
	queued.AppendPaintSegment(published, Point{})
	queued.Freeze()

	blue := &DisplayList{}
	blue.FillRect(Rect{Width: 10, Height: 10}, Color{B: 255, A: 255})
	next := CapturePaintSegment(Rect{Width: 10, Height: 10}, blue, PaintFingerprint{})
	if next == published {
		t.Fatal("CapturePaintSegment reused a published segment pointer")
	}

	var color Color
	queued.ForEachCommand(func(command displayCommand) bool {
		color = command.color
		return true
	})
	if color.R != 255 || color.B != 0 {
		t.Fatalf("queued command color = %#v, want the published red fill", color)
	}
}

func TestForEachVisibleCommandKeepsOverlayStateOutsideDamage(t *testing.T) {
	overlay := &DisplayList{}
	overlay.BeginEmbeddedSurfaceOverlay(Rect{Width: 40, Height: 40})
	overlaySegment := CapturePaintSegment(Rect{Width: 40, Height: 40}, overlay, PaintFingerprint{})

	control := &DisplayList{}
	control.FillRect(Rect{Width: 10, Height: 10}, Color{A: 255})
	controlSegment := CapturePaintSegment(Rect{Width: 10, Height: 10}, control, PaintFingerprint{})

	displayList := &DisplayList{}
	displayList.AppendPaintSegment(overlaySegment, Point{X: 0, Y: 0})
	displayList.AppendPaintSegment(controlSegment, Point{X: 80, Y: 80})

	var kinds []displayCommandKind
	displayList.ForEachVisibleCommand(Rect{X: 80, Y: 80, Width: 10, Height: 10}, func(command displayCommand) bool {
		kinds = append(kinds, command.kind)
		return true
	})
	if len(kinds) != 2 || kinds[0] != displayCommandBeginEmbeddedSurfaceOverlay || kinds[1] != displayCommandFillRoundedRect {
		t.Fatalf("overlay-visible commands = %v, want overlay begin then the damaged control", kinds)
	}
}

func TestRebasePaintSegmentRecomputesOverlayAndCounts(t *testing.T) {
	plain := &DisplayList{}
	plain.FillRect(Rect{Width: 10, Height: 10}, Color{A: 255})
	plain.DrawText("plain", Rect{Width: 10, Height: 10}, TextStyle{Size: 12}, Color{A: 255})
	plainChild := CapturePaintSegment(Rect{Width: 10, Height: 10}, plain, PaintFingerprint{})

	overlay := &DisplayList{}
	overlay.BeginEmbeddedSurfaceOverlay(Rect{Width: 10, Height: 10})
	overlay.DrawImage(&Image{Width: 1, Height: 1, pixels: []byte{1, 2, 3, 4}}, Rect{Width: 10, Height: 10})
	overlayChild := CapturePaintSegment(Rect{Width: 10, Height: 10}, overlay, PaintFingerprint{})

	parentLocal := &DisplayList{}
	parentLocal.FillRect(Rect{Width: 20, Height: 20}, Color{R: 255, A: 255})
	parentLocal.AppendPaintSegment(plainChild, Point{})
	parent := CapturePaintSegment(Rect{Width: 20, Height: 20}, parentLocal, PaintFingerprint{})
	if parent.HasEmbeddedSurfaceOverlay || parent.CommandCount != 3 || parent.TextDrawCount != 1 || parent.ImageDrawCount != 0 {
		t.Fatalf("plain parent aggregate = overlay %t commands %d text %d image %d", parent.HasEmbeddedSurfaceOverlay, parent.CommandCount, parent.TextDrawCount, parent.ImageDrawCount)
	}

	withOverlay := RebasePaintSegment(parent, map[*PaintSegment]*PaintSegment{plainChild: overlayChild})
	if withOverlay == parent || !withOverlay.HasEmbeddedSurfaceOverlay || withOverlay.CommandCount != 3 || withOverlay.TextDrawCount != 0 || withOverlay.ImageDrawCount != 1 {
		t.Fatalf("false→true rebase = overlay %t commands %d text %d image %d", withOverlay.HasEmbeddedSurfaceOverlay, withOverlay.CommandCount, withOverlay.TextDrawCount, withOverlay.ImageDrawCount)
	}

	restored := RebasePaintSegment(withOverlay, map[*PaintSegment]*PaintSegment{overlayChild: plainChild})
	if restored.HasEmbeddedSurfaceOverlay || restored.CommandCount != 3 || restored.TextDrawCount != 1 || restored.ImageDrawCount != 0 {
		t.Fatalf("true→false rebase = overlay %t commands %d text %d image %d", restored.HasEmbeddedSurfaceOverlay, restored.CommandCount, restored.TextDrawCount, restored.ImageDrawCount)
	}

	queued := &DisplayList{}
	queued.AppendPaintSegment(withOverlay, Point{})
	if queued.CommandCount() != 3 || queued.TextDrawCount() != 0 || queued.ImageDrawCount() != 1 {
		t.Fatalf("appended overlay aggregate = commands %d text %d image %d", queued.CommandCount(), queued.TextDrawCount(), queued.ImageDrawCount())
	}
	var sawOverlay bool
	queued.ForEachVisibleCommand(Rect{X: 80, Y: 80, Width: 10, Height: 10}, func(command displayCommand) bool {
		if command.kind == displayCommandBeginEmbeddedSurfaceOverlay {
			sawOverlay = true
		}
		return true
	})
	if !sawOverlay {
		t.Fatal("rebased overlay parent still skipped BeginEmbeddedSurfaceOverlay during damage culling")
	}
}

func TestPaintFingerprintIgnoresUnusedFocusAndCaret(t *testing.T) {
	fingerprint := PaintFingerprint{}
	if !fingerprint.Matches(1, 2, true) || !fingerprint.Matches(9, 8, false) {
		t.Fatal("unused fingerprint should match any focus or caret phase")
	}
	fingerprint.UsesCaret = true
	fingerprint.CaretVisible = true
	if fingerprint.Matches(1, 2, false) {
		t.Fatal("caret fingerprint matched a hidden caret")
	}
}
