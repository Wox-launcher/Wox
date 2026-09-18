package screenshot

import (
	"image"
	"image/color"
	"image/draw"
	"os"
	"strings"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

func TestScreenshotEditorGoRegularLacksCJKGlyphs(t *testing.T) {
	parsed, err := opentype.Parse(goregular.TTF)
	if err != nil {
		t.Fatalf("parse Go Regular: %v", err)
	}
	if screenshotEditorFontHasRune(parsed, '中') {
		t.Fatal("Go Regular unexpectedly contains 中; export no longer needs a system CJK face")
	}
	if !screenshotEditorFontHasRune(parsed, 'A') || !screenshotEditorFontHasRune(parsed, ',') {
		t.Fatal("Go Regular should still cover Latin letters and punctuation")
	}
}

func TestScreenshotEditorCJKFontScorePrefersSimplifiedChinese(t *testing.T) {
	sc := screenshotEditorCJKFontScore("Noto Sans CJK SC")
	if sc < screenshotEditorPreferredCJKScore {
		t.Fatalf("SC score = %d, want preferred CJK", sc)
	}
	if screenshotEditorCJKFontScore("Noto Sans CJK KR") >= sc {
		t.Fatal("Korean CJK face should rank below Simplified Chinese")
	}
	if screenshotEditorCJKFontScore("Noto Sans Mono CJK SC") >= screenshotEditorCJKFontScore("Noto Sans CJK JP") {
		t.Fatal("mono CJK faces should rank below proportional faces")
	}
	if screenshotEditorCJKFontScore("Microsoft YaHei") <= screenshotEditorCJKFontScore("Malgun Gothic") {
		t.Fatal("YaHei should rank above Korean UI faces")
	}
}

func TestPickScreenshotEditorCJKFontPrefersSCFromNotoCollection(t *testing.T) {
	path := "/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc"
	if _, err := os.Stat(path); err != nil {
		t.Skip("Noto Sans CJK Regular collection is not installed")
	}
	parsed, score := pickScreenshotEditorCJKFont(path)
	if parsed == nil || score < screenshotEditorPreferredCJKScore {
		t.Fatalf("Noto CJK pick score = %d, want Simplified Chinese", score)
	}
	var buf sfnt.Buffer
	family, err := parsed.Name(&buf, sfnt.NameIDFamily)
	if err != nil {
		t.Fatalf("font family: %v", err)
	}
	if !strings.Contains(family, "SC") || strings.Contains(strings.ToLower(family), "mono") {
		t.Fatalf("picked family = %q, want a proportional Simplified Chinese face", family)
	}
}

func TestScreenshotEditorExportFontsCoverCJK(t *testing.T) {
	fonts := screenshotEditorExportFonts()
	if len(fonts) == 0 {
		t.Fatal("export fonts were empty")
	}
	if !screenshotEditorFontsHaveRune(fonts, '中') {
		t.Skip("no system CJK font available for screenshot export")
	}
	if !screenshotEditorFontsHaveRune(fonts, 'A') {
		t.Fatal("export fonts missing Latin coverage used by mixed labels")
	}
}

func TestScreenshotEditorExportPaintsCJKInsteadOfTofu(t *testing.T) {
	fonts := screenshotEditorExportFonts()
	if !screenshotEditorFontsHaveRune(fonts, '中') {
		t.Skip("no system CJK font available for screenshot export")
	}

	text := "你好,世界."
	source := image.NewRGBA(image.Rect(0, 0, 280, 80))
	draw.Draw(source, source.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	output, err := renderScreenshotEditorAnnotations(source, []screenshotEditorAnnotation{{
		tool: screenshotEditorToolText, start: Point{X: 16, Y: 24}, text: text,
		color: screenshotEditorAnnotationColor, fontSize: 24,
	}}, Rect{Width: 280, Height: 80}, Size{Width: 280, Height: 80}, 1)
	if err != nil {
		t.Fatalf("export CJK text: %v", err)
	}

	exportedInk := countScreenshotEditorAnnotationInk(output)
	tofuInk := countScreenshotEditorDrawnInk(text, 24, goregularOnlyScreenshotEditorFont(t))
	if exportedInk <= tofuInk {
		t.Fatalf("CJK export ink = %d, tofu ink = %d, want real glyphs instead of .notdef boxes", exportedInk, tofuInk)
	}
}

func goregularOnlyScreenshotEditorFont(t *testing.T) *opentype.Font {
	t.Helper()
	parsed, err := opentype.Parse(goregular.TTF)
	if err != nil {
		t.Fatalf("parse Go Regular: %v", err)
	}
	return parsed
}

func countScreenshotEditorDrawnInk(text string, size float32, parsed *opentype.Font) int {
	img := image.NewRGBA(image.Rect(0, 0, 280, 80))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: float64(size), DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return 0
	}
	defer face.Close()
	drawer := font.Drawer{
		Dst: img, Src: image.NewUniform(color.RGBA{R: screenshotEditorAnnotationColor.R, G: screenshotEditorAnnotationColor.G, B: screenshotEditorAnnotationColor.B, A: 255}),
		Face: face, Dot: fixed.P(16, 24+int(size)),
	}
	drawer.DrawString(text)
	return countScreenshotEditorAnnotationInk(img)
}

func countScreenshotEditorAnnotationInk(img *image.RGBA) int {
	count := 0
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pixel := img.RGBAAt(x, y)
			if pixel.R > 200 && pixel.G < 130 && pixel.B < 90 {
				count++
			}
		}
	}
	return count
}
