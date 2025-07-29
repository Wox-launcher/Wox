package resource

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path"
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

//go:embed script_plugin_templates
var ScriptPluginTemplatesFS embed.FS

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

	// others
	othersDirectory := util.GetLocation().GetOthersDirectory()
	if util.IsDirExists(othersDirectory) {
		rmErr := os.RemoveAll(othersDirectory)
		if rmErr != nil {
			return rmErr
		}
	}
	othersErr := extractFiles(ctx, OthersFS, othersDirectory, "others", false)
	if othersErr != nil {
		return othersErr
	}

	// script_plugin_templates
	scriptPluginTemplatesDirectory := util.GetLocation().GetScriptPluginTemplatesDirectory()
	if util.IsDirExists(scriptPluginTemplatesDirectory) {
		rmErr := os.RemoveAll(scriptPluginTemplatesDirectory)
		if rmErr != nil {
			return rmErr
		}
	}
	scriptPluginTemplatesErr := extractFiles(ctx, ScriptPluginTemplatesFS, scriptPluginTemplatesDirectory, "script_plugin_templates", false)
	if scriptPluginTemplatesErr != nil {
		return scriptPluginTemplatesErr
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

func GetAppIcon() []byte {
	if util.IsWindows() {
		return appIconWindows
	}

	return appIcon
}
