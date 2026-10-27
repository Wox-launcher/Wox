package launcher

import (
	"strings"
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestChatMarkdownCacheRendersChinesePunctuationBold(t *testing.T) {
	cache := chatMarkdownCache{}
	props, _ := cache.measure("reply", "**通常不行。**孩子", chatMarkdownLayoutKey{width: 400}, woxcomponent.MarkdownProps{Width: 400})
	root := woxcomponent.WoxMarkdown(props).(woxwidget.Flex)
	paragraph := root.Children[0].(woxwidget.Wrap)
	var value strings.Builder
	for _, child := range paragraph.Children {
		text := child.(woxwidget.Text)
		value.WriteString(text.Value)
		if strings.Contains("通常不行。", text.Value) && text.Style.Weight != woxui.FontWeightSemibold {
			t.Fatalf("text %q is not bold", text.Value)
		}
	}
	if value.String() != "通常不行。孩子" {
		t.Fatalf("rendered text = %q", value.String())
	}
}
