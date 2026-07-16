package resource

import (
	"context"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"wox/util"
)

//go:embed hosts
var HostFS embed.FS

//go:embed lang
var LangFS embed.FS

//go:embed ui
var UIFS embed.FS

//go:embed app.png
var appIcon []byte

//go:embed app.ico
var appIconWindows []byte

//go:embed others
var OthersFS embed.FS

var embedThemes = []string{}

func Extract(ctx context.Context) error {
	start := util.GetSystemTimestamp()

	// hosts
	hostDirectory := util.GetLocation().GetHostDirectory()
	if util.IsDirExists(hostDirectory) {
		rmErr := os.RemoveAll(hostDirectory)
		if rmErr != nil {
			return rmErr
		}
	}
	extractHostErr := extractFiles(ctx, HostFS, hostDirectory, "hosts", false)
	if extractHostErr != nil {
		return extractHostErr
	}

	// ui
	uiDiretory := util.GetLocation().GetUIDirectory()
	if util.IsDirExists(uiDiretory) {
		rmErr := os.RemoveAll(uiDiretory)
		if rmErr != nil {
			return rmErr
		}
	}
	flutterErr := extractFiles(ctx, UIFS, uiDiretory, "ui/flutter", true)
	if flutterErr != nil {
		return flutterErr
	}
	goUIErr := extractFiles(ctx, UIFS, uiDiretory, "ui/go", true)
	if goUIErr != nil {
		return goUIErr
	}

	// others
	othersDirectory := util.GetLocation().GetOthersDirectory()
	if util.IsDirExists(othersDirectory) {
		rmErr := os.RemoveAll(othersDirectory)
		if rmErr != nil {
			return rmErr
		}
	}
	// Enable recursive extraction for nested directories such as others/dictation and others/woxmr.
	othersErr := extractFiles(ctx, OthersFS, othersDirectory, "others", true)
	if othersErr != nil {
		return othersErr
	}

	// themes
	themeErr := parseThemes(ctx)
	if themeErr != nil {
		return themeErr
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("extracted embed files, cost: %dms", util.GetSystemTimestamp()-start))
	return nil
}

func extractFiles(ctx context.Context, fs embed.FS, extractDirectory string, filePath string, recursive bool) error {
	dir, err := fs.ReadDir(filePath)
	if err != nil {
		return err
	}
	if len(dir) == 0 {
		return fmt.Errorf("no host file found")
	}

	extractDirectoryPath := path.Join(extractDirectory, strings.Join(strings.Split(filePath, "/")[1:], "/"))
	createDirErr := util.GetLocation().EnsureDirectoryExist(extractDirectoryPath)
	if createDirErr != nil {
		return createDirErr
	}

	for _, entry := range dir {
		if entry.IsDir() && recursive {
			extractErr := extractFiles(ctx, fs, extractDirectory, path.Join(filePath, entry.Name()), recursive)
			if extractErr != nil {
				return extractErr
			}
			continue
		}

		fileData, readErr := fs.ReadFile(path.Join(filePath, entry.Name()))
		if readErr != nil {
			return readErr
		}

		var subFilePath = path.Join(extractDirectoryPath, entry.Name())
		writeErr := os.WriteFile(subFilePath, fileData, 0644)
		if writeErr != nil {
			return writeErr
		}
	}

	return nil
}

func parseThemes(ctx context.Context) error {
	dir, err := UIFS.ReadDir(path.Join("ui", "themes"))
	if err != nil {
		return err
	}
	if len(dir) == 0 {
		return fmt.Errorf("no theme file found")
	}

	for _, entry := range dir {
		if entry.IsDir() {
			continue
		}

		start := util.GetSystemTimestamp()
		themeData, readErr := UIFS.ReadFile("ui/themes/" + entry.Name())
		if readErr != nil {
			return readErr
		}

		embedThemes = append(embedThemes, string(themeData))
		util.GetLogger().Info(ctx, fmt.Sprintf("extracted theme file: %s, cost: %dms", entry.Name(), util.GetSystemTimestamp()-start))
	}

	return nil
}

func GetLangJson(ctx context.Context, langCode string) ([]byte, error) {
	var langJsonPath = path.Join("lang", fmt.Sprintf("%s.json", langCode))
	return LangFS.ReadFile(langJsonPath)
}

func GetEmbedThemes(ctx context.Context) []string {
	return embedThemes
}

// GetDictationFile returns the embedded dictation resource bytes by name.
func GetDictationFile(name string) ([]byte, error) {
	return OthersFS.ReadFile(path.Join("others", "dictation", name))
}

// GetDictationResourcePath returns the extracted path for a dictation resource.
func GetDictationResourcePath(name string) string {
	return filepath.Join(util.GetLocation().GetOthersDirectory(), "dictation", filepath.FromSlash(name))
}

func GetAppIcon() []byte {
	if util.IsWindows() {
		return appIconWindows
	}

	return appIcon
}

// EnsureLinuxDesktopIcon installs Wox's icon into the user icon theme so the
// generated desktop entry resolves correctly for AppImage and manual binaries.
func EnsureLinuxDesktopIcon(ctx context.Context) {
	if !util.IsLinux() {
		return
	}

	iconPath, err := util.LinuxDesktopIconPath()
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to get Linux desktop icon path: %s", err.Error()))
		return
	}

	if err := os.MkdirAll(filepath.Dir(iconPath), 0755); err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to create Linux desktop icon directory: %s", err.Error()))
		return
	}
	if err := os.WriteFile(iconPath, appIcon, 0644); err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to write Linux desktop icon: %s", err.Error()))
		return
	}

	iconThemePath, err := util.LinuxIconThemePath()
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to get Linux icon theme path: %s", err.Error()))
		return
	}
	if err := exec.Command("gtk-update-icon-cache", "-q", "-t", "-f", iconThemePath).Run(); err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to update Linux icon cache: %s", err.Error()))
	}
}
