# Wox 架构

这篇文档解释 Wox 在仓库里的拆分方式，以及运行时数据在各层之间如何流动。

## 整体结构

Wox 是一个桌面启动器，Go UI 与 core 编译并运行在同一个进程里。第三方插件不会直接运行在主进程中，而是运行在独立的语言宿主中，再通过基于 WebSocket 的 JSON-RPC 和 core 通信。

从大结构看：

```text
Go UI + wox.core  <->  插件宿主  <->  插件
```

## 主要组件

### `wox.core`

`wox.core` 是 Wox 的运行时中心，负责：

- 查询路由
- 内置插件执行
- 第三方插件生命周期和元数据加载
- 设置与数据存储
- 进程内 UI 契约，以及主实例的最小 loopback 控制接口
- 最终桌面运行时资源的打包

建议优先熟悉这些目录：

- `wox.core/plugin/`：插件协议、管理器、查询/结果模型、宿主桥接
- `wox.core/common/`：共享 UI 载荷和通用运行时类型
- `wox.core/setting/`：设置定义与持久化
- `wox.core/appcontrol/`：仅包含 `/ping`、`/show`、deeplink 和诊断重启控制
- `wox.core/ui/`：launcher、retained widget、自动化契约与原生平台 runtime
- `wox.core/resource/`：嵌入式 UI、宿主二进制、翻译和其他运行时资源

### `wox.core/ui`

这是用户真正看到的桌面 UI，负责渲染：

- 启动器窗口
- 结果列表和操作面板
- 设置页面
- 截图流程
- webview 预览和相关原生桥接

Go UI 与 `wox.core` 位于同一个 Go module，并运行在同一进程。它不负责执行插件，重点是渲染状态、回传用户操作，并承载平台相关展示逻辑。

生命周期、查询/操作执行、terminal 订阅以及全部 core→UI 更新都通过类型化的 `Services`/`View` 接口。设置和目录类页面尚有一个过渡期的进程内适配器复用旧 router，后续会按领域迁移；该适配器不会监听任何端口。生产环境唯一的 loopback HTTP 表面是 `appcontrol`，测试自动化只在 `wox_automation` build tag 下编译。

### `wox.plugin.host.nodejs` 与 `wox.plugin.host.python`

这是全功能插件的长期运行宿主进程，负责：

- 启动对应语言运行时
- 从 `~/.wox/plugins` 加载插件代码
- 向插件作者暴露公共 API
- 把插件请求和回调继续转发给 `wox.core`

如果插件 API 形状改了，这一层和 core、SDK 一起对齐很关键。

### `wox.plugin.nodejs` 与 `wox.plugin.python`

这是第三方插件作者使用的 SDK，提供：

- 类型化的查询/结果模型
- 公共 API 包装
- 插件启动辅助能力

## 运行时数据流

### 1. 查询处理

用户在 Wox 输入查询后：

1. Go UI 调用 `wox.core` 的类型化查询服务
2. `wox.core` 判断应该触发哪些内置插件和第三方插件
3. 内置插件直接在 Go 内执行
4. 第三方插件通过对应语言宿主被调用
5. 结果被聚合后返回给 UI
6. UI 渲染结果列表、预览、尾部信息和操作

### 2. 操作执行

用户触发某个结果操作后：

1. UI 把操作上下文发给 `wox.core`
2. `wox.core` 判断这个操作属于内置插件还是宿主插件
3. 在正确的运行时里执行
4. 之后可以通过 `UpdateResult`、`PushResults`、`RefreshQuery`、`Notify`、`HideApp` 等 API 做后续 UI 更新

### 3. 插件发起的 UI 流程

有些流程不是从启动器 UI 主动开始，而是由插件发起，例如：

- toolbar message
- deep link
- 截图
- 剪贴板复制
- AI 流式返回

这类流程通常是：插件调用 SDK API，宿主转发到 `wox.core`，再由 core 协调 UI 或原生平台行为。

## 为什么边界很重要

很多问题一开始看起来像 UI bug，实际可能是协议或运行时边界问题。一个简单判断方法：

- 查询路由、插件元数据、设置持久化、运行时契约问题，先看 `wox.core`
- 视觉表现、交互、输入处理问题，先看 `wox.core/ui`
- 同一套插件 API 在不同语言 SDK 表现不一致，就一起看宿主和 SDK

## 仓库级工作流

跨项目开发优先使用顶层 `Makefile`：

- `make dev`：准备共享资源并构建插件宿主
- `make test`：运行 `wox.core/test` 下的 Go 测试
- `make test-go-ui-unit`：运行 retained widget 与自动化契约测试
- `make test-go-ui-smoke`：运行认证的原生 launcher smoke 与视觉 golden
- `make build`：构建完整应用和打包产物

只要改动涉及共享契约，`make build` 都应该作为最后检查。

## 运行时数据与日志

Wox 会把运行时数据存到用户主目录：

- macOS / Linux：`~/.wox`
- Windows：`C:\Users\<username>\.wox`

常用路径：

- `~/.wox/plugins/`：本地第三方插件目录
- `~/.wox/log/`：运行日志

调试插件或 UI 问题时，先看日志，再从失败的真实边界往回追，效率通常最高。
