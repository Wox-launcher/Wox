package engine

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
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

func formatMixedFraction(m, step *big.Rat) string {
	if m == nil {
		return ""
	}
	if step != nil && step.Num().Cmp(big.NewInt(1)) == 0 && step.Denom().Sign() > 0 {
		den := step.Denom().Int64()
		scaled := new(big.Rat).Mul(m, new(big.Rat).SetInt(step.Denom()))
		if scaled.IsInt() {
			num := scaled.Num().Int64()
			sign := ""
			if num < 0 {
				sign = "-"
				num = -num
			}
			whole := num / den
			rem := num % den
			if rem == 0 {
				return sign + strconv.FormatInt(whole, 10)
			}
			if whole == 0 {
				return sign + strconv.FormatInt(rem, 10) + "/" + strconv.FormatInt(den, 10)
			}
			return sign + strconv.FormatInt(whole, 10) + " " + strconv.FormatInt(rem, 10) + "/" + strconv.FormatInt(den, 10)
		}
	}
	return m.RatString()
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

func formatCivilSpan(months, days int) string {
	if months < 0 {
		months = -months
	}
	if days < 0 {
		days = -days
	}
	years := months / 12
	months = months % 12
	weeks := days / 7
	days = days % 7
	var parts []string
	if years != 0 {
		unit := "years"
		if years == 1 {
			unit = "year"
		}
		parts = append(parts, fmt.Sprintf("%d %s", years, unit))
	}
	if months != 0 {
		unit := "months"
		if months == 1 {
			unit = "month"
		}
		parts = append(parts, fmt.Sprintf("%d %s", months, unit))
	}
	if weeks != 0 {
		unit := "weeks"
		if weeks == 1 {
			unit = "week"
		}
		parts = append(parts, fmt.Sprintf("%d %s", weeks, unit))
	}
	if days != 0 || len(parts) == 0 {
		unit := "days"
		if days == 1 {
			unit = "day"
		}
		parts = append(parts, fmt.Sprintf("%d %s", days, unit))
	}
	return strings.Join(parts, " ")
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
			if symbol == "s" && !amount.IsInt() && len(parts) > 0 {
				f, _ := amount.Float64()
				amount = big.NewRat(int64(math.Round(f)), 1)
			}
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
	case Text:
		p.Raw = v.Text
		p.Formatted = v.Text
		if r.Target == "base64" {
			p.SubTitle = translate("plugin_converter_base64_encode", "Base64")
		} else {
			p.SubTitle = translate("plugin_converter_base64_decode", "Base64")
		}
		return p
	case Boolean:
		if v.Number.Sign() != 0 {
			p.Raw = "true"
		} else {
			p.Raw = "false"
		}
		p.Formatted = p.Raw
		return p
	case CalendarSpan:
		p.Raw = fmt.Sprintf("%d months %d days", v.Months, v.Days)
		p.Formatted = formatCivilSpan(v.Months, v.Days)
		return p
	case Date:
		p.Raw = v.Time.Format("2006-01-02")
		if r.Target == "weekday" {
			p.Formatted = v.Time.Weekday().String()
			return p
		}
		if r.Target == "datepattern" && len(r.FormatUnits) > 0 {
			p.Formatted = v.Time.Format(goDateLayout(r.FormatUnits[0]))
			return p
		}
		p.Formatted = fmt.Sprintf(translate("plugin_converter_weekday_date_format", "%s, %s"), weekday(v.Time), p.Raw)
		return p
	case Clock:
		p.Raw = v.Time.Format("15:04")
		p.Formatted = formatClock12(v.Time)
		if v.Days == -1 {
			p.Formatted = "Yesterday at " + p.Formatted
			p.Raw = p.Raw + " (-1 d)"
		} else if v.Days == 1 {
			p.Formatted = "Tomorrow at " + p.Formatted
			p.Raw = p.Raw + " (+1 d)"
		} else if v.Days != 0 {
			p.Formatted += fmt.Sprintf(" (%+d d)", v.Days)
			p.Raw = p.Formatted
		}
		return p
	case Instant:
		if r.Target == "iso8601" {
			p.Raw = v.Time.Format(time.RFC3339)
			p.Formatted = p.Raw
			return p
		}
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
	if (r.Target == "place" || r.Target == "gps") && len(r.FormatUnits) > 0 {
		p.Formatted = strings.Join(r.FormatUnits, "")
		p.Raw = p.Formatted
		return p
	}
	if r.Target == "timecode" {
		p.Formatted = formatTimecode(v.Number, r.FPS)
		p.Raw = p.Formatted
		return p
	}
	if r.Target == "dms" {
		p.Formatted = formatDMS(v.Number)
		p.Raw = p.Formatted
		return p
	}
	if r.Target == "pitch" {
		midi, _ := v.Number.Float64()
		p.Formatted = pitchName(midi)
		p.Raw = p.Formatted
		return p
	}
	if r.Target == "midi" {
		p.Formatted = plain(v.Number)
		p.Raw = p.Formatted
		return p
	}
	if r.Target == "pace" {
		p.Formatted = formatPace(v)
		p.Raw = p.Formatted
		return p
	}
	if r.Target == "sci" {
		f, _ := v.Number.Float64()
		p.Raw = strconv.FormatFloat(f, 'e', 6, 64)
		p.Formatted = p.Raw
		return p
	}
	if isBase(r.Target) {
		base := map[string]int{"bin": 2, "oct": 8, "dec": 10, "hex": 16}[r.Target]
		p.Raw = v.Number.Num().Text(base)
		if base == 16 {
			p.Raw = strings.ToUpper(p.Raw)
		}
		p.Formatted = p.Raw + " " + r.Target
		switch r.Target {
		case "hex":
			p.Formatted = "0x" + strings.ToUpper(v.Number.Num().Text(16))
		case "bin":
			p.Formatted = "0b" + v.Number.Num().Text(2)
		case "oct":
			p.Formatted = "0o" + v.Number.Num().Text(8)
		case "dec":
			p.Formatted = formatNumber(v.Number.Num().String(), o)
		}
		return p
	}
	if r.Target == "percent" {
		pct := new(big.Rat).Mul(v.Number, big.NewRat(100, 1))
		if v.Kind == Percent {
			pct = v.Number
			pct = new(big.Rat).Mul(v.Number, big.NewRat(100, 1))
		}
		p.Raw = plain(pct) + "%"
		p.Formatted = formatNumber(plain(pct), o) + "%"
		return p
	}
	if r.Target == "multiplier" || r.Target == "multiple" || r.Target == "x" {
		p.Raw = plain(v.Number) + "x"
		p.Formatted = p.Raw
		return p
	}
	if r.Target == "fraction" {
		m := roundedMagnitude(v.Number, r, false)
		p.Raw = m.RatString()
		p.Formatted = formatMixedFraction(m, r.Nearest)
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
		if c.dimensions(v.Unit)["money"] == 0 && c.dimensions(v.Unit)["time"] != -1 && c.dimensions(v.Unit)["frame"] == 0 && r.Substance == "" {
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
					case "money", "mass", "temperature", "volume", "pixel":
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
				if name == "lb" && !explicit {
					if parts, ok := formatPoundsOunces(v.Number); ok {
						p.Formatted = parts
						return p
					}
				}
				if name == "ft" && !explicit && r.Target == "ftin" {
					if parts, ok := formatFeetInches(v.Number); ok {
						p.Formatted = parts
						return p
					}
				}
				if d.Dimension == "time" {
					if name == "h" {
						if parts, ok := formatHourMinutes(v.Number); ok {
							p.Formatted = parts
							return p
						}
					}
					key := map[string]string{"ms": "milliseconds", "s": "seconds", "min": "minutes", "h": "hours", "d": "days", "w": "weeks", "y": "years", "workday": "workdays", "mo": "months", "ns": "nanoseconds", "us": "microseconds"}[name]
					unit = translate("plugin_converter_time_unit_"+key, d.Plural)
					if !explicit {
						if name == "w" && !v.Number.IsInt() {
							scaled := new(big.Rat).Mul(v.Number, big.NewRat(1000, 1))
							s = new(big.Rat).SetFrac(new(big.Int).Quo(scaled.Num(), scaled.Denom()), big.NewInt(1000)).FloatString(3)
						} else if !v.Number.IsInt() {
							if new(big.Rat).Abs(v.Number).Cmp(rational("0.01")) < 0 {
								s = plain(v.Number)
							} else {
								s = v.Number.FloatString(2)
							}
						}
					}
				} else if r.Substance != "" {
					unit = d.Symbol
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
				symbol := map[string]string{"USD": "$", "EUR": "€", "GBP": "£", "CNY": "¥", "JPY": "¥", "INR": "₹", "HKD": "HK$", "AUD": "A$", "CAD": "C$", "RUB": "₽", "NZD": "NZ$", "SGD": "S$", "TWD": "NT$", "BRL": "R$", "DKK": "kr"}[code]
				if symbol != "" {
					if r.Decimals == nil && r.Nearest == nil {
						s = strings.TrimRight(strings.TrimRight(v.Number.FloatString(2), "0"), ".")
						if s == "" || s == "-0" {
							s = "0"
						}
						if s == "0" && v.Number.Sign() != 0 {
							s = strings.TrimRight(strings.TrimRight(v.Number.FloatString(4), "0"), ".")
							if s == "" || s == "-0" {
								s = "0"
							}
						}
					}
					rest := copyUnit(v.Unit)
					delete(rest, code)
					p.Formatted = symbol + formatNumber(s, o)
					if len(rest) == 0 {
						if compact, ok := compactSI(v.Number, true); ok {
							p.Formatted = symbol + compact
						}
					} else {
						p.Formatted += rateSuffix(rest)
					}
					return p
				}
			}
		}
		if c.dimensions(v.Unit)["money"] == 0 {
			numName := ""
			den := false
			for k, n := range v.Unit {
				if c.Units[k].Dimension != "time" && k != "?m" && k != "mo" {
					continue
				}
				if n == 1 {
					numName = c.Units[k].Plural
					if new(big.Rat).Abs(magnitude).Cmp(big.NewRat(1, 1)) == 0 {
						numName = c.Units[k].Singular
					}
				}
				if n == -1 {
					den = true
				}
			}
			if numName != "" && den {
				p.Formatted = formatNumber(s, o) + " " + numName + rateSuffix(v.Unit)
				return p
			}
			if den && len(v.Unit) == 1 {
				p.Formatted = formatNumber(s, o) + rateSuffix(v.Unit)
				return p
			}
		}
		p.Formatted = formatNumber(s, o) + " " + unit
		return p
	}
	if compact, ok := compactSI(v.Number, false); ok && r.Target == "" {
		p.Formatted = compact
		if r.Ratio != "" {
			p.Formatted = r.Ratio
		}
		return p
	}
	p.Formatted = formatNumber(s, o)
	if r.Ratio != "" {
		p.Formatted = r.Ratio
	}
	return p
}

func compactSI(n *big.Rat, money bool) (string, bool) {
	if n == nil {
		return "", false
	}
	abs := new(big.Rat).Abs(n)
	if abs.Cmp(rational("100000")) < 0 {
		return "", false
	}
	type step struct {
		min, scale *big.Rat
		suffix     string
	}
	steps := []step{
		{rational("1000000000000"), rational("1000000000000"), "T"},
		{rational("1000000000"), rational("1000000000"), map[bool]string{true: "B", false: "G"}[money]},
		{rational("1000000"), rational("1000000"), "M"},
		{rational("100000"), rational("1000"), "k"},
	}
	sign := ""
	if n.Sign() < 0 {
		sign = "-"
	}
	for _, st := range steps {
		if abs.Cmp(st.min) >= 0 {
			q := new(big.Rat).Quo(abs, st.scale)
			full := q.FloatString(12)
			if i := strings.Index(full, "."); i >= 0 && strings.Trim(full[i+4:], "0") != "" {
				return "", false
			}
			return sign + strings.TrimRight(strings.TrimRight(q.FloatString(3), "0"), ".") + st.suffix, true
		}
	}
	return "", false
}

func rateSuffix(u Unit) string {
	names := map[string]string{"y": "year", "mo": "month", "month": "month", "w": "week", "d": "day", "h": "hour", "min": "minute", "s": "second", "workday": "workday"}
	for k, n := range u {
		if n == -1 {
			if name, ok := names[k]; ok {
				return "/" + name
			}
			return "/" + k
		}
	}
	return strings.TrimPrefix(unitText(u), "1")
}

func formatClock12(t time.Time) string {
	h, m := t.Hour(), t.Minute()
	suffix := "am"
	if h >= 12 {
		suffix = "pm"
	}
	h12 := h % 12
	if h12 == 0 {
		h12 = 12
	}
	return fmt.Sprintf("%d:%02d %s", h12, m, suffix)
}

// formatHourMinutes renders a non-integer hour quantity as "8 hours 35 min"
// when the minute remainder is exact, matching Soulver clock intervals.
func formatHourMinutes(n *big.Rat) (string, bool) {
	if n == nil || n.IsInt() {
		return "", false
	}
	mins := new(big.Rat).Mul(n, big.NewRat(60, 1))
	if !mins.IsInt() || !mins.Num().IsInt64() {
		return "", false
	}
	total := mins.Num().Int64()
	neg := ""
	if total < 0 {
		neg = "-"
		total = -total
	}
	h := total / 60
	m := total % 60
	if m == 0 {
		return "", false
	}
	return fmt.Sprintf("%s%d hours %d min", neg, h, m), true
}

func formatFeetInches(n *big.Rat) (string, bool) {
	if n == nil || n.Sign() <= 0 {
		return "", false
	}
	inches := new(big.Rat).Mul(n, big.NewRat(12, 1))
	if !inches.IsInt() || !inches.Num().IsInt64() {
		return "", false
	}
	total := inches.Num().Int64()
	ft := total / 12
	rem := total % 12
	if rem == 0 || ft == 0 {
		return "", false
	}
	return fmt.Sprintf("%d feet %d inches", ft, rem), true
}

func formatPoundsOunces(n *big.Rat) (string, bool) {
	if n == nil || n.Sign() <= 0 {
		return "", false
	}
	oz := new(big.Rat).Mul(n, big.NewRat(16, 1))
	if !oz.IsInt() || !oz.Num().IsInt64() {
		return "", false
	}
	total := oz.Num().Int64()
	lb := total / 16
	rem := total % 16
	if rem == 0 || lb == 0 {
		return "", false
	}
	return fmt.Sprintf("%d lb %d oz", lb, rem), true
}
