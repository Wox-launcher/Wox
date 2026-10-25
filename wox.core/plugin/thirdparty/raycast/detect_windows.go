//go:build windows

package raycast

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func detectInstalledRaycast() (string, string) {
	for _, path := range windowsRaycastCandidates() {
		if regularFile(path) {
			return path, ""
		}
	}
	if path, version := raycastFromUninstall(); path != "" {
		return path, version
	}
	// Store installs register an AppX package and do not leave a Win32 uninstall entry or a Start Menu shortcut.
	if path, version := raycastFromAppx(); path != "" {
		return path, version
	}
	if path := windowsStartMenuRaycast(); path != "" {
		return path, ""
	}
	return "", ""
}

func windowsRaycastCandidates() []string {
	var paths []string
	if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
		paths = append(paths,
			filepath.Join(local, "Programs", "Raycast", "Raycast.exe"),
			filepath.Join(local, "Raycast", "Raycast.exe"),
		)
	}
	if program := strings.TrimSpace(os.Getenv("ProgramFiles")); program != "" {
		paths = append(paths, filepath.Join(program, "Raycast", "Raycast.exe"))
	}
	if program := strings.TrimSpace(os.Getenv("ProgramFiles(x86)")); program != "" {
		paths = append(paths, filepath.Join(program, "Raycast", "Raycast.exe"))
	}
	return paths
}

func windowsStartMenuRaycast() string {
	roots := []string{}
	if roaming := strings.TrimSpace(os.Getenv("APPDATA")); roaming != "" {
		roots = append(roots, filepath.Join(roaming, "Microsoft", "Windows", "Start Menu", "Programs"))
	}
	if programData := strings.TrimSpace(os.Getenv("ProgramData")); programData != "" {
		roots = append(roots, filepath.Join(programData, "Microsoft", "Windows", "Start Menu", "Programs"))
	}
	for _, root := range roots {
		for _, name := range []string{"Raycast.lnk", filepath.Join("Raycast", "Raycast.lnk")} {
			path := filepath.Join(root, name)
			if regularFile(path) {
				return path
			}
		}
	}
	return ""
}

func raycastFromUninstall() (string, string) {
	roots := []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE}
	paths := []string{
		`Software\Microsoft\Windows\CurrentVersion\Uninstall`,
		`Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`,
	}
	for _, root := range roots {
		for _, path := range paths {
			if location, version := raycastUninstallKey(root, path); location != "" {
				return location, version
			}
		}
	}
	return "", ""
}

const appxPackagesKey = `Software\Classes\Local Settings\Software\Microsoft\Windows\CurrentVersion\AppModel\Repository\Packages`

func raycastFromAppx() (string, string) {
	var bestPath, bestVersion string
	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		key, err := registry.OpenKey(root, appxPackagesKey, registry.ENUMERATE_SUB_KEYS)
		if err != nil {
			continue
		}
		names, err := key.ReadSubKeyNames(0)
		key.Close()
		if err != nil {
			continue
		}
		for _, name := range names {
			if !isRaycastAppxPackage(name) {
				continue
			}
			sub, err := registry.OpenKey(root, appxPackagesKey+`\`+name, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			folder, _, _ := sub.GetStringValue("PackageRootFolder")
			sub.Close()
			path := appxRaycastPath(folder)
			if path == "" {
				continue
			}
			version := appxPackageVersion(name)
			if bestPath == "" || appxVersionNewer(version, bestVersion) {
				bestPath, bestVersion = path, version
			}
		}
	}
	return bestPath, bestVersion
}

func isRaycastAppxPackage(name string) bool {
	return strings.HasPrefix(name, "Raycast.Raycast_")
}

func appxPackageVersion(name string) string {
	rest := strings.TrimPrefix(name, "Raycast.Raycast_")
	version, _, _ := strings.Cut(rest, "_")
	return version
}

func appxRaycastPath(folder string) string {
	folder = strings.TrimSpace(folder)
	if folder == "" {
		return ""
	}
	exe := filepath.Join(folder, "Raycast", "Raycast.exe")
	if regularFile(exe) {
		return exe
	}
	info, err := os.Stat(folder)
	if err == nil && info.IsDir() {
		return folder
	}
	return ""
}

func appxVersionNewer(candidate, current string) bool {
	left := strings.Split(candidate, ".")
	right := strings.Split(current, ".")
	count := len(left)
	if len(right) > count {
		count = len(right)
	}
	for index := 0; index < count; index++ {
		var a, b int
		if index < len(left) {
			a, _ = strconv.Atoi(left[index])
		}
		if index < len(right) {
			b, _ = strconv.Atoi(right[index])
		}
		if a != b {
			return a > b
		}
	}
	return false
}

func raycastUninstallKey(root registry.Key, path string) (string, string) {
	key, err := registry.OpenKey(root, path, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return "", ""
	}
	defer key.Close()
	names, err := key.ReadSubKeyNames(0)
	if err != nil {
		return "", ""
	}
	for _, name := range names {
		sub, err := registry.OpenKey(key, name, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		display, _, _ := sub.GetStringValue("DisplayName")
		if !strings.EqualFold(strings.TrimSpace(display), "Raycast") {
			sub.Close()
			continue
		}
		version, _, _ := sub.GetStringValue("DisplayVersion")
		location, _, _ := sub.GetStringValue("InstallLocation")
		icon, _, _ := sub.GetStringValue("DisplayIcon")
		sub.Close()
		if regularFile(icon) {
			return strings.Trim(icon, `"`), version
		}
		exe := filepath.Join(location, "Raycast.exe")
		if regularFile(exe) {
			return exe, version
		}
		if strings.TrimSpace(location) != "" {
			return location, version
		}
	}
	return "", ""
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
