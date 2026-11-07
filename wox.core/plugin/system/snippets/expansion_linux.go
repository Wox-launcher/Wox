//go:build linux && cgo

package snippets

import "wox/util"

// newExpansionBackend selects an input implementation without mixing desktop policies.
func newExpansionBackend() expansionBackend {
	if util.IsLinuxWaylandSession() {
		return newWaylandExpansionBackend()
	}
	return newX11ExpansionBackend()
}
