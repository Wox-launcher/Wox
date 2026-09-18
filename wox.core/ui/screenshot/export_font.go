package screenshot

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"
	"wox/util"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

const screenshotEditorPreferredCJKScore = 100

type screenshotEditorExportFace struct {
	font *opentype.Font
	face font.Face
}

// screenshotEditorExportFonts returns faces used when baking labels into the saved image.
// Live preview uses the native system renderer, but JPEG export rasterizes with
// golang.org/x/image. Go Regular has no CJK glyphs, so a missing system face turns
// Chinese into .notdef boxes after save.
func screenshotEditorExportFonts() []*opentype.Font {
	screenshotEditorFontOnce.Do(func() {
		screenshotEditorFonts = loadScreenshotEditorExportFonts()
	})
	return screenshotEditorFonts
}

func screenshotEditorExportFont() *opentype.Font {
	fonts := screenshotEditorExportFonts()
	if len(fonts) == 0 {
		return nil
	}
	return fonts[0]
}

// loadScreenshotEditorExportFonts prefers a system CJK face and always keeps Go Regular as fallback.
func loadScreenshotEditorExportFonts() []*opentype.Font {
	var best *opentype.Font
	bestScore := -1
	seen := map[string]struct{}{}
	for _, path := range screenshotEditorExportFontPaths() {
		if path == "" {
			continue
		}
		if _, loaded := seen[path]; loaded {
			continue
		}
		seen[path] = struct{}{}
		parsed, score := pickScreenshotEditorCJKFont(path)
		if parsed == nil || score <= bestScore {
			continue
		}
		best = parsed
		bestScore = score
		if score >= screenshotEditorPreferredCJKScore {
			break
		}
	}

	fonts := make([]*opentype.Font, 0, 2)
	if best != nil {
		fonts = append(fonts, best)
		var buf sfnt.Buffer
		family, _ := best.Name(&buf, sfnt.NameIDFamily)
		util.GetLogger().Debug(context.Background(), fmt.Sprintf("screenshot export text font=%s", family))
	} else {
		util.GetLogger().Warn(context.Background(), "screenshot export font has no CJK glyphs; text labels may render as boxes")
	}
	if fallback, err := opentype.Parse(goregular.TTF); err == nil && fallback != nil {
		fonts = append(fonts, fallback)
	}
	return fonts
}

func screenshotEditorExportFontPaths() []string {
	paths := make([]string, 0, 24)
	if runtime.GOOS == "linux" {
		paths = append(paths, screenshotEditorFontconfigFiles()...)
	}
	return append(paths, screenshotEditorWellKnownFontPaths()...)
}

func screenshotEditorWellKnownFontPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
			"/Library/Fonts/Arial Unicode.ttf",
			"/System/Library/Fonts/PingFang.ttc",
			"/System/Library/Fonts/STHeiti Light.ttc",
			"/System/Library/Fonts/Hiragino Sans GB.ttc",
			"/System/Library/Fonts/Supplemental/Songti.ttc",
			"/System/Library/Fonts/AppleSDGothicNeo.ttc",
		}
	case "windows":
		fontsDir := filepath.Join(screenshotEditorWindowsDirectory(), "Fonts")
		return []string{
			filepath.Join(fontsDir, "msyh.ttc"),
			filepath.Join(fontsDir, "msyh.ttf"),
			filepath.Join(fontsDir, "msjh.ttc"),
			filepath.Join(fontsDir, "msjh.ttf"),
			filepath.Join(fontsDir, "simsun.ttc"),
			filepath.Join(fontsDir, "simhei.ttf"),
			filepath.Join(fontsDir, "malgun.ttf"),
			filepath.Join(fontsDir, "msgothic.ttc"),
			filepath.Join(fontsDir, "YuGothM.ttc"),
			filepath.Join(fontsDir, "arialuni.ttf"),
		}
	default:
		return []string{
			"/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/google-noto-cjk/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/opentype/noto/NotoSansCJKsc-Regular.otf",
			"/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
			"/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc",
			"/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf",
			"/usr/share/fonts/truetype/arphic/uming.ttc",
			"/usr/share/fonts/opentype/source-han-sans/SourceHanSansCN-Regular.otf",
		}
	}
}

func screenshotEditorWindowsDirectory() string {
	if windir := strings.TrimSpace(os.Getenv("WINDIR")); windir != "" {
		return windir
	}
	return `C:\Windows`
}

func screenshotEditorFontconfigFiles() []string {
	queries := []string{
		"Noto Sans CJK SC",
		"Source Han Sans SC",
		"WenQuanYi Micro Hei",
		"Noto Sans CJK",
		"Droid Sans Fallback",
		":lang=zh-cn",
		":lang=ja",
		":lang=ko",
	}
	files := make([]string, 0, len(queries))
	for _, query := range queries {
		path, err := screenshotEditorFontconfigFile(query)
		if err != nil || path == "" {
			continue
		}
		files = append(files, path)
	}
	return files
}

func screenshotEditorFontconfigFile(query string) (string, error) {
	output, err := exec.Command("fc-match", "-f", "%{file}\n", query).Output()
	if err != nil {
		return "", err
	}
	path, _, _ := strings.Cut(strings.TrimSpace(string(output)), "\n")
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("empty fc-match path")
	}
	return path, nil
}

// pickScreenshotEditorCJKFont chooses one CJK face from a TTF/OTF/TTC file.
// Noto Sans CJK collections include JP, KR, SC, TC, and mono faces; SC ranks highest.
func pickScreenshotEditorCJKFont(path string) (*opentype.Font, int) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, -1
	}
	collection, err := opentype.ParseCollection(data)
	if err != nil {
		return nil, -1
	}
	var buf sfnt.Buffer
	var best *opentype.Font
	bestScore := -1
	for index := 0; index < collection.NumFonts(); index++ {
		parsed, err := collection.Font(index)
		if err != nil || parsed == nil || !screenshotEditorFontHasRune(parsed, '中') {
			continue
		}
		family, _ := parsed.Name(&buf, sfnt.NameIDFamily)
		score := screenshotEditorCJKFontScore(family)
		if score > bestScore {
			best = parsed
			bestScore = score
		}
	}
	return best, bestScore
}

func screenshotEditorFontHasRune(parsed *opentype.Font, r rune) bool {
	if parsed == nil {
		return false
	}
	var buf sfnt.Buffer
	index, err := parsed.GlyphIndex(&buf, r)
	return err == nil && index != 0
}

func screenshotEditorFontsHaveRune(fonts []*opentype.Font, r rune) bool {
	for _, parsed := range fonts {
		if screenshotEditorFontHasRune(parsed, r) {
			return true
		}
	}
	return false
}

// screenshotEditorCJKFontScore ranks CJK family names, preferring Simplified Chinese UI faces.
func screenshotEditorCJKFontScore(family string) int {
	family = strings.ToLower(strings.TrimSpace(family))
	if family == "" {
		return 10
	}
	if strings.Contains(family, "mono") {
		return 1
	}
	switch {
	case strings.Contains(family, "cjk sc"), strings.Contains(family, "hans"), strings.Contains(family, "simplified"):
		return screenshotEditorPreferredCJKScore
	case strings.Contains(family, "yahei"), strings.Contains(family, "pingfang sc"), strings.Contains(family, "stheiti"), strings.Contains(family, "source han sans sc"):
		return 95
	case strings.Contains(family, "wenquanyi"), strings.Contains(family, "micro hei"), strings.Contains(family, "droid sans fallback"):
		return 80
	case strings.Contains(family, "cjk tc"), strings.Contains(family, "pingfang tc"), strings.Contains(family, "jhenghei"), strings.Contains(family, "hant"):
		return 70
	case strings.Contains(family, "cjk jp"), strings.Contains(family, "hiragino"), strings.Contains(family, "yu gothic"):
		return 60
	case strings.Contains(family, "cjk kr"), strings.Contains(family, "malgun"):
		return 50
	case strings.Contains(family, "pingfang"), strings.Contains(family, "noto sans cjk"), strings.Contains(family, "source han"), strings.Contains(family, "arial unicode"):
		return 40
	default:
		return 10
	}
}

// drawScreenshotEditorPixelTextWithFonts paints each rune with the first face that has a real glyph.
func drawScreenshotEditorPixelTextWithFonts(target *image.RGBA, clip image.Rectangle, text string, position image.Point, size float32, textColor color.RGBA, fonts []*opentype.Font) {
	if text == "" || target == nil || len(fonts) == 0 {
		return
	}
	faces := make([]screenshotEditorExportFace, 0, len(fonts))
	for _, parsed := range fonts {
		if parsed == nil {
			continue
		}
		face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: float64(size), DPI: 72, Hinting: font.HintingFull})
		if err != nil {
			continue
		}
		defer face.Close()
		faces = append(faces, screenshotEditorExportFace{font: parsed, face: face})
	}
	if len(faces) == 0 {
		return
	}
	clipped := target.SubImage(clip.Intersect(target.Bounds())).(*image.RGBA)
	drawer := font.Drawer{Dst: clipped, Src: image.NewUniform(textColor), Face: faces[0].face, Dot: fixed.P(position.X, position.Y+int(size))}
	var glyphBuf sfnt.Buffer
	var utfBuf [utf8.UTFMax]byte
	for _, r := range text {
		drawer.Face = screenshotEditorFaceForRune(r, faces, &glyphBuf)
		n := utf8.EncodeRune(utfBuf[:], r)
		drawer.DrawBytes(utfBuf[:n])
	}
}

func screenshotEditorFaceForRune(r rune, faces []screenshotEditorExportFace, buf *sfnt.Buffer) font.Face {
	for _, item := range faces {
		index, err := item.font.GlyphIndex(buf, r)
		if err == nil && index != 0 {
			return item.face
		}
	}
	return faces[0].face
}
