package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
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
