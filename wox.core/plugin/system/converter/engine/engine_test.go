package engine

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"
)

func fixture() (*Catalog, Env) {
	c := NewCatalog()
	prices := map[string]*big.Rat{}
	for code, price := range map[string]string{"USD": "1", "EUR": "2", "GBP": "1.25", "JPY": "0.01", "CNY": "0.125", "INR": "0.0125", "HKD": "0.125", "BTC": "80000", "ETH": "3000", "USDT": "1", "BNB": "850"} {
		c.AddCurrency(code, code == "BTC" || code == "ETH" || code == "USDT" || code == "BNB")
		prices[code] = rational(price)
	}
	return c, Env{Now: time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC), Local: time.UTC, DefaultCurrency: "USD", Prices: prices}
}

// TestCompatibilityCorpus is executable documentation; rates and time never depend on network or wall time.
func TestCompatibilityCorpus(t *testing.T) {
	c, env := fixture()
	for _, tc := range []struct{ input, raw string }{
		{"10ft in m", "3.048 m"}, {"100 usd in gbp", "80 GBP"}, {"52% of 900", "468"},
		{"square root of 625", "25"}, {"cube root of 27", "3"}, {"2 power 10", "1024"}, {"4 power 6", "4096"}, {"60 + 74", "134"},
		{"workhours in 2023", "2080 h"}, {"55h in workdays", "6.875 workday"}, {"2 inches in px at 72 ppi", "144 px"},
		{"20% off 80", "64"}, {"15% tip on 42", "6.3"}, {"ratio of 3 to 5", "0.6"}, {"USD1K", "1000 USD"}, {"10K", "10000"},
		{"10 usd in gbp", "8 GBP"}, {"45 jpy to inr", "36 INR"}, {"5 btc in gbp", "320000 GBP"},
		{"23C to F", "73.4 °F"}, {"29 inches to cm", "73.66 cm"}, {"4 feet to cm", "121.92 cm"},
		{"3 teaspoon in ml", "14.78676478125 ml"}, {"32% of 5", "1.6"}, {"8 dollars/hour in gbp", "6.4 GBP/h"}, {"19m + 47%", "27.93 min"},
		{"(100 USD + 50 EUR) * 2 to CNY", "3200 CNY"}, {"(1h + 30min) / 2 to minutes", "45 min"},
		{"(2 inches + 1 inch) in px at 72 ppi", "216 px"}, {"8 dollars/hour * 5 hours in gbp", "32 GBP"},
		{"8 dollars/hour * 30 minutes in gbp", "3.2 GBP"}, {"12% of $321 in jpy", "3852 JPY"}, {"12% of (100 USD + 50 USD)", "18 USD"},
		{"1 USD + 2 USD * 3", "7 USD"}, {"-(1 USD + 2 USD)", "-3 USD"},
		{"1km / 500m", "2"}, {"2 meters * 3 meters", "6 m²"}, {"50 EUR = ? USD", "100 USD"},
		{"0xff to dec", "255"}, {"255 dec to hex", "FF"}, {"17 oct to dec", "15"}, {"FF hex to dec", "255"},
		{"32 bytes to gb", "0.000000032 GB"}, {"32bytes =? gb", "0.000000032 GB"}, {"1 GB to MiB", "953.67431640625 MiB"},
		{"1 gigabyte to gibibyte", "0.9313225746154785 GiB"}, {"32 b to bytes", "32 B"},
		{"32f to c", "0 °C"}, {"1h", "60 min"}, {"1 week", "7 d"}, {"10 days", "1.4285714285714286 w"},
		{"1/3 to 2 dp", "0.33"}, {"π to 5 digits", "3.14159"}, {"pi to 5 digits", "3.14159"},
		{"2/3 to 2 dp", "0.67"}, {"1 to 2 dp", "1.00"}, {"2/3 to 0 dp", "1"},
		{"21 rounded up to nearest 5", "25"}, {"17 rounded down to nearest 3", "15"},
		{"20 rounded up to nearest 5", "20"}, {"37 to nearest 10", "40"},
		{"meters in 10 km", "10000 m"}, {"days in 3 weeks", "21 d"}, {"seconds in a day", "86400 s"},
		{"300 + 20 km", "320 km"}, {"$20 + 30", "50 USD"}, {"20 km + 300", "320 km"}, {"30 + $20", "50 USD"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			q, e := c.Parse(tc.input, ParseOptions{DecimalSeparator: "."})
			if e != nil {
				t.Fatal(e)
			}
			r, e := c.Evaluate(context.Background(), q, env)
			if e != nil {
				t.Fatal(e)
			}
			p := c.Format(r, FormatOptions{})
			if p.Raw != tc.raw {
				t.Fatalf("raw %q, want %q (display %q)", p.Raw, tc.raw, p.Formatted)
			}
		})
	}
}

func TestTimeCorpus(t *testing.T) {
	c, env := fixture()
	for _, tc := range []struct{ input, raw string }{
		{"5pm ldn in sf", "2026-09-08T09:00:00-07:00"}, {"time in tokyo", "2026-09-08T21:00:00+09:00"},
		{"now in sf", "2026-09-08T05:00:00-07:00"}, {"NOW in America/Los_Angeles", "2026-09-08T05:00:00-07:00"},
		{"monday in 3 weeks", "2026-10-05"}, {"Monday in 3 days", "2026-09-14"}, {"days until 31 Mar", "204 d"},
		{"time diff Paris", "2 h"}, {"diff Paris", "2 h"}, {"time in 4 hours", "2026-09-08T16:00:00Z"},
		{"time in 4 hours in San Francisco", "2026-09-08T09:00:00-07:00"},
		{"2024-03-15T14:30:00Z", "2024-03-15T14:30:00Z"}, {"August 5 + 5", "2026-08-10"}, {"3:45pm + 5", "20:45"},
		{"time in São Paulo", "2026-09-08T09:00:00-03:00"}, {"time in JFK", "2026-09-08T08:00:00-04:00"},
		{"Time in Dubai", "2026-09-08T16:00:00+04:00"}, {"Days until 25 Dec", "108 d"}, {"35 days ago", "2026-08-04"},
		{"2026-09-08 2 am ist to cet", "2026-09-07T22:30:00+02:00"}, {"2 am ist to cet", "2026-09-07T22:30:00+02:00"},
		{"3:30 pm in tokyo", "2026-09-08T06:30:00Z"}, {"2 am in ist to cet", "2026-09-07T22:30:00+02:00"}, {"3pm in Tokyo", "2026-09-08T06:00:00Z"}, {"2026-01-08 2 am ist to cet", "2026-01-07T21:30:00+01:00"},
		{"3pm GMT+8 to Paris", "2026-09-08T09:00:00+02:00"},
		{"7:30am LAX to Japan", "2026-09-08T23:30:00+09:00"},
		{"Tokyo time", "2026-09-08T21:00:00+09:00"},
		{"time difference between Seattle and Moscow", "10 h"},
		{"difference between PDT & AEST", "17 h"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			q, e := c.Parse(tc.input, ParseOptions{})
			if e != nil {
				t.Fatal(e)
			}
			r, e := c.Evaluate(context.Background(), q, env)
			if e != nil {
				t.Fatal(e)
			}
			if got := c.Format(r, FormatOptions{}).Raw; got != tc.raw {
				t.Fatalf("got %q, want %q", got, tc.raw)
			}
		})
	}
}

func TestInvalidAndResourceLimits(t *testing.T) {
	c, env := fixture()
	for _, input := range []string{"(1 USD + 2 USD", "1 USD +", "1 USD / 0", "1 USD + 2 meters", "1km/h in m", "2 meters^2 in ft", "2^100000", "(2^4096)^4096", "sqrt(-1)", "sin(1;2)", "1USD to EUR junk", "2026-03-29 02:30 Europe/Berlin to UTC", "2026-10-25 02:30 Europe/Berlin to UTC", "2026-02-30 2 am ist to cet", "time in nowhere-invalid", "1/3 to 65 dp", "now to 2 dp", "21 rounded up", "21 rounded up to nearest 0", "now rounded up to nearest 5", "meters in 10 kg", "difference between Seattle", "time difference between nowhere and Paris", strings.Repeat("(", 65) + "1" + strings.Repeat(")", 65), strings.Repeat("9", 4097)} {
		t.Run(input[:min(len(input), 80)], func(t *testing.T) {
			q, e := c.Parse(input, ParseOptions{})
			if e == nil {
				_, e = c.Evaluate(context.Background(), q, env)
			}
			if e == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestLocaleAndPresentation(t *testing.T) {
	c, env := fixture()
	q, e := c.Parse("1.000,50 EUR in USD", ParseOptions{ThousandsSeparator: ".", DecimalSeparator: ","})
	if e != nil {
		t.Fatal(e)
	}
	r, e := c.Evaluate(context.Background(), q, env)
	if e != nil {
		t.Fatal(e)
	}
	p := c.Format(r, FormatOptions{ThousandsSeparator: ".", DecimalSeparator: ","})
	if p.Raw != "2001 USD" || p.Formatted != "$2.001" {
		t.Fatalf("%+v", p)
	}
	q, e = c.Parse("145 mins to timespan", ParseOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e = c.Evaluate(context.Background(), q, env)
	if e != nil {
		t.Fatal(e)
	}
	if got := c.Format(r, FormatOptions{}).Formatted; got != "2 hours 25 minutes" {
		t.Fatal(got)
	}
	for _, input := range []string{"cot(1)", "csc(1)", "sinh(1)", "acos(0)"} {
		q, e = c.Parse(input, ParseOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = c.Evaluate(context.Background(), q, env); e != nil {
			t.Fatal(e)
		}
	}
}

func FuzzParseEvaluate(f *testing.F) {
	for _, s := range []string{"(1h + 30min) / 2", "2 am ist to cet", "12% of $321 in jpy", "1.000,50 EUR"} {
		f.Add(s)
	}
	c, env := fixture()
	f.Fuzz(func(t *testing.T, s string) {
		q, err := c.Parse(s, ParseOptions{})
		if err == nil {
			r, err := c.Evaluate(context.Background(), q, env)
			if err == nil {
				c.Format(r, FormatOptions{})
			}
		}
	})
}

// TestTypeMatrix exercises allowed temporal operations and explicit dimensional rejection.
func TestTypeMatrix(t *testing.T) {
	c, env := fixture()
	for _, tc := range []struct{ input, raw string }{
		{"August 5 - August 1", "4 d"},
		{"2026-09-08T12:00:00Z - 2026-09-08T11:59:59Z", "1 s"},
		{"2026-09-08T12:00:00Z + 1h", "2026-09-08T13:00:00Z"},
		{"01:00 - 23:00", "-22 h"}, {"(11pm + 2h)", "01:00 (+1 d)"},
		{"(August 5 + 2 months)", "2026-10-05"}, {"(2 months + 3 months) * 2", "10 months 0 days"},
		{"2026-09-08T12:00:00Z + 2 months", "2026-11-08T12:00:00Z"},
		{"2026-09-08T12:00:00.123Z + 2 months", "2026-11-08T12:00:00.123Z"},
		{"32f - 0c", "0 deltaF"}, {"32f + 5c", "41 °F"}, {"5 deltaC to deltaF", "9 deltaF"},
		{"10% + 20%", "0.3"}, {"10% * 20%", "0.02"}, {"100 USD / 50 EUR", "1"},
		{"1km/h in m/s", "0.2777777777777778 m/s"}, {"10 in to cm", "25.4 cm"},
		{"sqrt(4 meters^2)", "2 m"}, {"cube root of 8 meters^3", "2 m"}, {"0.1 USD + 0.2 USD", "0.3 USD"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			q, e := c.Parse(tc.input, ParseOptions{})
			if e != nil {
				t.Fatal(e)
			}
			r, e := c.Evaluate(context.Background(), q, env)
			if e != nil {
				t.Fatal(e)
			}
			if p := c.Format(r, FormatOptions{}); p.Raw != tc.raw {
				t.Fatalf("got %q want %q", p.Raw, tc.raw)
			}
		})
	}
	for _, input := range []string{"2026-09-08T12:00:00Z + 2", "August 5 * 2", "August 5 + August 1", "32f * 2", "32f + 10%", "2 months / 2", "-(August 5)", "1h + 1km + 1m"} {
		q, e := c.Parse(input, ParseOptions{})
		if e == nil {
			_, e = c.Evaluate(context.Background(), q, env)
		}
		if e == nil {
			t.Errorf("expected rejection: %s", input)
		}
	}
}

func TestFractionalTimespanAndErrorKinds(t *testing.T) {
	c, env := fixture()
	q, err := c.Parse("500ms to timespan", ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := c.Evaluate(context.Background(), q, env)
	if err != nil {
		t.Fatal(err)
	}
	if p := c.Format(r, FormatOptions{}); p.Formatted != "0.5 s" || p.Raw != "0.5 s" {
		t.Fatalf("fractional duration lost: %+v", p)
	}
	q, err = c.Parse("sqrt(-1)", ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Evaluate(context.Background(), q, env)
	if e, ok := err.(*Error); !ok || e.Kind != Invalid {
		t.Fatalf("untyped domain error: %v", err)
	}
}

// TestInstantExpressions fixes the clock and zone to verify elapsed arithmetic.
func TestInstantExpressions(t *testing.T) {
	c, env := fixture()
	env.Local = time.FixedZone("test", 8*3600)
	env.Now = time.Date(2026, 6, 10, 16, 0, 0, 0, env.Local)
	for _, tc := range []struct{ input, raw string }{
		{"now + 4d17h", "2026-06-15T09:00:00+08:00"},
		{"2026-06-14 16:00 - now", "345600 s"},
		{"now - 4d17h", "2026-06-05T23:00:00+08:00"},
		{"NOW - now", "0 s"},
		{"now + (4d17h * 2)", "2026-06-20T02:00:00+08:00"},
		{"(2026-06-14 16:00 - now) to hours", "96 h"},
		{"now + 1h30m15s", "2026-06-10T17:30:15+08:00"},
		{"now + 4d 17h", "2026-06-15T09:00:00+08:00"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			q, err := c.Parse(tc.input, ParseOptions{DecimalSeparator: "."})
			if err != nil {
				t.Fatal(err)
			}
			r, err := c.Evaluate(context.Background(), q, env)
			if err != nil {
				t.Fatal(err)
			}
			if got := c.Format(r, FormatOptions{}).Raw; got != tc.raw {
				t.Fatalf("got %q, want %q", got, tc.raw)
			}
		})
	}
	env.Local, _ = time.LoadLocation("Europe/Berlin")
	for _, input := range []string{"now + 4d17kg", "now + 4d17", "2026-02-30 16:00 - now", "2026-03-29 02:30 - now", "2026-10-25 02:30 - now"} {
		q, err := c.Parse(input, ParseOptions{DecimalSeparator: "."})
		if err == nil {
			_, err = c.Evaluate(context.Background(), q, env)
		}
		if err == nil {
			t.Errorf("accepted invalid expression %q", input)
		}
	}
}

func TestTimespanLaptimeAndSpeed(t *testing.T) {
	c, env := fixture()
	for _, tc := range []struct{ input, formatted string }{
		{"5.5 minutes as timespan", "5 min 30 s"},
		{"4.54 hours as timespan", "4 hours 32 minutes 24 seconds"},
		{"72 days as timespan", "10 weeks 2 days"},
		{"5.5 minutes as laptime", "00:05:30"},
		{"03:04:05 + 01:02:03", "04:06:08"},
		{"00:12:05 - 00:04:09", "00:07:56"},
		{"00:12:05 − 00:04:09", "00:07:56"},
		{"03:04:05 as timespan", "3 hours 4 minutes 5 seconds"},
		{"3 hours 4 minutes 5 seconds as laptime", "03:04:05"},
		{"12.5 minutes in minutes and seconds", "12 min 30 s"},
		{"1.4 weeks in hours and minutes", "235 hours 12 min"},
		{"4.5 weeks in days and hours", "31 days 12 hours"},
		{"1 hour 30 minutes at 1.5x", "1 hour"},
		{"time saved 5 min at 1.5x", "1 min 40 s"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			q, err := c.Parse(tc.input, ParseOptions{DecimalSeparator: "."})
			if err != nil {
				t.Fatal(err)
			}
			r, err := c.Evaluate(context.Background(), q, env)
			if err != nil {
				t.Fatal(err)
			}
			if got := c.Format(r, FormatOptions{}).Formatted; got != tc.formatted {
				t.Fatalf("got %q, want %q", got, tc.formatted)
			}
		})
	}
}
