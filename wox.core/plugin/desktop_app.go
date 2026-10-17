package plugin

// DesktopApp is one indexed desktop application a launcher import can bind to.
type DesktopApp struct {
	Name       string
	Path       string
	Executable string
	Arguments  string
}

// DesktopAppCatalog lists indexed desktop applications.
type DesktopAppCatalog interface {
	DesktopApps() []DesktopApp
}

// SettingsAppCatalog lists indexed Control Panel items a launcher import can bind to.
type SettingsAppCatalog interface {
	SettingsApps() []DesktopApp
}
