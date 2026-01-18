# Wox.UI.Windows 集成指南

本文档说明如何将 WPF UI 集成到 Wox 主项目的构建系统中。

## 1. 项目结构

WPF UI 位于 `wox.ui.windows/` 目录，与其他 UI 实现（Flutter、macOS）并列：

```
Wox/
├── wox.core/              # Go 后端核心
├── wox.ui.flutter/        # Flutter UI（跨平台）
├── wox.ui.macos/          # macOS 原生 UI
├── wox.ui.windows/        # Windows WPF UI（新）
├── wox.ui.windows/        # Windows UWP UI（旧）
└── ...
```

## 2. 构建系统集成

### 2.1 在根 Makefile 中添加 WPF UI 构建目标

编辑 `Wox/Makefile`，添加以下内容：

```makefile
# Windows WPF UI build targets
.PHONY: build-ui-wpf clean-ui-wpf restore-ui-wpf

build-ui-wpf: restore-ui-wpf
	@echo "Building Windows WPF UI..."
	cd wox.ui.windows && dotnet build --configuration Release

restore-ui-wpf:
	@echo "Restoring Windows WPF UI dependencies..."
	cd wox.ui.windows && dotnet restore

clean-ui-wpf:
	@echo "Cleaning Windows WPF UI..."
	cd wox.ui.windows && dotnet clean
	cd wox.ui.windows && rm -rf bin obj

publish-ui-wpf: build-ui-wpf
	@echo "Publishing Windows WPF UI..."
	cd wox.ui.windows && dotnet publish --configuration Release --output ../wox.core/resource/ui/wpf --self-contained false
```

### 2.2 集成到主构建流程

修改主 `build` 目标以包含 WPF UI：

```makefile
build: build-core build-ui-flutter build-ui-wpf
	@echo "Build completed"
```

### 2.3 Windows 特定构建

添加 Windows 平台专用构建：

```makefile
build-windows: build-core build-ui-wpf
	@echo "Windows build completed"
```

## 3. 运行时集成

### 3.1 启动逻辑

在 `wox.core/ui/manager.go` 中添加 WPF UI 启动逻辑：

```go
func (m *Manager) startWPFUI(ctx context.Context) error {
    uiPath := filepath.Join(util.GetLocation().GetUIDirectory(), "wpf", "Wox.exe")

    if !util.IsFileExists(uiPath) {
        return fmt.Errorf("WPF UI not found: %s", uiPath)
    }

    cmd := exec.Command(
        uiPath,
        strconv.Itoa(m.serverPort),
        strconv.Itoa(os.Getpid()),
        strconv.FormatBool(util.IsDev()),
    )

    if err := cmd.Start(); err != nil {
        return fmt.Errorf("failed to start WPF UI: %w", err)
    }

    m.uiProcess = cmd.Process
    return nil
}
```

### 3.2 UI 选择配置

在 `wox.core/setting/setting.go` 中添加 UI 类型配置：

```go
type WoxSetting struct {
    // ... existing fields
    UIType SettingDefinition `json:"UIType"` // "flutter" | "wpf" | "macos"
}

func GetDefaultWoxSetting() WoxSetting {
    return WoxSetting{
        // ... existing defaults
        UIType: NewSettingDefinition("flutter", SettingTypeString),
    }
}
```

### 3.3 条件启动

根据配置选择启动哪个 UI：

```go
func (m *Manager) StartUI(ctx context.Context) error {
    woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
    uiType := woxSetting.UIType.GetString()

    switch uiType {
    case "wpf":
        return m.startWPFUI(ctx)
    case "flutter":
        return m.startFlutterUI(ctx)
    case "macos":
        return m.startMacOSUI(ctx)
    default:
        // Default to Flutter
        return m.startFlutterUI(ctx)
    }
}
```

## 4. 发布打包

### 4.1 发布脚本

创建 Windows 发布脚本 `ci/build-windows.ps1`：

```powershell
# Build Wox for Windows with WPF UI

# Build Go core
Push-Location wox.core
go build -ldflags="-s -w" -o ../release/Wox.exe
Pop-Location

# Publish WPF UI
Push-Location wox.ui.windows
dotnet publish --configuration Release `
    --output ../release/ui/wpf `
    --self-contained false `
    --runtime win-x64
Pop-Location

# Copy resources
Copy-Item -Recurse wox.core/resource/* release/

Write-Host "Windows build completed: release/"
```

### 4.2 打包成安装程序

使用 WiX Toolset 或 Inno Setup 创建安装程序：

```iss
; Wox Installer Script (Inno Setup)
[Setup]
AppName=Wox
AppVersion=2.0.0
DefaultDirName={pf}\Wox
OutputDir=release
OutputBaseFilename=WoxSetup

[Files]
Source: "release\Wox.exe"; DestDir: "{app}"
Source: "release\ui\wpf\*"; DestDir: "{app}\ui\wpf"; Flags: recursesubdirs

[Run]
Filename: "{app}\Wox.exe"; Description: "Launch Wox"; Flags: postinstall nowait
```

## 5. 测试和调试

### 5.1 独立测试 WPF UI

```powershell
# 1. 启动 wox.core（开发模式）
cd wox.core
go run . --dev

# 2. 记下端口号（例如 34982）

# 3. 启动 WPF UI（手动传参）
cd wox.ui.windows
dotnet run 34982 0 true
```

### 5.2 集成测试

```powershell
# 完整构建和运行
make build-windows
cd release
./Wox.exe
```

## 6. CI/CD 集成

### 6.1 GitHub Actions

在 `.github/workflows/build-windows.yml` 中添加：

```yaml
name: Build Windows

on:
  push:
    branches: [main, dev]
  pull_request:

jobs:
  build:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v3

      - name: Setup .NET
        uses: actions/setup-dotnet@v3
        with:
          dotnet-version: "8.0.x"

      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: "1.21"

      - name: Build Core
        run: |
          cd wox.core
          go build

      - name: Build WPF UI
        run: |
          cd wox.ui.windows
          dotnet build --configuration Release

      - name: Run Tests
        run: |
          cd wox.core
          go test ./...

      - name: Upload Artifacts
        uses: actions/upload-artifact@v3
        with:
          name: wox-windows
          path: release/
```

## 7. 用户配置

用户可在设置中选择 UI 类型：

**设置界面**（在 Flutter/WPF 设置中添加）：

```json
{
  "key": "UIType",
  "type": "select",
  "label": "UI Type",
  "options": [
    { "value": "flutter", "label": "Flutter (Cross-platform)" },
    { "value": "wpf", "label": "WPF (Windows Native)" }
  ],
  "default": "flutter"
}
```

**命令行参数**：

```bash
Wox.exe --ui=wpf
```

## 8. 性能优化

### 8.1 AOT 编译（.NET 8+）

在 `.csproj` 中启用：

```xml
<PropertyGroup>
  <PublishAot>true</PublishAot>
  <SelfContained>true</SelfContained>
</PropertyGroup>
```

### 8.2 Trimming

```xml
<PropertyGroup>
  <PublishTrimmed>true</PublishTrimmed>
  <TrimMode>link</TrimMode>
</PropertyGroup>
```

## 9. 常见问题

### Q1: WPF UI 无法启动

- 检查是否安装了 .NET 8 Runtime
- 检查 UI 文件路径是否正确
- 查看 wox.core 日志

### Q2: WebSocket 连接失败

- 确认 wox.core 已启动且监听正确端口
- 检查防火墙设置
- 查看 WPF UI 控制台输出

### Q3: 主题不生效

- 确认 ThemeService 正确解析了主题 JSON
- 检查资源字典键名是否匹配
- 重启应用程序

## 10. 后续优化

- [ ] 添加崩溃报告（Sentry/AppCenter）
- [ ] 实现自动更新机制
- [ ] 优化启动性能
- [ ] 添加更多动画效果
- [ ] 支持插件 UI 扩展
- [ ] 完善无障碍功能

## 相关文档

- [DEVELOPMENT.md](DEVELOPMENT.md) - 开发指南
- [README.md](README.md) - 项目概览
- [../../AGENTS.md](../../AGENTS.md) - Wox 整体架构
