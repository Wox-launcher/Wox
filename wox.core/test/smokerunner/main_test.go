package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
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

	if _, err := smokeTestCommands("missing-package"); err == nil {
		t.Fatal("missing package selector should fail")
	}
	all, err := smokeTestCommands("")
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"test", "-failfast", "-tags", "wox_ui_smoke", "-count=1", "-v", "./test/smoke/launcher/plugin/calculator"}}
	if !reflect.DeepEqual(all, want) {
		t.Fatalf("all smoke args = %v, want %v", all, want)
	}
	one, err := smokeTestCommands("launcher/plugin/calculator/001")
	if err != nil {
		t.Fatal(err)
	}
	wantOne := [][]string{{"test", "-failfast", "-tags", "wox_ui_smoke", "-count=1", "-v", "-run", "^Test001", "./test/smoke/launcher/plugin/calculator"}}
	if !reflect.DeepEqual(one, wantOne) {
		t.Fatalf("single smoke args = %v, want %v", one, wantOne)
	}
	pkg, err := smokeTestCommands("launcher/plugin/calculator")
	if err != nil {
		t.Fatal(err)
	}
	wantPkg := [][]string{{"test", "-failfast", "-tags", "wox_ui_smoke", "-count=1", "-v", "./test/smoke/launcher/plugin/calculator"}}
	if !reflect.DeepEqual(pkg, wantPkg) {
		t.Fatalf("package smoke args = %v, want %v", pkg, wantPkg)
	}
}
