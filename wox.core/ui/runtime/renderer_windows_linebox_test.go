//go:build windows

package woxui

import (
	"math"
	"testing"
)

// TestDefaultUIFontKeepsMixedScriptBaseline guards the shared UI line box.
// Han used to adopt Microsoft YaHei UI's shorter ascent, and Hangul adopted
// Malgun Gothic's taller one, so the baseline moved as Latin, Kana, or another
// script joined the line.
func TestDefaultUIFontKeepsMixedScriptBaseline(t *testing.T) {
	samples := []string{"Ag", "中", "Ag中", "你", "你ni", "你一", "こんにちは", "helloこんにちは", "日本語", "日本語です", "안녕", "hello안녕", "안녕하세요", "ｶﾀｶﾅ"}
	for _, size := range []float32{13, 28} {
		for _, weight := range []uint8{0, 1} {
			first, result := measureDefaultUITextForTest(samples[0], size, weight)
			if result != 0 || first.Size.Width <= 0 || first.Size.Height <= 0 || first.Baseline <= 0 {
				t.Fatalf("size %v weight %d %q = %+v result %d", size, weight, samples[0], first, result)
			}
			for _, sample := range samples[1:] {
				got, result := measureDefaultUITextForTest(sample, size, weight)
				if result != 0 {
					t.Fatalf("size %v weight %d %q result %d", size, weight, sample, result)
				}
				if math.Abs(float64(got.Baseline-first.Baseline)) > 0.05 || math.Abs(float64(got.Size.Height-first.Size.Height)) > 0.05 {
					t.Fatalf("size %v weight %d %q baseline/height = %v/%v, want %v/%v", size, weight, sample, got.Baseline, got.Size.Height, first.Baseline, first.Size.Height)
				}
				if got.Size.Width <= 0 {
					t.Fatalf("size %v weight %d %q width = %v", size, weight, sample, got.Size.Width)
				}
			}
		}
	}
}
