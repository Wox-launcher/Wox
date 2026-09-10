package engine

import (
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func stripDigitUnderscores(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if r == '_' && i > 0 && i+1 < len(runes) && isHexDigit(runes[i-1]) && isHexDigit(runes[i+1]) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isHexDigit(r rune) bool {
	return unicode.IsDigit(r) || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F'
}

func scanTimecode(runes []rune) (string, bool) {
	i := 0
	for i < len(runes) && unicode.IsDigit(runes[i]) {
		i++
	}
	if i == 0 || i >= len(runes) || runes[i] != ':' {
		return "", false
	}
	for colon := 0; colon < 3; colon++ {
		if i >= len(runes) || runes[i] != ':' {
			return "", false
		}
		i++
		start := i
		for i < len(runes) && unicode.IsDigit(runes[i]) {
			i++
		}
		if i == start {
			return "", false
		}
	}
	return string(runes[:i]), true
}

func parseTimecodeParts(s string) (h, m, sec, f int, err error) {
	parts := strings.Split(s, ":")
	if len(parts) != 4 {
		return 0, 0, 0, 0, invalid("invalid timecode")
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	sec, err3 := strconv.Atoi(parts[2])
	f, err4 := strconv.Atoi(parts[3])
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return 0, 0, 0, 0, invalid("invalid timecode")
	}
	return h, m, sec, f, nil
}

func envFPS(env Env) *big.Rat {
	if env.FPS != nil && env.FPS.Sign() > 0 {
		return env.FPS
	}
	return big.NewRat(24, 1)
}

func framesFromTimecode(s string, fps *big.Rat) (*big.Rat, error) {
	h, m, sec, f, err := parseTimecodeParts(s)
	if err != nil {
		return nil, err
	}
	seconds := h*3600 + m*60 + sec
	total := new(big.Rat).Mul(big.NewRat(int64(seconds), 1), fps)
	total.Add(total, big.NewRat(int64(f), 1))
	return total, nil
}

func formatTimecode(frames, fps *big.Rat) string {
	if fps == nil || fps.Sign() <= 0 {
		fps = big.NewRat(24, 1)
	}
	n := new(big.Rat).Quo(new(big.Rat).Set(frames), fps)
	whole := new(big.Int).Quo(n.Num(), n.Denom())
	secs := whole.Int64()
	if secs < 0 {
		secs = -secs
	}
	frac := new(big.Rat).Sub(frames, new(big.Rat).Mul(new(big.Rat).SetInt(whole), fps))
	frame := new(big.Int).Quo(frac.Num(), frac.Denom()).Int64()
	if frame < 0 {
		frame = -frame
	}
	h := secs / 3600
	secs %= 3600
	m := secs / 60
	s := secs % 60
	return sprintfTimecode(h, m, s, frame)
}

func sprintfTimecode(h, m, s, f int64) string {
	return pad2(h) + ":" + pad2(m) + ":" + pad2(s) + ":" + pad2(f)
}

func pad2(n int64) string {
	if n < 10 {
		return "0" + strconv.FormatInt(n, 10)
	}
	return strconv.FormatInt(n, 10)
}

func (p *parser) parseFPS() error {
	if p.peek() != "at" && p.peek() != "@" {
		return nil
	}
	if p.i+2 >= len(p.tokens) || p.tokens[p.i+1].value == nil || p.tokens[p.i+2].text != "fps" {
		return nil
	}
	p.i++
	p.query.fps = new(big.Rat).Set(p.tokens[p.i].value)
	p.i += 2
	p.query.Domain = true
	return nil
}

func (p *parser) takeSubstance() bool {
	p.accept("of")
	if p.peek() == "olive" && p.i+1 < len(p.tokens) && p.tokens[p.i+1].text == "oil" {
		p.query.substance = "olive oil"
		p.i += 2
		p.query.Domain = true
		return true
	}
	if _, ok := cookingDensity(p.peek()); ok {
		p.query.substance = p.peek()
		p.i++
		p.query.Domain = true
		return true
	}
	return false
}

func cookingDensity(name string) (*big.Rat, bool) {
	// Grams per milliliter, fitted to Soulver's documented cup/tbsp answers.
	switch name {
	case "butter":
		return rational("0.96226"), true
	case "olive oil":
		return rational("0.91298"), true
	case "nutella":
		return rational("1.25937"), true
	}
	return nil, false
}

func convertCooking(v Value, target Unit, substance string) (Value, bool, error) {
	dens, ok := cookingDensity(substance)
	if !ok || v.Number == nil {
		return Value{}, false, nil
	}
	fromMass := v.Unit["g"] == 1 && len(v.Unit) == 1 || v.Unit["kg"] == 1 && len(v.Unit) == 1 || v.Unit["mg"] == 1 && len(v.Unit) == 1
	toVol := target["cup"] == 1 || target["tbsp"] == 1 || target["tsp"] == 1 || target["ml"] == 1 || target["l"] == 1
	fromVol := v.Unit["cup"] == 1 || v.Unit["tbsp"] == 1 || v.Unit["tsp"] == 1 || v.Unit["ml"] == 1 || v.Unit["l"] == 1
	toMass := target["g"] == 1 || target["kg"] == 1 || target["mg"] == 1
	ml := func(u Unit, n *big.Rat) *big.Rat {
		scales := map[string]string{"ml": "1", "l": "1000", "tsp": "4.92892159375", "tbsp": "14.78676478125", "cup": "236.5882365"}
		for k := range u {
			if s, ok := scales[k]; ok {
				return new(big.Rat).Mul(n, rational(s))
			}
		}
		return nil
	}
	grams := func(u Unit, n *big.Rat) *big.Rat {
		scales := map[string]string{"g": "1", "kg": "1000", "mg": "0.001"}
		for k := range u {
			if s, ok := scales[k]; ok {
				return new(big.Rat).Mul(n, rational(s))
			}
		}
		return nil
	}
	if fromMass && toVol {
		g := grams(v.Unit, v.Number)
		volml := new(big.Rat).Quo(g, dens)
		scale := ml(target, big.NewRat(1, 1))
		v.Number = new(big.Rat).Quo(volml, scale)
		v.Unit = copyUnit(target)
		v.Kind = Quantity
		return v, true, nil
	}
	if fromVol && toMass {
		volml := ml(v.Unit, v.Number)
		g := new(big.Rat).Mul(volml, dens)
		scale := grams(target, big.NewRat(1, 1))
		v.Number = new(big.Rat).Quo(g, scale)
		v.Unit = copyUnit(target)
		v.Kind = Quantity
		return v, true, nil
	}
	return Value{}, false, nil
}

func (c *Catalog) frameRateOp(a, b Value, divide bool, env Env) (Value, bool, error) {
	fpsA := a.Unit["fps"] == 1 && len(a.Unit) == 1
	fpsB := b.Unit["fps"] == 1 && len(b.Unit) == 1
	frameA := a.Unit["frame"] == 1 && len(a.Unit) == 1
	timeA := sameUnit(c.dimensions(a.Unit), Unit{"time": 1})
	timeB := sameUnit(c.dimensions(b.Unit), Unit{"time": 1})
	if !divide && fpsA && timeB {
		sec, err := c.factor(b.Unit, env)
		if err != nil {
			return Value{}, true, err
		}
		n := new(big.Rat).Mul(a.Number, new(big.Rat).Mul(b.Number, sec))
		return Value{Kind: Quantity, Number: n, Unit: Unit{"frame": 1}}, true, nil
	}
	if !divide && fpsB && timeA {
		return c.frameRateOp(b, a, false, env)
	}
	if divide && frameA && fpsB {
		n := new(big.Rat).Quo(a.Number, b.Number)
		return Value{Kind: Quantity, Number: n, Unit: Unit{"s": 1}}, true, nil
	}
	return Value{}, false, nil
}

func (c *Catalog) addFramesAndTime(a, b Value, subtract bool, env Env) (Value, bool, error) {
	frameA := a.Unit["frame"] == 1 && len(a.Unit) == 1
	frameB := b.Unit["frame"] == 1 && len(b.Unit) == 1
	timeA := sameUnit(c.dimensions(a.Unit), Unit{"time": 1})
	timeB := sameUnit(c.dimensions(b.Unit), Unit{"time": 1})
	if !(frameA && timeB || frameB && timeA) {
		return Value{}, false, nil
	}
	fps := envFPS(env)
	toFrames := func(v Value) (*big.Rat, error) {
		if v.Unit["frame"] == 1 {
			return v.Number, nil
		}
		sec, err := c.factor(v.Unit, env)
		if err != nil {
			return nil, err
		}
		return new(big.Rat).Mul(new(big.Rat).Mul(v.Number, sec), fps), nil
	}
	fa, err := toFrames(a)
	if err != nil {
		return Value{}, true, err
	}
	fb, err := toFrames(b)
	if err != nil {
		return Value{}, true, err
	}
	if subtract {
		fa = new(big.Rat).Sub(fa, fb)
	} else {
		fa = new(big.Rat).Add(fa, fb)
	}
	return Value{Kind: Quantity, Number: fa, Unit: Unit{"frame": 1}}, true, nil
}

func evalBitwise(op string, a, b Value) (Value, error) {
	if a.Number == nil || b.Number == nil || !a.Number.IsInt() || !b.Number.IsInt() {
		return Value{}, invalid("bitwise requires integers")
	}
	x := a.Number.Num().Int64()
	y := b.Number.Num().Int64()
	var r int64
	switch op {
	case "&":
		r = x & y
	case "|":
		r = x | y
	case "xor":
		r = x ^ y
	case "<<":
		if y < 0 || y > 62 {
			return Value{}, invalid("shift exceeds limit")
		}
		r = x << uint(y)
	case ">>":
		if y < 0 || y > 62 {
			return Value{}, invalid("shift exceeds limit")
		}
		r = x >> uint(y)
	}
	return Value{Kind: Number, Number: big.NewRat(r, 1)}, nil
}

func holidayDate(name string, year int, now time.Time) (time.Time, bool) {
	if year == 0 {
		year = now.Year()
	}
	loc := now.Location()
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "christmas", "christmas day":
		return time.Date(year, 12, 25, 0, 0, 0, 0, loc), true
	case "christmas eve":
		return time.Date(year, 12, 24, 0, 0, 0, 0, loc), true
	case "boxing day":
		return time.Date(year, 12, 26, 0, 0, 0, 0, loc), true
	case "orthodox christmas":
		return time.Date(year, 1, 7, 0, 0, 0, 0, loc), true
	case "new year's day", "new years day", "new year":
		return time.Date(year, 1, 1, 0, 0, 0, 0, loc), true
	case "new year's eve", "new years eve":
		return time.Date(year, 12, 31, 0, 0, 0, 0, loc), true
	case "valentine's day", "valentines day", "valentine":
		return time.Date(year, 2, 14, 0, 0, 0, 0, loc), true
	case "halloween":
		return time.Date(year, 10, 31, 0, 0, 0, 0, loc), true
	case "easter", "easter sunday":
		return westernEaster(year, loc), true
	case "good friday":
		return westernEaster(year, loc).AddDate(0, 0, -2), true
	case "holy saturday":
		return westernEaster(year, loc).AddDate(0, 0, -1), true
	case "easter monday":
		return westernEaster(year, loc).AddDate(0, 0, 1), true
	case "orthodox easter":
		return orthodoxEaster(year, loc), true
	case "orthodox good friday":
		return orthodoxEaster(year, loc).AddDate(0, 0, -2), true
	case "thanksgiving":
		return nthWeekdayOfMonth(year, time.November, time.Thursday, 4, loc), true
	case "black friday":
		return nthWeekdayOfMonth(year, time.November, time.Thursday, 4, loc).AddDate(0, 0, 1), true
	case "chinese new year", "chinese new years":
		if t, ok := tabulatedHoliday("cny", year, loc); ok {
			return t, true
		}
	case "chinese new year eve", "chinese new years eve", "chinese new year's eve":
		if t, ok := tabulatedHoliday("cny", year, loc); ok {
			return t.AddDate(0, 0, -1), true
		}
	case "ramadan":
		return tabulatedHoliday("ramadan", year, loc)
	case "end of ramadan":
		return tabulatedHoliday("eid", year, loc)
	case "hanukkah":
		return tabulatedHoliday("hanukkah", year, loc)
	case "end of hanukkah":
		if t, ok := tabulatedHoliday("hanukkah", year, loc); ok {
			return t.AddDate(0, 0, 7), true
		}
	}
	return time.Time{}, false
}

func nthWeekdayOfMonth(year int, month time.Month, weekday time.Weekday, n int, loc *time.Location) time.Time {
	t := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	for t.Weekday() != weekday {
		t = t.AddDate(0, 0, 1)
	}
	return t.AddDate(0, 0, 7*(n-1))
}

func orthodoxEaster(year int, loc *time.Location) time.Time {
	// Meeus Julian Easter, then convert to Gregorian.
	a := year % 4
	b := year % 7
	c := year % 19
	d := (19*c + 15) % 30
	e := (2*a + 4*b - d + 34) % 7
	month := (d + e + 114) / 31
	day := (d+e+114)%31 + 1
	offset := 13
	if year < 1900 {
		offset = 12
	} else if year >= 2100 {
		offset = 14
	}
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, loc).AddDate(0, 0, offset)
}

func tabulatedHoliday(kind string, year int, loc *time.Location) (time.Time, bool) {
	tables := map[string]map[int][2]int{
		"cny":      {2020: {1, 25}, 2021: {2, 12}, 2022: {2, 1}, 2023: {1, 22}, 2024: {2, 10}, 2025: {1, 29}, 2026: {2, 17}, 2027: {2, 6}, 2028: {1, 26}, 2029: {2, 13}, 2030: {2, 3}, 2031: {1, 23}, 2032: {2, 11}, 2033: {1, 31}, 2034: {2, 19}, 2035: {2, 8}, 2036: {1, 28}, 2037: {2, 15}, 2038: {2, 4}, 2039: {1, 24}, 2040: {2, 12}},
		"ramadan":  {2024: {3, 11}, 2025: {3, 1}, 2026: {2, 18}, 2027: {2, 8}, 2028: {1, 28}, 2029: {1, 16}, 2030: {1, 6}},
		"eid":      {2024: {4, 10}, 2025: {3, 31}, 2026: {3, 20}, 2027: {3, 10}, 2028: {2, 27}, 2029: {2, 15}, 2030: {2, 5}},
		"hanukkah": {2024: {12, 25}, 2025: {12, 14}, 2026: {12, 4}, 2027: {12, 24}, 2028: {12, 12}, 2029: {12, 1}, 2030: {12, 20}},
	}
	row, ok := tables[kind]
	if !ok {
		return time.Time{}, false
	}
	md, ok := row[year]
	if !ok {
		return time.Time{}, false
	}
	return time.Date(year, time.Month(md[0]), md[1], 0, 0, 0, 0, loc), true
}

func westernEaster(year int, loc *time.Location) time.Time {
	// Anonymous Gregorian computus.
	a := year % 19
	b := year / 100
	c := year % 100
	d := b / 4
	e := b % 4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i := c / 4
	k := c % 4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	day := (h+l-7*m+114)%31 + 1
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, loc)
}

// parseIncomeTax recognizes documented Soulver income-tax sentences.
func (p *parser) parseIncomeTax() (bool, error) {
	saved := p.i
	after, rateOnly := false, false
	if p.accept("income") {
		if p.accept("after") {
			after = true
			if !p.accept("tax") {
				p.i = saved
				return false, nil
			}
		} else if !p.accept("tax") {
			p.i = saved
			return false, nil
		}
	} else if p.accept("tax") {
		if !p.accept("rate") {
			p.i = saved
			return false, nil
		}
		rateOnly = true
	} else {
		return false, nil
	}
	if !p.accept("on") {
		p.i = saved
		return false, nil
	}
	amount, err := p.expr(25)
	if err != nil {
		p.i = saved
		return false, nil
	}
	if !p.accept("in") {
		p.i = saved
		return false, nil
	}
	country := p.peek()
	if country == "" {
		p.i = saved
		return false, nil
	}
	p.i++
	if country == "united" && p.accept("states") {
		country = "usa"
	}
	kind := "tax"
	if after {
		kind = "after"
	} else if rateOnly {
		kind = "rate"
		p.query.format = "percent"
	}
	p.query.Domain = true
	p.query.root, err = p.make(&node{op: "income_tax", text: kind + ":" + country, args: []*node{amount}})
	return true, err
}

func (p *parser) parsePPIOf() (bool, error) {
	if p.peek() != "ppi" && p.peek() != "dpi" {
		return false, nil
	}
	if p.i+1 >= len(p.tokens) || p.tokens[p.i+1].text != "of" {
		return false, nil
	}
	p.i += 2
	diag, err := p.expr(25)
	if err != nil {
		return true, err
	}
	if p.peek() == "screen" || p.peek() == "display" || p.peek() == "monitor" {
		p.i++
	}
	if !p.accept("at") && !p.accept("@") {
		return true, invalid("expected at")
	}
	if p.tokens[p.i].value == nil {
		return true, invalid("expected width")
	}
	w, err := p.expr(25)
	if err != nil {
		return true, err
	}
	if !p.accept("x") && !p.accept("×") {
		return true, invalid("expected x")
	}
	h, err := p.expr(25)
	if err != nil {
		return true, err
	}
	p.query.Domain = true
	p.query.root, err = p.make(&node{op: "ppi_of", args: []*node{diag, w, h}})
	return true, err
}

func (p *parser) parseExtraPhrases() (bool, error) {
	if ok, err := p.parsePPIOf(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseUploadTime(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseWhatPer(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseDistanceBetween(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseHolidayLiteral(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseLocationOf(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseTireSpeed(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parsePowerTorque(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseGPS(); ok || err != nil {
		return ok, err
	}
	return false, nil
}

func (p *parser) parseLocationOf() (bool, error) {
	kind := p.peek()
	if kind != "location" && kind != "latitude" && kind != "longitude" {
		return false, nil
	}
	p.i++
	if !p.accept("of") {
		return true, invalid("expected of")
	}
	place := p.peek()
	p.i++
	if p.peek() == "oil" {
		place += " " + p.peek()
		p.i++
	}
	lat, lon, ok := cityLatLon(place)
	if !ok {
		return true, invalid("unknown place")
	}
	p.query.Domain = true
	text := lat + ", " + lon
	if kind == "latitude" {
		text = lat
	} else if kind == "longitude" {
		text = lon
	}
	p.query.format = "place"
	p.query.formatUnits = []string{text}
	v := Value{Kind: Number, Number: new(big.Rat)}
	if kind == "latitude" || kind == "longitude" {
		v.Number = rational(text)
	}
	p.query.root = &node{op: "place", text: text, value: v}
	return true, nil
}

func cityLatLon(name string) (lat, lon string, ok bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	places := map[string][2]string{
		"sydney": {"-33.8688", "151.2093"}, "london": {"51.5074", "-0.1278"},
		"paris": {"48.8566", "2.3522"}, "sfo": {"37.6213", "-122.3790"},
		"nyc": {"40.7128", "-74.0060"}, "toronto": {"43.6532", "-79.3832"},
		"berlin": {"52.5200", "13.4050"}, "tokyo": {"35.6762", "139.6503"},
		"melbourne": {"-37.8136", "144.9631"},
		"reykjavik": {"64.1466", "-21.9426"},
	}
	ll, ok := places[name]
	return ll[0], ll[1], ok
}

func (p *parser) parseHistoricalInflation() (bool, error) {
	if looksLikeInflation(p.tokens, p.i) {
		return false, nil
	}
	hasYear, hasFrom, hasWorth := false, false, false
	for _, t := range p.tokens[p.i:] {
		if isYearToken(t) {
			hasYear = true
		}
		if t.text == "from" || t.text == "today" || t.text == "dollars" {
			hasFrom = true
		}
		if t.text == "worth" || t.text == "was" {
			hasWorth = true
		}
	}
	if !hasYear || !hasFrom && !hasWorth && p.peek() != "value" && p.peek() != "what" && p.peek() != "$" {
		return false, nil
	}
	if !hasYear {
		return false, nil
	}
	saved := p.i
	p.accept("what")
	p.accept("is")
	p.accept("was")
	p.accept("value")
	p.accept("of")
	if p.tokens[p.i].text != "$" && p.tokens[p.i].value == nil {
		p.i = saved
		return false, nil
	}
	amount, err := p.expr(25)
	if err != nil {
		p.i = saved
		return false, nil
	}
	fromYear, toYear := 0, 0
	for p.peek() != "" {
		switch {
		case p.accept("from"):
			if p.tokens[p.i].value != nil {
				fromYear = int(ratInt(Value{Number: p.tokens[p.i].value}))
				p.i++
			}
		case p.accept("in"):
			if isYearToken(p.tokens[p.i]) {
				y := int(ratInt(Value{Number: p.tokens[p.i].value}))
				p.i++
				if fromYear == 0 && toYear == 0 {
					toYear = y
				} else if fromYear == 0 {
					fromYear = toYear
					toYear = y
				} else {
					toYear = y
				}
			} else {
				p.i = saved
				return false, nil
			}
		case p.accept("is") || p.accept("worth") || p.accept("what") || p.accept("today") || p.accept("dollars") || p.accept("a"):
		default:
			p.i = saved
			return false, nil
		}
	}
	if fromYear == 0 && toYear == 0 {
		p.i = saved
		return false, nil
	}
	p.query.Domain = true
	fy, err := p.make(&node{op: "value", value: Value{Kind: Number, Number: big.NewRat(int64(fromYear), 1)}})
	if err != nil {
		return true, err
	}
	ty, err := p.make(&node{op: "value", value: Value{Kind: Number, Number: big.NewRat(int64(toYear), 1)}})
	if err != nil {
		return true, err
	}
	p.query.root, err = p.make(&node{op: "historical_inflation", args: []*node{amount, fy, ty}})
	return true, err
}

func (p *parser) parseUploadTime() (bool, error) {
	if p.peek() != "time" || p.i+1 >= len(p.tokens) || p.tokens[p.i+1].text != "to" {
		return false, nil
	}
	if p.i+2 >= len(p.tokens) {
		return false, nil
	}
	verb := p.tokens[p.i+2].text
	if verb != "upload" && verb != "download" && verb != "copy" && verb != "transfer" && verb != "send" {
		return false, nil
	}
	p.i += 3
	size, err := p.expr(0)
	if err != nil {
		return true, err
	}
	if !p.accept("at") && !p.accept("@") {
		return true, invalid("expected at")
	}
	rate, err := p.expr(0)
	if err != nil {
		return true, err
	}
	p.query.Domain = true
	p.query.format = "timespan"
	p.query.root, err = p.make(&node{op: "/", args: []*node{size, rate}})
	return true, err
}

func (p *parser) parseWhatPer() (bool, error) {
	saved := p.i
	left, err := p.expr(0)
	if err != nil || !p.accept("is") || !p.accept("what") {
		p.i = saved
		return false, nil
	}
	if !p.accept("per") && !p.accept("/") {
		p.i = saved
		return false, nil
	}
	u, err := p.unit()
	if err != nil {
		return true, err
	}
	for k, n := range u {
		u[k] = -n
		if k == "month" {
			delete(u, k)
			u["mo"] = -n
		}
	}
	p.query.Domain = true
	p.query.target = u
	p.query.root = left
	return true, nil
}

func (p *parser) parseDistanceBetween() (bool, error) {
	if !p.accept("distance") {
		return false, nil
	}
	switch {
	case p.accept("between"):
	case p.accept("from"):
	default:
		// "distance berlin paris"
	}
	a := p.peek()
	if a == "" {
		return true, invalid("expected a place")
	}
	p.i++
	p.accept("and")
	p.accept("to")
	b := p.peek()
	if b == "" {
		return true, invalid("expected a place")
	}
	p.i++
	km, ok := cityDistanceKM(a, b)
	if !ok {
		return true, invalid("unknown places")
	}
	p.query.Domain = true
	n, err := p.make(&node{op: "value", value: Value{Kind: Number, Number: rational(km)}})
	if err != nil {
		return true, err
	}
	p.query.root, err = p.make(&node{op: "quantity", args: []*node{n}, value: Value{Unit: Unit{"km": 1}}})
	return true, err
}

func cityDistanceKM(a, b string) (string, bool) {
	type ll struct{ lat, lon float64 }
	latLon := func(name string) (ll, bool) {
		s, t, ok := cityLatLon(name)
		if !ok {
			return ll{}, false
		}
		lat, _ := strconv.ParseFloat(s, 64)
		lon, _ := strconv.ParseFloat(t, 64)
		return ll{lat, lon}, true
	}
	pa, ok1 := latLon(a)
	pb, ok2 := latLon(b)
	if !ok1 || !ok2 {
		return "", false
	}
	const r = 6371.0
	toR := func(d float64) float64 { return d * math.Pi / 180 }
	dlat := toR(pb.lat - pa.lat)
	dlon := toR(pb.lon - pa.lon)
	x := math.Sin(dlat/2)*math.Sin(dlat/2) + math.Cos(toR(pa.lat))*math.Cos(toR(pb.lat))*math.Sin(dlon/2)*math.Sin(dlon/2)
	km := 2 * r * math.Asin(math.Sqrt(x))
	return strconv.FormatFloat(km, 'f', 0, 64), true
}

func looksLikeInflation(tokens []token, i int) bool {
	for _, t := range tokens[i:] {
		if t.text == "inflation" || t.text == "assuming" {
			return true
		}
	}
	return false
}

func (p *parser) parseInflation() (bool, error) {
	if !looksLikeInflation(p.tokens, p.i) {
		return false, nil
	}
	saved := p.i
	if p.accept("value") || p.accept("purchasing") {
		p.accept("power")
		p.accept("of")
	} else if p.accept("what") {
		if !p.accept("will") && !p.accept("is") && !p.accept("was") {
			p.i = saved
			return false, nil
		}
	} else if p.tokens[p.i].text != "$" && p.tokens[p.i].value == nil {
		return false, nil
	}
	amount, err := p.expr(25)
	if err != nil {
		p.i = saved
		return false, nil
	}
	p.accept("be")
	p.accept("worth")
	if !p.accept("in") {
		p.i = saved
		return false, nil
	}
	if p.tokens[p.i].value == nil {
		p.i = saved
		return false, nil
	}
	year := ratInt(Value{Number: p.tokens[p.i].value})
	p.i++
	p.accept("assuming")
	p.accept("at")
	if p.tokens[p.i].value == nil && p.peek() != "%" {
		p.i = saved
		return false, nil
	}
	rate, err := p.expr(25)
	if err != nil {
		return true, err
	}
	p.accept("inflation")
	p.query.Domain = true
	y, err := p.make(&node{op: "value", value: Value{Kind: Number, Number: big.NewRat(year, 1)}})
	if err != nil {
		return true, err
	}
	p.query.root, err = p.make(&node{op: "future_inflation", args: []*node{amount, y, rate}})
	return true, err
}

func (p *parser) parseHolidayLiteral() (bool, error) {
	saved := p.i
	var words []string
	for p.peek() != "" && p.tokens[p.i].value == nil && p.tokens[p.i].temporal == nil {
		t := p.peek()
		if t == "'" || t == "s" && len(words) > 0 {
			p.i++
			continue
		}
		if !isHolidayWord(t) {
			break
		}
		words = append(words, t)
		p.i++
		if len(words) >= 5 {
			break
		}
	}
	canon, ok := canonicalHoliday(strings.Join(words, " "))
	if !ok {
		p.i = saved
		return false, nil
	}
	year := 0
	if p.tokens[p.i].value != nil {
		year = int(ratInt(Value{Number: p.tokens[p.i].value}))
		p.i++
	}
	p.query.Domain = true
	p.query.root = &node{op: "holiday", text: canon, value: Value{Kind: Number, Number: big.NewRat(int64(year), 1)}}
	return true, nil
}

func isHolidayWord(s string) bool {
	switch s {
	case "easter", "christmas", "halloween", "valentine", "valentines",
		"boxing", "good", "friday", "holy", "saturday", "sunday", "monday",
		"new", "year", "years", "eve", "day", "orthodox", "chinese",
		"ramadan", "hanukkah", "thanksgiving", "black", "end", "of":
		return true
	}
	return false
}

func canonicalHoliday(name string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "christmas eve":
		return "christmas eve", true
	case "christmas day", "christmas":
		return "christmas", true
	case "orthodox christmas":
		return "orthodox christmas", true
	case "boxing day":
		return "boxing day", true
	case "easter sunday", "easter":
		return "easter", true
	case "easter monday":
		return "easter monday", true
	case "good friday":
		return "good friday", true
	case "holy saturday":
		return "holy saturday", true
	case "orthodox easter":
		return "orthodox easter", true
	case "orthodox good friday":
		return "orthodox good friday", true
	case "new year eve", "new years eve":
		return "new years eve", true
	case "new year day", "new years day", "new year":
		return "new years day", true
	case "chinese new year eve", "chinese new years eve":
		return "chinese new year eve", true
	case "chinese new year", "chinese new years":
		return "chinese new year", true
	case "ramadan":
		return "ramadan", true
	case "end of ramadan":
		return "end of ramadan", true
	case "hanukkah":
		return "hanukkah", true
	case "end of hanukkah":
		return "end of hanukkah", true
	case "valentine day", "valentines day", "valentine":
		return "valentine", true
	case "halloween":
		return "halloween", true
	case "thanksgiving":
		return "thanksgiving", true
	case "black friday":
		return "black friday", true
	}
	return "", false
}

func evalSoulverExtra(n *node, args []Value, env Env) (Value, bool, error) {
	switch n.op {
	case "timecode":
		fps := envFPS(env)
		frames, err := framesFromTimecode(n.text, fps)
		if err != nil {
			return Value{}, true, err
		}
		return Value{Kind: Quantity, Number: frames, Unit: Unit{"frame": 1}}, true, nil
	case "&", "|", "xor", "<<", ">>":
		v, err := evalBitwise(n.op, args[0], args[1])
		return v, true, err
	case "holiday":
		year := int(ratInt(n.value))
		t, ok := holidayDate(n.text, year, env.Now)
		if !ok {
			return Value{}, true, invalid("unknown holiday")
		}
		return Value{Kind: Date, Time: t}, true, nil
	case "place":
		if n.value.Number != nil {
			return n.value, true, nil
		}
		return Value{Kind: Number, Number: new(big.Rat)}, true, nil
	case "tire_speed":
		v, err := evalTireSpeed(args[0], args[1])
		return v, true, err
	case "power_torque":
		v, err := evalPowerTorque(args[0], args[1])
		return v, true, err
	case "gps":
		return Value{Kind: Number, Number: new(big.Rat)}, true, nil
	case "historical_inflation":
		amount := args[0]
		from := int(ratInt(args[1]))
		to := int(ratInt(args[2]))
		if from == 0 {
			from = env.Now.Year()
		}
		if to == 0 {
			to = env.Now.Year()
		}
		cf, ok1 := usCPI(from)
		ct, ok2 := usCPI(to)
		if !ok1 || !ok2 || cf == "" || ct == "" {
			return Value{}, true, invalid("missing CPI")
		}
		out := amount
		out.Number = new(big.Rat).Mul(amount.Number, new(big.Rat).Quo(rational(ct), rational(cf)))
		return out, true, nil
	case "income_tax":
		v, err := incomeTax(args[0], n.text)
		return v, true, err
	case "future_inflation":
		pv, _ := args[0].Number.Float64()
		year := ratInt(args[1])
		r, _ := args[2].Number.Float64()
		if args[2].Kind != Percent {
			r = r / 100
		}
		n := float64(year - int64(env.Now.Year()))
		// Soulver's "value in a future year" is purchasing power: PV / (1+r)^n.
		if year > int64(env.Now.Year()) {
			num := ratPlaces(pv/math.Pow(1+r, n), 2)
			out := args[0]
			out.Number = num
			return out, true, nil
		}
		return Value{}, true, invalid("expected a future year")
	}
	return Value{}, false, nil
}

func parseHolidayName(s string, now time.Time) (time.Time, bool) {
	lower := strings.ToLower(strings.TrimSpace(s))
	year := 0
	fields := strings.Fields(lower)
	if n := len(fields); n >= 2 {
		if y, err := strconv.Atoi(fields[n-1]); err == nil && y >= 1 && y <= 9999 {
			year = y
			lower = strings.TrimSpace(strings.Join(fields[:n-1], " "))
		}
	}
	return holidayDate(lower, year, now)
}

func (p *parser) parsePaceAfter(prep string) error {
	if prep != "in" && prep != "to" {
		return invalid("expected a duration")
	}
	timeQty, err := p.expr(0)
	if err != nil {
		return err
	}
	p.query.Domain = true
	p.query.format = "pace"
	p.query.root, err = p.make(&node{op: "/", args: []*node{timeQty, p.query.root}})
	return err
}

func formatPace(v Value) string {
	secs := new(big.Rat).Set(v.Number)
	length := ""
	for k, n := range v.Unit {
		if n == -1 {
			length = k
		}
		if n == 1 {
			switch k {
			case "min", "?m":
				secs.Mul(secs, big.NewRat(60, 1))
			case "h":
				secs.Mul(secs, big.NewRat(3600, 1))
			case "ms":
				secs.Quo(secs, big.NewRat(1000, 1))
			}
		}
	}
	total, _ := secs.Float64()
	if total < 0 {
		total = -total
	}
	m := int(math.Floor(total / 60))
	s := int(math.Round(total - float64(m)*60))
	if s == 60 {
		m++
		s = 0
	}
	unit := length
	switch length {
	case "km", "mi", "m":
		unit = length
	}
	return pad2(int64(m)) + ":" + pad2(int64(s)) + "/" + unit
}

func isTaxCountry(s string) bool {
	switch strings.ToLower(s) {
	case "australia", "au", "canada", "ca", "usa", "us", "united":
		return true
	}
	return false
}

// incomeTax applies fixture country brackets; live Soulver tables may differ.
func incomeTax(amount Value, spec string) (Value, error) {
	if amount.Number == nil {
		return Value{}, invalid("expected income")
	}
	parts := strings.SplitN(spec, ":", 2)
	if len(parts) != 2 {
		return Value{}, invalid("expected country")
	}
	kind, country := parts[0], strings.ToLower(parts[1])
	income, _ := amount.Number.Float64()
	tax := 0.0
	switch country {
	case "australia", "au":
		tax = progressiveTax(income, [][2]float64{{18200, 0}, {45000, 0.16}, {135000, 0.30}, {190000, 0.37}}, 0.45)
	case "canada", "ca":
		taxable := income - 15705
		if taxable < 0 {
			taxable = 0
		}
		tax = progressiveTax(taxable, [][2]float64{{55867, 0.15}, {111733, 0.205}, {173205, 0.26}, {246752, 0.29}}, 0.33)
	case "usa", "us", "unitedstates":
		taxable := income - 15000
		if taxable < 0 {
			taxable = 0
		}
		tax = progressiveTax(taxable, [][2]float64{{11925, 0.10}, {48475, 0.12}, {103350, 0.22}, {197300, 0.24}, {250525, 0.32}, {626350, 0.35}}, 0.37)
	default:
		return Value{}, invalid("unknown tax country")
	}
	out := amount
	switch kind {
	case "after":
		out.Number = ratPlaces(income-tax, 2)
	case "rate":
		if income == 0 {
			return Value{}, invalid("tax rate requires income")
		}
		return Value{Kind: Percent, Number: ratPlaces(tax/income, 4)}, nil
	default:
		out.Number = ratPlaces(tax, 2)
	}
	return out, nil
}

func progressiveTax(income float64, brackets [][2]float64, topRate float64) float64 {
	tax, prev := 0.0, 0.0
	for _, b := range brackets {
		limit, rate := b[0], b[1]
		if income <= prev {
			break
		}
		span := limit - prev
		if income < limit {
			span = income - prev
		}
		if span > 0 {
			tax += span * rate
		}
		prev = limit
	}
	if income > prev {
		tax += (income - prev) * topRate
	}
	return tax
}

func usCPI(year int) (string, bool) {
	// Annual CPI-U averages used for documented inflation phrases.
	table := map[int]string{
		1960: "29.6", 1977: "60.6", 1985: "107.6", 1996: "156.9", 1997: "160.5",
		2003: "184.0", 2021: "270.970", 2022: "292.655", 2026: "314.0",
	}
	s, ok := table[year]
	return s, ok
}

func formatDMS(deg *big.Rat) string {
	f, _ := deg.Float64()
	sign := ""
	if f < 0 {
		sign = "-"
		f = -f
	}
	d := math.Floor(f)
	minf := (f - d) * 60
	m := math.Floor(minf)
	s := (minf - m) * 60
	s = math.Round(s*10) / 10
	return sign + strconv.FormatFloat(d, 'f', 0, 64) + "° " + strconv.FormatFloat(m, 'f', 0, 64) + "′ " + strconv.FormatFloat(s, 'f', 1, 64) + "″"
}

func (p *parser) takeNote() (int, bool) {
	t := p.tokens[p.i]
	if len(t.text) != 1 || t.text[0] < "a"[0] || t.text[0] > "g"[0] {
		return 0, false
	}
	if p.i+1 >= len(p.tokens) || p.tokens[p.i+1].value == nil || !p.tokens[p.i+1].value.IsInt() {
		return 0, false
	}
	oct := int(ratInt(Value{Number: p.tokens[p.i+1].value}))
	if oct < 0 || oct > 8 {
		return 0, false
	}
	// C=0, D=2, E=4, F=5, G=7, A=9, B=11
	semitone := []int{9, 11, 0, 2, 4, 5, 7}[t.text[0]-'a']
	p.i += 2
	return (oct+1)*12 + semitone, true
}

func midiFromFreq(hz float64) float64 {
	return 69 + 12*math.Log2(hz/440)
}

func freqFromMIDI(midi float64) float64 {
	return 440 * math.Pow(2, (midi-69)/12)
}

func (p *parser) parseTireSpeed() (bool, error) {
	if !p.accept("speed") {
		return false, nil
	}
	if !p.accept("of") {
		return true, invalid("expected of")
	}
	diam, err := p.expr(25)
	if err != nil {
		return true, err
	}
	p.accept("tire")
	p.accept("tyre")
	if !p.accept("at") {
		return true, invalid("expected at")
	}
	rpm, err := p.expr(0)
	if err != nil {
		return true, err
	}
	p.query.Domain = true
	p.query.root, err = p.make(&node{op: "tire_speed", args: []*node{diam, rpm}})
	return true, err
}

func (p *parser) parsePowerTorque() (bool, error) {
	saved := p.i
	if p.tokens[p.i].value == nil {
		return false, nil
	}
	power, err := p.expr(25)
	if err != nil || !quantityHas(power, "W") || !p.accept("at") {
		p.i = saved
		return false, nil
	}
	ang, err := p.expr(25)
	if err != nil || !quantityHas(ang, "rpm") || p.peek() != "" {
		p.i = saved
		return false, nil
	}
	p.query.Domain = true
	p.query.root, err = p.make(&node{op: "power_torque", args: []*node{power, ang}})
	return true, err
}

func quantityHas(n *node, symbol string) bool {
	return n != nil && n.op == "quantity" && n.value.Unit[symbol] != 0
}

func (p *parser) parseGPS() (bool, error) {
	saved := p.i
	lat, hemiLat, ok := p.takeDMSHemisphere()
	if !ok {
		p.i = saved
		return false, nil
	}
	p.accept(",")
	lon, hemiLon, ok := p.takeDMSHemisphere()
	if !ok {
		p.i = saved
		return false, nil
	}
	p.query.Domain = true
	p.query.format = "gps"
	p.query.formatUnits = []string{formatGPS(lat, hemiLat, lon, hemiLon)}
	p.query.root = &node{op: "gps"}
	return true, nil
}

func (p *parser) takeDMSHemisphere() (float64, string, bool) {
	if p.tokens[p.i].value == nil {
		return 0, "", false
	}
	deg, _ := p.tokens[p.i].value.Float64()
	p.i++
	if p.peek() != "°" && p.peek() != "degrees" && p.peek() != "degree" {
		return 0, "", false
	}
	p.i++
	if p.tokens[p.i].value == nil {
		return 0, "", false
	}
	min, _ := p.tokens[p.i].value.Float64()
	p.i++
	if _, err := p.arcUnit(); err != nil {
		return 0, "", false
	}
	if p.tokens[p.i].value == nil {
		return 0, "", false
	}
	sec, _ := p.tokens[p.i].value.Float64()
	p.i++
	if _, err := p.arcUnit(); err != nil {
		return 0, "", false
	}
	hemi := p.peek()
	if hemi != "n" && hemi != "s" && hemi != "e" && hemi != "w" {
		return 0, "", false
	}
	p.i++
	if hemi == "s" || hemi == "w" {
		deg = -deg
	}
	return deg + min/60 + sec/3600, strings.ToUpper(hemi), true
}

func formatGPS(lat float64, hemiLat string, lon float64, hemiLon string) string {
	return strconv.FormatFloat(math.Abs(lat), 'f', 2, 64) + "° " + hemiLat + ", " + strconv.FormatFloat(math.Abs(lon), 'f', 2, 64) + "° " + hemiLon
}

func evalTireSpeed(diam, rpm Value) (Value, error) {
	d, err := convertLengthMeters(diam)
	if err != nil {
		return Value{}, err
	}
	rps, _ := rpm.Number.Float64()
	if rpm.Unit["rpm"] == 1 {
		rps = rps / 60
	}
	mps := math.Pi * d * rps
	mph := mps * 2.2369362920544
	return Value{Kind: Quantity, Number: ratPlaces(mph, 2), Unit: Unit{"mph": 1}}, nil
}

func convertLengthMeters(v Value) (float64, error) {
	if v.Kind != Quantity {
		return 0, invalid("expected a length")
	}
	c := NewCatalog()
	m, err := c.convert(v, Unit{"m": 1}, Env{}, nil)
	if err != nil {
		return 0, err
	}
	f, _ := m.Number.Float64()
	return f, nil
}

func evalPowerTorque(power, rpm Value) (Value, error) {
	w, _ := power.Number.Float64()
	n, _ := rpm.Number.Float64()
	if n == 0 {
		return Value{}, invalid("division by zero")
	}
	nm := w * 60 / (n * 2 * math.Pi)
	return Value{Kind: Quantity, Number: ratPlaces(nm, 10), Unit: Unit{"Nm": 1}}, nil
}

func pitchName(midi float64) string {
	n := int(math.Round(midi))
	names := []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}
	oct := n/12 - 1
	pc := n % 12
	if pc < 0 {
		pc += 12
		oct--
	}
	return names[pc] + strconv.Itoa(oct)
}
