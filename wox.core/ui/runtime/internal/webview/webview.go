package webview

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

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
	CornerRadius  float32
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
	Reset() error
	GoBack() error
	GoForward() error
	Reload() error
	OpenDevTools() error
	OpenInBrowser() error
	NavigationState() (NavigationState, error)
	Pointer(event PointerEvent) bool
	Close()
}

// Controller owns the portable lifecycle around one platform WebView driver.
type Controller struct {
	driver  Driver
	visible bool
}

// New creates a portable WebView controller around one native driver.
func New(driver Driver) *Controller {
	return &Controller{driver: driver}
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
	if c == nil || c.driver == nil {
		return ErrUnavailable
	}
	normalized, err := Normalize(content, bounds)
	if err != nil {
		return err
	}
	if err := c.driver.Show(normalized, bounds, scale); err != nil {
		return err
	}
	c.visible = true
	return nil
}

// Hide detaches the active native surface without discarding cached browser state.
func (c *Controller) Hide() error {
	if c == nil || c.driver == nil {
		return nil
	}
	if err := c.driver.Hide(); err != nil {
		return err
	}
	c.visible = false
	return nil
}

// Reset destroys active and cached native browser state.
func (c *Controller) Reset() error {
	if c == nil || c.driver == nil {
		return nil
	}
	if err := c.driver.Reset(); err != nil {
		return err
	}
	c.visible = false
	return nil
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

// Visible reports whether the controller currently contributes a native composition surface.
func (c *Controller) Visible() bool {
	return c != nil && c.visible
}

// Close permanently releases the native driver.
func (c *Controller) Close() {
	if c == nil || c.driver == nil {
		return
	}
	c.driver.Close()
	c.driver = nil
	c.visible = false
}
