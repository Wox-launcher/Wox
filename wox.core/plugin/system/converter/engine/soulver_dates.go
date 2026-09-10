package engine

import (
	"math/big"
	"strconv"
	"strings"
	"time"
)

// mergeCalendarSpan combines calendar months with day/week/year quantities.
func mergeCalendarSpan(a, b Value, op string) (Value, bool, error) {
	sign := 1
	if op == "-" {
		sign = -1
	}
	if op == "*" {
		if a.Kind == CalendarSpan && b.Kind == Number && b.Number.IsInt() && b.Number.Num().IsInt64() {
			n := b.Number.Num().Int64()
			if n < -120000 || n > 120000 {
				return Value{}, true, invalid("calendar multiplier exceeds limit")
			}
			a.Months *= int(n)
			a.Days *= int(n)
			v, err := boundedCalendar(a)
			return v, true, err
		}
		return Value{}, false, nil
	}
	if op != "+" && op != "-" {
		return Value{}, false, nil
	}
	span := func(v Value) (months, days int, ok bool) {
		switch v.Kind {
		case CalendarSpan:
			return v.Months, v.Days, true
		case Quantity:
			if len(v.Unit) != 1 || v.Number == nil || !v.Number.IsInt() || !v.Number.Num().IsInt64() {
				return 0, 0, false
			}
			n := int(v.Number.Num().Int64())
			switch {
			case v.Unit["d"] == 1:
				return 0, n, true
			case v.Unit["w"] == 1:
				return 0, n * 7, true
			case v.Unit["y"] == 1:
				return n * 12, 0, true
			case v.Unit["month"] == 1:
				return n, 0, true
			}
		}
		return 0, 0, false
	}
	am, ad, aok := span(a)
	bm, bd, bok := span(b)
	if !aok || !bok {
		return Value{}, false, nil
	}
	v, err := boundedCalendar(Value{Kind: CalendarSpan, Months: am + sign*bm, Days: ad + sign*bd})
	return v, true, err
}

// civilInterval is the signed-away span Soulver shows between two calendar dates.
func civilInterval(a, b time.Time, inclusive bool) (Value, error) {
	if b.Before(a) {
		a, b = b, a
	}
	a = time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, time.UTC)
	b = time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, time.UTC)
	if inclusive {
		b = b.AddDate(0, 0, 1)
	}
	years := b.Year() - a.Year()
	months := int(b.Month()) - int(a.Month())
	days := b.Day() - a.Day()
	if days < 0 {
		months--
		prev := time.Date(b.Year(), b.Month(), 0, 0, 0, 0, 0, time.UTC)
		days += prev.Day()
	}
	if months < 0 {
		years--
		months += 12
	}
	return boundedCalendar(Value{Kind: CalendarSpan, Months: years*12 + months, Days: days})
}

func isWeekend(t time.Time) bool {
	return t.Weekday() == time.Saturday || t.Weekday() == time.Sunday
}

func isPublicHoliday(t time.Time) bool {
	if t.Month() == time.January && t.Day() == 1 {
		return true
	}
	if t.Month() == time.December && (t.Day() == 25 || t.Day() == 26) {
		return true
	}
	if t.Month() == time.July && t.Day() == 4 {
		return true
	}
	if t.Month() == time.November && t.Weekday() == time.Thursday {
		day := t.Day()
		if day >= 22 && day <= 28 {
			return true
		}
	}
	return false
}

func isWorkday(t time.Time) bool {
	return !isWeekend(t) && !isPublicHoliday(t)
}

// addWorkdays walks civil days, skipping weekends and the common holidays Soulver accounts for.
func addWorkdays(start time.Time, n int) time.Time {
	t := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	step := 1
	if n < 0 {
		step = -1
		n = -n
	}
	for n > 0 {
		t = t.AddDate(0, 0, step)
		if isWorkday(t) {
			n--
		}
	}
	return t
}

func countWorkdays(start, end time.Time, inclusive bool) int {
	a := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	b := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	if b.Before(a) {
		a, b = b, a
	}
	if !inclusive {
		b = b.AddDate(0, 0, -1)
	}
	n := 0
	for t := a; !t.After(b); t = t.AddDate(0, 0, 1) {
		if isWorkday(t) {
			n++
		}
	}
	return n
}

func (p *parser) parseDatePhrases() (bool, error) {
	if ok, err := p.parseTimeSinceTo(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseDaysBetween(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseWorkdayPhrases(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseWeekdayOn(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseWeekNumber(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseDayParts(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseDaysInPeriod(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseAfterBefore(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseFromNow(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseYearRange(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseThroughDays(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseDateToWorkdays(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parsePeriodPercent(); ok || err != nil {
		return ok, err
	}
	return false, nil
}

// parsePeriodPercent handles year/month/week/day percentage of the current period.
func (p *parser) parsePeriodPercent() (bool, error) {
	period := p.peek()
	if period != "year" && period != "month" && period != "week" && period != "day" {
		return false, nil
	}
	if p.i+1 >= len(p.tokens) || (p.tokens[p.i+1].text != "percentage" && p.tokens[p.i+1].text != "percent" && p.tokens[p.i+1].text != "%") {
		return false, nil
	}
	p.i += 2
	p.query.Domain = true
	p.query.format = "percent"
	p.query.root = &node{op: "period_percent", text: period}
	return true, nil
}

func (p *parser) parseDateToWorkdays() (bool, error) {
	saved := p.i
	if p.tokens[p.i].temporal == nil && !isMonthWord(p.peek()) {
		return false, nil
	}
	a, err := p.expr(25)
	if err != nil || !p.accept("to") {
		p.i = saved
		return false, nil
	}
	b, err := p.expr(25)
	if err != nil || !p.accept("in") {
		p.i = saved
		return false, nil
	}
	if !p.accept("workdays") && !p.accept("workday") {
		p.i = saved
		return false, nil
	}
	p.query.Domain = true
	p.query.root, err = p.make(&node{op: "workdays_between", args: []*node{a, b}})
	return true, err
}

func (p *parser) parseTimeSinceTo() (bool, error) {
	if p.peek() != "time" || p.i+1 >= len(p.tokens) {
		return false, nil
	}
	kind := p.tokens[p.i+1].text
	if kind != "since" && kind != "to" {
		return false, nil
	}
	if p.i+2 >= len(p.tokens) {
		return false, nil
	}
	saved := p.i
	p.i += 2
	if !p.nextIsDatePhrase() {
		p.i = saved
		return false, nil
	}
	d, err := p.expr(0)
	if err != nil {
		return true, err
	}
	today, err := p.make(&node{op: "temporal", temporal: &temporalQuery{kind: "relative", date: "today"}})
	if err != nil {
		return true, err
	}
	p.query.Domain = true
	if kind == "since" {
		p.query.root, err = p.make(&node{op: "clock_to", args: []*node{d, today}})
	} else {
		p.query.root, err = p.make(&node{op: "clock_to", args: []*node{today, d}})
	}
	return true, err
}

func (p *parser) nextIsDatePhrase() bool {
	if p.i >= len(p.tokens) {
		return false
	}
	t := p.tokens[p.i]
	if t.temporal != nil {
		switch t.temporal.kind {
		case "dateLiteral", "localLiteral", "iso", "relative":
			return true
		}
	}
	switch t.text {
	case "today", "yesterday", "tomorrow":
		return true
	}
	return false
}

func (p *parser) parseDaysBetween() (bool, error) {
	if !p.accept("days") && !p.accept("day") {
		return false, nil
	}
	saved := p.i - 1
	if p.accept("between") || p.accept("from") {
		a, err := p.expr(25)
		if err != nil {
			return true, err
		}
		if !p.accept("and") && !p.accept("to") {
			return true, invalid("expected and")
		}
		b, err := p.expr(25)
		if err != nil {
			return true, err
		}
		p.query.Domain = true
		p.query.format = "days"
		p.query.root, err = p.make(&node{op: "days_between", args: []*node{a, b}})
		return true, err
	}
	if p.accept("to") || p.accept("till") || p.accept("until") {
		b, err := p.expr(0)
		if err != nil {
			return true, err
		}
		today, err := p.make(&node{op: "temporal", temporal: &temporalQuery{kind: "relative", date: "today"}})
		if err != nil {
			return true, err
		}
		p.query.Domain = true
		p.query.format = "days"
		p.query.root, err = p.make(&node{op: "days_between", args: []*node{today, b}})
		return true, err
	}
	if p.accept("in") {
		a, err := p.expr(25)
		if err == nil && p.accept("to") {
			b, err := p.expr(25)
			if err != nil {
				return true, err
			}
			p.query.Domain = true
			p.query.format = "days"
			p.query.root, err = p.make(&node{op: "days_between", args: []*node{a, b}})
			return true, err
		}
	}
	p.i = saved + 1
	if p.accept("left") && p.accept("in") {
		t := p.tokens[p.i]
		if t.value == nil || !t.value.IsInt() {
			return true, invalid("expected a year")
		}
		p.i++
		year := t.value.Num().Int64()
		end, err := p.make(&node{op: "value", value: Value{Kind: Date, Time: time.Date(int(year), 12, 31, 12, 0, 0, 0, time.UTC)}})
		if err != nil {
			return true, err
		}
		today, err := p.make(&node{op: "temporal", temporal: &temporalQuery{kind: "relative", date: "today"}})
		if err != nil {
			return true, err
		}
		p.query.Domain = true
		p.query.format = "days"
		p.query.root, err = p.make(&node{op: "days_between", args: []*node{today, end}})
		return true, err
	}
	p.i = saved
	return false, nil
}

func (p *parser) parseWorkdayPhrases() (bool, error) {
	if p.peek() == "working" || p.peek() == "business" {
		if p.i+1 >= len(p.tokens) || (p.tokens[p.i+1].text != "days" && p.tokens[p.i+1].text != "day") {
			return false, nil
		}
		p.i += 2
	} else if p.peek() == "workdays" || p.peek() == "workday" {
		p.i++
	} else {
		return false, nil
	}
	if p.accept("in") {
		if p.peek() == "q" && p.i+1 < len(p.tokens) && p.tokens[p.i+1].value != nil {
			qtr := ratInt(Value{Number: p.tokens[p.i+1].value})
			if qtr < 1 || qtr > 4 {
				return true, invalid("invalid quarter")
			}
			p.i += 2
			p.accept(",")
			year := 0
			if p.tokens[p.i].value != nil {
				year = int(ratInt(Value{Number: p.tokens[p.i].value}))
				p.i++
			}
			p.query.Domain = true
			n, err := p.make(&node{op: "workdays_in_quarter", text: strconv.Itoa(year), value: Value{Kind: Number, Number: big.NewRat(int64(qtr), 1)}})
			p.query.root = n
			return true, err
		}
		n, err := p.expr(0)
		if err != nil {
			return true, err
		}
		p.query.Domain = true
		p.query.root, err = p.make(&node{op: "workdays_in", args: []*node{n}})
		return true, err
	}
	if p.accept("from") {
		a, err := p.expr(25)
		if err != nil {
			return true, err
		}
		if !p.accept("to") {
			return true, invalid("expected to")
		}
		b, err := p.expr(25)
		if err != nil {
			return true, err
		}
		p.query.Domain = true
		p.query.root, err = p.make(&node{op: "workdays_between", args: []*node{a, b}})
		return true, err
	}
	return true, invalid("expected in or from")
}

func (p *parser) parseWeekdayOn() (bool, error) {
	if p.accept("weekday") {
		if !p.accept("on") {
			return true, invalid("expected on")
		}
		d, err := p.expr(0)
		if err != nil {
			return true, err
		}
		p.query.Domain = true
		p.query.format = "weekday"
		p.query.root, err = p.make(&node{op: "weekday_on", args: []*node{d}})
		return true, err
	}
	if p.peek() != "day" || p.i+3 >= len(p.tokens) || p.tokens[p.i+1].text != "of" || p.tokens[p.i+2].text != "the" || p.tokens[p.i+3].text != "week" {
		return false, nil
	}
	p.i += 4
	p.accept("on")
	d, err := p.expr(0)
	if err != nil {
		return true, err
	}
	p.query.Domain = true
	p.query.format = "weekday"
	p.query.root, err = p.make(&node{op: "weekday_on", args: []*node{d}})
	return true, err
}

func (p *parser) parseWeekNumber() (bool, error) {
	if p.peek() == "week" && p.i+2 < len(p.tokens) && p.tokens[p.i+1].text == "of" && p.tokens[p.i+2].text == "year" {
		p.i += 3
		p.query.Domain = true
		var err error
		p.query.root, err = p.make(&node{op: "week_of_year"})
		return true, err
	}
	if p.peek() == "week" && p.i+1 < len(p.tokens) && p.tokens[p.i+1].text == "number" {
		p.i += 2
		var d *node
		var err error
		if p.accept("on") {
			d, err = p.expr(0)
			if err != nil {
				return true, err
			}
		} else if p.peek() == "today" || p.peek() == "now" {
			p.i++
		}
		p.query.Domain = true
		if d != nil {
			p.query.root, err = p.make(&node{op: "week_of_year", args: []*node{d}})
		} else {
			p.query.root, err = p.make(&node{op: "week_of_year"})
		}
		return true, err
	}
	if p.peek() == "week" && p.i+2 < len(p.tokens) && p.tokens[p.i+1].text == "of" && p.tokens[p.i+2].text == "month" {
		p.i += 3
		var d *node
		var err error
		if p.accept("on") {
			d, err = p.expr(0)
			if err != nil {
				return true, err
			}
		}
		p.query.Domain = true
		if d != nil {
			p.query.root, err = p.make(&node{op: "week_of_month", args: []*node{d}})
		} else {
			p.query.root, err = p.make(&node{op: "week_of_month"})
		}
		return true, err
	}
	return false, nil
}

func (p *parser) parseDayParts() (bool, error) {
	if p.peek() != "day" {
		return false, nil
	}
	if p.i+1 < len(p.tokens) && p.tokens[p.i+1].text == "number" {
		p.i += 2
		var d *node
		var err error
		if p.accept("on") {
			d, err = p.expr(0)
			if err != nil {
				return true, err
			}
		}
		p.query.Domain = true
		if d != nil {
			p.query.root, err = p.make(&node{op: "day_of_year", args: []*node{d}})
		} else {
			p.query.root, err = p.make(&node{op: "day_of_year"})
		}
		return true, err
	}
	if p.i+2 >= len(p.tokens) || p.tokens[p.i+1].text != "of" {
		return false, nil
	}
	part := p.tokens[p.i+2].text
	if part != "month" && part != "year" && part != "number" {
		return false, nil
	}
	if part == "number" {
		p.i += 3
		p.accept("on")
		d, err := p.expr(0)
		if err != nil {
			return true, err
		}
		p.query.Domain = true
		p.query.root, err = p.make(&node{op: "day_of_year", args: []*node{d}})
		return true, err
	}
	p.i += 3
	op := "day_of_month"
	if part == "year" {
		op = "day_of_year"
	}
	var d *node
	var err error
	if p.accept("on") {
		d, err = p.expr(0)
		if err != nil {
			return true, err
		}
	}
	p.query.Domain = true
	if d != nil {
		p.query.root, err = p.make(&node{op: op, args: []*node{d}})
	} else {
		p.query.root, err = p.make(&node{op: op})
	}
	return true, err
}

func (p *parser) parseDaysInPeriod() (bool, error) {
	if p.peek() != "days" || p.i+1 >= len(p.tokens) || p.tokens[p.i+1].text != "in" {
		return false, nil
	}
	if p.i+2 < len(p.tokens) && (p.tokens[p.i+2].temporal != nil || strings.HasPrefix(p.tokens[p.i+2].text, "q") || isMonthWord(p.tokens[p.i+2].text)) {
		p.i += 2
		if p.peek() == "q" && p.i+1 < len(p.tokens) && p.tokens[p.i+1].value != nil {
			qtr := ratInt(Value{Number: p.tokens[p.i+1].value})
			if qtr < 1 || qtr > 4 {
				return true, invalid("invalid quarter")
			}
			p.i += 2
			p.query.Domain = true
			n, err := p.make(&node{op: "days_in_quarter", value: Value{Kind: Number, Number: big.NewRat(qtr, 1)}})
			p.query.root = n
			return true, err
		}
		if isMonthWord(p.peek()) && p.i+1 < len(p.tokens) && p.tokens[p.i+1].value != nil {
			monthTok := p.peek()
			p.i++
			year := ratInt(Value{Number: p.tokens[p.i].value})
			p.i++
			t, err := parseDate(monthTok+" 1 "+strconv.Itoa(int(year)), time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
			if err != nil {
				return true, err
			}
			d, err := p.make(&node{op: "value", value: Value{Kind: Date, Time: t}})
			if err != nil {
				return true, err
			}
			p.query.Domain = true
			p.query.root, err = p.make(&node{op: "days_in_month", args: []*node{d}})
			return true, err
		}
		d, err := p.expr(0)
		if err != nil {
			return true, err
		}
		p.query.Domain = true
		p.query.root, err = p.make(&node{op: "days_in_month", args: []*node{d}})
		return true, err
	}
	return false, nil
}

func weekOfMonth(t time.Time) int {
	first := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	wd := int(first.Weekday())
	if wd == 0 {
		wd = 7
	}
	daysInFirstWeek := 8 - wd
	offset := 0
	if daysInFirstWeek < 4 {
		offset = daysInFirstWeek
	} else {
		offset = -(wd - 1)
	}
	idx := t.Day() - 1 - offset
	if idx < 0 {
		return 1
	}
	return idx/7 + 1
}

func isMonthWord(s string) bool {
	switch s {
	case "january", "february", "march", "april", "may", "june", "july", "august",
		"september", "october", "november", "december", "jan", "feb", "mar", "apr",
		"jun", "jul", "aug", "sep", "oct", "nov", "dec":
		return true
	}
	return false
}

func (p *parser) parseAfterBefore() (bool, error) {
	saved := p.i
	offset, err := p.expr(25)
	if err != nil {
		p.i = saved
		return false, nil
	}
	kind := ""
	if p.accept("after") {
		kind = "after"
	} else if p.accept("before") {
		kind = "before"
	} else {
		p.i = saved
		return false, nil
	}
	if p.tokens[p.i].temporal == nil && !isMonthWord(p.peek()) {
		p.i = saved
		return false, nil
	}
	d, err := p.expr(0)
	if err != nil {
		return true, err
	}
	p.query.Domain = true
	op := "+"
	if kind == "before" {
		op = "-"
	}
	p.query.root, err = p.make(&node{op: op, args: []*node{d, offset}})
	return true, err
}

func (p *parser) parseFromNow() (bool, error) {
	saved := p.i
	n, err := p.expr(25)
	if err != nil {
		p.i = saved
		return false, nil
	}
	if !p.accept("from") || !(p.accept("now") || p.accept("today")) {
		p.i = saved
		return false, nil
	}
	p.query.Domain = true
	now, err := p.make(&node{op: "temporal", temporal: &temporalQuery{kind: "relative", date: "today"}})
	if err != nil {
		return true, err
	}
	p.query.root, err = p.make(&node{op: "+", args: []*node{now, n}})
	return true, err
}

func (p *parser) parseYearRange() (bool, error) {
	if p.tokens[p.i].value == nil {
		return false, nil
	}
	y1, ok1 := yearLiteral(p.tokens[p.i].value, p.tokens[p.i].text)
	if !ok1 || p.i+2 >= len(p.tokens) || p.tokens[p.i+1].text != "to" {
		return false, nil
	}
	y2, ok2 := yearLiteral(p.tokens[p.i+2].value, p.tokens[p.i+2].text)
	if !ok2 {
		return false, nil
	}
	p.i += 3
	diff := y2 - y1
	if diff < 0 {
		diff = -diff
	}
	// Soulver documents year-to-year as the count of intervening years.
	if diff > 0 {
		diff--
	}
	p.query.Domain = true
	n, err := p.make(&node{op: "value", value: Value{Kind: Number, Number: big.NewRat(int64(diff), 1)}})
	if err != nil {
		return true, err
	}
	p.query.root, err = p.make(&node{op: "quantity", args: []*node{n}, value: Value{Unit: Unit{"y": 1}}})
	return true, err
}

func yearLiteral(v *big.Rat, text string) (int, bool) {
	if v == nil || !v.IsInt() || !v.Num().IsInt64() {
		return 0, false
	}
	n := v.Num().Int64()
	if n < 1000 || n > 9999 || strings.ContainsAny(text, ".:") {
		return 0, false
	}
	return int(n), true
}

func (p *parser) parseThroughDays() (bool, error) {
	saved := p.i
	a, err := p.expr(25)
	if err != nil || !p.accept("through") {
		p.i = saved
		return false, nil
	}
	b, err := p.expr(25)
	if err != nil {
		return true, err
	}
	p.accept("in")
	p.accept("days")
	p.query.Domain = true
	p.query.format = "days"
	p.query.root, err = p.make(&node{op: "days_between_inclusive", args: []*node{a, b}})
	return true, err
}
