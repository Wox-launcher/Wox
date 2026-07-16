# 开发环境搭建

本文面向想在本地构建、运行和调试 Wox 的贡献者。

## 你要搭起来的是什么

这个仓库里通常一起协作的部分主要有四块：

- `wox.core/`：Go 后端、内置插件、设置、存储、打包入口
- `wox.ui.go/`：macOS / Windows / Linux 的 Go UI 桌面 UI
- `wox.plugin.host.nodejs/`：Node.js 插件宿主
- `wox.plugin.host.python/`：Python 插件宿主

顶层 `Makefile` 已经把这些部分串起来了。大多数情况下，应该从仓库根目录开始，而不是分别手动跑每个子项目。

## 前置依赖

先安装这些工具：

- [Go](https://go.dev/dl/)
- [Node.js](https://nodejs.org/)
- [pnpm](https://pnpm.io/)
- [uv](https://github.com/astral-sh/uv)

推荐编辑器：

- [Visual Studio Code](https://code.visualstudio.com/)，仓库里已经带了工作区配置

## 平台额外要求

### macOS

- 如果你要打 `.dmg` 包，需要安装 [create-dmg](https://github.com/create-dmg/create-dmg)

### Windows

- 请在 `MINGW64` shell 中运行构建命令
- 安装 [MinGW-w64](https://www.mingw-w64.org/)，这样 Windows 原生 runner 代码才能正常编译
- 安装 [NuGet CLI](https://www.nuget.org/downloads)，用于准备固定版本的 `WebView2Loader.dll`

### Linux

- 安装 `patchelf`
- 安装 `appimagetool`，或者把 `APPIMAGE_TOOL` 指向本地二进制路径

## 初始化开发环境

在仓库根目录运行：

```bash
make dev
```

它会做这些事情：

- 检查工具链依赖是否齐全
- 准备 `go:embed` 需要的资源目录
- 构建 `wox.core` 里的 `woxmr`
- 构建两个插件宿主

`make dev` 会准备嵌入式 Go UI 的原生资源和共享运行时。需要生成可运行的单文件 Wox 程序和平台安装包时，再执行完整构建。

## 常用命令

都在仓库根目录执行：

```bash
make dev
make test
make build
```

它们分别表示：

- `make dev`：准备本地开发环境
- `make test`：运行 `wox.core/test` 下的 Go 测试
- `make build`：把 Go UI 编译进 `wox.core`，并构建插件宿主和平台打包产物

如果你改的是后端和 UI、宿主之间的共享协议，最后一定要跑一次 `make build`，这是最容易暴露跨项目不一致的检查。

## 按模块开发时常用的命令

### Go 后端（`wox.core`）

适合处理：

- 插件运行时和元数据
- 内置插件逻辑
- 设置、存储、路由、打包

常用命令：

```bash
make -C wox.core build
```

### Go UI（`wox.ui.go`）

适合处理：

- 启动器界面
- 设置页
- 截图流程
- webview 和预览相关渲染

常用命令：

```bash
cd wox.ui.go && go test ./...
```

### 插件宿主

常用命令：

```bash
make -C wox.plugin.host.nodejs build
make -C wox.plugin.host.python build
```

如果你只改了宿主运行时行为，这两个命令通常比整仓 `make build` 更快。

## 本地预览文档站

文档站源码在 `www/docs`。本地预览：

```bash
cd www
pnpm install
pnpm docs:dev
```

生成生产构建：

```bash
cd www
pnpm docs:build
```

## Wox 本地数据目录

Wox 会把运行时数据存到用户主目录下：

- macOS / Linux：`~/.wox`
- Windows：`C:\Users\<username>\.wox`

常用子目录：

- `~/.wox/log/wox.log`：core 日志
- `~/.wox/log/ui.log`：UI 日志
- `~/.wox/plugins/`：本地插件开发目录

## 排错建议

如果 `make dev` 一开始就失败：

- 先确认 `go`、`node`、`pnpm`、`uv` 都在 `PATH` 里
- Windows 上还要确认 `nuget` 在 `PATH` 里
- Windows 上确认你在 `MINGW64` shell，而不是 PowerShell 或 CMD
- Linux 打包时确认 `patchelf` 和 `appimagetool` 已安装

如果某个子项目单独能编译，但 Wox 整体还是跑不起来，回到仓库根目录执行 `make build`。这是发现 `wox.core`、Go UI、插件宿主之间契约漂移的最快办法。
