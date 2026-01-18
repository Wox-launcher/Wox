# Wox UI Windows - Version History

## Version 1.0.0 (Initial Release) - 2026-01-18

### Features

- ✅ 完整的 WPF UI 实现
- ✅ WebSocket 实时通信
- ✅ 搜索输入和结果显示
- ✅ 键盘导航（Up/Down/Enter/Escape）
- ✅ 鼠标交互（点击执行、拖拽移动）
- ✅ 预览面板（基础文本）
- ✅ 图标渲染（File/Base64/URL）
- ✅ 主题框架
- ✅ 失焦自动隐藏
- ✅ 测试模式（独立运行）

### Architecture

- MVVM 架构（CommunityToolkit.Mvvm）
- 服务层设计（WoxApiService, ImageService, ThemeService）
- 事件驱动通信
- 双向数据绑定

### Dependencies

- .NET 8.0
- CommunityToolkit.Mvvm 8.2.2
- Websocket.Client 5.1.1
- System.Text.Json 8.0.4
- Wpf.Ui 3.0.4

### Known Limitations

- SVG 图标暂未支持（预留接口）
- Markdown 预览未实现
- 窗口位置不记忆
- 无动画效果
- 主题需要完善映射

### Files Created

- 27 个文件
- ~2000 行代码
- 完整文档（README, DEVELOPMENT, INTEGRATION, CHECKLIST, PROJECT_SUMMARY）

### Testing

- 提供测试窗口和示例数据
- 启动脚本（run-dev, run-test）
- 检查清单

---

## Roadmap

### Version 1.1.0 (Planned)

- [ ] SVG 图标支持
- [ ] Markdown 预览
- [ ] 窗口位置记忆
- [ ] 完整主题映射
- [ ] 性能优化

### Version 1.2.0 (Planned)

- [ ] 动画效果
- [ ] 文件拖放
- [ ] DPI 感知
- [ ] 错误处理优化
- [ ] 日志系统

### Version 2.0.0 (Future)

- [ ] 插件 UI 扩展
- [ ] 高级预览（图片、视频等）
- [ ] 自定义皮肤
- [ ] 无障碍功能
- [ ] 自动更新

---

## Contributors

- Initial implementation: AI Assistant
- Framework: Wox Project

## License

MIT License (same as Wox project)
