package engine

import (
	"fmt"
	"math"
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

const clockSyntax = `([0-9]{1,2}(?::[0-9]{2}(?::[0-9]{2})?)?\s*(?:a\.?m\.?|p\.?m\.?)|[0-9]{1,2}:[0-9]{2})`

var clockPrefixRE = regexp.MustCompile(`(?i)^(?:(\d{4}-\d{2}-\d{2})\s+)?` + clockSyntax + `\s+`)
var zoneConversionRE = regexp.MustCompile(`(?i)\s+(?:to|in)\s+`)
var delayRE = regexp.MustCompile(`(?i)^(\d+)\s+(milliseconds?|seconds?|minutes?|mins?|hours?|days?|weeks?)(?:\s+in\s+(.+))?$`)
var placeDelayRE = regexp.MustCompile(`(?i)^(.+?)\s+(\d+)\s+(milliseconds?|seconds?|minutes?|mins?|hours?|days?|weeks?)(?:\s+(ago|from\s+now))?$`)
var weekdayRE = regexp.MustCompile(`(?i)^(monday|tuesday|wednesday|thursday|friday|saturday|sunday)\s+in\s+(\d+)\s+(days?|weeks?)$`)
var agoRE = regexp.MustCompile(`(?i)^(\d+)\s+(days?|weeks?)\s+ago$`)
var explicitYearRE = regexp.MustCompile(`\d{4}`)
var dateRE = regexp.MustCompile(`(?i)^(\d{1,2})(?:st|nd|rd|th)?\s+([a-z]+)(?:,?\s+(\d{2,4}))?$`)
var monthFirstRE = regexp.MustCompile(`(?i)^([a-z]+)\s+(\d{1,2})(?:st|nd|rd|th)?(?:,?\s+(\d{2,4}))?$`)
var numericDateRE = regexp.MustCompile(`^(\d{1,2})[/.](\d{1,2})[/.](\d{4})$`)
var dateArithmeticRE = regexp.MustCompile(`(?i)^(.+?)\s+([+-])\s+(\d+)(?:\s+(days?|weeks?|months?|years?|hours?|minutes?))?$`)
var betweenRE = regexp.MustCompile(`(?i)^(?:time\s+)?difference\s+between\s+(.+?)\s+(?:and|&)\s+(.+)$`)
var utcOffsetRE = regexp.MustCompile(`(?i)^(gmt|utc)\s*([+-])\s*(\d{1,2})(?::(\d{2}))?$`)
var agoInRE = regexp.MustCompile(`(?i)^(\d+)\s+(milliseconds?|seconds?|minutes?|mins?|hours?|days?|weeks?)\s+ago(?:\s+in\s+(.+))?$`)
var fromNowRE = regexp.MustCompile(`(?i)^(\d+)\s+(milliseconds?|seconds?|minutes?|mins?|hours?|days?|weeks?)\s+from\s+now(?:\s+in\s+(.+))?$`)
var compactClockRE = regexp.MustCompile(`(?i)^(\d{3,4})\s+(.+)$`)
var relativeWeekdayRE = regexp.MustCompile(`(?i)^(monday|tuesday|wednesday|thursday|friday|saturday|sunday)\s+(next|last|this)\s+week(?:\s+(.+))?$`)
var sunEventRE = regexp.MustCompile(`(?i)^(sunrise|sunset)\s+in\s+(.+?)(?:\s+on\s+(.+))?$`)
var hoursUntilRE = regexp.MustCompile(`(?i)^hours?\s+(?:until|till)\s+(.+)$`)
var weekdayClockRangeRE = regexp.MustCompile(`(?i)^(monday|tuesday|wednesday|thursday|friday|saturday|sunday)\s+` + clockSyntax + `\s*[-–−]\s*` + clockSyntax + `$`)
var monthDayRangeRE = regexp.MustCompile(`(?i)^(` + monthNamePat + `)\s+(\d{1,2})\s*[-–−]\s*(\d{1,2})$`)

// parseTemporal owns all time-prefixed forms in one entry, avoiding route-order fallback.
func (c *Catalog) parseTemporal(input string) (*temporalQuery, bool, error) {
	s := strings.TrimSpace(input)
	lower := strings.ToLower(s)
	q := &temporalQuery{}
	if lower == "lunar day" || lower == "moon day" {
		q.kind = "lunar"
		return q, true, nil
	}
	if m := monthDayRangeRE.FindStringSubmatch(s); m != nil {
		q.kind = "dateRange"
		q.date = m[1] + " " + m[2]
		q.source = m[1] + " " + m[3]
		return q, true, nil
	}
	if m := weekdayClockRangeRE.FindStringSubmatch(s); m != nil {
		q.kind = "clockRange"
		q.clock = m[2]
		q.date = m[3]
		return q, true, nil
	}
	if lower == "next quarter" || lower == "this quarter" || lower == "last quarter" {
		q.kind = "dateLiteral"
		q.date = s
		return q, true, nil
	}
	if m := sunEventRE.FindStringSubmatch(s); m != nil {
		q.kind = strings.ToLower(m[1])
		q.target = strings.TrimSpace(m[2])
		q.date = strings.TrimSpace(m[3])
		if _, _, ok := cityLatLon(q.target); !ok {
			return nil, true, invalid("unknown place")
		}
		if _, err := c.location(q.target); err != nil {
			return nil, true, err
		}
		return q, true, nil
	}
	if m := agoInRE.FindStringSubmatch(s); m != nil {
		place := strings.TrimSpace(m[3])
		if isClockScaleUnit(m[2]) || place != "" {
			q.kind = "delay"
			q.amount, _ = strconv.Atoi(m[1])
			q.amount = -q.amount
			q.unit = m[2]
			q.target = place
			if q.target != "" {
				if _, err := c.location(q.target); err != nil {
					return nil, true, err
				}
			}
			return q, true, nil
		}
	}
	if m := fromNowRE.FindStringSubmatch(s); m != nil {
		place := strings.TrimSpace(m[3])
		if isClockScaleUnit(m[2]) || place != "" {
			q.kind = "delay"
			q.amount, _ = strconv.Atoi(m[1])
			q.unit = m[2]
			q.target = place
			if q.target != "" {
				if _, err := c.location(q.target); err != nil {
					return nil, true, err
				}
			}
			return q, true, nil
		}
	}
	if rest, ok := strings.CutPrefix(lower, "years to "); ok {
		year, err := strconv.Atoi(strings.TrimSpace(rest))
		if err != nil || year < 1 || year > 9999 {
			return nil, true, invalid("expected a year")
		}
		q.kind = "yearsTo"
		q.year = year
		return q, true, nil
	}
	if rest, ok := strings.CutPrefix(lower, "daylight hours in "); ok {
		q.kind = "daylight"
		q.target = strings.TrimSpace(rest)
		if _, _, ok := cityLatLon(q.target); !ok {
			return nil, true, invalid("unknown place")
		}
		return q, true, nil
	}
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
	if strings.HasPrefix(lower, "date in ") {
		q.kind = "dateIn"
		q.target = strings.TrimSpace(s[len("date in "):])
		if _, err := c.location(q.target); err != nil {
			return nil, true, err
		}
		return q, true, nil
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
		} else if m := placeDelayRE.FindStringSubmatch(rest); m != nil {
			q.kind = "delay"
			q.target = strings.TrimSpace(m[1])
			q.amount, _ = strconv.Atoi(m[2])
			q.unit = m[3]
			if strings.EqualFold(m[4], "ago") {
				q.amount = -q.amount
			}
		}
		if q.target != "" {
			if _, err := c.location(q.target); err != nil {
				return nil, true, err
			}
		}
		return q, true, nil
	}
	for _, prefix := range []string{"time difference ", "time diff ", "diff "} {
		if strings.HasPrefix(lower, prefix) && !strings.Contains(lower, " between ") {
			q.kind = "diff"
			q.target = strings.TrimSpace(s[len(prefix):])
			_, err := c.location(q.target)
			return q, true, err
		}
	}
	if rest, ok := strings.CutPrefix(lower, "work hours left in "); ok || strings.HasPrefix(lower, "workhours left in ") {
		if strings.HasPrefix(lower, "workhours left in ") {
			rest = strings.TrimSpace(s[len("workhours left in "):])
		} else {
			rest = strings.TrimSpace(s[len("work hours left in "):])
		}
		year, err := strconv.Atoi(rest)
		if err != nil || year < 1 || year > 9999 {
			return nil, true, invalid("expected a year")
		}
		q.kind = "workhoursLeft"
		q.year = year
		return q, true, nil
	}
	if rest, ok := strings.CutPrefix(lower, "workhours in "); ok || strings.HasPrefix(lower, "work hours in ") {
		if strings.HasPrefix(lower, "work hours in ") {
			rest = strings.TrimSpace(s[len("work hours in "):])
		} else {
			rest = strings.TrimSpace(s[len("workhours in "):])
		}
		if year, err := strconv.Atoi(rest); err == nil && year >= 1 && year <= 9999 {
			q.kind = "workhours"
			q.year = year
			return q, true, nil
		}
		q.kind = "workhoursMonth"
		q.date = rest
		return q, true, nil
	}
	if rest, ok := strings.CutPrefix(lower, "work hours between "); ok || strings.HasPrefix(lower, "workhours between ") {
		if strings.HasPrefix(lower, "workhours between ") {
			rest = strings.TrimSpace(s[len("workhours between "):])
		} else {
			rest = strings.TrimSpace(s[len("work hours between "):])
		}
		parts := strings.SplitN(rest, " and ", 2)
		if len(parts) != 2 {
			return nil, true, invalid("expected work hours between dates")
		}
		q.kind = "workhoursBetween"
		q.date = strings.TrimSpace(parts[0])
		q.source = strings.TrimSpace(parts[1])
		return q, true, nil
	}
	if m := hoursUntilRE.FindStringSubmatch(s); m != nil {
		q.kind = "hoursUntil"
		q.date = strings.TrimSpace(m[1])
		return q, true, nil
	}
	if strings.HasPrefix(lower, "days before ") || strings.HasPrefix(lower, "day before ") {
		q.kind = "until"
		q.date = strings.TrimSpace(s[strings.Index(lower, "before")+6:])
		return q, true, nil
	}
	if strings.HasPrefix(lower, "days until ") || strings.HasPrefix(lower, "day until ") {
		q.kind = "until"
		q.date = strings.TrimSpace(s[strings.Index(lower, "until")+5:])
		return q, true, nil
	}
	if strings.HasPrefix(lower, "days till ") || strings.HasPrefix(lower, "day till ") {
		q.kind = "until"
		q.date = strings.TrimSpace(s[strings.Index(lower, "till")+4:])
		return q, true, nil
	}
	if strings.HasPrefix(lower, "days since ") || strings.HasPrefix(lower, "day since ") {
		q.kind = "since"
		q.date = strings.TrimSpace(s[strings.Index(lower, "since")+5:])
		return q, true, nil
	}
	if m := weekdayRE.FindStringSubmatch(s); m != nil {
		q.kind = "weekday"
		q.weekday, _ = parseWeekdayName(m[1])
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
	if match := clockPrefixRE.FindStringSubmatch(s); match != nil && !dateArithmeticRE.MatchString(s) && !strings.HasPrefix(strings.TrimSpace(s[len(match[0]):]), "-") && !strings.HasPrefix(strings.TrimSpace(s[len(match[0]):]), "+") && !strings.HasPrefix(strings.TrimSpace(s[len(match[0]):]), "<") && !strings.HasPrefix(strings.TrimSpace(s[len(match[0]):]), ">") && !strings.HasPrefix(strings.TrimSpace(s[len(match[0]):]), "=") {
		q.date = match[1]
		q.clock = match[2]
		rest := strings.TrimSpace(s[len(match[0]):])
		if isDurationUnitWord(rest) {
			return nil, false, nil
		}
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
			restLower := strings.ToLower(rest)
			target := ""
			switch {
			case strings.HasPrefix(restLower, "here to "):
				target = strings.TrimSpace(rest[len("here to "):])
			case strings.HasPrefix(restLower, "to "):
				target = strings.TrimSpace(rest[3:])
				if _, e := parseClock(target); e == nil {
					return nil, false, nil
				}
			}
			if target != "" {
				if isFormatWord(strings.ToLower(target)) {
					return nil, false, nil
				}
				if _, err := c.location(target); err == nil {
					q.kind = "convert"
					q.source = "utc"
					q.target = target
					if _, err := parseClock(q.clock); err != nil {
						return nil, true, err
					}
					return q, true, nil
				}
			}
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
	if m := compactClockRE.FindStringSubmatch(s); m != nil && !dateArithmeticRE.MatchString(s) {
		clock, ok := parseCompactClock(m[1])
		if ok {
			rest := strings.TrimSpace(m[2])
			parts := zoneConversionRE.Split(rest, 2)
			if len(parts) == 2 {
				if _, err := c.location(parts[0]); err == nil {
					if _, err := c.location(parts[1]); err == nil {
						q.kind = "convert"
						q.clock = clock
						q.source = parts[0]
						q.target = parts[1]
						return q, true, nil
					}
				}
			}
		}
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
	if strings.HasPrefix(lower, "time saved ") || strings.HasPrefix(lower, "time from ") || strings.HasPrefix(lower, "time to ") || strings.HasPrefix(lower, "time since ") {
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
	s = strings.TrimSpace(strings.Trim(s, ","))
	if loc, ok := parseUTCOffset(s); ok {
		return loc, nil
	}
	if strings.HasSuffix(strings.ToLower(s), " time") {
		s = strings.TrimSpace(s[:len(s)-len(" time")])
	}
	if strings.EqualFold(s, "here") || strings.EqualFold(s, "local") {
		s = "utc"
	}
	names := []string{s}
	if i := strings.LastIndex(s, ","); i >= 0 {
		names = append(names, strings.TrimSpace(s[:i]))
		if j := strings.Index(s, ","); j >= 0 && j != i {
			names = append(names, strings.TrimSpace(s[:j]))
		}
	}
	var last string
	for _, name := range names {
		last = name
		lookup := name
		if alias, ok := c.Zones[strings.ToLower(name)]; ok {
			lookup = alias
		}
		if l, err := time.LoadLocation(lookup); err == nil {
			return l, nil
		}
	}
	return nil, invalid("unknown timezone " + last)
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
func parseWeekdayName(s string) (time.Weekday, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "sunday", "sun":
		return time.Sunday, true
	case "monday", "mon":
		return time.Monday, true
	case "tuesday", "tue", "tues":
		return time.Tuesday, true
	case "wednesday", "wed":
		return time.Wednesday, true
	case "thursday", "thu", "thur", "thurs":
		return time.Thursday, true
	case "friday", "fri":
		return time.Friday, true
	case "saturday", "sat":
		return time.Saturday, true
	}
	return 0, false
}

func clockHasAmpm(s string) bool {
	lower := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), ".", "")
	return strings.HasSuffix(lower, "am") || strings.HasSuffix(lower, "pm") ||
		strings.Contains(lower, " am") || strings.Contains(lower, " pm")
}

func parseCompactClock(digits string) (string, bool) {
	if len(digits) == 3 {
		digits = "0" + digits
	}
	if len(digits) != 4 {
		return "", false
	}
	h, err1 := strconv.Atoi(digits[:2])
	min, err2 := strconv.Atoi(digits[2:])
	if err1 != nil || err2 != nil || h > 23 || min > 59 {
		return "", false
	}
	return fmt.Sprintf("%02d:%02d", h, min), true
}

func parseClock(s string) (time.Time, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = normalizeClockAmpm(s)
	for _, layout := range []string{"3:04:05.999 pm", "3:04:05.999pm", "15:04:05.999", "3:04:05 pm", "3:04:05pm", "3:04 pm", "3:04pm", "3pm", "3 pm", "15:04:05", "15:04"} {
		if t, err := time.Parse(layout, s); err == nil {
			return time.Date(2000, 1, 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC), nil
		}
	}
	return time.Time{}, invalid("invalid clock")
}

// normalizeClockAmpm turns dotted P.M./A.M. into pm/am without touching fractional seconds.
func normalizeClockAmpm(s string) string {
	lower := strings.ToLower(strings.TrimSpace(s))
	for _, suf := range []string{"p.m.", "p.m", "pm.", "a.m.", "a.m", "am."} {
		if strings.HasSuffix(lower, suf) {
			repl := "pm"
			if strings.HasPrefix(suf, "a") {
				repl = "am"
			}
			return strings.TrimSpace(s[:len(s)-len(suf)]) + " " + repl
		}
	}
	return s
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
	s = strings.TrimSpace(strings.Trim(s, ","))
	if t, ok := parseQuarterOrPeriod(s, now); ok {
		return t, nil
	}
	if t, ok := parseRelativeWeekday(s, now); ok {
		return t, nil
	}
	if t, ok := parseHolidayName(s, now); ok {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02", s, now.Location()); err == nil {
		return t, nil
	}
	if n := numericDateRE.FindStringSubmatch(s); n != nil {
		day, _ := strconv.Atoi(n[1])
		monthN, _ := strconv.Atoi(n[2])
		year, _ := strconv.Atoi(n[3])
		t := time.Date(year, time.Month(monthN), day, 0, 0, 0, 0, now.Location())
		if year < 1 || year > 9999 || int(t.Month()) != monthN || t.Day() != day {
			return time.Time{}, invalid("invalid calendar date")
		}
		return t, nil
	}
	m := dateRE.FindStringSubmatch(s)
	if m == nil {
		if first := monthFirstRE.FindStringSubmatch(s); first != nil {
			m = []string{first[0], first[2], first[1], first[3]}
		}
	}
	if m == nil {
		lower := strings.ToLower(s)
		months := map[string]time.Month{"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6, "jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12}
		if len(lower) >= 3 {
			if month, ok := months[lower[:3]]; ok && (lower == strings.ToLower(month.String()) || lower == strings.ToLower(month.String()[:3])) {
				return time.Date(now.Year(), month, 1, 0, 0, 0, 0, now.Location()), nil
			}
		}
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
		if month == time.February && day == 29 && m[3] == "" {
			for y := year; y <= year+8; y++ {
				cand := time.Date(y, time.February, 29, 0, 0, 0, 0, now.Location())
				if cand.Month() == time.February && cand.Day() == 29 {
					if y == now.Year() && !cand.After(now) {
						continue
					}
					return cand, nil
				}
			}
		}
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
	case "yearsTo":
		n := q.year - now.Year()
		if n < 0 {
			n = -n
		}
		v = Value{Kind: Quantity, Number: big.NewRat(int64(n), 1), Unit: Unit{"y": 1}}
	case "lunar":
		v = Value{Kind: Number, Number: ratPlaces(lunarDay(now), 2)}
	case "daylight":
		lat, lon, ok := cityLatLon(q.target)
		if !ok {
			return r, invalid("unknown place")
		}
		lf, _ := strconv.ParseFloat(lat, 64)
		_, _ = lon, lf
		hours := daylightHours(lf, now)
		v = Value{Kind: Quantity, Number: ratPlaces(hours, 2), Unit: Unit{"h": 1}}
	case "relative":
		t := today
		switch strings.ToLower(q.date) {
		case "yesterday":
			t = today.AddDate(0, 0, -1)
		case "tomorrow":
			t = today.AddDate(0, 0, 1)
		}
		v = Value{Kind: Date, Time: t}
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
		v = Value{Kind: Clock, Time: t, Ampm: clockHasAmpm(q.clock)}
	case "dateIn":
		loc, e := c.location(q.target)
		if e != nil {
			return r, e
		}
		t := env.Now.In(loc)
		v = Value{Kind: Date, Time: time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)}
	case "nextWeekday":
		d := (int(q.weekday) - int(today.Weekday()) + 7) % 7
		if d == 0 {
			d = 7
		}
		v = Value{Kind: Date, Time: today.AddDate(0, 0, d)}
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
		v.Time, err = parseISOTime(q.date, env.Local)
		if err == nil {
			v.Time = v.Time.In(env.Local)
		}
		r.Target = "timestamp"
	case "sunrise", "sunset":
		lat, lon, ok := cityLatLon(q.target)
		if !ok {
			return r, invalid("unknown place")
		}
		lf, _ := strconv.ParseFloat(lat, 64)
		lonf, _ := strconv.ParseFloat(lon, 64)
		loc, e := c.location(q.target)
		if e != nil {
			return r, e
		}
		day := now.In(loc)
		if q.date != "" {
			day, e = parseDate(q.date, day)
			if e != nil {
				return r, e
			}
		}
		v = Value{Kind: Instant, Time: sunEvent(lf, lonf, day, loc, q.kind == "sunrise")}
		r.Target = "timestamp"
	case "hoursUntil":
		target, e := parseDate(q.date, now)
		if e != nil {
			return r, e
		}
		hours := target.Sub(now).Hours()
		if hours < 0 {
			hours = -hours
		}
		v = Value{Kind: Quantity, Number: ratPlaces(hours, 2), Unit: Unit{"h": 1}}
	case "until", "since":
		target, e := parseDate(q.date, now)
		if e != nil {
			return r, e
		}
		if !explicitYearRE.MatchString(q.date) && target.Before(today) && q.kind == "until" {
			target, e = parseDate(q.date, now.AddDate(1, 0, 0))
			if e != nil {
				return r, e
			}
		}
		if !explicitYearRE.MatchString(q.date) && !target.Before(today) && q.kind == "since" {
			target, e = parseDate(q.date, now.AddDate(-1, 0, 0))
			if e != nil {
				return r, e
			}
		}
		a := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
		b := time.Date(target.Year(), target.Month(), target.Day(), 0, 0, 0, 0, time.UTC)
		days := (b.Unix() - a.Unix()) / 86400
		if q.kind == "since" {
			days = -days
		}
		v = Value{Kind: Quantity, Number: big.NewRat(days, 1), Unit: Unit{"d": 1}}
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
	case "workhours", "workhoursMonth", "workhoursBetween", "workhoursLeft":
		start, end := time.Time{}, time.Time{}
		switch q.kind {
		case "workhours":
			start = time.Date(q.year, 1, 1, 0, 0, 0, 0, time.UTC)
			end = time.Date(q.year+1, 1, 1, 0, 0, 0, 0, time.UTC)
		case "workhoursLeft":
			start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
			if now.Year() != q.year {
				start = time.Date(q.year, 1, 1, 0, 0, 0, 0, time.UTC)
			}
			end = time.Date(q.year+1, 1, 1, 0, 0, 0, 0, time.UTC)
		case "workhoursMonth":
			t, e := parseDate(q.date, now)
			if e != nil {
				return r, e
			}
			start = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
			end = start.AddDate(0, 1, 0)
		default:
			a, e := parseDate(q.date, now)
			if e != nil {
				return r, e
			}
			b, e := parseDate(q.source, now)
			if e != nil {
				return r, e
			}
			start = time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, time.UTC)
			end = time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
		}
		count := int64(0)
		for t := start; t.Before(end); t = t.AddDate(0, 0, 1) {
			if t.Weekday() != time.Saturday && t.Weekday() != time.Sunday {
				count += 8
			}
		}
		v = Value{Kind: Quantity, Number: big.NewRat(count, 1), Unit: Unit{"h": 1}}
	case "dateRange":
		start, e := parseDate(q.date, now)
		if e != nil {
			return r, e
		}
		end, e := parseDate(q.source, now)
		if e != nil {
			return r, e
		}
		v, err = civilInterval(start, end, false)
	case "clockRange":
		start, e := parseClock(q.clock)
		if e != nil {
			return r, e
		}
		end, e := parseClock(q.date)
		if e != nil {
			return r, e
		}
		x := int64(start.Hour()*3600 + start.Minute()*60 + start.Second())
		y := int64(end.Hour()*3600 + end.Minute()*60 + end.Second())
		sec := y - x
		if sec < 0 {
			sec += 86400
		}
		v = Value{Kind: Quantity, Number: big.NewRat(sec, 3600), Unit: Unit{"h": 1}}
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
		hasClock := t.Hour() != 0 || t.Minute() != 0 || t.Second() != 0
		switch {
		case strings.HasPrefix(q.unit, "hour"):
			v = Value{Kind: Instant, Time: t.Add(time.Duration(q.amount) * time.Hour)}
		case strings.HasPrefix(q.unit, "minute"):
			v = Value{Kind: Instant, Time: t.Add(time.Duration(q.amount) * time.Minute)}
		default:
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
			t = addCivilDate(t, months, days)
			if years != 0 {
				t = addCivilDate(t, years*12, 0)
			}
			if hasClock {
				v = Value{Kind: Instant, Time: t}
			} else {
				v = Value{Kind: Date, Time: t}
			}
		}
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

func lunarDay(now time.Time) float64 {
	ref := time.Date(2000, 1, 6, 18, 14, 0, 0, time.UTC)
	const synodic = 29.530588
	days := now.UTC().Sub(ref).Hours() / 24
	age := math.Mod(days, synodic)
	if age < 0 {
		age += synodic
	}
	return age
}

func daylightHours(lat float64, t time.Time) float64 {
	n := float64(t.YearDay())
	decl := 23.44 * math.Sin(2*math.Pi/365*(n-81)) * math.Pi / 180
	latr := lat * math.Pi / 180
	x := -math.Tan(latr) * math.Tan(decl)
	if x <= -1 {
		return 24
	}
	if x >= 1 {
		return 0
	}
	return 24 * math.Acos(x) / math.Pi
}

func parseISOTime(s string, loc *time.Location) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
		if loc != nil {
			if t, err := time.ParseInLocation(layout, s, loc); err == nil {
				return t, nil
			}
		}
	}
	return time.Time{}, invalid("invalid ISO8601")
}

// offsetText includes fractional-hour timezone offsets without losing their sign.
func parseQuarterOrPeriod(s string, now time.Time) (time.Time, bool) {
	lower := strings.ToLower(strings.TrimSpace(s))
	year := now.Year()
	qtr := 0
	switch {
	case strings.HasPrefix(lower, "q") && len(lower) >= 2 && lower[1] >= '1' && lower[1] <= '4':
		qtr = int(lower[1] - '0')
		rest := strings.TrimSpace(strings.Trim(lower[2:], ","))
		if rest != "" {
			y, err := strconv.Atoi(rest)
			if err != nil {
				return time.Time{}, false
			}
			year = y
		} else {
			start := time.Date(year, time.Month((qtr-1)*3+1), 1, 0, 0, 0, 0, now.Location())
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			if start.Before(today) {
				year++
			}
		}
	case lower == "next quarter":
		qtr = quarterOf(now) + 1
		if qtr > 4 {
			qtr = 1
			year++
		}
	case lower == "this quarter":
		qtr = quarterOf(now)
	case lower == "last quarter":
		qtr = quarterOf(now) - 1
		if qtr < 1 {
			qtr = 4
			year--
		}
	case lower == "next year":
		return time.Date(year+1, 1, 1, 0, 0, 0, 0, now.Location()), true
	case lower == "next week":
		return nextWeekStart(now, 1), true
	case lower == "this week":
		return nextWeekStart(now, 0), true
	case lower == "last week":
		return nextWeekStart(now, -1), true
	default:
		return time.Time{}, false
	}
	if qtr == 0 {
		return time.Time{}, false
	}
	return time.Date(year, time.Month((qtr-1)*3+1), 1, 0, 0, 0, 0, now.Location()), true
}

func quarterOf(t time.Time) int {
	return (int(t.Month())-1)/3 + 1
}

func nextWeekStart(now time.Time, weeks int) time.Time {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	offset := int(today.Weekday())
	if offset == 0 {
		offset = 7
	}
	monday := today.AddDate(0, 0, -(offset - 1))
	return monday.AddDate(0, 0, 7*weeks)
}

func sunEvent(lat, lon float64, day time.Time, loc *time.Location, rise bool) time.Time {
	hours := daylightHours(lat, day)
	noonUTC := time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, time.UTC)
	noonUTC = noonUTC.Add(-time.Duration(lon/15*3600) * time.Second)
	half := time.Duration(hours/2*3600) * time.Second
	if rise {
		return noonUTC.Add(-half).In(loc)
	}
	return noonUTC.Add(half).In(loc)
}

func isClockScaleUnit(unit string) bool {
	switch strings.ToLower(unit) {
	case "millisecond", "milliseconds", "second", "seconds", "minute", "minutes", "min", "mins", "hour", "hours":
		return true
	}
	return false
}

func isDurationUnitWord(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "hour", "hours", "hr", "hrs", "h", "minute", "minutes", "min", "mins", "second", "seconds", "sec", "secs":
		return true
	}
	return false
}

// parseRelativeWeekday resolves "Tuesday next week" and an optional clock.
func parseRelativeWeekday(s string, now time.Time) (time.Time, bool) {
	m := relativeWeekdayRE.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, false
	}
	wd, ok := parseWeekdayName(m[1])
	if !ok {
		return time.Time{}, false
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	offset := int(today.Weekday())
	if offset == 0 {
		offset = 7
	}
	monday := today.AddDate(0, 0, -(offset - 1))
	switch strings.ToLower(m[2]) {
	case "next":
		monday = monday.AddDate(0, 0, 7)
	case "last":
		monday = monday.AddDate(0, 0, -7)
	}
	day := int(wd)
	if day == 0 {
		day = 7
	}
	t := monday.AddDate(0, 0, day-1)
	if rest := strings.TrimSpace(m[3]); rest != "" {
		clock, err := parseClock(rest)
		if err != nil {
			return time.Time{}, false
		}
		t = time.Date(t.Year(), t.Month(), t.Day(), clock.Hour(), clock.Minute(), clock.Second(), 0, now.Location())
	}
	return t, true
}

func offsetText(t time.Time) string {
	_, offset := t.Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	return fmt.Sprintf("UTC%s%02d:%02d", sign, offset/3600, offset%3600/60)
}
