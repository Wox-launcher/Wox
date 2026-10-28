package icons

import (
	"fmt"
	"wox/common"
)

// windowGlyph keeps window-management results visually consistent with other action icons.
func windowGlyph(paths string) common.WoxImage {
	return common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="var(--wox-theme-icon-color)" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">` + paths + `</svg>`)
}

// windowLayoutGlyph shows the target region without encoding a fixed theme color.
func windowLayoutGlyph(x, y, width, height float64) common.WoxImage {
	return windowGlyph(fmt.Sprintf(`<rect x="3" y="3" width="18" height="18" rx="2"/><rect x="%g" y="%g" width="%g" height="%g" rx="1" fill="var(--wox-theme-icon-color)" fill-opacity="0.3" stroke="none"/>`, x, y, width, height))
}

var defaultWindowIcons = map[string]common.WoxImage{
	LayoutLeftHalf:        windowLayoutGlyph(4, 4, 8, 16),
	LayoutRightHalf:       windowLayoutGlyph(12, 4, 8, 16),
	LayoutTopHalf:         windowLayoutGlyph(4, 4, 16, 8),
	LayoutBottomHalf:      windowLayoutGlyph(4, 12, 16, 8),
	LayoutTopLeft:         windowLayoutGlyph(4, 4, 8, 8),
	LayoutTopRight:        windowLayoutGlyph(12, 4, 8, 8),
	LayoutBottomLeft:      windowLayoutGlyph(4, 12, 8, 8),
	LayoutBottomRight:     windowLayoutGlyph(12, 12, 8, 8),
	LayoutFirstThird:      windowLayoutGlyph(4, 4, 5, 16),
	LayoutCenterThird:     windowLayoutGlyph(9.5, 4, 5, 16),
	LayoutLastThird:       windowLayoutGlyph(15, 4, 5, 16),
	LayoutFirstTwoThirds:  windowLayoutGlyph(4, 4, 10.5, 16),
	LayoutLastTwoThirds:   windowLayoutGlyph(9.5, 4, 10.5, 16),
	LayoutAlmostMaximize:  windowLayoutGlyph(5.5, 5.5, 13, 13),
	LayoutCenter:          windowLayoutGlyph(7, 6, 10, 12),
	LayoutReasonable:      windowLayoutGlyph(6, 6, 12, 12),
	LayoutMaximizeHeight:  windowLayoutGlyph(8, 4, 8, 16),
	LayoutMaximizeWidth:   windowLayoutGlyph(4, 8, 16, 8),
	WindowMaximize:        windowGlyph(`<rect x="3" y="3" width="18" height="18" rx="2"/><path d="M8 11V8h3M16 13v3h-3"/>`),
	WindowRestore:         windowGlyph(`<path d="M8 5V3h11a2 2 0 0 1 2 2v11h-2"/><rect x="3" y="8" width="13" height="13" rx="2"/>`),
	WindowMinimize:        windowGlyph(`<rect x="3" y="3" width="18" height="18" rx="2"/><path d="M8 16h8"/>`),
	WindowFullscreen:      windowGlyph(`<path d="M8 3H3v5M16 3h5v5M8 21H3v-5M16 21h5v-5"/>`),
	WindowFloating:        windowGlyph(`<path d="M4 4h13v3M4 4v13h3" stroke-dasharray="2 2"/><rect x="8" y="8" width="13" height="13" rx="2"/>`),
	WindowSwapLeft:        windowGlyph(`<rect x="3" y="9" width="8" height="10" rx="1"/><rect x="13" y="9" width="8" height="10" rx="1"/><path d="M18 5H6m3-3L6 5l3 3"/>`),
	WindowSwapRight:       windowGlyph(`<rect x="3" y="9" width="8" height="10" rx="1"/><rect x="13" y="9" width="8" height="10" rx="1"/><path d="M6 5h12m-3-3 3 3-3 3"/>`),
	WindowSwapUp:          windowGlyph(`<rect x="5" y="3" width="10" height="8" rx="1"/><rect x="5" y="13" width="10" height="8" rx="1"/><path d="M19 18V6m-3 3 3-3 3 3"/>`),
	WindowSwapDown:        windowGlyph(`<rect x="5" y="3" width="10" height="8" rx="1"/><rect x="5" y="13" width="10" height="8" rx="1"/><path d="M19 6v12m-3-3 3 3 3-3"/>`),
	WindowNextDisplay:     windowGlyph(`<rect x="2" y="5" width="8" height="11" rx="1"/><rect x="14" y="5" width="8" height="11" rx="1"/><path d="M6 19h12m-3-3 3 3-3 3"/>`),
	WindowPreviousDisplay: windowGlyph(`<rect x="2" y="5" width="8" height="11" rx="1"/><rect x="14" y="5" width="8" height="11" rx="1"/><path d="M18 19H6m3-3-3 3 3 3"/>`),
	WindowMoveWorkspace:   windowGlyph(`<rect x="2" y="4" width="9" height="11" rx="1"/><rect x="13" y="9" width="9" height="11" rx="1"/><path d="M5 19h6m-2-2 2 2-2 2"/>`),
	WindowFocus:           windowGlyph(`<rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="12" cy="12" r="3"/><path d="M12 6v2M12 16v2M6 12h2M16 12h2"/>`),
	WindowGroup:           windowGlyph(`<rect x="3" y="3" width="18" height="18" rx="2"/><path d="M12 3v18M3 12h9M15 9h3M15 15h3"/>`),
}
