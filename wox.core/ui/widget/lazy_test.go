package widget

import (
	"fmt"
	"math"
	"testing"

	woxui "wox/ui/runtime"
)

func TestLazyListBuildsSameVisibleCountForGrowingItemSets(t *testing.T) {
	viewport := float32(100)
	extent := float32(20)
	var counts []int
	for _, itemCount := range []int{50, 500, 5000} {
		built := 0
		root := (ScrollView{
			Width: 80, Height: viewport, Offset: 0,
			Child: LazyList{
				Width: 80, ItemCount: itemCount, ItemExtent: extent,
				ItemKey: func(index int) Key { return Key(fmt.Sprintf("item-%d", index)) },
				ItemBuilder: func(int) Widget {
					built++
					return Container{Width: 80, Height: extent}
				},
			},
		}).layout(context{window: &fakeHostServices{}}, constraints{width: 80, height: viewport})
		if root.bounds.Height != viewport {
			t.Fatalf("%d items: viewport height = %.0f, want %.0f", itemCount, root.bounds.Height, viewport)
		}
		counts = append(counts, built)
	}
	if counts[0] == 0 || counts[0] != counts[1] || counts[1] != counts[2] {
		t.Fatalf("built item counts = %v, want identical non-zero visible windows", counts)
	}
}

func TestLazyListVariableExtentsUsePrefixBinarySearch(t *testing.T) {
	extents := []float32{10, 40, 10, 80, 10, 10, 10}
	built := map[int]bool{}
	(ScrollView{
		Width: 80, Height: 50, Offset: 50,
		Child: LazyList{
			Key: "variable", Width: 80, ItemCount: len(extents), Overscan: NoLazyOverscan,
			ItemExtentAt: func(index int) float32 { return extents[index] },
			ItemKey:      func(index int) Key { return Key(fmt.Sprintf("v-%d", index)) },
			ItemBuilder: func(index int) Widget {
				built[index] = true
				return Container{Width: 80, Height: extents[index]}
			},
		},
	}).layout(context{window: &fakeHostServices{}}, constraints{width: 80, height: 50})

	if !built[3] || built[0] || built[6] {
		t.Fatalf("variable visible items = %v, want the 80px item at offset 50 and not the ends", built)
	}
}

func TestLazyListRebuildsPrefixWhenExtentsChangeWithoutRevision(t *testing.T) {
	extents := []float32{20, 20, 20, 20, 20}
	built := map[int]bool{}
	host := NewHost(func(woxui.FrameInfo) Widget {
		built = map[int]bool{}
		return ScrollView{
			Width: 80, Height: 40, Offset: 0,
			Child: LazyList{
				Key: "stale-extents", Width: 80, ItemCount: len(extents), Overscan: NoLazyOverscan,
				ItemExtentAt: func(index int) float32 { return extents[index] },
				ItemKey:      func(index int) Key { return Key(fmt.Sprintf("e-%d", index)) },
				ItemBuilder: func(index int) Widget {
					built[index] = true
					return Container{Width: 80, Height: extents[index]}
				},
			},
		}
	})
	host.AttachServices(&fakeHostServices{})
	host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 80, Height: 40}})
	if !built[0] || !built[1] || built[4] {
		t.Fatalf("first visible items = %v, want the first two 20px rows", built)
	}

	extents[0] = 80
	host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 80, Height: 40}})
	if !built[0] || built[1] || built[4] {
		t.Fatalf("grown first item visible set = %v, want only the 80px first row", built)
	}
}

func TestLazyGridBuildsOnlyVisibleRows(t *testing.T) {
	built := 0
	(ScrollView{
		Width: 90, Height: 40, Offset: 40,
		Child: LazyGrid{
			Width: 90, Columns: 3, ItemCount: 30, ItemExtent: 20, Overscan: NoLazyOverscan,
			ItemKey: func(index int) Key { return Key(fmt.Sprintf("cell-%d", index)) },
			ItemBuilder: func(int) Widget {
				built++
				return Container{Width: 30, Height: 20}
			},
		},
	}).layout(context{window: &fakeHostServices{}}, constraints{width: 90, height: 40})

	if built != 6 {
		t.Fatalf("built grid cells = %d, want 6 for two visible rows of 3", built)
	}
}

func TestLazyListDoesNotMaterializeAllItemsWhenViewportIsUnknown(t *testing.T) {
	built := 0
	(LazyList{
		Width: 80, ItemCount: 500, ItemExtent: 20,
		ItemBuilder: func(int) Widget {
			built++
			return Container{Width: 80, Height: 20}
		},
	}).layout(context{window: &fakeHostServices{}}, constraints{width: 80, height: math.MaxFloat32})
	if built != 0 {
		t.Fatalf("built %d items with unknown viewport, want 0", built)
	}
}

func TestLazyListStickToEndMovesParentOffset(t *testing.T) {
	itemCount := 5
	offset := float32(60)
	host := NewHost(func(woxui.FrameInfo) Widget {
		return ScrollView{
			Width: 80, Height: 40, Offset: offset,
			Child: LazyList{
				Key: "stick", Width: 80, ItemCount: itemCount, ItemExtent: 20, StickToEnd: true, Overscan: NoLazyOverscan,
				ItemKey:     func(index int) Key { return Key(fmt.Sprintf("s-%d", index)) },
				ItemBuilder: func(int) Widget { return Container{Width: 80, Height: 20} },
			},
		}
	})
	host.AttachServices(&fakeHostServices{})
	host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 80, Height: 40}})

	itemCount = 10
	host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 80, Height: 40}})
	if host.root == nil || len(host.root.children) == 0 {
		t.Fatal("expected a scrolled content child")
	}
	content := host.root.children[0]
	if content.bounds.Y != -160 {
		t.Fatalf("stick-to-end content Y = %.0f, want -160", content.bounds.Y)
	}
	list := findNodeByRole(host.root, woxui.AccessibilityRoleList)
	if list == nil || list.semantic == nil || list.semantic.value != "8-9/10" {
		t.Fatalf("stick-to-end visible range = %+v, want 8-9/10", list)
	}
}

func TestLazyListExposesVisibleRangeOnScrollContainer(t *testing.T) {
	root := (ScrollView{
		Width: 80, Height: 40, Offset: 0,
		Child: LazyList{
			Width: 80, ItemCount: 10, ItemExtent: 20, Overscan: NoLazyOverscan,
			ItemKey:     func(index int) Key { return Key(fmt.Sprintf("a11y-%d", index)) },
			ItemBuilder: func(int) Widget { return Container{Width: 80, Height: 20} },
		},
	}).layout(context{window: &fakeHostServices{}}, constraints{width: 80, height: 40})

	list := findNodeByRole(root, woxui.AccessibilityRoleList)
	if list == nil || list.semantic == nil || list.semantic.value != "0-1/10" {
		t.Fatalf("lazy list a11y value = %+v, want 0-1/10", list)
	}
}

func findNodeByRole(root *node, role woxui.AccessibilityRole) *node {
	if root == nil {
		return nil
	}
	if root.semantic != nil && root.semantic.role == role {
		return root
	}
	for _, child := range root.children {
		if found := findNodeByRole(child, role); found != nil {
			return found
		}
	}
	return nil
}
