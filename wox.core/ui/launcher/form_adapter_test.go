package launcher

import "testing"

func TestPluginFormFieldHeightsMatchCompactFlutterRows(t *testing.T) {
	tests := []struct {
		name       string
		definition formDefinition
		want       float32
	}{
		{name: "checkbox", definition: formDefinition{Type: "checkbox"}, want: 40},
		{name: "textbox", definition: formDefinition{Type: "textbox"}, want: 44},
		{name: "select tooltip", definition: formDefinition{Type: "select", Value: formDefinitionValue{Tooltip: "Details"}}, want: 64},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := formDefinitionHeight(test.definition); got != test.want {
				t.Fatalf("formDefinitionHeight() = %.0f, want %.0f", got, test.want)
			}
		})
	}
}
