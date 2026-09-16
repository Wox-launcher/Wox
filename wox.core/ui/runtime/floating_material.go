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
//     while the frame is encoded, compresses backdrop contrast/chroma around the
//     authored tint without changing sampled alpha, then paints the tint and edge.
//     Everything drawn before the material this frame, including other floating
//     surfaces, is part of the sampled backdrop, so stacking needs no cover.
//     An overlay over a native WebView cannot sample that page, so the same tint
//     is painted opaque (see opaqueFloatingMaterialTint).
//   - Linux does the same with its OpenGL back buffer. Compositor blur
//     (ext-background-effect-v1) is a whole-window desktop backdrop and cannot
//     sample Go content underneath a panel. Overlay-over-WebView uses the same
//     opaque fallback as Windows.

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

// FloatingMaterialBlurMargin is how far outside a renderer-blurred surface the kernel
// samples, in logical pixels. Three standard deviations cover the kernel, so the surface
// edge blends with real neighbours instead of transparent padding. Damage covering uses
// this halo to resample a surface without joining adjacent cards into one rectangle.
const FloatingMaterialBlurMargin = 3 * floatingMaterialBlurSigma

// opaqueFloatingMaterialTint keeps the authored colour when a floating surface
// cannot sample the pixels underneath. Theme tints are written for a blurred
// backdrop; without one, only full opacity still reads as a card.
func opaqueFloatingMaterialTint(tint Color) Color {
	if tint.A != 0 {
		tint.A = 255
	}
	return tint
}

// floatingMaterial is one material declared by a frame, in logical client coordinates.
type floatingMaterial struct {
	bounds Rect
	radius float32
	tint   Color
	edge   Color
}
