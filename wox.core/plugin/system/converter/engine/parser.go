package engine

import (
	"math/big"
	"strings"
	"unicode"
	"wox/util/calc"
)

type token struct {
	text     string
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
	root       *node
	temporal   *temporalQuery
	target     Unit
	format     string
	ppi        *big.Rat
	Expression string
	Domain     bool
	Crypto     bool
	Money      bool
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
			n, ok := new(big.Rat).SetString(s)
			if !ok {
				return nil, invalid("invalid number")
			}
			if n.Num().BitLen() > maxBits || n.Denom().BitLen() > maxBits {
				return nil, invalid("number exceeds limit")
			}
			tokens = append(tokens, token{text: string(runes[start:i]), value: n, pos: start})
			continue
		}
		if unicode.IsLetter(runes[i]) || runes[i] == '°' {
			i++
			for i < len(runes) && (unicode.IsLetter(runes[i]) || runes[i] == '_') {
				i++
			}
			tokens = append(tokens, token{text: strings.ToLower(string(runes[start:i])), pos: start})
			continue
		}
		if strings.ContainsRune("+-*/^(),;%=?$€£¥²³", runes[i]) {
			tokens = append(tokens, token{text: string(runes[i]), pos: i})
			i++
			continue
		}
		return nil, &Error{Kind: Unrecognized, Position: i, Message: "unknown character"}
	}
	tokens = append(tokens, token{pos: len(runes)})
	return tokens, nil
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
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, &Error{Kind: Unrecognized, Message: "empty input"}
	}
	q := &Query{Expression: input}
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
	q.root, err = p.expr(0)
	if err != nil {
		return nil, err
	}
	if p.accept("to") || p.accept("in") || p.accept("=") {
		q.Domain = true
		if p.tokens[p.i-1].text == "=" && !p.accept("?") {
			return nil, invalid("expected ?")
		}
		if strings.Contains("|timespan|bin|oct|dec|hex|", "|"+p.peek()+"|") && p.peek() != "" {
			q.format = p.peek()
			p.i++
		} else {
			q.target, err = p.unit()
			if err != nil {
				return nil, err
			}
		}
		if p.accept("at") {
			t := p.tokens[p.i]
			if t.value == nil || t.value.Sign() <= 0 {
				return nil, invalid("expected positive PPI")
			}
			q.ppi = t.value
			p.i++
			if !p.accept("ppi") {
				return nil, invalid("expected ppi")
			}
		}
	}
	if p.peek() != "" {
		return nil, &Error{Kind: Invalid, Position: p.tokens[p.i].pos, Message: "unexpected " + p.peek()}
	}
	if err = c.resolveAmbiguity(q); err != nil {
		return nil, err
	}
	return q, nil
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
		op := p.peek()
		precedence := 0
		switch op {
		case "+", "-":
			precedence = 10
		case "*", "/", "of", "off", "tip":
			precedence = 20
		case "^", "power":
			precedence = 30
		case "%":
			precedence = 40
		}
		if precedence == 0 || precedence < min {
			break
		}
		p.i++
		if op == "%" {
			p.query.Domain = true
			left, err = p.make(&node{op: "percent", args: []*node{left}})
			if err != nil {
				return nil, err
			}
			continue
		}
		if op == "power" || op == "of" || op == "off" || op == "tip" {
			p.query.Domain = true
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
	if p.accept("square") {
		p.query.Domain = true
		if !p.accept("root") || !p.accept("of") {
			return nil, invalid("expected root of")
		}
		n, e := p.expr(25)
		if e != nil {
			return nil, e
		}
		return p.make(&node{op: "function", text: "sqrt", args: []*node{n}})
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
	if t.temporal != nil {
		p.i++
		p.query.Domain = true
		return p.make(&node{op: "temporal", temporal: t.temporal})
	}
	if isBase(t.text) {
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
	if t.text == "pi" || t.text == "e" {
		p.i++
		v := rational("3.141592653589793")
		if t.text == "e" {
			v = rational("2.718281828459045")
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
		return p.make(&node{op: "quantity", value: Value{Unit: Unit{symbol: 1}}, args: []*node{n}})
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
	return p.suffix(n)
}

// magnitude recognizes attached K while leaving spaced temperature units intact.
func (p *parser) magnitude(n *node) (*node, error) {
	// K is a magnitude only when attached to the preceding numeric token.
	if p.peek() == "k" && p.i > 0 && p.tokens[p.i].pos == p.tokens[p.i-1].pos+len([]rune(p.tokens[p.i-1].text)) {
		p.i++
		p.query.Domain = true
		return p.make(&node{op: "*", args: []*node{n, {op: "value", value: number(1000)}}})
	}
	return n, nil
}

// suffix distinguishes an inch suffix from the conversion preposition in.
func (p *parser) suffix(n *node) (*node, error) {
	if _, ok := p.catalog.resolve(p.peek()); !ok {
		return n, nil
	}
	// in followed by a unit is a conversion preposition, not an inch suffix.
	if p.peek() == "in" {
		if _, ok := p.catalog.resolve(p.tokens[p.i+1].text); ok || p.tokens[p.i+1].text == "timespan" {
			return n, nil
		}
	}
	u, e := p.unit()
	if e != nil {
		return nil, e
	}
	p.query.Domain = true
	n, e = p.make(&node{op: "quantity", args: []*node{n}, value: Value{Unit: u}})
	// Keep compound durations in one operand so subtraction negates every part.
	for e == nil && p.catalog.isDurationUnit(u) && p.tokens[p.i].value != nil {
		t := p.tokens[p.i]
		p.i++
		u, e = p.unit()
		if e != nil {
			return nil, e
		}
		if !p.catalog.isDurationUnit(u) {
			return nil, invalid("compound durations require time units")
		}
		part, err := p.make(&node{op: "quantity", args: []*node{{op: "value", value: Value{Kind: Number, Number: t.value}}}, value: Value{Unit: u}})
		if err != nil {
			return nil, err
		}
		n, e = p.make(&node{op: "+", args: []*node{n, part}})
	}
	return n, e
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

// unit consumes unit factors only; numeric multiplication remains expression syntax.
func (p *parser) unit() (Unit, error) {
	u := Unit{}
	sign := 1
	for {
		symbol, ok := p.catalog.resolve(p.peek())
		if !ok {
			return nil, invalid("expected unit")
		}
		p.i++
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
		if exponent < -16 || exponent > 16 {
			return nil, invalid("unit exponent exceeds limit")
		}
		u[symbol] += sign * exponent
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
func (c *Catalog) resolveAmbiguity(q *Query) error {
	length, duration := false, false
	inspect := func(u Unit) {
		for k := range u {
			if k == "?m" {
				continue
			}
			d := c.Units[k].Dimension
			length = length || d == "length"
			duration = duration || d == "time" && len(u) == 1
		}
	}
	inspect(q.target)
	var walk func(*node)
	walk = func(n *node) {
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
		replace(n.value.Unit)
		for _, a := range n.args {
			walk(a)
		}
	}
	walk(q.root)
	if has && length && duration {
		return invalid("ambiguous m: use meters or minutes")
	}
	return nil
}
