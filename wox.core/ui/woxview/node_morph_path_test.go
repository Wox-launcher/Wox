package woxview

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

func TestMorphPathKeyframeTimeline(t *testing.T) {
	from := []PathCommand{{Op: PathMove, Values: []float64{0, 0}}, {Op: PathLine, Values: []float64{.2, 0}}}
	to := []PathCommand{{Op: PathMove, Values: []float64{0, 0}}, {Op: PathLine, Values: []float64{.9, 0}}}
	middle := []PathCommand{{Op: PathMove, Values: []float64{0, 0}}, {Op: PathLine, Values: []float64{.6, 0}}}
	p := MorphPathProps{Keyframes: []MorphPathKeyframe{{At: 0, Path: from}, {At: .4, Path: middle}, {At: 1, Path: to}}}
	if err := p.prepare(); err != nil {
		t.Fatal(err)
	}
	sample := func(progress float64) float64 {
		segment, position := p.segmentAt(progress)
		return segment.coordinate(1, 0, position)
	}
	if got := sample(.4); math.Abs(got-.6) > 1e-9 {
		t.Fatalf("keyframe: %v", got)
	}
	delta := .00001
	left := (sample(.4) - sample(.4-delta)) / delta
	right := (sample(.4+delta) - sample(.4)) / delta
	if math.Abs(left-right) > .001 || left <= 0 {
		t.Fatalf("velocity discontinuity: %v %v", left, right)
	}
	for i := 0; i <= 100; i++ {
		value := sample(float64(i) / 100)
		if value < .2-1e-9 || value > .9+1e-9 {
			t.Fatalf("overshot: %v", value)
		}
	}
	bad := []MorphPathKeyframe{{At: 0, Path: from}, {At: .5, Path: middle}, {At: .5, Path: middle}, {At: 1, Path: to}}
	p.Keyframes = bad
	if err := p.prepare(); err == nil {
		t.Fatal("accepted unordered keyframes")
	}
	p.Keyframes = []MorphPathKeyframe{{At: 0, Path: from}, {At: 1e-320, Path: middle}, {At: 1, Path: to}}
	if err := p.prepare(); err == nil {
		t.Fatal("accepted indistinguishable keyframes")
	}
	p.Keyframes = []MorphPathKeyframe{{At: 0, Path: from}, {At: 1, Path: to}}
	if err := p.prepare(); err != nil {
		t.Fatal(err)
	}
	if got := sample(.25); math.Abs(got-.375) > 1e-9 {
		t.Fatalf("two-frame interpolation is not linear: %v", got)
	}
}

// TestMorphPathKeyframesRenderAndReverse checks the same timeline through the widget clock.
func TestMorphPathKeyframesRenderAndReverse(t *testing.T) {
	frame := `[{"op":"M","values":[0,0]},{"op":"L","values":[0.7,0]},{"op":"L","values":[0.7,1]},{"op":"L","values":[0,1]},{"op":"Z"}]`
	doc := func(progress int) string {
		return fmt.Sprintf(`{"version":1,"root":{"type":"column","children":[{"type":"morph-path","key":"shape","props":{"keyframes":[{"at":0,"path":%s},{"at":0.5,"path":%s},{"at":1,"path":%s}],"progress":%d,"duration":200,"curve":"linear","style":{"width":100,"height":100,"color":"#ff0000"}}}]}}`, morphFrom, frame, morphTo, progress)
	}
	v := NewView()
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	now := time.Unix(1, 0)
	host.SetAnimationClock(func() time.Time { return now })
	v.SetTree(mustParse(t, doc(0)))
	statePixel(t, host, 1, 60, 50)
	v.SetTree(mustParse(t, doc(1)))
	statePixel(t, host, 1, 60, 50)
	now = now.Add(100 * time.Millisecond)
	if pixel := statePixel(t, host, 2, 60, 50); pixel.R < 240 {
		t.Fatalf("keyframe did not render: %+v", pixel)
	}
	v.SetTree(mustParse(t, doc(0)))
	if pixel := statePixel(t, host, 2, 60, 50); pixel.R < 240 {
		t.Fatalf("retarget jumped: %+v", pixel)
	}
	now = now.Add(200 * time.Millisecond)
	if pixel := statePixel(t, host, 2, 60, 50); pixel.A != 0 {
		t.Fatalf("reverse did not finish: %+v", pixel)
	}
}

// TestMorphPathImplicitCloseAndAlpha preserves SVG fill semantics for multiple open subpaths.
func TestMorphPathImplicitCloseAndAlpha(t *testing.T) {
	path := `[{"op":"M","values":[0,0]},{"op":"L","values":[0.4,0]},{"op":"L","values":[0,0.4]},{"op":"M","values":[0.6,0.6]},{"op":"L","values":[1,0.6]},{"op":"L","values":[1,1]}]`
	doc := fmt.Sprintf(`{"version":1,"root":{"type":"column","children":[{"type":"morph-path","props":{"keyframes":[{"at":0,"path":%s},{"at":1,"path":%s}],"style":{"width":100,"height":100,"color":"#ff000080"}}}]}}`, path, path)
	v := NewView()
	v.SetTree(mustParse(t, doc))
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	for _, pt := range [][2]float32{{10, 10}, {90, 70}} {
		pixel := statePixel(t, host, 2, pt[0], pt[1])
		if pixel.A < 126 || pixel.A > 129 || pixel.R != pixel.A {
			t.Fatalf("fill at %v: %+v", pt, pixel)
		}
	}
	if pixel := statePixel(t, host, 2, 50, 50); pixel.A != 0 {
		t.Fatalf("outside fill: %+v", pixel)
	}
}

const morphFrom = `[{"op":"M","values":[0,0]},{"op":"L","values":[0.2,0]},{"op":"L","values":[0.2,1]},{"op":"L","values":[0,1]},{"op":"Z"}]`
const morphTo = `[{"op":"M","values":[0,0]},{"op":"L","values":[0.8,0]},{"op":"L","values":[0.8,1]},{"op":"L","values":[0,1]},{"op":"Z"}]`

func morphDocument(progress float64) string {
	return fmt.Sprintf(`{"version":1,"root":{"type":"column","children":[{"type":"morph-path","key":"shape","props":{"keyframes":[{"at":0,"path":%s},{"at":1,"path":%s}],"progress":%g,"duration":200,"curve":"linear","style":{"width":100,"height":100,"color":"#ff0000"}}}]}}`, morphFrom, morphTo, progress)
}

// TestMorphPathInterpolatesAndRetargets exercises rendering and the shared animation clock.
func TestMorphPathInterpolatesAndRetargets(t *testing.T) {
	v := NewView()
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	now := time.Unix(1, 0)
	host.SetAnimationClock(func() time.Time { return now })
	v.SetTree(mustParse(t, morphDocument(0)))
	if pixel := statePixel(t, host, 1, 30, 50); pixel.A != 0 {
		t.Fatalf("initial path unexpectedly covers pixel: %+v", pixel)
	}
	v.SetTree(mustParse(t, morphDocument(1)))
	statePixel(t, host, 1, 30, 50)
	now = now.Add(100 * time.Millisecond)
	if pixel := statePixel(t, host, 1, 40, 50); pixel.R < 240 || pixel.A < 240 {
		t.Fatalf("interpolated path did not cover midpoint: %+v", pixel)
	}
	if pixel := statePixel(t, host, 1, 60, 50); pixel.A != 0 {
		t.Fatalf("interpolated path exceeded midpoint: %+v", pixel)
	}
	v.SetTree(mustParse(t, morphDocument(0)))
	if pixel := statePixel(t, host, 1, 40, 50); pixel.R < 240 {
		t.Fatalf("retarget jumped from current shape: %+v", pixel)
	}
	now = now.Add(200 * time.Millisecond)
	if pixel := statePixel(t, host, 1, 30, 50); pixel.A != 0 {
		t.Fatalf("retarget did not return to original path: %+v", pixel)
	}
}

// TestMorphPathResolvesThemeAndDPI verifies color changes do not mutate the source outline.
func TestMorphPathResolvesThemeAndDPI(t *testing.T) {
	v := NewView()
	document := strings.Replace(morphDocument(1), `"#ff0000"`, `"$accent"`, 1)
	v.SetTree(mustParse(t, document))
	theme := component.ControlTheme{Accent: ui.Color{R: 255, A: 255}}
	host := widget.NewHost(func(info ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height, Theme: theme})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	if pixel := statePixel(t, host, 1.5, 40, 50); pixel.R < 240 || pixel.A < 240 {
		t.Fatalf("red theme: %+v", pixel)
	}
	theme.Accent = ui.Color{B: 255, A: 255}
	if pixel := statePixel(t, host, 2, 40, 50); pixel.B < 240 || pixel.R != 0 {
		t.Fatalf("blue theme: %+v", pixel)
	}
}

// TestMorphPathCurvedNeck exercises a non-convex, continuously narrowing outline.
func TestMorphPathCurvedNeck(t *testing.T) {
	path := func(top, bottom float64) string {
		return fmt.Sprintf(`[{"op":"M","values":[0,0]},{"op":"C","values":[0.2,0,0.2,%g,0.5,%g]},{"op":"C","values":[0.8,%g,0.8,0,1,0]},{"op":"L","values":[1,1]},{"op":"C","values":[0.8,1,0.8,%g,0.5,%g]},{"op":"C","values":[0.2,%g,0.2,1,0,1]},{"op":"Z"}]`, top, top, top, bottom, bottom, bottom)
	}
	document := func(progress int) string {
		return fmt.Sprintf(`{"version":1,"root":{"type":"column","children":[{"type":"morph-path","key":"neck","props":{"keyframes":[{"at":0,"path":%s},{"at":1,"path":%s}],"progress":%d,"duration":200,"curve":"linear","style":{"width":100,"height":100,"color":"#ff0000"}}}]}}`, path(.49, .51), path(.2, .8), progress)
	}
	v := NewView()
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	now := time.Unix(1, 0)
	host.SetAnimationClock(func() time.Time { return now })
	v.SetTree(mustParse(t, document(0)))
	if pixel := statePixel(t, host, 1, 50, 35); pixel.A != 0 {
		t.Fatalf("narrow neck covered exterior: %+v", pixel)
	}
	v.SetTree(mustParse(t, document(1)))
	statePixel(t, host, 1, 50, 35)
	now = now.Add(200 * time.Millisecond)
	if pixel := statePixel(t, host, 1, 50, 35); pixel.R < 240 {
		t.Fatalf("expanded neck did not cover pixel: %+v", pixel)
	}
}

// TestMorphPathRejectsInvalidTopology keeps untrusted outlines bounded and compatible.
func TestMorphPathRejectsInvalidTopology(t *testing.T) {
	for _, raw := range []string{
		`{}`, `{"keyframes":[]}`, `{"keyframes":[{"at":0,"path":[{"op":"M","values":[0,0]}]}]}`,
		`{"keyframes":[{"at":0.1,"path":[{"op":"M","values":[0,0]}]},{"at":1,"path":[{"op":"M","values":[0,0]}]}]}`,
		`{"keyframes":[{"at":0,"path":[{"op":"L","values":[0,0]}]},{"at":1,"path":[{"op":"L","values":[0,0]}]}]}`,
		`{"keyframes":[{"at":0,"path":[{"op":"M","values":[0,0]}]},{"at":1,"path":[{"op":"M","values":[0]}]}]}`,
		`{"keyframes":[{"at":0,"path":[{"op":"M","values":[0,0]}]},{"at":1,"path":[{"op":"L","values":[0,0]}]}]}`,
		`{"keyframes":[{"at":0,"path":[{"op":"M","values":[0,0]}]},{"at":1,"path":[{"op":"M","values":[5,0]}]}]}`,
		`{"keyframes":[{"at":0,"path":[{"op":"M","values":[0,0]}]},{"at":1,"path":[{"op":"M","values":[0,0]}]}],"progress":2}`,
		`{"keyframes":[{"at":0,"path":[{"op":"M","values":[0,0]}]},{"at":1,"path":[{"op":"M","values":[0,0]}]}],"curve":"spring"}`,
		`{"keyframes":[{"at":0,"path":[{"op":"M","values":[0,0]},{"op":"Z"},{"op":"L","values":[1,1]}]},{"at":1,"path":[{"op":"M","values":[0,0]},{"op":"Z"},{"op":"L","values":[1,1]}]}]}`,
	} {
		if _, err := Parse([]byte(`{"version":1,"root":{"type":"morph-path","props":` + raw + `}}`)); err == nil || !strings.Contains(err.Error(), "morph-path") {
			t.Fatalf("accepted invalid path: %s (%v)", raw, err)
		}
	}
}
