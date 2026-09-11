package woxui

// A floating material is one native translucent surface placed behind a panel
// that floats above other Go UI content in the same window: the launcher action
// panel, dialogs, dropdown menus, context menus and tooltips. It blurs the Go
// content underneath and carries the theme tint over that blur, so the panel
// reads as a separate card, which the process-wide window material (see
// window_material.go) cannot do because that material sits behind the whole
// render view. Its look must not depend on window focus: the material is pinned
// to its active state just like the window material.
//
// Surfaces do not manage materials themselves. They record
// DisplayList.FloatingMaterial while painting, and the platform window
// reconciles the materials declared by each presented frame with the native
// views it owns: a material that a frame no longer declares disappears with
// that frame, so there is no separate show/hide lifecycle to keep in sync.
//
// Only two composition surfaces exist (main and overlay), so every material
// samples the main surface and cannot blur another floating surface. A surface
// stacked over one therefore paints an opaque fill of its tint colour on the
// overlay (see DisplayList.FloatingMaterial); its edge and shadow still come
// from the material.

// floatingMaterial is one material declared by a frame, in logical client coordinates.
type floatingMaterial struct {
	bounds Rect
	radius float32
	tint   Color
	edge   Color
}
