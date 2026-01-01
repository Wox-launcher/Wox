package app

// PrefPaneInfo contains the display information for a System Preference Pane.
// This is used to provide correct icons and titles for preference panes on modern macOS,
// where the system does not expose these assets via standard APIs.
type PrefPaneInfo struct {
	DisplayName     string // i18n key or localized display name (e.g., "i18n:plugin_app_macos_prefpane_privacy_security")
	SFSymbol        string // SF Symbol name (e.g., "hand.raised.fill")
	BackgroundColor string // Color name: "blue", "red", "gray", "indigo", "pink", "purple", "cyan", "orange", "green", "teal"
	IconStyle       string // "filled" (colored bg + white symbol) or "outlined" (white bg + colored symbol), default is "filled"
}

// PrefPaneMappings maps preference pane filenames to their display information.
// These mappings are based on macOS Ventura/Sonoma System Settings appearance.
var PrefPaneMappings = map[string]PrefPaneInfo{
	// Privacy & Security
	"Security.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_privacy_security",
		SFSymbol:        "hand.raised.fill",
		BackgroundColor: "blue",
	},

	// Notifications
	"Notifications.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_notifications",
		SFSymbol:        "bell.badge.fill",
		BackgroundColor: "red",
	},

	// Wi-Fi (part of Network)
	"Network.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_network",
		SFSymbol:        "network",
		BackgroundColor: "blue",
	},

	// Bluetooth
	"Bluetooth.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_bluetooth",
		SFSymbol:        "bluetooth",
		BackgroundColor: "blue",
	},

	// Sound
	"Sound.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_sound",
		SFSymbol:        "speaker.wave.3.fill",
		BackgroundColor: "pink",
	},

	// Focus
	"Expose.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_focus",
		SFSymbol:        "moon.fill",
		BackgroundColor: "indigo",
	},

	// Screen Time
	"ScreenTime.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_screen_time",
		SFSymbol:        "hourglass",
		BackgroundColor: "indigo",
	},

	// General (Appearance in older)
	"Appearance.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_appearance",
		SFSymbol:        "paintbrush.fill",
		BackgroundColor: "gray",
	},

	// Accessibility
	"UniversalAccessPref.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_accessibility",
		SFSymbol:        "accessibility",
		BackgroundColor: "blue",
	},

	// Siri (Speech)
	"Speech.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_siri",
		SFSymbol:        "mic.fill",
		BackgroundColor: "purple",
	},

	// Spotlight
	"Spotlight.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_spotlight",
		SFSymbol:        "magnifyingglass",
		BackgroundColor: "gray",
	},

	// Wallpaper (Desktop & Screen Saver)
	"DesktopScreenEffectsPref.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_wallpaper",
		SFSymbol:        "photo",
		BackgroundColor: "cyan",
	},

	// Displays
	"Displays.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_displays",
		SFSymbol:        "display",
		BackgroundColor: "blue",
	},

	// Desktop & Dock
	"Dock.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_desktop_dock",
		SFSymbol:        "dock.rectangle",
		BackgroundColor: "gray",
	},

	// Battery / Energy
	"Battery.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_battery",
		SFSymbol:        "battery.100percent",
		BackgroundColor: "green",
	},
	"EnergySaver.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_energy",
		SFSymbol:        "bolt.fill",
		BackgroundColor: "green",
	},
	"EnergySaverPref.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_energy_saver",
		SFSymbol:        "bolt.fill",
		BackgroundColor: "green",
	},

	// Lock Screen (Touch ID)
	"TouchID.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_touch_id_password",
		SFSymbol:        "touchid",
		BackgroundColor: "red",
	},

	// Users & Groups
	"Accounts.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_users_groups",
		SFSymbol:        "person.2.fill",
		BackgroundColor: "blue",
	},

	// Passwords
	"Passwords.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_passwords",
		SFSymbol:        "key.fill",
		BackgroundColor: "gray",
	},

	// Internet Accounts
	"InternetAccounts.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_internet_accounts",
		SFSymbol:        "at",
		BackgroundColor: "blue",
	},

	// Game Center
	// (No direct prefPane, skip)

	// Wallet
	"Wallet.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_wallet_apple_pay",
		SFSymbol:        "wallet.pass.fill",
		BackgroundColor: "orange",
	},

	// Keyboard
	"Keyboard.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_keyboard",
		SFSymbol:        "keyboard",
		BackgroundColor: "gray",
	},

	// Trackpad
	"Trackpad.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_trackpad",
		SFSymbol:        "rectangle.and.hand.point.up.left.filled",
		BackgroundColor: "gray",
	},

	// Mouse
	"Mouse.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_mouse",
		SFSymbol:        "magicmouse.fill",
		BackgroundColor: "gray",
	},

	// Printers & Scanners
	"PrintAndScan.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_printers_scanners",
		SFSymbol:        "printer.fill",
		BackgroundColor: "gray",
	},
	"PrintAndFax.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_printers_scanners",
		SFSymbol:        "printer.fill",
		BackgroundColor: "gray",
	},

	// Date & Time
	"DateAndTime.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_date_time",
		SFSymbol:        "calendar",
		BackgroundColor: "red",
	},

	// Sharing
	"SharingPref.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_sharing",
		SFSymbol:        "shareplay",
		BackgroundColor: "blue",
	},

	// Time Machine
	"TimeMachine.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_time_machine",
		SFSymbol:        "clock.arrow.circlepath",
		BackgroundColor: "teal",
	},

	// Startup Disk
	"StartupDisk.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_startup_disk",
		SFSymbol:        "internaldrive.fill",
		BackgroundColor: "gray",
	},

	// Extensions
	"Extensions.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_extensions",
		SFSymbol:        "puzzlepiece.extension.fill",
		BackgroundColor: "gray",
	},

	// Profiles
	"Profiles.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_profiles",
		SFSymbol:        "person.text.rectangle.fill",
		BackgroundColor: "gray",
	},

	// Software Update
	"SoftwareUpdate.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_software_update",
		SFSymbol:        "gear.badge",
		BackgroundColor: "gray",
	},

	// Apple ID
	"AppleIDPrefPane.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_apple_id",
		SFSymbol:        "apple.logo",
		BackgroundColor: "gray",
	},

	// Family Sharing
	"FamilySharingPrefPane.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_family",
		SFSymbol:        "person.2.fill",
		BackgroundColor: "blue",
		IconStyle:       "outlined", // White background with colored symbol
	},

	// Language & Region
	"Localization.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_language_region",
		SFSymbol:        "globe",
		BackgroundColor: "blue",
	},

	// CDs & DVDs
	"DigiHubDiscs.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_cds_dvds",
		SFSymbol:        "opticaldisc.fill",
		BackgroundColor: "gray",
	},

	// Classroom
	"ClassroomSettings.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_classroom",
		SFSymbol:        "graduationcap.fill",
		BackgroundColor: "blue",
	},
	"ClassKitPreferencePane.prefPane": {
		DisplayName:     "i18n:plugin_app_macos_prefpane_classkit",
		SFSymbol:        "graduationcap.fill",
		BackgroundColor: "blue",
	},
}

// GetPrefPaneInfo returns the display info for a preference pane, or nil if not found.
func GetPrefPaneInfo(fileName string) *PrefPaneInfo {
	if info, ok := PrefPaneMappings[fileName]; ok {
		return &info
	}
	return nil
}
