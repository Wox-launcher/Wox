package screenshot

import (
	"fmt"
	"math"
	"testing"
	"time"
)

// TestRecordingOutlineHasUniformCoverage guards against gaps and thick diagonal spokes at glyph terminals.
func TestRecordingOutlineHasUniformCoverage(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2, 2.5} {
		width := 8 * scale
		offsets := recordingOutlineOffsets(width)
		for _, offset := range offsets {
			if radius := math.Hypot(float64(offset.X), float64(offset.Y)); math.Abs(radius-float64(width)) > 0.001 {
				t.Fatalf("scale %v: uneven outline radius %v, want %v", scale, radius, width)
			}
		}
		for degrees := range 360 {
			angle := float64(degrees) * math.Pi / 180
			x, y := float64(width)*math.Cos(angle), float64(width)*math.Sin(angle)
			nearest := math.Inf(1)
			for _, offset := range offsets {
				nearest = min(nearest, math.Hypot(x-float64(offset.X), y-float64(offset.Y)))
			}
			if nearest > 0.5 {
				t.Fatalf("scale %v: outline gap at %d degrees, nearest sample %v units away", scale, degrees, nearest)
			}
		}
	}
}

// TestRecordingCountdownFadesInPlace checks that only opacity changes within each second.
func TestRecordingCountdownFadesInPlace(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2, 2.5} {
		for _, seconds := range []int{3, 2, 1} {
			for _, sample := range []struct {
				elapsed time.Duration
				alpha   uint8
			}{
				{0, 0},
				{90 * time.Millisecond, 127},
				{180 * time.Millisecond, 255},
				{900 * time.Millisecond, 255},
			} {
				remaining := time.Duration(seconds)*time.Second - sample.elapsed
				actual := &DisplayList{}
				drawRecordingCountdown(actual, Rect{Width: 800 * scale, Height: 600 * scale}, remaining, scale)
				expected := &DisplayList{}
				// This digit box stays centered at (400, 300) logical units throughout the fade.
				drawRecordingOutlinedText(expected, fmt.Sprint(seconds), Rect{X: 350.4 * scale, Y: 212 * scale, Width: 99.2 * scale, Height: 176 * scale},
					TextStyle{Size: 160 * scale, Weight: FontWeightSemibold},
					Color{R: 255, G: 59, B: 48, A: sample.alpha}, Color{R: 255, G: 255, B: 255, A: sample.alpha}, 8*scale)
				if err := actual.Compare(expected); err != nil {
					t.Errorf("scale %v, digit %d, elapsed %v: countdown moved or used the wrong opacity: %v", scale, seconds, sample.elapsed, err)
				}
			}
		}
	}
}

// TestRecordingCountdownUsesOverlayCoordinates keeps desktop origins out of countdown layout.
func TestRecordingCountdownUsesOverlayCoordinates(t *testing.T) {
	now := time.Unix(100, 0)
	session := &recordingSession{
		state: recordingStateCountdown, countdownEndsAt: now.Add(2800 * time.Millisecond),
		config: recordingSessionConfig{Now: func() time.Time { return now }},
	}
	for _, origin := range []Point{{}, {X: -1920, Y: -1080}, {X: 2560, Y: 240}} {
		state := &recordingToolbarState{session: session, selection: Rect{X: origin.X, Y: origin.Y, Width: 800, Height: 600}}
		actual := &DisplayList{}
		state.drawOverlay(actual, FrameInfo{Size: Size{Width: 800, Height: 600}, Scale: 2})
		expected := &DisplayList{}
		drawRecordingCountdown(expected, Rect{Width: 800, Height: 600}, 2800*time.Millisecond, 1)
		if err := actual.Compare(expected); err != nil {
			t.Errorf("desktop origin %+v changed countdown layout: %v", origin, err)
		}
	}
}
