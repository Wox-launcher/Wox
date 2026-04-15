//go:build !windows

package platform

func NewDefaultTextInputHost() TextInputHost {
	return &NoopTextInputHost{}
}
