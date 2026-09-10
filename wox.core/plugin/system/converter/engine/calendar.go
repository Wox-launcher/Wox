package engine

import (
	"math/big"
	"regexp"
	"strings"
	"time"
	"unicode"
)

var isoPrefix = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})?`)
var baseTargetSuffix = regexp.MustCompile(`(?i)^\s*(?:to|in|=\s*\?)\s+(?:hex|bin|oct|dec)\b`)
var monthNamePat = `january|february|march|april|may|june|july|august|september|october|november|december|jan|feb|mar|apr|jun|jul|aug|sep|oct|nov|dec`
var datePrefix = regexp.MustCompile(`(?i)^(?:\d{4}-\d{2}-\d{2}|(?:` + monthNamePat + `)\s+\d{1,2}(?:st|nd|rd|th)?(?:,?\s+\d{2,4})?|\d{1,2}(?:st|nd|rd|th)?\s+(?:` + monthNamePat + `)(?:,?\s+\d{2,4})?|\d{1,2}[/.]\d{1,2}[/.]\d{4})(?:\b)`)
var clockLiteralPrefix = regexp.MustCompile(`(?i)^` + clockSyntax)
var dateStampClockRE = regexp.MustCompile(`(?i)^\d{1,2}:\d{2}:\d{2}(?:\.\d+)?(?:\s*(?:a\.?m\.?|p\.?m\.?))?`)
var shortDatePrefix = regexp.MustCompile(`(?i)^(?:(?:` + monthNamePat + `)\s+\d{1,2}(?:st|nd|rd|th)?|\d{1,2}(?:st|nd|rd|th)?\s+(?:` + monthNamePat + `))`)

func splitDatePattern(input string) (expr, pattern string, ok bool) {
	lower := strings.ToLower(input)
	i := strings.LastIndex(lower, " as ")
	if i < 0 {
		return "", "", false
	}
	rest := strings.TrimSpace(input[i+4:])
	if !looksLikeDatePattern(rest) {
		return "", "", false
	}
	return strings.TrimSpace(input[:i]), rest, true
}

func looksLikeDatePattern(s string) bool {
	l := strings.ToLower(s)
	if strings.Contains(l, "yyyy") || strings.Contains(l, "eeee") || strings.Contains(l, "mmm") {
		return true
	}
	if !strings.ContainsAny(s, "/-:") {
		return false
	}
	for _, r := range s {
		if unicode.IsLetter(r) && !strings.ContainsRune("yYmMdDeEhHsSaA", r) {
			return false
		}
	}
	return strings.ContainsAny(l, "ymd")
}

// goDateLayout maps Soulver/NSDateFormatter tokens onto Go's reference layout.
func goDateLayout(pattern string) string {
	repl := []struct{ from, to string }{
		{"EEEE", "Monday"}, {"EEEE", "Monday"}, {"eeee", "Monday"},
		{"EEE", "Mon"}, {"eee", "Mon"},
		{"MMMM", "January"}, {"mmmm", "January"},
		{"MMM", "Jan"}, {"mmm", "Jan"},
		{"MM", "01"}, {"yyyy", "2006"}, {"YYYY", "2006"},
		{"dd", "02"}, {"yy", "06"},
	}
	out := pattern
	for _, r := range repl {
		out = strings.ReplaceAll(out, r.from, r.to)
	}
	var b strings.Builder
	runes := []rune(out)
	for i, r := range runes {
		if (r == 'd' || r == 'D') && (i == 0 || !unicode.IsLetter(runes[i-1])) && (i+1 == len(runes) || !unicode.IsLetter(runes[i+1])) {
			b.WriteByte('2')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// matchClockLiteral reads a clock token and rejects glued letter/digit tails like "3pmax".
func matchClockLiteral(input string) string {
	s := clockLiteralPrefix.FindString(input)
	if s == "" {
		return ""
	}
	rest := input[len(s):]
	if rest != "" {
		r := []rune(rest)[0]
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ':' {
			return ""
		}
	}
	return s
}

// matchDateAttachedClock allows HH:MM:SS[.ms] after a date, which would otherwise be a laptime.
func matchDateAttachedClock(input string) string {
	if s := matchStampClock(input); s != "" {
		return s
	}
	return matchClockLiteral(input)
}

// matchStampClock reads HH:MM:SS[.ms] used on datestamps, not bare laptimes.
func matchStampClock(input string) string {
	s := dateStampClockRE.FindString(input)
	if s == "" {
		return ""
	}
	rest := input[len(s):]
	if rest != "" {
		r := []rune(rest)[0]
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ':' {
			return ""
		}
	}
	return s
}

// splitDateAndClock prefers "March 12, 09:30:35" over treating 09 as a two-digit year.
func splitDateAndClock(input string) (date, clock, full string) {
	date = shortDatePrefix.FindString(input)
	if date == "" {
		return "", "", ""
	}
	rest := input[len(date):]
	gap := strings.TrimLeft(rest, " \t")
	if strings.HasPrefix(gap, ",") {
		gap = strings.TrimLeft(gap[1:], " \t")
	}
	clock = matchDateAttachedClock(gap)
	if clock == "" || len(gap) >= len(rest) && !strings.Contains(rest, ",") {
		return "", "", ""
	}
	return date, clock, date + rest[:len(rest)-len(gap)] + clock
}

// temporalLiteral recognizes literal syntax before ordinary numeric punctuation.
func temporalLiteral(input string) (string, *temporalQuery) {
	if s := isoPrefix.FindString(input); s != "" {
		return s, &temporalQuery{kind: "iso", date: s}
	}
	if date, clock, full := splitDateAndClock(input); full != "" {
		return full, &temporalQuery{kind: "localLiteral", date: date, clock: clock}
	}
	if s := datePrefix.FindString(input); s != "" {
		if baseTargetSuffix.MatchString(input[len(s):]) && (strings.HasSuffix(strings.ToLower(s), "oct") || strings.HasSuffix(strings.ToLower(s), "dec")) {
			return "", nil
		}
		rest := input[len(s):]
		gap := strings.TrimLeft(rest, " \t")
		comma := false
		if strings.HasPrefix(gap, ",") {
			comma = true
			gap = strings.TrimLeft(gap[1:], " \t")
		}
		if clock := matchDateAttachedClock(gap); clock != "" && (comma || len(gap) < len(rest)) {
			return s + rest[:len(rest)-len(gap)] + clock, &temporalQuery{kind: "localLiteral", date: s, clock: clock}
		}
		return s, &temporalQuery{kind: "dateLiteral", date: s}
	}
	if s := matchClockLiteral(input); s != "" {
		return s, &temporalQuery{kind: "clockLiteral", clock: s}
	}
	return "", nil
}

// temporalOperation is the explicit type matrix for calendar values. Unsupported
// combinations fail here instead of being coerced into timestamp arithmetic.
func (c *Catalog) temporalOperation(a, b Value, op string, env Env) (Value, error) {
	subtract := op == "-"
	if op == "clock_to" && a.Kind == Clock && b.Kind == Clock {
		x := int64(a.Time.Hour()*3600 + a.Time.Minute()*60 + a.Time.Second())
		y := int64(b.Time.Hour()*3600 + b.Time.Minute()*60 + b.Time.Second())
		sec := y - x
		if sec < 0 {
			sec += 86400
		}
		return Value{Kind: Quantity, Number: big.NewRat(sec, 1), Unit: Unit{"s": 1}}, nil
	}
	if op == "clock_to" && a.Kind == Date && b.Kind == Date {
		return civilInterval(a.Time, b.Time, false)
	}
	if op != "+" && op != "-" && op != "*" {
		return Value{}, invalid("unsupported temporal operation")
	}
	if a.Kind == CalendarSpan || b.Kind == CalendarSpan {
		if v, ok, err := mergeCalendarSpan(a, b, op); ok || err != nil {
			return v, err
		}
	}
	if op == "*" {
		return Value{}, invalid("cannot multiply temporal values")
	}
	if a.Kind == b.Kind && subtract {
		switch a.Kind {
		case Date:
			x := time.Date(a.Time.Year(), a.Time.Month(), a.Time.Day(), 0, 0, 0, 0, time.UTC)
			y := time.Date(b.Time.Year(), b.Time.Month(), b.Time.Day(), 0, 0, 0, 0, time.UTC)
			if x.Before(y) {
				return civilInterval(x, y, false)
			}
			return Value{Kind: Quantity, Number: big.NewRat((x.Unix()-y.Unix())/86400, 1), Unit: Unit{"d": 1}}, nil
		case Instant:
			seconds := big.NewRat(a.Time.Unix()-b.Time.Unix(), 1)
			seconds.Add(seconds, big.NewRat(int64(a.Time.Nanosecond()-b.Time.Nanosecond()), 1e9))
			return Value{Kind: Quantity, Number: seconds, Unit: Unit{"s": 1}}, nil
		case Clock:
			x := int64(a.Days*86400 + a.Time.Hour()*3600 + a.Time.Minute()*60 + a.Time.Second())
			y := int64(b.Days*86400 + b.Time.Hour()*3600 + b.Time.Minute()*60 + b.Time.Second())
			sec := x - y
			if (a.Ampm || b.Ampm) && sec < 0 {
				sec = -sec
			}
			return Value{Kind: Quantity, Number: big.NewRat(sec, 3600), Unit: Unit{"h": 1}}, nil
		}
	}
	sign := int64(1)
	if subtract {
		sign = -1
	}
	if a.Kind == Date {
		months, days := 0, 0
		switch b.Kind {
		case Number:
			if !b.Number.IsInt() || !b.Number.Num().IsInt64() {
				return Value{}, invalid("date offset requires integer days")
			}
			if b.Number.Num().Int64() < -1000000 || b.Number.Num().Int64() > 1000000 {
				return Value{}, invalid("date offset exceeds limit")
			}
			days = int(b.Number.Num().Int64())
		case CalendarSpan:
			months = b.Months
			days = b.Days
		case Quantity:
			// Workdays are 8 hours; N*8h can land on an exact calendar day and must not skip weekday arithmetic.
			if b.Unit["workday"] == 1 && len(b.Unit) == 1 && b.Number != nil && b.Number.IsInt() && b.Number.Num().IsInt64() {
				n := b.Number.Num().Int64()
				if n < -1000000 || n > 1000000 {
					return Value{}, invalid("date offset exceeds limit")
				}
				a.Time = addWorkdays(a.Time, int(sign)*int(n))
				return boundedCalendar(a)
			}
			if sameUnit(c.dimensions(b.Unit), Unit{"time": 1}) && b.Number != nil {
				sec, err := c.factor(b.Unit, Env{})
				if err == nil {
					total := new(big.Rat).Mul(b.Number, sec)
					daysRat := new(big.Rat).Quo(total, big.NewRat(86400, 1))
					if daysRat.IsInt() && daysRat.Num().IsInt64() {
						n := daysRat.Num().Int64()
						if n >= -1000000 && n <= 1000000 {
							a.Time = addCivilDate(a.Time, 0, int(sign)*int(n))
							return boundedCalendar(a)
						}
					}
				}
			}
			if len(b.Unit) != 1 || !b.Number.IsInt() || !b.Number.Num().IsInt64() {
				return Value{}, invalid("date offset requires calendar units")
			}
			n := b.Number.Num().Int64()
			if n < -1000000 || n > 1000000 {
				return Value{}, invalid("date offset exceeds limit")
			}
			switch {
			case b.Unit["d"] == 1:
				days = int(n)
			case b.Unit["w"] == 1:
				days = int(n) * 7
			case b.Unit["y"] == 1:
				months = int(n) * 12
			default:
				return Value{}, invalid("date offset requires days, weeks or years")
			}
		default:
			return Value{}, invalid("invalid date offset")
		}
		a.Time = addCivilDate(a.Time, int(sign)*months, int(sign)*days)
		return boundedCalendar(a)
	}
	if a.Kind == Instant && b.Kind == CalendarSpan {
		date := a.Time.AddDate(0, int(sign)*b.Months, int(sign)*b.Days)
		t, err := localInstant(date, a.Time, a.Time.Location())
		if err != nil {
			return Value{}, err
		}
		a.Time = t
		return boundedCalendar(a)
	}
	if a.Kind == Clock || a.Kind == Instant {
		var seconds *big.Rat
		if b.Kind == Number && a.Kind == Clock {
			seconds = new(big.Rat).Mul(b.Number, big.NewRat(3600, 1))
		} else if b.Kind == Quantity && sameUnit(c.dimensions(b.Unit), Unit{"time": 1}) {
			factor, err := c.factor(b.Unit, env)
			if err != nil {
				return Value{}, err
			}
			seconds = new(big.Rat).Mul(b.Number, factor)
		} else {
			return Value{}, invalid("explicit duration required")
		}
		ns := new(big.Rat).Mul(seconds, big.NewRat(sign*1e9, 1))
		if !ns.IsInt() || !ns.Num().IsInt64() {
			return Value{}, invalid("duration outside nanosecond range")
		}
		if a.Kind == Instant {
			a.Time = a.Time.Add(time.Duration(ns.Num().Int64()))
			return boundedCalendar(a)
		}
		anchor := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
		t := time.Date(2000, 1, 1+a.Days, a.Time.Hour(), a.Time.Minute(), a.Time.Second(), a.Time.Nanosecond(), time.UTC).Add(time.Duration(ns.Num().Int64()))
		days := (t.Unix() - anchor.Unix()) / 86400
		if t.Before(anchor) && (t.Unix()-anchor.Unix())%86400 != 0 {
			days--
		}
		a.Days = int(days)
		a.Time = t
		return a, nil
	}
	return Value{}, invalid("incompatible temporal operands")
}

// addCivilDate keeps the day-of-month when possible and clamps to the last
// valid day so January 31 + 1 month is February 28/29, matching Soulver.
func addCivilDate(t time.Time, months, days int) time.Time {
	y, m, d := t.Date()
	total := int(m) + months
	y += (total - 1) / 12
	mod := (total - 1) % 12
	if mod < 0 {
		mod += 12
		y--
	}
	m = time.Month(mod + 1)
	last := time.Date(y, m+1, 0, 0, 0, 0, 0, t.Location()).Day()
	if d > last {
		d = last
	}
	return time.Date(y, m, d, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location()).AddDate(0, 0, days)
}

// boundedCalendar rejects results outside the documented civil-date and span limits.
func boundedCalendar(v Value) (Value, error) {
	if v.Months < -120000 || v.Months > 120000 || v.Days < -1000000 || v.Days > 1000000 {
		return Value{}, invalid("calendar span exceeds limit")
	}
	if !v.Time.IsZero() && (v.Time.Year() < 1 || v.Time.Year() > 9999) {
		return Value{}, invalid("date exceeds supported range")
	}
	return v, nil
}
