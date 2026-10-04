package screenshot

import "testing"

// TestRecordingShortcutRestoresCursor covers the hidden brush cursor carried by the reused screenshot window.
func TestRecordingShortcutRestoresCursor(t *testing.T) {
	state := &screenshotEditorOverlayState{
		frameSize: Size{Width: 800, Height: 600}, selection: Rect{X: 20, Y: 20, Width: 700, Height: 500},
		hasSelection: true, allowVideoRecording: true,
		result: make(chan screenshotEditorOverlayOutcome, 1),
	}
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 200, Y: 200}})
	state.key(KeyEvent{Key: Key("m"), Down: true})
	if state.pointerCursor != PointerCursorHidden {
		t.Fatal("mosaic should hide the native cursor before the recording shortcut")
	}
	if !state.key(KeyEvent{Key: Key("v"), Down: true}) {
		t.Fatal("recording shortcut was not handled")
	}
	if outcome := <-state.result; !outcome.record {
		t.Fatal("recording shortcut did not start the handoff")
	}
	if state.pointerCursor != PointerCursorDefault {
		t.Fatal("recording toolbar inherited the hidden screenshot cursor")
	}
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 210, Y: 200}})
	if state.pointerCursor == PointerCursorHidden {
		t.Fatal("pointer movement during the recording handoff hid the cursor again")
	}
}

// TestRecordingBorderCursorPersistsWithinBand models native cursor resets between nearby mouse positions.
func TestRecordingBorderCursorPersistsWithinBand(t *testing.T) {
	selection := Rect{X: 200, Y: 200, Width: 600, Height: 400}
	for _, scale := range []float32{1, 1.25, 1.5, 2, 2.5} {
		nativeCursor := PointerCursorDefault
		state := &recordingToolbarState{
			selection: selection,
			editor: &screenshotEditorOverlayState{
				selection: selection, hasSelection: true,
				chromeScale: func(Rect) float32 { return scale },
			},
			setBorderCursor: func(cursor PointerCursor) error {
				nativeCursor = cursor
				return nil
			},
		}
		tolerance := recordingSelectionInteractiveTolerance(scale)
		for _, point := range []Point{
			{X: 200 - tolerance, Y: 300}, {X: 200 - 7.5, Y: 300},
			{X: 200, Y: 300}, {X: 200 + 7.5, Y: 301}, {X: 200 + tolerance, Y: 302},
			{X: 200 + tolerance, Y: 302}, // A stationary pointer still needs recovery after a native reset.
		} {
			nativeCursor = PointerCursorDefault
			state.borderPointer(PointerEvent{Kind: PointerMove, Position: point})
			if nativeCursor != PointerCursorMove {
				t.Fatalf("scale %v, point %+v: native cursor reverted inside the move band", scale, point)
			}
		}
		grip := Point{X: 200, Y: 200}
		for range 2 {
			nativeCursor = PointerCursorDefault
			state.updateBorderCursor(&grip)
			if nativeCursor != PointerCursorResizeNWSE {
				t.Fatal("resize grip cursor was not restored after a native reset")
			}
		}
		state.updateBorderCursor(nil)
		if nativeCursor != PointerCursorDefault {
			t.Fatal("leaving the border retained its cursor")
		}
		nativeCursor = PointerCursorText
		state.updateBorderCursor(nil)
		if nativeCursor != PointerCursorText {
			t.Fatal("idle border overwrote another application's cursor")
		}
	}
}

// TestRecordingBorderMoveBand covers hover and actual drag starts from both sides of every stroke.
func TestRecordingBorderMoveBand(t *testing.T) {
	selection := Rect{X: 200, Y: 200, Width: 600, Height: 400}
	for _, scale := range []float32{1, 1.25, 1.5, 2, 2.5} {
		tolerance := recordingSelectionInteractiveTolerance(scale)
		for _, edge := range []struct {
			point  Point
			normal Point
		}{
			{Point{X: 200, Y: 300}, Point{X: 1}},
			{Point{X: 800, Y: 300}, Point{X: 1}},
			{Point{X: 350, Y: 200}, Point{Y: 1}},
			{Point{X: 350, Y: 600}, Point{Y: 1}},
		} {
			for _, offset := range []float32{-tolerance - 1, -tolerance, -6, -0.5, 0, 0.5, 6, tolerance, tolerance + 1} {
				point := Point{X: edge.point.X + edge.normal.X*offset, Y: edge.point.Y + edge.normal.Y*offset}
				wantHit := offset >= -tolerance && offset <= tolerance
				if hit := recordingSelectionEdgeContains(selection, point, tolerance); hit != wantHit {
					t.Fatalf("scale %v, point %+v: intercept = %v, want %v", scale, point, hit, wantHit)
				}
				state := &recordingToolbarState{
					selection: selection,
					editor: &screenshotEditorOverlayState{
						selection: selection, hasSelection: true, frameSize: Size{Width: 1400, Height: 1000},
						chromeScale: func(Rect) float32 { return scale },
					},
				}
				state.borderPointer(PointerEvent{Kind: PointerMove, Position: point})
				state.borderPointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: point})
				if !wantHit {
					if state.borderCursor != PointerCursorDefault || state.editor.editMode != screenshotEditorEditNone {
						t.Fatalf("scale %v, point %+v: outside the band still advertises or starts a drag", scale, point)
					}
					continue
				}
				if state.borderCursor != PointerCursorMove || state.editor.editMode != screenshotEditorEditMoveSelection {
					t.Fatalf("scale %v, point %+v: cursor and drag do not match the move band", scale, point)
				}
				state.borderPointer(PointerEvent{Kind: PointerMove, Position: Point{X: point.X + 20, Y: point.Y + 15}})
				want := Rect{X: selection.X + 20, Y: selection.Y + 15, Width: selection.Width, Height: selection.Height}
				if state.selection != want {
					t.Fatalf("scale %v, point %+v: drag jumped to %+v, want %+v", scale, point, state.selection, want)
				}
			}
		}
	}
}

// TestRecordingBorderPointerCursors covers the eight grips, move edges, and window-local coordinate conversion.
func TestRecordingBorderPointerCursors(t *testing.T) {
	selection := Rect{X: 100, Y: 100, Width: 400, Height: 300}
	for _, scale := range []float32{1, 1.25, 1.5, 2, 2.5} {
		for _, origin := range []Point{{}, {X: 60, Y: 60}, {X: -1920, Y: -1080}} {
			state := &recordingToolbarState{
				selection: selection, borderOrigin: origin,
				editor: &screenshotEditorOverlayState{
					selection: selection, hasSelection: true,
					chromeScale: func(Rect) float32 { return scale },
				},
			}
			for _, sample := range []struct {
				point  Point
				cursor PointerCursor
			}{
				{Point{X: 100, Y: 100}, PointerCursorResizeNWSE},
				{Point{X: 300, Y: 100}, PointerCursorResizeVertical},
				{Point{X: 500, Y: 100}, PointerCursorResizeNESW},
				{Point{X: 500, Y: 250}, PointerCursorResizeHorizontal},
				{Point{X: 500, Y: 400}, PointerCursorResizeNWSE},
				{Point{X: 300, Y: 400}, PointerCursorResizeVertical},
				{Point{X: 100, Y: 400}, PointerCursorResizeNESW},
				{Point{X: 100, Y: 250}, PointerCursorResizeHorizontal},
				{Point{X: 200, Y: 100}, PointerCursorMove},
				{Point{X: 500, Y: 175}, PointerCursorMove},
				{Point{X: 200, Y: 400}, PointerCursorMove},
				{Point{X: 100, Y: 175}, PointerCursorMove},
				{Point{X: 300, Y: 250}, PointerCursorDefault},
				{Point{X: 50, Y: 50}, PointerCursorDefault},
				{Point{X: 100 - 10*scale, Y: 100}, PointerCursorResizeNWSE},
			} {
				state.borderPointer(PointerEvent{Kind: PointerMove, Position: Point{X: sample.point.X - origin.X, Y: sample.point.Y - origin.Y}})
				if state.borderCursor != sample.cursor {
					t.Fatalf("scale %v, origin %+v, point %+v: cursor = %v, want %v", scale, origin, sample.point, state.borderCursor, sample.cursor)
				}
			}
			state.borderPointer(PointerEvent{Kind: PointerLeave})
			if state.borderCursor != PointerCursorDefault {
				t.Fatal("leaving an idle border did not restore the default cursor")
			}
		}
	}
}

// TestRecordingBorderCursorFollowsDragMode keeps pointer feedback stable while a drag crosses other targets.
func TestRecordingBorderCursorFollowsDragMode(t *testing.T) {
	for _, sample := range []struct {
		point  Point
		cursor PointerCursor
	}{
		{Point{X: 100, Y: 175}, PointerCursorMove},
		{Point{X: 500, Y: 175}, PointerCursorMove},
		{Point{X: 200, Y: 400}, PointerCursorMove},
		{Point{X: 100, Y: 100}, PointerCursorResizeNWSE},
	} {
		selection := Rect{X: 100, Y: 100, Width: 400, Height: 300}
		state := &recordingToolbarState{
			selection: selection,
			editor: &screenshotEditorOverlayState{
				selection: selection, hasSelection: true, frameSize: Size{Width: 1000, Height: 800},
			},
		}
		state.borderPointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: sample.point})
		state.borderPointer(PointerEvent{Kind: PointerMove, Position: Point{X: 300, Y: 250}})
		state.borderPointer(PointerEvent{Kind: PointerLeave})
		if state.borderCursor != sample.cursor {
			t.Fatalf("drag cursor = %v, want %v until release", state.borderCursor, sample.cursor)
		}
		state.borderPointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 300, Y: 250}})
		state.borderPointer(PointerEvent{Kind: PointerMove, Position: Point{X: 10, Y: 10}})
		if state.borderCursor != PointerCursorDefault {
			t.Fatal("completed drag left a stale cursor outside the selection")
		}
		point := Point{X: state.selection.X, Y: state.selection.Y}
		state.updateBorderCursor(&point)
		state.session = &recordingSession{state: recordingStateCountdown}
		state.updateBorderCursor(&point)
		if state.borderCursor != PointerCursorDefault {
			t.Fatal("locked recording region still advertised resize interaction")
		}
	}
}

// TestRecordingBorderCursorUsesCurrentDisplayScale keeps hover and drag aligned before the next redraw.
func TestRecordingBorderCursorUsesCurrentDisplayScale(t *testing.T) {
	scale := float32(1)
	selection := Rect{X: 100, Y: 100, Width: 400, Height: 300}
	state := &recordingToolbarState{
		selection: selection,
		editor: &screenshotEditorOverlayState{
			selection: selection, hasSelection: true, uiScale: 1,
			chromeScale: func(Rect) float32 { return scale },
		},
	}
	point := Point{X: 80, Y: 100}
	state.updateBorderCursor(&point)
	if state.borderCursor != PointerCursorDefault {
		t.Fatal("point outside the 1x grip should keep the default cursor")
	}
	scale = 2.5
	state.updateBorderCursor(&point)
	if state.borderCursor != PointerCursorResizeNWSE {
		t.Fatal("cursor polling did not pick up the new display's larger grip")
	}
	state.borderPointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: point})
	if state.editor.editMode != screenshotEditorEditResizeSelection || state.editor.uiScale != scale {
		t.Fatal("drag hit-testing did not use the same display scale as hover")
	}
}
