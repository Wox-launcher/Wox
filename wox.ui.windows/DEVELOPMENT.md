# Wox.UI.Windows - 安装和开发指南

## 前置要求

### 1. 安装 .NET 8 SDK

从 Microsoft 官网下载并安装 .NET 8 SDK：

- 下载地址: https://dotnet.microsoft.com/download/dotnet/8.0
- 选择 "SDK x64" 版本进行安装
- 安装后重启终端/IDE

验证安装：

```powershell
dotnet --version
# 应该显示 8.0.x 版本号
```

## 构建步骤

### 1. 恢复依赖

```powershell
cd wox.ui.windows
dotnet restore
```

### 2. 编译项目

```powershell
dotnet build --configuration Release
```

### 3. 运行项目（开发模式）

```powershell
dotnet run
```

## 依赖包说明

项目使用以下 NuGet 包：

1. **CommunityToolkit.Mvvm** (8.2.2)
   - 提供 MVVM 架构支持
   - 包含 ObservableObject、RelayCommand 等

2. **Websocket.Client** (5.1.1)
   - WebSocket 客户端实现
   - 用于与 wox.core 的实时通信

3. **System.Text.Json** (8.0.4)
   - 高性能 JSON 序列化/反序列化
   - 用于 RPC 消息处理

4. **Wpf.Ui** (3.0.4)
   - 现代化 WPF UI 控件库
   - 提供 Fluent Design 风格界面

## 项目结构

```
wox.ui.windows/
├── App.xaml                    # 应用程序定义
├── App.xaml.cs                 # 应用程序入口
├── MainWindow.xaml             # 主窗口界面
├── MainWindow.xaml.cs          # 主窗口逻辑
├── GlobalUsings.cs             # 全局 using 指令
├── Models/                     # 数据模型
│   ├── WebsocketMsg.cs        # WebSocket 消息模型
│   └── Query.cs               # 查询和结果模型
├── ViewModels/                 # 视图模型
│   └── MainViewModel.cs       # 主窗口 ViewModel
├── Services/                   # 服务层
│   └── WoxApiService.cs       # API 通信服务
├── Converters/                 # XAML 转换器
│   └── BooleanToVisibilityConverter.cs
└── README.md                   # 项目说明
```

## 开发说明

### 通信协议

UI 与 wox.core 通过 WebSocket 和 HTTP 通信：

**WebSocket** (`ws://localhost:<port>/ws`)

- 用于实时查询和结果推送
- 双向通信，支持请求/响应模式

**HTTP** (`http://localhost:<port>`)

- 用于初始化和事件通知
- 单向请求

### 消息格式

所有 WebSocket 消息使用 JSON 格式：

```json
{
  "RequestId": "uuid",
  "TraceId": "uuid",
  "SessionId": "uuid",
  "Method": "Query|PushResults|Action|...",
  "Type": "request|response",
  "Data": {...},
  "Success": true
}
```

### 启动参数

应用程序接收来自 wox.core 的启动参数：

```
Wox.exe <ServerPort> <ServerPid> <IsDev>
```

示例：

```
Wox.exe 34982 12345 false
```

开发调试时可在 `App.xaml.cs` 中设置默认值。

## 集成到 Wox 构建系统

在根目录的 Makefile 中添加：

```makefile
.PHONY: build-ui-windows

build-ui-windows:
	$(MAKE) -C wox.ui.windows build
```

## 调试技巧

1. **查看 WebSocket 消息**
   - 在 `WoxApiService.HandleWebSocketMessage` 设置断点
   - 观察收发的 JSON 消息

2. **测试 UI 渲染**
   - 可以硬编码一些测试数据到 `MainViewModel`
   - 不依赖 wox.core 进行 UI 调试

3. **日志输出**
   - 使用 `Console.WriteLine` 输出到控制台
   - 或集成 `Microsoft.Extensions.Logging`

## 已知限制

1. 图标渲染：目前仅显示占位符图标，需要实现 WoxImage 到 WPF ImageSource 的转换
2. 主题切换：暂未实现动态主题切换逻辑
3. 预览面板：仅支持纯文本，需要添加 Markdown/HTML 渲染
4. 窗口位置：暂未实现位置记忆功能

## 下一步计划

- [ ] 实现图标渲染（支持 base64, svg, file path 等格式）
- [ ] 添加主题服务，支持动态切换
- [ ] 集成 Markdown 预览渲染器
- [ ] 添加窗口动画效果
- [ ] 实现窗口位置和大小记忆
- [ ] 支持高 DPI 显示器
- [ ] 添加文件拖放功能
- [ ] 集成全局热键注册（P/Invoke）

## 贡献

欢迎提交 PR 改进 WPF UI 实现！

## 许可证

遵循 Wox 项目的 MIT 许可证。
