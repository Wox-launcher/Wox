//go:build windows

package platform

func NewDefaultBundle() Bundle {
	return NewWindowsNativeShellBundle()
}

func NewDefaultHost() Host {
	return NewDefaultBundle().Host
}
