package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"wox/setting/definition"
	"wox/util"
	"wox/util/shell"
)

const (
	webSearchDefaultBrowserSettingKey = "defaultBrowser"

	webSearchBrowserUseDefault = "default"
	webSearchBrowserSystem     = "system"
	webSearchBrowserChrome     = "chrome"
	webSearchBrowserEdge       = "edge"
	webSearchBrowserFirefox    = "firefox"
	webSearchBrowserBrave      = "brave"
	webSearchBrowserOpera      = "opera"
	webSearchBrowserVivaldi    = "vivaldi"
	webSearchBrowserChromium   = "chromium"
	webSearchBrowserSafari     = "safari"
)

type webSearchBrowserOption struct {
	Id    string
	Label string
}

var supportedWebSearchBrowsers = []webSearchBrowserOption{
	{Id: webSearchBrowserChrome, Label: "i18n:plugin_websearch_browser_google_chrome"},
	{Id: webSearchBrowserEdge, Label: "i18n:plugin_websearch_browser_microsoft_edge"},
	{Id: webSearchBrowserFirefox, Label: "i18n:plugin_websearch_browser_mozilla_firefox"},
	{Id: webSearchBrowserBrave, Label: "i18n:plugin_websearch_browser_brave"},
	{Id: webSearchBrowserOpera, Label: "i18n:plugin_websearch_browser_opera"},
	{Id: webSearchBrowserVivaldi, Label: "i18n:plugin_websearch_browser_vivaldi"},
	{Id: webSearchBrowserChromium, Label: "i18n:plugin_websearch_browser_chromium"},
	{Id: webSearchBrowserSafari, Label: "i18n:plugin_websearch_browser_safari"},
}

func normalizeWebSearchBrowser(browser string) string {
	return strings.ToLower(strings.TrimSpace(browser))
}

func resolveWebSearchBrowser(itemBrowser string, defaultBrowser string) string {
	normalizedItemBrowser := normalizeWebSearchBrowser(itemBrowser)
	normalizedDefaultBrowser := normalizeWebSearchBrowser(defaultBrowser)
	if normalizedDefaultBrowser == "" {
		normalizedDefaultBrowser = webSearchBrowserSystem
	}

	switch normalizedItemBrowser {
	case "", webSearchBrowserUseDefault:
		return normalizedDefaultBrowser
	default:
		return normalizedItemBrowser
	}
}

func getWebSearchDefaultBrowserOptions() []definition.PluginSettingValueSelectOption {
	options := []definition.PluginSettingValueSelectOption{
		{Label: "i18n:plugin_websearch_browser_system_default", Value: webSearchBrowserSystem},
	}

	for _, browser := range getInstalledWebSearchBrowsers() {
		options = append(options, definition.PluginSettingValueSelectOption{
			Label: browser.Label,
			Value: browser.Id,
		})
	}

	return options
}

func getWebSearchItemBrowserOptions() []definition.PluginSettingValueSelectOption {
	options := []definition.PluginSettingValueSelectOption{
		{Label: "i18n:plugin_websearch_browser_use_default", Value: webSearchBrowserUseDefault},
	}

	for _, browser := range getInstalledWebSearchBrowsers() {
		options = append(options, definition.PluginSettingValueSelectOption{
			Label: browser.Label,
			Value: browser.Id,
		})
	}

	return options
}

func getInstalledWebSearchBrowsers() []webSearchBrowserOption {
	var installed []webSearchBrowserOption
	for _, browser := range supportedWebSearchBrowsers {
		if isWebSearchBrowserInstalled(browser.Id) {
			installed = append(installed, browser)
		}
	}
	return installed
}

func isWebSearchBrowserInstalled(browser string) bool {
	switch {
	case util.IsWindows():
		_, ok := resolveWindowsBrowserExecutable(browser)
		return ok
	case util.IsMacOS():
		_, ok := resolveMacBrowserApp(browser)
		return ok
	case util.IsLinux():
		_, ok := resolveLinuxBrowserCommand(browser)
		return ok
	default:
		return false
	}
}

func openURLInWebSearchBrowser(url string, browser string) error {
	switch normalizeWebSearchBrowser(browser) {
	case "", webSearchBrowserSystem:
		return shell.Open(url)
	}

	switch {
	case util.IsWindows():
		executable, ok := resolveWindowsBrowserExecutable(browser)
		if !ok {
			return shell.Open(url)
		}
		_, err := shell.Run(executable, url)
		if err != nil {
			return openURLInSystemBrowserWithFallback(url, err)
		}
		return nil
	case util.IsMacOS():
		appPath, ok := resolveMacBrowserApp(browser)
		if !ok {
			return shell.Open(url)
		}
		_, err := shell.Run("open", "-a", appPath, url)
		if err != nil {
			return openURLInSystemBrowserWithFallback(url, err)
		}
		return nil
	case util.IsLinux():
		command, ok := resolveLinuxBrowserCommand(browser)
		if !ok {
			return shell.Open(url)
		}
		_, err := shell.Run(command, url)
		if err != nil {
			return openURLInSystemBrowserWithFallback(url, err)
		}
		return nil
	default:
		return shell.Open(url)
	}
}

func openURLInSystemBrowserWithFallback(url string, openErr error) error {
	fallbackErr := shell.Open(url)
	if fallbackErr != nil {
		return fmt.Errorf("failed to open url with configured browser: %w, fallback to system browser failed: %w", openErr, fallbackErr)
	}
	return nil
}

func resolveWindowsBrowserExecutable(browser string) (string, bool) {
	for _, candidate := range getWindowsBrowserCandidateExecutables(normalizeWebSearchBrowser(browser)) {
		if util.IsFileExists(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func getWindowsBrowserCandidateExecutables(browser string) []string {
	var candidates []string

	addProgramFilesCandidate := func(paths ...string) {
		for _, base := range getWindowsProgramFilesDirs() {
			candidates = append(candidates, filepath.Join(append([]string{base}, paths...)...))
		}
	}

	addLocalAppDataCandidate := func(paths ...string) {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			return
		}
		candidates = append(candidates, filepath.Join(append([]string{localAppData}, paths...)...))
	}

	switch browser {
	case webSearchBrowserChrome:
		addProgramFilesCandidate("Google", "Chrome", "Application", "chrome.exe")
		addLocalAppDataCandidate("Google", "Chrome", "Application", "chrome.exe")
	case webSearchBrowserEdge:
		addProgramFilesCandidate("Microsoft", "Edge", "Application", "msedge.exe")
		addLocalAppDataCandidate("Microsoft", "Edge", "Application", "msedge.exe")
	case webSearchBrowserFirefox:
		addProgramFilesCandidate("Mozilla Firefox", "firefox.exe")
		addLocalAppDataCandidate("Mozilla Firefox", "firefox.exe")
	case webSearchBrowserBrave:
		addProgramFilesCandidate("BraveSoftware", "Brave-Browser", "Application", "brave.exe")
		addLocalAppDataCandidate("BraveSoftware", "Brave-Browser", "Application", "brave.exe")
	case webSearchBrowserOpera:
		addProgramFilesCandidate("Opera", "launcher.exe")
		addLocalAppDataCandidate("Programs", "Opera", "opera.exe")
	case webSearchBrowserVivaldi:
		addProgramFilesCandidate("Vivaldi", "Application", "vivaldi.exe")
		addLocalAppDataCandidate("Vivaldi", "Application", "vivaldi.exe")
	case webSearchBrowserChromium:
		addProgramFilesCandidate("Chromium", "Application", "chrome.exe")
		addLocalAppDataCandidate("Chromium", "Application", "chrome.exe")
	}

	return uniqueNonEmptyPaths(candidates)
}

func getWindowsProgramFilesDirs() []string {
	return uniqueNonEmptyPaths([]string{
		os.Getenv("ProgramFiles"),
		os.Getenv("ProgramFiles(x86)"),
	})
}

func resolveMacBrowserApp(browser string) (string, bool) {
	for _, candidate := range getMacBrowserAppCandidates(normalizeWebSearchBrowser(browser)) {
		if util.IsDirExists(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func getMacBrowserAppCandidates(browser string) []string {
	homeDir, _ := os.UserHomeDir()

	addAppCandidates := func(appName string) []string {
		candidates := []string{
			filepath.Join("/Applications", appName),
		}
		if homeDir != "" {
			candidates = append(candidates, filepath.Join(homeDir, "Applications", appName))
		}
		return candidates
	}

	switch browser {
	case webSearchBrowserSafari:
		candidates := []string{"/System/Applications/Safari.app"}
		return append(candidates, addAppCandidates("Safari.app")...)
	case webSearchBrowserChrome:
		return addAppCandidates("Google Chrome.app")
	case webSearchBrowserEdge:
		return addAppCandidates("Microsoft Edge.app")
	case webSearchBrowserFirefox:
		return addAppCandidates("Firefox.app")
	case webSearchBrowserBrave:
		return addAppCandidates("Brave Browser.app")
	case webSearchBrowserOpera:
		return addAppCandidates("Opera.app")
	case webSearchBrowserVivaldi:
		return addAppCandidates("Vivaldi.app")
	case webSearchBrowserChromium:
		return addAppCandidates("Chromium.app")
	default:
		return nil
	}
}

func resolveLinuxBrowserCommand(browser string) (string, bool) {
	for _, command := range getLinuxBrowserCandidateCommands(normalizeWebSearchBrowser(browser)) {
		if executable, err := exec.LookPath(command); err == nil {
			return executable, true
		}
	}
	return "", false
}

func getLinuxBrowserCandidateCommands(browser string) []string {
	switch browser {
	case webSearchBrowserChrome:
		return []string{"google-chrome", "google-chrome-stable"}
	case webSearchBrowserEdge:
		return []string{"microsoft-edge", "microsoft-edge-stable"}
	case webSearchBrowserFirefox:
		return []string{"firefox"}
	case webSearchBrowserBrave:
		return []string{"brave-browser"}
	case webSearchBrowserOpera:
		return []string{"opera"}
	case webSearchBrowserVivaldi:
		return []string{"vivaldi-stable", "vivaldi"}
	case webSearchBrowserChromium:
		return []string{"chromium", "chromium-browser"}
	default:
		return nil
	}
}

func uniqueNonEmptyPaths(values []string) []string {
	unique := make(map[string]struct{})
	var result []string

	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, ok := unique[normalized]; ok {
			continue
		}
		unique[normalized] = struct{}{}
		result = append(result, normalized)
	}

	return result
}
