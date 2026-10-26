//go:build linux

package window

// linuxCosmicWindowPolicy keeps query-driven sizing under client control.
// COSMIC retains a mapped xdg_toplevel's size; launcher result changes need a layer surface.
// Settings remain ordinary application windows and never enter the layer-shell path.
func linuxCosmicWindowPolicy() linuxWindowPolicy {
	return linuxWindowPolicy{name: "cosmic", layerShell: true}
}
