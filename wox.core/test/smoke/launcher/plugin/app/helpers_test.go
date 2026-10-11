//go:build wox_ui_smoke

package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"wox/common"
	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	smokeAppName          = "WoxSmokeApp7f3a"
	smokeAppMarkerText    = "launched"
	appDirectoriesFieldID = "plugin-settings-field-1"
	// Custom-root updates publish their own completion after the query index changes.
	// Unrelated full scans must not satisfy a wait for an App Directories edit.
	appDirectoryUpdateLogMarker = "app directories updated: "
)

// smokeAppLaunchState tracks shared-process side effects that reset does not undo.
type smokeAppLaunchState struct {
	directoryPending bool
	launched         bool
}

// smokeAppFixtureDirectory is the custom App Directories root for this case.
func smokeAppFixtureDirectory(t *testing.T) string {
	t.Helper()
	dataDirectory := strings.TrimSpace(os.Getenv(automationdriver.SharedDataDirectoryEnvironment))
	if dataDirectory == "" {
		t.Fatalf("%s is not configured; run smoke through make smoke", automationdriver.SharedDataDirectoryEnvironment)
	}
	return filepath.Join(dataDirectory, "app-smoke")
}

// createSmokeAppFixture writes a platform launcher whose display name is smokeAppName.
// The Windows shortcut targets a private .cmd. Resolving it to cmd.exe would activate
// a console that is already running and would never write the marker.
func createSmokeAppFixture(t *testing.T, dir, marker string) {
	t.Helper()
	switch runtime.GOOS {
	case "windows", "linux", "darwin":
	default:
		t.Skip("app directory launch smoke supports Windows, macOS, and Linux")
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove previous app smoke fixture: %v", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create app smoke fixture: %v", err)
	}
	switch runtime.GOOS {
	case "windows":
		createWindowsSmokeApp(t, dir, marker)
	case "linux":
		createLinuxSmokeApp(t, dir, marker)
	case "darwin":
		createDarwinSmokeApp(t, dir, marker)
	}
}

func createWindowsSmokeApp(t *testing.T, dir, marker string) {
	t.Helper()
	commandPath := filepath.Join(dir, "launch.cmd")
	script := "@echo off\r\n> \"" + marker + "\" echo " + smokeAppMarkerText + "\r\nexit /b 0\r\n"
	if err := os.WriteFile(commandPath, []byte(script), 0o644); err != nil {
		t.Fatalf("write app smoke command: %v", err)
	}
	shortcutPath := filepath.Join(dir, smokeAppName+".lnk")
	powershell := strings.Join([]string{
		"$ErrorActionPreference = 'Stop'",
		"$shell = New-Object -ComObject WScript.Shell",
		"$shortcut = $shell.CreateShortcut(" + powershellSingleQuoted(shortcutPath) + ")",
		"$shortcut.TargetPath = " + powershellSingleQuoted(commandPath),
		"$shortcut.WindowStyle = 7",
		"$shortcut.WorkingDirectory = " + powershellSingleQuoted(dir),
		"$shortcut.Save()",
		"$written = $shell.CreateShortcut(" + powershellSingleQuoted(shortcutPath) + ")",
		"Write-Output $written.TargetPath",
	}, "\n")
	scriptPath := filepath.Join(dir, "create-shortcut.ps1")
	if err := os.WriteFile(scriptPath, []byte(powershell), 0o644); err != nil {
		t.Fatalf("write shortcut script: %v", err)
	}
	output, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", scriptPath).CombinedOutput()
	if err != nil {
		t.Fatalf("create app smoke shortcut: %v\n%s", err, output)
	}
	target := lastNonEmptyLine(string(output))
	if !pathsEqual(target, commandPath) {
		t.Fatalf("shortcut target = %q, want %q", target, commandPath)
	}
}

func createLinuxSmokeApp(t *testing.T, dir, marker string) {
	t.Helper()
	scriptPath := filepath.Join(dir, "launch.sh")
	writeExecutable(t, scriptPath, "#!/bin/sh\nprintf '%s\\n' '"+smokeAppMarkerText+"' > "+shellSingleQuote(marker)+"\n")
	desktopPath := filepath.Join(dir, smokeAppName+".desktop")
	desktop := "[Desktop Entry]\nType=Application\nName=" + smokeAppName + "\nExec=" + shellDoubleQuote(scriptPath) + "\nTerminal=false\n"
	writeExecutable(t, desktopPath, desktop)
	// gio launch refuses an untrusted desktop file. The product launches through gio.
	_ = exec.Command("gio", "set", desktopPath, "metadata::trusted", "true").Run()
}

func createDarwinSmokeApp(t *testing.T, dir, marker string) {
	t.Helper()
	bundlePath := filepath.Join(dir, smokeAppName+".app")
	contentsPath := filepath.Join(bundlePath, "Contents")
	macOSPath := filepath.Join(contentsPath, "MacOS")
	if err := os.MkdirAll(macOSPath, 0o755); err != nil {
		t.Fatalf("create app bundle: %v", err)
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleDisplayName</key>
	<string>%s</string>
	<key>CFBundleName</key>
	<string>%s</string>
	<key>CFBundleExecutable</key>
	<string>%s</string>
	<key>CFBundleIdentifier</key>
	<string>com.wox.smoke.%s</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleVersion</key>
	<string>1</string>
	<key>CFBundleShortVersionString</key>
	<string>1.0</string>
	<key>CFBundleInfoDictionaryVersion</key>
	<string>6.0</string>
</dict>
</plist>
`, smokeAppName, smokeAppName, smokeAppName, smokeAppName)
	if err := os.WriteFile(filepath.Join(contentsPath, "Info.plist"), []byte(plist), 0o644); err != nil {
		t.Fatalf("write app bundle Info.plist: %v", err)
	}
	writeExecutable(t, filepath.Join(macOSPath, smokeAppName), "#!/bin/sh\nprintf '%s\\n' '"+smokeAppMarkerText+"' > "+shellSingleQuote(marker)+"\n")
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
}

func powershellSingleQuoted(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func shellDoubleQuote(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value) + `"`
}

func lastNonEmptyLine(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		if line := strings.TrimSpace(lines[index]); line != "" {
			return line
		}
	}
	return ""
}

func smokeDataLogPath(t *testing.T) string {
	t.Helper()
	dataDirectory := strings.TrimSpace(os.Getenv(automationdriver.SharedDataDirectoryEnvironment))
	if dataDirectory == "" {
		t.Fatalf("%s is not configured; run smoke through make smoke", automationdriver.SharedDataDirectoryEnvironment)
	}
	return filepath.Join(dataDirectory, "log", "wox.log")
}

func appDirectoryUpdateCount(logPath string) int {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return 0
	}
	return strings.Count(string(data), appDirectoryUpdateLogMarker)
}

// waitForAppDirectoryUpdateAfter waits until a custom-directory update newer than before completes.
// An update already in flight can finish before it sees the settings change.
func waitForAppDirectoryUpdateAfter(t *testing.T, ctx context.Context, logPath string, before int) int {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	last := before
	for {
		last = appDirectoryUpdateCount(logPath)
		if last > before {
			return last
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for app directory update after %d completions (last %d): %v\n%s", before, last, ctx.Err(), logTail(logPath))
		case <-ticker.C:
		}
	}
}

func logTail(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return err.Error()
	}
	text := string(data)
	if len(text) > 800 {
		text = text[len(text)-800:]
	}
	return text
}

func openAppDirectorySettings(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	smoke.OpenInstalledPluginSettings(t, ctx, client, common.AppPluginID)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		add, found := automationdriver.Find(snapshot, appDirectoriesFieldID+"-add")
		return found && add.Enabled
	}); err != nil {
		t.Fatalf("wait for app directory settings: %v", err)
	}
}

func closeTableEditorIfOpen(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read table editor: %v", err)
	}
	if _, found := automationdriver.Find(snapshot, "form-table-row-cancel"); !found {
		return
	}
	if err := client.Perform(ctx, "form-table-row-cancel", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("close table editor: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, "form-table-row-save")
		return !found
	}); err != nil {
		t.Fatalf("wait for table editor to close: %v", err)
	}
}

// addSmokeAppDirectory appends the fixture directory through the App Directories table.
func addSmokeAppDirectory(t *testing.T, ctx context.Context, client *automationdriver.Client, dir string) {
	t.Helper()
	closeTableEditorIfOpen(t, ctx, client)
	rowIndex := smoke.ApplicationTableRowCount(t, ctx, client, appDirectoriesFieldID)
	if err := client.Perform(ctx, appDirectoriesFieldID+"-add", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("add app directory row: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, fieldFound := automationdriver.Find(snapshot, "form-table-row-field-0")
		_, saveFound := automationdriver.Find(snapshot, "form-table-row-save")
		return fieldFound && saveFound
	}); err != nil {
		t.Fatalf("wait for app directory editor: %v", err)
	}
	setFormTableText(t, ctx, client, 0, dir)
	if err := client.Perform(ctx, "form-table-row-save", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("save app directory: %v", err)
	}
	waitForSavedTableRow(t, ctx, client, appDirectoriesFieldID, rowIndex)
	cellID := fmt.Sprintf("%s-row-%d-cell-0", appDirectoriesFieldID, rowIndex)
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read saved app directory: %v", err)
	}
	cell, found := automationdriver.Find(snapshot, cellID)
	if !found || !pathsEqual(cell.Value, dir) {
		t.Fatalf("saved app directory = %q, want %q", cell.Value, dir)
	}
}

// removeSmokeAppDirectoryRows deletes every App Directories row for dir.
func removeSmokeAppDirectoryRows(t *testing.T, ctx context.Context, client *automationdriver.Client, dir string) int {
	t.Helper()
	closeTableEditorIfOpen(t, ctx, client)
	removed := 0
	for {
		snapshot, err := client.Snapshot(ctx)
		if err != nil {
			t.Fatalf("read app directory rows: %v", err)
		}
		rowIndex, found := tableRowByCell(snapshot, appDirectoriesFieldID, 0, func(value string) bool {
			return pathsEqual(value, dir)
		})
		if !found {
			return removed
		}
		smoke.RemoveApplicationTableRow(t, ctx, client, appDirectoriesFieldID, rowIndex)
		removed++
	}
}

// removeSmokeAppDirectoryAndWait drops the fixture directory and waits until search forgets it.
func removeSmokeAppDirectoryAndWait(t *testing.T, ctx context.Context, client *automationdriver.Client, dir string) {
	t.Helper()
	logPath := smokeDataLogPath(t)
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("hide before removing app directory: %v", err)
	}
	openAppDirectorySettings(t, ctx, client)
	before := appDirectoryUpdateCount(logPath)
	removed := removeSmokeAppDirectoryRows(t, ctx, client, dir)
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("hide after removing app directory: %v", err)
	}
	if removed == 0 {
		if _, _, found := querySmokeApp(t, ctx, client, dir); !found {
			return
		}
		before = appDirectoryUpdateCount(logPath)
	}
	waitUntilSmokeApp(t, ctx, client, logPath, before, dir, false)
}

// waitUntilSmokeApp waits for a directory update and checks whether the fixture launcher is listed.
func waitUntilSmokeApp(t *testing.T, ctx context.Context, client *automationdriver.Client, logPath string, before int, dir string, wantPresent bool) string {
	t.Helper()
	after := waitForAppDirectoryUpdateAfter(t, ctx, logPath, before)
	snapshot, resultID, found := querySmokeApp(t, ctx, client, dir)
	if found == wantPresent {
		smoke.AssertNoDiagnostics(t, snapshot)
		return resultID
	}
	if after == before+1 {
		waitForAppDirectoryUpdateAfter(t, ctx, logPath, after)
		snapshot, resultID, found = querySmokeApp(t, ctx, client, dir)
		if found == wantPresent {
			smoke.AssertNoDiagnostics(t, snapshot)
			return resultID
		}
	}
	if wantPresent {
		t.Fatalf("query %q did not show the app in %s; results: %s", smokeAppName, dir, formatLauncherResults(snapshot))
	}
	t.Fatalf("query %q still shows the app after the directory was removed; results: %s", smokeAppName, formatLauncherResults(snapshot))
	return ""
}

func querySmokeApp(t *testing.T, ctx context.Context, client *automationdriver.Client, dir string) (woxwidget.AutomationSnapshot, string, bool) {
	t.Helper()
	smoke.ShowLauncher(t, ctx, client)
	snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, smokeAppName)
	resultID, found := smokeAppResultID(snapshot, dir)
	return snapshot, resultID, found
}

func smokeAppResultID(snapshot woxwidget.AutomationSnapshot, dir string) (string, bool) {
	for _, node := range snapshot.Tree.Nodes {
		if !strings.HasPrefix(node.AutomationID, "launcher.result.") || strings.TrimSpace(node.Label) != smokeAppName {
			continue
		}
		if pathTextHasDirectory(node.Description, dir) {
			return node.AutomationID, true
		}
	}
	return "", false
}

func formatLauncherResults(snapshot woxwidget.AutomationSnapshot) string {
	parts := make([]string, 0)
	for _, node := range snapshot.Tree.Nodes {
		if !strings.HasPrefix(node.AutomationID, "launcher.result.") {
			continue
		}
		parts = append(parts, node.Label+" | "+node.Description)
		if len(parts) == 20 {
			break
		}
	}
	if len(parts) == 0 {
		return "(no results)"
	}
	return strings.Join(parts, "; ")
}

func waitForSavedTableRow(t *testing.T, ctx context.Context, client *automationdriver.Client, fieldID string, rowIndex int) {
	t.Helper()
	editID := fmt.Sprintf("%s-row-%d-edit", fieldID, rowIndex)
	deleteID := fmt.Sprintf("%s-row-%d-delete", fieldID, rowIndex)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		edit, editFound := automationdriver.Find(snapshot, editID)
		remove, deleteFound := automationdriver.Find(snapshot, deleteID)
		add, addFound := automationdriver.Find(snapshot, fieldID+"-add")
		_, editorFound := automationdriver.Find(snapshot, "form-table-row-save")
		return editFound && edit.Enabled && deleteFound && remove.Enabled && addFound && add.Enabled && !editorFound
	}); err != nil {
		t.Fatalf("wait for %s row %d: %v", fieldID, rowIndex, err)
	}
}

func setFormTableText(t *testing.T, ctx context.Context, client *automationdriver.Client, index int, value string) {
	t.Helper()
	fieldID := "form-table-row-field-" + strconv.Itoa(index)
	if err := client.Perform(ctx, fieldID, woxui.AccessibilityActionSetValue, value); err != nil {
		t.Fatalf("set table field %d: %v", index, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		field, found := automationdriver.Find(snapshot, fieldID)
		return found && field.Value == value
	}); err != nil {
		t.Fatalf("confirm table field %d: %v", index, err)
	}
}

func tableRowByCell(snapshot woxwidget.AutomationSnapshot, fieldID string, column int, matches func(string) bool) (int, bool) {
	prefix := fieldID + "-row-"
	suffix := fmt.Sprintf("-cell-%d", column)
	for _, node := range snapshot.Tree.Nodes {
		if !strings.HasPrefix(node.AutomationID, prefix) || !strings.HasSuffix(node.AutomationID, suffix) {
			continue
		}
		if !matches(node.Value) {
			continue
		}
		middle := strings.TrimSuffix(strings.TrimPrefix(node.AutomationID, prefix), suffix)
		index, err := strconv.Atoi(middle)
		if err != nil {
			continue
		}
		return index, true
	}
	return 0, false
}

func pathsEqual(left, right string) bool {
	left = filepath.Clean(strings.TrimSpace(left))
	right = filepath.Clean(strings.TrimSpace(right))
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func pathTextHasDirectory(value, dir string) bool {
	value = filepath.ToSlash(value)
	dir = filepath.ToSlash(filepath.Clean(dir))
	if runtime.GOOS == "windows" {
		value = strings.ToLower(value)
		dir = strings.ToLower(dir)
	}
	return value == dir || strings.HasPrefix(value, dir+"/")
}

// waitForLaunchMarker polls the fixture marker until the launched process writes it.
func waitForLaunchMarker(t *testing.T, ctx context.Context, marker string) {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var last string
	var lastErr error
	for {
		data, err := os.ReadFile(marker)
		if err == nil {
			last = string(data)
			lastErr = nil
			if strings.TrimSpace(last) == smokeAppMarkerText {
				return
			}
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for app launch marker %s: last=%q err=%v: %v", marker, last, lastErr, ctx.Err())
		case <-ticker.C:
		}
	}
}

// cleanupSmokeAppLaunch restores App Directories and removes the fixture even when the case fails.
func cleanupSmokeAppLaunch(t *testing.T, client *automationdriver.Client, dir string, state *smokeAppLaunchState) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), smoke.CaseTimeout)
	defer cancel()
	if state.directoryPending {
		removeSmokeAppDirectoryAndWait(t, ctx, client, dir)
		state.directoryPending = false
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Errorf("remove app smoke fixture %s: %v", dir, err)
	}
	if state.launched {
		deleteSmokeAppMRU(t, smokeAppName)
	}
}

// deleteSmokeAppMRU removes the one-use record created by launching the fixture.
// A single launch stays below the start-page threshold, so a missing row is acceptable.
func deleteSmokeAppMRU(t *testing.T, title string) {
	t.Helper()
	userDataDirectory := strings.TrimSpace(os.Getenv(automationdriver.SharedUserDataDirectoryEnvironment))
	if userDataDirectory == "" {
		t.Errorf("%s is not configured", automationdriver.SharedUserDataDirectoryEnvironment)
		return
	}
	database, err := sql.Open("sqlite3", filepath.Join(userDataDirectory, "wox.db")+"?_busy_timeout=5000")
	if err != nil {
		t.Errorf("open smoke database: %v", err)
		return
	}
	defer database.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		var count int
		err = database.QueryRowContext(ctx, "SELECT COUNT(*) FROM mru_records WHERE plugin_id = ? AND title = ?", common.AppPluginID, title).Scan(&count)
		if err == nil && count > 0 {
			break
		}
		select {
		case <-ctx.Done():
			err = nil
			count = 0
		case <-ticker.C:
			continue
		}
		break
	}
	if _, err := database.Exec("DELETE FROM mru_records WHERE plugin_id = ? AND title = ?", common.AppPluginID, title); err != nil && !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("delete app smoke MRU %q: %v", title, err)
	}
}
