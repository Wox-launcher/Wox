package webview

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"wox/util"
)

const htmlCacheKey = "html"
const htmlIdleTimeout = 10 * time.Second

// ErrUnavailable reports that the current desktop is missing its system WebView runtime.
var ErrUnavailable = errors.New("woxui: system WebView is unavailable")

// Content describes one embedded browser document while Rect is controlled separately by layout.
type Content struct {
	URL           string
	HTML          string
	InjectCSS     string
	UserAgent     string
	CacheDisabled bool
	CacheKey      string
	// CornerRadius clips the native surface so it stays concentric with the preview shell.
	CornerRadius float32
}

// NavigationState mirrors the live browser chrome for an attached WebView.
type NavigationState struct {
	URL          string
	CanGoBack    bool
	CanGoForward bool
}

// Rect describes the WebView placement in logical client coordinates.
type Rect struct {
	X      float32
	Y      float32
	Width  float32
	Height float32
}

// Point describes a surface-local pointer position or scroll delta.
type Point struct {
	X float32
	Y float32
}

// PointerEvent is the platform-neutral input forwarded after Host hit testing.
type PointerEvent struct {
	Kind      uint8
	Position  Point
	Button    uint8
	Scroll    Point
	Modifiers uint8
}

// Driver is the narrow native-window bridge implemented by each runtime platform.
type Driver interface {
	Show(content Content, bounds Rect, scale float32) error
	Hide() error
	// Evict releases one cached session without disturbing another active page.
	Evict(cacheKey string) error
	Reset() error
	GoBack() error
	GoForward() error
	Reload() error
	OpenDevTools() error
	OpenInBrowser() error
	NavigationState() (NavigationState, error)
	Pointer(event PointerEvent) bool
	// Focus moves keyboard input to the native page. A first-show controller may
	// still be creating; implementations may queue the request until it is ready.
	Focus() error
	Close()
}

// Controller owns the portable lifecycle around one platform WebView driver.
type Controller struct {
	driver   Driver
	dispatch func(func()) error
	visible  atomic.Bool
	// These fields are owned by the UI thread, including timer completion.
	htmlVisible    bool
	htmlRetained   bool
	htmlTimer      *time.Timer
	htmlGeneration uint64
}

// New creates a portable WebView controller around one native driver.
func New(driver Driver, dispatch func(func()) error) *Controller {
	return &Controller{driver: driver, dispatch: dispatch}
}

// call serializes lifecycle changes with native callbacks on the owning UI thread.
func (c *Controller) call(fn func() error) error {
	if c == nil {
		return ErrUnavailable
	}
	var result error
	if err := c.dispatch(func() { result = fn() }); err != nil {
		return err
	}
	return result
}

// Normalize validates content and placement before native work begins.
func Normalize(content Content, bounds Rect) (Content, error) {
	content.URL = strings.TrimSpace(content.URL)
	content.UserAgent = strings.TrimSpace(content.UserAgent)
	content.CacheKey = strings.TrimSpace(content.CacheKey)
	if content.URL == "" && content.HTML == "" {
		return Content{}, errors.New("webview content requires a URL or HTML")
	}
	if content.HTML == "" && !IsAbsoluteURL(content.URL) {
		return Content{}, fmt.Errorf("webview URL must be an absolute http(s) URL: %q", content.URL)
	}
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return Content{}, errors.New("webview bounds must have a positive size")
	}
	content.CornerRadius = min(max(content.CornerRadius, 0), min(bounds.Width, bounds.Height)/2)
	return content, nil
}

// IsAbsoluteURL reports whether a URL can be navigated by a WebView preview.
func IsAbsoluteURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Scheme, "http") || strings.EqualFold(parsed.Scheme, "https")
}

// Show attaches or updates the native WebView after validating the portable contract.
func (c *Controller) Show(content Content, bounds Rect, scale float32) error {
	normalized, err := Normalize(content, bounds)
	if err != nil {
		return err
	}
	// HTML owns one temporary slot regardless of plugin cache options. URL keys
	// live in a separate namespace so a plugin cannot collide with that slot.
	if normalized.HTML != "" {
		normalized.CacheKey = htmlCacheKey
		normalized.CacheDisabled = false
	} else if normalized.CacheKey != "" {
		normalized.CacheKey = "url|" + normalized.CacheKey
	}
	return c.call(func() error {
		if c.driver == nil {
			return ErrUnavailable
		}
		if normalized.HTML != "" {
			// A driver can allocate a session before reporting a load failure;
			// the subsequent hide still needs to release that temporary entry.
			c.htmlRetained = true
		}
		if err := c.driver.Show(normalized, bounds, scale); err != nil {
			if !c.htmlVisible {
				c.scheduleHTMLExpiry()
			}
			return err
		}
		c.visible.Store(true)
		c.htmlVisible = normalized.HTML != ""
		if c.htmlVisible {
			c.cancelHTMLExpiry()
		} else {
			c.scheduleHTMLExpiry()
		}
		return nil
	})
}

// Hide detaches the active native surface without discarding cached browser state.
func (c *Controller) Hide() error {
	if c == nil {
		return nil
	}
	return c.call(func() error {
		if c.driver == nil {
			return nil
		}
		if err := c.driver.Hide(); err != nil {
			return err
		}
		c.visible.Store(false)
		c.htmlVisible = false
		c.scheduleHTMLExpiry()
		return nil
	})
}

// scheduleHTMLExpiry starts once on leaving HTML; repeated hides or URL frames
// must not postpone cleanup. A queued old callback cannot evict a reused slot.
func (c *Controller) scheduleHTMLExpiry() {
	if !c.htmlRetained || c.htmlTimer != nil {
		return
	}
	c.htmlGeneration++
	generation := c.htmlGeneration
	c.htmlTimer = time.AfterFunc(htmlIdleTimeout, func() {
		if err := c.call(func() error {
			if c.driver == nil || c.htmlVisible || generation != c.htmlGeneration {
				return nil
			}
			c.htmlTimer = nil
			if err := c.driver.Evict(htmlCacheKey); err != nil {
				return err
			}
			c.htmlRetained = false
			return nil
		}); err != nil {
			util.GetLogger().Error(context.Background(), "expire idle HTML WebView: "+err.Error())
		}
	})
}

// cancelHTMLExpiry invalidates callbacks that already reached the UI queue.
func (c *Controller) cancelHTMLExpiry() {
	c.htmlGeneration++
	if c.htmlTimer != nil {
		c.htmlTimer.Stop()
		c.htmlTimer = nil
	}
}

// Reset destroys active and cached native browser state.
func (c *Controller) Reset() error {
	if c == nil {
		return nil
	}
	return c.call(func() error {
		if c.driver == nil {
			return nil
		}
		if err := c.driver.Reset(); err != nil {
			return err
		}
		c.cancelHTMLExpiry()
		c.htmlRetained, c.htmlVisible = false, false
		c.visible.Store(false)
		return nil
	})
}

// GoBack navigates the active document backward.
func (c *Controller) GoBack() error {
	if c == nil || c.driver == nil {
		return ErrUnavailable
	}
	return c.driver.GoBack()
}

// GoForward navigates the active document forward.
func (c *Controller) GoForward() error {
	if c == nil || c.driver == nil {
		return ErrUnavailable
	}
	return c.driver.GoForward()
}

// Reload reloads the active document.
func (c *Controller) Reload() error {
	if c == nil || c.driver == nil {
		return ErrUnavailable
	}
	return c.driver.Reload()
}

// OpenDevTools opens the platform inspector for the active document.
func (c *Controller) OpenDevTools() error {
	if c == nil || c.driver == nil {
		return ErrUnavailable
	}
	return c.driver.OpenDevTools()
}

// OpenInBrowser opens the active document in the system browser.
func (c *Controller) OpenInBrowser() error {
	if c == nil || c.driver == nil {
		return ErrUnavailable
	}
	return c.driver.OpenInBrowser()
}

// NavigationState returns the current browser navigation state.
func (c *Controller) NavigationState() (NavigationState, error) {
	if c == nil || c.driver == nil {
		return NavigationState{}, ErrUnavailable
	}
	return c.driver.NavigationState()
}

// Pointer forwards host-tested, surface-local input to the native browser.
func (c *Controller) Pointer(event PointerEvent) bool {
	return c != nil && c.driver != nil && c.driver.Pointer(event)
}

// Focus asks the native browser to take keyboard input for scrolling and page shortcuts.
func (c *Controller) Focus() error {
	if c == nil || c.driver == nil {
		return ErrUnavailable
	}
	return c.driver.Focus()
}

// Visible reports whether the controller currently contributes a native composition surface.
func (c *Controller) Visible() bool {
	return c != nil && c.visible.Load()
}

// Close permanently releases the native driver.
func (c *Controller) Close() {
	if c == nil {
		return
	}
	if err := c.call(func() error {
		c.cancelHTMLExpiry()
		if c.driver != nil {
			c.driver.Close()
			c.driver = nil
		}
		c.htmlRetained, c.htmlVisible = false, false
		c.visible.Store(false)
		return nil
	}); err != nil {
		util.GetLogger().Error(context.Background(), "close WebView: "+err.Error())
	}
}
