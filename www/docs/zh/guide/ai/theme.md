# 主题生成

AI 主题生成会根据一段描述创建 Wox 主题。使用前先配置 [AI 设置](./settings.md)。

## 生成主题

打开 Wox 并运行：

```text
theme ai dark graphite with teal accents
```

建议在 prompt 里描述对比度、氛围和强调色：

```text
theme ai light theme, warm background, blue accent, low contrast borders
theme ai high contrast black theme with orange selection
theme ai macOS style translucent gray with green accent
```

![AI Theme](/images/ai_theme.jpg)

## 保留前先检查

AI 生成的主题可能接近目标，但仍需要人工判断：

- 普通和选中状态下的搜索文本都清楚可读。
- 当前选中的结果足够明显。
- 副标题、尾部信息和动作文字有足够对比度。
- 在你平时使用的窗口大小下仍然正常。

如果效果不理想，用更具体的颜色、对比度和选中态描述再生成一次。
