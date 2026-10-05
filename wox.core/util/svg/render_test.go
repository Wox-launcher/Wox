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

// codeOSSIcon is the Illustrator export shipped as com.visualstudio.code.oss.svg.
// Its colors live only in <style> class rules, and the first path covers the whole canvas.
const codeOSSIcon = `<svg id="Layer_1" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024"><style>.st0{fill:#f6f6f6;fill-opacity:0}.st1{fill:#fff}.st2{fill:#167abf}</style><path class="st0" d="M1024 1024H0V0h1024v1024z"/><path class="st1" d="M1024 85.333v853.333H0V85.333h1024z"/><path class="st2" d="M0 85.333h298.667v853.333H0V85.333zm1024 0v853.333H384V85.333h640zm-554.667 160h341.333v-64H469.333v64zm341.334 533.334H469.333v64h341.333l.001-64zm128-149.334H597.333v64h341.333l.001-64zm0-149.333H597.333v64h341.333l.001-64zm0-149.333H597.333v64h341.333l.001-64z"/></svg>`

func TestRenderStyleSheetClassColors(t *testing.T) {
	rgba, err := Render(codeOSSIcon, 64, 64)
	if err != nil {
		t.Fatalf("render class-styled SVG: %v", err)
	}
	if got := rgba.RGBAAt(0, 0); got.A != 0 {
		t.Fatalf("canvas pixel = %+v, want the full-size path to stay transparent", got)
	}
	if got := rgba.RGBAAt(21, 20); got != (color.RGBA{R: 255, G: 255, B: 255, A: 255}) {
		t.Fatalf("editor background = %+v, want white", got)
	}
	if got := rgba.RGBAAt(9, 32); got != (color.RGBA{R: 0x16, G: 0x7a, B: 0xbf, A: 255}) {
		t.Fatalf("logo pixel = %+v, want #167abf", got)
	}
}

func TestRenderStyleSheetCascade(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   color.RGBA
	}{
		{
			name:   "class beats presentation attribute",
			source: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><style>.box{fill:#167abf}</style><rect class="box" fill="#ff0000" width="10" height="10"/></svg>`,
			want:   color.RGBA{R: 0x16, G: 0x7a, B: 0xbf, A: 255},
		},
		{
			name:   "inline style beats class",
			source: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><style>.box{fill:#ff0000}</style><rect class="box" style="fill:#167abf" width="10" height="10"/></svg>`,
			want:   color.RGBA{R: 0x16, G: 0x7a, B: 0xbf, A: 255},
		},
		{
			name:   "style element after the shape",
			source: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect class="box" width="10" height="10"/><style>.box{fill:#167abf}</style></svg>`,
			want:   color.RGBA{R: 0x16, G: 0x7a, B: 0xbf, A: 255},
		},
		{
			name:   "grouped selectors",
			source: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><style>.a, .b { fill: #167abf }</style><rect class="b" width="10" height="10"/></svg>`,
			want:   color.RGBA{R: 0x16, G: 0x7a, B: 0xbf, A: 255},
		},
		{
			name:   "element selector",
			source: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><style>rect { fill: #167abf }</style><rect width="10" height="10"/></svg>`,
			want:   color.RGBA{R: 0x16, G: 0x7a, B: 0xbf, A: 255},
		},
		{
			name:   "id selector",
			source: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><style>#mark{fill:#167abf}</style><rect id="mark" width="10" height="10"/></svg>`,
			want:   color.RGBA{R: 0x16, G: 0x7a, B: 0xbf, A: 255},
		},
		{
			name:   "compound class requires every class",
			source: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><style>.a.b{fill:#ff0000}</style><rect class="a" fill="#167abf" width="10" height="10"/></svg>`,
			want:   color.RGBA{R: 0x16, G: 0x7a, B: 0xbf, A: 255},
		},
		{
			name:   "descendant selector is ignored",
			source: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><style>.missing .box { fill: #ff0000 }</style><rect class="box" fill="#167abf" width="10" height="10"/></svg>`,
			want:   color.RGBA{R: 0x16, G: 0x7a, B: 0xbf, A: 255},
		},
		{
			name:   "media rule is ignored",
			source: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><style>@media screen { .box { fill: #ff0000 } }</style><rect class="box" fill="#167abf" width="10" height="10"/></svg>`,
			want:   color.RGBA{R: 0x16, G: 0x7a, B: 0xbf, A: 255},
		},
		{
			name:   "invalid declaration is ignored",
			source: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><style>.box{fill:inherit}</style><rect class="box" fill="#167abf" width="10" height="10"/></svg>`,
			want:   color.RGBA{R: 0x16, G: 0x7a, B: 0xbf, A: 255},
		},
		{
			name:   "cdata style",
			source: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><style><![CDATA[.box{fill:#167abf}]]></style><rect class="box" width="10" height="10"/></svg>`,
			want:   color.RGBA{R: 0x16, G: 0x7a, B: 0xbf, A: 255},
		},
		{
			name:   "comment wrapped style",
			source: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><style><!-- .box{fill:#167abf} --></style><rect class="box" width="10" height="10"/></svg>`,
			want:   color.RGBA{R: 0x16, G: 0x7a, B: 0xbf, A: 255},
		},
		{
			name:   "parent class is inherited",
			source: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><style>.group{fill:#167abf}</style><g class="group"><rect width="10" height="10"/></g></svg>`,
			want:   color.RGBA{R: 0x16, G: 0x7a, B: 0xbf, A: 255},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rgba, err := Render(test.source, 10, 10)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if got := rgba.RGBAAt(5, 5); got != test.want {
				t.Fatalf("pixel = %+v, want %+v", got, test.want)
			}
		})
	}
}
