package dictationoverlay

import "testing"

func TestDictationWaveformWidthReservesCloseButtonAfterBalancedPadding(t *testing.T) {
	closableWidth := float32(dictationOverlayWidth + dictationOverlayCloseReserve)
	if width := dictationWaveformWidth(closableWidth, true); width != dictationOverlayWidth {
		t.Fatalf("closable waveform width = %v, want %v", width, dictationOverlayWidth)
	}
	if width := dictationWaveformWidth(dictationOverlayWidth, false); width != dictationOverlayWidth {
		t.Fatalf("plain waveform width = %v, want %v", width, dictationOverlayWidth)
	}
}
