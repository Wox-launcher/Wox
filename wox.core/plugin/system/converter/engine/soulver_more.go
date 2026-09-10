package engine

import (
	"math"
	"math/big"
	"strconv"
	"time"
)

func (p *parser) parseVAT() (bool, error) {
	if p.peek() != "vat" && p.peek() != "gst" {
		saved := p.i
		if p.tokens[p.i].value == nil && p.tokens[p.i].text != "$" {
			return false, nil
		}
		left, err := p.expr(25)
		if err != nil {
			p.i = saved
			return false, nil
		}
		if p.accept("+") && (p.peek() == "vat" || p.peek() == "gst") {
			p.i++
			p.query.Domain = true
			p.query.root, err = p.make(&node{op: "vat_add", args: []*node{left}})
			return true, err
		}
		if p.accept("-") && (p.peek() == "vat" || p.peek() == "gst") {
			p.i++
			p.query.Domain = true
			p.query.root, err = p.make(&node{op: "vat_off", args: []*node{left}})
			return true, err
		}
		p.i = saved
		return false, nil
	}
	p.i++
	form := "on"
	if p.accept("off") {
		form = "off"
	} else if p.accept("in") || p.accept("of") || p.accept("from") {
		form = "in"
	} else {
		p.accept("on")
	}
	n, err := p.expr(0)
	if err != nil {
		return true, err
	}
	op := "vat_on"
	if form == "off" {
		op = "vat_off"
	} else if form == "in" {
		op = "vat_in"
	}
	p.query.Domain = true
	p.query.root, err = p.make(&node{op: op, args: []*node{n}})
	return true, err
}

func (p *parser) parseMortgage() (bool, error) {
	period := ""
	switch p.peek() {
	case "daily", "monthly", "annual", "total":
		period = p.peek()
		p.i++
	default:
		return false, nil
	}
	kind := "repayment"
	if p.accept("interest") {
		p.accept("paid")
		kind = "interest"
	} else if !p.accept("repayment") && !p.accept("payment") {
		p.i--
		return false, nil
	}
	p.accept("on")
	amount, err := p.expr(25)
	if err != nil {
		return true, err
	}
	if !p.accept("over") && !p.accept("for") {
		return true, invalid("expected over")
	}
	years, err := p.expr(25)
	if err != nil {
		return true, err
	}
	p.accept("years")
	p.accept("year")
	if !p.accept("at") && !p.accept("@") {
		return true, invalid("expected at")
	}
	rate, err := p.expr(25)
	if err != nil {
		return true, err
	}
	p.query.Domain = true
	p.query.root, err = p.make(&node{op: "mortgage", text: period + "_" + kind, args: []*node{amount, years, rate}})
	return true, err
}

func (p *parser) parseInvestmentRequired() (bool, error) {
	if p.peek() != "investment" && p.peek() != "deposit" {
		return false, nil
	}
	p.i++
	if !p.accept("required") && !p.accept("needed") {
		return true, invalid("expected required")
	}
	p.accept("for")
	amount, err := p.expr(25)
	if err != nil {
		return true, err
	}
	if !p.accept("at") && !p.accept("@") {
		return true, invalid("expected at")
	}
	rate, err := p.expr(25)
	if err != nil {
		return true, err
	}
	p.query.Domain = true
	p.query.root, err = p.make(&node{op: "annuity_required", args: []*node{amount, rate}})
	return true, err
}

func (p *parser) parsePresentValue() (bool, error) {
	if !p.accept("present") {
		return false, nil
	}
	if !p.accept("value") || !p.accept("of") {
		return true, invalid("expected value of")
	}
	fv, err := p.expr(25)
	if err != nil {
		return true, err
	}
	if !p.accept("after") {
		return true, invalid("expected after")
	}
	years, err := p.expr(25)
	if err != nil {
		return true, err
	}
	p.accept("years")
	p.accept("year")
	if !p.accept("at") && !p.accept("@") {
		return true, invalid("expected at")
	}
	rate, err := p.expr(25)
	if err != nil {
		return true, err
	}
	p.query.Domain = true
	p.query.root, err = p.make(&node{op: "present_value", args: []*node{fv, years, rate}})
	return true, err
}

func (p *parser) parseROI() (bool, error) {
	if p.accept("annual") {
		if !p.accept("return") || !p.accept("on") {
			return true, invalid("expected return on")
		}
		invested, err := p.expr(25)
		if err != nil {
			return true, err
		}
		if !p.accept("invested") {
			return true, invalid("expected invested")
		}
		returned, err := p.expr(25)
		if err != nil {
			return true, err
		}
		if !p.accept("returned") {
			return true, invalid("expected returned")
		}
		p.accept("after")
		years, err := p.expr(25)
		if err != nil {
			return true, err
		}
		p.accept("years")
		p.accept("year")
		p.query.Domain = true
		p.query.format = "percent"
		p.query.root, err = p.make(&node{op: "cagr", args: []*node{invested, returned, years}})
		return true, err
	}
	saved := p.i
	if p.tokens[p.i].text != "$" && p.tokens[p.i].value == nil {
		return false, nil
	}
	invested, err := p.expr(25)
	if err != nil || !p.accept("invested") {
		p.i = saved
		return false, nil
	}
	returned, err := p.expr(25)
	if err != nil {
		return true, err
	}
	if !p.accept("returned") {
		return true, invalid("expected returned")
	}
	p.query.Domain = true
	p.query.format = "multiplier"
	p.query.root, err = p.make(&node{op: "roi", args: []*node{invested, returned}})
	return true, err
}

func (p *parser) parsePercentRatio() (bool, error) {
	// 20% is 500, what is 750
	saved := p.i
	if p.tokens[p.i].value == nil && p.peek() != "$" {
		if !p.accept("if") {
			return false, nil
		}
		// if 20 is 30%, what is 60%
		a, err := p.expr(25)
		if err != nil || !p.accept("is") {
			p.i = saved
			return false, nil
		}
		pct, err := p.expr(25)
		if err != nil {
			return true, err
		}
		if !p.accept(",") {
			p.accept(",")
		}
		if !p.accept("what") || !p.accept("is") {
			return true, invalid("expected what is")
		}
		other, err := p.expr(0)
		if err != nil {
			return true, err
		}
		p.query.Domain = true
		p.query.root, err = p.make(&node{op: "pct_scale_amount", args: []*node{a, pct, other}})
		return true, err
	}
	pct, err := p.expr(25)
	if err != nil || !p.accept("is") {
		p.i = saved
		return false, nil
	}
	base, err := p.expr(25)
	if err != nil {
		p.i = saved
		return false, nil
	}
	p.accept(",")
	if !p.accept("what") || !p.accept("is") {
		p.i = saved
		return false, nil
	}
	other, err := p.expr(0)
	if err != nil {
		return true, err
	}
	p.query.Domain = true
	p.query.format = "percent"
	p.query.root, err = p.make(&node{op: "pct_scale", args: []*node{pct, base, other}})
	return true, err
}

func evalSoulverMore(n *node, args []Value, env Env) (Value, bool, error) {
	switch n.op {
	case "vat_add":
		args[0].Number.Mul(args[0].Number, big.NewRat(115, 100))
		return args[0], true, nil
	case "vat_on":
		args[0].Number.Mul(args[0].Number, big.NewRat(15, 100))
		return args[0], true, nil
	case "vat_off":
		args[0].Number.Quo(args[0].Number, big.NewRat(115, 100))
		return args[0], true, nil
	case "vat_in":
		gross := new(big.Rat).Set(args[0].Number)
		net := new(big.Rat).Quo(gross, big.NewRat(115, 100))
		args[0].Number.Sub(gross, net)
		return args[0], true, nil
	case "present_value":
		fv, _ := args[0].Number.Float64()
		years, _ := args[1].Number.Float64()
		r, _ := args[2].Number.Float64()
		if args[2].Kind != Percent {
			r = r / 100
		}
		num, err := ratFromFloat(fv / math.Pow(1+r, years))
		out := args[0]
		out.Number = num
		return out, true, err
	case "roi":
		if args[0].Number.Sign() == 0 {
			return Value{}, true, invalid("division by zero")
		}
		r := new(big.Rat).Quo(args[1].Number, args[0].Number)
		r.Sub(r, big.NewRat(1, 1))
		return Value{Kind: Number, Number: r}, true, nil
	case "cagr":
		start, _ := args[0].Number.Float64()
		end, _ := args[1].Number.Float64()
		years, _ := args[2].Number.Float64()
		if start <= 0 || years == 0 {
			return Value{}, true, invalid("invalid return")
		}
		num := ratPlaces(math.Pow(end/start, 1/years)-1, 4)
		return Value{Kind: Percent, Number: num}, true, nil
	case "mortgage":
		v, err := mortgageValue(args[0], args[1], args[2], n.text)
		return v, true, err
	case "days_between", "days_between_inclusive":
		if args[0].Kind != Date || args[1].Kind != Date {
			return Value{}, true, invalid("expected dates")
		}
		a := civilDay(args[0].Time)
		b := civilDay(args[1].Time)
		if b.Before(a) {
			a, b = b, a
		}
		if n.op == "days_between_inclusive" {
			b = b.AddDate(0, 0, 1)
		}
		days := (b.Unix() - a.Unix()) / 86400
		return Value{Kind: Quantity, Number: big.NewRat(days, 1), Unit: Unit{"d": 1}}, true, nil
	case "workdays_in":
		nwd := workdaysIn(args[0])
		return Value{Kind: Quantity, Number: big.NewRat(int64(nwd), 1), Unit: Unit{"workday": 1}}, true, nil
	case "workdays_between":
		if args[0].Kind != Date || args[1].Kind != Date {
			return Value{}, true, invalid("expected dates")
		}
		n := countWorkdays(args[0].Time, args[1].Time, false)
		return Value{Kind: Quantity, Number: big.NewRat(int64(n), 1), Unit: Unit{"workday": 1}}, true, nil
	case "period_percent":
		now := env.Now.In(env.Local)
		var part, whole float64
		switch n.text {
		case "year":
			days := 365.0
			if time.Date(now.Year(), 12, 31, 0, 0, 0, 0, time.UTC).YearDay() == 366 {
				days = 366
			}
			part, whole = float64(now.YearDay()), days
		case "month":
			part, whole = float64(now.Day()), float64(time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day())
		case "week":
			wd := int(now.Weekday())
			if wd == 0 {
				wd = 7
			}
			part, whole = float64(wd), 7
		case "day":
			part, whole = float64(now.Hour()*3600+now.Minute()*60+now.Second())/86400, 1
			return Value{Kind: Percent, Number: ratPlaces(part, 4)}, true, nil
		}
		return Value{Kind: Percent, Number: ratPlaces(part/whole, 4)}, true, nil
	case "weekday_on":
		if args[0].Kind != Date {
			return Value{}, true, invalid("expected a date")
		}
		return args[0], true, nil
	case "week_of_year":
		t := env.Now
		if len(args) > 0 && args[0].Kind == Date {
			t = args[0].Time
		}
		_, w := t.ISOWeek()
		return Value{Kind: Number, Number: big.NewRat(int64(w), 1)}, true, nil
	case "day_of_year":
		t := env.Now
		if len(args) > 0 && args[0].Kind == Date {
			t = args[0].Time
		}
		return Value{Kind: Number, Number: big.NewRat(int64(t.YearDay()), 1)}, true, nil
	case "day_of_month":
		t := env.Now
		if len(args) > 0 && args[0].Kind == Date {
			t = args[0].Time
		}
		return Value{Kind: Number, Number: big.NewRat(int64(t.Day()), 1)}, true, nil
	case "week_of_month":
		t := env.Now
		if len(args) > 0 && args[0].Kind == Date {
			t = args[0].Time
		}
		return Value{Kind: Number, Number: big.NewRat(int64(weekOfMonth(t)), 1)}, true, nil
	case "days_in_quarter", "workdays_in_quarter":
		q := ratInt(n.value)
		year := env.Now.Year()
		if n.text != "" && n.text != "0" {
			if y, err := strconv.Atoi(n.text); err == nil && y > 0 {
				year = y
			}
		}
		start := time.Date(year, time.Month((q-1)*3+1), 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 3, 0)
		if n.op == "workdays_in_quarter" {
			wd := countWorkdays(start, end.AddDate(0, 0, -1), true)
			return Value{Kind: Quantity, Number: big.NewRat(int64(wd), 1), Unit: Unit{"workday": 1}}, true, nil
		}
		days := (end.Unix() - start.Unix()) / 86400
		return Value{Kind: Quantity, Number: big.NewRat(days, 1), Unit: Unit{"d": 1}}, true, nil
	case "days_in_month":
		if args[0].Kind != Date {
			return Value{}, true, invalid("expected a month")
		}
		t := args[0].Time
		days := time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1).Day()
		return Value{Kind: Quantity, Number: big.NewRat(int64(days), 1), Unit: Unit{"d": 1}}, true, nil
	case "pct_scale":
		if args[1].Number.Sign() == 0 {
			return Value{}, true, invalid("division by zero")
		}
		r := new(big.Rat).Quo(args[2].Number, args[1].Number)
		r.Mul(r, args[0].Number)
		return Value{Kind: Percent, Number: r}, true, nil
	case "pct_scale_amount":
		if args[1].Number.Sign() == 0 {
			return Value{}, true, invalid("division by zero")
		}
		r := new(big.Rat).Quo(args[0].Number, args[1].Number)
		r.Mul(r, args[2].Number)
		out := args[0]
		out.Number = r
		return out, true, nil
	case "growth_rate":
		start, _ := args[0].Number.Float64()
		end, _ := args[1].Number.Float64()
		periods, _ := args[2].Number.Float64()
		if args[2].Kind == Quantity && args[2].Unit["w"] == 1 {
			periods *= 7
		}
		if start <= 0 || periods == 0 {
			return Value{}, true, invalid("invalid growth")
		}
		num := ratPlaces(math.Pow(end/start, 1/periods)-1, 2)
		return Value{Kind: Percent, Number: num}, true, nil
	}
	return Value{}, false, nil
}

func civilDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func mortgageValue(principal, years, rate Value, spec string) (Value, error) {
	p, _ := principal.Number.Float64()
	y, _ := years.Number.Float64()
	r, _ := rate.Number.Float64()
	if rate.Kind != Percent {
		r = r / 100
	}
	n := y * 12
	per := r / 12
	monthly := p * per * math.Pow(1+per, n) / (math.Pow(1+per, n) - 1)
	total := monthly * n
	interest := total - p
	out := principal
	var val float64
	switch {
	case stringsHasSuffix(spec, "interest") && stringsHasPrefix(spec, "total"):
		val = interest
	case stringsHasSuffix(spec, "interest") && stringsHasPrefix(spec, "daily"):
		val = interest / (y * 365)
	case stringsHasSuffix(spec, "interest") && stringsHasPrefix(spec, "monthly"):
		val = interest / n
	case stringsHasSuffix(spec, "interest") && stringsHasPrefix(spec, "annual"):
		val = interest / y
	case stringsHasPrefix(spec, "total"):
		val = total
	case stringsHasPrefix(spec, "daily"):
		val = monthly * 12 / 365
	case stringsHasPrefix(spec, "annual"):
		val = monthly * 12
	default:
		val = monthly
	}
	out.Number = ratPlaces(val, 2)
	return out, nil
}

func stringsHasPrefix(s, p string) bool {
	return len(s) >= len(p) && s[:len(p)] == p
}

func stringsHasSuffix(s, p string) bool {
	return len(s) >= len(p) && s[len(s)-len(p):] == p
}

func workdaysIn(v Value) int {
	if v.Kind == Quantity && v.Unit["w"] == 1 {
		n := ratInt(v)
		return int(n) * 5
	}
	if v.Kind == CalendarSpan {
		return v.Months*21 + (v.Days/7)*5
	}
	if v.Kind == Quantity && v.Unit["d"] == 1 {
		return int(ratInt(v) * 5 / 7)
	}
	return int(ratInt(v) * 5)
}
