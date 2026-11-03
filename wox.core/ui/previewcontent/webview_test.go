package previewcontent

import "testing"

func TestDecodeWebView(t *testing.T) {
	tests := []struct {
		input   string
		url     string
		html    string
		invalid bool
	}{
		{input: " https://example.com ", url: "https://example.com"},
		{input: `{"html":"<p>Hello</p>"}`, html: "<p>Hello</p>"},
		{input: "", invalid: true},
		{input: `{}`, invalid: true},
		{input: `{"html":"ok","script":"run()"}`, invalid: true},
		{input: `{"updateMode":"script","script":"run()"}`, invalid: true},
		{input: `{invalid`, invalid: true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := DecodeWebView(tt.input)
			if (err != nil) != tt.invalid {
				t.Fatalf("error = %v, invalid = %v", err, tt.invalid)
			}
			if !tt.invalid && (got.URL != tt.url || got.HTML != tt.html) {
				t.Fatalf("got %#v", got)
			}
		})
	}
}
