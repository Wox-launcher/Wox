package util

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

const homebrewWoxAppImage = "wox-linux-amd64.AppImage"

type homebrewAppImageInstall struct {
	managed      bool
	desktopEntry string
}

// Keep the install identity for the lifetime of this process, including while
// Brew is replacing or removing the package files.
var homebrewAppImage = sync.OnceValue(detectHomebrewAppImage)

func IsHomebrewAppImage() bool {
	return homebrewAppImage().managed
}

// detectHomebrewAppImage uses Brew's staged AppImage to identify the
// install even when Wox is opened directly and no desktop launcher ran.
func detectHomebrewAppImage() homebrewAppImageInstall {
	image := os.Getenv("APPIMAGE")
	if image == "" {
		return homebrewAppImageInstall{}
	}
	image, err := filepath.EvalSymlinks(image)
	if err != nil {
		return homebrewAppImageInstall{}
	}

	// The Brew-generated icon identifies custom prefixes even when brew is
	// missing from the desktop session's PATH.
	defaultEntry, _ := defaultLinuxDesktopEntryPath()
	desktopEntries := []string{os.Getenv("GIO_LAUNCHED_DESKTOP_FILE"), defaultEntry}
	if homeDir, err := os.UserHomeDir(); err == nil {
		// Older Brew installs could put the entry in the standard directory even with XDG_DATA_HOME set.
		desktopEntries = append(desktopEntries, filepath.Join(homeDir, ".local", "share", "applications", linuxDesktopFile))
	}
	if dataHome := os.Getenv("HOMEBREW_XDG_DATA_HOME"); filepath.IsAbs(dataHome) {
		desktopEntries = append(desktopEntries, filepath.Join(dataHome, "applications", linuxDesktopFile))
	}
	for _, entry := range desktopEntries {
		if install := homebrewInstallForDesktopEntry(entry, image); install.managed {
			return install
		}
	}

	// Still recognize an installed AppImage if its desktop entry was replaced
	// or removed, or if Brew predates AppImage desktop registration.
	prefixes := []string{os.Getenv("HOMEBREW_PREFIX"), "/home/linuxbrew/.linuxbrew"}
	if brew, err := exec.LookPath("brew"); err == nil {
		prefixes = append(prefixes, filepath.Dir(filepath.Dir(brew)))
	}
	for _, prefix := range prefixes {
		if !filepath.IsAbs(prefix) {
			continue
		}
		cask := filepath.Join(prefix, "Caskroom", "wox")
		versions, _ := os.ReadDir(cask)
		for _, version := range versions {
			if !version.IsDir() {
				continue
			}
			if install := homebrewInstallForCask(filepath.Join(cask, version.Name()), image); install.managed {
				return install
			}
		}
	}
	return homebrewAppImageInstall{}
}

// homebrewInstallForDesktopEntry recovers the Caskroom directory from the
// absolute icon path that Homebrew writes into the installed desktop entry.
func homebrewInstallForDesktopEntry(path, image string) homebrewAppImageInstall {
	if !filepath.IsAbs(path) || filepath.Base(path) != linuxDesktopFile {
		return homebrewAppImageInstall{}
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return homebrewAppImageInstall{}
	}
	file, err := os.ReadFile(path)
	if err != nil {
		return homebrewAppImageInstall{}
	}
	for line := range strings.SplitSeq(string(file), "\n") {
		icon, ok := strings.CutPrefix(strings.TrimSpace(line), "Icon=")
		if !ok || !filepath.IsAbs(icon) || filepath.Base(icon) != linuxDesktopIconFile {
			continue
		}
		dir := filepath.Dir(icon)
		cask := filepath.Dir(dir)
		if filepath.Base(cask) == "wox" && filepath.Base(filepath.Dir(cask)) == "Caskroom" {
			return homebrewInstallForCask(dir, image)
		}
	}
	return homebrewAppImageInstall{}
}

// homebrewInstallForCask matches the AppImage in both Caskroom layouts: older
// Brew releases kept the file here, and newer releases leave a symlink to it.
func homebrewInstallForCask(dir, image string) homebrewAppImageInstall {
	target, err := filepath.EvalSymlinks(filepath.Join(dir, homebrewWoxAppImage))
	if err != nil || target != image {
		return homebrewAppImageInstall{}
	}
	// The desktop symlink records the file Brew will remove on uninstall.
	desktopEntry, _ := os.Readlink(filepath.Join(dir, linuxDesktopFile))
	if filepath.Base(desktopEntry) != linuxDesktopFile {
		desktopEntry = ""
	} else if !filepath.IsAbs(desktopEntry) {
		desktopEntry = filepath.Join(dir, desktopEntry)
	}
	return homebrewAppImageInstall{managed: true, desktopEntry: desktopEntry}
}
