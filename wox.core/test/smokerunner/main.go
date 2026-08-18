package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"wox/test/automationdriver"
)

var caseSelectorPattern = regexp.MustCompile(`^[a-z0-9_-]+(?:/[a-z0-9_-]+)*/[0-9]{3}$`)
var packageSelectorPattern = regexp.MustCompile(`^[a-z0-9_-]+(?:/[a-z0-9_-]+)*$`)
var caseFilePattern = regexp.MustCompile(`^[0-9]{3}_.+_test\.go$`)

func main() {
	caseSelector := flag.String("case", "", "functional path and case number, for example launcher/plugin/calculator/001")
	flag.Parse()
	code, err := run(strings.TrimSpace(*caseSelector))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}

// run owns the single Wox process while the selected Go smoke packages execute.
func run(caseSelector string) (int, error) {
	testCommands, err := smokeTestCommands(caseSelector)
	if err != nil {
		return 2, err
	}
	executable := strings.TrimSpace(os.Getenv("WOX_GO_UI_SMOKE_BINARY"))
	if executable == "" {
		return 2, errors.New("WOX_GO_UI_SMOKE_BINARY is not configured")
	}
	absoluteExecutable, err := filepath.Abs(executable)
	if err != nil {
		return 2, fmt.Errorf("resolve Wox smoke binary: %w", err)
	}
	suiteDirectory, err := os.MkdirTemp("", "wox-smoke-suite-")
	if err != nil {
		return 1, fmt.Errorf("create smoke suite directory: %w", err)
	}
	retainSuiteDirectory := false
	defer func() {
		if retainSuiteDirectory {
			fmt.Fprintf(os.Stderr, "smoke failure artifacts retained at %s\n", suiteDirectory)
			return
		}
		_ = os.RemoveAll(suiteDirectory)
	}()

	port, err := availablePort()
	if err != nil {
		return 1, err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	woxDataDirectory := filepath.Join(suiteDirectory, "wox-data")
	userDataDirectory := filepath.Join(suiteDirectory, "user-data")
	process, err := automationdriver.Launch(ctx, absoluteExecutable, automationdriver.LaunchOptions{
		Environment: []string{
			"WOX_TEST_DATA_DIR=" + woxDataDirectory,
			"WOX_TEST_USER_DIR=" + userDataDirectory,
			fmt.Sprintf("WOX_TEST_SERVER_PORT=%d", port),
			"WOX_TEST_DISABLE_TELEMETRY=true",
			"WOX_TEST_SKIP_ONBOARDING=true",
			"WOX_DEBUG_REPAINT=verify",
		},
		StartupTimeout: 45 * time.Second,
	})
	if err != nil {
		retainSuiteDirectory = true
		return 1, fmt.Errorf("launch shared Wox smoke process: %w", err)
	}
	defer process.Close()

	testEnvironment := replaceEnvironment(os.Environ(), automationdriver.SharedInfoFileEnvironment, process.InfoFile())
	testEnvironment = replaceEnvironment(testEnvironment, automationdriver.SharedDataDirectoryEnvironment, woxDataDirectory)
	testEnvironment = replaceEnvironment(testEnvironment, automationdriver.SharedUserDataDirectoryEnvironment, userDataDirectory)
	for _, testArgs := range testCommands {
		command := exec.CommandContext(ctx, "go", testArgs...)
		command.Env = testEnvironment
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		if err := command.Run(); err != nil {
			retainSuiteDirectory = true
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return exitErr.ExitCode(), nil
			}
			return 1, fmt.Errorf("run smoke cases: %w", err)
		}
	}
	if err := process.Close(); err != nil {
		retainSuiteDirectory = true
		return 1, fmt.Errorf("close shared Wox smoke process: %w", err)
	}
	return 0, nil
}

// smokeTestCommands maps the selector to serial package commands so the first dirty reset stops the suite.
func smokeTestCommands(caseSelector string) ([][]string, error) {
	baseArgs := []string{"test", "-failfast", "-tags", "wox_ui_smoke", "-count=1", "-v"}
	if caseSelector == "" {
		packages := map[string]struct{}{}
		err := filepath.WalkDir(filepath.FromSlash("test/smoke"), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() && caseFilePattern.MatchString(entry.Name()) {
				packages[filepath.ToSlash(filepath.Dir(path))] = struct{}{}
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("discover smoke packages: %w", err)
		}
		paths := make([]string, 0, len(packages))
		for path := range packages {
			paths = append(paths, "./"+path)
		}
		sort.Strings(paths)
		if len(paths) == 0 {
			return nil, errors.New("no numbered smoke cases were found")
		}
		commands := make([][]string, 0, len(paths))
		for _, path := range paths {
			commands = append(commands, append(append([]string(nil), baseArgs...), path))
		}
		return commands, nil
	}
	if packageSelectorPattern.MatchString(caseSelector) && !caseSelectorPattern.MatchString(caseSelector) {
		return smokePackageCommand(baseArgs, caseSelector)
	}
	if !caseSelectorPattern.MatchString(caseSelector) {
		return nil, fmt.Errorf("invalid smoke CASE %q; expected a package like perf or a path like launcher/plugin/calculator/001", caseSelector)
	}
	matches, err := filepath.Glob(filepath.FromSlash("test/smoke/" + caseSelector + "_*_test.go"))
	if err != nil {
		return nil, fmt.Errorf("resolve smoke CASE %q: %w", caseSelector, err)
	}
	if len(matches) != 1 {
		return nil, fmt.Errorf("smoke CASE %q matched %d files, want exactly one", caseSelector, len(matches))
	}
	directory, number := filepath.Split(filepath.FromSlash(caseSelector))
	packagePath := "./test/smoke/" + filepath.ToSlash(strings.TrimSuffix(directory, string(filepath.Separator)))
	args := append(append([]string(nil), baseArgs...), "-run", "^Test"+number, packagePath)
	return [][]string{args}, nil
}

// smokePackageCommand runs every numbered case in one smoke package, such as perf.
func smokePackageCommand(baseArgs []string, caseSelector string) ([][]string, error) {
	packageDir := filepath.FromSlash("test/smoke/" + caseSelector)
	info, err := os.Stat(packageDir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("smoke package %q was not found", caseSelector)
	}
	entries, err := os.ReadDir(packageDir)
	if err != nil {
		return nil, fmt.Errorf("read smoke package %q: %w", caseSelector, err)
	}
	found := false
	for _, entry := range entries {
		if !entry.IsDir() && caseFilePattern.MatchString(entry.Name()) {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("smoke package %q has no numbered cases", caseSelector)
	}
	return [][]string{append(append([]string(nil), baseArgs...), "./test/smoke/"+caseSelector)}, nil
}

// availablePort reserves and releases a loopback port for the isolated core server.
func availablePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("reserve smoke server port: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		return 0, fmt.Errorf("release smoke server port: %w", err)
	}
	return port, nil
}

// replaceEnvironment applies one deterministic override without duplicate keys.
func replaceEnvironment(environment []string, key, value string) []string {
	prefix := key + "="
	result := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			result = append(result, entry)
		}
	}
	return append(result, prefix+value)
}
