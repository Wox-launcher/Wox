package engine

import (
	"math/big"
	"regexp"
	"strings"
	"time"
)

var isoPrefix = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})`)
var baseTargetSuffix = regexp.MustCompile(`(?i)^\s*(?:to|in|=\s*\?)\s+(?:hex|bin|oct|dec)\b`)
var datePrefix = regexp.MustCompile(`(?i)^(?:\d{4}-\d{2}-\d{2}|(?:january|february|march|april|may|june|july|august|september|october|november|december|jan|feb|mar|apr|jun|jul|aug|sep|oct|nov|dec)\s+\d{1,2}(?:\s+\d{4})?|\d{1,2}\s+(?:jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)(?:\s+\d{4})?)(?:\b)`)
var clockLiteralPrefix = regexp.MustCompile(`(?i)^` + clockSyntax + `\b`)

// temporalLiteral recognizes literal syntax before ordinary numeric punctuation.
func temporalLiteral(input string) (string, *temporalQuery) {
	if s := isoPrefix.FindString(input); s != "" {
		return s, &temporalQuery{kind: "iso", date: s}
	}
	if s := datePrefix.FindString(input); s != "" {
		if baseTargetSuffix.MatchString(input[len(s):]) && (strings.HasSuffix(strings.ToLower(s), "oct") || strings.HasSuffix(strings.ToLower(s), "dec")) {
			return "", nil
		}
		rest := input[len(s):]
		trimmed := strings.TrimLeft(rest, " \t")
		if clock := clockLiteralPrefix.FindString(trimmed); len(trimmed) < len(rest) && clock != "" {
			return s + rest[:len(rest)-len(trimmed)] + clock, &temporalQuery{kind: "localLiteral", date: s, clock: clock}
		}
		return s, &temporalQuery{kind: "dateLiteral", date: s}
	}
	if s := clockLiteralPrefix.FindString(input); s != "" {
		return s, &temporalQuery{kind: "clockLiteral", clock: s}
	}
	return "", nil
}

// temporalOperation is the explicit type matrix for calendar values. Unsupported
// combinations fail here instead of being coerced into timestamp arithmetic.
func (c *Catalog) temporalOperation(a, b Value, op string, env Env) (Value, error) {
	subtract := op == "-"
	if op != "+" && op != "-" && op != "*" {
		return Value{}, invalid("unsupported temporal operation")
	}
	if a.Kind == CalendarSpan {
		if b.Kind == CalendarSpan && (op == "+" || op == "-") {
			sign := 1
			if subtract {
				sign = -1
			}
			a.Months += sign * b.Months
			a.Days += sign * b.Days
			return boundedCalendar(a)
		}
		if b.Kind == Number && op == "*" && b.Number.IsInt() && b.Number.Num().IsInt64() {
			n := b.Number.Num().Int64()
			if n < -120000 || n > 120000 {
				return Value{}, invalid("calendar multiplier exceeds limit")
			}
			a.Months *= int(n)
			a.Days *= int(n)
			return boundedCalendar(a)
		}
		return Value{}, invalid("invalid calendar operation")
	}
	if op == "*" {
		return Value{}, invalid("cannot multiply temporal values")
	}
	if a.Kind == b.Kind && subtract {
		switch a.Kind {
		case Date:
			x := time.Date(a.Time.Year(), a.Time.Month(), a.Time.Day(), 0, 0, 0, 0, time.UTC)
			y := time.Date(b.Time.Year(), b.Time.Month(), b.Time.Day(), 0, 0, 0, 0, time.UTC)
			return Value{Kind: Quantity, Number: big.NewRat((x.Unix()-y.Unix())/86400, 1), Unit: Unit{"d": 1}}, nil
		case Instant:
			seconds := big.NewRat(a.Time.Unix()-b.Time.Unix(), 1)
			seconds.Add(seconds, big.NewRat(int64(a.Time.Nanosecond()-b.Time.Nanosecond()), 1e9))
			return Value{Kind: Quantity, Number: seconds, Unit: Unit{"s": 1}}, nil
		case Clock:
			x := int64(a.Days*86400 + a.Time.Hour()*3600 + a.Time.Minute()*60 + a.Time.Second())
			y := int64(b.Days*86400 + b.Time.Hour()*3600 + b.Time.Minute()*60 + b.Time.Second())
			return Value{Kind: Quantity, Number: big.NewRat(x-y, 3600), Unit: Unit{"h": 1}}, nil
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
		a.Time = a.Time.AddDate(0, int(sign)*months, int(sign)*days)
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
