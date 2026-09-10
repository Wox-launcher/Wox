package engine

import (
	"math/big"
	"strings"
	"unicode"
)

// stripSoulverNoise removes comments and labels so a single query can keep
// the surrounding words Soulver would ignore.
func stripSoulverNoise(s string) string {
	if i := strings.Index(s, "//"); i >= 0 && !strings.Contains(strings.ToLower(s[:i]), "http:") {
		s = strings.TrimSpace(s[:i])
	}
	var b strings.Builder
	inQuote := false
	runes := []rune(s)
	for i, r := range runes {
		if r == '"' {
			if inQuote {
				inQuote = false
				continue
			}
			if i > 0 && unicode.IsDigit(runes[i-1]) {
				b.WriteRune(r)
				continue
			}
			inQuote = true
			continue
		}
		if !inQuote {
			b.WriteRune(r)
		}
	}
	s = strings.TrimSpace(b.String())
	s = stripWordParens(s)
	if i := strings.LastIndex(s, ": "); i > 0 && unicode.IsLetter([]rune(s)[0]) {
		left := s[:i]
		if !strings.Contains(left, ":") && !clockLooks(left) {
			s = strings.TrimSpace(s[i+2:])
		}
	}
	return strings.TrimSpace(s)
}

func clockLooks(s string) bool {
	for _, r := range s {
		if r == ':' {
			return true
		}
	}
	return false
}

// stripWordParens drops (comment) groups that contain no digits or operators.
func stripWordParens(s string) string {
	var out strings.Builder
	i := 0
	runes := []rune(s)
	for i < len(runes) {
		if runes[i] == '(' {
			end := i + 1
			for end < len(runes) && runes[end] != ')' {
				end++
			}
			if end < len(runes) {
				inner := string(runes[i+1 : end])
				innerTrim := strings.ToLower(strings.TrimSpace(inner))
				hasLetter := strings.ContainsFunc(inner, unicode.IsLetter)
				baseLit := strings.HasPrefix(innerTrim, "0x") || strings.HasPrefix(innerTrim, "0b") || strings.HasPrefix(innerTrim, "0o")
				startsDigit := len(innerTrim) > 0 && unicode.IsDigit([]rune(innerTrim)[0])
				if hasLetter && !baseLit && !startsDigit && !strings.ContainsAny(inner, "+-*/^=<>") {
					i = end + 1
					continue
				}
				// Soulver treats a recent year in brackets after an amount as a comment.
				if precededByAmount(runes, i) && isYearComment(innerTrim) && !strings.ContainsAny(inner, "+-*/^=<>") {
					i = end + 1
					continue
				}
			}
		}
		out.WriteRune(runes[i])
		i++
	}
	return out.String()
}

// precededByAmount reports whether a '(' follows a numeric amount, not a function name.
func precededByAmount(runes []rune, i int) bool {
	j := i - 1
	for j >= 0 && unicode.IsSpace(runes[j]) {
		j--
	}
	return j >= 0 && (unicode.IsDigit(runes[j]) || runes[j] == '.')
}

// isYearComment reports a four-digit year used as a Soulver amount comment.
func isYearComment(s string) bool {
	if len(s) != 4 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s >= "1900" && s <= "2100"
}

func isFillerWord(s string) bool {
	switch s {
	case "the", "my", "i", "spent", "clothes", "breakfast", "uber", "iphone",
		"boing", "cost", "bottle", "bottles", "discount":
		return true
	}
	return false
}

func reservedWord(s string) bool {
	switch s {
	case "plus", "minus", "multiplied", "divided", "remainder", "mod", "power", "exponent",
		"of", "off", "on", "tip", "to", "in", "as", "at", "per", "what", "is",
		"and", "or", "if", "then", "else", "unless", "between", "from", "after",
		"before", "ago", "now", "today", "yesterday", "tomorrow", "rounded",
		"nearest", "up", "down", "half", "midpoint", "larger", "greater",
		"smaller", "lesser", "gcd", "lcm", "clamp", "total", "sum", "average",
		"avg", "mean", "count", "median", "standard", "deviation", "root", "log",
		"base", "square", "cube", "permutation", "permutations", "combination",
		"combinations", "random", "number", "decimal", "dec", "hex", "bin",
		"oct", "percent", "fraction", "multiplier", "multiple", "x", "timespan",
		"laptime", "true", "false", "by", "the", "a", "an", "for", "every",
		"compounding", "compounded", "monthly", "quarterly", "yearly", "daily", "hourly", "weekly", "annual", "interest",
		"investment", "deposit", "required", "needed", "paid",
		"gratuity", "raised", "num", "imperial",
		"invested", "returned", "present", "value", "repayment", "over",
		"vat", "gst", "tax", "income", "growth", "time", "saved", "week", "weeks",
		"month", "months", "year", "years", "day", "days", "hour", "hours",
		"minute", "minutes", "second", "seconds", "workday", "workdays",
		"workhours", "million", "billion", "thousand", "hundred", "dp",
		"digits", "digit", "dps", "pi", "tau", "phi", "e", "since", "till",
		"percentage", "sci",
		"until", "through", "weekday", "payment", "frames", "fps", "upload",
		"download", "copy", "binary", "hexadecimal", "octal", "current",
		"timestamp", "iso8601", "iso", "date", "transfer", "send", "inflation",
		"assuming", "dms", "easter", "christmas", "halloween", "valentine",
		"thanksgiving", "ramadan", "hanukkah", "orthodox", "chinese",
		"next", "halfway":
		return true
	}
	return false
}

func isFormatWord(s string) bool {
	return s == "timespan" || s == "laptime" || s == "number" || s == "num" || s == "decimal" ||
		s == "fraction" || s == "percent" || s == "percentage" || s == "multiplier" || s == "multiple" ||
		s == "binary" || s == "hexadecimal" || s == "octal" || s == "date" ||
		s == "timestamp" || s == "iso8601" || s == "iso" || s == "dms" || s == "timecode" || s == "sci" ||
		s == "pitch" || s == "midi" || isBase(s)
}

func canonicalFormat(s string) string {
	switch s {
	case "binary":
		return "bin"
	case "hexadecimal":
		return "hex"
	case "octal":
		return "oct"
	case "decimal":
		return "dec"
	case "num":
		return "number"
	case "percentage":
		return "percent"
	case "iso":
		return "iso8601"
	}
	return s
}

func (p *parser) skipFillers() {
	for {
		t := p.peek()
		if t == "" {
			return
		}
		if t == "on" || t == "for" {
			if p.i+1 >= len(p.tokens) {
				p.i++
				p.query.Domain = true
				continue
			}
			next := p.tokens[p.i+1]
			if next.value != nil || next.temporal != nil || reservedWord(next.text) && next.text != "the" {
				if next.text != "the" && !isFillerWord(next.text) && next.text != "" {
					if _, ok := p.catalog.resolve(next.text); !ok && !unicode.IsLetter([]rune(next.text)[0]) {
						return
					}
					if reservedWord(next.text) && next.text != "the" {
						return
					}
				}
			}
			p.i++
			p.query.Domain = true
			continue
		}
		if t == "the" && p.i+1 < len(p.tokens) && p.tokens[p.i+1].text == "power" {
			return
		}
		if isFillerWord(t) || t == "the" {
			p.i++
			p.query.Domain = true
			continue
		}
		return
	}
}

// parseSoulverPhrases recognizes documented sentence forms before Pratt.
func (p *parser) parseSoulverPhrases() (bool, error) {
	if ok, err := p.parseIsPrime(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseDatePhrases(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseIncomeTax(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseVAT(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseInvestmentRequired(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseMortgage(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parsePresentValue(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseROI(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parsePercentRatio(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseRemainderPhrase(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseStatPhrase(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseCompareWords(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseMidpointPhrase(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseGcdLcm(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parsePermComb(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseClamp(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseRootLog(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseIfThen(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseProportion(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parsePercentSentences(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseFinancePhrases(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseGrowthPhrases(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseRoundPhrase(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parsePowerQuestion(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseInflation(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseHistoricalInflation(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseExtraPhrases(); ok || err != nil {
		return ok, err
	}
	if ok, err := p.parseNumberDiff(); ok || err != nil {
		return ok, err
	}
	return false, nil
}

// parseNumberDiff evaluates "500 to 3000" as the signed difference.
func (p *parser) parseNumberDiff() (bool, error) {
	if p.tokens[p.i].value == nil {
		return false, nil
	}
	if p.i+2 >= len(p.tokens) || p.tokens[p.i+1].text != "to" || p.tokens[p.i+2].value == nil {
		return false, nil
	}
	if p.i+3 < len(p.tokens) && p.tokens[p.i+3].text != "" {
		return false, nil
	}
	left := p.tokens[p.i].value
	right := p.tokens[p.i+2].value
	p.i += 3
	diff := new(big.Rat).Sub(right, left)
	n, err := p.make(&node{op: "value", value: Value{Kind: Number, Number: diff}})
	if err != nil {
		return true, err
	}
	p.query.Domain = true
	p.query.root = n
	return true, nil
}

func lastContent(tokens []token) string {
	for i := len(tokens) - 1; i >= 0; i-- {
		if tokens[i].text != "" {
			return tokens[i].text
		}
	}
	return ""
}

func (p *parser) parseIsPrime() (bool, error) {
	if lastContent(p.tokens) != "prime" {
		return false, nil
	}
	saved := p.i
	p.accept("is")
	if p.tokens[p.i].value == nil {
		p.i = saved
		return false, nil
	}
	n, err := p.expr(25)
	if err != nil || !p.accept("prime") {
		p.i = saved
		return false, nil
	}
	p.query.Domain = true
	p.query.root, err = p.make(&node{op: "is_prime", args: []*node{n}})
	return true, err
}

func (p *parser) parseRoundPhrase() (bool, error) {
	if p.peek() != "round" {
		return false, nil
	}
	if p.i+1 < len(p.tokens) && p.tokens[p.i+1].text == "(" {
		return false, nil
	}
	p.i++
	dir := "nearest"
	if p.accept("down") {
		dir = "down"
	} else if p.accept("up") {
		dir = "up"
	}
	n, err := p.expr(25)
	if err != nil {
		return true, err
	}
	if p.accept("up") {
		dir = "up"
	} else if p.accept("down") {
		dir = "down"
	}
	if err = p.parseNearest(dir); err != nil {
		return true, err
	}
	p.query.Domain = true
	p.query.root = n
	return true, nil
}

func (p *parser) parsePowerQuestion() (bool, error) {
	// "81 is 9 to what power"
	saved := p.i
	x, err := p.expr(25)
	if err != nil || !p.accept("is") {
		p.i = saved
		return false, nil
	}
	base, err := p.expr(25)
	if err != nil || !p.accept("to") {
		p.i = saved
		return false, nil
	}
	p.accept("the")
	if !p.accept("what") {
		p.i = saved
		return false, nil
	}
	p.accept("power")
	p.query.Domain = true
	p.query.root, err = p.make(&node{op: "logbase", args: []*node{x, base}})
	return true, err
}

func (p *parser) parseRemainderPhrase() (bool, error) {
	if !p.accept("remainder") {
		return false, nil
	}
	if !p.accept("of") {
		return true, invalid("expected of")
	}
	left, err := p.expr(25)
	if err != nil {
		return true, err
	}
	if p.peek() != "divided" {
		return true, invalid("expected divided by")
	}
	p.i++
	if !p.accept("by") {
		return true, invalid("expected by")
	}
	right, err := p.expr(0)
	if err != nil {
		return true, err
	}
	p.query.Domain = true
	p.query.root, err = p.make(&node{op: "mod", args: []*node{left, right}})
	return true, err
}

func (p *parser) parseNumberList() ([]*node, error) {
	var items []*node
	for {
		n, err := p.expr(25)
		if err != nil {
			return nil, err
		}
		items = append(items, n)
		if p.accept(",") || p.accept("and") {
			continue
		}
		if p.peek() == "(" || p.peek() == "$" || (p.i < len(p.tokens) && p.tokens[p.i].value != nil) {
			continue
		}
		break
	}
	if len(items) == 0 {
		return nil, invalid("expected a list")
	}
	return items, nil
}

func (p *parser) parseStatPhrase() (bool, error) {
	if (p.peek() == "min" || p.peek() == "max") && p.i+1 < len(p.tokens) && p.tokens[p.i+1].text == "(" {
		fn := p.tokens[p.i]
		par := p.tokens[p.i+1]
		if par.pos == fn.pos+len(fn.text) {
			return false, nil
		}
	}
	op := ""
	switch p.peek() {
	case "total", "sum":
		op = "list_sum"
	case "average", "mean", "avg":
		op = "list_avg"
	case "count":
		op = "list_count"
	case "median":
		op = "list_median"
	case "min":
		op = "list_min"
	case "max":
		op = "list_max"
	case "standard":
		p.i++
		if !p.accept("deviation") {
			return true, invalid("expected deviation")
		}
		if !p.accept("of") {
			return true, invalid("expected of")
		}
		items, err := p.parseNumberList()
		if err != nil {
			return true, err
		}
		p.query.Domain = true
		p.query.root, err = p.make(&node{op: "list_stddev", args: items})
		return true, err
	default:
		return false, nil
	}
	p.i++
	p.accept("of")
	items, err := p.parseNumberList()
	if err != nil {
		return true, err
	}
	p.query.Domain = true
	p.query.root, err = p.make(&node{op: op, args: items})
	return true, err
}

func (p *parser) parseCompareWords() (bool, error) {
	op := ""
	switch p.peek() {
	case "larger", "greater":
		op = "max2"
	case "smaller", "lesser":
		op = "min2"
	default:
		return false, nil
	}
	p.i++
	if !p.accept("of") {
		return true, invalid("expected of")
	}
	a, err := p.expr(25)
	if err != nil {
		return true, err
	}
	if !p.accept("and") {
		return true, invalid("expected and")
	}
	b, err := p.expr(25)
	if err != nil {
		return true, err
	}
	p.query.Domain = true
	p.query.root, err = p.make(&node{op: op, args: []*node{a, b}})
	return true, err
}

func (p *parser) parseMidpointPhrase() (bool, error) {
	if p.accept("half") {
		if !p.accept("of") {
			return true, invalid("expected of")
		}
		n, err := p.expr(0)
		if err != nil {
			return true, err
		}
		p.query.Domain = true
		p.query.root, err = p.make(&node{op: "half", args: []*node{n}})
		return true, err
	}
	if !p.accept("midpoint") && !p.accept("halfway") {
		return false, nil
	}
	if !p.accept("between") {
		return true, invalid("expected between")
	}
	a, err := p.expr(25)
	if err != nil {
		return true, err
	}
	if !p.accept("and") {
		return true, invalid("expected and")
	}
	b, err := p.expr(25)
	if err != nil {
		return true, err
	}
	p.query.Domain = true
	p.query.root, err = p.make(&node{op: "midpoint", args: []*node{a, b}})
	return true, err
}

func (p *parser) parseGcdLcm() (bool, error) {
	op := p.peek()
	if op != "gcd" && op != "lcm" {
		return false, nil
	}
	p.i++
	p.accept("of")
	items, err := p.parseNumberList()
	if err != nil {
		return true, err
	}
	if len(items) < 2 {
		return true, invalid("expected and")
	}
	p.query.Domain = true
	p.query.root, err = p.make(&node{op: op, args: items})
	return true, err
}

func (p *parser) parsePermComb() (bool, error) {
	if p.tokens[p.i].value == nil {
		return false, nil
	}
	if p.i+1 >= len(p.tokens) {
		return false, nil
	}
	next := p.tokens[p.i+1].text
	if next != "permutation" && next != "permutations" && next != "combination" && next != "combinations" {
		return false, nil
	}
	n, err := p.expr(25)
	if err != nil {
		return true, err
	}
	kind := p.peek()
	p.i++
	op := "perm"
	if strings.HasPrefix(kind, "combination") {
		op = "comb"
	}
	if p.accept("of") {
		m, e := p.expr(0)
		if e != nil {
			return true, e
		}
		p.query.Domain = true
		// "3 permutations of 10" is P(10,3)
		p.query.root, err = p.make(&node{op: op, args: []*node{m, n}})
		return true, err
	}
	m, err := p.expr(0)
	if err != nil {
		return true, err
	}
	p.query.Domain = true
	p.query.root, err = p.make(&node{op: op, args: []*node{n, m}})
	return true, err
}

func (p *parser) parseClamp() (bool, error) {
	if !p.accept("clamp") {
		return false, nil
	}
	v, err := p.expr(25)
	if err != nil {
		return true, err
	}
	if p.accept("between") {
		lo, e := p.expr(25)
		if e != nil {
			return true, e
		}
		if !p.accept("and") {
			return true, invalid("expected and")
		}
		hi, e := p.expr(25)
		if e != nil {
			return true, e
		}
		p.query.Domain = true
		p.query.root, err = p.make(&node{op: "clamp", args: []*node{v, lo, hi}})
		return true, err
	}
	if p.accept("from") {
		lo, e := p.expr(25)
		if e != nil {
			return true, e
		}
		if !p.accept("to") {
			return true, invalid("expected to")
		}
		hi, e := p.expr(25)
		if e != nil {
			return true, e
		}
		p.query.Domain = true
		p.query.root, err = p.make(&node{op: "clamp", args: []*node{v, lo, hi}})
		return true, err
	}
	return true, invalid("expected between or from")
}

func (p *parser) parseRootLog() (bool, error) {
	if p.tokens[p.i].value != nil && p.i+1 < len(p.tokens) && p.tokens[p.i+1].text == "root" {
		n, err := p.expr(25)
		if err != nil || !p.accept("root") || !p.accept("of") {
			return true, invalid("expected root of")
		}
		x, err := p.expr(0)
		if err != nil {
			return true, err
		}
		p.query.Domain = true
		p.query.root, err = p.make(&node{op: "nthroot", args: []*node{n, x}})
		return true, err
	}
	if p.accept("root") {
		n, err := p.expr(25)
		if err != nil {
			return true, err
		}
		if !p.accept("of") {
			return true, invalid("expected of")
		}
		x, err := p.expr(0)
		if err != nil {
			return true, err
		}
		p.query.Domain = true
		p.query.root, err = p.make(&node{op: "nthroot", args: []*node{n, x}})
		return true, err
	}
	if p.accept("log") {
		if p.peek() == "(" {
			p.i--
			return false, nil
		}
		n, err := p.expr(25)
		if err != nil {
			return true, err
		}
		if !p.accept("base") {
			return true, invalid("expected base")
		}
		b, err := p.expr(0)
		if err != nil {
			return true, err
		}
		p.query.Domain = true
		p.query.root, err = p.make(&node{op: "logbase", args: []*node{n, b}})
		return true, err
	}
	return false, nil
}

func (p *parser) parseIfThen() (bool, error) {
	if !p.accept("if") {
		return false, nil
	}
	cond, err := p.expr(0)
	if err != nil {
		return true, err
	}
	if !p.accept("then") {
		return true, invalid("expected then")
	}
	yes, err := p.expr(0)
	if err != nil {
		return true, err
	}
	if !p.accept("else") {
		return true, invalid("expected else")
	}
	no, err := p.expr(0)
	if err != nil {
		return true, err
	}
	p.query.Domain = true
	p.query.root, err = p.make(&node{op: "if", args: []*node{cond, yes, no}})
	return true, err
}

func (p *parser) parsePercentSentences() (bool, error) {
	if p.peek() == "what" && p.i+2 < len(p.tokens) &&
		(p.tokens[p.i+1].text == "percentage" || p.tokens[p.i+1].text == "percent") &&
		p.tokens[p.i+2].text == "change" {
		p.i += 3
		p.accept("is")
		left, err := p.expr(25)
		if err != nil {
			return true, err
		}
		if !p.accept("to") {
			return true, invalid("expected to")
		}
		right, err := p.expr(0)
		if err != nil {
			return true, err
		}
		p.query.Domain = true
		p.query.format = "percent"
		p.query.root, err = p.make(&node{op: "pct_change", args: []*node{left, right}})
		return true, err
	}
	saved := p.i
	if n, err := p.parseIsFractionOfWhat(); n != nil || err != nil {
		p.query.root = n
		return n != nil, err
	}
	// N is P% of/on/off what
	if p.tokens[p.i].value != nil || p.tokens[p.i].text == "$" || p.tokens[p.i].temporal != nil {
		left, err := p.expr(0)
		if err != nil {
			p.i = saved
			return false, nil
		}
		if p.accept("is") {
			if p.accept("what") {
				form := "of"
				if p.accept("%") || p.accept("percent") || p.accept("x") || p.accept("multiplier") {
					if p.accept("with") {
						items, e := p.parseNumberList()
						if e != nil {
							return true, e
						}
						p.query.Domain = true
						p.query.format = "percent"
						p.query.root, err = p.make(&node{op: "pct_of_set", args: append([]*node{left}, items...)})
						return true, err
					}
					if p.accept("after") {
						chg, e := p.expr(0)
						if e != nil {
							return true, e
						}
						p.query.Domain = true
						p.query.format = "percent"
						p.query.root, err = p.make(&node{op: "pct_after", args: []*node{left, chg}})
						return true, err
					}
					if p.peek() == "x" || p.peek() == "multiplier" || p.peek() == "%" {
						p.i++
					}
					if p.accept("off") {
						form = "off"
					} else if p.accept("on") {
						form = "on"
					} else {
						p.accept("of")
					}
					right, e := p.expr(0)
					if e != nil {
						return true, e
					}
					p.query.Domain = true
					p.query.format = "percent"
					if form != "of" {
						p.query.root, err = p.make(&node{op: "pct_" + form, args: []*node{left, right}})
					} else {
						p.query.root, err = p.make(&node{op: "pct_of", args: []*node{left, right}})
					}
					return true, err
				}
			}
			// left is P% of/on/off what
			pct, err := p.expr(25)
			if err == nil && (p.accept("of") || p.peek() == "off" || p.peek() == "on") {
				form := "of"
				if p.accept("off") {
					form = "off"
				} else if p.accept("on") {
					form = "on"
				}
				if p.accept("what") {
					p.query.Domain = true
					p.query.root, err = p.make(&node{op: "pct_base_" + form, args: []*node{left, pct}})
					return true, err
				}
			}
		}
		if p.accept("as") {
			p.accept("a")
			mult := p.accept("x") || p.accept("multiplier") || p.accept("multiple")
			if mult && (p.accept("of") || p.accept("on") || p.accept("off")) {
				form := p.tokens[p.i-1].text
				right, e := p.expr(0)
				if e != nil {
					return true, e
				}
				p.query.Domain = true
				p.query.format = "multiplier"
				op := "pct_of"
				if form == "on" {
					op = "pct_on"
				}
				p.query.root, err = p.make(&node{op: op, args: []*node{left, right}})
				return true, err
			}
			if p.accept("%") || p.accept("percent") {
				if p.accept("of") {
					right, e := p.expr(0)
					if e != nil {
						return true, e
					}
					p.query.Domain = true
					p.query.format = "percent"
					p.query.root, err = p.make(&node{op: "pct_of", args: []*node{left, right}})
					return true, err
				}
				p.query.Domain = true
				p.query.format = "percent"
				p.query.root = left
				return true, nil
			}
			if mult {
				p.query.Domain = true
				p.query.format = "multiplier"
				p.query.root = left
				return true, nil
			}
		}
		if p.accept("to") {
			right, err := p.expr(0)
			if err == nil && (p.accept("is") || p.peek() == "as") {
				if p.peek() == "as" && p.i+1 < len(p.tokens) {
					n := p.tokens[p.i+1].text
					if n != "%" && n != "percent" && n != "x" && n != "multiplier" && n != "what" {
						p.i = saved
						return false, nil
					}
				}
				p.accept("as")
				mult := false
				if p.accept("what") {
					mult = p.accept("x") || p.accept("multiplier")
					p.accept("%")
					p.accept("percent")
				} else {
					mult = p.accept("x") || p.accept("multiplier")
					p.accept("%")
					p.accept("percent")
				}
				p.query.Domain = true
				op := "pct_change"
				p.query.format = "percent"
				if mult {
					op = "ratio_change"
					p.query.format = "multiplier"
				}
				p.query.root, err = p.make(&node{op: op, args: []*node{left, right}})
				return true, err
			}
		}
		p.i = saved
	}
	if ok, err := p.parseBarePercentOf(); ok || err != nil {
		return ok, err
	}
	return false, nil
}

// parseBarePercentOf handles documented shorthands: 15 of 200, 3k on 50k, 50 off 150.
func (p *parser) parseBarePercentOf() (bool, error) {
	saved := p.i
	if p.tokens[p.i].value == nil && p.peek() != "$" {
		return false, nil
	}
	left, err := p.expr(25)
	if err != nil {
		p.i = saved
		return false, nil
	}
	if left.op == "%" || left.op == "percent" || left.op == "/" || left.value.Kind == Percent {
		p.i = saved
		return false, nil
	}
	form := ""
	switch {
	case p.accept("on"):
		form = "on"
	case p.accept("off"):
		form = "off"
	case p.accept("of"):
		form = "of"
	default:
		p.i = saved
		return false, nil
	}
	right, err := p.expr(25)
	if err != nil || p.peek() != "" {
		p.i = saved
		return false, nil
	}
	p.query.Domain = true
	p.query.format = "percent"
	_ = form
	p.query.root, err = p.make(&node{op: "pct_of", args: []*node{left, right}})
	return true, err
}

func (p *parser) parseIsFractionOfWhat() (*node, error) {
	saved := p.i
	left, err := p.expr(25)
	if err != nil || !p.accept("is") {
		p.i = saved
		return nil, nil
	}
	if p.peek() == "what" || p.peek() == "to" {
		p.i = saved
		return nil, nil
	}
	pct, err := p.expr(21)
	if err != nil {
		p.i = saved
		return nil, nil
	}
	form := "of"
	if p.accept("off") {
		form = "off"
	} else if p.accept("on") {
		form = "on"
	} else if !p.accept("of") {
		p.i = saved
		return nil, nil
	}
	if !p.accept("what") {
		p.i = saved
		return nil, nil
	}
	p.query.Domain = true
	n, err := p.make(&node{op: "pct_base_" + form, args: []*node{left, pct}})
	return n, err
}

func (p *parser) parseProportion() (bool, error) {
	// A is to B as C is to what  /  A is to B as what is to D
	saved := p.i
	a, err := p.expr(0)
	if err != nil {
		p.i = saved
		return false, nil
	}
	if !p.accept("is") || !p.accept("to") {
		p.i = saved
		return false, nil
	}
	b, err := p.expr(0)
	if err != nil {
		return true, err
	}
	if !p.accept("as") {
		return true, invalid("expected as")
	}
	if p.accept("what") {
		if !p.accept("is") || !p.accept("to") {
			return true, invalid("expected is to")
		}
		d, e := p.expr(0)
		if e != nil {
			return true, e
		}
		p.query.Domain = true
		p.query.root, err = p.make(&node{op: "proportion", args: []*node{a, b, nilNode(), d}})
		return true, err
	}
	c, err := p.expr(0)
	if err != nil {
		return true, err
	}
	if !p.accept("is") || !p.accept("to") || !p.accept("what") {
		return true, invalid("expected is to what")
	}
	p.query.Domain = true
	p.query.root, err = p.make(&node{op: "proportion", args: []*node{a, b, c, nilNode()}})
	return true, err
}

func nilNode() *node {
	return &node{op: "value", value: Value{Kind: Number, Number: new(big.Rat)}}
}

func (p *parser) parseFinancePhrases() (bool, error) {
	payout := ""
	switch p.peek() {
	case "hourly", "daily", "weekly", "monthly", "annual", "yearly":
		if p.i+1 < len(p.tokens) && p.tokens[p.i+1].text == "interest" {
			payout = p.peek()
			p.i++
		}
	}
	interestOnly := false
	if p.accept("interest") {
		if !p.accept("on") {
			return true, invalid("expected on")
		}
		interestOnly = true
	}
	if p.tokens[p.i].text != "$" && p.tokens[p.i].value == nil {
		if interestOnly {
			return true, invalid("expected amount")
		}
		return false, nil
	}
	saved := p.i
	pv, err := p.expr(25)
	if err != nil {
		p.i = saved
		if interestOnly {
			return true, err
		}
		return false, nil
	}
	periodOp := ""
	if p.accept("after") {
		periodOp = "after"
	} else if p.accept("for") {
		periodOp = "for"
	} else {
		p.i = saved
		if interestOnly {
			return true, invalid("expected after or for")
		}
		return false, nil
	}
	years, err := p.expr(25)
	if err != nil {
		p.i = saved
		return false, nil
	}
	p.accept("years")
	p.accept("year")
	p.accept("months")
	p.accept("month")
	if !p.accept("at") && !p.accept("@") {
		p.i = saved
		return false, nil
	}
	rate, err := p.expr(25)
	if err != nil {
		return true, err
	}
	comp := "yearly"
	if p.accept("compounding") || p.accept("compounded") {
		if p.accept("monthly") {
			comp = "monthly"
		} else if p.accept("quarterly") {
			comp = "quarterly"
		} else if p.accept("daily") {
			comp = "daily"
		}
	}
	if p.accept("per") || p.accept("every") {
		switch p.peek() {
		case "month", "months":
			comp = "per_month"
			p.i++
		case "year", "years":
			comp = "yearly"
			p.i++
		case "day", "days":
			comp = "per_day"
			p.i++
		}
	}
	p.query.Domain = true
	op := "compound"
	if interestOnly {
		op = "compound_interest"
	}
	spec := comp
	if payout != "" {
		spec = payout + ":" + comp
	}
	n, err := p.make(&node{op: op, text: spec, args: []*node{pv, years, rate}})
	if err != nil {
		return true, err
	}
	_ = periodOp
	p.query.root = n
	return true, nil
}

func (p *parser) parseGrowthPhrases() (bool, error) {
	if p.peek() == "growth" && p.i+1 < len(p.tokens) && p.tokens[p.i+1].text == "per" {
		p.accept("growth")
		p.accept("per")
		p.i++ // day/week/month
		if !p.accept("from") {
			return true, invalid("expected from")
		}
		start, err := p.expr(0)
		if err != nil {
			return true, err
		}
		if !p.accept("to") {
			return true, invalid("expected to")
		}
		end, err := p.expr(0)
		if err != nil {
			return true, err
		}
		p.accept("over")
		period, err := p.expr(0)
		if err != nil {
			return true, err
		}
		p.query.Domain = true
		p.query.format = "percent"
		p.query.root, err = p.make(&node{op: "growth_rate", args: []*node{start, end, period}})
		return true, err
	}
	if p.peek() == "time" && p.i+1 < len(p.tokens) && p.tokens[p.i+1].text == "from" {
		p.accept("time")
		p.accept("from")
		start, err := p.expr(0)
		if err != nil {
			return true, err
		}
		if !p.accept("to") {
			return true, invalid("expected to")
		}
		end, err := p.expr(0)
		if err != nil {
			return true, err
		}
		if !p.accept("at") {
			return true, invalid("expected at")
		}
		rate, err := p.expr(0)
		if err != nil {
			return true, err
		}
		kind := "add"
		if p.accept("growth") {
			kind = "growth"
		}
		p.accept("every")
		p.accept("per")
		unit := p.peek()
		if unit != "" {
			p.i++
		}
		p.accept("in")
		if p.peek() == unit || p.peek() == "month" || p.peek() == "months" || p.peek() == "year" || p.peek() == "years" || p.peek() == "day" || p.peek() == "days" {
			p.i++
		}
		p.query.Domain = true
		p.query.root, err = p.make(&node{op: "time_to", text: kind, args: []*node{start, end, rate}})
		return true, err
	}
	if p.peek() == "time" && p.i+1 < len(p.tokens) && p.tokens[p.i+1].text == "to" {
		verb := ""
		if p.i+2 < len(p.tokens) {
			verb = p.tokens[p.i+2].text
		}
		if verb == "upload" || verb == "download" || verb == "copy" || verb == "transfer" || verb == "send" {
			return false, nil
		}
		p.accept("time")
		p.accept("to")
		end, err := p.expr(0)
		if err != nil {
			return true, err
		}
		if !p.accept("at") {
			return true, invalid("expected at")
		}
		rate, err := p.expr(0)
		if err != nil {
			return true, err
		}
		kind := "add"
		if p.accept("growth") {
			kind = "growth"
		}
		var period *node
		if p.accept("every") || p.accept("per") {
			period, err = p.expr(25)
			if err != nil {
				return true, err
			}
		}
		zero, err := p.make(&node{op: "value", value: Value{Kind: Number, Number: new(big.Rat)}})
		if err != nil {
			return true, err
		}
		args := []*node{zero, end, rate}
		if period != nil {
			args = append(args, period)
		}
		p.query.Domain = true
		p.query.root, err = p.make(&node{op: "time_to", text: kind, args: args})
		return true, err
	}
	return false, nil
}

func siMagnitude(s string) *big.Rat {
	switch s {
	case "m", "mn":
		return big.NewRat(1000000, 1)
	case "b", "bn":
		return big.NewRat(1000000000, 1)
	case "t", "tn":
		return big.NewRat(1000000000000, 1)
	}
	return nil
}

func siLetterMagnitude(raw string) *big.Rat {
	switch raw {
	case "M":
		return big.NewRat(1000000, 1)
	case "G":
		return big.NewRat(1000000000, 1)
	case "T":
		return big.NewRat(1000000000000, 1)
	}
	return nil
}

func wordScale(s string) *big.Rat {
	switch s {
	case "hundred":
		return big.NewRat(100, 1)
	case "thousand":
		return big.NewRat(1000, 1)
	case "million":
		return big.NewRat(1000000, 1)
	case "billion", "bn":
		return big.NewRat(1000000000, 1)
	case "trillion", "tn":
		return big.NewRat(1000000000000, 1)
	}
	return nil
}
