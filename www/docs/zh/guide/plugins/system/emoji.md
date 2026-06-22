# Emoji 插件

使用 `emoji` 在 Wox 中搜索并复制 Emoji。

## 快速开始

```text
emoji smile
emoji check
emoji heart
emoji flag
```

结果以网格展示，方便快速扫描。按 `Enter` 复制选中的 Emoji。

![Emoji 插件网格结果](/images/system-plugin-emoji.png)

## AI 匹配

AI 匹配是可选功能。启用后，Wox 可以匹配不在内置名称里的描述性短语。

1. 先在 [AI 设置](../../ai/settings.md) 中配置 provider。
2. 打开 **设置 -> 插件 -> Emoji**。
3. 启用 AI 匹配并选择模型。

示例：

```text
emoji green success mark
emoji red warning
emoji happy face
emoji cloudy weather
```

AI 匹配会把查询文本发送给选中的模型。如果你希望 Emoji 搜索完全本地化，保持关闭即可。

## 排序

经常使用的 Emoji 会逐渐靠前。如果第一次搜索时常用项没有排在第一，正常复制几次后 Wox 会根据使用记录调整。

## 动作

打开操作面板后，可以在支持时复制更大的 Emoji 图片、添加关键字，或从常用结果中移除某一项。
