package app

import (
	"context"
	"wox/common"
)

type windowsSettingItem struct {
	NameKey string
	URI     string
	Icon    common.WoxImage
	// SearchableNames stores stable aliases and Settings page description
	// keywords. The names shown to users are still localized through NameKey,
	// but these aliases keep English Windows Settings terms searchable.
	SearchableNames []string
}

var (
	// Bug fix: Windows Settings categories used to extract icons from legacy
	// Control Panel binaries such as ncpa.cpl. Those files still expose old
	// artwork on Windows 11, so the built-in SVGs below keep Wox aligned with
	// the modern Settings sidebar without depending on private Settings resources.
	windowsSettingSystemIcon          = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48"><defs><linearGradient id="wox-settings-system-a" x1="8" x2="40" y1="10" y2="36" gradientUnits="userSpaceOnUse"><stop stop-color="#45F3FF"/><stop offset="1" stop-color="#0078D4"/></linearGradient><linearGradient id="wox-settings-system-b" x1="10" x2="38" y1="12" y2="32" gradientUnits="userSpaceOnUse"><stop stop-color="#64F6FF"/><stop offset="1" stop-color="#0099BC"/></linearGradient></defs><rect x="7" y="10" width="34" height="25" rx="3" fill="url(#wox-settings-system-a)"/><rect x="10" y="13" width="28" height="18" rx="1.8" fill="url(#wox-settings-system-b)"/><rect x="16" y="37" width="16" height="2" rx="1" fill="#6B7280"/><circle cx="34" cy="32" r="1.2" fill="#0F172A" opacity=".55"/><circle cx="38" cy="32" r="1.2" fill="#0F172A" opacity=".55"/></svg>`)
	windowsSettingBluetoothIcon       = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48"><defs><linearGradient id="wox-settings-bluetooth-a" x1="10" x2="38" y1="8" y2="40" gradientUnits="userSpaceOnUse"><stop stop-color="#2FA8FF"/><stop offset="1" stop-color="#0078D4"/></linearGradient></defs><circle cx="24" cy="24" r="19" fill="url(#wox-settings-bluetooth-a)"/><path d="M21 10v28l11-10-8-4 8-4L21 10Z" fill="none" stroke="#FFFFFF" stroke-width="3" stroke-linejoin="round"/><path d="m15 17 9 7-9 7" fill="none" stroke="#FFFFFF" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/></svg>`)
	windowsSettingNetworkIcon         = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48"><defs><linearGradient id="wox-settings-network-a" x1="8" x2="40" y1="12" y2="38" gradientUnits="userSpaceOnUse"><stop stop-color="#53D8FF"/><stop offset="1" stop-color="#0078D4"/></linearGradient></defs><path d="M5 18.5C15.6 9.8 32.4 9.8 43 18.5L36.5 25C29.7 19.8 18.3 19.8 11.5 25L5 18.5Z" fill="url(#wox-settings-network-a)"/><path d="M14.5 28C20 24.2 28 24.2 33.5 28L24 38 14.5 28Z" fill="#0067B8"/><path d="M9.3 22.8C18 15.9 30 15.9 38.7 22.8" fill="none" stroke="#7CE7FF" stroke-width="3" stroke-linecap="round" opacity=".7"/></svg>`)
	windowsSettingPersonalizationIcon = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48"><defs><linearGradient id="wox-settings-personalization-a" x1="12" x2="36" y1="38" y2="10" gradientUnits="userSpaceOnUse"><stop stop-color="#FF8C00"/><stop offset=".55" stop-color="#D7DEE8"/><stop offset="1" stop-color="#8A98A8"/></linearGradient></defs><path d="M28.5 9.5c2.2-2.2 5.6-2.3 7.6-.4 1.9 1.9 1.7 5.3-.4 7.5L22.8 29.5l-7.2 1.9 1.9-7.2 11-14.7Z" fill="url(#wox-settings-personalization-a)"/><path d="M12.5 29.5c-3 2.8-3.3 6.5-2.5 9.5 3 .8 6.7.5 9.5-2.5 2-2.2 1.3-5.2-.7-7-2.1-1.9-4.5-1.8-6.3 0Z" fill="#FF8C00"/><path d="M20.2 27.5 33.6 14" stroke="#FFFFFF" stroke-width="2.2" stroke-linecap="round" opacity=".65"/></svg>`)
	windowsSettingAppsIcon            = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48"><defs><linearGradient id="wox-settings-apps-a" x1="10" x2="30" y1="11" y2="38" gradientUnits="userSpaceOnUse"><stop stop-color="#BFC7D1"/><stop offset="1" stop-color="#4B5563"/></linearGradient><linearGradient id="wox-settings-apps-b" x1="25" x2="40" y1="11" y2="27" gradientUnits="userSpaceOnUse"><stop stop-color="#40D9FF"/><stop offset="1" stop-color="#0078D4"/></linearGradient></defs><rect x="9" y="10" width="15" height="15" rx="2" fill="url(#wox-settings-apps-a)"/><rect x="9" y="27" width="15" height="12" rx="2" fill="#6B7280"/><rect x="26" y="26" width="12" height="13" rx="2" fill="#94A3B8"/><path d="M31.5 8 42 18.5 31.5 29 21 18.5 31.5 8Z" fill="url(#wox-settings-apps-b)"/></svg>`)
	windowsSettingAccountsIcon        = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48"><defs><linearGradient id="wox-settings-accounts-a" x1="13" x2="35" y1="8" y2="40" gradientUnits="userSpaceOnUse"><stop stop-color="#28D7C7"/><stop offset="1" stop-color="#00A383"/></linearGradient></defs><circle cx="24" cy="16" r="9" fill="url(#wox-settings-accounts-a)"/><path d="M12 39c1.4-8 6.2-12 12-12s10.6 4 12 12c-3.2 3.2-20.8 3.2-24 0Z" fill="url(#wox-settings-accounts-a)"/></svg>`)
	windowsSettingTimeLanguageIcon    = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48"><defs><linearGradient id="wox-settings-time-a" x1="14" x2="42" y1="8" y2="36" gradientUnits="userSpaceOnUse"><stop stop-color="#26D8D1"/><stop offset=".55" stop-color="#1596E8"/><stop offset="1" stop-color="#0067B8"/></linearGradient></defs><circle cx="27" cy="22" r="17" fill="url(#wox-settings-time-a)"/><path d="M10 22h34M27 5c4.4 4.6 6.5 10.2 6.5 17S31.4 34.4 27 39M27 5c-4.4 4.6-6.5 10.2-6.5 17S22.6 34.4 27 39" fill="none" stroke="#77EAF1" stroke-width="1.6" opacity=".7"/><circle cx="16" cy="32" r="10" fill="#E5E7EB"/><circle cx="16" cy="32" r="8" fill="#C8D0DA"/><path d="M16 26v6h5" fill="none" stroke="#3B4450" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>`)
	windowsSettingGamingIcon          = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48"><defs><linearGradient id="wox-settings-gaming-a" x1="8" x2="40" y1="18" y2="36" gradientUnits="userSpaceOnUse"><stop stop-color="#E5E7EB"/><stop offset="1" stop-color="#9CA3AF"/></linearGradient></defs><path d="M14 18h20c5 0 8 4.2 8 9.8 0 5.8-3.2 9.2-7 9.2-2.7 0-4.4-1.9-5.8-4H18.8c-1.4 2.1-3.1 4-5.8 4-3.8 0-7-3.4-7-9.2C6 22.2 9 18 14 18Z" fill="url(#wox-settings-gaming-a)"/><path d="M15 24v8M11 28h8" stroke="#4B5563" stroke-width="2.4" stroke-linecap="round"/><circle cx="31" cy="27" r="2.5" fill="#0078D4"/><circle cx="36" cy="31" r="2.5" fill="#0078D4"/><path d="M20 18c.5-3 2-5 4-5s3.5 2 4 5" fill="none" stroke="#AAB3C0" stroke-width="2" stroke-linecap="round"/></svg>`)
	windowsSettingAccessibilityIcon   = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48"><defs><linearGradient id="wox-settings-accessibility-a" x1="14" x2="34" y1="5" y2="43" gradientUnits="userSpaceOnUse"><stop stop-color="#24C6FF"/><stop offset="1" stop-color="#0078D4"/></linearGradient></defs><circle cx="24" cy="8" r="4" fill="url(#wox-settings-accessibility-a)"/><path d="M9 18c8.8 3.2 21.2 3.2 30 0" fill="none" stroke="url(#wox-settings-accessibility-a)" stroke-width="5" stroke-linecap="round"/><path d="M24 19v8M24 27l-8 15M24 27l8 15" fill="none" stroke="url(#wox-settings-accessibility-a)" stroke-width="5" stroke-linecap="round" stroke-linejoin="round"/></svg>`)
	windowsSettingPrivacySecurityIcon = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48"><defs><linearGradient id="wox-settings-privacy-a" x1="12" x2="36" y1="8" y2="41" gradientUnits="userSpaceOnUse"><stop stop-color="#F0F4F8"/><stop offset="1" stop-color="#8D99A8"/></linearGradient><linearGradient id="wox-settings-privacy-b" x1="20" x2="36" y1="13" y2="37" gradientUnits="userSpaceOnUse"><stop stop-color="#D7DEE8"/><stop offset="1" stop-color="#768292"/></linearGradient></defs><path d="M24 6 40 12v10c0 10.5-6.3 17.2-16 21-9.7-3.8-16-10.5-16-21V12l16-6Z" fill="url(#wox-settings-privacy-a)"/><path d="M24 11 35 15v7.2c0 7.4-4.1 12.3-11 15.5V11Z" fill="url(#wox-settings-privacy-b)" opacity=".95"/><path d="M24 6v37" stroke="#FFFFFF" stroke-width="1.5" opacity=".35"/></svg>`)
	windowsSettingWindowsUpdateIcon   = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48"><defs><linearGradient id="wox-settings-update-a" x1="8" x2="40" y1="8" y2="40" gradientUnits="userSpaceOnUse"><stop stop-color="#34B7FF"/><stop offset="1" stop-color="#0078D4"/></linearGradient></defs><circle cx="24" cy="24" r="19" fill="url(#wox-settings-update-a)"/><path d="M15 24a9 9 0 0 1 15.2-6.5L34 21M33 24a9 9 0 0 1-15.2 6.5L14 27" fill="none" stroke="#FFFFFF" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/><path d="M34 14v7h-7M14 34v-7h7" fill="none" stroke="#FFFFFF" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/></svg>`)
	windowsSettingDisplayIcon         = newWindowsSettingSystemPageIcon(`<rect x="10" y="13" width="28" height="18" rx="2"/><path d="M18 36h12M21 31v5M27 31v5"/>`)
	windowsSettingSoundIcon           = newWindowsSettingSystemPageIcon(`<path d="M10 20h7l8-7v22l-8-7h-7z"/><path d="M30 18c2.4 3.2 2.4 8.8 0 12M35 14c4.8 5.6 4.8 14.4 0 20"/>`)
	windowsSettingNotificationsIcon   = newWindowsSettingSystemPageIcon(`<path d="M16 30h16l-2-3v-6a6 6 0 0 0-12 0v6z"/><path d="M21 34a3 3 0 0 0 6 0"/>`)
	windowsSettingFocusIcon           = newWindowsSettingSystemPageIcon(`<circle cx="24" cy="24" r="10"/><path d="M24 8v4M24 36v4M8 24h4M36 24h4M12.7 12.7l2.8 2.8M32.5 32.5l2.8 2.8M35.3 12.7l-2.8 2.8M15.5 32.5l-2.8 2.8"/>`)
	windowsSettingPowerIcon           = newWindowsSettingSystemPageIcon(`<path d="M24 10v13"/><path d="M16 16.5a12 12 0 1 0 16 0"/>`)
	windowsSettingStorageIcon         = newWindowsSettingSystemPageIcon(`<rect x="10" y="17" width="28" height="14" rx="3"/><path d="M15 24h16M34 24h.1"/>`)
	windowsSettingNearbySharingIcon   = newWindowsSettingSystemPageIcon(`<path d="M15 30h-3a3 3 0 0 1-3-3V13a3 3 0 0 1 3-3h16a3 3 0 0 1 3 3v3"/><path d="M24 22h11v11"/><path d="M35 22 22 35"/><path d="M18 35h4"/>`)
	windowsSettingMultitaskingIcon    = newWindowsSettingSystemPageIcon(`<rect x="10" y="14" width="16" height="16" rx="2"/><rect x="22" y="20" width="16" height="16" rx="2"/>`)
	windowsSettingAdvancedIcon        = newWindowsSettingSystemPageIcon(`<path d="M15 10v12M11 10v8a4 4 0 0 0 8 0v-8M15 22v16"/><path d="M30 10v10M26 10v10M34 10v10M30 20v18"/>`)
	windowsSettingActivationIcon      = newWindowsSettingSystemPageIcon(`<circle cx="24" cy="24" r="14"/><path d="m17 24 5 5 10-11"/>`)
	windowsSettingTroubleshootIcon    = newWindowsSettingSystemPageIcon(`<path d="M32 10a8 8 0 0 0-9.5 10L11 31.5a3.5 3.5 0 0 0 5 5L27.5 25A8 8 0 0 0 38 15l-5 5-5-5z"/>`)
	windowsSettingRecoveryIcon        = newWindowsSettingSystemPageIcon(`<path d="M18 16h12a8 8 0 0 1 0 16H16"/><path d="m18 10-6 6 6 6"/><path d="M12 16h18M12 36h24"/>`)
	windowsSettingProjectingIcon      = newWindowsSettingSystemPageIcon(`<rect x="10" y="15" width="21" height="14" rx="2"/><rect x="18" y="23" width="20" height="14" rx="2"/><path d="M18 34h-6M24 29v8"/>`)
	windowsSettingRemoteDesktopIcon   = newWindowsSettingSystemPageIcon(`<rect x="10" y="13" width="28" height="18" rx="2"/><path d="M19 36h10M22 31v5M26 21h8M30 17l4 4-4 4"/>`)
	windowsSettingClipboardIcon       = newWindowsSettingSystemPageIcon(`<rect x="14" y="11" width="20" height="28" rx="3"/><path d="M20 11a4 4 0 0 1 8 0M19 21h10M19 28h10"/>`)
	windowsSettingAboutIcon           = newWindowsSettingSystemPageIcon(`<circle cx="24" cy="24" r="14"/><path d="M24 22v10M24 16h.1"/>`)
)

func newWindowsSettingSystemPageIcon(paths string) common.WoxImage {
	// Feature change: System subpages use simple built-in line SVGs instead of
	// extracting icons from private Settings search resources. The duplicated
	// stroke gives the white Windows-style glyph enough contrast in both Wox
	// dark and light themes without adding a separate theme-dependent image path.
	return common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48"><g fill="none" stroke="#334155" stroke-width="4.8" stroke-linecap="round" stroke-linejoin="round" opacity=".9">` + paths + `</g><g fill="none" stroke="#F8FAFC" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">` + paths + `</g></svg>`)
}

func (a *WindowsRetriever) getWindowsSettingsApps(ctx context.Context) []appInfo {
	items := []windowsSettingItem{
		{
			NameKey: "i18n:plugin_app_windows_settings_system",
			URI:     "ms-settings:system",
			Icon:    windowsSettingSystemIcon,
		},
		// Feature change: expose first-level System pages as searchable Wox
		// results. Windows has private XML search indexes for these rows, but the
		// supported launch surface is the ms-settings URI scheme, so Wox keeps a
		// small explicit table that is predictable across machines.
		{
			NameKey:         "i18n:plugin_app_windows_settings_system_display",
			URI:             "ms-settings:display",
			Icon:            windowsSettingDisplayIcon,
			SearchableNames: []string{"system display", "monitor brightness night light display profile"},
		},
		{
			NameKey:         "i18n:plugin_app_windows_settings_system_sound",
			URI:             "ms-settings:sound",
			Icon:            windowsSettingSoundIcon,
			SearchableNames: []string{"system sound", "volume output input sound devices"},
		},
		{
			NameKey:         "i18n:plugin_app_windows_settings_system_notifications",
			URI:             "ms-settings:notifications",
			Icon:            windowsSettingNotificationsIcon,
			SearchableNames: []string{"system notifications", "alerts apps system do not disturb"},
		},
		{
			NameKey:         "i18n:plugin_app_windows_settings_system_focus",
			URI:             "ms-settings:quiethours",
			Icon:            windowsSettingFocusIcon,
			SearchableNames: []string{"system focus", "focus assist reduce distractions quiet hours"},
		},
		{
			NameKey:         "i18n:plugin_app_windows_settings_system_power",
			URI:             "ms-settings:powersleep",
			Icon:            windowsSettingPowerIcon,
			SearchableNames: []string{"system power", "power sleep screen power mode energy saver battery"},
		},
		{
			NameKey:         "i18n:plugin_app_windows_settings_system_storage",
			URI:             "ms-settings:storagesense",
			Icon:            windowsSettingStorageIcon,
			SearchableNames: []string{"system storage", "storage space drives configuration rules storage sense"},
		},
		{
			NameKey:         "i18n:plugin_app_windows_settings_system_nearby_sharing",
			URI:             "ms-settings:crossdevice",
			Icon:            windowsSettingNearbySharingIcon,
			SearchableNames: []string{"system nearby sharing", "shared experiences discoverability received files location"},
		},
		{
			NameKey:         "i18n:plugin_app_windows_settings_system_multitasking",
			URI:             "ms-settings:multitasking",
			Icon:            windowsSettingMultitaskingIcon,
			SearchableNames: []string{"system multitasking", "snap windows desktops task switching"},
		},
		{
			NameKey:         "i18n:plugin_app_windows_settings_system_advanced",
			URI:             "ms-settings:developers",
			Icon:            windowsSettingAdvancedIcon,
			SearchableNames: []string{"system advanced", "for developers performance optimization developer features"},
		},
		{
			NameKey:         "i18n:plugin_app_windows_settings_system_activation",
			URI:             "ms-settings:activation",
			Icon:            windowsSettingActivationIcon,
			SearchableNames: []string{"system activation", "activation state subscriptions product key"},
		},
		{
			NameKey:         "i18n:plugin_app_windows_settings_system_troubleshoot",
			URI:             "ms-settings:troubleshoot",
			Icon:            windowsSettingTroubleshootIcon,
			SearchableNames: []string{"system troubleshoot", "recommended troubleshooters preferences history"},
		},
		{
			NameKey:         "i18n:plugin_app_windows_settings_system_recovery",
			URI:             "ms-settings:recovery",
			Icon:            windowsSettingRecoveryIcon,
			SearchableNames: []string{"system recovery", "reset advanced startup go back"},
		},
		{
			NameKey:         "i18n:plugin_app_windows_settings_system_projecting",
			URI:             "ms-settings:project",
			Icon:            windowsSettingProjectingIcon,
			SearchableNames: []string{"system projecting to this pc", "permissions pairing pin discoverability"},
		},
		{
			NameKey:         "i18n:plugin_app_windows_settings_system_remote_desktop",
			URI:             "ms-settings:remotedesktop",
			Icon:            windowsSettingRemoteDesktopIcon,
			SearchableNames: []string{"system remote desktop", "remote desktop virtual workspace"},
		},
		{
			NameKey:         "i18n:plugin_app_windows_settings_system_clipboard",
			URI:             "ms-settings:clipboard",
			Icon:            windowsSettingClipboardIcon,
			SearchableNames: []string{"system clipboard", "clipboard history sync"},
		},
		{
			NameKey:         "i18n:plugin_app_windows_settings_system_about",
			URI:             "ms-settings:about",
			Icon:            windowsSettingAboutIcon,
			SearchableNames: []string{"system about", "device specifications windows specifications pc name"},
		},
		{
			NameKey: "i18n:plugin_app_windows_settings_bluetooth",
			URI:     "ms-settings:bluetooth",
			Icon:    windowsSettingBluetoothIcon,
		},
		{
			NameKey: "i18n:plugin_app_windows_settings_network",
			URI:     "ms-settings:network",
			Icon:    windowsSettingNetworkIcon,
		},
		{
			NameKey: "i18n:plugin_app_windows_settings_personalization",
			URI:     "ms-settings:personalization",
			Icon:    windowsSettingPersonalizationIcon,
		},
		{
			NameKey: "i18n:plugin_app_windows_settings_apps",
			URI:     "ms-settings:appsfeatures",
			Icon:    windowsSettingAppsIcon,
		},
		{
			NameKey: "i18n:plugin_app_windows_settings_accounts",
			URI:     "ms-settings:yourinfo",
			Icon:    windowsSettingAccountsIcon,
		},
		{
			NameKey: "i18n:plugin_app_windows_settings_time_language",
			URI:     "ms-settings:time-language",
			Icon:    windowsSettingTimeLanguageIcon,
		},
		{
			NameKey: "i18n:plugin_app_windows_settings_gaming",
			URI:     "ms-settings:gaming-gamebar",
			Icon:    windowsSettingGamingIcon,
		},
		{
			NameKey: "i18n:plugin_app_windows_settings_accessibility",
			URI:     "ms-settings:easeofaccess",
			Icon:    windowsSettingAccessibilityIcon,
		},
		{
			NameKey: "i18n:plugin_app_windows_settings_privacy_security",
			URI:     "ms-settings:privacy",
			Icon:    windowsSettingPrivacySecurityIcon,
		},
		{
			NameKey: "i18n:plugin_app_windows_settings_windows_update",
			URI:     "ms-settings:windowsupdate",
			Icon:    windowsSettingWindowsUpdateIcon,
		},
	}

	apps := make([]appInfo, 0, len(items))
	for _, item := range items {
		icon := item.Icon
		if icon.IsEmpty() {
			icon = appIcon
		}

		apps = append(apps, appInfo{
			Name:            item.NameKey,
			Path:            item.URI,
			Icon:            icon,
			Type:            AppTypeWindowsSetting,
			SearchableNames: item.SearchableNames,
		})
	}

	return apps
}
