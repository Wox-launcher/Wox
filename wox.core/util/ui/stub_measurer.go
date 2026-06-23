package ui

// stubMeasurer is a fallback TextMeasurer that estimates text size
// without a native shaping engine. It uses a rough heuristic:
// each rune is ~0.6× fontSize wide, height ≈ fontSize × 1.2.
//
// This is only used when no native backend is available (e.g. running
// tests on a headless machine). Production code always supplies a
// real TextMeasurer from the platform renderer.
type stubMeasurer struct{}

func (stubMeasurer) MeasureText(text string, fontSize float32, fontFamily string) (width, height float32) {
	runeCount := float32(0)
	for range text {
		runeCount++
	}
	width = runeCount * fontSize * 0.6
	height = fontSize * 1.2
	return
}

// NewStubMeasurer returns a heuristic-based TextMeasurer for testing.
func NewStubMeasurer() TextMeasurer { return stubMeasurer{} }