package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"wox/test/automationdriver"
	"wox/test/smokefixture"

	_ "github.com/mattn/go-sqlite3"
)

// smokeArtifactDirEnvironment relocates retained failure artifacts out of the OS
// temp directory so CI can upload them.
const smokeArtifactDirEnvironment = "WOX_SMOKE_ARTIFACT_DIR"

// smokeModulePath is the wox.core module path. Compiled test summaries use it so
// package lines stay identical to a direct go test run.
const smokeModulePath = "wox"

// smokePackageTimeout matches go test's default budget for one package binary.
const smokePackageTimeout = "10m"

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
// Every selected package is compiled by one go test invocation. Separate go test
// processes repeat package loading and test-binary linking, which on a warm cache
// still costs a few seconds per package before any case runs.
func run(caseSelector string) (int, error) {
	targets, err := smokeTargets(caseSelector)
	if err != nil {
		return 2, err
	}
	if err := duplicateSmokeBinaries(targets); err != nil {
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
	var results []smokeCaseResult
	var suiteStarted time.Time
	// Registered first so the table is the last thing printed, including after a failure.
	defer func() {
		if len(results) == 0 || suiteStarted.IsZero() {
			return
		}
		printSmokeSummary(os.Stdout, results, time.Since(suiteStarted))
	}()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	binDir, err := os.MkdirTemp("", "wox-smoke-bins-")
	if err != nil {
		return 1, fmt.Errorf("create smoke test binary directory: %w", err)
	}
	defer os.RemoveAll(binDir)
	if err := compileSmokeTargets(ctx, binDir, targets); err != nil {
		return 1, err
	}

	artifactRoot, err := suiteArtifactRoot()
	if err != nil {
		return 2, err
	}
	suiteDirectory, err := os.MkdirTemp(artifactRoot, "wox-smoke-suite-")
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
	woxDataDirectory := filepath.Join(suiteDirectory, "wox-data")
	userDataDirectory := filepath.Join(woxDataDirectory, "user-data")
	if caseSelector == "" || caseSelector == "launcher/plugin/url" || strings.HasPrefix(caseSelector, "launcher/plugin/url/") {
		if err := seedMissingFaviconURLHistoryFixture(woxDataDirectory, userDataDirectory); err != nil {
			retainSuiteDirectory = true
			return 1, err
		}
	}
	launchOptions := automationdriver.LaunchOptions{
		Environment: []string{
			"WOX_TEST_DATA_DIR=" + woxDataDirectory,
			"WOX_TEST_USER_DIR=" + userDataDirectory,
			fmt.Sprintf("WOX_TEST_SERVER_PORT=%d", port),
			"WOX_TEST_DISABLE_TELEMETRY=true",
			"WOX_TEST_SKIP_ONBOARDING=true",
			"WOX_DEBUG_REPAINT=verify",
		},
		StartupTimeout: 45 * time.Second,
	}
	launchProcess := func() (*automationdriver.Process, error) {
		process, err := automationdriver.Launch(ctx, absoluteExecutable, launchOptions)
		if err != nil {
			return nil, err
		}
		waitForAppIndexSettled(ctx, woxDataDirectory)
		return process, nil
	}
	process, err := launchProcess()
	if err != nil {
		retainSuiteDirectory = true
		return 1, fmt.Errorf("launch shared Wox smoke process: %w", err)
	}
	defer func() { _ = process.Close() }()

	testEnvironment := replaceEnvironment(os.Environ(), automationdriver.SharedInfoFileEnvironment, process.InfoFile())
	testEnvironment = replaceEnvironment(testEnvironment, automationdriver.SharedDataDirectoryEnvironment, woxDataDirectory)
	testEnvironment = replaceEnvironment(testEnvironment, automationdriver.SharedUserDataDirectoryEnvironment, userDataDirectory)
	suiteStarted = time.Now()
	for _, target := range targets {
		phaseCount := 1
		// ponytail: privacy is the only restart case; add lifecycle descriptors when a second case needs phases.
		if target.dir == "test/smoke/setting/privacy" {
			phaseCount = 4
		}
		if phaseCount > 1 {
			if err := process.Close(); err != nil {
				retainSuiteDirectory = true
				return 1, fmt.Errorf("close shared Wox before lifecycle test: %w", err)
			}
			process, err = launchProcess()
			if err != nil {
				retainSuiteDirectory = true
				return 1, fmt.Errorf("launch Wox for lifecycle test: %w", err)
			}
			testEnvironment = replaceEnvironment(testEnvironment, automationdriver.SharedInfoFileEnvironment, process.InfoFile())
		}
		for phase := 1; phase <= phaseCount; phase++ {
			commandEnv := testEnvironment
			phaseLabel := ""
			if phaseCount > 1 {
				phaseLabel = fmt.Sprintf("phase %d", phase)
				commandEnv = replaceEnvironment(commandEnv, automationdriver.SharedLifecyclePhaseEnvironment, fmt.Sprintf("%d", phase))
				commandEnv = replaceEnvironment(commandEnv, automationdriver.SharedLifecycleStateEnvironment, filepath.Join(suiteDirectory, "privacy-lifecycle.json"))
			}
			started := time.Now()
			caseResults, err := runSmokeTarget(ctx, binDir, target, commandEnv, phaseLabel)
			elapsed := time.Since(started).Round(10 * time.Millisecond)
			if err != nil && !smokeResultsHaveFailure(caseResults) {
				result := smokeCaseResult{
					name:    qualifySmokeName(target.dir, "(package)", phaseLabel),
					elapsed: elapsed,
					status:  smokeStatusFail,
				}
				fmt.Printf("smoke  %s  %s  %s\n", formatSmokeDuration(result.elapsed), result.status.label(), result.name)
				caseResults = append(caseResults, result)
			}
			results = append(results, caseResults...)
			if err != nil {
				retainSuiteDirectory = true
				fmt.Printf("FAIL\t%s\t%s\n", target.importPath(), elapsed)
				if state, exited := process.ExitState(); exited {
					fmt.Fprintf(os.Stderr, "shared Wox process died during %s: %s\n", target.importPath(), state)
				}
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) {
					return exitErr.ExitCode(), nil
				}
				return 1, fmt.Errorf("run smoke cases: %w", err)
			}
			fmt.Printf("ok  \t%s\t%s\n", target.importPath(), elapsed)
			if phase == phaseCount {
				continue
			}
			waitCtx, waitCancel := context.WithTimeout(ctx, 30*time.Second)
			waitErr := process.Wait(waitCtx)
			waitCancel()
			if waitErr != nil {
				retainSuiteDirectory = true
				return 1, fmt.Errorf("wait for lifecycle phase %d to exit Wox: %w", phase, waitErr)
			}
			if phase == 1 || phase == 3 {
				cleanupCtx, cleanupCancel := context.WithTimeout(ctx, 30*time.Second)
				cleanupErr := waitForPrivacyCleanup(cleanupCtx, woxDataDirectory)
				cleanupCancel()
				if cleanupErr != nil {
					retainSuiteDirectory = true
					return 1, fmt.Errorf("wait for lifecycle phase %d privacy cleanup: %w", phase, cleanupErr)
				}
			}
			if err := process.Close(); err != nil {
				retainSuiteDirectory = true
				return 1, fmt.Errorf("close lifecycle phase %d: %w", phase, err)
			}
			process, err = launchProcess()
			if err != nil {
				retainSuiteDirectory = true
				return 1, fmt.Errorf("restart Wox after lifecycle phase %d: %w", phase, err)
			}
			testEnvironment = replaceEnvironment(testEnvironment, automationdriver.SharedInfoFileEnvironment, process.InfoFile())
		}
	}
	if err := process.Close(); err != nil {
		retainSuiteDirectory = true
		return 1, fmt.Errorf("close shared Wox smoke process: %w", err)
	}
	return 0, nil
}

// suiteArtifactRoot returns the parent directory for the suite directory, or an
// empty string for the OS temp directory. CI sets WOX_SMOKE_ARTIFACT_DIR so a
// failing suite leaves its Wox data, logs, and settings inside the workspace,
// where the workflow can upload them; a temp directory dies with the runner.
func suiteArtifactRoot() (string, error) {
	root := strings.TrimSpace(os.Getenv(smokeArtifactDirEnvironment))
	if root == "" {
		return "", nil
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", fmt.Errorf("create smoke artifact directory %q: %w", root, err)
	}
	return root, nil
}

// appIndexSettleTimeout bounds the wait for a cold app index. A healthy runner
// settles in well under a second, but a Windows runner with a cold disk cache has
// been observed still scanning after 110 seconds, and giving up early is what puts
// the suite back inside the storm this wait exists to avoid. The budget therefore
// covers that worst case rather than the typical one; it costs nothing when the
// index is already warm.
const appIndexSettleTimeout = 3 * time.Minute

// appIndexCacheRelativePath is where the Apps plugin writes its index, which it
// only does once an indexing pass completes.
var appIndexCacheRelativePath = filepath.Join("cache", "wox-app-cache.json")

// waitForAppIndexSettled waits until a completed app index exists on disk. Wox
// answers automation RPCs as soon as its endpoint binds, while a cold app index
// keeps hammering CPU and disk long after that. Cases that start inside that window
// fail for machine load instead of behavior: an asynchronous settings save misses
// the action timeout, or a query fans out slowly enough that the result flush holds
// the UI thread past a synchronous RPC's budget. The privacy lifecycle package makes
// this systematic by wiping the data directory, which leaves the next package facing
// a cold index every run.
//
// The wait is best effort on purpose, and it stays correct on a machine with no
// applications because a completed empty pass still writes the cache.
func waitForAppIndexSettled(ctx context.Context, woxDataDirectory string) {
	cachePath := filepath.Join(woxDataDirectory, appIndexCacheRelativePath)
	deadline := time.Now().Add(appIndexSettleTimeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(cachePath); err == nil {
			return
		}
		if time.Now().After(deadline) {
			fmt.Fprintf(os.Stderr, "app index did not settle within %s; running cases anyway\n", appIndexSettleTimeout)
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// waitForPrivacyCleanup waits until the exit helper leaves only the profile needed by the next startup.
func waitForPrivacyCleanup(ctx context.Context, root string) error {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		entries, err := os.ReadDir(root)
		if err == nil && len(entries) == 1 && entries[0].Name() == "privacy.json" {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// seedMissingFaviconURLHistoryFixture prepares persisted URL history before the shared Wox process starts.
func seedMissingFaviconURLHistoryFixture(woxDataDirectory, userDataDirectory string) error {
	iconPath := smokefixture.MissingFaviconURLHistoryIconPath(woxDataDirectory)
	if err := os.MkdirAll(filepath.Dir(iconPath), 0755); err != nil {
		return fmt.Errorf("create URL smoke favicon directory: %w", err)
	}
	if err := os.WriteFile(iconPath, []byte("wox URL smoke favicon fixture"), 0644); err != nil {
		return fmt.Errorf("write URL smoke favicon fixture: %w", err)
	}

	historyJSON, err := json.Marshal([]map[string]any{
		{
			"Url": smokefixture.MissingFaviconURLHistoryURL,
			"Icon": map[string]string{
				"ImageType": "absolute",
				"ImageData": iconPath,
			},
			"Title": "Missing favicon smoke fixture",
		},
	})
	if err != nil {
		return fmt.Errorf("encode URL smoke history fixture: %w", err)
	}

	if err := os.MkdirAll(userDataDirectory, 0755); err != nil {
		return fmt.Errorf("create smoke user data directory: %w", err)
	}
	db, err := sql.Open("sqlite3", filepath.Join(userDataDirectory, "wox.db"))
	if err != nil {
		return fmt.Errorf("open smoke user database: %w", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS plugin_settings (
		plugin_id TEXT NOT NULL,
		key TEXT NOT NULL,
		value TEXT,
		is_local numeric NOT NULL DEFAULT false,
		PRIMARY KEY (plugin_id, key)
	)`); err != nil {
		return fmt.Errorf("create smoke plugin settings table: %w", err)
	}
	if _, err := db.Exec(
		"INSERT OR REPLACE INTO plugin_settings(plugin_id, key, value, is_local) VALUES (?, ?, ?, ?)",
		smokefixture.URLPluginID,
		"recentUrls",
		string(historyJSON),
		false,
	); err != nil {
		return fmt.Errorf("seed URL smoke history: %w", err)
	}

	return nil
}

// smokeTarget is one smoke package compiled into a single test binary.
type smokeTarget struct {
	// dir is the package directory relative to the wox.core module, using forward slashes.
	dir string
	// runPattern limits execution to one numbered case. Empty runs the whole package.
	runPattern string
}

func (target smokeTarget) importPath() string {
	return smokeModulePath + "/" + target.dir
}

// binaryName is the file go test -c -o <dir> writes for this package.
func (target smokeTarget) binaryName() string {
	name := filepath.Base(target.dir) + ".test"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// smokeTargets maps the selector to packages that still run one after another.
func smokeTargets(caseSelector string) ([]smokeTarget, error) {
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
		dirs := make([]string, 0, len(packages))
		for dir := range packages {
			dirs = append(dirs, dir)
		}
		sort.Strings(dirs)
		if len(dirs) == 0 {
			return nil, errors.New("no numbered smoke cases were found")
		}
		targets := make([]smokeTarget, 0, len(dirs))
		for _, dir := range dirs {
			targets = append(targets, smokeTarget{dir: dir})
		}
		return targets, nil
	}
	if packageSelectorPattern.MatchString(caseSelector) && !caseSelectorPattern.MatchString(caseSelector) {
		return smokePackageTarget(caseSelector)
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
	dir := "test/smoke/" + filepath.ToSlash(strings.TrimSuffix(directory, string(filepath.Separator)))
	return []smokeTarget{{dir: dir, runPattern: "^Test" + number}}, nil
}

// smokePackageTarget runs every numbered case in one smoke package, such as perf.
func smokePackageTarget(caseSelector string) ([]smokeTarget, error) {
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
	return []smokeTarget{{dir: "test/smoke/" + caseSelector}}, nil
}

// duplicateSmokeBinaries reports two packages that would overwrite one test binary.
// go test -c names the binary from the last import-path element.
func duplicateSmokeBinaries(targets []smokeTarget) error {
	seen := make(map[string]string, len(targets))
	for _, target := range targets {
		name := target.binaryName()
		if previous, ok := seen[name]; ok {
			return fmt.Errorf("smoke packages %s and %s both compile to %s", previous, target.dir, name)
		}
		seen[name] = target.dir
	}
	return nil
}

// smokeCompileArgs builds one go test -c command for every selected package.
func smokeCompileArgs(binDir string, targets []smokeTarget) []string {
	args := make([]string, 0, 6+len(targets))
	args = append(args, "test", "-tags", "wox_ui_smoke", "-c", "-o", binDir)
	for _, target := range targets {
		args = append(args, "./"+target.dir)
	}
	return args
}

// compileSmokeTargets writes every selected test binary before Wox starts.
func compileSmokeTargets(ctx context.Context, binDir string, targets []smokeTarget) error {
	fmt.Fprintf(os.Stderr, "compiling %d smoke package(s)\n", len(targets))
	command := exec.CommandContext(ctx, "go", smokeCompileArgs(binDir, targets)...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("compile smoke tests: %w", err)
	}
	for _, target := range targets {
		binary := filepath.Join(binDir, target.binaryName())
		if _, err := os.Stat(binary); err != nil {
			return fmt.Errorf("smoke test binary %s was not written: %w", binary, err)
		}
	}
	return nil
}

// smokeRunArgs are the test-binary flags that match the previous go test invocation.
func smokeRunArgs(target smokeTarget) []string {
	args := []string{"-test.failfast", "-test.count=1", "-test.v", "-test.timeout", smokePackageTimeout}
	if target.runPattern != "" {
		args = append(args, "-test.run", target.runPattern)
	}
	return args
}

type smokeStatus string

const (
	smokeStatusPass smokeStatus = "pass"
	smokeStatusFail smokeStatus = "fail"
	smokeStatusSkip smokeStatus = "skip"
)

// smokeCaseResult is one top-level smoke case captured from the test binary.
type smokeCaseResult struct {
	name    string
	elapsed time.Duration
	status  smokeStatus
}

func (status smokeStatus) label() string {
	switch status {
	case smokeStatusPass:
		return "PASS"
	case smokeStatusFail:
		return "FAIL"
	case smokeStatusSkip:
		return "SKIP"
	default:
		return string(status)
	}
}

// qualifySmokeName joins the package path under test/smoke with the test function.
func qualifySmokeName(packageDir, testName, phaseLabel string) string {
	name := strings.TrimPrefix(packageDir, "test/smoke/") + "/" + testName
	if phaseLabel != "" {
		name += " (" + phaseLabel + ")"
	}
	return name
}

var smokeResultLinePattern = regexp.MustCompile(`^--- (PASS|FAIL|SKIP): (.+) \(([0-9.]+s)\)$`)
var smokeRunLinePattern = regexp.MustCompile(`^=== RUN   (.+)$`)

// runSmokeTarget executes one already compiled package against the shared Wox process.
// It streams the test output and records each top-level case duration.
func runSmokeTarget(ctx context.Context, binDir string, target smokeTarget, env []string, phaseLabel string) ([]smokeCaseResult, error) {
	command := exec.CommandContext(ctx, filepath.Join(binDir, target.binaryName()), smokeRunArgs(target)...)
	command.Dir = filepath.FromSlash(target.dir)
	command.Env = env
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		return nil, err
	}
	results, scanErr := collectSmokeResults(stdout, os.Stdout, target.dir, phaseLabel)
	waitErr := command.Wait()
	if waitErr != nil {
		return results, waitErr
	}
	return results, scanErr
}

// collectSmokeResults echoes test output and keeps one row per top-level case.
// A case that starts and never reports a result is recorded as a failure so a
// crashed binary still appears in the summary.
func collectSmokeResults(reader io.Reader, echo io.Writer, packageDir, phaseLabel string) ([]smokeCaseResult, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	var results []smokeCaseResult
	pending := ""
	pendingAt := time.Time{}
	finishPending := func() {
		if pending == "" || strings.Contains(pending, "/") {
			pending = ""
			return
		}
		results = recordSmokeResult(echo, results, unfinishedSmokeResult(packageDir, pending, phaseLabel, pendingAt))
		pending = ""
	}
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		fmt.Fprintln(echo, line)
		if matches := smokeRunLinePattern.FindStringSubmatch(line); matches != nil {
			name := matches[1]
			// A nested run belongs to the current top-level case. Starting another
			// top-level case means the previous one never reported a result.
			if !strings.Contains(name, "/") {
				finishPending()
			}
			pending = name
			pendingAt = time.Now()
			continue
		}
		matches := smokeResultLinePattern.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		testName := matches[2]
		if pending == testName {
			pending = ""
		}
		if strings.Contains(testName, "/") {
			continue
		}
		elapsed, err := time.ParseDuration(matches[3])
		if err != nil {
			continue
		}
		status := smokeStatusPass
		switch matches[1] {
		case "FAIL":
			status = smokeStatusFail
		case "SKIP":
			status = smokeStatusSkip
		}
		results = recordSmokeResult(echo, results, smokeCaseResult{
			name:    qualifySmokeName(packageDir, testName, phaseLabel),
			elapsed: elapsed,
			status:  status,
		})
	}
	finishPending()
	return results, scanner.Err()
}

// recordSmokeResult keeps one case and prints its duration as soon as it finishes.
func recordSmokeResult(echo io.Writer, results []smokeCaseResult, result smokeCaseResult) []smokeCaseResult {
	fmt.Fprintf(echo, "smoke  %s  %s  %s\n", formatSmokeDuration(result.elapsed), result.status.label(), result.name)
	return append(results, result)
}

// unfinishedSmokeResult records a case the test binary started and did not finish.
func unfinishedSmokeResult(packageDir, testName, phaseLabel string, started time.Time) smokeCaseResult {
	elapsed := time.Duration(0)
	if !started.IsZero() {
		elapsed = time.Since(started)
	}
	return smokeCaseResult{
		name:    qualifySmokeName(packageDir, testName, phaseLabel),
		elapsed: elapsed,
		status:  smokeStatusFail,
	}
}

func smokeResultsHaveFailure(results []smokeCaseResult) bool {
	for _, result := range results {
		if result.status == smokeStatusFail {
			return true
		}
	}
	return false
}

// formatSmokeDuration renders a case or suite duration with two decimal places.
func formatSmokeDuration(elapsed time.Duration) string {
	if elapsed < 0 {
		elapsed = 0
	}
	elapsed = elapsed.Round(10 * time.Millisecond)
	if elapsed < time.Minute {
		return fmt.Sprintf("%.2fs", elapsed.Seconds())
	}
	minutes := int(elapsed / time.Minute)
	seconds := (elapsed % time.Minute).Seconds()
	return fmt.Sprintf("%dm%05.2fs", minutes, seconds)
}

// printSmokeSummary writes the case table and the wall-clock total.
// Cases are slowest first. The total stays last and includes resets and time between cases,
// so it can exceed the sum of the rows.
func printSmokeSummary(w io.Writer, results []smokeCaseResult, total time.Duration) {
	ordered := append([]smokeCaseResult(nil), results...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].elapsed > ordered[j].elapsed
	})
	nameWidth := len("TOTAL (wall)")
	durationWidth := len("DURATION")
	for _, result := range ordered {
		if width := len(result.name); width > nameWidth {
			nameWidth = width
		}
	}
	totalText := formatSmokeDuration(total)
	for _, result := range ordered {
		if width := len(formatSmokeDuration(result.elapsed)); width > durationWidth {
			durationWidth = width
		}
	}
	if width := len(totalText); width > durationWidth {
		durationWidth = width
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Smoke summary")
	fmt.Fprintln(w, "TOTAL (wall) includes resets and the time between cases.")
	fmt.Fprintf(w, "%-*s  %*s  %s\n", nameWidth, "NAME", durationWidth, "DURATION", "RESULT")
	for _, result := range ordered {
		fmt.Fprintf(w, "%-*s  %*s  %s\n", nameWidth, result.name, durationWidth, formatSmokeDuration(result.elapsed), result.status.label())
	}
	fmt.Fprintf(w, "%-*s  %*s  %s\n", nameWidth, "TOTAL (wall)", durationWidth, totalText, smokeResultCounts(results))
}

// smokeResultCounts summarizes the table for the total row.
func smokeResultCounts(results []smokeCaseResult) string {
	passed, failed, skipped := 0, 0, 0
	for _, result := range results {
		switch result.status {
		case smokeStatusPass:
			passed++
		case smokeStatusFail:
			failed++
		case smokeStatusSkip:
			skipped++
		}
	}
	parts := []string{fmt.Sprintf("%d passed", passed)}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", skipped))
	}
	return strings.Join(parts, ", ")
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
