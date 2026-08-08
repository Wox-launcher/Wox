package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSmokeTestArgs(t *testing.T) {
	directory := t.TempDir()
	t.Chdir(directory)
	caseDirectory := filepath.Join("test", "smoke", "launcher", "query", "plugin", "calculator")
	if err := os.MkdirAll(caseDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseDirectory, "001_launcher_query_calculator_test.go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := smokeTestCommands("launcher/query/not-a-number"); err == nil {
		t.Fatal("invalid selector should fail")
	}
	all, err := smokeTestCommands("")
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"test", "-failfast", "-tags", "wox_ui_smoke", "-count=1", "-v", "./test/smoke/launcher/query/plugin/calculator"}}
	if !reflect.DeepEqual(all, want) {
		t.Fatalf("all smoke args = %v, want %v", all, want)
	}
	one, err := smokeTestCommands("launcher/query/plugin/calculator/001")
	if err != nil {
		t.Fatal(err)
	}
	wantOne := [][]string{{"test", "-failfast", "-tags", "wox_ui_smoke", "-count=1", "-v", "-run", "^Test001", "./test/smoke/launcher/query/plugin/calculator"}}
	if !reflect.DeepEqual(one, wantOne) {
		t.Fatalf("single smoke args = %v, want %v", one, wantOne)
	}
}
