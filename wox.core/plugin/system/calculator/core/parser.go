package core

import (
	"context"
	"fmt"
	"strings"
	"wox/util"

	"github.com/shopspring/decimal"
)

// NodeKind represents the type of AST node
type NodeKind string

const (
	AddNode   NodeKind = "+"
	SubNode   NodeKind = "-"
	MulNode   NodeKind = "*"
	DivNode   NodeKind = "/"
	FuncNode  NodeKind = "func"
	NumNode   NodeKind = "num"
	IdentNode NodeKind = "ident"
)

// Node represents a node in the AST
type Node struct {
	Kind     NodeKind
	Left     *Node
	Right    *Node
	FuncName string
	Args     []*Node
	Val      decimal.Decimal
	Str      string // Used for identifiers
}

// Result represents a calculation result
type Result struct {
	// The display value that will be shown to user
	DisplayValue string
	// The raw value that will be used for calculation (optional)
	RawValue *decimal.Decimal
	// The unit of the result (optional)
	Unit string
}

type Parser struct {
	tokens []Token
	i      int
	log    *util.Log
}

func NewParser(tokens []Token) *Parser {
	return &Parser{
		tokens: tokens,
		i:      0,
		log:    util.GetLogger(),
	}
}

func (p *Parser) numberNode(ctx context.Context) (*Node, error) {
	if p.i >= len(p.tokens) {
		return nil, fmt.Errorf("unexpected end of input")
	}
	t := p.tokens[p.i]
	if t.Kind != NumberToken {
		return nil, fmt.Errorf("expected a number: %s", t.Str)
	}
	p.i++
	return &Node{Kind: NumNode, Val: t.Val}, nil
}

func (p *Parser) identNode(ctx context.Context) (*Node, error) {
	if p.i >= len(p.tokens) {
		return nil, fmt.Errorf("unexpected end of input")
	}
	t := p.tokens[p.i]
	if t.Kind != IdentToken {
		return nil, fmt.Errorf("expected an identifier: %s", t.Str)
	}
	p.i++
	return &Node{Kind: IdentNode, Str: strings.ToLower(t.Str)}, nil
}

func (p *Parser) consume(ctx context.Context, s string) bool {
	if p.i >= len(p.tokens) {
		return false
	}
	t := p.tokens[p.i]
	if t.Kind != ReservedToken || t.Str != s {
		return false
	}
	p.i++
	return true
}

func (p *Parser) Parse(ctx context.Context) (*Node, error) {
	if len(p.tokens) == 0 {
		return nil, fmt.Errorf("empty input")
	}
	p.log.Debug(ctx, fmt.Sprintf("Parser.Parse: tokens=%+v", p.tokens))
	node, err := p.add(ctx)
	if err != nil {
		p.log.Error(ctx, fmt.Sprintf("Parser.Parse: add error=%v", err))
		return nil, err
	}
	p.log.Debug(ctx, fmt.Sprintf("Parser.Parse: node=%+v", node))
	return node, nil
}

func (p *Parser) insert(ctx context.Context, n *Node, f func(context.Context) (*Node, error), kind NodeKind) (*Node, error) {
	left := n
	right, err := f(ctx)
	if err != nil {
		return n, err
	}
	return &Node{Kind: kind, Left: left, Right: right}, err
}

func (p *Parser) add(ctx context.Context) (*Node, error) {
	n, err := p.mul(ctx)
	if err != nil {
		return nil, err
	}

	for p.i < len(p.tokens) {
		if p.consume(ctx, "+") {
			n, err = p.insert(ctx, n, p.mul, AddNode)
			if err != nil {
				return nil, err
			}
		} else if p.consume(ctx, "-") {
			n, err = p.insert(ctx, n, p.mul, SubNode)
			if err != nil {
				return nil, err
			}
		} else {
			return n, nil
		}
	}
	return n, nil
}

func (p *Parser) mul(ctx context.Context) (*Node, error) {
	n, err := p.unary(ctx)
	if err != nil {
		return nil, err
	}

	for p.i < len(p.tokens) {
		if p.consume(ctx, "*") {
			n, err = p.insert(ctx, n, p.unary, MulNode)
			if err != nil {
				return nil, err
			}
		} else if p.consume(ctx, "/") {
			n, err = p.insert(ctx, n, p.unary, DivNode)
			if err != nil {
				return nil, err
			}
		} else {
			return n, nil
		}
	}
	return n, nil
}

func (p *Parser) unary(ctx context.Context) (*Node, error) {
	if p.consume(ctx, "+") {
		return p.primary(ctx)
	} else if p.consume(ctx, "-") {
		return p.insert(ctx, &Node{Kind: NumNode, Val: decimal.Zero}, p.primary, SubNode)
	}
	return p.primary(ctx)
}

func (p *Parser) primary(ctx context.Context) (*Node, error) {
	if p.consume(ctx, "(") {
		n, err := p.add(ctx)
		if err != nil {
			return nil, err
		}
		if !p.consume(ctx, ")") {
			return nil, fmt.Errorf("expected closing parenthesis")
		}
		return n, nil
	}

	if p.i >= len(p.tokens) {
		return nil, fmt.Errorf("unexpected end of input")
	}

	if p.tokens[p.i].Kind == IdentToken {
		ident, err := p.identNode(ctx)
		if err != nil {
			return nil, err
		}

		// Check if this is a function call
		if p.i < len(p.tokens) && p.tokens[p.i].Kind == ReservedToken && p.tokens[p.i].Str == "(" {
			// Consume the opening parenthesis
			p.i++

			// Parse function arguments
			var args []*Node
			for {
				if p.i >= len(p.tokens) {
					return nil, fmt.Errorf("unexpected end of input in function call")
				}

				// Check for empty argument list or end of arguments
				if p.tokens[p.i].Kind == ReservedToken && p.tokens[p.i].Str == ")" {
					p.i++ // Consume the closing parenthesis
					break
				}

				// Parse the argument
				arg, err := p.add(ctx)
				if err != nil {
					return nil, err
				}
				args = append(args, arg)

				// Check for comma or closing parenthesis
				if p.i >= len(p.tokens) {
					return nil, fmt.Errorf("unexpected end of input in function call")
				}
				if p.tokens[p.i].Kind == ReservedToken {
					if p.tokens[p.i].Str == ")" {
						p.i++ // Consume the closing parenthesis
						break
					} else if p.tokens[p.i].Str == "," {
						p.i++ // Consume the comma
						continue
					}
				}
				return nil, fmt.Errorf("expected ',' or ')' in function call")
			}

			return &Node{
				Kind:     FuncNode,
				FuncName: strings.ToLower(ident.Str),
				Args:     args,
			}, nil
		}

		return ident, nil
	}
	return p.numberNode(ctx)
}
