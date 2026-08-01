//go:build wox_ui_smoke

package gouismoke

import (
	"encoding/hex"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"strings"
	"testing"

	"golang.org/x/image/draw"
)

const (
	visualHashWidth       = 16
	visualHashHeight      = 16
	visualHashTolerance   = 64
	visualMeanTolerance   = 25
	visualSigmaTolerance  = 20
	visualAspectTolerance = 0.08
)

type visualSignature struct {
	Hash   string
	MeanR  float64
	MeanG  float64
	MeanB  float64
	Sigma  float64
	Aspect float64
}

var launcherQueryGolden = visualSignature{
	Hash:   "00000000c000c00000000000ffffffffffffffffffffffff00000000001e0000",
	MeanR:  63.9296875,
	MeanG:  63.94140625,
	MeanB:  66.48828125,
	Sigma:  15.224796975182672,
	Aspect: 4.245810055865922,
}

// assertVisualGolden compares normalized composition while tolerating native font and scale differences.
func assertVisualGolden(t *testing.T, path string) {
	t.Helper()
	actual, err := readVisualSignature(path)
	if err != nil {
		t.Fatalf("read launcher visual signature: %v", err)
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("WOX_UPDATE_GO_UI_GOLDEN")), "true") {
		t.Logf("launcher query golden: %+v", actual)
		return
	}
	if launcherQueryGolden.Hash == "" {
		t.Fatal("launcher query visual golden is not configured")
	}
	hashDistance, err := hashDistance(actual.Hash, launcherQueryGolden.Hash)
	if err != nil {
		t.Fatalf("compare launcher visual hash: %v", err)
	}
	if hashDistance > visualHashTolerance ||
		math.Abs(actual.MeanR-launcherQueryGolden.MeanR) > visualMeanTolerance ||
		math.Abs(actual.MeanG-launcherQueryGolden.MeanG) > visualMeanTolerance ||
		math.Abs(actual.MeanB-launcherQueryGolden.MeanB) > visualMeanTolerance ||
		math.Abs(actual.Sigma-launcherQueryGolden.Sigma) > visualSigmaTolerance ||
		math.Abs(actual.Aspect-launcherQueryGolden.Aspect) > visualAspectTolerance {
		t.Fatalf("launcher visual changed beyond tolerance: hash_distance=%d actual=%+v golden=%+v", hashDistance, actual, launcherQueryGolden)
	}
}

func readVisualSignature(path string) (visualSignature, error) {
	file, err := os.Open(path)
	if err != nil {
		return visualSignature{}, err
	}
	source, err := png.Decode(file)
	_ = file.Close()
	if err != nil {
		return visualSignature{}, err
	}
	bounds := source.Bounds()
	if bounds.Empty() {
		return visualSignature{}, fmt.Errorf("visual image is empty")
	}
	normalized := image.NewRGBA(image.Rect(0, 0, visualHashWidth, visualHashHeight))
	draw.CatmullRom.Scale(normalized, normalized.Bounds(), source, bounds, draw.Src, nil)
	gray := make([]float64, 0, visualHashWidth*visualHashHeight)
	meanR := 0.0
	meanG := 0.0
	meanB := 0.0
	for y := 0; y < visualHashHeight; y++ {
		for x := 0; x < visualHashWidth; x++ {
			r, g, b, _ := normalized.At(x, y).RGBA()
			red := float64(r >> 8)
			green := float64(g >> 8)
			blue := float64(b >> 8)
			meanR += red
			meanG += green
			meanB += blue
			gray = append(gray, red*0.299+green*0.587+blue*0.114)
		}
	}
	count := float64(len(gray))
	meanR /= count
	meanG /= count
	meanB /= count
	meanGray := 0.0
	for _, value := range gray {
		meanGray += value
	}
	meanGray /= count
	variance := 0.0
	bits := make([]byte, (len(gray)+7)/8)
	for index, value := range gray {
		delta := value - meanGray
		variance += delta * delta
		if value >= meanGray {
			bits[index/8] |= 1 << uint(7-index%8)
		}
	}
	return visualSignature{
		Hash:   hex.EncodeToString(bits),
		MeanR:  meanR,
		MeanG:  meanG,
		MeanB:  meanB,
		Sigma:  math.Sqrt(variance / count),
		Aspect: float64(bounds.Dx()) / float64(bounds.Dy()),
	}, nil
}

func hashDistance(left, right string) (int, error) {
	leftBytes, err := hex.DecodeString(left)
	if err != nil {
		return 0, err
	}
	rightBytes, err := hex.DecodeString(right)
	if err != nil {
		return 0, err
	}
	if len(leftBytes) != len(rightBytes) {
		return 0, fmt.Errorf("visual hashes have different lengths")
	}
	distance := 0
	for index := range leftBytes {
		value := leftBytes[index] ^ rightBytes[index]
		for value != 0 {
			distance++
			value &= value - 1
		}
	}
	return distance, nil
}
