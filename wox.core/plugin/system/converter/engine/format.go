package engine

import (
	"fmt"
	"math/big"
	"strings"
	"time"
)

type FormatOptions struct {
	ThousandsSeparator, DecimalSeparator string
	Translate                            func(string) string
}

// plain renders the shared 16-place output precision without intermediate rounding.
func plain(n *big.Rat) string {
	if n == nil {
		return ""
	}
	s := n.FloatString(16)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "-0" {
		return "0"
	}
	return s
}

// formatNumber groups only the numeric portion and preserves its sign.
func formatNumber(s string, o FormatOptions) string {
	parts := strings.SplitN(s, ".", 2)
	integer := parts[0]
	sign := ""
	if strings.HasPrefix(integer, "-") {
		sign = "-"
		integer = integer[1:]
	}
	if o.ThousandsSeparator != "" {
		for i := len(integer) - 3; i > 0; i -= 3 {
			integer = integer[:i] + o.ThousandsSeparator + integer[i:]
		}
	}
	result := sign + integer
	if len(parts) > 1 {
		sep := o.DecimalSeparator
		if sep == "" {
			sep = "."
		}
		result += sep + parts[1]
	}
	return result
}

// Format is the only layer that rounds or translates values. Raw retains required units.
func (c *Catalog) Format(r Evaluation, o FormatOptions) Presentation {
	p := Presentation{Expression: r.Expression, Currency: r.Currency, RateUpdatedAt: r.RateUpdatedAt}
	v := r.Value
	translate := func(key, fallback string) string {
		if o.Translate != nil {
			s := o.Translate(key)
			if s != "" && s != key {
				return s
			}
		}
		return fallback
	}
	weekday := func(t time.Time) string {
		keys := []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}
		return translate("ui_weekday_"+keys[t.Weekday()], t.Weekday().String()[:3])
	}
	switch v.Kind {
	case CalendarSpan:
		p.Raw = fmt.Sprintf("%d months %d days", v.Months, v.Days)
		p.Formatted = p.Raw
		return p
	case Date:
		p.Raw = v.Time.Format("2006-01-02")
		p.Formatted = fmt.Sprintf(translate("plugin_converter_weekday_date_format", "%s, %s"), weekday(v.Time), p.Raw)
		return p
	case Clock:
		p.Raw = v.Time.Format("15:04")
		p.Formatted = p.Raw
		if v.Days != 0 {
			p.Formatted += fmt.Sprintf(" (%+d d)", v.Days)
			p.Raw = p.Formatted
		}
		return p
	case Instant:
		p.Raw = v.Time.Format(time.RFC3339Nano)
		clock := fmt.Sprintf(translate("plugin_converter_time_format", "%02d:%02d (%s)"), v.Time.Hour(), v.Time.Minute(), weekday(v.Time))
		p.Formatted = clock
		if r.Target != "current" {
			p.Formatted = fmt.Sprintf(translate("plugin_converter_time_with_date_format", "%s (%s)"), clock, v.Time.Format("2006-01-02"))
		}
		p.TimeZone = v.Time.Location().String()
		p.SubTitle = v.Time.Format("2006-01-02 15:04") + " " + p.TimeZone + " " + offsetText(v.Time)
		if r.Source != nil {
			s := *r.Source
			p.SubTitle = s.Format("2006-01-02 15:04") + " " + s.Location().String() + " " + offsetText(s) + " → " + p.SubTitle
		}
		return p
	}
	if isBase(r.Target) {
		base := map[string]int{"bin": 2, "oct": 8, "dec": 10, "hex": 16}[r.Target]
		p.Raw = v.Number.Num().Text(base)
		if base == 16 {
			p.Raw = strings.ToUpper(p.Raw)
		}
		p.Formatted = p.Raw + " " + r.Target
		return p
	}
	if r.Target == "timespan" {
		f, _ := c.factor(v.Unit, Env{})
		seconds := new(big.Rat).Mul(v.Number, f)
		p.Raw = plain(seconds) + " s"
		negative := seconds.Sign() < 0
		seconds.Abs(seconds)
		integer := new(big.Int).Quo(seconds.Num(), seconds.Denom())
		fraction := new(big.Rat).Sub(seconds, new(big.Rat).SetInt(integer))
		parts := []string{}
		for _, part := range []struct {
			n int64
			s string
		}{{86400, "d"}, {3600, "h"}, {60, "min"}, {1, "s"}} {
			q, rem := new(big.Int), new(big.Int)
			q.QuoRem(integer, big.NewInt(part.n), rem)
			if part.n == 1 && fraction.Sign() != 0 {
				parts = append(parts, formatNumber(plain(new(big.Rat).Add(new(big.Rat).SetInt(q), fraction)), o)+" s")
			} else if q.Sign() != 0 {
				parts = append(parts, q.String()+" "+part.s)
			}
			integer = rem
		}
		if len(parts) == 0 {
			parts = append(parts, "0 s")
		}
		p.Formatted = strings.Join(parts, " ")
		if negative {
			p.Formatted = "-" + p.Formatted
		}
		return p
	}
	s := plain(v.Number)
	p.Raw = s
	if v.Kind == Percent {
		s = plain(new(big.Rat).Mul(v.Number, big.NewRat(100, 1)))
		p.Formatted = formatNumber(s, o) + "%"
		return p
	}
	if v.Kind == Quantity {
		// Existing duration/storage/unit rows do not group digits; parsing still shares Calculator settings.
		if c.dimensions(v.Unit)["money"] == 0 {
			o.ThousandsSeparator = ""
		}
		p.Raw += " " + unitText(v.Unit)
		unit := unitText(v.Unit)
		if len(v.Unit) == 1 && firstExponent(v.Unit) == 1 {
			for name := range v.Unit {
				d := c.Units[name]
				precision := -1
				switch d.Dimension {
				case "money", "mass", "temperature", "volume":
					precision = 2
				case "length":
					precision = 3
				case "time":
					precision = 2
				}
				if precision >= 0 {
					s = strings.TrimRight(strings.TrimRight(v.Number.FloatString(precision), "0"), ".")
					if s == "" || s == "-0" {
						s = "0"
					}
				}
				if d.Dimension == "time" {
					key := map[string]string{"ms": "milliseconds", "s": "seconds", "min": "minutes", "h": "hours", "d": "days", "w": "weeks", "y": "years", "workday": "workdays"}[name]
					unit = translate("plugin_converter_time_unit_"+key, d.Plural)
					if name == "w" && !v.Number.IsInt() {
						scaled := new(big.Rat).Mul(v.Number, big.NewRat(1000, 1))
						s = new(big.Rat).SetFrac(new(big.Int).Quo(scaled.Num(), scaled.Denom()), big.NewInt(1000)).FloatString(3)
					} else if !v.Number.IsInt() {
						s = v.Number.FloatString(2)
					}
				} else if d.Dimension != "storage" && d.Dimension != "money" {
					unit = d.Plural
					if new(big.Rat).Abs(v.Number).Cmp(big.NewRat(1, 1)) == 0 {
						unit = d.Singular
					}
				}
			}
		}
		if c.dimensions(v.Unit)["money"] == 1 {
			for code := range v.Unit {
				if c.Units[code].Dimension != "money" {
					continue
				}
				symbol := map[string]string{"USD": "$", "EUR": "€", "GBP": "£", "CNY": "¥", "JPY": "¥", "INR": "₹", "HKD": "HK$", "AUD": "A$", "CAD": "C$"}[code]
				if symbol != "" {
					rest := copyUnit(v.Unit)
					delete(rest, code)
					p.Formatted = symbol + formatNumber(s, o)
					if len(rest) > 0 {
						suffix := unitText(rest)
						p.Formatted += strings.TrimPrefix(suffix, "1")
					}
					return p
				}
			}
		}
		p.Formatted = formatNumber(s, o) + " " + unit
		return p
	}
	p.Formatted = formatNumber(s, o)
	if r.Ratio != "" {
		p.Formatted = r.Ratio
	}
	return p
}
