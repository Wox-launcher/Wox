package preview

import (
	"fmt"
	"math"
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestChatPickerLayoutScalesRowsAndStaysInsideNarrowHost(t *testing.T) {
	for _, scale := range []float32{0.9, 1, 1.1} {
		for _, dpi := range []float32{1, 1.5, 2} {
			t.Run(fmt.Sprintf("density-%g-dpi-%g", scale, dpi), func(t *testing.T) {
				contentHeight := 3*ChatScaledSize(scale, 28) + ChatScaledSize(scale, 38) + ChatScaledSize(scale, 50) + ChatScaledSize(scale, ChatPickerSectionGap)
				height := ChatPickerHeight(contentHeight, scale, 400)
				props := ChatCatalogProps{Picker: true, Width: 280, Height: height, DensityScale: scale,
					ContentHeight: contentHeight, Filters: []ChatCatalogFilter{{ID: "all", Label: "All", Selected: true}},
					Items: []ChatCatalogItemProps{
						{SelectID: "model", Kind: "models", Picker: true, Title: "long/provider/model-name", GroupLabel: "Provider", SectionLabel: "Models", RowHeight: 38, Current: true},
						{SelectID: "skill", Kind: "skills", Picker: true, Title: "skill", Subtitle: "A longer description", GroupLabel: "Skills", SectionLabel: "Skills", RowHeight: 50},
					},
				}
				host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget { return ChatCatalog(props) })
				host.AttachServices(terminalPreviewHostServices{})
				defer host.Dispose()
				host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{
					Size: woxui.Size{Width: 280, Height: height}, Scale: dpi,
					PixelSize: woxui.PixelSize{Width: int(280 * dpi), Height: int(height * dpi)},
				})
				found := 0
				for _, node := range host.Snapshot().Tree.Nodes {
					if node.AutomationID != "model" && node.AutomationID != "skill" {
						continue
					}
					found++
					wantHeight := ChatScaledSize(scale, 38)
					if node.AutomationID == "skill" {
						wantHeight = ChatScaledSize(scale, 50)
					}
					if math.Abs(float64(node.Bounds.Height-wantHeight)) > 0.1 {
						t.Fatalf("%s height = %g, want %g", node.AutomationID, node.Bounds.Height, wantHeight)
					}
					if node.Bounds.X < 0 || node.Bounds.X+node.Bounds.Width > 280.1 || node.Bounds.Y+node.Bounds.Height > height+0.1 {
						t.Fatalf("%s outside picker: %+v height %g", node.AutomationID, node.Bounds, height)
					}
				}
				if found != 2 {
					t.Fatalf("found %d accessible rows", found)
				}
			})
		}
	}
}

func TestChatPickerCurrentModelAndKeyboardFocusRemainIndependent(t *testing.T) {
	theme := woxcomponent.Theme{SelectedBackground: woxui.Color{R: 50, A: 255}}
	for _, current := range []bool{false, true} {
		for _, selected := range []bool{false, true} {
			item := ChatCatalogItemProps{Kind: "models", Current: current, Selected: selected}
			row := chatPickerItem(item, 300, 38, theme, false, nil).(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
			if (row.Color.A != 0) != selected {
				t.Fatal("current model changed the keyboard highlight")
			}
			children := row.Child.(woxwidget.Flex).Children
			if (len(children) == 2) != current {
				t.Fatal("checkmark does not match current model")
			}
		}
	}
}

func TestChatPickerShowsRealIconsAndSeparatesOnlyContentTypes(t *testing.T) {
	providerIcon := &woxui.Image{}
	catalog := ChatCatalog(ChatCatalogProps{
		Picker: true, Width: 400, Height: 150, ContentHeight: 300,
		Theme: woxcomponent.Theme{ResultSubtitle: woxui.Color{A: 255}},
		Items: []ChatCatalogItemProps{
			{Kind: "models", SectionLabel: "Models", GroupLabel: "deepseek", GroupIcon: providerIcon, Picker: true, RowHeight: 38},
			{Kind: "models", SectionLabel: "Models", GroupLabel: "magpie", GroupIcon: providerIcon, Picker: true, RowHeight: 38},
			{Kind: "skills", SectionLabel: "Skills", Picker: true, RowHeight: 50},
		},
	}).(woxwidget.Container)
	body := catalog.Child.(woxwidget.Flex)
	scroll := body.Children[1].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	rows := scroll.Content.(woxwidget.Flex).Children
	if len(rows) != 9 {
		t.Fatalf("want type titles, provider headers, rows, whitespace and one divider; got %d", len(rows))
	}
	if title := rows[0].(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Text).Value; title != "Models" {
		t.Fatalf("missing Models title: %q", title)
	}
	for _, index := range []int{1, 4} {
		header := rows[index].(woxwidget.Container).Child.(woxwidget.Flex)
		if icon := header.Children[0].(woxwidget.Image); icon.Source != providerIcon {
			t.Fatal("provider header did not use the supplied real icon")
		}
	}
	gap := rows[3].(woxwidget.Container)
	if gap.Height != ChatPickerSectionGap || gap.Child != nil || gap.Color.A != 0 {
		t.Fatal("provider boundary must be whitespace only")
	}
	divider := rows[6].(woxwidget.Align)
	line := divider.Child.(woxwidget.Container)
	if divider.Height != ChatPickerSectionGap || line.Height != 1 || line.Color.A == 0 {
		t.Fatal("model/skill boundary lost its divider")
	}
	if title := rows[7].(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Text).Value; title != "Skills" {
		t.Fatalf("missing Skills title: %q", title)
	}
}

func TestMentionPickerUsesSharedDimensionsAndOriginalIcons(t *testing.T) {
	for _, scale := range []float32{0.9, 1, 1.1} {
		for _, width := range []float32{320, 1000} {
			props := prepareChatConversation(ChatConversationProps{ChatPreviewProps: ChatPreviewProps{
				Width: width, Height: 900,
				Catalog: &ChatCatalogProps{Picker: true, DensityScale: scale, ContentHeight: 1000},
			}})
			if props.Catalog.Width != min(width-20, ChatScaledSize(scale, 560)) || props.Catalog.Height != ChatScaledSize(scale, 420) {
				t.Fatalf("picker geometry at density %g width %g: %+v", scale, width, props.Catalog)
			}
			icon := &woxui.Image{}
			row := chatPickerItem(ChatCatalogItemProps{Kind: "files", Icon: icon, DensityScale: scale},
				280, ChatScaledSize(scale, 38), woxcomponent.Theme{}, false, nil).(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
			image := row.Child.(woxwidget.Flex).Children[0].(woxwidget.Image)
			if image.Source != icon || image.Width != ChatScaledSize(scale, 20) {
				t.Fatal("mention icon source or density was lost")
			}
		}
	}
}
