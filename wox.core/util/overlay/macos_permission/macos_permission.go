package macospermission

import (
	"bytes"
	"fmt"
	"runtime"
	"sync"
	"time"

	"wox/resource"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/overlay"
	"wox/util/permission"
)

const (
	overlayIDPrefix    = "macos-permission"
	panelWidth         = float32(330)
	panelHeightWithApp = float32(184)
	panelHeightManual  = float32(158)
	trackingInterval   = 150 * time.Millisecond
	refreshInterval    = 800 * time.Millisecond
	windowMissingGrace = 600 * time.Millisecond
	startupTimeout     = 8 * time.Second
)

const (
	// PlacementRight attaches the guide to the trailing edge of System Settings.
	PlacementRight = iota
	// PlacementBottom attaches the guide below System Settings.
	PlacementBottom
)

// Options describes one macOS drag-to-authorize overlay session.
type Options struct {
	PermissionType     permission.MacOSPermissionType
	Title              string
	RightInstruction   string
	BottomInstruction  string
	ManualInstruction  string
	CloseLabel         string
	Theme              woxcomponent.Theme
	LightAppearance    bool
	OnClosed           func()
	OnRefreshRequested func()
}

// Rect is a logical rectangle in the runtime's top-left virtual-desktop space.
type Rect struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// Layout is the floating overlay origin next to System Settings.
type Layout struct {
	X         float64
	Y         float64
	Placement int
}

type session struct {
	id           string
	opts         Options
	appPath      string
	appIcon      *woxui.Image
	stop         chan struct{}
	wasRunning   bool
	windowSeen   bool
	startedAt    time.Time
	missingSince time.Time
	lastRefresh  time.Time
}

var activeSession struct {
	sync.Mutex
	current *session
	nextID  uint64
}

func (r Rect) maxX() float64 { return r.X + r.Width }
func (r Rect) maxY() float64 { return r.Y + r.Height }

func clamp(value, low, high float64) float64 {
	if high < low {
		return low
	}
	return min(max(value, low), high)
}

// PanelLayout keeps the guide outside System Settings so it cannot cover the permission list.
func PanelLayout(settings Rect, width, height float64, workArea Rect) Layout {
	rightSpace := workArea.maxX() - settings.maxX()
	bottomSpace := workArea.maxY() - settings.maxY()
	right := Layout{
		X:         settings.maxX(),
		Y:         clamp(settings.Y, workArea.Y, workArea.maxY()-height),
		Placement: PlacementRight,
	}
	bottom := Layout{
		X:         clamp(settings.maxX()-width, workArea.X, workArea.maxX()-width),
		Y:         settings.maxY(),
		Placement: PlacementBottom,
	}
	if rightSpace >= width {
		return right
	}
	if bottomSpace >= height {
		return bottom
	}
	if width > 0 && height > 0 && rightSpace/width >= bottomSpace/height {
		return right
	}
	return bottom
}

// Open starts one permission guide and replaces any existing guide without notifying it.
func Open(opts Options) error {
	if !permission.IsValidMacOSPermissionType(opts.PermissionType) {
		return fmt.Errorf("invalid macOS permission type: %s", opts.PermissionType)
	}
	anchor := permission.MacOSPermissionSettingsAnchor(opts.PermissionType)
	if anchor == "" {
		return fmt.Errorf("missing System Settings anchor for permission: %s", opts.PermissionType)
	}
	if runtime.GOOS != "darwin" {
		return nil
	}

	Close()
	appPath := permissionApplicationPath()
	appIcon := loadAppIcon()
	activeSession.Lock()
	activeSession.nextID++
	instance := &session{
		id:   fmt.Sprintf("%s-%d", overlayIDPrefix, activeSession.nextID),
		opts: opts, appPath: appPath, appIcon: appIcon, stop: make(chan struct{}), startedAt: time.Now(),
	}
	activeSession.current = instance
	activeSession.Unlock()

	openPermissionSettings(anchor)
	go instance.track()
	return nil
}

// Close stops the active guide without firing its user-close callback.
func Close() {
	activeSession.Lock()
	instance := activeSession.current
	activeSession.current = nil
	if instance != nil {
		close(instance.stop)
	}
	activeSession.Unlock()
	if instance != nil {
		overlay.Close(instance.id)
	}
}

// Complete ends the matching active guide and notifies its owner that the permission flow finished.
func Complete(permissionType permission.MacOSPermissionType) {
	activeSession.Lock()
	instance := activeSession.current
	activeSession.Unlock()
	if instance != nil && instance.opts.PermissionType == permissionType {
		instance.finish(true)
	}
}

// track follows System Settings until the user closes either side of the permission flow.
func (s *session) track() {
	ticker := time.NewTicker(trackingInterval)
	defer ticker.Stop()
	s.trackOnce()
	for {
		select {
		case <-ticker.C:
			s.trackOnce()
		case <-s.stop:
			return
		}
	}
}

// trackOnce refreshes permission state and repositions the shared overlay beside System Settings.
func (s *session) trackOnce() {
	settings, workArea, running := permissionSettingsWindow()
	if !running {
		if s.wasRunning || time.Since(s.startedAt) >= startupTimeout {
			s.finish(true)
		}
		return
	}
	s.wasRunning = true
	s.requestRefresh()
	if settings.Width <= 0 || settings.Height <= 0 || workArea.Width <= 0 || workArea.Height <= 0 {
		if !s.windowSeen {
			if time.Since(s.startedAt) >= startupTimeout {
				s.finish(true)
			}
			return
		}
		if s.missingSince.IsZero() {
			s.missingSince = time.Now()
		} else if time.Since(s.missingSince) >= windowMissingGrace {
			s.finish(true)
		}
		return
	}
	s.windowSeen = true
	s.missingSince = time.Time{}

	height := panelHeightManual
	if s.appPath != "" {
		height = panelHeightWithApp
	}
	layout := PanelLayout(settings, float64(panelWidth), float64(height), workArea)
	instruction := s.opts.BottomInstruction
	if layout.Placement == PlacementRight {
		instruction = s.opts.RightInstruction
	}
	s.show(layout, workArea, height, instruction)
}

// show updates the existing shared overlay instead of creating an AppKit panel.
func (s *session) show(layout Layout, layoutWorkArea Rect, height float32, instruction string) {
	if !s.isActive() {
		return
	}
	workArea := woxui.Rect{X: float32(layoutWorkArea.X), Y: float32(layoutWorkArea.Y), Width: float32(layoutWorkArea.Width), Height: float32(layoutWorkArea.Height)}
	shown := overlay.ShowWindow(overlay.WindowOptions{
		ID: s.id, Topmost: true, AbsolutePosition: true,
		OffsetX: layout.X, OffsetY: layout.Y, Width: float64(panelWidth), Height: float64(height),
		WorkArea:        &workArea,
		LightAppearance: s.opts.LightAppearance, FollowsThemeAppearance: true,
		OnClose: s.closedByUser,
	}, overlay.View{
		Kind: "macos-permission",
		Build: func(window *woxui.Window, frame woxui.FrameInfo) woxwidget.Widget {
			return s.build(window, frame, instruction)
		},
		OnDispose: s.closedByRuntime,
	})
	if !shown {
		s.finish(true)
		return
	}
	if shown && !s.isActive() {
		overlay.Close(s.id)
	}
}

// build composes the permission guide using portable widgets over the native overlay material.
func (s *session) build(window *woxui.Window, frame woxui.FrameInfo, instruction string) woxwidget.Widget {
	theme := permissionTheme(s.opts.Theme, s.opts.LightAppearance)
	body := instruction
	if s.appPath == "" {
		body = s.opts.ManualInstruction
	}
	content := []woxwidget.Widget{
		woxwidget.Container{Height: 18},
		woxwidget.TextBlock{Value: s.opts.Title, Width: panelWidth - 64, Height: 20, MaxLines: 1, Centered: true, Style: woxui.TextStyle{Size: 15, Weight: woxui.FontWeightSemibold}, Color: theme.ActionText},
		woxwidget.Container{Height: 8},
		woxwidget.TextBlock{Value: body, Width: panelWidth - 48, Height: 42, LineHeight: 18, MaxLines: 3, Centered: true, Style: woxui.TextStyle{Size: 13, Weight: permissionInstructionWeight(s.appPath)}, Color: permissionInstructionColor(theme, s.appPath)},
	}
	if s.appPath != "" {
		content = append(content,
			woxwidget.Container{Height: 12},
			woxwidget.Semantics{Role: woxui.AccessibilityRoleButton, Label: body, Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate}, Child: woxwidget.Gesture{
				ID: "macos-permission-app-drag", OnDragStart: func() { _, _ = window.StartFileDrag([]string{s.appPath}) },
				Child: woxwidget.Container{Width: 64, Height: 64, Radius: 12, Color: theme.QueryBackground, Child: woxwidget.Align{Width: 64, Height: 64, Horizontal: 0.5, Vertical: 0.5, Child: permissionAppIcon(s.appIcon)}},
			}},
		)
	}

	closeLabel := s.opts.CloseLabel
	if closeLabel == "" {
		closeLabel = "Close"
	}
	closeButton := woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
		ID: "macos-permission-close", Label: closeLabel, Icon: woxcomponent.CloseGlyph(16, theme.ResultSubtitle),
		Width: 28, Height: 28, Radius: 14, HoverBackground: permissionAlpha(theme.ResultSubtitle, 28), OnTap: func() { overlay.RequestClose(s.id) },
	})
	return woxwidget.Container{Width: frame.Size.Width, Height: frame.Size.Height, Color: overlay.CurrentThemeChrome().Background, Child: woxwidget.Stack{Width: frame.Size.Width, Height: frame.Size.Height, Children: []woxwidget.StackChild{
		{Child: woxwidget.Align{Width: frame.Size.Width, Height: frame.Size.Height, Horizontal: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Vertical, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: content}}},
		{Left: frame.Size.Width - 38, Top: 8, Child: closeButton},
	}}}
}

// requestRefresh throttles passive permission probes while the System Settings window is tracked.
func (s *session) requestRefresh() {
	if time.Since(s.lastRefresh) < refreshInterval {
		return
	}
	s.lastRefresh = time.Now()
	if s.opts.OnRefreshRequested != nil {
		s.opts.OnRefreshRequested()
	}
}

// closedByUser detaches the session before RequestClose disposes the overlay window.
func (s *session) closedByUser() {
	s.notifyClosed()
}

// closedByRuntime restores the owner when the native overlay disappears outside RequestClose.
func (s *session) closedByRuntime() {
	s.notifyClosed()
}

// notifyClosed restores the owner exactly once regardless of which window lifecycle ended first.
func (s *session) notifyClosed() {
	if s.detach() && s.opts.OnClosed != nil {
		s.opts.OnClosed()
	}
}

// finish closes a session from external lifecycle tracking and optionally restores its owner.
func (s *session) finish(notify bool) {
	if !s.detach() {
		return
	}
	overlay.Close(s.id)
	if notify && s.opts.OnClosed != nil {
		s.opts.OnClosed()
	}
}

// detach makes close notifications idempotent across the overlay and System Settings lifecycles.
func (s *session) detach() bool {
	activeSession.Lock()
	defer activeSession.Unlock()
	if activeSession.current != s {
		return false
	}
	activeSession.current = nil
	close(s.stop)
	return true
}

func (s *session) isActive() bool {
	activeSession.Lock()
	defer activeSession.Unlock()
	return activeSession.current == s
}

// loadAppIcon decodes the embedded app icon once for the lifetime of the permission guide.
func loadAppIcon() *woxui.Image {
	icon, err := woxui.DecodeImage(bytes.NewReader(resource.GetAppIconPNG()))
	if err != nil {
		return nil
	}
	return icon
}

func permissionAppIcon(icon *woxui.Image) woxwidget.Widget {
	if icon == nil {
		return woxwidget.Container{Width: 48, Height: 48, Radius: 10}
	}
	return woxwidget.Image{Source: icon, Width: 48, Height: 48, Fit: woxwidget.ImageFitContain, Radius: 10}
}

func permissionTheme(theme woxcomponent.Theme, light bool) woxcomponent.Theme {
	if theme.ActionText.A != 0 && theme.ResultSubtitle.A != 0 && theme.QueryBackground.A != 0 {
		return theme
	}
	if light {
		return woxcomponent.Theme{ActionText: woxui.Color{R: 28, G: 28, B: 30, A: 255}, ResultSubtitle: woxui.Color{R: 99, G: 99, B: 102, A: 255}, QueryBackground: woxui.Color{R: 255, G: 255, B: 255, A: 190}}
	}
	return woxcomponent.Theme{ActionText: woxui.Color{R: 245, G: 245, B: 247, A: 255}, ResultSubtitle: woxui.Color{R: 199, G: 199, B: 204, A: 255}, QueryBackground: woxui.Color{R: 28, G: 28, B: 30, A: 220}}
}

func permissionInstructionWeight(appPath string) woxui.FontWeight {
	if appPath != "" {
		return woxui.FontWeightSemibold
	}
	return woxui.FontWeightRegular
}

func permissionInstructionColor(theme woxcomponent.Theme, appPath string) woxui.Color {
	if appPath != "" {
		return woxui.Color{R: 255, G: 149, B: 0, A: 255}
	}
	return theme.ResultSubtitle
}

func permissionAlpha(color woxui.Color, alpha uint8) woxui.Color {
	color.A = alpha
	return color
}
