package svg

import (
	"image/color"
	"testing"
)

const iconifyTwoToneLamp = `<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 48 48"><defs><mask id="SVG1erRqbAS"><g fill="none" stroke="#fff" stroke-width="4"><path fill="#555555" d="M8 24.596C8 25.37 8.629 26 9.404 26h29.192C39.37 26 40 25.371 40 24.596V20c0-8.837-7.163-16-16-16S8 11.163 8 20z"/><path stroke-linecap="round" stroke-linejoin="round" d="M24 42V26m-9 6v-6m18 16H15"/></g></mask></defs><path fill="currentColor" d="M0 0h48v48H0z" mask="url(#SVG1erRqbAS)"/></svg>`

func TestRenderAppliesLuminanceMask(t *testing.T) {
	rgba, err := RenderWithCurrentColor(iconifyTwoToneLamp, 48, 48, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	if err != nil {
		t.Fatalf("render masked SVG: %v", err)
	}

	corner := rgba.RGBAAt(0, 0)
	if corner.A != 0 {
		t.Fatalf("corner alpha = %d, want 0 so the mask punched out the currentColor rectangle", corner.A)
	}

	body := rgba.RGBAAt(24, 16)
	if body.A == 0 {
		t.Fatal("lamp body is transparent, want the two-tone mask fill")
	}
	if body.R < 40 || body.G < 40 || body.B < 40 {
		t.Fatalf("lamp body color = %+v, want a visible currentColor sample", body)
	}
}

func TestRenderCurrentColorOverridesDefaultBlack(t *testing.T) {
	const source = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10" fill="currentColor"/></svg>`
	rgba, err := RenderWithCurrentColor(source, 10, 10, color.NRGBA{R: 16, G: 80, B: 240, A: 255})
	if err != nil {
		t.Fatalf("render currentColor SVG: %v", err)
	}
	pixel := rgba.RGBAAt(5, 5)
	if pixel.R != 16 || pixel.G != 80 || pixel.B != 240 || pixel.A != 255 {
		t.Fatalf("currentColor pixel = %+v, want {16 80 240 255}", pixel)
	}
}

func TestRenderAnimationSurvivesFreezeAndRotates(t *testing.T) {
	const source = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect x="0" y="4" width="4" height="2" fill="#000"><animateTransform attributeName="transform" type="rotate" from="0 5 5" to="180 5 5" dur="1s" repeatCount="indefinite" fill="freeze"/></rect></svg>`
	frames, delays, err := RenderFrames(source, 10, 10, nil)
	if err != nil {
		t.Fatalf("render animated SVG: %v", err)
	}
	if len(frames) < 2 || len(delays) != len(frames) {
		t.Fatalf("frames = %d delays = %d", len(frames), len(delays))
	}
	if frames[0].RGBAAt(1, 5).A < 200 {
		t.Fatalf("start pose alpha = %d, want the unrotated rect", frames[0].RGBAAt(1, 5).A)
	}
	if frames[0].RGBAAt(8, 5).A > 40 {
		t.Fatalf("start pose unexpectedly covers the far side: alpha %d", frames[0].RGBAAt(8, 5).A)
	}
	last := frames[len(frames)-1]
	left, right := 0, 0
	for x := 0; x < 5; x++ {
		left += int(last.RGBAAt(x, 5).A)
	}
	for x := 5; x < 10; x++ {
		right += int(last.RGBAAt(x, 5).A)
	}
	if right <= left {
		t.Fatalf("later pose stayed on the left (left=%d right=%d)", left, right)
	}
}

func TestRenderOpacityAnimationFadesIn(t *testing.T) {
	const source = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10" fill="#000"><animate attributeName="opacity" from="0" to="1" dur="1s" repeatCount="indefinite" fill="freeze"/></rect></svg>`
	frames, _, err := RenderFrames(source, 10, 10, nil)
	if err != nil {
		t.Fatalf("render opacity animation: %v", err)
	}
	if frames[0].RGBAAt(5, 5).A > 20 {
		t.Fatalf("start alpha = %d, want a transparent pose", frames[0].RGBAAt(5, 5).A)
	}
	end := frames[len(frames)-1].RGBAAt(5, 5).A
	if end < 200 {
		t.Fatalf("end alpha = %d, want a nearly opaque pose", end)
	}
}

func TestRenderKeepsExplicitColors(t *testing.T) {
	const source = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10" fill="#2f88ff"/></svg>`
	rgba, err := RenderWithCurrentColor(source, 10, 10, color.NRGBA{R: 255, G: 0, B: 0, A: 255})
	if err != nil {
		t.Fatalf("render explicit-color SVG: %v", err)
	}
	pixel := rgba.RGBAAt(5, 5)
	if pixel.R != 0x2f || pixel.G != 0x88 || pixel.B != 0xff {
		t.Fatalf("explicit color pixel = %+v, want #2f88ff", pixel)
	}
}
