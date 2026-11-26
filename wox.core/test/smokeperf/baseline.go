package smokeperf

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Scenario distinguishes timings that have different work or latency expectations.
type Scenario string

const (
	ScenarioList           Scenario = "list-500"
	ScenarioGrid           Scenario = "grid-500"
	ScenarioPluginCatalog  Scenario = "plugin-catalog-500"
	ScenarioThemeCatalog   Scenario = "theme-catalog-500"
	ScenarioChatActive     Scenario = "chat-stream"
	ScenarioChatSettled    Scenario = "chat-settled"
	ScenarioChatScrollUp   Scenario = "chat-scroll-up"
	ScenarioChatScrollDown Scenario = "chat-scroll-down"
	ScenarioHover          Scenario = "local-hover"
	ScenarioSelection      Scenario = "local-selection"
	ScenarioResize         Scenario = "local-resize"
	ScenarioShow           Scenario = "show-to-visible"
	ScenarioKeystroke      Scenario = "keystroke-to-results"
)

type LatencyMetric string

const (
	LatencyMetricFrameWork LatencyMetric = "frame_work"
	LatencyMetricShow      LatencyMetric = "show_to_visible"
	LatencyMetricKeystroke LatencyMetric = "keystroke_to_results"
)

// PerformanceArtifact is the comparison header shared by the native smoke cases.
type PerformanceArtifact struct {
	Case     string        `json:"case"`
	Scenario Scenario      `json:"scenario"`
	OS       string        `json:"os"`
	Metric   LatencyMetric `json:"metric"`
	P50Ms    float64       `json:"p50_ms"`
	P95Ms    float64       `json:"p95_ms"`
	MaxMs    float64       `json:"max_ms"`
}

// Baselines maps OS and scenario to the reference P95 in milliseconds.
// A null entry is reported as awaiting calibration.
type Baselines map[string]map[Scenario]*float64

// LoadBaselines rejects invalid reference values before reading the current run.
func LoadBaselines(path string) (Baselines, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var baselines Baselines
	if err := json.Unmarshal(data, &baselines); err != nil {
		return nil, err
	}
	for platform, scenarios := range baselines {
		for scenario, p95 := range scenarios {
			if p95 != nil && *p95 <= 0 {
				return nil, fmt.Errorf("baseline %s/%s must be positive or null", platform, scenario)
			}
		}
	}
	return baselines, nil
}

// LoadPerformanceArtifacts reads one Nightly platform's perf directory.
func LoadPerformanceArtifacts(dir, platform string) ([]PerformanceArtifact, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var artifacts []PerformanceArtifact
	seen := map[Scenario]bool{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var wire struct {
			PerformanceArtifact
			P95Ms *float64 `json:"p95_ms"`
		}
		if err := json.Unmarshal(data, &wire); err != nil {
			return nil, fmt.Errorf("decode %s: %w", entry.Name(), err)
		}
		if wire.OS != platform || wire.Scenario == "" || wire.P95Ms == nil || *wire.P95Ms < 0 {
			return nil, fmt.Errorf("%s: expected a %s artifact with a scenario and non-negative p95_ms", entry.Name(), platform)
		}
		if seen[wire.Scenario] {
			return nil, fmt.Errorf("duplicate %s artifact for %s", platform, wire.Scenario)
		}
		seen[wire.Scenario] = true
		wire.PerformanceArtifact.P95Ms = *wire.P95Ms
		artifacts = append(artifacts, wire.PerformanceArtifact)
	}
	return artifacts, nil
}

// WriteBaselineSummary compares P95 per OS and scenario, returning the number of regressions.
func WriteBaselineSummary(w io.Writer, platform string, baselines Baselines, artifacts []PerformanceArtifact) (int, error) {
	current := make(map[Scenario]float64, len(artifacts))
	references := baselines[platform]
	scenarios := make([]Scenario, 0, len(references)+len(artifacts))
	for scenario := range references {
		scenarios = append(scenarios, scenario)
	}
	for _, artifact := range artifacts {
		if artifact.OS != platform {
			continue
		}
		current[artifact.Scenario] = artifact.P95Ms
		if _, exists := references[artifact.Scenario]; !exists {
			scenarios = append(scenarios, artifact.Scenario)
		}
	}
	slices.Sort(scenarios)

	var summary strings.Builder
	fmt.Fprintf(&summary, "## UI performance baseline (%s)\n\n", platform)
	summary.WriteString("| Scenario | Baseline P95 | Current P95 | Delta | Change | Status |\n| --- | ---: | ---: | ---: | ---: | --- |\n")
	regressions := 0
	for _, scenario := range scenarios {
		reference := references[scenario]
		p95, observed := current[scenario]
		baseText, currentText, deltaText, changeText := "—", "—", "—", "—"
		status := "Awaiting calibration"
		if reference != nil {
			baseText = fmt.Sprintf("%.2f ms", *reference)
		}
		if observed {
			currentText = fmt.Sprintf("%.2f ms", p95)
		}
		if !observed {
			status = "No artifact"
		} else if reference != nil {
			delta := p95 - *reference
			deltaText = fmt.Sprintf("%+.2f ms", delta)
			changeText = fmt.Sprintf("%+.1f%%", 100*delta / *reference)
			status = "OK"
			if delta >= 15 && p95 > *reference*1.30 {
				status = "🔴 **Regression**"
				regressions++
			}
		}
		fmt.Fprintf(&summary, "| %s | %s | %s | %s | %s | %s |\n", scenario, baseText, currentText, deltaText, changeText, status)
	}
	_, err := io.WriteString(w, summary.String()+"\n")
	return regressions, err
}
