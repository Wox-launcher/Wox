//go:build windows

package component

import (
	"context"
	"fmt"
	"testing"
	"time"

	"wox/i18n"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestMarkdownCodeLineFitsNativeFontMetrics(t *testing.T) {
	err := woxui.Run(func() error {
		window, err := woxui.Open(woxui.WindowOptions{Title: "Markdown font measurement test"})
		if err != nil {
			return err
		}
		defer window.Close()
		for _, fontSize := range []float32{11.7, 13, 14.3, 18} {
			props := MarkdownProps{ID: "test", Width: 120, FontSize: fontSize, Window: window, Document: ParseMarkdown("```text\ngyp mixed 中文\n```")}
			root := WoxMarkdown(props).(woxwidget.Flex)
			code := root.Children[0].(woxwidget.Container).Child.(woxwidget.Flex)
			field := code.Children[1].(woxwidget.Stateful).Widget.(TextFieldProps)
			metrics, err := window.MeasureText(field.Value, field.Style)
			if err != nil {
				return err
			}
			if field.LineHeight < metrics.Size.Height {
				return fmt.Errorf("font size %g: line height %g is smaller than measured font height %g", fontSize, field.LineHeight, metrics.Size.Height)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestMarkdownCopyFeedbackResetsAfterLatestCopy(t *testing.T) {
	SetClipboardProvider(&memoryClipboard{})
	defer SetClipboardProvider(nil)
	var result error
	err := woxui.Run(func() error {
		window, err := woxui.Open(woxui.WindowOptions{Title: "Markdown copy feedback test"})
		if err != nil {
			return err
		}
		host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
			return WoxMarkdown(MarkdownProps{ID: "md", Document: ParseMarkdown("```\ntext\n```"), Width: 300})
		})
		host.AttachServices(&hotkeyRecorderHostServices{})
		frame := func() string {
			host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 300, Height: 100}, Scale: 1})
			for _, node := range host.Snapshot().Tree.Nodes {
				if node.AutomationID == "md-code-copy-1" {
					return node.Label
				}
			}
			return ""
		}
		frame()
		if !host.FocusAutomationID("md-code-copy-1") || !host.Key(woxui.KeyEvent{Key: woxui.KeyEnter, Down: true}) {
			host.Dispose()
			window.Close()
			return fmt.Errorf("could not activate copy button")
		}
		go func() {
			time.Sleep(700 * time.Millisecond)
			_ = woxui.Call(func() { host.Key(woxui.KeyEvent{Key: woxui.KeyEnter, Down: true}) })
			time.Sleep(700 * time.Millisecond)
			_ = woxui.Call(func() {
				if label := frame(); label != i18n.GetI18nManager().TranslateWox(context.Background(), "toolbar_copied") {
					result = fmt.Errorf("feedback ended before the latest copy: %q", label)
				}
			})
			time.Sleep(700 * time.Millisecond)
			_ = woxui.Call(func() {
				if label := frame(); label != i18n.GetI18nManager().TranslateWox(context.Background(), "selection_copy_to_clipboard") {
					result = fmt.Errorf("feedback did not reset: %q", label)
				}
				host.Dispose()
				window.Close()
			})
		}()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Fatal(result)
	}
}
