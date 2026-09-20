package webview

import (
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

type fakeDriver struct {
	shown      []Content
	hideCalls  int
	resetCalls int
	devTools   int
	focusCalls int
	closeCalls int
	showErr    error
	evicted    []string
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
func (d *fakeDriver) Focus() error                              { d.focusCalls++; return nil }
func (d *fakeDriver) Close()                                    { d.closeCalls++ }
func (d *fakeDriver) Evict(key string) error                    { d.evicted = append(d.evicted, key); return nil }

func immediateDispatch(fn func()) error { fn(); return nil }

func TestControllerOwnsVisibleLifecycle(t *testing.T) {
	driver := &fakeDriver{}
	controller := New(driver, immediateDispatch)
	if err := controller.Show(Content{URL: " https://example.com ", UserAgent: " ExampleBrowser/1.0 ", CacheKey: " cache "}, Rect{Width: 100, Height: 80}, 1); err != nil {
		t.Fatalf("show WebView: %v", err)
	}
	if !controller.Visible() || len(driver.shown) != 1 || driver.shown[0].URL != "https://example.com" || driver.shown[0].UserAgent != "ExampleBrowser/1.0" || driver.shown[0].CacheKey != "url|cache" {
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
	if err := controller.Focus(); err != nil || driver.focusCalls != 1 {
		t.Fatalf("focus calls = %d err %v", driver.focusCalls, err)
	}
	controller.Close()
	if driver.closeCalls != 1 || controller.Visible() {
		t.Fatalf("close state = visible %v calls %d", controller.Visible(), driver.closeCalls)
	}
}

func TestControllerDoesNotPublishFailedShow(t *testing.T) {
	driver := &fakeDriver{showErr: errors.New("show failed")}
	controller := New(driver, immediateDispatch)
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
	controller := New(driver, immediateDispatch)
	if err := controller.Show(Content{URL: "https://example.com", CornerRadius: 7}, Rect{Width: 100, Height: 80}, 1); err != nil {
		t.Fatalf("show WebView: %v", err)
	}
	if len(driver.shown) != 1 || driver.shown[0].CornerRadius != 7 {
		t.Fatalf("shown corner radius = %+v, want 7", driver.shown)
	}
}

// TestHTMLIdleLifecycle uses virtual time to cover reuse, expiry, and isolation from URL caches.
func TestHTMLIdleLifecycle(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		driver := &fakeDriver{}
		var ui sync.Mutex
		controller := New(driver, func(fn func()) error {
			ui.Lock()
			defer ui.Unlock()
			fn()
			return nil
		})
		defer controller.Close()
		bounds := Rect{Width: 100, Height: 80}
		for _, content := range []Content{
			{HTML: "<p>first</p>"},
			{HTML: "<p>second</p>", CacheKey: "plugin-key"},
			{HTML: "<p>third</p>", CacheKey: "another-key", CacheDisabled: true},
		} {
			if err := controller.Show(content, bounds, 1); err != nil {
				t.Fatal(err)
			}
			shown := driver.shown[len(driver.shown)-1]
			if shown.CacheKey != htmlCacheKey || shown.CacheDisabled || shown.HTML != content.HTML {
				t.Fatalf("HTML slot = %+v", shown)
			}
		}
		time.Sleep(11 * time.Second)
		synctest.Wait()
		if len(driver.evicted) != 0 {
			t.Fatal("visible HTML expired")
		}
		if err := controller.Hide(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(9 * time.Second)
		if err := controller.Show(Content{HTML: "<p>reused</p>"}, bounds, 1); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Second)
		synctest.Wait()
		if len(driver.evicted) != 0 {
			t.Fatal("reused HTML expired at its old deadline")
		}

		// Even a URL plugin using the reserved word must not share the HTML slot.
		url := Content{URL: "https://example.com", CacheKey: htmlCacheKey}
		if err := controller.Show(url, bounds, 1); err != nil {
			t.Fatal(err)
		}
		time.Sleep(9 * time.Second)
		if err := controller.Show(url, bounds, 1); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Second)
		synctest.Wait()
		if len(driver.evicted) != 1 || driver.evicted[0] != htmlCacheKey || driver.resetCalls != 0 || !controller.Visible() {
			t.Fatalf("expiry disturbed URL: evicted=%v resets=%d visible=%v", driver.evicted, driver.resetCalls, controller.Visible())
		}
		if driver.shown[len(driver.shown)-1].CacheKey == htmlCacheKey {
			t.Fatal("URL cache collided with HTML")
		}
		if err := controller.Show(Content{HTML: "<p>fresh</p>"}, bounds, 1); err != nil {
			t.Fatal(err)
		}
		if err := controller.Hide(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(9 * time.Second)
		if err := controller.Hide(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Second)
		synctest.Wait()
		if len(driver.evicted) != 2 {
			t.Fatal("repeated hide postponed HTML expiry")
		}
	})
}

// TestHTMLQueuedExpiryCannotReleaseReusedOrClosedSession covers a timer already queued on the UI thread.
func TestHTMLQueuedExpiryCannotReleaseReusedOrClosedSession(t *testing.T) {
	for _, action := range []string{"reuse", "reset", "close"} {
		t.Run(action, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				driver := &fakeDriver{}
				var queued func()
				queueTimer := false
				var ui sync.Mutex
				controller := New(driver, func(fn func()) error {
					ui.Lock()
					defer ui.Unlock()
					if queueTimer {
						queued = fn
					} else {
						fn()
					}
					return nil
				})
				bounds := Rect{Width: 100, Height: 80}
				if err := controller.Show(Content{HTML: "<p>old</p>"}, bounds, 1); err != nil {
					t.Fatal(err)
				}
				if err := controller.Hide(); err != nil {
					t.Fatal(err)
				}
				ui.Lock()
				queueTimer = true
				ui.Unlock()
				time.Sleep(htmlIdleTimeout)
				synctest.Wait()
				ui.Lock()
				queueTimer = false
				ui.Unlock()
				if queued == nil {
					t.Fatal("expiry was not dispatched")
				}
				switch action {
				case "reuse":
					if err := controller.Show(Content{HTML: "<p>new</p>"}, bounds, 1); err != nil {
						t.Fatal(err)
					}
				case "reset":
					if err := controller.Reset(); err != nil {
						t.Fatal(err)
					}
				case "close":
					controller.Close()
				}
				queued()
				if len(driver.evicted) != 0 {
					t.Fatal("stale timer evicted the HTML slot")
				}
				controller.Close()
			})
		})
	}
}
