//go:build wox_ui_smoke

package query

import (
	"context"
	"encoding/hex"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/image/draw"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
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

// Test001LauncherQueryCalculator covers the native launcher query path and its visual baseline.
func Test001LauncherQueryCalculator(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		initialBounds, err := client.Bounds(ctx)
		if err != nil {
			t.Fatalf("read initial launcher bounds: %v", err)
		}
		movedBounds := initialBounds
		movedBounds.X += 37
		movedBounds.Y += 29
		if err := client.SetBounds(ctx, movedBounds); err != nil {
			t.Fatalf("move launcher before query: %v", err)
		}
		actualMovedBounds, err := client.Bounds(ctx)
		if err != nil {
			t.Fatalf("read moved launcher bounds: %v", err)
		}
		assertWindowOrigin(t, actualMovedBounds, movedBounds)
		for _, query := range []string{"s", "sm", "smo", "smok", "smoke"} {
			if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, query); err != nil {
				t.Fatalf("enter rapid query %q: %v", query, err)
			}
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			node, found := automationdriver.Find(snapshot, "launcher.query.input")
			return found && node.Value == "smoke"
		}); err != nil {
			t.Fatalf("wait for rapid query input: %v", err)
		}
		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, "1+1"); err != nil {
			t.Fatalf("enter calculator query: %v", err)
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, found := calculatorResult(snapshot)
			return found
		})
		if err != nil {
			t.Fatalf("wait for query result: %v", err)
		}
		if len(snapshot.Diagnostics) > 0 {
			t.Fatalf("launcher semantics diagnostics: %v", snapshot.Diagnostics)
		}
		resultBounds := waitForExpandedLauncher(t, ctx, client, initialBounds.Height)
		assertWindowOrigin(t, resultBounds, movedBounds)
		capturePath := smoke.ArtifactPath(t, "launcher-query-001-calculator")
		if err := client.Capture(ctx, capturePath); err != nil {
			t.Fatalf("capture launcher visual: %v", err)
		}
		smoke.AssertPNG(t, capturePath)
		assertVisualGolden(t, capturePath)
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("hide launcher: %v", err)
		}
	})
}

// waitForExpandedLauncher waits for native resize to catch up with the published result snapshot.
func waitForExpandedLauncher(t *testing.T, ctx context.Context, client *automationdriver.Client, initialHeight float32) woxui.Rect {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		bounds, err := client.Bounds(ctx)
		if err != nil {
			t.Fatalf("read launcher bounds after query: %v", err)
		}
		if bounds.Height > initialHeight+1 {
			return bounds
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for launcher result resize: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}

func assertWindowOrigin(t *testing.T, actual, expected woxui.Rect) {
	t.Helper()
	if math.Abs(float64(actual.X-expected.X)) > 1 || math.Abs(float64(actual.Y-expected.Y)) > 1 {
		t.Fatalf("launcher origin = %.1f,%.1f, want %.1f,%.1f", actual.X, actual.Y, expected.X, expected.Y)
	}
}

func calculatorResult(snapshot woxwidget.AutomationSnapshot) (string, bool) {
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") && strings.TrimSpace(node.Label) == "2" {
			return node.AutomationID, true
		}
	}
	return "", false
}

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
