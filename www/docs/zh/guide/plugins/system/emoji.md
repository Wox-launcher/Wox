# Emoji 插件

使用 `emoji` 在 Wox 中搜索并复制 Emoji。

![Emoji 插件](/images/guide/plugin_emoji.jpg)

## 快速开始

```text
emoji smile
emoji check
emoji heart
emoji flag
```

结果以网格展示，方便快速扫描。按 `Enter` 复制选中的 Emoji。操作面板可以复制更大的 Emoji 图片、添加关键字，或在支持时粘贴到当前窗口。

## AI 匹配

AI 匹配是可选功能。启用后，Wox 可以匹配不在内置名称里的描述性短语。

1. 先在 [AI 设置](../../ai/settings.md) 中配置 provider。
2. 打开 **设置 -> 插件 -> Emoji**。
3. 启用 AI 匹配并选择模型。

```text
emoji green success mark
emoji red warning
emoji cloudy weather
```

AI 匹配会把查询文本发送给选中的模型。如果你希望 Emoji 搜索完全本地化，保持关闭即可。

## 排序

经常使用的 Emoji 会逐渐靠前。如果第一次搜索时常用项没有排在第一，正常复制几次后 Wox 会根据使用记录调整。
