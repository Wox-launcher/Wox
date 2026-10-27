package markdown

import (
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// NewParser creates Wox's GFM parser with support for punctuation-adjacent bold text.
func NewParser() parser.Parser {
	return goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithInlineParsers(util.Prioritized(emphasisParser{}, 499))),
	).Parser()
}

// emphasisParser accepts bold spans ending in non-ASCII punctuation immediately
// followed by text, without modifying the source or code spans.
type emphasisParser struct{}

func (emphasisParser) Trigger() []byte { return []byte{'*', '_'} }

func (emphasisParser) Parse(parent ast.Node, reader text.Reader, context parser.Context) ast.Node {
	before := reader.PrecendingCharacter()
	line, _ := reader.PeekLine()
	node := parser.NewEmphasisParser().Parse(parent, reader, context)
	if delimiter, ok := node.(*parser.Delimiter); ok && delimiter.Char == '*' && delimiter.OriginalLength == 2 && !delimiter.CanClose && before > 127 && unicode.IsPunct(before) {
		after, _ := utf8.DecodeRune(line[delimiter.OriginalLength:])
		if unicode.IsLetter(after) || unicode.IsNumber(after) {
			delimiter.CanClose = true
		}
	}
	return node
}
