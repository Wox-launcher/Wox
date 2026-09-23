package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSuiteArtifactRootCreatesConfiguredDirectory(t *testing.T) {
	root, err := suiteArtifactRoot()
	if err != nil {
		t.Fatal(err)
	}
	if root != "" {
		t.Fatalf("unset artifact root = %q, want the OS temp directory", root)
	}

	configured := filepath.Join(t.TempDir(), "artifacts", "smoke")
	t.Setenv(smokeArtifactDirEnvironment, configured)
	root, err = suiteArtifactRoot()
	if err != nil {
		t.Fatal(err)
	}
	if root != configured {
		t.Fatalf("artifact root = %q, want %q", root, configured)
	}
	if info, statErr := os.Stat(configured); statErr != nil || !info.IsDir() {
		t.Fatalf("artifact root was not created: %v", statErr)
	}
}

func TestWaitForAppIndexSettledReturnsWhenTheIndexAppears(t *testing.T) {
	woxDataDirectory := t.TempDir()
	cachePath := filepath.Join(woxDataDirectory, appIndexCacheRelativePath)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(150 * time.Millisecond)
		_ = os.WriteFile(cachePath, []byte(`{"version":14,"apps":[]}`), 0o644)
	}()

	start := time.Now()
	waitForAppIndexSettled(context.Background(), woxDataDirectory)
	elapsed := time.Since(start)
	if elapsed < 100*time.Millisecond {
		t.Fatalf("wait returned after %s, want it to block until the index existed", elapsed)
	}
	if elapsed >= appIndexSettleTimeout {
		t.Fatalf("wait returned after %s, want it to notice the index instead of burning the budget", elapsed)
	}
}

func TestWaitForAppIndexSettledStopsWithTheSuiteContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	waitForAppIndexSettled(ctx, t.TempDir())
	if elapsed := time.Since(start); elapsed >= appIndexSettleTimeout {
		t.Fatalf("cancelled wait took %s, want a prompt return", elapsed)
	}
}

func TestSmokeTestArgs(t *testing.T) {
	directory := t.TempDir()
	t.Chdir(directory)
	caseDirectory := filepath.Join("test", "smoke", "launcher", "plugin", "calculator")
	if err := os.MkdirAll(caseDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseDirectory, "001_launcher_query_calculator_test.go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := smokeTargets("missing-package"); err == nil {
		t.Fatal("missing package selector should fail")
	}
	all, err := smokeTargets("")
	if err != nil {
		t.Fatal(err)
	}
	want := []smokeTarget{{dir: "test/smoke/launcher/plugin/calculator"}}
	if !reflect.DeepEqual(all, want) {
		t.Fatalf("all smoke targets = %+v, want %+v", all, want)
	}
	one, err := smokeTargets("launcher/plugin/calculator/001")
	if err != nil {
		t.Fatal(err)
	}
	wantOne := []smokeTarget{{dir: "test/smoke/launcher/plugin/calculator", runPattern: "^Test001"}}
	if !reflect.DeepEqual(one, wantOne) {
		t.Fatalf("single smoke targets = %+v, want %+v", one, wantOne)
	}
	pkg, err := smokeTargets("launcher/plugin/calculator")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pkg, want) {
		t.Fatalf("package smoke targets = %+v, want %+v", pkg, want)
	}

	compileArgs := smokeCompileArgs("bins", []smokeTarget{
		{dir: "test/smoke/launcher"},
		{dir: "test/smoke/perf"},
	})
	wantCompile := []string{"test", "-tags", "wox_ui_smoke", "-c", "-o", "bins", "./test/smoke/launcher", "./test/smoke/perf"}
	if !reflect.DeepEqual(compileArgs, wantCompile) {
		t.Fatalf("compile args = %v, want %v", compileArgs, wantCompile)
	}
	runArgs := smokeRunArgs(wantOne[0])
	wantRun := []string{"-test.failfast", "-test.count=1", "-test.v", "-test.timeout", smokePackageTimeout, "-test.run", "^Test001"}
	if !reflect.DeepEqual(runArgs, wantRun) {
		t.Fatalf("run args = %v, want %v", runArgs, wantRun)
	}
	if err := duplicateSmokeBinaries([]smokeTarget{{dir: "a/ui"}, {dir: "b/ui"}}); err == nil {
		t.Fatal("packages that share a binary name should fail")
	}
}

func TestCollectSmokeResultsRecordsCaseDurationAndStatus(t *testing.T) {
	output := strings.Join([]string{
		"=== RUN   Test001One",
		"--- PASS: Test001One (0.29s)",
		"=== RUN   Test002Parent",
		"=== RUN   Test002Parent/child",
		"--- PASS: Test002Parent/child (0.01s)",
		"--- PASS: Test002Parent (0.40s)",
		"=== RUN   Test003Skip",
		"--- SKIP: Test003Skip (0.00s)",
		"=== RUN   Test004Fail",
		"--- FAIL: Test004Fail (1.50s)",
		"FAIL",
		"",
	}, "\n")
	var echoed bytes.Buffer
	results, err := collectSmokeResults(strings.NewReader(output), &echoed, "test/smoke/launcher/plugin/calculator", "")
	if err != nil {
		t.Fatal(err)
	}
	want := []smokeCaseResult{
		{name: "launcher/plugin/calculator/Test001One", elapsed: 290 * time.Millisecond, status: smokeStatusPass},
		{name: "launcher/plugin/calculator/Test002Parent", elapsed: 400 * time.Millisecond, status: smokeStatusPass},
		{name: "launcher/plugin/calculator/Test003Skip", elapsed: 0, status: smokeStatusSkip},
		{name: "launcher/plugin/calculator/Test004Fail", elapsed: 1500 * time.Millisecond, status: smokeStatusFail},
	}
	if !reflect.DeepEqual(results, want) {
		t.Fatalf("results = %+v, want %+v", results, want)
	}
	if !strings.Contains(echoed.String(), "smoke  0.29s  PASS  launcher/plugin/calculator/Test001One") {
		t.Fatalf("echo = %s, want the case duration while the test is running", echoed.String())
	}
	if strings.Contains(echoed.String(), "Test002Parent/child") && strings.Contains(echoed.String(), "smoke  0.01s") {
		t.Fatalf("echo = %s, want subtests omitted from the timing line", echoed.String())
	}
}

func TestCollectSmokeResultsMarksACrashedCaseFailed(t *testing.T) {
	output := "=== RUN   Test005Crash\npanic: boom\n"
	results, err := collectSmokeResults(strings.NewReader(output), ioDiscard{}, "test/smoke/setting/privacy", "phase 2")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %+v, want the unfinished case", results)
	}
	if results[0].name != "setting/privacy/Test005Crash (phase 2)" || results[0].status != smokeStatusFail {
		t.Fatalf("result = %+v, want the phase-qualified crash", results[0])
	}
}

func TestPrintSmokeSummaryShowsNameDurationAndResult(t *testing.T) {
	var buffer bytes.Buffer
	printSmokeSummary(&buffer, []smokeCaseResult{
		{name: "launcher/plugin/calculator/Test001One", elapsed: 290 * time.Millisecond, status: smokeStatusPass},
		{name: "setting/ui/Test002Slow", elapsed: 90*time.Second + 200*time.Millisecond, status: smokeStatusFail},
	}, 2*time.Minute+5*time.Second)
	text := buffer.String()
	for _, line := range []string{
		"NAME",
		"DURATION",
		"RESULT",
		"TOTAL (wall) includes resets and the time between cases.",
		"launcher/plugin/calculator/Test001One",
		"0.29s",
		"PASS",
		"setting/ui/Test002Slow",
		"1m30.20s",
		"FAIL",
		"TOTAL (wall)",
		"2m05.00s",
		"1 passed, 1 failed",
	} {
		if !strings.Contains(text, line) {
			t.Fatalf("summary = %s, want %q", text, line)
		}
	}
	slow := strings.Index(text, "setting/ui/Test002Slow")
	fast := strings.Index(text, "launcher/plugin/calculator/Test001One")
	if slow < 0 || fast < 0 || slow > fast {
		t.Fatalf("summary = %s, want the slower case before the faster one", text)
	}
}

// ioDiscard drops echoed test output in parser tests.
type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }
