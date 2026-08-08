package launcher

import "testing"

func TestMeasureFormLabelWidthUsesSurfaceMinimumWithoutWindow(t *testing.T) {
	app := &App{}
	if got := app.measureFormLabelWidth(nil, nil, 60, 0); got != 60 {
		t.Fatalf("form label width = %.0f, want action form minimum 60", got)
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
