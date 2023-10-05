package util

import (
	"bytes"
	"compress/gzip"
	"github.com/shopspring/decimal"
	"io/ioutil"
	"strings"
	"unicode"
)

// PadChar left-pads s with the rune r, to length n.
// If n is smaller than s, PadChar is a no-op.
func LeftPad(s string, n int, r rune) string {
	if len(s) > n {
		return s
	}
	return strings.Repeat(string(r), n-len(s)) + s
}

func JoinDecimals(a []decimal.Decimal, sep string) string {
	switch len(a) {
	case 0:
		return ""
	case 1:
		return a[0].String()
	}
	n := len(sep) * (len(a) - 1)
	for i := 0; i < len(a); i++ {
		n += len(a[i].String())
	}

	var b strings.Builder
	b.Grow(n)
	b.WriteString(a[0].String())
	for _, s := range a[1:] {
		b.WriteString(sep)
		b.WriteString(s.String())
	}
	return b.String()
}

func Compress(data []byte) []byte {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	gz.Write(data)
	gz.Flush()
	gz.Close()

	return b.Bytes()
}

func DeCompress(data []byte) []byte {
	rdata := bytes.NewReader(data)
	r, _ := gzip.NewReader(rdata)
	defer r.Close()

	s, _ := ioutil.ReadAll(r)
	return s
}

func Capitalize(str string) string {
	for i, v := range str {
		return string(unicode.ToUpper(v)) + str[i+1:]
	}

	return ""
}
