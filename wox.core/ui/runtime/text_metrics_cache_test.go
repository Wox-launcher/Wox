package woxui

import (
	"errors"
	"strings"
	"testing"
)

func TestTextMetricsCacheHitAndEviction(t *testing.T) {
	cache := newTextMetricsCache(2, textMetricsCacheMaxBytes)
	keyA := textMetricsCacheKey{text: "a", size: 12, weight: FontWeightRegular}
	keyB := textMetricsCacheKey{text: "b", size: 12, weight: FontWeightRegular}
	keyC := textMetricsCacheKey{text: "c", size: 12, weight: FontWeightRegular}
	metricsA := TextMetrics{Size: Size{Width: 1, Height: 12}, Baseline: 10}
	metricsB := TextMetrics{Size: Size{Width: 2, Height: 12}, Baseline: 10}
	metricsC := TextMetrics{Size: Size{Width: 3, Height: 12}, Baseline: 10}

	cache.put(keyA, metricsA)
	cache.put(keyB, metricsB)
	if got, ok := cache.get(keyA); !ok || got != metricsA {
		t.Fatalf("get(A) = (%v, %t), want (%v, true)", got, ok, metricsA)
	}

	cache.put(keyC, metricsC)
	if cache.len() != 2 {
		t.Fatalf("len = %d, want 2", cache.len())
	}
	if _, ok := cache.get(keyB); ok {
		t.Fatal("B should have been evicted after A was refreshed")
	}
	if got, ok := cache.get(keyA); !ok || got != metricsA {
		t.Fatalf("get(A) after eviction = (%v, %t), want (%v, true)", got, ok, metricsA)
	}
	if got, ok := cache.get(keyC); !ok || got != metricsC {
		t.Fatalf("get(C) = (%v, %t), want (%v, true)", got, ok, metricsC)
	}
}

func TestTextMetricsCacheKeysIncludeStyleAndFamily(t *testing.T) {
	cache := newTextMetricsCache(8, textMetricsCacheMaxBytes)
	base := textMetricsCacheKey{text: "Wox", size: 14, weight: FontWeightRegular, family: "Inter"}
	cache.put(base, TextMetrics{Size: Size{Width: 10, Height: 14}})

	variants := []textMetricsCacheKey{
		{text: "Wox", size: 15, weight: FontWeightRegular, family: "Inter"},
		{text: "Wox", size: 14, weight: FontWeightSemibold, family: "Inter"},
		{text: "Wox", size: 14, weight: FontWeightRegular, family: "JetBrains Mono"},
		{text: "Wox", size: 14, weight: FontWeightRegular, family: "Inter", kind: FontFamilyMonospace},
		{text: "Wox", size: 14, weight: FontWeightRegular, family: "Inter", italic: true},
		{text: "WOX", size: 14, weight: FontWeightRegular, family: "Inter"},
	}
	for _, key := range variants {
		if _, ok := cache.get(key); ok {
			t.Fatalf("unexpected hit for %#v", key)
		}
	}
	if got, ok := cache.get(base); !ok || got.Size.Width != 10 {
		t.Fatalf("base key miss: got (%v, %t)", got, ok)
	}
}

func TestTextMetricsCacheSkipsOversizedText(t *testing.T) {
	cache := newTextMetricsCache(8, textMetricsCacheMaxBytes)
	longText := strings.Repeat("x", textMetricsCacheMaxTextBytes+1)
	key := textMetricsCacheKey{text: longText, size: 12, weight: FontWeightRegular}
	cache.put(key, TextMetrics{Size: Size{Width: 99, Height: 12}})
	if cache.len() != 0 {
		t.Fatalf("oversized text should not be cached, len = %d", cache.len())
	}
	if _, ok := cache.get(key); ok {
		t.Fatal("oversized text should miss")
	}
}

func TestTextMetricsCacheEvictsByByteBudget(t *testing.T) {
	cache := newTextMetricsCache(32, 20)
	keyA := textMetricsCacheKey{text: strings.Repeat("a", 12), size: 12}
	keyB := textMetricsCacheKey{text: strings.Repeat("b", 12), size: 12}
	cache.put(keyA, TextMetrics{Size: Size{Width: 1}})
	cache.put(keyB, TextMetrics{Size: Size{Width: 2}})
	if cache.len() != 1 {
		t.Fatalf("len = %d, want 1 after byte-budget eviction", cache.len())
	}
	if cache.byteSize() > 20 {
		t.Fatalf("byteSize = %d, want <= 20", cache.byteSize())
	}
	if _, ok := cache.get(keyA); ok {
		t.Fatal("A should have been evicted by byte budget")
	}
	if got, ok := cache.get(keyB); !ok || got.Size.Width != 2 {
		t.Fatalf("get(B) = (%v, %t), want width 2", got, ok)
	}
}

func TestMeasureTextUsesCacheAndRespectsClosedWindow(t *testing.T) {
	previous := globalTextMetricsCache
	globalTextMetricsCache = newTextMetricsCache(32, textMetricsCacheMaxBytes)
	t.Cleanup(func() { globalTextMetricsCache = previous })

	calls := 0
	window := &Window{
		measureTextFn: func(text string, style TextStyle) (TextMetrics, error) {
			calls++
			return TextMetrics{Size: Size{Width: float32(len(text)) * style.Size, Height: style.Size}, Baseline: style.Size * 0.8}, nil
		},
	}

	style := TextStyle{Size: 12, Weight: FontWeightRegular}
	first, err := window.MeasureText("hello", style)
	if err != nil {
		t.Fatalf("first MeasureText: %v", err)
	}
	second, err := window.MeasureText("hello", style)
	if err != nil {
		t.Fatalf("second MeasureText: %v", err)
	}
	if calls != 1 {
		t.Fatalf("native calls = %d, want 1 (second should hit cache)", calls)
	}
	if first != second {
		t.Fatalf("cached metrics mismatch: %#v vs %#v", first, second)
	}

	if err := window.SetFontFamily("Inter"); err != nil {
		t.Fatalf("SetFontFamily: %v", err)
	}
	if _, err := window.MeasureText("hello", style); err != nil {
		t.Fatalf("MeasureText after font change: %v", err)
	}
	if calls != 2 {
		t.Fatalf("native calls after font change = %d, want 2", calls)
	}

	if err := window.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := window.MeasureText("hello", style); !errors.Is(err, errWindowClosed) {
		t.Fatalf("MeasureText after Close error = %v, want closed", err)
	}
	if calls != 2 {
		t.Fatalf("closed window should not call native, calls = %d", calls)
	}
}

func TestMeasureTextDoesNotCacheErrorsOrOversizedText(t *testing.T) {
	previous := globalTextMetricsCache
	globalTextMetricsCache = newTextMetricsCache(32, textMetricsCacheMaxBytes)
	t.Cleanup(func() { globalTextMetricsCache = previous })

	calls := 0
	failOnce := true
	window := &Window{
		measureTextFn: func(text string, style TextStyle) (TextMetrics, error) {
			calls++
			if failOnce {
				failOnce = false
				return TextMetrics{}, errors.New("measure failed")
			}
			return TextMetrics{Size: Size{Width: float32(len(text)), Height: style.Size}}, nil
		},
	}

	if _, err := window.MeasureText("retry", TextStyle{Size: 10}); err == nil {
		t.Fatal("expected measure failure")
	}
	if _, err := window.MeasureText("retry", TextStyle{Size: 10}); err != nil {
		t.Fatalf("retry after failure: %v", err)
	}
	if calls != 2 {
		t.Fatalf("failed measurements must not be cached, calls = %d", calls)
	}

	longText := strings.Repeat("z", textMetricsCacheMaxTextBytes+1)
	before := calls
	if _, err := window.MeasureText(longText, TextStyle{Size: 10}); err != nil {
		t.Fatalf("oversized MeasureText: %v", err)
	}
	if _, err := window.MeasureText(longText, TextStyle{Size: 10}); err != nil {
		t.Fatalf("second oversized MeasureText: %v", err)
	}
	if calls != before+2 {
		t.Fatalf("oversized text must bypass cache, calls = %d, want %d", calls, before+2)
	}
}

func TestMeasureTextCloseFailureRollsBackLifecycle(t *testing.T) {
	previous := globalTextMetricsCache
	globalTextMetricsCache = newTextMetricsCache(32, textMetricsCacheMaxBytes)
	t.Cleanup(func() { globalTextMetricsCache = previous })

	calls := 0
	window := &Window{
		measureTextFn: func(text string, style TextStyle) (TextMetrics, error) {
			calls++
			return TextMetrics{Size: Size{Width: float32(len(text)), Height: style.Size}}, nil
		},
		closeFn: func() error {
			return errors.New("native close failed")
		},
	}

	style := TextStyle{Size: 11}
	if _, err := window.MeasureText("keep", style); err != nil {
		t.Fatalf("warm MeasureText: %v", err)
	}
	if err := window.Close(); err == nil {
		t.Fatal("expected Close failure")
	}
	if window.isClosed() {
		t.Fatal("failed Close must not leave window closed")
	}
	if windowLifecycle(window.lifecycle.Load()) != windowLifecycleOpen {
		t.Fatalf("lifecycle = %d, want open after failed Close", window.lifecycle.Load())
	}

	before := calls
	if _, err := window.MeasureText("keep", style); err != nil {
		t.Fatalf("MeasureText after failed Close: %v", err)
	}
	if calls != before {
		t.Fatalf("cache should still hit after failed Close, calls = %d", calls)
	}
}

func TestMeasureTextRejectsCacheAfterNativeOnClosed(t *testing.T) {
	previous := globalTextMetricsCache
	globalTextMetricsCache = newTextMetricsCache(32, textMetricsCacheMaxBytes)
	t.Cleanup(func() { globalTextMetricsCache = previous })

	userClosed := false
	calls := 0
	window := &Window{
		measureTextFn: func(text string, style TextStyle) (TextMetrics, error) {
			calls++
			return TextMetrics{Size: Size{Width: float32(len(text)), Height: style.Size}}, nil
		},
		userOnClosed: func() { userClosed = true },
	}

	style := TextStyle{Size: 11}
	if _, err := window.MeasureText("native", style); err != nil {
		t.Fatalf("warm MeasureText: %v", err)
	}

	// Simulate Linux/macOS external destroy → platformWindow.markClosed → OnClosed.
	window.handleNativeClosed()
	if !userClosed {
		t.Fatal("user OnClosed should run after native closed callback")
	}
	if !window.isClosed() {
		t.Fatal("native OnClosed should mark wrapper closed")
	}

	before := calls
	if _, err := window.MeasureText("native", style); !errors.Is(err, errWindowClosed) {
		t.Fatalf("MeasureText after native OnClosed = %v, want closed", err)
	}
	if calls != before {
		t.Fatalf("closed window must not call measurer, calls = %d", calls)
	}
}

func TestMeasureTextRejectsCacheWhileClosing(t *testing.T) {
	previous := globalTextMetricsCache
	globalTextMetricsCache = newTextMetricsCache(32, textMetricsCacheMaxBytes)
	t.Cleanup(func() { globalTextMetricsCache = previous })

	calls := 0
	enteredClosing := make(chan struct{})
	releaseClose := make(chan struct{})
	window := &Window{
		measureTextFn: func(text string, style TextStyle) (TextMetrics, error) {
			calls++
			return TextMetrics{Size: Size{Width: float32(len(text)), Height: style.Size}}, nil
		},
		closeFn: func() error {
			close(enteredClosing)
			<-releaseClose
			return nil
		},
	}

	style := TextStyle{Size: 11}
	if _, err := window.MeasureText("closing", style); err != nil {
		t.Fatalf("warm MeasureText: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- window.Close() }()
	<-enteredClosing
	if windowLifecycle(window.lifecycle.Load()) != windowLifecycleClosing {
		t.Fatalf("lifecycle = %d, want closing", window.lifecycle.Load())
	}

	before := calls
	if _, err := window.MeasureText("closing", style); !errors.Is(err, errWindowClosed) {
		t.Fatalf("MeasureText while closing = %v, want closed", err)
	}
	if err := window.SetFontFamily("Inter"); !errors.Is(err, errWindowClosed) {
		t.Fatalf("SetFontFamily while closing = %v, want closed", err)
	}
	if calls != before {
		t.Fatalf("closing window must not call measurer, calls = %d", calls)
	}

	close(releaseClose)
	if err := <-done; err != nil {
		t.Fatalf("Close: %v", err)
	}
}
