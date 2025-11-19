package calculator

import (
	"fmt"
	"math"
	"strings"

	"github.com/shopspring/decimal"
)

type nodeKind string

const (
	addNode  nodeKind = "+"
	subNode  nodeKind = "-"
	mulNode  nodeKind = "*"
	divNode  nodeKind = "/"
	powNode  nodeKind = "^"
	funcNode nodeKind = "func"
	numNode  nodeKind = "num"
)

type node struct {
	kind  nodeKind
	left  *node
	right *node

	funcName string
	args     []*node

	val decimal.Decimal
}

type parser struct {
	tokens []token
	i      int
}

func newParser(tokens []token) *parser {
	return &parser{tokens: tokens, i: 0}
}

func (p *parser) numberNode() (*node, error) {
	t := p.tokens[p.i]
	if t.kind != numberToken {
		return nil, fmt.Errorf("expected a number: %s", t.str)
	}
	p.i++
	return &node{kind: numNode, val: t.val}, nil
}

func (p *parser) constantNode(str string) (*node, error) {
	constants := map[string]float64{
		"e":   math.E,
		"pi":  math.Pi,
		"phi": math.Phi,

		"sqrt2":   math.Sqrt2,
		"sqrte":   math.SqrtE,
		"sqrtpi":  math.SqrtPi,
		"sqrtphi": math.SqrtPhi,

		"ln2":    math.Ln2,
		"log2e":  math.Log2E,
		"ln10":   math.Ln10,
		"log10e": math.Log10E,
	}
	val, ok := constants[strings.ToLower(str)]
	if !ok {
		return nil, fmt.Errorf("unknown constant: %s", str)
	}
	p.i++
	return &node{kind: numNode, val: decimal.NewFromFloat(val)}, nil
}

func argumentNumber(funcName string) (int, error) {
	f, ok := functions[funcName]
	if !ok {
		return 0, fmt.Errorf("unknown function: %s", funcName)
	}

	switch f.(type) {
	case func() float64:
		return 0, nil
	case func(float64) float64:
		return 1, nil
	case func(float64, float64) float64:
		return 2, nil
	case func(float64, float64, float64) float64:
		return 3, nil
	default:
		return 0, fmt.Errorf("invalid function: %s", funcName)
	}
}

func (p *parser) functionNode(str string) (*node, error) {
	funcName := strings.ToLower(str)
	num, err := argumentNumber(funcName)
	if err != nil {
		return nil, err
	}

	if p.consume(")") {
		if num != 0 {
			return nil, fmt.Errorf("%s should have argument(s)", funcName)
		}
		return &node{kind: funcNode, funcName: funcName}, nil
	}

	args := []*node{}

	n, err := p.add()
	if err != nil {
		return nil, err
	}
	args = append(args, n)

	for p.consume(",") {
		n, err := p.add()
		if err != nil {
			return nil, err
		}
		args = append(args, n)
	}
	if len(args) != num {
		return nil, fmt.Errorf("%s should have %d argument(s) but has %d arguments(s)",
			funcName, num, len(args))
	}
	p.consume(")")
	return &node{kind: funcNode, funcName: funcName, args: args}, nil
}

func (p *parser) consume(s string) bool {
	t := p.tokens[p.i]
	if t.kind != reservedToken || t.str != s {
		return false
	}
	p.i++
	return true
}

func (p *parser) parse() (*node, error) {
	n, err := p.add()
	if err != nil {
		return nil, err
	}
	if p.tokens[p.i].kind != eosToken {
		return nil, fmt.Errorf("unexpected token: %s", p.tokens[p.i].str)
	}
	return n, nil
}

func (p *parser) insert(n *node, f func() (*node, error), kind nodeKind) (*node, error) {
	left := n
	right, err := f()
	if err != nil {
		return n, err
	}
	return &node{kind: kind, left: left, right: right}, err
}

func (p *parser) add() (*node, error) {
	n, err := p.mul()
	if err != nil {
		return nil, err
	}

	for {
		if p.consume("+") {
			n, err = p.insert(n, p.mul, addNode)
			if err != nil {
				return nil, err
			}
		} else if p.consume("-") {
			n, err = p.insert(n, p.mul, subNode)
			if err != nil {
				return nil, err
			}
		} else {
			return n, nil
		}
	}
}

func (p *parser) mul() (*node, error) {
	n, err := p.pow()
	if err != nil {
		return nil, err
	}

	for {
		if p.consume("*") {
			n, err = p.insert(n, p.pow, mulNode)
			if err != nil {
				return nil, err
			}
		} else if p.consume("/") {
			n, err = p.insert(n, p.pow, divNode)
			if err != nil {
				return nil, err
			}
		} else {
			return n, nil
		}
	}
}

func (p *parser) pow() (*node, error) {
	n, err := p.unary()
	if err != nil {
		return nil, err
	}

	// Right associative: 2^3^2 = 2^(3^2) = 2^9 = 512
	if p.consume("^") {
		right, err := p.pow()
		if err != nil {
			return nil, err
		}
		return &node{kind: powNode, left: n, right: right}, nil
	}
	return n, nil
}

func (p *parser) unary() (*node, error) {
	if p.consume("+") {
		return p.primary()
	} else if p.consume("-") {
		return p.insert(&node{kind: numNode, val: decimal.Zero}, p.primary, subNode)
	}
	return p.primary()
}

func (p *parser) primary() (*node, error) {
	if p.consume("(") {
		n, err := p.add()
		if err != nil {
			return nil, err
		}
		p.consume(")")
		return n, nil
	}

	if p.tokens[p.i].kind == identToken {
		str := p.tokens[p.i].str
		p.i++
		if p.i < len(p.tokens) && p.consume("(") {
			return p.functionNode(str)
		}
		p.i--
		return p.constantNode(str)
	}
	return p.numberNode()
}
