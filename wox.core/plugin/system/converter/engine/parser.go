package engine

import (
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode"
	"wox/util/calc"
)

type token struct {
	text     string
	raw      string
	value    *big.Rat
	pos      int
	temporal *temporalQuery
}
type node struct {
	temporal *temporalQuery
	op       string
	value    Value
	args     []*node
	text     string
}
type Query struct {
	root          *node
	temporal      *temporalQuery
	target        Unit
	format        string
	decimals      *int
	nearest       *big.Rat
	roundDir      string
	ppi           *big.Rat
	speed         *big.Rat
	formatUnits   []string
	timeSaved     bool
	laptime       bool
	Expression    string
	Domain        bool
	Crypto        bool
	Money         bool
	fps           *big.Rat
	substance     string
	timecode      bool
	compoundTime  bool
	compoundAngle bool
}
type parser struct {
	tokens          []token
	i, depth, nodes int
	catalog         *Catalog
	query           *Query
}

// lex recognizes numbers using the same separators as Calculator. Words retain
// their positions; date and unit meanings are assigned by grammar, not priority.
func lex(input string, options ParseOptions) ([]token, error) {
	runes := []rune(calc.NormalizeNumberSeparators(input))
	var tokens []token
	for i := 0; i < len(runes); {
		if unicode.IsSpace(runes[i]) {
			i++
			continue
		}
		start := i
		// Two-colon HH:MM:SS is a duration. Clock literals only have one colon, so
		// they must not steal the first HH:MM of a laptime before the second colon.
		if runes[i] >= '0' && runes[i] <= '9' {
			if text, ok := scanTimecode(runes[i:]); ok {
				tokens = append(tokens, token{text: "timecode:" + text, pos: i})
				i += len([]rune(text))
				continue
			}
			if text, sec, ok := scanLaptime(runes[i:]); ok {
				tokens = append(tokens, token{text: "laptime", value: sec, pos: i})
				i += len([]rune(text))
				continue
			}
		}
		if text, t := temporalLiteral(string(runes[i:])); t != nil {
			tokens = append(tokens, token{text: text, pos: i, temporal: t})
			i += len([]rune(text))
			continue
		}
		if runes[i] >= '0' && runes[i] <= '9' || string(runes[i]) == options.DecimalSeparator {
			if i+2 < len(runes) && runes[i] == '0' && strings.ContainsRune("xXbBoO", runes[i+1]) {
				i += 2
				for i < len(runes) && (unicode.IsDigit(runes[i]) || strings.ContainsRune("abcdefABCDEF", runes[i])) {
					i++
				}
				s := string(runes[start:i])
				n, ok := new(big.Int).SetString(s, 0)
				if !ok {
					return nil, invalid("invalid base literal")
				}
				tokens = append(tokens, token{text: s, value: new(big.Rat).SetInt(n), pos: start})
				continue
			}
			s, err := calc.NumberPrefix(runes, &i, len(runes), options.ThousandsSeparator, options.DecimalSeparator)
			if err != nil {
				return nil, err
			}
			if exp, end, ok := scanScientificExponent(runes, i); ok {
				s += "e" + exp
				i = end
			}
			n, ok := new(big.Rat).SetString(s)
			if !ok {
				if parts := strings.SplitN(s, "e", 2); len(parts) == 2 {
					n, ok = scientificRat(parts[0], parts[1])
				}
			}
			if !ok {
				return nil, invalid("invalid number")
			}
			if n.Num().BitLen() > maxBits || n.Denom().BitLen() > maxBits {
				return nil, invalid("number exceeds limit")
			}
			tokens = append(tokens, token{text: string(runes[start:i]), value: n, pos: start})
			continue
		}
		if unicode.IsLetter(runes[i]) || runes[i] == '°' || runes[i] == 'º' {
			if n, end, ok := scanEnglishNumber(runes, i); ok {
				tokens = append(tokens, token{text: string(runes[i:end]), value: n, pos: i})
				i = end
				continue
			}
			i++
			for i < len(runes) && (unicode.IsLetter(runes[i]) || runes[i] == '_') {
				i++
			}
			raw := string(runes[start:i])
			text := strings.ToLower(raw)
			if i < len(runes) && unicode.IsDigit(runes[i]) {
				j := i
				for j < len(runes) && unicode.IsDigit(runes[j]) {
					j++
				}
				combined := text + string(runes[i:j])
				if _, ok := calc.Functions[combined]; ok || isExtraFunction(combined) {
					text = combined
					i = j
				}
			}
			if i < len(runes) && runes[i] == '$' {
				raw += "$"
				text += "$"
				i++
			}
			tokens = append(tokens, token{text: text, raw: raw, pos: start})
			continue
		}
		if i+1 < len(runes) {
			two := string(runes[i : i+2])
			if two == "**" {
				tokens = append(tokens, token{text: "^", pos: i})
				i += 2
				continue
			}
			if two == "<<" || two == ">>" {
				tokens = append(tokens, token{text: two, pos: i})
				i += 2
				continue
			}
			if two == "==" || two == "!=" || two == ">=" || two == "<=" || two == "&&" || two == "||" {
				tokens = append(tokens, token{text: two, pos: i})
				i += 2
				continue
			}
		}
		switch runes[i] {
		case '√':
			tokens = append(tokens, token{text: "√", pos: i})
			i++
			continue
		case '′':
			tokens = append(tokens, token{text: "′", pos: i})
			i++
			continue
		case '″':
			tokens = append(tokens, token{text: "″", pos: i})
			i++
			continue
		case '×':
			tokens = append(tokens, token{text: "*", pos: i})
			i++
			continue
		case '÷':
			tokens = append(tokens, token{text: "/", pos: i})
			i++
			continue
		case '>', '<':
			tokens = append(tokens, token{text: string(runes[i]), pos: i})
			i++
			continue
		case '@':
			tokens = append(tokens, token{text: "@", pos: i})
			i++
			continue
		}
		if runes[i] == '\'' {
			if i > 0 && unicode.IsDigit(runes[i-1]) {
				tokens = append(tokens, token{text: "'", pos: i})
			}
			i++
			continue
		}
		if runes[i] == '"' {
			tokens = append(tokens, token{text: "\"", pos: i})
			i++
			continue
		}
		if strings.ContainsRune("+-*/^(),;%=?$€£¥²³−–&|", runes[i]) {
			text := string(runes[i])
			if runes[i] == '−' || runes[i] == '–' {
				text = "-"
			}
			tokens = append(tokens, token{text: text, pos: i})
			i++
			continue
		}
		return nil, &Error{Kind: Unrecognized, Position: i, Message: "unknown character"}
	}
	tokens = append(tokens, token{pos: len(runes)})
	return tokens, nil
}

func englishNumberPart(w string) (value, scale int, ok bool) {
	switch w {
	case "zero":
		return 0, 0, true
	case "one":
		return 1, 0, true
	case "two":
		return 2, 0, true
	case "three":
		return 3, 0, true
	case "four":
		return 4, 0, true
	case "five":
		return 5, 0, true
	case "six":
		return 6, 0, true
	case "seven":
		return 7, 0, true
	case "eight":
		return 8, 0, true
	case "nine":
		return 9, 0, true
	case "ten":
		return 10, 0, true
	case "eleven":
		return 11, 0, true
	case "twelve":
		return 12, 0, true
	case "thirteen":
		return 13, 0, true
	case "fourteen":
		return 14, 0, true
	case "fifteen":
		return 15, 0, true
	case "sixteen":
		return 16, 0, true
	case "seventeen":
		return 17, 0, true
	case "eighteen":
		return 18, 0, true
	case "nineteen":
		return 19, 0, true
	case "twenty":
		return 20, 0, true
	case "thirty":
		return 30, 0, true
	case "forty":
		return 40, 0, true
	case "fifty":
		return 50, 0, true
	case "sixty":
		return 60, 0, true
	case "seventy":
		return 70, 0, true
	case "eighty":
		return 80, 0, true
	case "ninety":
		return 90, 0, true
	case "hundred":
		return 0, 100, true
	case "thousand":
		return 0, 1000, true
	case "million":
		return 0, 1000000, true
	case "billion":
		return 0, 1000000000, true
	case "trillion":
		return 0, 1000000000000, true
	}
	return 0, 0, false
}

// scanEnglishNumber folds "five hundred thirty three" into one numeric token.
func scanEnglishNumber(runes []rune, i int) (*big.Rat, int, bool) {
	total, current := 0, 0
	lastScale := 0
	got := false
	end := i
	for end < len(runes) {
		for end < len(runes) && unicode.IsSpace(runes[end]) {
			end++
		}
		if end >= len(runes) {
			break
		}
		if runes[end] == '-' && got {
			end++
			continue
		}
		if !unicode.IsLetter(runes[end]) {
			break
		}
		start := end
		for end < len(runes) && unicode.IsLetter(runes[end]) {
			end++
		}
		w := strings.ToLower(string(runes[start:end]))
		if w == "and" {
			if lastScale < 100 {
				end = start
				break
			}
			continue
		}
		v, scale, ok := englishNumberPart(w)
		if !ok {
			end = start
			break
		}
		if !got && scale >= 100 {
			return nil, i, false
		}
		got = true
		switch {
		case scale == 100:
			if current == 0 {
				current = 1
			}
			current *= 100
			lastScale = 100
		case scale >= 1000:
			if current == 0 {
				current = 1
			}
			total += current * scale
			current = 0
			lastScale = scale
		default:
			current += v
			lastScale = 0
		}
	}
	if !got {
		return nil, i, false
	}
	total += current
	return big.NewRat(int64(total), 1), end, true
}

func scanScientificExponent(runes []rune, i int) (string, int, bool) {
	if i >= len(runes) || (runes[i] != 'e' && runes[i] != 'E') {
		return "", i, false
	}
	j := i + 1
	if j < len(runes) && (runes[j] == '+' || runes[j] == '-') {
		j++
	}
	start := j
	seenDot := false
	for j < len(runes) {
		if runes[j] >= '0' && runes[j] <= '9' {
			j++
			continue
		}
		if !seenDot && runes[j] == '.' && j+1 < len(runes) && runes[j+1] >= '0' && runes[j+1] <= '9' {
			seenDot = true
			j++
			continue
		}
		break
	}
	if j == start {
		return "", i, false
	}
	return string(runes[i+1 : j]), j, true
}

func scientificRat(mantissa, exp string) (*big.Rat, bool) {
	m, ok := new(big.Rat).SetString(mantissa)
	if !ok {
		return nil, false
	}
	if !strings.ContainsAny(exp, ".") {
		e, err := strconv.Atoi(exp)
		if err != nil {
			return nil, false
		}
		pow := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(absInt(e))), nil)
		scale := new(big.Rat).SetInt(pow)
		if e >= 0 {
			return m.Mul(m, scale), true
		}
		return m.Quo(m, scale), true
	}
	ef, err := strconv.ParseFloat(exp, 64)
	if err != nil {
		return nil, false
	}
	mf, _ := m.Float64()
	r, err := calc.RatFromFloat(mf * math.Pow(10, ef))
	if err != nil {
		return nil, false
	}
	return r, true
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// Parse constructs syntax only: neither clocks nor prices are read here.
func (c *Catalog) Parse(input string, options ParseOptions) (result *Query, err error) {
	defer func() {
		if err != nil {
			if _, typed := err.(*Error); !typed {
				err = invalid(err.Error())
			}
		}
	}()

	if len(input) > 4096 {
		return nil, invalid("input exceeds limit")
	}
	input = stripDigitUnderscores(stripSoulverNoise(strings.TrimSpace(input)))
	if input == "" {
		return nil, &Error{Kind: Unrecognized, Message: "empty input"}
	}
	if encoded, ok := parseBase64Query(input); ok {
		return encoded, nil
	}
	q := &Query{Expression: input}
	if expr, pattern, ok := splitDatePattern(input); ok {
		input = expr
		q.format = "datepattern"
		q.formatUnits = []string{pattern}
		q.Domain = true
	}
	if t, matched, err := c.parseTemporal(input); matched {
		if err != nil {
			return nil, err
		}
		q.temporal = t
		q.Domain = true
		return q, nil
	}
	if options.DecimalSeparator == "" {
		options.DecimalSeparator = "."
	}
	tokens, err := lex(input, options)
	if err != nil {
		return nil, err
	}
	p := parser{tokens: tokens, catalog: c, query: q}
	for isFillerWord(p.peek()) || p.peek() == "the" || p.peek() == "is" && p.i+1 < len(p.tokens) && (p.tokens[p.i+1].text == "$" || p.tokens[p.i+1].value != nil) && lastContent(p.tokens) != "prime" {
		p.i++
		q.Domain = true
	}
	if ok, phraseErr := p.parseSoulverPhrases(); ok {
		if phraseErr != nil {
			return nil, phraseErr
		}
	} else if phraseErr != nil {
		return nil, phraseErr
	} else if p.peek() == "time" && p.i+1 < len(p.tokens) && p.tokens[p.i+1].text == "saved" {
		p.accept("time")
		p.accept("saved")
		q.Domain = true
		q.timeSaved = true
		q.root, err = p.expr(0)
		if err != nil {
			return nil, err
		}
		if err = p.parsePlayback(); err != nil {
			return nil, err
		}
	} else if p.looksLikeInverted() {
		if err = p.parseInverted(); err != nil {
			return nil, err
		}
	} else if p.looksLikeUnitPair() {
		if err = p.parseUnitPair(); err != nil {
			return nil, err
		}
	} else {
		q.root, err = p.expr(0)
		if err != nil {
			return nil, err
		}
	}
	if !q.timeSaved && q.target == nil && p.accept("rounded") {
		q.Domain = true
		dir := "nearest"
		if p.accept("up") {
			dir = "up"
		} else if p.accept("down") {
			dir = "down"
		}
		if p.peek() == "to" {
			if err = p.parseNearest(dir); err != nil {
				return nil, err
			}
		} else {
			p.query.nearest = big.NewRat(1, 1)
			p.query.roundDir = dir
		}
	} else if !q.timeSaved && q.target == nil && p.takeSubstance() {
		// Conversion of a cooking mass/volume follows the substance name.
		if p.accept("to") || p.accept("in") || p.accept("as") {
			q.Domain = true
			var u Unit
			u, err = p.unit()
			if err != nil {
				return nil, err
			}
			q.target = u
		}
	} else if !q.timeSaved && q.target == nil && (p.accept("to") || p.accept("in") || p.accept("as") || p.accept("=")) {
		q.Domain = true
		prep := p.tokens[p.i-1].text
		if prep == "=" && !p.accept("?") {
			return nil, invalid("expected ?")
		}
		// Rounding uses "to N dp|digits" or "to nearest N". Claim it before unit().
		rounded := false
		if prep == "to" {
			if p.accept("nearest") {
				if err = p.finishNearest("nearest"); err != nil {
					return nil, err
				}
				rounded = true
			} else {
				var ok bool
				ok, err = p.tryRounding()
				if err != nil {
					return nil, err
				}
				rounded = ok
			}
		}
		if !rounded && p.tokens[p.i].value != nil {
			if err = p.parsePaceAfter(prep); err != nil {
				return nil, err
			}
		} else if !rounded {
			if p.accept("base") {
				t := p.tokens[p.i]
				if t.value == nil || !t.value.IsInt() {
					return nil, invalid("expected base")
				}
				n := t.value.Num().Int64()
				switch n {
				case 2:
					q.format = "bin"
				case 8:
					q.format = "oct"
				case 10:
					q.format = "dec"
				case 16:
					q.format = "hex"
				default:
					return nil, invalid("unsupported base")
				}
				p.i++
			} else if p.peek() == "/" && p.i+1 < len(p.tokens) {
				p.i++
				var u Unit
				u, err = p.unit()
				if err != nil {
					return nil, err
				}
				for k, n := range u {
					u[k] = -n
					if k == "month" {
						delete(u, k)
						u["mo"] = -n
					}
				}
				q.target = u
			} else if p.peek() == "iso" && p.i+1 < len(p.tokens) && p.tokens[p.i+1].text == "8601" {
				q.format = "iso8601"
				p.i += 2
			} else if p.peek() == "time" && p.i+1 < len(p.tokens) && p.tokens[p.i+1].text == "span" {
				q.format = "timespan"
				p.i += 2
			} else if isFormatWord(p.peek()) {
				q.format = canonicalFormat(p.peek())
				p.i++
			} else {
				var u Unit
				u, err = p.unit()
				if err != nil {
					return nil, err
				}
				if p.accept("and") {
					if err = p.parseUnitParts(u); err != nil {
						return nil, err
					}
				} else {
					q.target = u
					if (p.peek() == "at" || p.peek() == "@") && p.i+2 < len(p.tokens) && p.tokens[p.i+1].value != nil && (p.tokens[p.i+2].text == "ppi" || p.tokens[p.i+2].text == "dpi") {
						p.i++
						q.ppi = p.tokens[p.i].value
						p.i++
						p.i++
					}
				}
			}
		}
		if p.accept("to") {
			if p.accept("nearest") {
				if err = p.finishNearest("nearest"); err != nil {
					return nil, err
				}
			} else if ok, e := p.tryRounding(); e != nil {
				return nil, e
			} else if !ok {
				return nil, invalid("expected rounding")
			}
		}
	}
	if !q.timeSaved {
		if err = p.parseFPS(); err != nil {
			return nil, err
		}
		if err = p.parseRateAt(); err != nil {
			return nil, err
		}
		if q.speed == nil {
			if err = p.parsePlayback(); err != nil {
				return nil, err
			}
		}
		if err = p.parseForPeriod(); err != nil {
			return nil, err
		}
	}
	p.skipFillers()
	if p.peek() != "" && q.target == nil {
		saved := p.i
		if u, unitErr := p.unit(); unitErr == nil && p.peek() == "" {
			q.Domain = true
			q.target = u
		} else {
			p.i = saved
		}
	}
	if p.peek() != "" {
		return nil, &Error{Kind: Invalid, Position: p.tokens[p.i].pos, Message: "unexpected " + p.peek()}
	}
	if q.format == "" && len(q.target) == 0 && q.speed == nil && !q.timecode && q.fps == nil && q.compoundTime && !hasTemporalRoot(q.root) {
		q.format = "timespan"
	}
	if q.format == "" && len(q.target) == 0 && q.compoundAngle {
		q.format = "dms"
	}
	if err = c.resolveAmbiguity(q); err != nil {
		return nil, err
	}
	return q, nil
}

func hasTemporalRoot(n *node) bool {
	if n == nil {
		return false
	}
	if n.op == "temporal" || n.temporal != nil {
		return true
	}
	for _, a := range n.args {
		if hasTemporalRoot(a) {
			return true
		}
	}
	return false
}

func (p *parser) peek() string { return p.tokens[p.i].text }
func (p *parser) accept(s string) bool {
	if p.peek() == s {
		p.i++
		return true
	}
	return false
}

// make enforces a bound on constructed syntax before evaluation.
func (p *parser) make(n *node) (*node, error) {
	p.nodes++
	if p.nodes > 1024 {
		return nil, invalid("too many nodes")
	}
	return n, nil
}

// expr is a Pratt parser; percentages remain nodes until the enclosing operation evaluates.
func (p *parser) expr(min int) (*node, error) {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > 64 {
		return nil, invalid("nesting exceeds limit")
	}
	left, err := p.primary()
	if err != nil {
		return nil, err
	}
	for {
		if !reservedWord(p.peek()) {
			if _, ok := calc.Functions[p.peek()]; ok || isExtraFunction(p.peek()) {
				right, e := p.primary()
				if e != nil {
					return nil, e
				}
				p.query.Domain = true
				left, err = p.make(&node{op: "*", args: []*node{left, right}})
				if err != nil {
					return nil, err
				}
				continue
			}
		}
		p.skipFillers()
		if err = p.parseFPS(); err != nil {
			return nil, err
		}
		op := p.peek()
		precedence := 0
		switch op {
		case "+", "-", "plus", "minus":
			precedence = 10
		case "of", "off", "on", "tip", "gratuity":
			precedence = 20
		case "*", "/", "multiplied", "divided", "mod":
			precedence = 22
		case "^", "power", "**", "raised", "exponent":
			precedence = 30
		case "<<", ">>":
			precedence = 8
		case "&", "xor":
			precedence = 7
		case "|":
			precedence = 6
		case "==", "!=", ">", "<", ">=", "<=":
			precedence = 5
		case "and", "or", "&&", "||":
			precedence = 4
		case "%":
			precedence = 40
		case "to":
			if p.i+3 < len(p.tokens) && p.tokens[p.i+1].text == "the" && p.tokens[p.i+2].text == "power" && p.tokens[p.i+3].text == "of" {
				precedence = 30
			} else if left.op == "temporal" && p.i+1 < len(p.tokens) && p.tokens[p.i+1].temporal != nil {
				precedence = 10
			}
		}
		if precedence == 0 || precedence < min {
			break
		}
		p.i++
		if op == "%" {
			p.query.Domain = true
			pctTok := p.tokens[p.i-1]
			attached := p.i >= 2 && pctTok.pos == p.tokens[p.i-2].pos+len([]rune(p.tokens[p.i-2].text))
			if !attached {
				p.query.format = "percent"
				continue
			}
			left, err = p.make(&node{op: "percent", args: []*node{left}})
			if err != nil {
				return nil, err
			}
			continue
		}
		if op == "plus" {
			op = "+"
		}
		if op == "minus" {
			op = "-"
		}
		if op == "to" {
			if p.peek() == "the" {
				p.accept("the")
				p.accept("power")
				p.accept("of")
				op = "^"
			} else {
				op = "clock_to"
				p.query.Domain = true
				if p.query.format == "" && left.op == "temporal" && left.temporal != nil && left.temporal.kind == "clockLiteral" {
					p.query.format = "parts"
					p.query.formatUnits = []string{"h", "min"}
				}
			}
		}
		if op == "multiplied" {
			if !p.accept("by") {
				return nil, invalid("expected by")
			}
			op = "*"
		}
		if op == "divided" {
			if !p.accept("by") {
				return nil, invalid("expected by")
			}
			op = "/"
		}
		if op == "&&" {
			op = "and"
		}
		if op == "||" {
			op = "or"
		}
		if op == "exponent" {
			op = "power"
		}
		if op == "power" || op == "raised" || op == "of" || op == "off" || op == "on" || op == "tip" || op == "gratuity" || op == "mod" || op == "and" || op == "or" || op == "==" || op == "!=" || op == ">" || op == "<" || op == ">=" || op == "<=" || op == "&" || op == "|" || op == "xor" || op == "<<" || op == ">>" {
			p.query.Domain = true
		}
		if op == "gratuity" {
			op = "tip"
		}
		if op == "raised" {
			if !p.accept("to") {
				return nil, invalid("expected to")
			}
			op = "power"
		}
		if op == "tip" && !p.accept("on") {
			return nil, invalid("expected on")
		}
		next := precedence + 1
		if op == "^" || op == "power" {
			next = precedence
		}
		right, e := p.expr(next)
		if e != nil {
			return nil, e
		}
		left, err = p.make(&node{op: op, args: []*node{left, right}})
		if err != nil {
			return nil, err
		}
	}
	return left, nil
}

// primary assigns literal and domain meanings without consuming conversion targets.
func (p *parser) primary() (*node, error) {
	p.skipFillers()
	if p.accept("true") || p.accept("false") {
		p.query.Domain = true
		wantTrue := p.tokens[p.i-1].text == "true"
		if p.peek() == "if" || p.peek() == "unless" {
			unless := p.accept("unless")
			if !unless {
				p.accept("if")
			}
			cond, err := p.expr(0)
			if err != nil {
				return nil, err
			}
			if wantTrue == unless {
				return p.make(&node{op: "not", args: []*node{cond}})
			}
			return cond, nil
		}
		v := big.NewRat(0, 1)
		if wantTrue {
			v = big.NewRat(1, 1)
		}
		return p.make(&node{op: "value", value: Value{Kind: Boolean, Number: v}})
	}
	if p.accept("half") {
		if !p.accept("of") {
			return nil, invalid("expected of")
		}
		p.query.Domain = true
		arg, err := p.expr(0)
		if err != nil {
			return nil, err
		}
		return p.make(&node{op: "half", args: []*node{arg}})
	}
	if p.accept("√") {
		p.query.Domain = true
		arg, err := p.expr(40)
		if err != nil {
			return nil, err
		}
		return p.make(&node{op: "function", text: "sqrt", args: []*node{arg}})
	}
	if p.accept("+") {
		return p.expr(25)
	}
	if p.accept("-") {
		n, e := p.expr(25)
		if e != nil {
			return nil, e
		}
		return p.make(&node{op: "neg", args: []*node{n}})
	}
	if p.accept("(") {
		n, e := p.expr(0)
		if e != nil {
			return nil, e
		}
		if !p.accept(")") {
			return nil, invalid("missing closing parenthesis")
		}
		return p.suffix(n)
	}
	if p.peek() == "square" || p.peek() == "cube" {
		name := "sqrt"
		if p.peek() == "cube" {
			name = "cbrt"
		}
		p.i++
		p.query.Domain = true
		if !p.accept("root") || !p.accept("of") {
			return nil, invalid("expected root of")
		}
		n, e := p.expr(25)
		if e != nil {
			return nil, e
		}
		return p.make(&node{op: "function", text: name, args: []*node{n}})
	}
	if p.accept("ratio") {
		p.query.Domain = true
		if !p.accept("of") {
			return nil, invalid("expected of")
		}
		a, e := p.expr(0)
		if e != nil {
			return nil, e
		}
		if !p.accept("to") {
			return nil, invalid("expected to")
		}
		b, e := p.expr(0)
		if e != nil {
			return nil, e
		}
		return p.make(&node{op: "ratio", args: []*node{a, b}})
	}
	t := p.tokens[p.i]
	if t.text == "now" {
		t.temporal = &temporalQuery{kind: "now"}
	}
	if t.text == "next" && p.i+1 < len(p.tokens) {
		if wd, ok := parseWeekdayName(p.tokens[p.i+1].text); ok {
			p.i += 2
			p.query.Domain = true
			return p.make(&node{op: "temporal", temporal: &temporalQuery{kind: "nextWeekday", weekday: wd}})
		}
	}
	if t.text == "today" || t.text == "yesterday" || t.text == "tomorrow" {
		t.temporal = &temporalQuery{kind: "relative", date: t.text}
	}
	if (t.text == "current" || t.text == "new") && p.i+1 < len(p.tokens) && p.tokens[p.i+1].text == "timestamp" {
		p.i += 2
		p.query.Domain = true
		p.query.format = "timestamp"
		return p.make(&node{op: "temporal", temporal: &temporalQuery{kind: "now"}})
	}
	if t.temporal != nil {
		p.i++
		p.query.Domain = true
		if t.temporal.kind == "clockLiteral" {
			if n, ok, err := p.clockAsDuration(t.temporal); ok || err != nil {
				return n, err
			}
		}
		return p.make(&node{op: "temporal", temporal: t.temporal})
	}
	if isBase(t.text) && (p.i+1 >= len(p.tokens) || p.tokens[p.i+1].text != "(") {
		p.i++
		digits := p.peek()
		if digits == "" {
			return nil, invalid("missing base number")
		}
		p.i++
		n, e := parseBase(digits, t.text)
		if e != nil {
			return nil, e
		}
		p.query.Domain = true
		return p.make(&node{op: "value", value: Value{Kind: Number, Number: n}, text: "base"})
	}
	if _, ok := calc.Functions[t.text]; ok || isExtraFunction(t.text) {
		p.i++
		if isExtraFunction(t.text) {
			p.query.Domain = true
		}
		if !p.accept("(") {
			return nil, invalid("expected function arguments")
		}
		var args []*node
		if !p.accept(")") {
			for {
				n, e := p.expr(0)
				if e != nil {
					return nil, e
				}
				args = append(args, n)
				if p.accept(")") {
					break
				}
				if !p.accept(",") && !p.accept(";") {
					return nil, invalid("expected argument separator")
				}
			}
		}
		n, e := p.make(&node{op: "function", text: t.text, args: args})
		if e != nil {
			return nil, e
		}
		return p.suffix(n)
	}
	if note, ok := p.takeNote(); ok {
		p.query.Domain = true
		return p.make(&node{op: "note", value: Value{Kind: Number, Number: ratPlaces(float64(note), 0)}})
	}
	if t.text == "pi" || t.text == "π" || t.text == "e" || t.text == "tau" || t.text == "phi" {
		p.i++
		v := rational("3.1415926535897932384626433832795028841971693993751058209749445923")
		if t.text == "e" {
			v = rational("2.7182818284590452353602874713526624977572470936999595749669676277")
		} else if t.text == "tau" {
			v.Mul(v, big.NewRat(2, 1))
		} else if t.text == "phi" {
			v = rational("1.6180339887498948482045868343656381177203091798057628621354486227")
		}
		return p.make(&node{op: "value", value: Value{Kind: Number, Number: v}})
	}
	// Prefix currencies must precede a numeric literal, e.g. USD1K or $12.
	if symbol, ok := p.catalog.resolve(t.text); ok && p.catalog.Units[symbol].Dimension == "money" {
		p.i++
		v := p.tokens[p.i]
		if v.value == nil {
			return nil, invalid("expected currency amount")
		}
		p.i++
		n, e := p.make(&node{op: "value", value: Value{Kind: Number, Number: v.value}})
		if e != nil {
			return nil, e
		}
		n, e = p.magnitude(n)
		if e != nil {
			return nil, e
		}
		p.query.Domain = true
		p.query.Crypto = p.query.Crypto || p.catalog.Crypto[symbol]
		p.query.Money = p.query.Money || p.catalog.Units[symbol].Dimension == "money"
		n, e = p.make(&node{op: "quantity", value: Value{Unit: Unit{symbol: 1}}, args: []*node{n}})
		if e != nil {
			return nil, e
		}
		return p.suffix(n)
	}
	if strings.HasPrefix(t.text, "timecode:") {
		p.i++
		p.query.Domain = true
		p.query.timecode = true
		return p.make(&node{op: "timecode", text: strings.TrimPrefix(t.text, "timecode:")})
	}
	if t.value != nil && t.text == "laptime" {
		p.i++
		p.query.Domain = true
		p.query.laptime = true
		n, e := p.make(&node{op: "value", value: Value{Kind: Number, Number: t.value}})
		if e != nil {
			return nil, e
		}
		return p.make(&node{op: "quantity", args: []*node{n}, value: Value{Unit: Unit{"s": 1}}})
	}
	if t.value == nil {
		// Hexadecimal suffix form contains letters and is meaningful only before hex.
		if p.i+1 < len(p.tokens) && p.tokens[p.i+1].text == "hex" {
			n, e := parseBase(t.text, "hex")
			if e == nil {
				p.i += 2
				p.query.Domain = true
				return p.make(&node{op: "value", value: Value{Kind: Number, Number: n}, text: "base"})
			}
		}
		return nil, &Error{Kind: Unrecognized, Position: t.pos, Message: "expected value"}
	}
	p.i++
	v := t.value
	base := strings.HasPrefix(strings.ToLower(t.text), "0x") || strings.HasPrefix(strings.ToLower(t.text), "0b") || strings.HasPrefix(strings.ToLower(t.text), "0o")
	if isBase(p.peek()) {
		var e error
		v, e = parseBase(t.text, p.peek())
		if e != nil {
			return nil, e
		}
		p.i++
		base = true
	}
	n, e := p.make(&node{op: "value", value: Value{Kind: Number, Number: v}})
	if e != nil {
		return nil, e
	}
	if base {
		n.text = "base"
		p.query.Domain = true
	}
	n, e = p.magnitude(n)
	if e != nil {
		return nil, e
	}
	if p.accept("²") {
		p.query.Domain = true
		n, e = p.make(&node{op: "^", args: []*node{n, {op: "value", value: number(2)}}})
		if e != nil {
			return nil, e
		}
	}
	if p.tokens[p.i].value != nil && p.i+2 < len(p.tokens) && p.tokens[p.i+1].text == "/" && p.tokens[p.i+2].value != nil {
		num := p.tokens[p.i].value
		den := p.tokens[p.i+2].value
		if den.Sign() != 0 {
			p.i += 3
			p.query.Domain = true
			frac := new(big.Rat).Quo(new(big.Rat).Set(num), new(big.Rat).Set(den))
			n, e = p.make(&node{op: "+", args: []*node{n, {op: "value", value: Value{Kind: Number, Number: frac}}}})
			if e != nil {
				return nil, e
			}
		}
	}
	return p.suffix(n)
}

// magnitude recognizes attached K while leaving spaced temperature units intact.
func (p *parser) magnitude(n *node) (*node, error) {
	// K is a magnitude only when attached to the preceding numeric token.
	attached := p.i > 0 && p.tokens[p.i].pos == p.tokens[p.i-1].pos+len([]rune(p.tokens[p.i-1].text))
	if p.peek() == "k" && attached {
		p.i++
		p.query.Domain = true
		return p.make(&node{op: "*", args: []*node{n, {op: "value", value: number(1000)}}})
	}
	// $1M keeps the currency prefix; a following attached m is 10^6, not minutes.
	if attached && p.i >= 2 {
		if symbol, ok := p.catalog.resolve(p.tokens[p.i-2].text); ok && p.catalog.Units[symbol].Dimension == "money" {
			if scale := siMagnitude(p.peek()); scale != nil {
				p.i++
				p.query.Domain = true
				return p.make(&node{op: "*", args: []*node{n, {op: "value", value: Value{Kind: Number, Number: scale}}}})
			}
		}
	}
	// Soulver treats attached uppercase M/G/T as million/billion/trillion; lowercase m/g/t stay units.
	if attached {
		if scale := siLetterMagnitude(p.tokens[p.i].raw); scale != nil {
			p.i++
			p.query.Domain = true
			return p.make(&node{op: "*", args: []*node{n, {op: "value", value: Value{Kind: Number, Number: scale}}}})
		}
	}
	if scale := wordScale(p.peek()); scale != nil {
		p.i++
		p.query.Domain = true
		return p.make(&node{op: "*", args: []*node{n, {op: "value", value: Value{Kind: Number, Number: scale}}}})
	}
	return n, nil
}

func (p *parser) quantityWith(n *node, u Unit) (*node, error) {
	if n.op == "quantity" {
		if n.value.Unit == nil {
			n.value.Unit = Unit{}
		}
		for k, v := range u {
			n.value.Unit[k] += v
			if n.value.Unit[k] == 0 {
				delete(n.value.Unit, k)
			}
		}
		return n, nil
	}
	return p.make(&node{op: "quantity", args: []*node{n}, value: Value{Unit: u}})
}

// suffix distinguishes an inch suffix from the conversion preposition in.
// clockAsDuration turns "1:34 hour" and "2:35 min" into a duration quantity.
func (p *parser) clockAsDuration(q *temporalQuery) (*node, bool, error) {
	major := ""
	switch p.peek() {
	case "hour", "hours", "hr", "hrs", "h":
		major = "h"
	case "minute", "minutes", "min", "mins":
		major = "min"
	default:
		return nil, false, nil
	}
	clock, err := parseClock(q.clock)
	if err != nil {
		return nil, false, nil
	}
	p.i++
	var sec int
	if major == "h" {
		sec = clock.Hour()*3600 + clock.Minute()*60 + clock.Second()
	} else {
		sec = clock.Hour()*60 + clock.Minute()
	}
	n, err := p.make(&node{op: "value", value: Value{Kind: Number, Number: big.NewRat(int64(sec), 1)}})
	if err != nil {
		return nil, true, err
	}
	p.query.compoundTime = true
	out, err := p.make(&node{op: "quantity", args: []*node{n}, value: Value{Unit: Unit{"s": 1}}})
	return out, true, err
}

func (p *parser) suffix(n *node) (*node, error) {
	for isFillerWord(p.peek()) {
		p.i++
		p.query.Domain = true
	}
	if (p.peek() == "a" || p.peek() == "an") && p.i+1 < len(p.tokens) {
		if _, ok := p.catalog.resolve(p.tokens[p.i+1].text); ok {
			p.i++
			u, e := p.unit()
			if e != nil {
				return nil, e
			}
			for k, v := range u {
				u[k] = -v
			}
			rateMonth(u)
			p.query.Domain = true
			return p.quantityWith(n, u)
		}
	}
	if (p.peek() == "/" || p.peek() == "per") && p.i+1 < len(p.tokens) {
		if _, ok := p.catalog.resolve(p.tokens[p.i+1].text); ok {
			p.i++
			u, e := p.unit()
			if e != nil {
				return nil, e
			}
			for k, v := range u {
				u[k] = -v
			}
			rateMonth(u)
			p.query.Domain = true
			return p.quantityWith(n, u)
		}
	}
	if _, ok := p.catalog.resolve(p.peek()); !ok {
		if (p.peek() == "fl" && p.i+1 < len(p.tokens) && p.tokens[p.i+1].text == "oz") ||
			(p.peek() == "work" && p.i+1 < len(p.tokens) && (p.tokens[p.i+1].text == "day" || p.tokens[p.i+1].text == "days")) ||
			(p.peek() == "imperial" && p.i+1 < len(p.tokens) && (p.tokens[p.i+1].text == "pint" || p.tokens[p.i+1].text == "pints")) ||
			((p.peek() == "cubic" || p.peek() == "square") && p.i+1 < len(p.tokens)) {
			u, e := p.unit()
			if e != nil {
				return nil, e
			}
			p.query.Domain = true
			n, e = p.make(&node{op: "quantity", args: []*node{n}, value: Value{Unit: u}})
			if e != nil {
				return nil, e
			}
			return p.suffix(n)
		}
		return n, nil
	}
	// in followed by a unit is a conversion preposition, not an inch suffix.
	if p.peek() == "in" {
		next := p.tokens[p.i+1]
		if _, ok := p.catalog.resolve(next.text); ok || isFormatWord(next.text) {
			return n, nil
		}
		if isYearToken(next) || isTaxCountry(next.text) {
			return n, nil
		}
	}
	u, e := p.unit()
	if e != nil {
		return nil, e
	}
	p.query.Domain = true
	n, e = p.make(&node{op: "quantity", args: []*node{n}, value: Value{Unit: u}})
	if e != nil {
		return nil, e
	}
	if p.peek() == "and" && p.i+1 < len(p.tokens) && p.tokens[p.i+1].value != nil {
		saved := p.i
		p.i++
		rhs, err := p.primary()
		if err == nil {
			return p.make(&node{op: "+", args: []*node{n, rhs}})
		}
		p.i = saved
	}
	if len(u) == 1 && u["ft"] == 1 && p.tokens[p.i].value != nil {
		saved := p.i
		t := p.tokens[p.i]
		p.i++
		inch, err := p.unit()
		if err == nil && len(inch) == 1 && inch["in"] == 1 {
			part, err := p.make(&node{op: "quantity", args: []*node{{op: "value", value: Value{Kind: Number, Number: t.value}}}, value: Value{Unit: inch}})
			if err == nil {
				p.query.format = "ftin"
				return p.make(&node{op: "+", args: []*node{n, part}})
			}
		}
		p.i = saved
	}
	angle := isAngleUnit(u)
	// Keep compound durations in one operand so subtraction negates every part.
	for e == nil && (p.catalog.isDurationUnit(u) || isMonthUnit(u) || isFrameUnit(u) || angle) && p.tokens[p.i].value != nil {
		t := p.tokens[p.i]
		p.i++
		if angle {
			u, e = p.arcUnit()
		} else {
			u, e = p.unit()
		}
		if e != nil {
			return nil, e
		}
		if angle {
			if !isAngleUnit(u) {
				return nil, invalid("compound angles require angle units")
			}
		} else if !p.catalog.isDurationUnit(u) && !isMonthUnit(u) && !isFrameUnit(u) {
			return nil, invalid("compound durations require time units")
		}
		part, err := p.make(&node{op: "quantity", args: []*node{{op: "value", value: Value{Kind: Number, Number: t.value}}}, value: Value{Unit: u}})
		if err != nil {
			return nil, err
		}
		n, e = p.make(&node{op: "+", args: []*node{n, part}})
		if !angle {
			p.query.compoundTime = true
		} else {
			p.query.compoundAngle = true
		}
	}
	return n, e
}

func isAngleUnit(u Unit) bool {
	if len(u) != 1 {
		return false
	}
	return u["deg"] != 0 || u["arcmin"] != 0 || u["arcsec"] != 0 || u["rad"] != 0
}

func (p *parser) arcUnit() (Unit, error) {
	switch p.peek() {
	case "minute", "minutes", "′", "'":
		p.i++
		return Unit{"arcmin": 1}, nil
	case "second", "seconds", "″", "\"":
		p.i++
		return Unit{"arcsec": 1}, nil
	}
	return p.unit()
}

// isDurationUnit excludes compound dimensions and calendar months.
func (c *Catalog) isDurationUnit(u Unit) bool {
	if len(u) != 1 {
		return false
	}
	for symbol, exponent := range u {
		return exponent == 1 && (symbol == "?m" || c.Units[symbol].Dimension == "time")
	}
	return false
}

func isMonthUnit(u Unit) bool {
	return len(u) == 1 && u["month"] == 1
}

func rateMonth(u Unit) {
	if n := u["month"]; n != 0 {
		delete(u, "month")
		u["mo"] += n
	}
}

func isFrameUnit(u Unit) bool {
	return len(u) == 1 && u["frame"] == 1
}

// unit consumes unit factors only; numeric multiplication remains expression syntax.
func (p *parser) unit() (Unit, error) {
	power := 1
	if p.peek() == "cubic" {
		power = 3
		p.i++
	} else if p.peek() == "square" {
		power = 2
		p.i++
	}
	if p.peek() == "work" && p.i+1 < len(p.tokens) && (p.tokens[p.i+1].text == "day" || p.tokens[p.i+1].text == "days") {
		p.i += 2
		return Unit{"workday": 1}, nil
	}
	if p.peek() == "fl" && p.i+1 < len(p.tokens) && p.tokens[p.i+1].text == "oz" {
		p.i += 2
		return Unit{"floz": 1}, nil
	}
	if p.peek() == "imperial" && p.i+1 < len(p.tokens) && (p.tokens[p.i+1].text == "pint" || p.tokens[p.i+1].text == "pints") {
		p.i += 2
		return Unit{"ipt": 1}, nil
	}
	u := Unit{}
	sign := 1
	for {
		symbol, ok := p.catalog.resolve(p.peek())
		if !ok {
			return nil, invalid("expected unit")
		}
		p.i++
		if symbol == "month" && sign < 0 {
			symbol = "mo"
		}
		exponent := 1
		if p.accept("²") {
			exponent = 2
		} else if p.accept("³") {
			exponent = 3
		} else if p.accept("^") {
			neg := p.accept("-")
			t := p.tokens[p.i]
			if t.value == nil || !t.value.IsInt() || !t.value.Num().IsInt64() {
				return nil, invalid("expected unit exponent")
			}
			exponent = int(t.value.Num().Int64())
			p.i++
			if neg {
				exponent = -exponent
			}
		}
		exponent *= power
		power = 1
		if exponent < -16 || exponent > 16 {
			return nil, invalid("unit exponent exceeds limit")
		}
		u[symbol] += sign * exponent
		if symbol == "Mbps" {
			u["s"] -= sign * exponent
		}
		p.query.Crypto = p.query.Crypto || p.catalog.Crypto[symbol]
		p.query.Money = p.query.Money || p.catalog.Units[symbol].Dimension == "money"
		if (p.peek() == "/" || p.peek() == "*") && p.i+1 < len(p.tokens) {
			if _, ok := p.catalog.resolve(p.tokens[p.i+1].text); ok {
				sign = 1
				if p.peek() == "/" {
					sign = -1
				}
				p.i++
				continue
			}
		}
		break
	}
	return u, nil
}
func isBase(s string) bool { return s == "hex" || s == "bin" || s == "oct" || s == "dec" }

func isYearToken(t token) bool {
	if t.value == nil || !t.value.IsInt() || !t.value.Num().IsInt64() {
		return false
	}
	y := t.value.Num().Int64()
	return y >= 1000 && y <= 9999
}

// scanLaptime reads HH:MM:SS[.ms]. Two colons distinguish it from a clock.
func scanLaptime(runes []rune) (string, *big.Rat, bool) {
	i := 0
	for i < len(runes) && unicode.IsDigit(runes[i]) {
		i++
	}
	if i == 0 || i >= len(runes) || runes[i] != ':' {
		return "", nil, false
	}
	if i+2 >= len(runes) || !unicode.IsDigit(runes[i+1]) || !unicode.IsDigit(runes[i+2]) {
		return "", nil, false
	}
	if i+3 >= len(runes) || runes[i+3] != ':' {
		return "", nil, false
	}
	if i+5 >= len(runes) || !unicode.IsDigit(runes[i+4]) || !unicode.IsDigit(runes[i+5]) {
		return "", nil, false
	}
	end := i + 6
	if end < len(runes) && runes[end] == ':' {
		return "", nil, false
	}
	if end < len(runes) && runes[end] == '.' {
		frac := end + 1
		for frac < len(runes) && unicode.IsDigit(runes[frac]) {
			frac++
		}
		if frac == end+1 {
			return "", nil, false
		}
		end = frac
	}
	if trailingAmpm(runes[end:]) {
		return "", nil, false
	}
	text := string(runes[:end])
	parts := strings.SplitN(text, ":", 3)
	hours, ok1 := new(big.Rat).SetString(parts[0])
	mins, ok2 := new(big.Rat).SetString(parts[1])
	secs, ok3 := new(big.Rat).SetString(parts[2])
	if !ok1 || !ok2 || !ok3 || mins.Cmp(big.NewRat(60, 1)) >= 0 || secs.Cmp(big.NewRat(60, 1)) >= 0 {
		return "", nil, false
	}
	total := new(big.Rat).Mul(hours, big.NewRat(3600, 1))
	total.Add(total, new(big.Rat).Mul(mins, big.NewRat(60, 1)))
	total.Add(total, secs)
	return text, total, true
}

func trailingAmpm(runes []rune) bool {
	s := strings.ToLower(strings.TrimSpace(string(runes)))
	s = strings.ReplaceAll(s, ".", "")
	return s == "am" || s == "pm" || strings.HasPrefix(s, "am ") || strings.HasPrefix(s, "pm ")
}

// parseRateAt consumes "at $30/hour" after a duration or amount.
func (p *parser) parseRateAt() error {
	if p.peek() != "at" && p.peek() != "@" {
		return nil
	}
	if p.i+2 < len(p.tokens) && p.tokens[p.i+1].value != nil && (p.tokens[p.i+2].text == "x" || p.tokens[p.i+2].text == "fps") {
		return nil
	}
	if !p.accept("at") {
		p.accept("@")
	}
	rate, err := p.expr(0)
	if err != nil {
		return err
	}
	p.query.Domain = true
	p.query.root, err = p.make(&node{op: "rate_at", args: []*node{p.query.root, rate}})
	return err
}

// parseForPeriod consumes "for a year" after a rate, multiplying by that period.
func (p *parser) parseForPeriod() error {
	if !p.accept("for") {
		return nil
	}
	p.accept("a")
	p.accept("an")
	u, err := p.unit()
	if err != nil {
		return err
	}
	one, err := p.make(&node{op: "value", value: Value{Kind: Number, Number: big.NewRat(1, 1)}})
	if err != nil {
		return err
	}
	period, err := p.make(&node{op: "quantity", args: []*node{one}, value: Value{Unit: u}})
	if err != nil {
		return err
	}
	p.query.Domain = true
	p.query.root, err = p.make(&node{op: "*", args: []*node{p.query.root, period}})
	return err
}

// parsePlayback consumes "at 1.5x" after a duration.
func (p *parser) parsePlayback() error {
	if !p.accept("at") {
		if p.query.timeSaved {
			return invalid("expected at")
		}
		return nil
	}
	t := p.tokens[p.i]
	if t.value == nil || t.value.Sign() <= 0 {
		return invalid("expected positive speed")
	}
	p.i++
	if !p.accept("x") {
		return invalid("expected x")
	}
	p.query.speed = new(big.Rat).Set(t.value)
	p.query.Domain = true
	return nil
}

// parseUnitParts reads "minutes and seconds" after the first unit and in/as/to.
func (p *parser) parseUnitParts(first Unit) error {
	symbol, err := p.singleTimeUnit(first)
	if err != nil {
		return err
	}
	p.query.format = "parts"
	p.query.formatUnits = []string{symbol}
	for {
		u, e := p.unit()
		if e != nil {
			return e
		}
		symbol, e = p.singleTimeUnit(u)
		if e != nil {
			return e
		}
		p.query.formatUnits = append(p.query.formatUnits, symbol)
		if !p.accept("and") {
			return nil
		}
	}
}

func (p *parser) singleTimeUnit(u Unit) (string, error) {
	if len(u) != 1 {
		return "", invalid("expected a time unit")
	}
	for symbol, exponent := range u {
		if exponent != 1 {
			return "", invalid("expected a time unit")
		}
		if symbol != "?m" && p.catalog.Units[symbol].Dimension != "time" {
			return "", invalid("expected a time unit")
		}
		return symbol, nil
	}
	return "", invalid("expected a time unit")
}

// looksLikeInverted detects "meters in 10 km": a leading unit, then in/to.
func (p *parser) looksLikeInverted() bool {
	start := p.i
	crypto, money := p.query.Crypto, p.query.Money
	_, err := p.unit()
	ok := err == nil && (p.peek() == "in" || p.peek() == "to")
	p.i = start
	p.query.Crypto, p.query.Money = crypto, money
	return ok
}

func (p *parser) looksLikeUnitPair() bool {
	start := p.i
	crypto, money := p.query.Crypto, p.query.Money
	_, err1 := p.unit()
	_, err2 := p.unit()
	ok := err1 == nil && err2 == nil && p.peek() == ""
	p.i = start
	p.query.Crypto, p.query.Money = crypto, money
	return ok
}

func (p *parser) parseUnitPair() error {
	from, err := p.unit()
	if err != nil {
		return err
	}
	to, err := p.unit()
	if err != nil {
		return err
	}
	one, err := p.make(&node{op: "value", value: number(1)})
	if err != nil {
		return err
	}
	p.query.Domain = true
	p.query.target = to
	p.query.root, err = p.make(&node{op: "quantity", args: []*node{one}, value: Value{Unit: from}})
	return err
}

// parseInverted reads how-many-X-in-Y, including "seconds in a day".
func (p *parser) parseInverted() error {
	target, err := p.unit()
	if err != nil {
		return err
	}
	if !p.accept("in") && !p.accept("to") {
		return invalid("expected in")
	}
	p.query.Domain = true
	p.query.target = target
	if p.accept("a") || p.accept("an") {
		u, err := p.unit()
		if err != nil {
			return err
		}
		n, err := p.make(&node{op: "value", value: number(1)})
		if err != nil {
			return err
		}
		p.query.root, err = p.make(&node{op: "quantity", args: []*node{n}, value: Value{Unit: u}})
		return err
	}
	p.query.root, err = p.expr(0)
	return err
}

func isDecimalPlacesWord(s string) bool {
	return s == "dp" || s == "dps" || s == "digit" || s == "digits"
}

// tryRounding recognizes "N dp" / "N digits" after to. These request output
// precision, so they must be claimed before unit() treats the number as a unit.
func (p *parser) tryRounding() (bool, error) {
	t := p.tokens[p.i]
	if t.value == nil || !t.value.IsInt() || t.value.Sign() < 0 || !t.value.Num().IsInt64() {
		return false, nil
	}
	word := ""
	if p.i+1 < len(p.tokens) {
		word = p.tokens[p.i+1].text
	}
	if !isDecimalPlacesWord(word) {
		return false, nil
	}
	n := t.value.Num().Int64()
	if n > 64 {
		return true, invalid("decimal places exceed limit")
	}
	places := int(n)
	p.query.decimals = &places
	p.i += 2
	return true, nil
}

// parseNearest consumes "to nearest N" after rounded/up/down.
func (p *parser) parseNearest(dir string) error {
	if !p.accept("to") || !p.accept("nearest") {
		return invalid("expected to nearest")
	}
	return p.finishNearest(dir)
}

// finishNearest reads the positive step after the word nearest.
func (p *parser) finishNearest(dir string) error {
	t := p.tokens[p.i]
	if t.value != nil && p.i+1 < len(p.tokens) {
		ord := p.tokens[p.i+1].text
		if ord == "th" || ord == "st" || ord == "nd" || ord == "rd" {
			if t.value.Sign() <= 0 {
				return invalid("expected positive step")
			}
			p.i += 2
			p.query.nearest = new(big.Rat).Inv(new(big.Rat).Set(t.value))
			p.query.roundDir = dir
			p.query.format = "fraction"
			return nil
		}
	}
	if scale := wordScale(t.text); scale != nil {
		p.i++
		p.query.nearest = scale
		p.query.roundDir = dir
		return nil
	}
	if t.text != "" && strings.HasSuffix(t.text, "th") && t.value == nil {
		// "16th" arrives as a word; recover the denominator from the prefix.
		n, ok := new(big.Rat).SetString(strings.TrimSuffix(t.text, "th"))
		if !ok || n.Sign() <= 0 {
			return invalid("expected positive step")
		}
		p.i++
		p.query.nearest = new(big.Rat).Inv(n)
		p.query.roundDir = dir
		p.query.format = "fraction"
		return nil
	}
	if t.value == nil || t.value.Sign() <= 0 {
		return invalid("expected positive step")
	}
	p.i++
	p.query.nearest = new(big.Rat).Set(t.value)
	p.query.roundDir = dir
	return nil
}

// parseBase interprets digits using the explicitly selected base.
func parseBase(s, base string) (*big.Rat, error) {
	radix := map[string]int{"hex": 16, "bin": 2, "oct": 8, "dec": 10}[base]
	n, ok := new(big.Int).SetString(s, radix)
	if !ok {
		return nil, invalid("invalid base number")
	}
	return new(big.Rat).SetInt(n), nil
}

// resolveAmbiguity only resolves the known m alias. It does not infer arbitrary dimensions.
func isMeterProduct(n *node) bool {
	if n == nil || n.op != "*" || len(n.args) != 2 {
		return false
	}
	onlyM := func(x *node) bool {
		return x != nil && x.op == "quantity" && len(x.value.Unit) == 1 && x.value.Unit["?m"] != 0
	}
	return onlyM(n.args[0]) && onlyM(n.args[1])
}

func (c *Catalog) resolveAmbiguity(q *Query) error {
	length, duration := false, false
	inspect := func(u Unit) {
		for k := range u {
			if k == "?m" {
				if u["s"] < 0 {
					length = true
				}
				continue
			}
			d := c.Units[k].Dimension
			length = length || d == "length"
			duration = duration || d == "time" && len(u) == 1
		}
	}
	inspect(q.target)
	if isMeterProduct(q.root) {
		length = true
	}
	var walk func(*node)
	walk = func(n *node) {
		if n == nil {
			return
		}
		inspect(n.value.Unit)
		for _, a := range n.args {
			walk(a)
		}
	}
	walk(q.root)
	var has bool
	var replace func(Unit)
	replace = func(u Unit) {
		if n, ok := u["?m"]; ok {
			has = true
			delete(u, "?m")
			k := "min"
			if length {
				k = "m"
			}
			u[k] += n
		}
	}
	replace(q.target)
	walk = func(n *node) {
		if n == nil {
			return
		}
		replace(n.value.Unit)
		for _, a := range n.args {
			walk(a)
		}
	}
	walk(q.root)
	for i, symbol := range q.formatUnits {
		if symbol != "?m" {
			continue
		}
		has = true
		k := "min"
		if length {
			k = "m"
		}
		q.formatUnits[i] = k
	}
	if has && length && duration {
		return invalid("ambiguous m: use meters or minutes")
	}
	return nil
}
