# Wox UI Windows - 快速检查清单

## 构建前检查

- [ ] 已安装 .NET 8 SDK (`dotnet --version` 应显示 8.0.x)
- [ ] 已克隆完整的 Wox 仓库
- [ ] 位于 `wox.ui.windows` 目录

## 首次构建

```powershell
# 1. 恢复依赖
dotnet restore

# 2. 编译项目
dotnet build --configuration Debug

# 3. 检查编译输出
ls bin/Debug/net8.0-windows/

# 预期输出文件：
# - Wox.exe
# - Wox.dll
# - *.deps.json
# - *.runtimeconfig.json
```

## 运行前检查

- [ ] wox.core 正在运行（Go 后端）
- [ ] 知道 WebSocket 端口号（查看 wox.core 日志）
- [ ] 防火墙允许本地连接

## 运行测试

```powershell
# 方式 1: 使用启动脚本（推荐）
.\run-dev.ps1

# 方式 2: 手动运行（需指定正确的端口）
dotnet run <ServerPort> 0 true
# 例如: dotnet run 34982 0 true

# 方式 3: 运行已编译的 exe
.\bin\Debug\net8.0-windows\Wox.exe 34982 0 true
```

## 验证功能

启动后，检查以下功能是否正常：

### 窗口显示

- [ ] 窗口正常显示（无边框、圆角、阴影）
- [ ] 窗口置顶（始终在最前面）
- [ ] 搜索框自动聚焦

### 输入和查询

- [ ] 在搜索框输入文字
- [ ] 输入后会发送查询到 Core（查看控制台输出）
- [ ] 结果列表会更新

### 键盘操作

- [ ] `Down` 键：向下移动选中项
- [ ] `Up` 键：向上移动选中项
- [ ] `Enter` 键：执行选中的结果
- [ ] `Escape` 键：隐藏窗口

### 鼠标操作

- [ ] 点击列表项可执行
- [ ] 拖拽窗口可移动
- [ ] 点击窗口外会隐藏窗口

### WebSocket 通信

- [ ] 控制台显示 WebSocket 已连接
- [ ] 发送查询后有响应
- [ ] 接收 PushResults 消息

## 常见问题排查

### 问题 1: 编译失败

```
Error: The SDK 'Microsoft.NET.Sdk' specified could not be found
```

**解决**: 安装 .NET 8 SDK

### 问题 2: 运行时错误

```
Unhandled exception. System.Net.WebSockets.WebSocketException
```

**解决**:

- 检查 wox.core 是否运行
- 检查端口号是否正确
- 查看防火墙设置

### 问题 3: 窗口不显示

```
No window appears
```

**解决**:

- 检查是否有异常输出
- 尝试设置 `Topmost="False"` 调试
- 检查显示器配置

### 问题 4: 结果列表为空

```
Results list is empty
```

**解决**:

- 检查 WebSocket 连接状态
- 查看控制台是否收到 PushResults
- 检查 ResultsReceived 事件处理

## 调试技巧

### 启用详细日志

在 `App.xaml.cs` 中添加：

```csharp
Console.WriteLine($"Port: {_serverPort}");
Console.WriteLine($"SessionId: {_sessionId}");
```

### 查看 WebSocket 消息

在 `WoxApiService.HandleWebSocketMessage` 中添加：

```csharp
Console.WriteLine($"Received: {message}");
```

### 断点位置

- `App.xaml.cs` - `OnStartup`
- `WoxApiService.cs` - `ConnectAsync`
- `MainViewModel.cs` - `OnResultsReceived`

## 性能检查

- [ ] 输入响应流畅（无明显延迟）
- [ ] 结果列表更新迅速
- [ ] 内存占用合理（< 100MB）
- [ ] CPU 占用低（待机时 < 1%）

## 发布检查

```powershell
# 发布 Release 版本
dotnet publish --configuration Release --output ./publish

# 检查发布文件
ls publish/

# 预期文件大小
# Wox.exe: ~200KB
# 总大小: ~50MB（包含依赖）
```

## 集成检查

- [ ] 可以从 wox.core 启动
- [ ] 接收正确的命令行参数
- [ ] WebSocket 自动连接
- [ ] UI 就绪通知成功发送
- [ ] 与 Flutter UI 行为一致

## 提交前检查

- [ ] 代码已格式化
- [ ] 无编译警告
- [ ] 无明显的性能问题
- [ ] 文档已更新
- [ ] 已测试主要功能

## 测试环境

- Windows 版本: ****\_\_\_****
- .NET 版本: ****\_\_\_****
- 测试日期: ****\_\_\_****
- 测试人员: ****\_\_\_****

## 测试结果

- [ ] ✅ 通过所有检查
- [ ] ⚠️ 部分功能有问题（请在下方说明）
- [ ] ❌ 无法运行（请提供错误信息）

### 备注

```
（在此填写测试备注、问题描述等）
```

---

**提示**: 如果所有检查都通过，项目已准备好集成到 Wox 主项目！
