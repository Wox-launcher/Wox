package theme

import "testing"

func TestParseColorSupportsRGBA(t *testing.T) {
	t.Parallel()

	color, ok := ParseColor("rgba(35, 41, 51, 0.75)")
	if !ok {
		t.Fatal("ParseColor should accept rgba colors")
	}

	if color.R != 35 || color.G != 41 || color.B != 51 {
		t.Fatalf("unexpected rgba channels: %+v", color)
	}

	if color.A != 191 {
		t.Fatalf("unexpected alpha: got %d want 191", color.A)
	}
}

func TestParseColorSupportsHex(t *testing.T) {
	t.Parallel()

	color, ok := ParseColor("#E2E8F0")
	if !ok {
		t.Fatal("ParseColor should accept hex colors")
	}

	if color.R != 0xE2 || color.G != 0xE8 || color.B != 0xF0 || color.A != 0xFF {
		t.Fatalf("unexpected hex color: %+v", color)
	}
}

func TestRGBAColorCompositeOver(t *testing.T) {
	t.Parallel()

	foreground := RGBAColor{R: 49, G: 56, B: 68, A: 77}
	background := RGBAColor{R: 35, G: 41, B: 51, A: 255}

	composited := foreground.CompositeOver(background)

	if composited.A != 255 {
		t.Fatalf("composited color should be opaque, got alpha=%d", composited.A)
	}

	if composited.R <= background.R || composited.G <= background.G || composited.B <= background.B {
		t.Fatalf("composited color should be lighter than the background, got %+v", composited)
	}
}
