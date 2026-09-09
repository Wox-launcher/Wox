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

// roundToNearest applies ceil, floor, or half-away-from-zero to a multiple of step.
func roundToNearest(n, step *big.Rat, dir string) *big.Rat {
	q := new(big.Rat).Quo(n, step)
	var k *big.Int
	switch dir {
	case "up":
		k = ceilRat(q)
	case "down":
		k = floorRat(q)
	default:
		k = roundHalfAwayRat(q)
	}
	return new(big.Rat).Mul(new(big.Rat).SetInt(k), step)
}

// floorRat is mathematical floor; QuoRem truncates toward zero.
func floorRat(r *big.Rat) *big.Int {
	q, rem := new(big.Int).QuoRem(new(big.Int).Set(r.Num()), r.Denom(), new(big.Int))
	if rem.Sign() != 0 && r.Sign() < 0 {
		q.Sub(q, big.NewInt(1))
	}
	return q
}

// ceilRat is mathematical ceiling; QuoRem truncates toward zero.
func ceilRat(r *big.Rat) *big.Int {
	q, rem := new(big.Int).QuoRem(new(big.Int).Set(r.Num()), r.Denom(), new(big.Int))
	if rem.Sign() != 0 && r.Sign() > 0 {
		q.Add(q, big.NewInt(1))
	}
	return q
}

// roundHalfAwayRat rounds halves away from zero.
func roundHalfAwayRat(r *big.Rat) *big.Int {
	abs := new(big.Rat).Abs(r)
	q, rem := new(big.Int).QuoRem(new(big.Int).Set(abs.Num()), abs.Denom(), new(big.Int))
	if new(big.Int).Lsh(rem, 1).Cmp(abs.Denom()) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	if r.Sign() < 0 {
		q.Neg(q)
	}
	return q
}

// roundedMagnitude applies nearest-step rounding to the number that will be shown.
func roundedMagnitude(n *big.Rat, r Evaluation, percent bool) *big.Rat {
	if n == nil {
		return nil
	}
	m := n
	if percent {
		m = new(big.Rat).Mul(n, big.NewRat(100, 1))
	}
	if r.Nearest != nil {
		m = roundToNearest(m, r.Nearest, r.RoundDir)
	}
	return m
}

// formatMagnitude keeps requested decimal places, including trailing zeros.
func formatMagnitude(n *big.Rat, decimals *int) string {
	if n == nil {
		return ""
	}
	if decimals == nil {
		return plain(n)
	}
	s := n.FloatString(*decimals)
	if strings.HasPrefix(s, "-") && strings.Trim(s[1:], "0.") == "" {
		return s[1:]
	}
	return s
}

func durationScale(symbol string) *big.Rat {
	switch symbol {
	case "w":
		return big.NewRat(604800, 1)
	case "d":
		return big.NewRat(86400, 1)
	case "h":
		return big.NewRat(3600, 1)
	case "min":
		return big.NewRat(60, 1)
	case "ms":
		return big.NewRat(1, 1000)
	default:
		return big.NewRat(1, 1)
	}
}

func durationLabel(symbol string, n *big.Rat, longMinSec bool) string {
	one := n.Cmp(big.NewRat(1, 1)) == 0
	switch symbol {
	case "w":
		if one {
			return "week"
		}
		return "weeks"
	case "d":
		if one {
			return "day"
		}
		return "days"
	case "h":
		if one {
			return "hour"
		}
		return "hours"
	case "min":
		if longMinSec {
			if one {
				return "minute"
			}
			return "minutes"
		}
		return "min"
	case "ms":
		return "ms"
	default:
		if longMinSec {
			if one {
				return "second"
			}
			return "seconds"
		}
		return "s"
	}
}

// formatDurationParts splits seconds into the requested units, largest first.
func formatDurationParts(seconds *big.Rat, units []string, o FormatOptions, longMinSec bool) string {
	if len(units) == 0 {
		units = []string{"w", "d", "h", "min", "s"}
	}
	negative := seconds.Sign() < 0
	remain := new(big.Rat).Abs(seconds)
	var parts []string
	for i, symbol := range units {
		scale := durationScale(symbol)
		if i == len(units)-1 {
			amount := new(big.Rat).Quo(remain, scale)
			if amount.Sign() == 0 && len(parts) > 0 {
				break
			}
			parts = append(parts, formatNumber(plain(amount), o)+" "+durationLabel(symbol, amount, longMinSec))
			break
		}
		quot := new(big.Rat).Quo(remain, scale)
		if !quot.IsInt() {
			quot.SetInt(new(big.Int).Quo(quot.Num(), quot.Denom()))
		}
		if quot.Sign() > 0 {
			parts = append(parts, formatNumber(plain(quot), o)+" "+durationLabel(symbol, quot, longMinSec))
			remain.Sub(remain, new(big.Rat).Mul(quot, scale))
		}
	}
	if len(parts) == 0 {
		parts = append(parts, "0 "+durationLabel(units[len(units)-1], big.NewRat(0, 1), longMinSec))
	}
	text := strings.Join(parts, " ")
	if negative {
		return "-" + text
	}
	return text
}

// formatLaptime renders a duration as HH:MM:SS, the Soulver laptime form.
func formatLaptime(seconds *big.Rat) string {
	negative := seconds.Sign() < 0
	total := new(big.Rat).Abs(seconds)
	whole := new(big.Int).Quo(total.Num(), total.Denom())
	frac := new(big.Rat).Sub(total, new(big.Rat).SetInt(whole))
	h := new(big.Int).Quo(whole, big.NewInt(3600))
	rem := new(big.Int).Mod(whole, big.NewInt(3600))
	m := new(big.Int).Quo(rem, big.NewInt(60))
	s := new(big.Int).Mod(rem, big.NewInt(60))
	text := fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	if frac.Sign() != 0 {
		text += strings.TrimPrefix(plain(frac), "0")
	}
	if negative {
		return "-" + text
	}
	return text
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
	if r.Target == "timespan" || r.Target == "laptime" || r.Target == "parts" {
		f, _ := c.factor(v.Unit, Env{})
		seconds := new(big.Rat).Mul(v.Number, f)
		p.Raw = plain(seconds) + " s"
		if r.Target == "laptime" {
			p.Formatted = formatLaptime(seconds)
			p.Raw = p.Formatted
			return p
		}
		units := r.FormatUnits
		if r.Target == "timespan" {
			units = []string{"w", "d", "h", "min", "s"}
		}
		// Soulver timespan uses "minutes"/"seconds" once hours or larger appear;
		// requested unit lists keep the compact min/s forms.
		longMinSec := r.Target == "timespan" && seconds.Cmp(big.NewRat(3600, 1)) >= 0
		p.Formatted = formatDurationParts(seconds, units, o, longMinSec)
		return p
	}
	magnitude := roundedMagnitude(v.Number, r, v.Kind == Percent)
	s := formatMagnitude(magnitude, r.Decimals)
	p.Raw = s
	if v.Kind == Percent {
		if r.Decimals == nil && r.Nearest == nil {
			p.Raw = plain(v.Number)
		}
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
				explicit := r.Decimals != nil || r.Nearest != nil
				if !explicit {
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
				}
				if d.Dimension == "time" {
					key := map[string]string{"ms": "milliseconds", "s": "seconds", "min": "minutes", "h": "hours", "d": "days", "w": "weeks", "y": "years", "workday": "workdays"}[name]
					unit = translate("plugin_converter_time_unit_"+key, d.Plural)
					if !explicit {
						if name == "w" && !v.Number.IsInt() {
							scaled := new(big.Rat).Mul(v.Number, big.NewRat(1000, 1))
							s = new(big.Rat).SetFrac(new(big.Int).Quo(scaled.Num(), scaled.Denom()), big.NewInt(1000)).FloatString(3)
						} else if !v.Number.IsInt() {
							s = v.Number.FloatString(2)
						}
					}
				} else if d.Dimension != "storage" && d.Dimension != "money" {
					unit = d.Plural
					if new(big.Rat).Abs(magnitude).Cmp(big.NewRat(1, 1)) == 0 {
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
