package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"

	"wox/test/smokeperf"
)

func main() {
	baselinePath := flag.String("baselines", "test/smoke/perf/baselines.json", "baseline JSON file")
	artifactDir := flag.String("artifacts", "../artifacts/perf", "directory containing perf JSON artifacts")
	platform := flag.String("os", runtime.GOOS, "GOOS of the smoke runner")
	failOnRegression := flag.Bool("fail-on-regression", false, "exit nonzero when the regression threshold is exceeded")
	flag.Parse()

	regressions, err := check(*baselinePath, *artifactDir, *platform)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *failOnRegression && regressions > 0 {
		os.Exit(1)
	}
}

// check writes to GitHub Step Summary when available and to stdout for local runs.
func check(baselinePath, artifactDir, platform string) (int, error) {
	baselines, err := smokeperf.LoadBaselines(baselinePath)
	if err != nil {
		return 0, fmt.Errorf("load performance baselines: %w", err)
	}
	artifacts, err := smokeperf.LoadPerformanceArtifacts(artifactDir, platform)
	if err != nil {
		return 0, fmt.Errorf("load performance artifacts: %w", err)
	}
	var output io.Writer = os.Stdout
	if path := os.Getenv("GITHUB_STEP_SUMMARY"); path != "" {
		file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return 0, err
		}
		defer file.Close()
		output = file
	}
	return smokeperf.WriteBaselineSummary(output, platform, baselines, artifacts)
}
