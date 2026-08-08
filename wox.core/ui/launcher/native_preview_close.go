package launcher

import (
	"os"
	"runtime"
	"strings"

	"wox/common"
	"wox/util/overlay"
	"wox/util/overlay/iconoverlay"
)

const (
	nativePreviewCloseOverlayIDPrefix = "wox.launcher.native-preview-close."
	nativePreviewCloseRightOffset     = -36
	nativePreviewCloseTopOffset       = 20
	nativePreviewCloseSize            = 28
)

// shouldUseNativePreviewClose enables the independent close affordance only for an active
// Windows file preview whose child window can cover the normal in-process control.
func (a *App) shouldUseNativePreviewClose(preview queryPreview) bool {
	return runtime.GOOS == "windows" &&
		preview.PreviewType == "file" &&
		strings.TrimSpace(a.nativeFilePreviewPath) == strings.TrimSpace(preview.PreviewData) &&
		a.nativeFilePreviewError == "" &&
		launcherChromeHidden(a.show, a.chatFullscreen)
}

// nativePreviewCloseOverlayID keeps close overlays isolated when several launcher instances coexist.
func (a *App) nativePreviewCloseOverlayID() string {
	return nativePreviewCloseOverlayIDPrefix + a.sessionID
}

// reconcileNativePreviewCloseOverlay shows or removes the optional close button for the selected preview.
func (a *App) reconcileNativePreviewCloseOverlay(preview queryPreview) {
	id := a.nativePreviewCloseOverlayID()
	if !a.shouldUseNativePreviewClose(preview) {
		overlay.Close(id)
		return
	}

	closeIconSource := common.UIIcon("control.close")
	closeIcon, err := closeIconSource.ToImage()
	if err != nil {
		overlay.Close(id)
		return
	}

	iconoverlay.Show(iconoverlay.Options{
		Window: overlay.WindowOptions{
			ID:              id,
			Transparent:     true,
			HitTestIconOnly: true,
			Topmost:         true,
			StickyWindowPid: os.Getpid(),
			Anchor:          overlay.AnchorTopRight,
			OffsetX:         nativePreviewCloseRightOffset,
			OffsetY:         nativePreviewCloseTopOffset,
			Width:           nativePreviewCloseSize,
			Height:          nativePreviewCloseSize,
		},
		Icon:     closeIcon,
		IconSize: 20,
		OnClick: func() bool {
			a.closePreviewWindow()
			return true
		},
	})
}

// closeNativePreviewCloseOverlay removes the independent close button during preview teardown.
func (a *App) closeNativePreviewCloseOverlay() {
	overlay.Close(a.nativePreviewCloseOverlayID())
}
