package websearch

import (
	"strings"
	"wox/setting/definition"
	"wox/util"
)

const (
	webviewUserAgentAuto          = ""
	webviewUserAgentDesktopEdge   = "desktop_edge"
	webviewUserAgentDesktopChrome = "desktop_chrome"
	webviewUserAgentDesktopSafari = "desktop_safari"
	webviewUserAgentMobileSafari  = "mobile_safari"
	webviewUserAgentMobileChrome  = "mobile_chrome"
	webviewDesktopChromeUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"
	webviewMobileSafariUserAgent  = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_7_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Mobile/15E148 Safari/604.1"
	webviewMobileChromeUserAgent  = "Mozilla/5.0 (Linux; Android 16; Pixel 9) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Mobile Safari/537.36"
)

// getWebviewUserAgentOptions keeps desktop presets consistent with the native engine available on this platform.
func getWebviewUserAgentOptions() []definition.PluginSettingValueSelectOption {
	options := []definition.PluginSettingValueSelectOption{
		{Label: "i18n:plugin_websearch_user_agent_auto", Value: webviewUserAgentAuto},
	}
	if util.IsMacOS() {
		options = append(options, definition.PluginSettingValueSelectOption{Label: "i18n:plugin_websearch_user_agent_desktop_safari", Value: webviewUserAgentDesktopSafari})
	} else {
		options = append(options,
			definition.PluginSettingValueSelectOption{Label: "i18n:plugin_websearch_user_agent_desktop_edge", Value: webviewUserAgentDesktopEdge},
			definition.PluginSettingValueSelectOption{Label: "i18n:plugin_websearch_user_agent_desktop_chrome", Value: webviewUserAgentDesktopChrome},
		)
	}
	return append(options,
		definition.PluginSettingValueSelectOption{Label: "i18n:plugin_websearch_user_agent_mobile_safari", Value: webviewUserAgentMobileSafari},
		definition.PluginSettingValueSelectOption{Label: "i18n:plugin_websearch_user_agent_mobile_chrome", Value: webviewUserAgentMobileChrome},
	)
}

// resolveWebviewUserAgent leaves desktop defaults to the installed engine so its version and capabilities stay consistent.
func resolveWebviewUserAgent(userAgent string) string {
	switch trimmed := strings.TrimSpace(userAgent); trimmed {
	case "", "auto", webviewUserAgentDesktopEdge, webviewUserAgentDesktopSafari:
		return ""
	case webviewUserAgentDesktopChrome:
		return webviewDesktopChromeUserAgent
	case webviewUserAgentMobileSafari:
		return webviewMobileSafariUserAgent
	case webviewUserAgentMobileChrome:
		return webviewMobileChromeUserAgent
	default:
		return trimmed
	}
}
