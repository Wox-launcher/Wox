package calc

import "testing"

func TestNumberPrefixDigitSeparator(t *testing.T) {
	tests := []struct {
		input        string
		thousandsSep string
		decimalSep   string
		want         string
		consumed     int
		wantErr      bool
	}{
		{input: "1_000_000", thousandsSep: ",", decimalSep: ".", want: "1000000", consumed: len("1_000_000")},
		{input: "1_000_000.25", thousandsSep: ",", decimalSep: ".", want: "1000000.25", consumed: len("1_000_000.25")},
		{input: "3.141_592", thousandsSep: ",", decimalSep: ".", want: "3.141592", consumed: len("3.141_592")},
		{input: "1_000,25", thousandsSep: ".", decimalSep: ",", want: "1000.25", consumed: len("1_000,25")},
		{input: "1_234,567.89", thousandsSep: ",", decimalSep: ".", want: "1234567.89", consumed: len("1_234,567.89")},
		{input: "10_0", thousandsSep: "", decimalSep: ".", want: "100", consumed: 4},
		{input: "1__000", thousandsSep: ",", decimalSep: ".", want: "1", consumed: 1},
		{input: "1_000_", thousandsSep: ",", decimalSep: ".", want: "1000", consumed: len("1_000")},
		{input: "1_.5", thousandsSep: ",", decimalSep: ".", want: "1", consumed: 1},
		{input: "1._5", thousandsSep: ",", decimalSep: ".", want: "1.", consumed: 2},
		{input: "_1000", thousandsSep: ",", decimalSep: ".", wantErr: true},
	}

	for _, test := range tests {
		chars := []rune(test.input)
		i := 0
		got, err := NumberPrefix(chars, &i, len(chars), test.thousandsSep, test.decimalSep)
		if test.wantErr {
			if err == nil {
				t.Errorf("NumberPrefix(%q) succeeded with %q", test.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("NumberPrefix(%q) returned an error: %v", test.input, err)
			continue
		}
		if got != test.want || i != test.consumed {
			t.Errorf("NumberPrefix(%q) = %q consumed %d, want %q consumed %d", test.input, got, i, test.want, test.consumed)
		}
	}
}
