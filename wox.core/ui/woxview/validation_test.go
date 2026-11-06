package woxview

import (
	"errors"
	"strings"
	"testing"
)

// TestValidationErrorLocationsPreserveDiagnostics checks typed locations without changing wire-visible error text.
func TestValidationErrorLocationsPreserveDiagnostics(t *testing.T) {
	cases := []struct{ data, path, message string }{
		{`{`, "$", "invalid woxview tree: unexpected EOF"},
		{`{"version":2}`, "version", "unsupported woxview version 2"},
		{`{"version":1}`, "root", "woxview tree has no root"},
		{`{"version":1,"root":{"type":"column","children":[{"type":"unknown"}]}}`, "root/column[0].type", `root/column[0]: unknown node type "unknown"`},
		{`{"version":1,"root":{"type":"column","children":[{"type":"text","props":{"txt":"x"}}]}}`, "root/column[0]/text.props", `root/column[0]/text props: json: unknown field "txt"`},
		{`{"version":1,"root":{"type":"text-field","props":{"maxLines":101}}}`, "root/text-field.props", "root/text-field props: maxLines must be between 0 and 100"},
		{`{"version":1,"root":{"type":"button","props":{"onPress":{"$handler":""}}}}`, "root/button.props.onPress", "root/button props: onPress: handler id must be 1-128 bytes"},
		{`{"version":1,"root":{"type":"text","children":[{"type":"text"}]}}`, "root/text.children", "root/text: text only accepts span children"},
		{`{"version":1,"root":{"type":"column","children":[{"type":"submit"}]}}`, "root/column[0]/submit", "submit must be inside a form"},
		{`{"version":1,"root":{"type":"form","children":[{"type":"form"}]}}`, "root/form", "root/form: nested forms are not supported"},
	}
	for _, tc := range cases {
		_, err := Parse([]byte(tc.data))
		var validation *ValidationError
		if !errors.As(err, &validation) {
			t.Fatalf("%s: expected ValidationError, got %v", tc.data, err)
		}
		if validation.Path != tc.path || validation.Message != tc.message || err.Error() != tc.message {
			t.Errorf("%s: got %+v, want path %q and message %q", tc.data, validation, tc.path, tc.message)
		}
	}
}

// TestNewTreeReturnsStructuredErrors covers Go-only invalid inputs, including typed nil props.
func TestNewTreeReturnsStructuredErrors(t *testing.T) {
	var nilProps *TextProps
	cases := []*Node{
		nil,
		{Type: NodeText, Props: nilProps},
		{Type: NodeText, Props: &ButtonProps{}},
		{Type: NodeColumn, Children: []*Node{nil}},
		{Type: NodeColumn, Children: []*Node{{Type: NodeText, Key: "same"}, {Type: NodeText, Key: "same"}}},
		{Type: NodeText, Key: strings.Repeat("x", 257)},
	}
	for _, root := range cases {
		_, err := NewTree(root)
		var validation *ValidationError
		if !errors.As(err, &validation) || validation.Path == "" || validation.Message == "" {
			t.Fatalf("unstructured error: %v", err)
		}
	}
}

// TestTreeLimitsReturnStructuredErrors keeps resource-limit failures on the same error contract.
func TestTreeLimitsReturnStructuredErrors(t *testing.T) {
	deep := strings.Repeat(`{"type":"view","children":[`, MaxDepth+1) + `{"type":"text"}` + strings.Repeat(`]}`, MaxDepth+1)
	many := `{"type":"column","children":[` + strings.TrimSuffix(strings.Repeat(`{"type":"spacer"},`, MaxNodes), ",") + `]}`
	for _, data := range []string{strings.Repeat(" ", MaxBytes+1), `{"version":1,"root":` + deep + `}`, `{"version":1,"root":` + many + `}`} {
		_, err := Parse([]byte(data))
		var validation *ValidationError
		if !errors.As(err, &validation) || validation.Path == "" {
			t.Fatalf("unstructured limit error: %v", err)
		}
	}
}
