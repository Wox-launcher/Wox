package engine

import (
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

// temporalQuery is syntax, not an evaluated time; the reference date comes from Env.
type temporalQuery struct {
	kind, clock, date, source, target, unit string
	amount                                  int
	weekday                                 time.Weekday
	year                                    int
}

const clockSyntax = `([0-9]{1,2}(?::[0-9]{2})?\s*(?:am|pm)|[0-9]{1,2}:[0-9]{2})`

var clockPrefixRE = regexp.MustCompile(`(?i)^(?:(\d{4}-\d{2}-\d{2})\s+)?` + clockSyntax + `\s+`)
var zoneConversionRE = regexp.MustCompile(`(?i)\s+(?:to|in)\s+`)
var delayRE = regexp.MustCompile(`(?i)^(\d+)\s+(milliseconds?|seconds?|minutes?|mins?|hours?|days?|weeks?)(?:\s+in\s+(.+))?$`)
var weekdayRE = regexp.MustCompile(`(?i)^(monday|tuesday|wednesday|thursday|friday|saturday|sunday)\s+in\s+(\d+)\s+(days?|weeks?)$`)
var agoRE = regexp.MustCompile(`(?i)^(\d+)\s+(days?|weeks?)\s+ago$`)
var explicitYearRE = regexp.MustCompile(`\d{4}`)
var dateRE = regexp.MustCompile(`(?i)^(\d{1,2})(?:st|nd|rd|th)?\s+([a-z]+)(?:\s+(\d{4}))?$`)
var monthFirstRE = regexp.MustCompile(`(?i)^([a-z]+)\s+(\d{1,2})(?:\s+(\d{4}))?$`)
var dateArithmeticRE = regexp.MustCompile(`(?i)^(.+?)\s+([+-])\s+(\d+)(?:\s+(days?|weeks?|months?|years?|hours?|minutes?))?$`)
var betweenRE = regexp.MustCompile(`(?i)^(?:time\s+)?difference\s+between\s+(.+?)\s+(?:and|&)\s+(.+)$`)
var utcOffsetRE = regexp.MustCompile(`(?i)^(gmt|utc)\s*([+-])\s*(\d{1,2})(?::(\d{2}))?$`)

// parseTemporal owns all time-prefixed forms in one entry, avoiding route-order fallback.
func (c *Catalog) parseTemporal(input string) (*temporalQuery, bool, error) {
	s := strings.TrimSpace(input)
	lower := strings.ToLower(s)
	q := &temporalQuery{}
	if m := betweenRE.FindStringSubmatch(s); m != nil {
		q.kind = "between"
		q.source = strings.TrimSpace(m[1])
		q.target = strings.TrimSpace(m[2])
		if _, err := c.location(q.source); err != nil {
			return q, true, err
		}
		_, err := c.location(q.target)
		return q, true, err
	}
	if strings.HasPrefix(lower, "time in ") || strings.HasPrefix(lower, "now in ") {
		rest := strings.TrimSpace(s[strings.Index(lower, " in ")+len(" in "):])
		q.kind = "now"
		q.target = rest
		if m := delayRE.FindStringSubmatch(rest); m != nil {
			q.kind = "delay"
			q.amount, _ = strconv.Atoi(m[1])
			q.unit = m[2]
			q.target = m[3]
		}
		if q.target != "" {
			if _, err := c.location(q.target); err != nil {
				return nil, true, err
			}
		}
		return q, true, nil
	}
	for _, prefix := range []string{"time diff ", "diff "} {
		if strings.HasPrefix(lower, prefix) {
			q.kind = "diff"
			q.target = strings.TrimSpace(s[len(prefix):])
			_, err := c.location(q.target)
			return q, true, err
		}
	}
	if strings.HasPrefix(lower, "workhours in ") {
		q.kind = "workhours"
		year, err := strconv.Atoi(strings.TrimSpace(s[len("workhours in "):]))
		if err != nil || year < 1 || year > 9999 {
			return nil, true, invalid("invalid work year")
		}
		q.year = year
		return q, true, nil
	}
	if strings.HasPrefix(lower, "days until ") || strings.HasPrefix(lower, "day until ") {
		q.kind = "until"
		q.date = strings.TrimSpace(s[strings.Index(lower, "until")+5:])
		return q, true, nil
	}
	if m := weekdayRE.FindStringSubmatch(s); m != nil {
		q.kind = "weekday"
		q.weekday = map[string]time.Weekday{"sunday": 0, "monday": 1, "tuesday": 2, "wednesday": 3, "thursday": 4, "friday": 5, "saturday": 6}[strings.ToLower(m[1])]
		q.amount, _ = strconv.Atoi(m[2])
		q.unit = m[3]
		return q, true, nil
	}
	if m := agoRE.FindStringSubmatch(s); m != nil {
		q.kind = "ago"
		q.amount, _ = strconv.Atoi(m[1])
		q.unit = m[2]
		return q, true, nil
	}
	if match := clockPrefixRE.FindStringSubmatch(s); match != nil && !dateArithmeticRE.MatchString(s) && !strings.HasPrefix(strings.TrimSpace(s[len(match[0]):]), "-") && !strings.HasPrefix(strings.TrimSpace(s[len(match[0]):]), "+") {
		q.date = match[1]
		q.clock = match[2]
		rest := strings.TrimSpace(s[len(match[0]):])
		hasIn := strings.HasPrefix(strings.ToLower(rest), "in ")
		if hasIn {
			rest = strings.TrimSpace(rest[3:])
		}
		parts := zoneConversionRE.Split(rest, 2)
		if len(parts) == 2 {
			q.kind = "convert"
			q.source = parts[0]
			q.target = parts[1]
		} else if hasIn {
			q.kind = "local"
			q.source = rest
		} else {
			return nil, true, invalid("expected source and target timezone")
		}

		if _, err := parseClock(q.clock); err != nil {
			return nil, true, err
		}
		if _, err := c.location(q.source); err != nil {
			return nil, true, err
		}
		if q.target != "" {
			if _, err := c.location(q.target); err != nil {
				return nil, true, err
			}
		}
		return q, true, nil
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		q.kind = "iso"
		q.date = t.Format(time.RFC3339Nano)
		return q, true, nil
	}
	if m := dateArithmeticRE.FindStringSubmatch(s); m != nil {
		if _, err := parseClock(m[1]); err == nil {
			q.kind = "clockAdd"
			q.clock = m[1]
		} else if isDateText(m[1]) {
			q.kind = "dateAdd"
			q.date = m[1]
		} else {
			return nil, false, nil
		}
		q.amount, _ = strconv.Atoi(m[3])
		if m[2] == "-" {
			q.amount = -q.amount
		}
		q.unit = m[4]
		return q, true, nil
	}
	if strings.HasSuffix(lower, " time") {
		place := strings.TrimSpace(s[:len(s)-len(" time")])
		if place != "" {
			if _, err := c.location(place); err == nil {
				q.kind = "now"
				q.target = place
				return q, true, nil
			}
		}
	}
	if strings.HasPrefix(lower, "time saved ") {
		return nil, false, nil
	}
	if strings.HasPrefix(lower, "time ") || strings.HasPrefix(lower, "workhours ") {
		return nil, true, invalid("invalid time query")
	}
	return nil, false, nil
}
func isDateText(s string) bool {
	if len(s) == 10 && s[4] == '-' && s[7] == '-' {
		return true
	}
	// dateRE / monthFirstRE also match "20 km"; require a real month name.
	_, err := parseDate(s, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	return err == nil
}

// location resolves only registered aliases or canonical IANA names.
func (c *Catalog) location(s string) (*time.Location, error) {
	s = strings.TrimSpace(s)
	if loc, ok := parseUTCOffset(s); ok {
		return loc, nil
	}
	if alias, ok := c.Zones[strings.ToLower(s)]; ok {
		s = alias
	}
	l, err := time.LoadLocation(s)
	if err != nil {
		return nil, invalid("unknown timezone " + s)
	}
	return l, nil
}

// parseUTCOffset accepts GMT+8 / UTC-07:00 as a fixed offset, not an IANA name.
func parseUTCOffset(s string) (*time.Location, bool) {
	m := utcOffsetRE.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return nil, false
	}
	hours, _ := strconv.Atoi(m[3])
	mins := 0
	if m[4] != "" {
		mins, _ = strconv.Atoi(m[4])
	}
	if hours > 14 || mins > 59 {
		return nil, false
	}
	offset := hours*3600 + mins*60
	if m[2] == "-" {
		offset = -offset
	}
	name := strings.ToUpper(m[1]) + m[2] + m[3]
	if m[4] != "" {
		name += ":" + m[4]
	}
	return time.FixedZone(name, offset), true
}

// parseClock anchors a wall clock to a neutral date without reading local time.
func parseClock(s string) (time.Time, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	for _, layout := range []string{"3:04 pm", "3:04pm", "3pm", "3 pm", "15:04"} {
		if t, err := time.Parse(layout, s); err == nil {
			return time.Date(2000, 1, 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC), nil
		}
	}
	return time.Time{}, invalid("invalid clock")
}

// localInstant rejects both missing and repeated wall times. Candidate offsets are
// sampled around the date rather than assuming DST always changes by one hour.
func localInstant(date, timeOfDay time.Time, loc *time.Location) (time.Time, error) {
	y, m, d := date.Date()
	h, min, sec := timeOfDay.Clock()
	wall := time.Date(y, m, d, h, min, sec, timeOfDay.Nanosecond(), time.UTC)
	offsets := map[int]bool{}
	for step := -48; step <= 48; step += 6 {
		_, offset := wall.Add(time.Duration(step) * time.Hour).In(loc).Zone()
		offsets[offset] = true
	}
	var matches []time.Time
	for offset := range offsets {
		candidate := wall.Add(-time.Duration(offset) * time.Second).In(loc)
		cy, cm, cd := candidate.Date()
		ch, cmin, cs := candidate.Clock()
		if cy == y && cm == m && cd == d && ch == h && cmin == min && cs == sec {
			matches = append(matches, candidate)
		}
	}
	if len(matches) != 1 {
		return time.Time{}, invalid("local time is missing or ambiguous at a timezone transition")
	}
	return matches[0], nil
}

// parseDate validates authored calendar fields before Go can normalize overflow.
func parseDate(s string, now time.Time) (time.Time, error) {
	if t, err := time.ParseInLocation("2006-01-02", s, now.Location()); err == nil {
		return t, nil
	}
	m := dateRE.FindStringSubmatch(s)
	if m == nil {
		if first := monthFirstRE.FindStringSubmatch(s); first != nil {
			m = []string{first[0], first[2], first[1], first[3]}
		}
	}
	if m == nil {
		return time.Time{}, invalid("invalid date")
	}
	months := map[string]time.Month{"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6, "jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12}
	name := strings.ToLower(m[2])
	if len(name) < 3 {
		return time.Time{}, invalid("invalid month")
	}
	month, ok := months[name[:3]]
	if !ok {
		return time.Time{}, invalid("invalid month")
	}
	if name != strings.ToLower(month.String()) && name != strings.ToLower(month.String()[:3]) {
		return time.Time{}, invalid("invalid month")
	}
	day, _ := strconv.Atoi(m[1])
	year := now.Year()
	if m[3] != "" {
		year, _ = strconv.Atoi(m[3])
	}
	t := time.Date(year, month, day, 0, 0, 0, 0, now.Location())
	if year < 1 || year > 9999 || t.Month() != month || t.Day() != day {
		return time.Time{}, invalid("invalid calendar date")
	}
	return t, nil
}

// durationSeconds bounds integer offsets before conversion into elapsed time.
func durationSeconds(amount int, unit string) (*big.Rat, error) {
	scale := int64(0)
	switch {
	case strings.HasPrefix(unit, "millisecond"):
		return big.NewRat(int64(amount), 1000), nil
	case strings.HasPrefix(unit, "second"):
		scale = 1
	case strings.HasPrefix(unit, "min"):
		scale = 60
	case strings.HasPrefix(unit, "hour"):
		scale = 3600
	case strings.HasPrefix(unit, "day"):
		scale = 86400
	case strings.HasPrefix(unit, "week"):
		scale = 604800
	default:
		return nil, invalid("unknown duration")
	}
	if amount < -1000000 || amount > 1000000 {
		return nil, invalid("date amount exceeds limit")
	}
	return big.NewRat(int64(amount)*scale, 1), nil
}

// evaluateTemporal supplies the captured source date and timezone to domain queries.
func (c *Catalog) evaluateTemporal(query *Query, env Env) (Evaluation, error) {
	q := query.temporal
	now := env.Now.In(env.Local)
	r := Evaluation{Expression: query.Expression}
	v := Value{Kind: Instant}
	var err error
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, env.Local)
	if q.amount < -1000000 || q.amount > 1000000 {
		return r, invalid("date amount exceeds limit")
	}
	switch q.kind {
	case "dateLiteral":
		t, e := parseDate(q.date, now)
		if e != nil {
			return r, e
		}
		v = Value{Kind: Date, Time: t}
	case "localLiteral":
		date, e := parseDate(q.date, now)
		if e != nil {
			return r, e
		}
		clock, e := parseClock(q.clock)
		if e != nil {
			return r, e
		}
		v.Time, err = localInstant(date, clock, env.Local)
	case "clockLiteral":
		t, e := parseClock(q.clock)
		if e != nil {
			return r, e
		}
		v = Value{Kind: Clock, Time: t}
	case "now", "delay", "diff", "between":
		if q.kind == "between" {
			src, e := c.location(q.source)
			if e != nil {
				return r, e
			}
			dst, e := c.location(q.target)
			if e != nil {
				return r, e
			}
			_, a := env.Now.In(src).Zone()
			_, b := env.Now.In(dst).Zone()
			hours := a - b
			if hours < 0 {
				hours = -hours
			}
			v = Value{Kind: Quantity, Number: big.NewRat(int64(hours), 3600), Unit: Unit{"h": 1}}
			break
		}
		loc := env.Local
		if q.target != "" {
			loc, err = c.location(q.target)
			if err != nil {
				return r, err
			}
		}
		t := env.Now.In(loc)
		if q.kind == "delay" {
			seconds, e := durationSeconds(q.amount, q.unit)
			if e != nil {
				return r, e
			}
			ns := new(big.Rat).Mul(seconds, big.NewRat(1e9, 1))
			if !ns.IsInt() || !ns.Num().IsInt64() {
				return r, invalid("duration exceeds limit")
			}
			t = t.Add(time.Duration(ns.Num().Int64()))
		}
		if q.kind == "diff" {
			_, a := t.Zone()
			_, b := now.Zone()
			v = Value{Kind: Quantity, Number: big.NewRat(int64(a-b), 3600), Unit: Unit{"h": 1}}
		} else {
			v.Time = t
			r.Target = "current"
		}
	case "convert", "local":
		loc, e := c.location(q.source)
		if e != nil {
			return r, e
		}
		date := env.Now.In(loc)
		if q.date != "" {
			date, e = parseDate(q.date, date)
			if e != nil {
				return r, e
			}
		}
		clock, e := parseClock(q.clock)
		if e != nil {
			return r, e
		}
		source, e := localInstant(date, clock, loc)
		if e != nil {
			return r, e
		}
		r.Source = &source
		target := env.Local
		if q.target != "" {
			target, e = c.location(q.target)
			if e != nil {
				return r, e
			}
		}
		v.Time = source.In(target)
		r.Target = "timestamp"
	case "iso":
		v.Time, err = time.Parse(time.RFC3339Nano, q.date)
		v.Time = v.Time.In(env.Local)
		r.Target = "timestamp"
	case "until":
		target, e := parseDate(q.date, now)
		if e != nil {
			return r, e
		}
		if !explicitYearRE.MatchString(q.date) && target.Before(today) {
			target, e = parseDate(q.date, now.AddDate(1, 0, 0))
			if e != nil {
				return r, e
			}
		}
		a := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
		b := time.Date(target.Year(), target.Month(), target.Day(), 0, 0, 0, 0, time.UTC)
		v = Value{Kind: Quantity, Number: big.NewRat((b.Unix()-a.Unix())/86400, 1), Unit: Unit{"d": 1}}
	case "weekday", "ago":
		days := q.amount
		if strings.HasPrefix(strings.ToLower(q.unit), "week") {
			days *= 7
		}
		if q.kind == "ago" {
			days = -days
		}
		t := today.AddDate(0, 0, days)
		if q.kind == "weekday" {
			t = t.AddDate(0, 0, (int(q.weekday)-int(t.Weekday())+7)%7)
		}
		v = Value{Kind: Date, Time: t}
	case "workhours":
		count := int64(0)
		for t := time.Date(q.year, 1, 1, 0, 0, 0, 0, time.UTC); t.Year() == q.year; t = t.AddDate(0, 0, 1) {
			if t.Weekday() != time.Saturday && t.Weekday() != time.Sunday {
				count += 8
			}
		}
		v = Value{Kind: Quantity, Number: big.NewRat(count, 1), Unit: Unit{"h": 1}}
	case "clockAdd":
		t, e := parseClock(q.clock)
		if e != nil {
			return r, e
		}
		unit := strings.ToLower(q.unit)
		if unit == "" {
			unit = "hours"
		}
		seconds, e := durationSeconds(q.amount, unit)
		if e != nil {
			return r, e
		}
		if !seconds.IsInt() || !seconds.Num().IsInt64() {
			return r, invalid("invalid clock offset")
		}
		total := int64(t.Hour()*3600+t.Minute()*60) + seconds.Num().Int64()
		day := total / 86400
		if total < 0 && total%86400 != 0 {
			day--
		}
		second := total - day*86400
		v = Value{Kind: Clock, Time: time.Date(2000, 1, 1, int(second/3600), int(second%3600/60), int(second%60), 0, time.UTC), Days: int(day)}
	case "dateAdd":
		t, e := parseDate(q.date, now)
		if e != nil {
			return r, e
		}
		days, months, years := 0, 0, 0
		switch {
		case q.unit == "" || strings.HasPrefix(q.unit, "day"):
			days = q.amount
		case strings.HasPrefix(q.unit, "week"):
			days = q.amount * 7
		case strings.HasPrefix(q.unit, "month"):
			months = q.amount
		case strings.HasPrefix(q.unit, "year"):
			years = q.amount
		default:
			return r, invalid("date needs calendar units")
		}
		v = Value{Kind: Date, Time: t.AddDate(years, months, days)}
	default:
		return r, invalid("unknown time operation")
	}
	if err != nil {
		return r, err
	}
	if !v.Time.IsZero() && (v.Time.Year() < 1 || v.Time.Year() > 9999) {
		return r, invalid("date exceeds supported range")
	}
	r.Value = v
	return r, nil
}

// offsetText includes fractional-hour timezone offsets without losing their sign.
func offsetText(t time.Time) string {
	_, offset := t.Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	return fmt.Sprintf("UTC%s%02d:%02d", sign, offset/3600, offset%3600/60)
}
