package svg

import (
	"image/color"
	"strconv"
	"strings"
	"time"

	"github.com/srwiley/rasterx"
)

const (
	// svgFrameStep is the nominal sampling interval for an SMIL cycle.
	svgFrameStep = 50 * time.Millisecond
	// svgMaxFrames caps how many rasters one icon keeps. Playback loops, so this
	// bounds memory rather than cutting the animation short.
	svgMaxFrames = 16
	// svgFiniteHold keeps a non-repeating animation on its last pose. The widget
	// timeline always loops, and this delay makes the restart rare.
	svgFiniteHold = 30 * time.Second
	// svgTimelineCap stops mismatched durations from building a huge sample.
	svgTimelineCap = 4 * time.Second
)

// svgAnimation is one SMIL animation attached to a shape or group.
type svgAnimation struct {
	kind          string
	attribute     string
	transformType string
	values        []string
	begin         time.Duration
	dur           time.Duration
	repeat        float64
	indefinite    bool
	freeze        bool
	additive      bool
	discrete      bool
	keyTimes      []float64
}

// isAnimationElement reports SMIL tags whose fill attribute is a timing mode.
func isAnimationElement(tag string) bool {
	switch tag {
	case "animate", "animateTransform", "animateMotion", "set":
		return true
	default:
		return false
	}
}

// readAnimation parses one animation. Malformed or unsupported animations are
// skipped so the static icon still draws.
func readAnimation(tag string, attributes map[string]string) (svgAnimation, bool) {
	if tag == "animateMotion" {
		return svgAnimation{}, false
	}
	anim := svgAnimation{
		kind: tag, attribute: attributes["attributename"], transformType: strings.ToLower(attributes["type"]),
		repeat: 1,
	}
	if anim.attribute == "" {
		anim.attribute = "transform"
	}
	if value := attributes["begin"]; value != "" {
		begin, ok := parseClockList(value)
		if !ok {
			return svgAnimation{}, false
		}
		anim.begin = begin
	}
	if tag != "set" {
		dur, ok := parseClock(attributes["dur"])
		if !ok || dur <= 0 {
			return svgAnimation{}, false
		}
		anim.dur = dur
	} else if dur, ok := parseClock(attributes["dur"]); ok && dur > 0 {
		anim.dur = dur
		anim.discrete = true
	} else {
		return svgAnimation{}, false
	}
	if value := attributes["repeatcount"]; value != "" {
		if strings.EqualFold(value, "indefinite") {
			anim.indefinite = true
		} else if parsed, err := strconv.ParseFloat(value, 64); err == nil && parsed > 0 {
			anim.repeat = parsed
		}
	}
	if value := attributes["repeatdur"]; value != "" && !anim.indefinite {
		if repeatDur, ok := parseClock(value); ok && anim.dur > 0 {
			anim.repeat = float64(repeatDur) / float64(anim.dur)
		}
	}
	anim.freeze = strings.EqualFold(attributes["fill"], "freeze")
	anim.additive = strings.EqualFold(attributes["additive"], "sum")
	anim.discrete = anim.discrete || strings.EqualFold(attributes["calcmode"], "discrete")
	if value := attributes["values"]; value != "" {
		for _, part := range strings.Split(value, ";") {
			part = strings.TrimSpace(part)
			if part != "" {
				anim.values = append(anim.values, part)
			}
		}
	} else if tag == "set" {
		if attributes["to"] == "" {
			return svgAnimation{}, false
		}
		anim.values = []string{attributes["to"]}
	} else if attributes["from"] != "" || attributes["to"] != "" {
		from := attributes["from"]
		to := attributes["to"]
		if to == "" {
			to = from
		}
		if from == "" && tag == "animateTransform" {
			from = "0"
		}
		anim.values = []string{from, to}
	}
	if len(anim.values) == 0 {
		return svgAnimation{}, false
	}
	if value := attributes["keytimes"]; value != "" {
		for _, part := range strings.Split(value, ";") {
			parsed, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
			if err != nil {
				anim.keyTimes = nil
				break
			}
			anim.keyTimes = append(anim.keyTimes, parsed)
		}
		if len(anim.keyTimes) != len(anim.values) {
			anim.keyTimes = nil
		}
	}
	return anim, true
}

// addAnimation attaches an animation to the nearest open shape, or else the nearest group.
func (icon *Icon) addAnimation(stack []openElement, anim svgAnimation) {
	for index := len(stack) - 1; index >= 0; index-- {
		node := stack[index]
		if node.hasShape {
			if node.maskID != "" {
				shapes := icon.masks[node.maskID]
				if node.shape >= 0 && node.shape < len(shapes) {
					shapes[node.shape].animations = append(shapes[node.shape].animations, anim)
					icon.masks[node.maskID] = shapes
				}
				return
			}
			if node.shape >= 0 && node.shape < len(icon.shapes) {
				icon.shapes[node.shape].animations = append(icon.shapes[node.shape].animations, anim)
			}
			return
		}
		if node.group >= 0 && node.group < len(icon.groups) {
			icon.groups[node.group].animations = append(icon.groups[node.group].animations, anim)
			return
		}
	}
}

// animationTimeline reports the sample length and whether any animation repeats forever.
func (icon *Icon) animationTimeline() (time.Duration, bool) {
	var cycles []time.Duration
	indefinite := false
	consider := func(anims []svgAnimation) {
		for _, anim := range anims {
			if anim.dur <= 0 {
				continue
			}
			cycle := anim.dur
			if anim.indefinite {
				indefinite = true
			} else if anim.repeat > 0 {
				cycle = time.Duration(float64(anim.dur) * anim.repeat)
			}
			cycles = append(cycles, cycle)
		}
	}
	for _, shape := range icon.shapes {
		consider(shape.animations)
	}
	for _, group := range icon.groups {
		consider(group.animations)
	}
	for _, shapes := range icon.masks {
		for _, shape := range shapes {
			consider(shape.animations)
		}
	}
	if len(cycles) == 0 {
		return 0, false
	}
	duration := cycles[0]
	for _, cycle := range cycles[1:] {
		duration = lcmDuration(duration, cycle)
		if duration > svgTimelineCap {
			return svgTimelineCap, indefinite
		}
	}
	if duration > svgTimelineCap {
		return svgTimelineCap, indefinite
	}
	return duration, indefinite
}

func lcmDuration(left, right time.Duration) time.Duration {
	a := left.Milliseconds()
	b := right.Milliseconds()
	if a <= 0 || b <= 0 {
		if left > right {
			return left
		}
		return right
	}
	return time.Duration(a/gcd(a, b)*b) * time.Millisecond
}

func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 {
		return -a
	}
	return a
}

// resolveShape applies the timeline to one shape and bakes the matrix used for painting.
func (icon *Icon) resolveShape(shape svgShape, target rasterx.Matrix2D, at time.Duration) (svgShape, error) {
	resolved := shape
	attributes := cloneAttributes(shape.attributes)
	geometry := false
	for _, anim := range shape.animations {
		if anim.kind == "animateTransform" {
			continue
		}
		base := ""
		if attributes != nil {
			base = attributes[anim.attribute]
		}
		value, ok := anim.sample(at, base)
		if !ok {
			continue
		}
		if attributes == nil {
			attributes = map[string]string{}
		}
		attributes[anim.attribute] = value
		if isGeometryAttribute(anim.attribute) {
			geometry = true
		}
		applyAnimatedStyle(&resolved.style, anim.attribute, value)
	}
	if geometry {
		path, recognized, err := readShape(shape.tag, attributes)
		if err != nil {
			return shape, err
		}
		if recognized && len(path) > 0 {
			resolved.path = path
		}
	}
	resolved.style.opacity = icon.composedOpacity(shape, at)
	resolved.style.matrix = icon.composedMatrix(shape, target, at)
	resolved.localMatrix = resolved.style.matrix
	return resolved, nil
}

func cloneAttributes(attributes map[string]string) map[string]string {
	if len(attributes) == 0 {
		return nil
	}
	clone := make(map[string]string, len(attributes))
	for key, value := range attributes {
		clone[key] = value
	}
	return clone
}

func isGeometryAttribute(name string) bool {
	switch name {
	case "d", "x", "y", "x1", "y1", "x2", "y2", "width", "height", "cx", "cy", "r", "rx", "ry", "points":
		return true
	default:
		return false
	}
}

func applyAnimatedStyle(style *pathStyle, attribute, value string) {
	switch attribute {
	case "fill":
		if paintValue, err := parsePaint(value, style.currentColor); err == nil {
			style.fill = paintValue
		}
	case "stroke":
		if paintValue, err := parsePaint(value, style.currentColor); err == nil {
			style.stroke = paintValue
		}
	case "fill-opacity":
		if opacity, err := parseFraction(value); err == nil {
			style.fillOpacity = clampUnit(opacity)
		}
	case "stroke-opacity":
		if opacity, err := parseFraction(value); err == nil {
			style.strokeOpacity = clampUnit(opacity)
		}
	case "stroke-width":
		if width, err := parseLength(value); err == nil {
			style.lineWidth = width
		}
	case "stroke-dashoffset":
		if offset, err := parseLength(value); err == nil {
			style.dashOffset = offset
		}
	case "stroke-dasharray":
		if strings.EqualFold(value, "none") {
			style.dashes = nil
			return
		}
		if dashes, err := parseNumberList(value); err == nil {
			style.dashes = dashes
		}
	}
}

func (icon *Icon) composedMatrix(shape svgShape, target rasterx.Matrix2D, at time.Duration) rasterx.Matrix2D {
	matrix := target
	for _, id := range icon.groupChain(shape.parentGroup) {
		matrix = matrix.Mult(icon.groups[id].matrixAt(at))
	}
	return matrix.Mult(shape.matrixAt(at))
}

func (icon *Icon) composedOpacity(shape svgShape, at time.Duration) float64 {
	opacity := 1.0
	for _, id := range icon.groupChain(shape.parentGroup) {
		opacity *= icon.groups[id].opacityAt(at)
	}
	return opacity * shape.opacityAt(at)
}

func (icon *Icon) groupChain(id int) []int {
	chain := []int{}
	for id >= 0 && id < len(icon.groups) && len(chain) < 64 {
		chain = append(chain, id)
		id = icon.groups[id].parent
	}
	for left, right := 0, len(chain)-1; left < right; left, right = left+1, right-1 {
		chain[left], chain[right] = chain[right], chain[left]
	}
	return chain
}

func (group svgGroup) matrixAt(at time.Duration) rasterx.Matrix2D {
	return matrixAt(group.animations, group.localMatrix, at)
}

func (shape svgShape) matrixAt(at time.Duration) rasterx.Matrix2D {
	return matrixAt(shape.animations, shape.localMatrix, at)
}

func matrixAt(animations []svgAnimation, base rasterx.Matrix2D, at time.Duration) rasterx.Matrix2D {
	matrix := base
	replaced := false
	for _, anim := range animations {
		if anim.kind != "animateTransform" {
			continue
		}
		value, ok := anim.sample(at, "")
		if !ok || value == "" {
			continue
		}
		local, err := parseTransform(rasterx.Identity, value)
		if err != nil {
			continue
		}
		if anim.additive {
			if !replaced {
				matrix = base.Mult(local)
			} else {
				matrix = matrix.Mult(local)
			}
			continue
		}
		matrix = local
		replaced = true
	}
	return matrix
}

func (group svgGroup) opacityAt(at time.Duration) float64 {
	return opacityAt(group.animations, group.localOpacity, at)
}

func (shape svgShape) opacityAt(at time.Duration) float64 {
	return opacityAt(shape.animations, shape.localOpacity, at)
}

func opacityAt(animations []svgAnimation, base float64, at time.Duration) float64 {
	opacity := base
	for _, anim := range animations {
		if anim.attribute != "opacity" {
			continue
		}
		value, ok := anim.sample(at, strconv.FormatFloat(base, 'f', -1, 64))
		if !ok {
			continue
		}
		parsed, err := parseFraction(value)
		if err != nil {
			continue
		}
		opacity = clampUnit(parsed)
	}
	return opacity
}

// sample returns the animated value when the interval is active.
func (anim svgAnimation) sample(at time.Duration, base string) (string, bool) {
	if at < anim.begin || anim.dur <= 0 || len(anim.values) == 0 {
		return base, false
	}
	elapsed := at - anim.begin
	if !anim.indefinite && anim.repeat > 0 {
		limit := time.Duration(float64(anim.dur) * anim.repeat)
		if elapsed >= limit {
			if anim.freeze || anim.kind == "set" {
				return anim.values[len(anim.values)-1], true
			}
			return base, false
		}
	}
	if anim.kind == "set" || anim.discrete || len(anim.values) == 1 {
		return anim.values[len(anim.values)-1], true
	}
	values := anim.values
	if values[0] == "" {
		values = append([]string{base}, values[1:]...)
	}
	progress := float64(elapsed%anim.dur) / float64(anim.dur)
	if !anim.indefinite && anim.repeat > 0 {
		limit := time.Duration(float64(anim.dur) * anim.repeat)
		if elapsed >= limit-time.Millisecond {
			progress = 1
		}
	}
	return interpolateAnimation(anim, values, progress), true
}

func interpolateAnimation(anim svgAnimation, values []string, progress float64) string {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	count := len(values)
	if count == 1 {
		return values[0]
	}
	left, local := 0, progress
	if len(anim.keyTimes) == count {
		for index := 1; index < count; index++ {
			if progress <= anim.keyTimes[index] || index == count-1 {
				span := anim.keyTimes[index] - anim.keyTimes[index-1]
				left = index - 1
				if span <= 0 {
					local = 0
				} else {
					local = (progress - anim.keyTimes[index-1]) / span
				}
				break
			}
		}
	} else {
		scaled := progress * float64(count-1)
		left = int(scaled)
		if left >= count-1 {
			left = count - 2
			local = 1
		} else {
			local = scaled - float64(left)
		}
	}
	if anim.discrete {
		if local < 1 {
			return values[left]
		}
		return values[left+1]
	}
	from := values[left]
	to := values[left+1]
	if anim.kind == "animateTransform" {
		return interpolateTransform(anim.transformType, from, to, local)
	}
	if fromColor, ok := paintColor(from); ok {
		if toColor, ok := paintColor(to); ok {
			return formatColor(lerpColor(fromColor, toColor, local))
		}
	}
	fromNumbers, fromOK := parseNumberList(from)
	toNumbers, toOK := parseNumberList(to)
	if fromOK == nil && toOK == nil && len(fromNumbers) > 0 && len(fromNumbers) == len(toNumbers) {
		parts := make([]string, len(fromNumbers))
		for index := range fromNumbers {
			parts[index] = strconv.FormatFloat(fromNumbers[index]+(toNumbers[index]-fromNumbers[index])*local, 'f', -1, 64)
		}
		return strings.Join(parts, " ")
	}
	if local < 0.5 {
		return from
	}
	return to
}

func interpolateTransform(kind, from, to string, progress float64) string {
	if kind == "" {
		kind = "rotate"
	}
	fromNumbers, fromErr := parseNumberList(from)
	toNumbers, toErr := parseNumberList(to)
	if fromErr != nil || toErr != nil || len(fromNumbers) == 0 || len(fromNumbers) != len(toNumbers) {
		if progress < 1 {
			return kind + "(" + from + ")"
		}
		return kind + "(" + to + ")"
	}
	parts := make([]string, len(fromNumbers))
	for index := range fromNumbers {
		parts[index] = strconv.FormatFloat(fromNumbers[index]+(toNumbers[index]-fromNumbers[index])*progress, 'f', -1, 64)
	}
	return kind + "(" + strings.Join(parts, " ") + ")"
}

func paintColor(value string) (color.NRGBA, bool) {
	parsed, err := parsePaint(value, color.NRGBA{A: 255})
	if err != nil || parsed.disabled || parsed.gradientID != "" || parsed.color == nil {
		return color.NRGBA{}, false
	}
	converted := color.NRGBAModel.Convert(parsed.color).(color.NRGBA)
	return converted, true
}

func lerpColor(from, to color.NRGBA, progress float64) color.NRGBA {
	blend := func(start, end uint8) uint8 {
		return uint8(float64(start) + (float64(end)-float64(start))*progress)
	}
	return color.NRGBA{R: blend(from.R, to.R), G: blend(from.G, to.G), B: blend(from.B, to.B), A: blend(from.A, to.A)}
}

func formatColor(value color.NRGBA) string {
	return "#" + hexByte(value.R) + hexByte(value.G) + hexByte(value.B) + hexByte(value.A)
}

func hexByte(value uint8) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[value>>4], digits[value&0x0f]})
}

func parseClockList(value string) (time.Duration, bool) {
	for _, part := range strings.Split(value, ";") {
		if parsed, ok := parseClock(part); ok {
			return parsed, true
		}
	}
	return 0, false
}

// parseClock reads an SVG/SMIL clock value. A bare number is seconds.
func parseClock(value string) (time.Duration, bool) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" || value == "indefinite" {
		return 0, false
	}
	if strings.Contains(value, ":") {
		parts := strings.Split(value, ":")
		if len(parts) != 2 && len(parts) != 3 {
			return 0, false
		}
		numbers := make([]float64, len(parts))
		for index, part := range parts {
			parsed, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
			if err != nil {
				return 0, false
			}
			numbers[index] = parsed
		}
		seconds := numbers[len(numbers)-1]
		minutes := numbers[len(numbers)-2]
		hours := 0.0
		if len(numbers) == 3 {
			hours = numbers[0]
		}
		return time.Duration((hours*3600 + minutes*60 + seconds) * float64(time.Second)), true
	}
	scale := time.Second
	switch {
	case strings.HasSuffix(value, "ms"):
		scale = time.Millisecond
		value = strings.TrimSuffix(value, "ms")
	case strings.HasSuffix(value, "min"):
		scale = time.Minute
		value = strings.TrimSuffix(value, "min")
	case strings.HasSuffix(value, "h"):
		scale = time.Hour
		value = strings.TrimSuffix(value, "h")
	case strings.HasSuffix(value, "s"):
		value = strings.TrimSuffix(value, "s")
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, false
	}
	return time.Duration(parsed * float64(scale)), true
}
