//go:build darwin

package main

import (
	"log"
	"time"

	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const capturePath = "/tmp/wox-table-qa-artifacts/table-preview.png"

func main() {
	if err := woxui.Run(func() error {
		theme := previewTheme()
		host := woxwidget.NewHost(func(frame woxui.FrameInfo) woxwidget.Widget {
			return buildPreview(frame.Size.Width, frame.Size.Height, theme)
		})
		window, err := woxui.Open(woxui.WindowOptions{
			Title:   "Wox table preview",
			Size:    woxui.Size{Width: 1200, Height: 800},
			Role:    woxui.WindowRoleApplication,
			OnFrame: host.Frame,
		})
		if err != nil {
			return err
		}
		host.Attach(window)
		if _, err := window.Show(); err != nil {
			return err
		}
		go func() {
			time.Sleep(750 * time.Millisecond)
			_ = woxui.Call(func() {
				if err := window.CapturePNG(capturePath); err != nil {
					log.Printf("capture table preview: %v", err)
				}
				_ = window.Close()
			})
		}()
		return nil
	}); err != nil {
		log.Fatal(err)
	}
}

func buildPreview(width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
	contentWidth := width - 56
	queryHotkeysHeight := launcherview.FormTableFieldHeight(true, "通过快捷键快速触发预定义的查询。支持使用变量来构建动态查询，还可以设置静默执行模式自动执行单一结果。", 0, 300)
	queryShortcutsHeight := launcherview.FormTableFieldHeight(true, "为常用查询设置简短的别名。只有当缩写是查询的第一个完整单词时才会展开。", 0, 300)
	return woxwidget.Container{
		Width: width, Height: height, Color: theme.Background,
		Padding: woxwidget.Insets{Left: 28, Top: 30, Right: 28, Bottom: 30},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			launcherview.FormTableField(launcherview.FormTableFieldProps{
				ID: "query-hotkeys", Title: "快捷键查询", Description: "通过快捷键快速触发预定义的查询。支持使用变量来构建动态查询，还可以设置静默执行模式自动执行单一结果。",
				Width: contentWidth, Height: queryHotkeysHeight, MaxHeight: 300, InlineTitle: true,
				Columns:  []launcherview.FormTableColumn{{Label: "名称", Tooltip: "查询名称", Width: 140}, {Label: "快捷键", Tooltip: "触发快捷键", Width: 120}, {Label: "查询", Tooltip: "查询内容"}, {Label: "禁用", Tooltip: "是否禁用", Width: 60}},
				AddLabel: "添加", EditLabel: "编辑", OperationLabel: "操作", EmptyLabel: "暂无数据", Theme: theme,
			}),
			launcherview.FormTableField(launcherview.FormTableFieldProps{
				ID: "query-shortcuts", Title: "查询缩写", Description: "为常用查询设置简短的别名。只有当缩写是查询的第一个完整单词时才会展开。",
				Width: contentWidth, Height: queryShortcutsHeight, MaxHeight: 300, InlineTitle: true,
				Columns:  []launcherview.FormTableColumn{{Label: "快捷键", Tooltip: "查询缩写", Width: 120}, {Label: "查询", Tooltip: "查询内容"}, {Label: "禁用", Tooltip: "是否禁用", Width: 60}},
				AddLabel: "添加", EditLabel: "编辑", OperationLabel: "操作", EmptyLabel: "暂无数据", Theme: theme,
			}),
		}},
	}
}

func previewTheme() woxcomponent.Theme {
	return woxcomponent.Theme{
		Background: woxui.Color{R: 29, G: 32, B: 34, A: 255}, QueryBackground: woxui.Color{R: 40, G: 43, B: 47, A: 235},
		ResultTitle: woxui.Color{R: 238, G: 240, B: 244, A: 255}, ResultSubtitle: woxui.Color{R: 174, G: 179, B: 189, A: 255},
		ActionHeader: woxui.Color{R: 174, G: 179, B: 189, A: 255}, ActionText: woxui.Color{R: 238, G: 240, B: 244, A: 255},
		ActionSelected: woxui.Color{R: 78, G: 84, B: 92, A: 255}, ActionSelectedText: woxui.Color{R: 255, G: 255, B: 255, A: 255},
		PreviewText: woxui.Color{R: 238, G: 240, B: 244, A: 255}, PreviewSplit: woxui.Color{R: 82, G: 87, B: 94, A: 180},
	}
}
