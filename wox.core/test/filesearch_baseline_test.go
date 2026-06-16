package test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/struCoder/pidusage"
)

type fileSearchBaselineResult struct {
	Title    string `json:"title"`
	SubTitle string `json:"subtitle"`
}

type fileSearchBaselineSample struct {
	Query     string                     `json:"query"`
	P95Millis int64                      `json:"p95_millis"`
	Results   []fileSearchBaselineResult `json:"results"`
}

type fileSearchBaselineArtifact struct {
	CapturedAt             string                     `json:"captured_at"`
	BaselineKind           string                     `json:"baseline_kind"`
	SteadyStateCPUPercent  float64                    `json:"steady_state_cpu_percent"`
	SteadyStateMemoryBytes uint64                     `json:"steady_state_memory_bytes"`
	IndexSnapshotSummary   string                     `json:"index_snapshot_summary"`
	IndexTopRootsSummary   string                     `json:"index_top_roots_summary"`
	Queries                []fileSearchBaselineSample `json:"queries"`
}

func TestCaptureFileSearchIndexedOnlyBaseline(t *testing.T) {
	if os.Getenv("WOX_CAPTURE_FILESEARCH_BASELINE") != "1" {
		t.Skip("set WOX_CAPTURE_FILESEARCH_BASELINE=1 to capture indexed-only baseline")
	}

	suite := NewTestSuite(t)
	ctx := suite.ctx

	tempRoot := t.TempDir()
	fileRootPath := filepath.Join(tempRoot, "filesearch-baseline-root")
	if err := os.MkdirAll(fileRootPath, 0755); err != nil {
		t.Fatalf("create baseline root: %v", err)
	}

	fileQueries := []string{"readme", "plugin", "setting"}
	for _, name := range fileQueries {
		if err := os.WriteFile(filepath.Join(fileRootPath, name), []byte(name), 0644); err != nil {
			t.Fatalf("create baseline fixture %q: %v", name, err)
		}
	}

	filePlugin := findPluginInstance("979d6363-025a-4f51-88d3-0b04e9dc56bf")
	if filePlugin == nil {
		t.Fatal("file plugin instance not found")
	}

	rootsPayload, err := json.Marshal([]map[string]string{{"Path": fileRootPath}})
	if err != nil {
		t.Fatalf("marshal baseline roots: %v", err)
	}
	filePlugin.API.SaveSetting(ctx, "roots", string(rootsPayload), false)

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		allReady := true
		for _, name := range fileQueries {
			results, err := runQuery(ctx, "f "+name)
			if err != nil {
				t.Fatalf("warm up file search baseline: %v", err)
			}

			ready := false
			expectedPath := filepath.Join(fileRootPath, name)
			for _, result := range results {
				if result.Title == name && result.SubTitle == expectedPath {
					ready = true
					break
				}
			}

			if !ready {
				allReady = false
				break
			}
		}

		if allReady {
			break
		}

		time.Sleep(200 * time.Millisecond)
	}

	for _, name := range fileQueries {
		results, err := runQuery(ctx, "f "+name)
		if err != nil {
			t.Fatalf("final warm up check for %q: %v", name, err)
		}

		expectedPath := filepath.Join(fileRootPath, name)
		ready := false
		for _, result := range results {
			if result.Title == name && result.SubTitle == expectedPath {
				ready = true
				break
			}
		}

		if !ready {
			t.Fatalf("timed out waiting for %q under %q to become searchable", name, fileRootPath)
		}
	}

	queries := []string{
		"f readme",
		"f plugin",
		"f setting",
	}

	samples := make([]fileSearchBaselineSample, 0, len(queries))
	for _, query := range queries {
		durations := make([]time.Duration, 0, 15)
		var resultsSnapshot []fileSearchBaselineResult
		for i := 0; i < 15; i++ {
			start := time.Now()
			results, err := runQuery(ctx, query)
			if err != nil {
				t.Fatalf("run baseline query %q: %v", query, err)
			}
			durations = append(durations, time.Since(start))
			if i == 0 {
				resultsSnapshot = make([]fileSearchBaselineResult, 0, len(results))
				rootPrefix := fileRootPath + string(filepath.Separator)
				for _, result := range results {
					if result.SubTitle == fileRootPath || strings.HasPrefix(result.SubTitle, rootPrefix) {
						resultsSnapshot = append(resultsSnapshot, fileSearchBaselineResult{
							Title:    result.Title,
							SubTitle: result.SubTitle,
						})
					}
				}
				if len(resultsSnapshot) == 0 {
					t.Fatalf("query %q did not return any result from baseline root %q", query, fileRootPath)
				}
			}
		}

		sort.Slice(durations, func(i, j int) bool {
			return durations[i] < durations[j]
		})
		p95Index := int(math.Round(0.95 * float64(len(durations)-1)))

		samples = append(samples, fileSearchBaselineSample{
			Query:     query,
			P95Millis: durations[p95Index].Milliseconds(),
			Results:   resultsSnapshot,
		})
	}

	steadyStateCPU, steadyStateMemory := captureSteadyStateProcessUsage(t)
	engine, err := getFileSearchEngine()
	if err != nil {
		t.Fatalf("get file search engine for baseline snapshot: %v", err)
	}

	artifact := fileSearchBaselineArtifact{
		CapturedAt:             time.Now().UTC().Format(time.RFC3339),
		BaselineKind:           "indexed-only",
		SteadyStateCPUPercent:  steadyStateCPU,
		SteadyStateMemoryBytes: steadyStateMemory,
		IndexSnapshotSummary:   engine.IndexSnapshotSummary(),
		IndexTopRootsSummary:   engine.IndexTopRootsSummary(),
		Queries:                samples,
	}

	payload, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		t.Fatalf("marshal baseline: %v", err)
	}

	outputPath := os.Getenv("WOX_FILESEARCH_BASELINE_PATH")
	if outputPath == "" {
		outputPath = filepath.Join("testdata", "filesearch_indexed_only_baseline.json")
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		t.Fatalf("create baseline artifact directory: %v", err)
	}
	if err := os.WriteFile(outputPath, payload, 0644); err != nil {
		t.Fatalf("write baseline artifact %q: %v", outputPath, err)
	}

	t.Logf("baseline artifact written to %s", outputPath)
	t.Log(string(payload))
}

func captureSteadyStateProcessUsage(t *testing.T) (float64, uint64) {
	t.Helper()

	pid := os.Getpid()
	if _, err := pidusage.GetStat(pid); err != nil {
		t.Fatalf("prime pidusage baseline: %v", err)
	}

	samples := make([]*pidusage.SysInfo, 0, 5)
	for range 5 {
		time.Sleep(300 * time.Millisecond)

		sample, err := pidusage.GetStat(pid)
		if err != nil {
			t.Fatalf("capture pidusage sample: %v", err)
		}
		samples = append(samples, sample)
	}

	totalCPU := 0.0
	maxMemory := float64(0)
	for _, sample := range samples {
		totalCPU += sample.CPU
		if sample.Memory > maxMemory {
			maxMemory = sample.Memory
		}
	}

	return totalCPU / float64(len(samples)), uint64(maxMemory)
}
