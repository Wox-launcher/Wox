package svg

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/srwiley/rasterx"
	"golang.org/x/image/math/fixed"
)

type pathParser struct {
	data               string
	position           int
	path               rasterx.Path
	x, y               float64
	startX, startY     float64
	controlX, controlY float64
	lastCommand        byte
	hasOpenSubpath     bool
}

// parsePath compiles the complete SVG path command set into a rasterx path.
func parsePath(data string) (rasterx.Path, error) {
	parser := &pathParser{data: data}
	if err := parser.parse(); err != nil {
		return nil, err
	}
	return parser.path, nil
}

func (p *pathParser) parse() error {
	var command byte
	for {
		p.skipSeparators()
		if p.position >= len(p.data) {
			return nil
		}
		if isPathCommand(p.data[p.position]) {
			command = p.data[p.position]
			p.position++
		} else if command == 0 {
			return p.errorf("expected path command")
		}

		start := p.position
		if err := p.execute(command); err != nil {
			return err
		}
		if command != 'Z' && command != 'z' && p.position == start {
			return p.errorf("command %c has no parameters", command)
		}
		if command == 'M' {
			command = 'L'
		} else if command == 'm' {
			command = 'l'
		} else if command == 'Z' || command == 'z' {
			command = 0
		}
	}
}

func (p *pathParser) execute(command byte) error {
	relative := command >= 'a' && command <= 'z'
	switch command {
	case 'M', 'm':
		first := true
		for p.hasNumber() {
			x, y, err := p.readPair()
			if err != nil {
				return err
			}
			if relative {
				x += p.x
				y += p.y
			}
			if first {
				p.path.Start(toFixedPoint(x, y))
				p.startX, p.startY = x, y
				p.hasOpenSubpath = true
				first = false
				p.lastCommand = command
			} else {
				p.path.Line(toFixedPoint(x, y))
				p.lastCommand = mapRelativeCommand(command, 'L')
			}
			p.x, p.y = x, y
		}
	case 'L', 'l':
		for p.hasNumber() {
			x, y, err := p.readPair()
			if err != nil {
				return err
			}
			if relative {
				x += p.x
				y += p.y
			}
			p.path.Line(toFixedPoint(x, y))
			p.x, p.y = x, y
			p.lastCommand = command
		}
	case 'H', 'h':
		for p.hasNumber() {
			x, err := p.readNumber()
			if err != nil {
				return err
			}
			if relative {
				x += p.x
			}
			p.path.Line(toFixedPoint(x, p.y))
			p.x = x
			p.lastCommand = command
		}
	case 'V', 'v':
		for p.hasNumber() {
			y, err := p.readNumber()
			if err != nil {
				return err
			}
			if relative {
				y += p.y
			}
			p.path.Line(toFixedPoint(p.x, y))
			p.y = y
			p.lastCommand = command
		}
	case 'C', 'c':
		for p.hasNumber() {
			x1, y1, err := p.readPair()
			if err != nil {
				return err
			}
			x2, y2, err := p.readPair()
			if err != nil {
				return err
			}
			x, y, err := p.readPair()
			if err != nil {
				return err
			}
			if relative {
				x1, y1 = x1+p.x, y1+p.y
				x2, y2 = x2+p.x, y2+p.y
				x, y = x+p.x, y+p.y
			}
			p.path.CubeBezier(toFixedPoint(x1, y1), toFixedPoint(x2, y2), toFixedPoint(x, y))
			p.controlX, p.controlY = x2, y2
			p.x, p.y = x, y
			p.lastCommand = command
		}
	case 'S', 's':
		for p.hasNumber() {
			x2, y2, err := p.readPair()
			if err != nil {
				return err
			}
			x, y, err := p.readPair()
			if err != nil {
				return err
			}
			if relative {
				x2, y2 = x2+p.x, y2+p.y
				x, y = x+p.x, y+p.y
			}
			x1, y1 := p.x, p.y
			if commandMatches(p.lastCommand, 'C', 'S') {
				x1, y1 = 2*p.x-p.controlX, 2*p.y-p.controlY
			}
			p.path.CubeBezier(toFixedPoint(x1, y1), toFixedPoint(x2, y2), toFixedPoint(x, y))
			p.controlX, p.controlY = x2, y2
			p.x, p.y = x, y
			p.lastCommand = command
		}
	case 'Q', 'q':
		for p.hasNumber() {
			x1, y1, err := p.readPair()
			if err != nil {
				return err
			}
			x, y, err := p.readPair()
			if err != nil {
				return err
			}
			if relative {
				x1, y1 = x1+p.x, y1+p.y
				x, y = x+p.x, y+p.y
			}
			p.path.QuadBezier(toFixedPoint(x1, y1), toFixedPoint(x, y))
			p.controlX, p.controlY = x1, y1
			p.x, p.y = x, y
			p.lastCommand = command
		}
	case 'T', 't':
		for p.hasNumber() {
			x, y, err := p.readPair()
			if err != nil {
				return err
			}
			if relative {
				x, y = x+p.x, y+p.y
			}
			x1, y1 := p.x, p.y
			if commandMatches(p.lastCommand, 'Q', 'T') {
				x1, y1 = 2*p.x-p.controlX, 2*p.y-p.controlY
			}
			p.path.QuadBezier(toFixedPoint(x1, y1), toFixedPoint(x, y))
			p.controlX, p.controlY = x1, y1
			p.x, p.y = x, y
			p.lastCommand = command
		}
	case 'A', 'a':
		for p.hasNumber() {
			rx, err := p.readNumber()
			if err != nil {
				return err
			}
			ry, err := p.readNumber()
			if err != nil {
				return err
			}
			rotation, err := p.readNumber()
			if err != nil {
				return err
			}
			largeArc, err := p.readFlag()
			if err != nil {
				return err
			}
			sweep, err := p.readFlag()
			if err != nil {
				return err
			}
			x, y, err := p.readPair()
			if err != nil {
				return err
			}
			if relative {
				x, y = x+p.x, y+p.y
			}
			rx, ry = math.Abs(rx), math.Abs(ry)
			if rx == 0 || ry == 0 || (x == p.x && y == p.y) {
				p.path.Line(toFixedPoint(x, y))
			} else {
				parameters := []float64{rx, ry, rotation, largeArc, sweep, x, y}
				centerX, centerY := rasterx.FindEllipseCenter(&parameters[0], &parameters[1], rotation*math.Pi/180, p.x, p.y, x, y, sweep == 0, largeArc == 0)
				rasterx.AddArc(parameters, centerX, centerY, p.x, p.y, &p.path)
			}
			p.x, p.y = x, y
			p.lastCommand = command
		}
	case 'Z', 'z':
		if p.hasOpenSubpath {
			p.path.Stop(true)
			p.x, p.y = p.startX, p.startY
			p.hasOpenSubpath = false
		}
		p.lastCommand = command
	default:
		return p.errorf("unsupported path command %q", command)
	}
	return nil
}

func (p *pathParser) readPair() (float64, float64, error) {
	x, err := p.readNumber()
	if err != nil {
		return 0, 0, err
	}
	y, err := p.readNumber()
	return x, y, err
}

func (p *pathParser) readFlag() (float64, error) {
	p.skipSeparators()
	if p.position >= len(p.data) || (p.data[p.position] != '0' && p.data[p.position] != '1') {
		return 0, p.errorf("expected arc flag")
	}
	value := float64(p.data[p.position] - '0')
	p.position++
	return value, nil
}

func (p *pathParser) readNumber() (float64, error) {
	p.skipSeparators()
	start := p.position
	if p.position < len(p.data) && (p.data[p.position] == '+' || p.data[p.position] == '-') {
		p.position++
	}
	hasDigits := false
	for p.position < len(p.data) && p.data[p.position] >= '0' && p.data[p.position] <= '9' {
		hasDigits = true
		p.position++
	}
	if p.position < len(p.data) && p.data[p.position] == '.' {
		p.position++
		for p.position < len(p.data) && p.data[p.position] >= '0' && p.data[p.position] <= '9' {
			hasDigits = true
			p.position++
		}
	}
	if !hasDigits {
		return 0, p.errorf("expected number")
	}
	if p.position < len(p.data) && (p.data[p.position] == 'e' || p.data[p.position] == 'E') {
		exponent := p.position
		p.position++
		if p.position < len(p.data) && (p.data[p.position] == '+' || p.data[p.position] == '-') {
			p.position++
		}
		exponentDigits := p.position
		for p.position < len(p.data) && p.data[p.position] >= '0' && p.data[p.position] <= '9' {
			p.position++
		}
		if exponentDigits == p.position {
			p.position = exponent
		}
	}
	value, err := strconv.ParseFloat(p.data[start:p.position], 64)
	if err != nil {
		return 0, p.errorf("invalid number: %v", err)
	}
	return value, nil
}

func (p *pathParser) hasNumber() bool {
	p.skipSeparators()
	if p.position >= len(p.data) {
		return false
	}
	value := p.data[p.position]
	return value == '+' || value == '-' || value == '.' || (value >= '0' && value <= '9')
}

func (p *pathParser) skipSeparators() {
	for p.position < len(p.data) {
		switch p.data[p.position] {
		case ' ', '\t', '\r', '\n', ',':
			p.position++
		default:
			return
		}
	}
}

func (p *pathParser) errorf(format string, args ...any) error {
	end := min(p.position+20, len(p.data))
	return fmt.Errorf("svg path at byte %d near %q: %s", p.position, p.data[p.position:end], fmt.Sprintf(format, args...))
}

func toFixedPoint(x, y float64) fixed.Point26_6 {
	return fixed.Point26_6{X: fixed.Int26_6(math.Round(x * 64)), Y: fixed.Int26_6(math.Round(y * 64))}
}

func isPathCommand(value byte) bool {
	return strings.ContainsRune("MmZzLlHhVvCcSsQqTtAa", rune(value))
}

func commandMatches(actual byte, commands ...byte) bool {
	actual = byte(strings.ToUpper(string(actual))[0])
	for _, command := range commands {
		if actual == command {
			return true
		}
	}
	return false
}

func mapRelativeCommand(source, absolute byte) byte {
	if source >= 'a' && source <= 'z' {
		return absolute + ('a' - 'A')
	}
	return absolute
}
