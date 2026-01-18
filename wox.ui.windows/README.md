# Wox UI Windows (WPF)

原生 Windows WPF 前端，实现 Wox 启动器功能。

## 技术栈

- .NET 8 / WPF
- CommunityToolkit.Mvvm (MVVM 架构)
- Wpf.Ui (现代化 UI 控件)
- Websocket.Client (WebSocket 通信)
- System.Text.Json (JSON 序列化)

## 架构

### 通信层 (`Services/`)

- **WoxApiService**: WebSocket 和 HTTP 客户端，负责与 `wox.core` 通信
  - WebSocket: `ws://localhost:<port>/ws` 用于实时查询和结果推送
  - HTTP: 用于初始化通知 (`/on/ready`, `/on/focus/lost`)

### 数据模型 (`Models/`)

- **WebsocketMsg**: JSON-RPC 消息格式
- **Query**: 查询请求模型
- **QueryResult**: 查询结果模型
- **ResultItem**: 单个结果项（包含标题、副标题、图标、预览等）

### 视图模型 (`ViewModels/`)

- **MainViewModel**: 主窗口的 ViewModel
  - 管理查询文本、结果列表、选中项
  - 处理键盘导航和动作执行

### UI (`MainWindow.xaml`)

- 无边框、透明、置顶窗口
- 三部分布局：
  - **顶部**: 搜索输入框
  - **中部**: 结果列表 + 预览面板（可选）
  - **底部**: 状态栏

## 协议

### UI → Core (Request)

- `Query`: 发送搜索查询
- `Action`: 执行选中结果的动作
- `Log`: 发送日志

### Core → UI (Request)

- `PushResults`: 推送搜索结果
- `ChangeQuery`: 更改搜索框内容
- `ShowApp`: 显示窗口
- `HideApp`: 隐藏窗口
- `ToggleApp`: 切换窗口显示状态

## 启动

应用程序接收三个命令行参数：

```
Wox.exe <ServerPort> <ServerPid> <IsDev>
```

开发调试时可在 `App.xaml.cs` 中设置默认值。

### 测试模式（无需 wox.core）

可以使用测试模式独立测试 UI，无需运行 wox.core：

```powershell
# 使用测试脚本
.\run-test.ps1

# 或手动运行
dotnet run -- --test
```

测试窗口提供：

- 预设示例数据
- 主题切换测试
- UI 组件预览
- 不依赖后端

## 构建

### 开发构建（需要 .NET 8 SDK）

```bash
cd wox.ui.windows
dotnet restore
dotnet build
dotnet run
```

### 发布版本（自包含，用户无需安装 .NET）

```bash
make publish
# 输出: ./publish/Wox.exe （约 70-80MB，包含运行时）
```

**说明**: 发布版本使用 Self-Contained 模式，将 .NET 8 运行时打包到 exe 中，Windows 10/11 用户可直接运行，无需安装任何额外组件。

## 功能特性

- ✅ WebSocket 实时通信
- ✅ 搜索结果流式更新
- ✅ 键盘导航（Up/Down/Enter/Escape）
- ✅ 鼠标点击执行
- ✅ 失去焦点自动隐藏
- ✅ 拖拽移动窗口
- ✅ 结果预览面板
- ✅ 现代化 UI 设计

## TODO

- [ ] 图标渲染（支持多种格式）
- [ ] 主题动态切换
- [ ] Markdown 预览
- [ ] 动画效果
- [ ] 窗口位置记忆
- [ ] DPI 感知
- [ ] 文件拖放
- [ ] 全局热键注册
