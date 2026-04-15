//go:build !windows

package platform

func NewDefaultBundle() Bundle {
	return Bundle{
		Host:      NewPlaceholderHost(),
		TextInput: NewDefaultTextInputHost(),
	}
}

func NewDefaultHost() Host {
	return NewDefaultBundle().Host
}
