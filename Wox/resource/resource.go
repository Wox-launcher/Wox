package resource

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path"
	"wox/util"
)

//go:embed hosts
var HostFS embed.FS

//go:embed lang
var LangFS embed.FS

//go:embed ui
var UIFS embed.FS

var embedThemes = []string{}

func Extract(ctx context.Context) error {
	extractErr := extractHosts(ctx)
	if extractErr != nil {
		return extractErr
	}

	extractErr = extractUI(ctx)
	if extractErr != nil {
		return extractErr
	}

	extractErr = extractThemes(ctx)
	if extractErr != nil {
		return extractErr
	}

	return nil
}

func extractHosts(ctx context.Context) error {
	dir, err := HostFS.ReadDir("hosts")
	if err != nil {
		return err
	}
	if len(dir) == 0 {
		return fmt.Errorf("no host file found")
	}

	for _, entry := range dir {
		start := util.GetSystemTimestamp()
		hostData, readErr := HostFS.ReadFile("hosts/" + entry.Name())
		if readErr != nil {
			return readErr
		}

		var hostFilePath = path.Join(util.GetLocation().GetHostDirectory(), entry.Name())
		writeErr := os.WriteFile(hostFilePath, hostData, 0644)
		if writeErr != nil {
			return writeErr
		}
		util.GetLogger().Info(ctx, fmt.Sprintf("extracted host file: %s, cost: %dms", entry.Name(), util.GetSystemTimestamp()-start))
	}

	return nil
}

func extractUI(ctx context.Context) error {
	// only extract UI in prod mode
	if util.IsDev() {
		return nil
	}

	dir, err := UIFS.ReadDir("ui")
	if err != nil {
		return err
	}
	if len(dir) == 0 {
		return fmt.Errorf("no ui file found")
	}

	for _, entry := range dir {
		if entry.IsDir() {
			continue
		}

		start := util.GetSystemTimestamp()
		uiData, readErr := UIFS.ReadFile("ui/" + entry.Name())
		if readErr != nil {
			return readErr
		}

		var hostFilePath = path.Join(util.GetLocation().GetUIDirectory(), entry.Name())
		writeErr := os.WriteFile(hostFilePath, uiData, 0777)
		if writeErr != nil {
			return writeErr
		}

		util.GetLogger().Info(ctx, fmt.Sprintf("extracted ui file: %s, cost: %dms", entry.Name(), util.GetSystemTimestamp()-start))
	}

	if _, statErr := os.Stat(util.GetLocation().GetUIAppPath()); os.IsNotExist(statErr) {
		return fmt.Errorf("failed to extract ui: not found")
	}

	return nil
}

func extractThemes(ctx context.Context) error {
	dir, err := UIFS.ReadDir(path.Join("ui", "themes"))
	if err != nil {
		return err
	}
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
