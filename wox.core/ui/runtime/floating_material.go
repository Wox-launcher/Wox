package woxui

// A floating material is one translucent backdrop placed behind a panel that
// floats above other Go UI content in the same window: the launcher action
// panel, dialogs, dropdown menus, context menus and tooltips. It blurs the Go
// content underneath and carries the theme tint over that blur, so the panel
// reads as a separate card, which the process-wide window material (see
// window_material.go) cannot do because that material sits behind the whole
// render view. Its look must not depend on window focus: the material is pinned
// to its active state just like the window material.
//
// Surfaces do not manage materials themselves. They record
// DisplayList.FloatingMaterial while painting, and each platform realises the
// recorded materials in the way its compositor allows (see floatingMaterialMode):
//
//   - macOS hosts one native visual-effect view per material between the main and
//     overlay composition surfaces. The platform window reconciles the materials
//     declared by each presented frame with the views it owns: a material that a
//     frame no longer declares disappears with that frame, so there is no separate
//     show/hide lifecycle to keep in sync. Only two composition surfaces exist, so
//     every material samples the main surface and cannot blur another floating
//     surface; a surface stacked over one paints an opaque cover of its tint
//     instead (see DisplayList.FloatingMaterial).
//   - Windows blurs the Direct2D back buffer under the surface rectangle in place
//     while the frame is encoded, then paints the tint and edge over the blur.
//     Everything drawn before the material this frame, including other floating
//     surfaces, is part of the sampled backdrop, so stacking needs no cover.
//   - Linux paints the tint and edge as ordinary commands; there is no blur.

// floatingMaterialMode is how the current platform realises a floating material.
type floatingMaterialMode uint8

const (
	// floatingMaterialPainted paints the tint and hairline edge as ordinary commands on
	// the main surface; themes author an opaque tint there.
	floatingMaterialPainted floatingMaterialMode = iota
	// floatingMaterialOverlay backs the surface with a native view between the main and
	// overlay composition surfaces, so the surface content moves onto the overlay.
	floatingMaterialOverlay
	// floatingMaterialRendered blurs the main surface under the rectangle inside the
	// renderer while the frame is encoded; the surface keeps painting on the main surface.
	floatingMaterialRendered
)

// floatingMaterialBlurSigma is the Gaussian standard deviation, in logical pixels, of the
// renderer blur. It only has to smear text underneath into unreadable shapes; a larger
// value costs more and makes the backdrop drift towards a flat colour.
const floatingMaterialBlurSigma float32 = 12

// floatingMaterialBlurMargin is how far outside a surface the renderer blur samples, in
// logical pixels. Three standard deviations cover the kernel, so the surface edge blends
// with real neighbours instead of transparent padding.
const floatingMaterialBlurMargin = 3 * floatingMaterialBlurSigma

// floatingMaterial is one material declared by a frame, in logical client coordinates.
type floatingMaterial struct {
	bounds Rect
	radius float32
	tint   Color
	edge   Color
}
