package common

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"wox/util"

	"github.com/disintegration/imaging"
	"github.com/google/uuid"
)

func TestConvertIconWithSizeMaybeLazyDefersLargeRasterIcon(t *testing.T) {
	initConvertIconTestLocation(t)
	sourcePath := writeTestImage(t, 640, 640)

	converted := ConvertIconWithSizeMaybeLazy(context.Background(), NewWoxImageAbsolutePath(sourcePath), "", ResultListIconSize)

	if converted.ImageType != WoxImageTypeLazyLoad {
		t.Fatalf("expected lazyloadimage, got %+v", converted)
	}
	payload, err := ParseWoxLazyLoadImagePayload(converted)
	if err != nil {
		t.Fatalf("parse lazy payload: %v", err)
	}
	if payload.Source == nil || payload.Source.ImageType != WoxImageTypeAbsolutePath || payload.Source.ImageData != sourcePath {
		t.Fatalf("unexpected lazy source: %+v", payload.Source)
	}
	if payload.Token != "" {
		t.Fatalf("common lazy marker should not allocate token, got %q", payload.Token)
	}
}

func TestWoxImageIsGifDetectsPathAndBase64(t *testing.T) {
	tests := []struct {
		name  string
		image WoxImage
		want  bool
	}{
		{name: "absolute gif", image: NewWoxImageAbsolutePath(`C:\icons\loading.gif`), want: true},
		{name: "url with query", image: NewWoxImageUrl("https://example.com/icon.gif?size=32"), want: true},
		{name: "base64 gif", image: NewWoxImageBase64("data:image/gif;base64,R0lGODlh"), want: true},
		{name: "png path", image: NewWoxImageAbsolutePath(`C:\icons\app.png`), want: false},
		{name: "empty", image: WoxImage{}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.image.IsGif(); got != test.want {
				t.Fatalf("IsGif() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestConvertIconPreservesGifAbsolutePath(t *testing.T) {
	initConvertIconTestLocation(t)
	sourcePath := writeTestGIF(t)

	converted := ConvertIconWithSize(context.Background(), NewWoxImageAbsolutePath(sourcePath), "", ResultListIconSize)

	if converted.ImageType != WoxImageTypeAbsolutePath || converted.ImageData != sourcePath {
		t.Fatalf("expected original GIF path, got %+v", converted)
	}
}

func TestParseWoxImageRejectsRemovedLottieType(t *testing.T) {
	if _, err := ParseWoxImage("lottie:{}"); err == nil {
		t.Fatal("expected removed lottie image type to be rejected")
	}
}

func TestConvertIconWithSizeMaybeLazyKeepsSmallRasterSynchronous(t *testing.T) {
	initConvertIconTestLocation(t)
	sourcePath := writeTestImage(t, 64, 64)

	converted := ConvertIconWithSizeMaybeLazy(context.Background(), NewWoxImageAbsolutePath(sourcePath), "", ResultListIconSize)

	if converted.ImageType != WoxImageTypeAbsolutePath || converted.ImageData == "" {
		t.Fatalf("expected converted absolute icon, got %+v", converted)
	}
}

func TestConvertIconWithSizeMaybeLazyUsesWarmResizeCacheForLargeRaster(t *testing.T) {
	initConvertIconTestLocation(t)
	sourcePath := writeTestImage(t, 640, 640)
	woxImage := NewWoxImageAbsolutePath(sourcePath)
	warmConverted := ConvertIconWithSize(context.Background(), woxImage, "", ResultListIconSize)
	if warmConverted.ImageType != WoxImageTypeAbsolutePath || warmConverted.ImageData == "" {
		t.Fatalf("failed to warm resize cache: %+v", warmConverted)
	}

	converted := ConvertIconWithSizeMaybeLazy(context.Background(), woxImage, "", ResultListIconSize)

	if converted.ImageType == WoxImageTypeLazyLoad {
		t.Fatal("expected warm resize cache to bypass lazy loading")
	}
	if converted.ImageData != warmConverted.ImageData {
		t.Fatalf("expected warm resize cache path %q, got %q", warmConverted.ImageData, converted.ImageData)
	}
}

func TestConvertIconWithSizeMaterializesRemoteGIF(t *testing.T) {
	initConvertIconTestLocation(t)
	requestCount := 0
	gifBytes := encodedTestGIF(t)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requestCount++
		response.Header().Set("Content-Type", "image/gif")
		if _, err := response.Write(gifBytes); err != nil {
			t.Errorf("write remote gif: %v", err)
		}
	}))
	defer server.Close()

	converted := ConvertIconWithSize(context.Background(), NewWoxImageUrl(server.URL+"/doge.gif"), "", ResultListIconSize)

	if requestCount != 1 {
		t.Fatalf("remote GIF conversion made %d requests, want 1", requestCount)
	}
	if converted.ImageType != WoxImageTypeAbsolutePath || !converted.IsAnimatedGif() {
		t.Fatalf("expected cached GIF file, got %+v", converted)
	}
	if _, err := os.Stat(converted.ImageData); err != nil {
		t.Fatalf("cached GIF file missing: %v", err)
	}
}

func TestConvertIconWithSizeMaybeLazyDefersRemoteGIFWithoutRequest(t *testing.T) {
	initConvertIconTestLocation(t)
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requestCount++
	}))
	defer server.Close()

	converted := ConvertIconWithSizeMaybeLazy(context.Background(), NewWoxImageUrl(server.URL+"/doge.gif"), "", ResultListIconSize)

	if requestCount != 0 {
		t.Fatalf("remote GIF polish made %d synchronous requests", requestCount)
	}
	if converted.ImageType != WoxImageTypeLazyLoad {
		t.Fatalf("expected remote GIF lazy marker, got %+v", converted)
	}
}

func TestConvertIconWithSizeMaybeLazyDefersRemoteURLWithoutRequest(t *testing.T) {
	initConvertIconTestLocation(t)
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requestCount++
	}))
	defer server.Close()

	converted := ConvertIconWithSizeMaybeLazy(context.Background(), NewWoxImageUrl(server.URL+"/icon.png"), "", ResultListIconSize)

	if requestCount != 0 {
		t.Fatalf("remote URL conversion made %d synchronous requests", requestCount)
	}
	if converted.ImageType != WoxImageTypeLazyLoad {
		t.Fatalf("expected remote URL lazy marker, got %+v", converted)
	}
	payload, err := ParseWoxLazyLoadImagePayload(converted)
	if err != nil {
		t.Fatalf("parse lazy payload: %v", err)
	}
	if payload.Source == nil || payload.Source.ImageType != WoxImageTypeUrl || payload.Source.ImageData != server.URL+"/icon.png" {
		t.Fatalf("unexpected lazy URL source: %+v", payload.Source)
	}
}

func TestConvertIconWithSizeMaybeLazyUsesWarmResizeCacheForRemoteURL(t *testing.T) {
	initConvertIconTestLocation(t)
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requestCount++
		response.Header().Set("Content-Type", "image/png")
		if err := png.Encode(response, image.NewRGBA(image.Rect(0, 0, 64, 64))); err != nil {
			t.Errorf("encode remote image: %v", err)
		}
	}))
	defer server.Close()
	source := NewWoxImageUrl(server.URL + "/icon.png")

	warmConverted := ConvertIconWithSize(context.Background(), source, "", ResultListIconSize)
	if warmConverted.ImageType != WoxImageTypeAbsolutePath {
		t.Fatalf("failed to warm remote resize cache: %+v", warmConverted)
	}
	converted := ConvertIconWithSizeMaybeLazy(context.Background(), source, "", ResultListIconSize)

	if converted.ImageType == WoxImageTypeLazyLoad {
		t.Fatal("warm remote resize cache should bypass lazy loading")
	}
	if converted.ImageData != warmConverted.ImageData {
		t.Fatalf("expected warm resize cache path %q, got %q", warmConverted.ImageData, converted.ImageData)
	}
	if requestCount != 1 {
		t.Fatalf("warm remote URL made %d requests, want 1", requestCount)
	}
}

func TestWoxImage_ToImage(t *testing.T) {
	errIcon := NewWoxImageBase64(`data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADAAAAAwCAYAAABXAvmHAAAABHNCSVQICAgIfAhkiAAAAAlwSFlzAAAOxAAADsQBlSsOGwAABmlJREFUaIHtmntMW9cdxz/HGK6D7QQwwzgZGJukTYCuDmh5jHWPtlOSJlpXJeuyMk1dmNaQrmojTZu2VKvUpOqmSWvRJmg3VdUezTK2SqvWoqHuoalDhDQBJzxCUnBi6Hgtxibgx8WPuz+MHUJCUl8ITqR+/7u/8zv3fr/3nPO7v9+5R3Aj1NdL5K7X3tDnVsPbF+Hpp+WFmsU1lqa3bCjhgyB2IpQiFDJuKcGbQRBFEUOgvEOEl6nZ47q6eS6ONT2BJuMlYMVyckwBQWIcZO/uVxOGKwKONT2B0DQixLWjcjtBURQUUZcQESfb9JYNIj3cvm9+PoKElQpq9rg0APE5f8eQB1iBlmcA4gIQO9PJRh3inAX19RKWNVMgMtNNKUWEGfnQqCF3vTbtoVINFCUDiyVTk24ei8XHAtKNW5bnFEgSBZIEwLgsMy4vmM4sCksqoNpkYp/NyvZCMxad7qq2kVCIltExXrvgptXjWbJnLokAu15PQ6WDraY8+qenyZckvnemi/KVKwE4PzXNkYoyHDk5NN+3hjbPBAc6nLj8/kU/e9FrYEehmVMP3s9FfwDHu/+gQNLx7ZMd/Px8P4FolEA0yk/PneepztNYVuio+vs/uegPcOrB+3mo0JxeATsKzfxp62ae7HSyv6OTb5VYOT3p43fuwWt8X3FdoHvyMrW2EvZ3dPJkp5OmrZsXLUK1ALtezx+2bOLZ7l6ODg4haTTUldp54ey5Bfs833uW79ht6DQajg4O8aOuHo5u2YRdr1dLQ/0aaKh04J2Z4cV7yvmMKQ93IMjlcITjnokF+7Re8uCbCXO4ogyzTsdXVq/GMyPTUOlg+3utyyeg2mRic14e9ua/YdVnU1NcxOMlVkxZWXge3oXL72csJHO30YBAUPZ5I/lZElZ9Niu1WmqKi/mN281n//Vv3AE//Tu2UW0yqYpOqgTss1k5OjiINxzG65vE6ZtkR2Ehh3v7aPNMUKLPJl/KIiezGCHg7eFRxmWZc1PTVOXm8A1rMT/s6kne7/WLbmpt1uUTsL3QzDdPnExeC2CtQY/T5+N9r5f3vV4AymbD6Esf9Cd9Y4rC8+UbrrrfX4dHOLZlkxoqqS/ixBe2fcKbtOVkZpKl0TASuvnXdjQUwiRJZMypXE/7JjHrdMkvdypIeQQKJImwovDiPeVJm1GrRQjBD9bfRTAaTdo/l58PwC823pu06TQaMoSgodLBTCyWtCuKQoEkpZxyqAqjmfPqfjkWi9fays37JvYMdJqrHx3+KJ2vg5RHYFyWEUJwqKuHy5EIABlCUGsr4bmeXoZDoaRv4s0/1Xk6aVtnMFBrK+EZ5xm84TAABq2W/XabqoQv5REYl2XGQiHuzVmVtEUVhUuyTOG8BO56KNRJyNEovlnyAJvzclVnrKqiUMvoGF9ebeG9S1fC3gfT03w6LxeNgLuNRgokiarcHBQF6kptXJJn4vlSTg79037mTphH1qymZXRMDRV1Al674OYv1Vs4crYPa3Y2Xy8uolRvoLHSwVQkgjsQ4H+yjFnSoaCwy2LBrJMo1etZlZmJZ2aGn32qgjcGh3D7A9QUF7HrP23LJ6DV4+GU10fnlx6gQJJoGvovr7hcPF5iZW1zC4nYMn8NaICBh7bx5ofD2PV62h/4IsPBIO0TXtU1gupk7kCHk7ysLA5197Dv5Cl+0neO7IwMqvNNC/b5QsEnMGi1HOruYXdbO98/00VuVhYHOpxqaagX4PL7eez4CV6oKKemuIiZmELjgIsfl224rr8AnivbQOOACzkW46ufXMPh8jIeO35iUYXNouqB5tExHm1r55cbHfy6qpLfu4dYZzCw3267xve7a0ux6/W8ftFNY6WDX1VtZO/xEzSrXLwJCH7bokeauowQi6oNGiod3JdvYsDvZ4PRyLPdvdxlNADxCHWkohynz8c6g2FpSkpFiSFiq5ZEQALVJhO1NivblqOonxWwpLsSrR5Pktwdua0yF7eS9Fzc8TtzHwtINzR4+yIIojd3vc0gRJSRkXC8uvjjmwMI7GmmlBoUXHxtd+nsFFLeSS8bNYhzjguI8DIQTCedFBGc5TwroGaPixgHUVQWpsuJOMeDiSMHV6LQ3t2voog6bu+RCCJEHY9e76hBAm/82R7/iSx2IihCUdJ82ENEUfiIhz3mo75ewmJJ7//jkZHwjY7b/B/vpHHiBJxF3wAAAABJRU5ErkJggg==`)
	overlayIcon := NewWoxImageEmoji("🚫")
	icon := errIcon.OverlayFullPercentage(overlayIcon, 1)

	img, err := icon.ToImage()
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
		return
	}

	util.GetLocation().Init()
	path := fmt.Sprintf("%s/%s.png", util.GetLocation().GetImageCacheDirectory(), uuid.NewString())
	imaging.Save(img, path)
	t.Log(path)
}

func TestWoxImage_Emoji(t *testing.T) {
	util.GetLocation().Init()
	emojiImg := NewWoxImageEmoji("😀")
	img, err := emojiImg.ToImage()
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
		return
	}

	path := fmt.Sprintf("%s/%s.png", util.GetLocation().GetImageCacheDirectory(), uuid.NewString())
	imaging.Save(img, path)
	t.Log(path)
}

func TestWoxImageEmojiCodePointCandidates(t *testing.T) {
	tests := []struct {
		name  string
		emoji string
		want  []string
	}{
		{name: "variation selector fallback", emoji: "⚙️", want: []string{"2699-fe0f", "2699"}},
		{name: "single code point", emoji: "😀", want: []string{"1f600"}},
		{name: "ZWJ sequence", emoji: "👨‍👩‍👧‍👦", want: []string{"1f468-200d-1f469-200d-1f467-200d-1f466"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			icon := NewWoxImageEmoji(tt.emoji)
			got, err := icon.emojiImageCodePointCandidates(tt.emoji)
			if err != nil {
				t.Fatalf("emojiImageCodePointCandidates(%q): %v", tt.emoji, err)
			}
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Fatalf("emojiImageCodePointCandidates(%q) = %v, want %v", tt.emoji, got, tt.want)
			}
		})
	}
}

func TestWoxImage_ToImage_Base64JPEG(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	src.Set(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	src.Set(1, 0, color.RGBA{R: 0, G: 255, B: 0, A: 255})
	src.Set(0, 1, color.RGBA{R: 0, G: 0, B: 255, A: 255})
	src.Set(1, 1, color.RGBA{R: 255, G: 255, B: 0, A: 255})

	buf := bytes.NewBuffer(nil)
	if err := jpeg.Encode(buf, src, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("failed to encode jpeg: %v", err)
	}

	base64Jpeg := fmt.Sprintf("data:image/jpeg;base64,%s", base64.StdEncoding.EncodeToString(buf.Bytes()))
	woxImage := NewWoxImageBase64(base64Jpeg)
	img, err := woxImage.ToImage()
	if err != nil {
		t.Fatalf("expected jpeg base64 can be decoded, got err: %v", err)
	}

	if img.Bounds().Dx() != 2 || img.Bounds().Dy() != 2 {
		t.Fatalf("unexpected image size: %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestWoxImage_AbsolutePathSvg(t *testing.T) {
	svgPath := fmt.Sprintf("%s/icon.svg", t.TempDir())
	svgContent := `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 16 16"><rect width="16" height="16" fill="#ff0000"/></svg>`

	if err := os.WriteFile(svgPath, []byte(svgContent), 0644); err != nil {
		t.Fatalf("failed to write svg file: %v", err)
	}

	woxImage := NewWoxImageAbsolutePath(svgPath)

	img, err := woxImage.ToImage()
	if err != nil {
		t.Fatalf("expected svg absolute path can be decoded, got err: %v", err)
	}

	if img.Bounds().Dx() != 32 || img.Bounds().Dy() != 32 {
		t.Fatalf("unexpected rendered svg size: %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}

	pngImg, err := woxImage.ToPng()
	if err != nil {
		t.Fatalf("expected svg absolute path can be converted to png, got err: %v", err)
	}

	if pngImg.Bounds().Dx() != 32 || pngImg.Bounds().Dy() != 32 {
		t.Fatalf("unexpected png svg size: %dx%d", pngImg.Bounds().Dx(), pngImg.Bounds().Dy())
	}
}

func initConvertIconTestLocation(t *testing.T) {
	t.Helper()

	dataDir := t.TempDir()
	t.Setenv(util.TestWoxDataDirEnv, dataDir)
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(dataDir, "user"))
	if err := util.GetLocation().Init(); err != nil {
		t.Fatalf("init test location: %v", err)
	}
	ClearConvertIconPathExistenceCache()
}

func writeTestImage(t *testing.T, width int, height int) string {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 128, A: 255})
		}
	}

	filePath := filepath.Join(t.TempDir(), fmt.Sprintf("image-%dx%d.png", width, height))
	if err := imaging.Save(img, filePath); err != nil {
		t.Fatalf("write test image: %v", err)
	}
	return filePath
}

func writeTestGIF(t *testing.T) string {
	t.Helper()

	frame := image.NewPaletted(image.Rect(0, 0, 8, 8), color.Palette{color.RGBA{R: 255, A: 255}})
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			frame.SetColorIndex(x, y, 0)
		}
	}
	filePath := filepath.Join(t.TempDir(), "icon.gif")
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("create gif: %v", err)
	}
	defer file.Close()
	if err := gif.EncodeAll(file, &gif.GIF{
		Image:     []*image.Paletted{frame, frame},
		Delay:     []int{10, 10},
		LoopCount: 0,
		Config:    image.Config{ColorModel: frame.Palette, Width: 8, Height: 8},
	}); err != nil {
		t.Fatalf("encode gif: %v", err)
	}
	return filePath
}

func encodedTestGIF(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(writeTestGIF(t))
	if err != nil {
		t.Fatalf("read test gif: %v", err)
	}
	return data
}
