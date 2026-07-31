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
		{name: "dictation hotkey", definition: formDefinition{Type: "dictationHotkey"}, want: 44},
		{name: "dictation hotkey tooltip", definition: formDefinition{Type: "dictationHotkey", Value: formDefinitionValue{Tooltip: "Details"}}, want: 64},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := formDefinitionHeight(test.definition); got != test.want {
				t.Fatalf("formDefinitionHeight() = %.0f, want %.0f", got, test.want)
			}
		})
	}
}

func TestAIModelsFromOptionsPreservesProviderAlias(t *testing.T) {
	models := aiModelsFromOptions([]formOption{
		{Value: `{"Name":"deepseek-v4-flash","Provider":"deepseek","ProviderAlias":"work"}`},
		{Value: `not-json`},
	})
	if len(models) != 1 {
		t.Fatalf("parsed model count = %d, want 1", len(models))
	}
	if models[0].Name != "deepseek-v4-flash" || models[0].Provider != "deepseek" || models[0].ProviderAlias != "work" {
		t.Fatalf("parsed model = %#v", models[0])
	}
}
