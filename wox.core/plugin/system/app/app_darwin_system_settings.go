package app

// SystemSettingInfo contains the display information for a macOS System Setting.
type SystemSettingInfo struct {
	DisplayNames    []string // i18n keys for display name and aliases
	SFSymbol        string   // SF Symbol name
	BackgroundColor string   // Color name
	IconStyle       string   // "filled" or "outlined"
	URI             string   // x-apple.systempreferences URL
}

// systemSettings contains all macOS system settings that should appear as searchable apps.
var systemSettings = map[string]SystemSettingInfo{
	"wifi":             {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_wifi"}, SFSymbol: "wifi", BackgroundColor: "blue", URI: "com.apple.Wi-Fi-Settings.extension"},
	"bluetooth":        {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_bluetooth"}, SFSymbol: "bluetooth", BackgroundColor: "blue", URI: "com.apple.BluetoothSettings"},
	"network":          {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_network"}, SFSymbol: "network", BackgroundColor: "blue", URI: "com.apple.Network-Settings.extension"},
	"vpn":              {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_vpn"}, SFSymbol: "network.badge.shield.half.filled", BackgroundColor: "blue", URI: "com.apple.NetworkExtensionSettingsUI.NESettingsUIExtension"},
	"notifications":    {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_notifications"}, SFSymbol: "bell.badge.fill", BackgroundColor: "red", URI: "com.apple.Notifications-Settings.extension"},
	"sound":            {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_sound"}, SFSymbol: "speaker.wave.3.fill", BackgroundColor: "pink", URI: "com.apple.Sound-Settings.extension"},
	"focus":            {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_focus"}, SFSymbol: "moon.fill", BackgroundColor: "indigo", URI: "com.apple.Focus-Settings.extension"},
	"screentime":       {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_screen_time"}, SFSymbol: "hourglass", BackgroundColor: "indigo", URI: "com.apple.Screen-Time-Settings.extension"},
	"general":          {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_general"}, SFSymbol: "gear", BackgroundColor: "gray", URI: "com.apple.systempreferences.GeneralSettings"},
	"appearance":       {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_appearance"}, SFSymbol: "paintbrush.fill", BackgroundColor: "gray", URI: "com.apple.Appearance-Settings.extension"},
	"accessibility":    {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_accessibility"}, SFSymbol: "accessibility", BackgroundColor: "blue", URI: "com.apple.Accessibility-Settings.extension"},
	"controlcenter":    {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_control_center"}, SFSymbol: "switch.2", BackgroundColor: "gray", URI: "com.apple.ControlCenter-Settings.extension"},
	"siri":             {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_siri"}, SFSymbol: "mic.fill", BackgroundColor: "purple", URI: "com.apple.Siri-Settings.extension"},
	"spotlight":        {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_spotlight"}, SFSymbol: "magnifyingglass", BackgroundColor: "gray", URI: "com.apple.Spotlight-Settings.extension"},
	"privacy":          {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_privacy_security"}, SFSymbol: "hand.raised.fill", BackgroundColor: "blue", URI: "com.apple.settings.PrivacySecurity.extension"},
	"desktop_dock":     {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_desktop_dock"}, SFSymbol: "dock.rectangle", BackgroundColor: "gray", URI: "com.apple.Desktop-Settings.extension"},
	"displays":         {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_displays"}, SFSymbol: "display", BackgroundColor: "blue", URI: "com.apple.Displays-Settings.extension"},
	"wallpaper":        {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_wallpaper"}, SFSymbol: "photo", BackgroundColor: "cyan", URI: "com.apple.Wallpaper-Settings.extension"},
	"screensaver":      {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_screen_saver"}, SFSymbol: "tv", BackgroundColor: "gray", URI: "com.apple.ScreenSaver-Settings.extension"},
	"energy":           {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_energy", "i18n:plugin_app_macos_prefpane_battery"}, SFSymbol: "bolt.fill", BackgroundColor: "yellow", URI: "com.apple.Battery-Settings.extension?energy"},
	"lockscreen":       {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_lock_screen"}, SFSymbol: "lock.fill", BackgroundColor: "gray", URI: "com.apple.Lock-Screen-Settings.extension"},
	"touchid":          {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_touch_id_password"}, SFSymbol: "touchid", BackgroundColor: "red", URI: "com.apple.Touch-ID-Settings.extension"},
	"users":            {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_users_groups"}, SFSymbol: "person.2.fill", BackgroundColor: "blue", URI: "com.apple.Users-Groups-Settings.extension"},
	"passwords":        {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_passwords"}, SFSymbol: "key.fill", BackgroundColor: "gray", URI: "com.apple.Passwords-Settings.extension"},
	"internetaccounts": {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_internet_accounts"}, SFSymbol: "at", BackgroundColor: "blue", URI: "com.apple.Internet-Accounts-Settings.extension"},
	"gamecenter":       {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_game_center"}, SFSymbol: "gamecontroller.fill", BackgroundColor: "gray", URI: "com.apple.Game-Center-Settings.extension"},
	"wallet":           {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_wallet_apple_pay"}, SFSymbol: "wallet.pass.fill", BackgroundColor: "orange", URI: "com.apple.WalletSettingsExtension"},
	"keyboard":         {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_keyboard"}, SFSymbol: "keyboard", BackgroundColor: "gray", URI: "com.apple.Keyboard-Settings.extension"},
	"trackpad":         {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_trackpad"}, SFSymbol: "rectangle.and.hand.point.up.left.filled", BackgroundColor: "gray", URI: "com.apple.Trackpad-Settings.extension"},
	"mouse":            {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_mouse"}, SFSymbol: "magicmouse.fill", BackgroundColor: "gray", URI: "com.apple.Mouse-Settings.extension"},
	"printers":         {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_printers_scanners"}, SFSymbol: "printer.fill", BackgroundColor: "gray", URI: "com.apple.Print-Scan-Settings.extension"},
	"family":           {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_family"}, SFSymbol: "person.2.fill", BackgroundColor: "blue", IconStyle: "outlined"},

	// General Sub-Settings
	"general.about":          {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_about"}, SFSymbol: "info.circle", BackgroundColor: "gray", URI: "com.apple.SystemProfiler.AboutExtension"},
	"general.softwareupdate": {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_software_update"}, SFSymbol: "gear.badge", BackgroundColor: "gray", URI: "com.apple.Software-Update-Settings.extension"},
	"general.storage":        {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_storage"}, SFSymbol: "internaldrive", BackgroundColor: "gray", URI: "com.apple.settings.Storage"},
	"general.airdrop":        {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_airdrop_handoff"}, SFSymbol: "airplayaudio", BackgroundColor: "teal", URI: "com.apple.AirDrop-Handoff-Settings.extension"},
	"general.loginitems":     {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_login_items"}, SFSymbol: "person.badge.key", BackgroundColor: "gray", URI: "com.apple.LoginItems-Settings.extension"},
	"general.datetime":       {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_date_time"}, SFSymbol: "calendar", BackgroundColor: "red", URI: "com.apple.Date-Time-Settings.extension"},
	"general.language":       {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_language_region"}, SFSymbol: "globe", BackgroundColor: "blue", URI: "com.apple.Localization-Settings.extension"},
	"general.sharing":        {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_sharing"}, SFSymbol: "shareplay", BackgroundColor: "blue", URI: "com.apple.Sharing-Settings.extension"},
	"general.startupdisk":    {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_startup_disk"}, SFSymbol: "internaldrive.fill", BackgroundColor: "gray", URI: "com.apple.Startup-Disk-Settings.extension"},
	"general.timemachine":    {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_time_machine"}, SFSymbol: "clock.arrow.circlepath", BackgroundColor: "teal", URI: "com.apple.Time-Machine-Settings.extension"},
	"general.transfer":       {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_transfer_reset"}, SFSymbol: "arrow.right.arrow.left.square", BackgroundColor: "gray", URI: "com.apple.Transfer-Reset-Settings.extension"},
	"general.profiles":       {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_profiles"}, SFSymbol: "person.text.rectangle.fill", BackgroundColor: "gray"},

	// Accessibility Sub-Settings
	"accessibility.voiceover":     {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_voiceover"}, SFSymbol: "speaker.wave.2.fill", BackgroundColor: "blue", URI: "com.apple.Accessibility-Settings.extension?VoiceOver"},
	"accessibility.zoom":          {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_zoom"}, SFSymbol: "plus.magnifyingglass", BackgroundColor: "blue", URI: "com.apple.Accessibility-Settings.extension?Zoom"},
	"accessibility.display":       {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_accessibility_display"}, SFSymbol: "display", BackgroundColor: "blue", URI: "com.apple.Accessibility-Settings.extension?Display"},
	"accessibility.spokencontent": {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_spoken_content"}, SFSymbol: "text.bubble.fill", BackgroundColor: "blue", URI: "com.apple.Accessibility-Settings.extension?SpokenContent"},
	"accessibility.voicecontrol":  {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_voice_control"}, SFSymbol: "mic.fill", BackgroundColor: "blue", URI: "com.apple.Accessibility-Settings.extension?VoiceControl"},
	"accessibility.keyboard":      {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_accessibility_keyboard"}, SFSymbol: "keyboard", BackgroundColor: "blue", URI: "com.apple.Accessibility-Settings.extension?Keyboard"},
	"accessibility.pointer":       {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_pointer_control"}, SFSymbol: "cursorarrow.rays", BackgroundColor: "blue", URI: "com.apple.Accessibility-Settings.extension?PointerControl"},
	"accessibility.switchcontrol": {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_switch_control"}, SFSymbol: "switch.2", BackgroundColor: "blue", URI: "com.apple.Accessibility-Settings.extension?SwitchControl"},

	// Misc
	"extensions": {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_extensions"}, SFSymbol: "puzzlepiece.extension.fill", BackgroundColor: "gray"},
	"appleid":    {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_apple_id"}, SFSymbol: "apple.logo", BackgroundColor: "gray"},
	"cd_dvd":     {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_cds_dvds"}, SFSymbol: "opticaldisc.fill", BackgroundColor: "gray"},
	"classroom":  {DisplayNames: []string{"i18n:plugin_app_macos_prefpane_classroom"}, SFSymbol: "graduationcap.fill", BackgroundColor: "blue"},
}
