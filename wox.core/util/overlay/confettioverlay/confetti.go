package confettioverlay

import (
	"errors"
	"math"
	"math/rand"
	"strconv"
	"sync"
	"time"

	woxui "wox/ui/runtime"
	"wox/util/overlay"
	"wox/util/screen"
)

const (
	confettiDuration = 6 * time.Second
	confettiFadeAt   = 4.8
)

var confettiColors = [...]woxui.Color{
	{R: 255, G: 82, B: 82, A: 255},
	{R: 255, G: 193, B: 58, A: 255},
	{R: 65, G: 219, B: 140, A: 255},
	{R: 51, G: 153, B: 255, A: 255},
	{R: 125, G: 87, B: 255, A: 255},
	{R: 244, G: 77, B: 178, A: 255},
	{R: 255, G: 137, B: 70, A: 255},
}

var confettiRuntime = struct {
	sync.Mutex
	session *confettiSession
	nextID  uint64
}{}

type confettiSession struct {
	started time.Time
	surface *confettiSurface
	timer   *time.Timer
}

type confettiSurface struct {
	window    *woxui.Window
	managed   *woxui.ManagedWindow
	width     float32
	height    float32
	particles []confettiParticle
}

type confettiParticle struct {
	x, y             float32
	vx, vy           float32
	gravity, drag    float32
	wind             float32
	width, height    float32
	rotation, spin   float32
	sway, swaySpeed  float32
	phase, flipSpeed float32
	delay            float32
	shape            uint8
	color            woxui.Color
}

// Show starts or restarts a passive confetti animation on the display under the mouse.
func Show() error {
	var showErr error
	if err := woxui.Call(func() { showErr = showConfettiOnUI() }); err != nil {
		return err
	}
	return showErr
}

// showConfettiOnUI reuses a running surface or creates one on the display under the mouse.
func showConfettiOnUI() error {
	manager := overlay.WindowManager()
	if manager == nil {
		return errors.New("confetti window manager is not initialized")
	}
	displays, err := screen.ListDisplays()
	if err != nil {
		return err
	}
	if len(displays) == 0 {
		return errors.New("no displays are available for confetti")
	}
	display, ok := confettiDisplayAtMouse(displays, screen.GetMouseScreen())
	if !ok {
		return errors.New("failed to find the display under the mouse")
	}

	confettiRuntime.Lock()
	activeSession := confettiRuntime.session
	if activeSession == nil {
		confettiRuntime.nextID++
	}
	sessionID := confettiRuntime.nextID
	confettiRuntime.Unlock()
	if activeSession != nil {
		return activeSession.restart(display)
	}

	session := &confettiSession{started: time.Now()}
	size := confettiDisplaySize(display)
	if size.Width <= 0 || size.Height <= 0 {
		return errors.New("no usable displays are available for confetti")
	}
	surface := &confettiSurface{
		width: size.Width, height: size.Height,
		particles: newConfettiParticles(size.Width, size.Height, rand.New(rand.NewSource(time.Now().UnixNano()))),
	}
	session.surface = surface
	managed, _, openErr := manager.Open(woxui.WindowID(confettiWindowID(sessionID)), confettiWindowOptions(func(displayList *woxui.DisplayList, _ woxui.FrameInfo) {
		surface.draw(displayList, time.Since(session.started).Seconds())
	}))
	if openErr != nil {
		return openErr
	}
	surface.managed = managed
	surface.window = managed.Window()
	if boundsErr := setConfettiDisplayBounds(surface.window, display); boundsErr != nil {
		session.close()
		return boundsErr
	}
	if passthroughErr := surface.window.SetPointerPassthrough(true); passthroughErr != nil {
		session.close()
		return passthroughErr
	}
	if _, showErr := managed.Show(); showErr != nil {
		session.close()
		return showErr
	}
	_ = surface.window.RequestAnimationFrame()

	confettiRuntime.Lock()
	confettiRuntime.session = session
	confettiRuntime.Unlock()
	session.resetTimer()
	return nil
}

func confettiWindowID(sessionID uint64) string {
	return "confetti." + strconv.FormatUint(sessionID, 10)
}

// confettiDisplayAtMouse resolves one display from the mouse screen center and falls back to the primary display.
func confettiDisplayAtMouse(displays []screen.Display, mouseScreen screen.Size) (screen.Display, bool) {
	centerX := mouseScreen.X + mouseScreen.Width/2
	centerY := mouseScreen.Y + mouseScreen.Height/2
	for _, display := range displays {
		bounds := display.Bounds
		if centerX >= bounds.X && centerX < bounds.Right() && centerY >= bounds.Y && centerY < bounds.Bottom() {
			return display, true
		}
	}
	for _, display := range displays {
		if display.Primary {
			return display, true
		}
	}
	return screen.Display{}, false
}

// confettiWindowOptions keeps the initial GPU surface small until platform bounds apply the target DPI once.
func confettiWindowOptions(onFrame func(*woxui.DisplayList, woxui.FrameInfo)) woxui.WindowOptions {
	return woxui.WindowOptions{
		Title: "Wox Confetti", Size: woxui.Size{Width: 100, Height: 100}, Role: woxui.WindowRoleScreenshot,
		Nonactivating: true, Topmost: true, OnFrame: onFrame,
	}
}

// restart moves the existing surface to the current mouse display and replaces its particles.
func (session *confettiSession) restart(display screen.Display) error {
	if session.surface == nil || session.surface.window == nil {
		return errors.New("confetti surface is not initialized")
	}
	size := confettiDisplaySize(display)
	if size.Width <= 0 || size.Height <= 0 {
		return errors.New("mouse display has no usable confetti area")
	}
	if err := setConfettiDisplayBounds(session.surface.window, display); err != nil {
		return err
	}
	session.started = time.Now()
	session.surface.width = size.Width
	session.surface.height = size.Height
	session.surface.particles = newConfettiParticles(size.Width, size.Height, rand.New(rand.NewSource(time.Now().UnixNano())))
	_ = session.surface.window.RequestAnimationFrame()
	session.resetTimer()
	return nil
}

// resetTimer closes the native surfaces after the final fade completes.
func (session *confettiSession) resetTimer() {
	if session.timer != nil {
		session.timer.Stop()
	}
	started := session.started
	session.timer = time.AfterFunc(confettiDuration, func() {
		_ = woxui.Call(func() {
			confettiRuntime.Lock()
			if confettiRuntime.session != session || session.started != started {
				confettiRuntime.Unlock()
				return
			}
			confettiRuntime.session = nil
			confettiRuntime.Unlock()
			session.close()
		})
	})
}

// close releases every surface created for this animation session.
func (session *confettiSession) close() {
	if session.timer != nil {
		session.timer.Stop()
	}
	if session.surface != nil && session.surface.managed != nil {
		_ = session.surface.managed.Close()
	}
}

// draw renders one frame and requests the next vsync while the animation is active.
func (surface *confettiSurface) draw(displayList *woxui.DisplayList, elapsed float64) {
	displayList.Clear(woxui.Color{})
	if elapsed >= confettiDuration.Seconds() {
		return
	}
	t := float32(elapsed)
	for _, particle := range surface.particles {
		particle.draw(displayList, t)
	}
	if surface.window != nil {
		_ = surface.window.RequestAnimationFrame()
	}
}

// newConfettiParticles creates a density-bounded particle set for one display.
func newConfettiParticles(width, height float32, random *rand.Rand) []confettiParticle {
	count := min(450, max(220, int(width*height/7200)))
	particles := make([]confettiParticle, count)
	for index := range particles {
		fromLeft := index%2 == 0
		// Pair both sides at continuously spaced launch speeds while varying the raised
		// firing angle, so the burst remains balanced without constraining its apex.
		launchStrength := (float32(index/2) + 0.5) / float32((count+1)/2)
		direction := float32(1)
		x := float32(random.Float64()) * height * 0.025
		if !fromLeft {
			direction = -1
			x = width - x
		}
		scale := min(float32(2.4), max(float32(1), height/1080))
		shape := uint8(random.Intn(5))
		thickness := float32(4+random.Float64()*5) * scale
		lengthRandom := random.Float64()
		length := float32(8+lengthRandom*lengthRandom*34) * scale
		if shape == 0 {
			length = thickness * float32(0.8+random.Float64()*0.5)
		} else if shape == 1 {
			length = thickness * float32(1.3+random.Float64()*0.8)
		}
		launchSpeed := height * float32(0.51+1.06*launchStrength)
		launchAngle := float32((38 + random.Float64()*40) * math.Pi / 180)
		particles[index] = confettiParticle{
			x:         x,
			y:         height * float32(0.82+random.Float64()*0.15),
			vx:        direction * launchSpeed * float32(math.Cos(float64(launchAngle))),
			vy:        -launchSpeed * float32(math.Sin(float64(launchAngle))),
			gravity:   height * 0.78,
			drag:      float32(0.22 + random.Float64()*0.2),
			wind:      width * float32(-0.02+random.Float64()*0.04),
			width:     length,
			height:    thickness,
			rotation:  float32(random.Float64() * 2 * math.Pi),
			spin:      float32(-4 + random.Float64()*8),
			sway:      float32(12+random.Float64()*30) * scale,
			swaySpeed: float32(1.5 + random.Float64()*3),
			phase:     float32(random.Float64() * 2 * math.Pi),
			flipSpeed: float32(2 + random.Float64()*5),
			delay:     float32(random.Float64() * 0.18),
			shape:     shape,
			color:     confettiColors[random.Intn(len(confettiColors))],
		}
	}
	return particles
}

// draw evaluates one particle directly from elapsed time so dropped frames do not change its path.
func (particle confettiParticle) draw(displayList *woxui.DisplayList, elapsed float32) {
	x, y, visible := particle.positionAt(elapsed)
	if !visible {
		return
	}
	t := elapsed - particle.delay
	width := particle.width * max(0.15, float32(math.Abs(math.Cos(float64(particle.phase+particle.flipSpeed*t)))))
	rotation := particle.rotation + particle.spin*t
	color := particle.color
	if elapsed > confettiFadeAt {
		color.A = uint8(float32(color.A) * max(0, 1-(elapsed-confettiFadeAt)/float32(confettiDuration.Seconds()-confettiFadeAt)))
	}

	switch particle.shape {
	case 0:
		displayList.FillRoundedRect(woxui.Rect{X: x - width/2, Y: y - particle.height/2, Width: width, Height: particle.height}, min(width, particle.height)/2, color)
	case 1:
		displayList.FillConvexPolygon(rotatedConfettiPoints(x, y, width, particle.height, rotation, true), color)
	default:
		displayList.FillConvexPolygon(rotatedConfettiPoints(x, y, width, particle.height, rotation, false), color)
	}
}

// positionAt evaluates gravity, wind, and linear air resistance without accumulating frame error.
func (particle confettiParticle) positionAt(elapsed float32) (float32, float32, bool) {
	t := elapsed - particle.delay
	if t < 0 {
		return 0, 0, false
	}
	horizontal := dampedDisplacement(particle.vx, particle.wind, particle.drag, t)
	vertical := dampedDisplacement(particle.vy, particle.gravity, particle.drag, t)
	sway := particle.sway * (float32(math.Sin(float64(particle.phase+particle.swaySpeed*t))) - float32(math.Sin(float64(particle.phase))))
	x := particle.x + horizontal + sway
	y := particle.y + vertical
	return x, y, true
}

// dampedDisplacement is the exact displacement for constant acceleration with linear air resistance.
func dampedDisplacement(initialVelocity, acceleration, drag, elapsed float32) float32 {
	if drag <= 0 {
		return initialVelocity*elapsed + acceleration*elapsed*elapsed/2
	}
	terminalVelocity := acceleration / drag
	return terminalVelocity*elapsed + (initialVelocity-terminalVelocity)*(1-float32(math.Exp(float64(-drag*elapsed))))/drag
}

// rotatedConfettiPoints builds an antialiased rectangle or triangle around a particle center.
func rotatedConfettiPoints(x, y, width, height, rotation float32, triangle bool) []woxui.Point {
	points := []woxui.Point{{X: -width / 2, Y: -height / 2}, {X: width / 2, Y: -height / 2}, {X: width / 2, Y: height / 2}, {X: -width / 2, Y: height / 2}}
	if triangle {
		points = []woxui.Point{{Y: -height / 2}, {X: width / 2, Y: height / 2}, {X: -width / 2, Y: height / 2}}
	}
	sin, cos := float32(math.Sin(float64(rotation))), float32(math.Cos(float64(rotation)))
	for index, point := range points {
		points[index] = woxui.Point{X: x + point.X*cos - point.Y*sin, Y: y + point.X*sin + point.Y*cos}
	}
	return points
}
