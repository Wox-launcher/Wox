package webview

import (
	"errors"
	"testing"
)

type fakeDriver struct {
	shown      []Content
	hideCalls  int
	resetCalls int
	devTools   int
	closeCalls int
	showErr    error
}

func (d *fakeDriver) Show(content Content, bounds Rect, scale float32) error {
	d.shown = append(d.shown, content)
	return d.showErr
}

func (d *fakeDriver) Hide() error                               { d.hideCalls++; return nil }
func (d *fakeDriver) Reset() error                              { d.resetCalls++; return nil }
func (d *fakeDriver) GoBack() error                             { return nil }
func (d *fakeDriver) GoForward() error                          { return nil }
func (d *fakeDriver) Reload() error                             { return nil }
func (d *fakeDriver) OpenDevTools() error                       { d.devTools++; return nil }
func (d *fakeDriver) OpenInBrowser() error                      { return nil }
func (d *fakeDriver) NavigationState() (NavigationState, error) { return NavigationState{}, nil }
func (d *fakeDriver) Pointer(event PointerEvent) bool           { return true }
func (d *fakeDriver) Close()                                    { d.closeCalls++ }

func TestControllerOwnsVisibleLifecycle(t *testing.T) {
	driver := &fakeDriver{}
	controller := New(driver)
	if err := controller.Show(Content{URL: " https://example.com ", UserAgent: " ExampleBrowser/1.0 ", CacheKey: " cache "}, Rect{Width: 100, Height: 80}, 1); err != nil {
		t.Fatalf("show WebView: %v", err)
	}
	if !controller.Visible() || len(driver.shown) != 1 || driver.shown[0].URL != "https://example.com" || driver.shown[0].UserAgent != "ExampleBrowser/1.0" || driver.shown[0].CacheKey != "cache" {
		t.Fatalf("show state = visible %v content %+v", controller.Visible(), driver.shown)
	}
	if err := controller.Hide(); err != nil || controller.Visible() || driver.hideCalls != 1 {
		t.Fatalf("hide state = visible %v calls %d err %v", controller.Visible(), driver.hideCalls, err)
	}
	if err := controller.Show(Content{HTML: "<p>preview</p>"}, Rect{Width: 100, Height: 80}, 1); err != nil {
		t.Fatalf("show HTML WebView: %v", err)
	}
	if err := controller.Reset(); err != nil || controller.Visible() || driver.resetCalls != 1 {
		t.Fatalf("reset state = visible %v calls %d err %v", controller.Visible(), driver.resetCalls, err)
	}
	if err := controller.OpenDevTools(); err != nil || driver.devTools != 1 {
		t.Fatalf("open developer tools calls = %d err %v", driver.devTools, err)
	}
	controller.Close()
	if driver.closeCalls != 1 || controller.Visible() {
		t.Fatalf("close state = visible %v calls %d", controller.Visible(), driver.closeCalls)
	}
}

func TestControllerDoesNotPublishFailedShow(t *testing.T) {
	driver := &fakeDriver{showErr: errors.New("show failed")}
	controller := New(driver)
	if err := controller.Show(Content{URL: "https://example.com"}, Rect{Width: 100, Height: 80}, 1); err == nil {
		t.Fatal("show unexpectedly succeeded")
	}
	if controller.Visible() {
		t.Fatal("failed show marked the WebView visible")
	}
}

func TestIsAbsoluteURL(t *testing.T) {
	for _, test := range []struct {
		value string
		valid bool
	}{
		{value: "https://example.com/path", valid: true},
		{value: "http://localhost:8080", valid: true},
		{value: "example.com/path", valid: false},
		{value: "file:///tmp/index.html", valid: false},
		{value: "https:///missing-host", valid: false},
	} {
		if actual := IsAbsoluteURL(test.value); actual != test.valid {
			t.Fatalf("IsAbsoluteURL(%q) = %t, want %t", test.value, actual, test.valid)
		}
	}
}

func TestNormalizeClampsCornerRadiusToBounds(t *testing.T) {
	normalized, err := Normalize(Content{URL: "https://example.com", CornerRadius: 80}, Rect{Width: 100, Height: 40})
	if err != nil {
		t.Fatalf("normalize WebView: %v", err)
	}
	if normalized.CornerRadius != 20 {
		t.Fatalf("corner radius = %v, want 20", normalized.CornerRadius)
	}

	normalized, err = Normalize(Content{URL: "https://example.com", CornerRadius: -1}, Rect{Width: 100, Height: 40})
	if err != nil {
		t.Fatalf("normalize WebView with negative radius: %v", err)
	}
	if normalized.CornerRadius != 0 {
		t.Fatalf("negative corner radius normalized to %v, want 0", normalized.CornerRadius)
	}
}

func TestControllerForwardsCornerRadius(t *testing.T) {
	driver := &fakeDriver{}
	controller := New(driver)
	if err := controller.Show(Content{URL: "https://example.com", CornerRadius: 7}, Rect{Width: 100, Height: 80}, 1); err != nil {
		t.Fatalf("show WebView: %v", err)
	}
	if len(driver.shown) != 1 || driver.shown[0].CornerRadius != 7 {
		t.Fatalf("shown corner radius = %+v, want 7", driver.shown)
	}
}
