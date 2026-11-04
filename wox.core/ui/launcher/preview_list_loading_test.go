package launcher

import (
	"testing"

	"wox/common/icons"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestListPreviewUsesSharedLoadingIndicator(t *testing.T) {
	loading := fromCoreImage(icons.Get(icons.StatusLoading))
	ordinary := fromCoreImage(icons.Get(icons.ActionSearch))
	image := &woxui.Image{}
	loaded := 0
	controller := previewController{imageFor: func(source woxImage) *woxui.Image {
		if source != ordinary {
			t.Fatalf("unexpected image lookup: %+v", source)
		}
		loaded++
		return image
	}}

	view := controller.buildListPreview(previewListData{Items: []previewListItem{
		{Title: "loading", Icon: &loading},
		{Title: "image", Icon: &ordinary},
		{Title: "no icon"},
	}}, uiPalette{}, 300, 200).(woxwidget.Container)
	rows := view.Child.(woxwidget.Flex).Children
	icon := func(index int) woxwidget.Widget {
		return rows[index].(woxwidget.Container).Child.(woxwidget.Flex).Children[0]
	}
	if indicator, ok := icon(0).(woxwidget.LoopAnimation); !ok || indicator.Key != "wox-loading-indicator" {
		t.Fatalf("loading icon = %#v, want shared loading indicator", icon(0))
	}
	if got, ok := icon(1).(woxwidget.Image); !ok || got.Source != image || loaded != 1 {
		t.Fatalf("ordinary icon = %#v, image lookups = %d", icon(1), loaded)
	}
	if _, ok := icon(2).(woxwidget.Container); !ok {
		t.Fatalf("missing icon = %#v, want fallback", icon(2))
	}
}
