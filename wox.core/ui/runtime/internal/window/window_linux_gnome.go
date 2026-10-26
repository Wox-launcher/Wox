//go:build linux

package window

// linuxGnomeWindowPolicy preserves ordinary GTK windows on Mutter rather than
// assuming that another compositor's layer-shell policy applies to GNOME.
func linuxGnomeWindowPolicy() linuxWindowPolicy {
	return linuxWindowPolicy{name: "gnome"}
}
