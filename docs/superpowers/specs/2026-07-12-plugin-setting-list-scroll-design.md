# 插件设置列表自动定位设计

## 问题

通过插件 action 打开 `/plugin/setting` 时，详情区会选中目标插件，但插件列表偶尔不会把该插件滚动到可视区域中部。

当前 `ensurePluginVisible` 使用固定的 `88px` 估算行高。实际列表项高度与该值不同；此外，目标项尚未由 `ListView.builder` 创建时，提前读取的 `GlobalKey` 为 `null`，滚动完成后无法精确校正。

## 方案

只修改共享的 `ensurePluginVisible`：

1. 根据目标索引、列表长度和当前实际 `maxScrollExtent` 计算近似居中位置，移除固定行高假设。
2. 完成首次滚动并等待下一帧后，重新读取目标项的 `GlobalKey`。
3. 如果目标项已经挂载，调用 `Scrollable.ensureVisible(alignment: 0.5)` 做最终居中校正。

插件列表布局、筛选行为和 action 路由保持不变，不引入新依赖或抽象。

## 验证

按仓库要求格式化修改的 Dart 文件，并只对该文件运行 Flutter 静态分析；不运行 Flutter build、单元测试或 smoke test。
