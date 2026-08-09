package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestLauncherQueryRemainsFocusableWithoutOwningFocus(t *testing.T) {
	query := launcherQueryEditable(LauncherQueryView(LauncherQueryProps{Height: 40, Focused: false, Enabled: true}))
	if query.Disabled {
		t.Fatal("unfocused query was disabled instead of remaining pointer-focusable")
	}
}

func TestLauncherQueryKeepsMinimumEditableAreaBeforeDragOverlay(t *testing.T) {
	tapped := false
	query := LauncherQueryView(LauncherQueryProps{
		Width: 500, Height: 40, TextWidth: 50, Enabled: true,
		OnTapEnd: func() { tapped = true },
	}).(woxwidget.Stack)
	dragArea := query.Children[1]
	if dragArea.Left != 350 {
		t.Fatalf("drag area left = %v, want 350", dragArea.Left)
	}
	dragGesture := dragArea.Child.(woxwidget.Gesture)
	if dragGesture.Cursor != woxui.PointerCursorDefault {
		t.Fatalf("drag area cursor = %v, want default", dragGesture.Cursor)
	}
	dragGesture.OnTap()
	if !tapped {
		t.Fatal("drag area tap did not request query focus")
	}
}

func TestLauncherQueryForwardsMultiClickSelection(t *testing.T) {
	doubleTaps, tripleTaps := 0, 0
	query := launcherQueryEditable(LauncherQueryView(LauncherQueryProps{
		Height: 40, Enabled: true, OnDoubleTapAt: func(woxui.Point) { doubleTaps++ }, OnTripleTapAt: func(woxui.Point) { tripleTaps++ },
	}))
	gesture := query.Child.(woxwidget.Gesture)
	gesture.OnDoubleTapAt(woxui.Point{X: 10})
	gesture.OnTripleTapAt(woxui.Point{X: 10})
	if doubleTaps != 1 || tripleTaps != 1 {
		t.Fatalf("query multi-click callbacks = double %d, triple %d, want 1 each", doubleTaps, tripleTaps)
	}
}

func TestLauncherQueryUsesSharedScrollViewForHiddenLines(t *testing.T) {
	query := LauncherQueryView(LauncherQueryProps{
		Width: 100, Height: 136, LineHeight: 34, CaretHeight: 34, CaretLine: 4, Lines: make([]LauncherQueryLine, 5),
	}).(woxwidget.Stateful)
	props := query.Widget.(woxcomponent.ScrollViewProps)
	if props.ContentHeight != 170 || props.KeepVisible == nil || props.KeepVisible.Start != 136 || props.KeepVisible.End != 170 || !props.AlwaysShowScrollbar || props.AutomationID != "launcher.query.scroll" {
		t.Fatalf("query scroll props = content %.0f keep %#v always visible %v automation %q", props.ContentHeight, props.KeepVisible, props.AlwaysShowScrollbar, props.AutomationID)
	}
}

func TestLauncherQueryLeavesSharedScrollbarGutterOutsideDragOverlay(t *testing.T) {
	query := LauncherQueryView(LauncherQueryProps{
		Width: 500, Height: 136, TextWidth: 50, LineHeight: 34, Lines: make([]LauncherQueryLine, 5),
	}).(woxwidget.Stack)
	dragArea := query.Children[1].Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	if dragArea.Width != 136 {
		t.Fatalf("multiline drag area width = %.0f, want 136", dragArea.Width)
	}
}

func TestLauncherQueryExposesInlineCompletionSuffix(t *testing.T) {
	query := LauncherQueryView(LauncherQueryProps{Width: 500, Height: 40, CompletionSuffix: "pleted", Enabled: true}).(woxwidget.Stack)
	scrollSemantic := query.Children[0].Child.(woxwidget.Semantics)
	scrollStack := scrollSemantic.Child.(woxwidget.Gesture).Child.(woxwidget.Stack)
	scroll := scrollStack.Children[0].Child.(woxwidget.ScrollView)
	content := scroll.Child.(woxwidget.Stack)
	completion := content.Children[1].Child.(woxwidget.Semantics)
	if completion.AutomationID != "launcher.query.completion" || completion.Role != woxui.AccessibilityRoleText || completion.Value != "pleted" || !completion.ReadOnly || completion.LiveRegion != woxui.AccessibilityLiveRegionPolite {
		t.Fatalf("query completion semantics = %#v", completion)
	}
}

func TestLauncherHeaderExposesQueryLoadingProgress(t *testing.T) {
	header := LauncherHeaderView(LauncherHeaderProps{
		Width: 500, Height: 50, QueryBoxHeight: 50, QueryEditorHeight: 34, QueryWidth: 400,
		Loading: true, LoadingWidth: 49, LoadingSize: 20,
	}).(woxwidget.Container)
	row := header.Child.(woxwidget.Container).Child.(woxwidget.Flex)
	loading := row.Children[1].(woxwidget.Semantics)
	if loading.AutomationID != "launcher.query.loading" || loading.Role != woxui.AccessibilityRoleProgressBar || loading.Value != "loading" || !loading.ReadOnly {
		t.Fatalf("query loading semantics = id %q role %q value %q readonly %v", loading.AutomationID, loading.Role, loading.Value, loading.ReadOnly)
	}
	boundary := loading.Child.(woxwidget.Boundary[launcherQueryLoadingProps])
	if boundary.Key != LauncherQueryLoadingBoundaryKey {
		t.Fatalf("query loading boundary key = %q, want %q", boundary.Key, LauncherQueryLoadingBoundaryKey)
	}
}

func TestLauncherQueryBoundaryEqualCoversAllFields(t *testing.T) {
	woxwidget.AssertEqualCoversAllFields(t, LauncherQueryProps{})
	woxwidget.AssertEqualCoversAllFields(t, launcherQueryLoadingProps{})
}

func TestGlanceBoundaryEqualCoversAllFields(t *testing.T) {
	woxwidget.AssertEqualCoversAllFields(t, GlanceProps{})
}

func launcherQueryEditable(widget woxwidget.Widget) woxwidget.EditableText {
	if stack, ok := widget.(woxwidget.Stack); ok {
		widget = stack.Children[0].Child
	}
	if stateful, ok := widget.(woxwidget.Stateful); ok {
		return stateful.Widget.(woxcomponent.ScrollViewProps).Content.(woxwidget.EditableText)
	}
	if semantics, ok := widget.(woxwidget.Semantics); ok {
		widget = semantics.Child
	}
	stack := widget.(woxwidget.Gesture).Child.(woxwidget.Stack)
	scroll := stack.Children[0].Child.(woxwidget.ScrollView)
	return scroll.Child.(woxwidget.EditableText)
}
