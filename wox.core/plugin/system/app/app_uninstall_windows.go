package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"wox/util/shell"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const windowsUninstallCacheTTL = 30 * time.Second

var (
	errWindowsUninstallNotFound    = errors.New("windows uninstall information not found")
	errWindowsUninstallNotAllowed  = errors.New("windows app cannot be uninstalled")
	errWindowsUninstallUnsupported = errors.New("windows uninstall is not supported for this app")

	windowsUninstallCacheMu      sync.Mutex
	windowsUninstallCache        []windowsUninstallEntry
	windowsUninstallCacheExpires time.Time
)

func isWindowsUninstallNotFound(err error) bool {
	return errors.Is(err, errWindowsUninstallNotFound)
}

func isWindowsUninstallNotAllowed(err error) bool {
	return errors.Is(err, errWindowsUninstallNotAllowed)
}

func executeWindowsUninstall(ctx context.Context, info appInfo) error {
	if info.Type == AppTypeUWP {
		familyName := uwpPackageFamilyName(info.Path)
		if familyName == "" {
			return errWindowsUninstallNotFound
		}
		return uninstallUWPPackage(familyName)
	}
	if info.Type != AppTypeDesktop {
		return errWindowsUninstallUnsupported
	}

	entry := findWindowsUninstallEntry(info, collectWindowsUninstallTargetPaths(ctx, info), getWindowsUninstallEntries())
	if entry == nil {
		return errWindowsUninstallNotFound
	}

	file, parameters := splitUninstallCommand(entry.UninstallString)
	file, parameters = rewriteMsiUninstallCommand(file, parameters, entry.WindowsInstaller, entry.KeyName)
	if strings.TrimSpace(file) == "" {
		return errWindowsUninstallNotFound
	}

	return launchWindowsUninstallCommand(file, parameters)
}

func collectWindowsUninstallTargetPaths(ctx context.Context, info appInfo) []string {
	seen := map[string]struct{}{}
	var paths []string
	add := func(path string) {
		clean := canonicalizeWindowsPath(path)
		if clean == "" {
			return
		}
		if _, exists := seen[clean]; exists {
			return
		}
		seen[clean] = struct{}{}
		paths = append(paths, path)
	}

	add(info.Path)
	add(info.IconSourcePath)
	if strings.EqualFold(filepath.Ext(info.Path), ".lnk") {
		if target, err := resolveShortcutTarget(ctx, info.Path); err == nil {
			add(target)
		}
	}

	return paths
}

func getWindowsUninstallEntries() []windowsUninstallEntry {
	now := time.Now()
	windowsUninstallCacheMu.Lock()
	defer windowsUninstallCacheMu.Unlock()
	if windowsUninstallCache != nil && now.Before(windowsUninstallCacheExpires) {
		return windowsUninstallCache
	}

	windowsUninstallCache = loadWindowsUninstallEntries()
	windowsUninstallCacheExpires = now.Add(windowsUninstallCacheTTL)
	return windowsUninstallCache
}

func loadWindowsUninstallEntries() []windowsUninstallEntry {
	roots := []struct {
		key  registry.Key
		path string
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.CURRENT_USER, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
	}

	var entries []windowsUninstallEntry
	for _, root := range roots {
		entries = append(entries, readWindowsUninstallRoot(root.key, root.path)...)
	}
	return entries
}

func readWindowsUninstallRoot(root registry.Key, path string) []windowsUninstallEntry {
	key, err := registry.OpenKey(root, path, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer key.Close()

	names, err := key.ReadSubKeyNames(-1)
	if err != nil {
		return nil
	}

	entries := make([]windowsUninstallEntry, 0, len(names))
	for _, name := range names {
		subKey, openErr := registry.OpenKey(key, name, registry.QUERY_VALUE)
		if openErr != nil {
			continue
		}
		entry, ok := readWindowsUninstallEntry(subKey, name)
		subKey.Close()
		if ok {
			entries = append(entries, entry)
		}
	}
	return entries
}

func readWindowsUninstallEntry(key registry.Key, keyName string) (windowsUninstallEntry, bool) {
	displayName, _, err := key.GetStringValue("DisplayName")
	if err != nil || strings.TrimSpace(displayName) == "" {
		return windowsUninstallEntry{}, false
	}

	uninstallString, _, _ := key.GetStringValue("UninstallString")
	installLocation, _, _ := key.GetStringValue("InstallLocation")
	displayIcon, _, _ := key.GetStringValue("DisplayIcon")

	return windowsUninstallEntry{
		DisplayName:      displayName,
		UninstallString:  uninstallString,
		InstallLocation:  installLocation,
		DisplayIcon:      displayIcon,
		KeyName:          keyName,
		NoRemove:         readWindowsRegistryBool(key, "NoRemove"),
		WindowsInstaller: readWindowsRegistryBool(key, "WindowsInstaller"),
	}, true
}

func readWindowsRegistryBool(key registry.Key, name string) bool {
	if value, _, err := key.GetIntegerValue(name); err == nil {
		return value != 0
	}
	if value, _, err := key.GetStringValue(name); err == nil {
		return value == "1" || strings.EqualFold(value, "true")
	}
	return false
}

func launchWindowsUninstallCommand(file string, parameters string) error {
	err := shell.OpenWithParameters(file, parameters)
	if err == nil {
		return nil
	}
	if !errors.Is(err, windows.ERROR_ELEVATION_REQUIRED) {
		return err
	}

	_, elevatedErr := shell.RunElevated(file, parameters, "")
	return elevatedErr
}

func uninstallUWPPackage(packageFamilyName string) error {
	escapedFamily := strings.ReplaceAll(packageFamilyName, "'", "''")
	script := fmt.Sprintf(`
		[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
		$ErrorActionPreference = 'Stop'
		$family = '%s'
		$pkg = Get-AppxPackage | Where-Object { $_.PackageFamilyName -eq $family } | Select-Object -First 1
		if (-not $pkg) { Write-Error 'package not found'; exit 2 }
		if ($pkg.NonRemovable) { Write-Error 'package is not removable'; exit 3 }
		$pkg | Remove-AppxPackage
	`, escapedFamily)

	_, err := shell.RunOutput("powershell", "-NoProfile", "-Command", script)
	if err == nil {
		return nil
	}

	message := strings.ToLower(err.Error())
	if strings.Contains(message, "exit status 2") || strings.Contains(message, "package not found") {
		return errWindowsUninstallNotFound
	}
	if strings.Contains(message, "exit status 3") || strings.Contains(message, "not removable") {
		return errWindowsUninstallNotAllowed
	}
	return err
}
