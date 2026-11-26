//go:build wox_ui_smoke

package perf

import (
	"os"
	"testing"
	"time"

	"wox/test/smokeperf"
)

// requireNativePerf opts the scheduled native matrix into the existing local perf cases.
func requireNativePerf(t *testing.T) {
	t.Helper()
	if os.Getenv("GITHUB_ACTIONS") == "true" && os.Getenv("WOX_NIGHTLY_PERF") != "1" {
		t.Skip("presented-frame performance sampling requires the native perf matrix")
	}
}

func perfTimingBudgetMultiplier() int64 {
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		return 5
	}
	return 1
}

// frameBudget contains local phase ceilings in microseconds. Regression detection uses
// measured P95 independently so generous initial CI ceilings cannot hide drift.
type frameBudget struct {
	build, snapshot, draw, a11y, goTotal, native int64
}

// frameBudgetFor keeps light interactions below heavy catalog and chat ceilings.
func frameBudgetFor(scenario smokeperf.Scenario, platform string) frameBudget {
	budget := frameBudget{build: 10000, snapshot: 2000, draw: 7000, a11y: 2000, goTotal: 18000}
	switch scenario {
	case smokeperf.ScenarioHover, smokeperf.ScenarioSelection, smokeperf.ScenarioResize:
		budget.build, budget.draw, budget.a11y, budget.goTotal = 5000, 4000, 1000, 10000
	case smokeperf.ScenarioPluginCatalog, smokeperf.ScenarioThemeCatalog, smokeperf.ScenarioChatActive,
		smokeperf.ScenarioChatSettled, smokeperf.ScenarioChatScrollUp, smokeperf.ScenarioChatScrollDown:
		budget = frameBudget{build: 20000, snapshot: 4000, draw: 15000, a11y: 4000, goTotal: 35000}
	}
	switch platform {
	case "darwin":
		budget.native = budget.goTotal * 5
	case "windows":
		budget.native = budget.goTotal * 8
	default:
		budget.native = budget.goTotal * 11
	}
	return budget
}

// latencyBudget applies the P95 ceiling for the interaction and platform.
func latencyBudget(scenario smokeperf.Scenario, platform string) time.Duration {
	budget := 300 * time.Millisecond
	if scenario == smokeperf.ScenarioKeystroke {
		switch platform {
		case "darwin":
			budget = 125 * time.Millisecond
		case "windows":
			budget = 160 * time.Millisecond
		default:
			budget = 200 * time.Millisecond
		}
	}
	return budget * time.Duration(perfTimingBudgetMultiplier())
}
