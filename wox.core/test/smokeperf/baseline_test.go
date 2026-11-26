package smokeperf

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBaselineRequiresBothThresholdsAndKeepsPlatformsSeparate(t *testing.T) {
	base, low, high := 50.0, 10.0, 100.0
	baselines := Baselines{
		"linux":   {ScenarioList: &base, ScenarioGrid: &low, ScenarioPluginCatalog: &high, ScenarioThemeCatalog: nil, ScenarioChatActive: &base},
		"windows": {ScenarioList: &high},
	}
	artifacts := []PerformanceArtifact{
		{OS: "linux", Scenario: ScenarioList, P95Ms: 65},
		{OS: "linux", Scenario: ScenarioGrid, P95Ms: 24.99},
		{OS: "linux", Scenario: ScenarioPluginCatalog, P95Ms: 131},
		{OS: "linux", Scenario: ScenarioThemeCatalog, P95Ms: 70},
		{OS: "windows", Scenario: ScenarioList, P95Ms: 200},
	}
	var summary bytes.Buffer
	got, err := WriteBaselineSummary(&summary, "linux", baselines, artifacts)
	if err != nil || got != 1 {
		t.Fatalf("expected one Linux regression, got %d, %v:\n%s", got, err, summary.String())
	}
	for _, expected := range []string{"| 65.00 ms | +15.00 ms | +30.0% | OK", "🔴 **Regression**", "Awaiting calibration", "No artifact"} {
		if !strings.Contains(summary.String(), expected) {
			t.Errorf("summary missing %q:\n%s", expected, summary.String())
		}
	}
}

func TestLoadPerformanceArtifactsRequiresMeasurement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "latency.json")
	if err := os.WriteFile(path, []byte(`{"os":"windows","scenario":"keystroke-to-results","p95_ms":12.5}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPerformanceArtifacts(dir, "windows")
	if err != nil || len(got) != 1 || got[0].P95Ms != 12.5 || got[0].Scenario != ScenarioKeystroke {
		t.Fatalf("unexpected artifact: %+v, %v", got, err)
	}
	if _, err := LoadPerformanceArtifacts(dir, "linux"); err == nil {
		t.Error("accepted a measurement from another platform")
	}
	if err := os.WriteFile(path, []byte(`{"os":"windows","scenario":"keystroke-to-results"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPerformanceArtifacts(dir, "windows"); err == nil {
		t.Error("accepted an artifact without a P95 measurement")
	}
}
