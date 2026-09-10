package engine

import (
	"encoding/base64"
	"strings"
	"unicode"
	"unicode/utf8"
)

const minAutoBase64Len = 8

// parseBase64Query recognizes a whole-query base64 literal or an explicit
// encode/decode sentence. It does not compete with other converter syntax.
func parseBase64Query(input string) (*Query, bool) {
	if payload, ok := trimKnownPrefix(input, "base64 encode ", "encode base64 "); ok {
		return textQuery(input, encodeBase64(payload), "base64"), true
	}
	if payload, ok := trimKnownSuffix(input, " to base64", " as base64"); ok {
		return textQuery(input, encodeBase64(payload), "base64"), true
	}
	if payload, ok := trimKnownPrefix(input, "base64 decode ", "decode base64 ", "base64 "); ok {
		text, ok := decodeBase64Text(compactBase64(payload), false)
		if !ok {
			return nil, false
		}
		return textQuery(input, text, "text"), true
	}
	if payload, ok := trimKnownSuffix(input, " to text", " as text", " to utf8", " as utf8", " to utf-8", " as utf-8", " from base64"); ok {
		text, ok := decodeBase64Text(compactBase64(payload), false)
		if !ok {
			return nil, false
		}
		return textQuery(input, text, "text"), true
	}
	if text, ok := decodeBase64Text(input, true); ok {
		return textQuery(input, text, "text"), true
	}
	return nil, false
}

func textQuery(expr, text, format string) *Query {
	return &Query{
		Expression: expr,
		Domain:     true,
		format:     format,
		root:       &node{op: "value", value: Value{Kind: Text, Text: text}},
	}
}

func encodeBase64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// decodeBase64Text accepts standard or URL-safe base64. Auto-detect stays
// conservative so ordinary words and calculator fragments are not stolen.
func decodeBase64Text(s string, auto bool) (string, bool) {
	if auto && !looksLikeAutoBase64(s) {
		return "", false
	}
	if !auto && !looksLikeBase64Token(s) {
		return "", false
	}
	raw, ok := decodeBase64Bytes(s)
	if !ok || !isDecodedText(raw) {
		return "", false
	}
	return string(raw), true
}

// looksLikeAutoBase64 requires a long enough token with a letter so short
// words and digit-only fragments do not become accidental decode results.
func looksLikeAutoBase64(s string) bool {
	if len(s) < minAutoBase64Len || !looksLikeBase64Token(s) {
		return false
	}
	hasLetter := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
			break
		}
	}
	return hasLetter && !looksLikeArithmeticBase64(s)
}

// looksLikeBase64Token accepts a single standard or URL-safe token, including
// optional end padding. Spaces and mid-string '=' are rejected.
func looksLikeBase64Token(s string) bool {
	if s == "" {
		return false
	}
	padding := 0
	for i, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '+', r == '/', r == '-', r == '_':
			if padding > 0 {
				return false
			}
		case r == '=':
			padding++
			if padding > 2 {
				return false
			}
		default:
			return false
		}
		if padding > 0 && i == 0 {
			return false
		}
	}
	if padding > 0 && len(s)%4 != 0 {
		return false
	}
	return true
}

// looksLikeArithmeticBase64 rejects digit/operator strings such as 100+100=
// that happen to be valid base64 alphabet.
func looksLikeArithmeticBase64(s string) bool {
	body := strings.TrimRight(s, "=")
	if body == "" {
		return false
	}
	sawDigit := false
	sawOp := false
	for _, r := range body {
		if r >= '0' && r <= '9' {
			sawDigit = true
			continue
		}
		if (r == '+' || r == '-' || r == '*' || r == '/') && sawDigit {
			sawOp = true
			sawDigit = false
			continue
		}
		return false
	}
	return sawOp && sawDigit
}

// decodeBase64Bytes tries padded and raw alphabets. URL-safe is used only when
// the token contains - or _, so + / keep their standard meaning.
func decodeBase64Bytes(s string) ([]byte, bool) {
	var encodings []*base64.Encoding
	switch {
	case strings.ContainsAny(s, "-_"):
		encodings = []*base64.Encoding{base64.URLEncoding, base64.RawURLEncoding}
	case strings.ContainsAny(s, "+/"):
		encodings = []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding}
	default:
		encodings = []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding}
	}
	for _, enc := range encodings {
		raw, err := enc.DecodeString(s)
		if err == nil {
			return raw, true
		}
	}
	return nil, false
}

// isDecodedText keeps binary payloads out of the result list. Printable UTF-8
// also filters most ordinary English words that merely look like base64.
func isDecodedText(raw []byte) bool {
	if len(raw) == 0 || !utf8.Valid(raw) {
		return false
	}
	if strings.TrimSpace(string(raw)) == "" {
		return false
	}
	for _, r := range string(raw) {
		if r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

func compactBase64(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

func trimKnownPrefix(s string, prefixes ...string) (string, bool) {
	for _, prefix := range prefixes {
		if len(s) > len(prefix) && strings.EqualFold(s[:len(prefix)], prefix) {
			rest := strings.TrimSpace(s[len(prefix):])
			if rest != "" {
				return rest, true
			}
		}
	}
	return "", false
}

func trimKnownSuffix(s string, suffixes ...string) (string, bool) {
	for _, suffix := range suffixes {
		if len(s) > len(suffix) && strings.EqualFold(s[len(s)-len(suffix):], suffix) {
			rest := strings.TrimSpace(s[:len(s)-len(suffix)])
			if rest != "" {
				return rest, true
			}
		}
	}
	return "", false
}
