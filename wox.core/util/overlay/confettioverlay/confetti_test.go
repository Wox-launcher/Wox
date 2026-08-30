package confettioverlay

import (
	"math"
	"math/rand"
	"sort"
	"testing"

	woxui "wox/ui/runtime"
	"wox/util/screen"
)

func TestConfettiWindowStartsSmallBeforePhysicalResize(t *testing.T) {
	options := confettiWindowOptions(nil)
	if options.Size.Width != 100 || options.Size.Height != 100 || options.Role != woxui.WindowRoleScreenshot {
		t.Fatalf("initial confetti window options = %+v", options)
	}
}

func TestConfettiDisplayAtMouseSelectsOneDisplay(t *testing.T) {
	displays := []screen.Display{
		{ID: "left", Bounds: screen.Rect{X: -1920, Width: 1920, Height: 1080}},
		{ID: "right", Bounds: screen.Rect{Width: 2560, Height: 1440}, Primary: true},
	}
	display, ok := confettiDisplayAtMouse(displays, screen.Size{X: -1920, Width: 1920, Height: 1040})
	if !ok || display.ID != "left" {
		t.Fatalf("mouse display = %q, %t, want left, true", display.ID, ok)
	}
}

func TestNewConfettiParticlesLaunchFromBothBottomCorners(t *testing.T) {
	particles := newConfettiParticles(1920, 1080, rand.New(rand.NewSource(1)))
	if len(particles) != 288 {
		t.Fatalf("particle count = %d, want 288", len(particles))
	}
	left, right := 0, 0
	leftSpeeds, rightSpeeds := make([]float32, 0, len(particles)/2), make([]float32, 0, len(particles)/2)
	minAngle, maxAngle := float64(90), float64(0)
	minRibbon, maxRibbon := float32(1000), float32(0)
	for _, particle := range particles {
		if particle.y < 885.6 || particle.y > 1047.6 {
			t.Fatalf("particle spawned outside bottom launch band: x=%v y=%v", particle.x, particle.y)
		}
		if particle.x <= 27 && particle.vx > 0 {
			left++
			leftSpeeds = append(leftSpeeds, -particle.vy/1080)
		} else if particle.x >= 1893 && particle.vx < 0 {
			right++
			rightSpeeds = append(rightSpeeds, -particle.vy/1080)
		} else {
			t.Fatalf("particle did not launch inward from a corner: x=%v vx=%v", particle.x, particle.vx)
		}
		if particle.gravity <= 0 || particle.width <= 0 || particle.height <= 0 {
			t.Fatalf("particle has invalid motion or size: %+v", particle)
		}
		angle := math.Atan2(float64(-particle.vy), math.Abs(float64(particle.vx))) * 180 / math.Pi
		minAngle = min(minAngle, angle)
		maxAngle = max(maxAngle, angle)
		if particle.delay > 0.18 {
			t.Fatalf("particle delay = %v, want one launch wave", particle.delay)
		}
		if particle.shape >= 2 {
			minRibbon = min(minRibbon, particle.width)
			maxRibbon = max(maxRibbon, particle.width)
		}
	}
	if left != 144 || right != 144 {
		t.Fatalf("corner launch counts = %d, %d, want 144 each", left, right)
	}
	for _, speeds := range [][]float32{leftSpeeds, rightSpeeds} {
		sort.Slice(speeds, func(i, j int) bool { return speeds[i] < speeds[j] })
		if speeds[0] > 0.5 || speeds[len(speeds)-1] < 1.3 {
			t.Fatalf("launch speeds do not span bottom to main arc: %v..%v", speeds[0], speeds[len(speeds)-1])
		}
		for index := 1; index < len(speeds); index++ {
			if speeds[index]-speeds[index-1] > 0.11 {
				t.Fatalf("launch wave has a speed gap: %v..%v", speeds[index-1], speeds[index])
			}
		}
	}
	if maxRibbon < minRibbon*3 {
		t.Fatalf("ribbon lengths are not varied enough: min=%v max=%v", minRibbon, maxRibbon)
	}
	if minAngle < 38 || maxAngle > 78 || maxAngle-minAngle < 34 {
		t.Fatalf("launch angles do not form a raised fan: %v..%v", minAngle, maxAngle)
	}

	particle := particles[0]
	_, launchY, _ := particle.positionAt(particle.delay)
	_, risingY, _ := particle.positionAt(particle.delay + 0.3)
	_, fallingY, _ := particle.positionAt(particle.delay + 5)
	if risingY >= launchY || fallingY <= risingY {
		t.Fatalf("particle arc did not rise then fall: launch=%v rising=%v falling=%v", launchY, risingY, fallingY)
	}

	fastParticle := particles[len(particles)-1]
	_, firstFallY, _ := fastParticle.positionAt(fastParticle.delay + 1.5)
	_, secondFallY, _ := fastParticle.positionAt(fastParticle.delay + 2)
	_, thirdFallY, _ := fastParticle.positionAt(fastParticle.delay + 2.5)
	if thirdFallY-secondFallY <= secondFallY-firstFallY {
		t.Fatalf("fall did not accelerate under gravity: positions=%v, %v, %v", firstFallY, secondFallY, thirdFallY)
	}
}
