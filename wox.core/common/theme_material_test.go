package common

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestThemeToolbarMaterial checks decimals, desktop overrides, zero, reset, and lossless save.
func TestThemeToolbarMaterial(t *testing.T) {
	input := strings.TrimSuffix(minimalV2Theme, "}") + `,"ToolbarBlurSigma":12,"ToolbarBlurBrightness":1,"linux":{"variants":{"kde":{"backgroundBlur":{"ToolbarBlurSigma":4.5,"ToolbarBlurBrightness":0.8,"ToolbarBlurSaturation":0}}}}}`
	var theme Theme
	if err := json.Unmarshal([]byte(input), &theme); err != nil {
		t.Fatal(err)
	}
	resolved, err := theme.ResolveForTarget("linux", "kde", "backgroundBlur")
	if err != nil {
		t.Fatal(err)
	}
	if *resolved.ToolbarBlurSigma != 4.5 || *resolved.ToolbarBlurBrightness != .8 || *resolved.ToolbarBlurSaturation != 0 {
		t.Fatal("material overrides lost")
	}
	saved, err := json.Marshal(resolved)
	if err != nil {
		t.Fatal(err)
	}
	var restored Theme
	if err := json.Unmarshal(saved, &restored); err != nil {
		t.Fatal(err)
	}
	fallback, err := restored.ResolveForTarget("linux", "kde")
	if err != nil || *fallback.ToolbarBlurSigma != 12 || *fallback.ToolbarBlurBrightness != 1 || fallback.ToolbarBlurSaturation != nil {
		t.Fatalf("save flattened active material: %v", err)
	}
	input = strings.Replace(input, `"ToolbarBlurSigma":4.5`, `"ToolbarBlurSigma":null`, 1)
	if err := json.Unmarshal([]byte(input), &theme); err != nil {
		t.Fatal(err)
	}
	resolved, err = theme.ResolveForTarget("linux", "kde", "backgroundBlur")
	if err != nil || resolved.ToolbarBlurSigma != nil {
		t.Fatalf("null did not restore native default: %v", err)
	}
}

// TestThemeToolbarMaterialValidation also rejects invalid inactive desktop capabilities.
func TestThemeToolbarMaterialValidation(t *testing.T) {
	for _, fields := range []string{`"ToolbarBlurSigma":-1`, `"ToolbarBlurSigma":65`, `"ToolbarBlurBrightness":2.1`, `"ToolbarBlurSaturation":-0.1`, `"ToolbarBlurBrightness":"NaN"`} {
		var theme Theme
		input := strings.TrimSuffix(minimalV2Theme, "}") + `,"linux":{"variants":{"kde":{"backgroundBlur":{` + fields + `}}}}}`
		if err := json.Unmarshal([]byte(input), &theme); err == nil {
			t.Fatalf("accepted %s", fields)
		}
	}
}
