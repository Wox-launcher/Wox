package setting

import "testing"

func TestUiDensityScaleBuckets(t *testing.T) {
	if UiDensityScale(UiDensityCompact) != 0.9 || UiDensityScale(UiDensityNormal) != 1 || UiDensityScale(UiDensityComfortable) != 1.1 {
		t.Fatalf("scales = %v/%v/%v", UiDensityScale(UiDensityCompact), UiDensityScale(UiDensityNormal), UiDensityScale(UiDensityComfortable))
	}
	if UiDensityScale("oversized") != 1 {
		t.Fatal("unknown density must stay on the normal scale")
	}
}

func TestScaleUiDensityRoundsAuthoredSizes(t *testing.T) {
	if got := ScaleUiDensity(13, UiDensityNormal); got != 13 {
		t.Fatalf("normal size = %v, want 13", got)
	}
	if got := ScaleUiDensity(13, UiDensityComfortable); got != 14 {
		t.Fatalf("comfortable size = %v, want 14", got)
	}
	if got := ScaleUiDensity(13, UiDensityCompact); got != 12 {
		t.Fatalf("compact size = %v, want 12", got)
	}
}

func TestPeekUiDensityBeforeInitIsNormal(t *testing.T) {
	if PeekUiDensity() != UiDensityNormal || CurrentUiDensityScale() != 1 {
		t.Fatalf("uninitialized density = %s/%v", PeekUiDensity(), CurrentUiDensityScale())
	}
}
