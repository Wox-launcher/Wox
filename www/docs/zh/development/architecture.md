# Wox 架构

本文档提供了 Wox 项目架构的概览，解释了不同组件之间如何交互。

## 概览

Wox 是一个基于微服务架构构建的跨平台启动器。应用程序由几个关键组件组成：

- **wox.core**: 处理核心功能的 Go 后端
- **wox.ui.flutter**: 提供用户界面的 Flutter 前端
- **wox.plugin.host.python**: Python 插件宿主
- **wox.plugin.host.nodejs**: NodeJS 插件宿主
- **wox.plugin.python**: Python 插件库
- **wox.plugin.nodejs**: NodeJS 插件库

## 组件交互

```
┌─────────────────┐           ┌─────────────────┐
│                 │           │                 │
│  wox.ui.flutter │◄─────────►│    wox.core     │
│  (Flutter UI)   │  WebSocket│   (Go Backend)  │
│                 │    & HTTP │                 │
└─────────────────┘           └────────┬────────┘
                                       │
                                       │ WebSocket
                                       │
                              ┌────────▼────────┐
                              │                 │
                              │  Plugin Hosts   │
                              │                 │
                              └────────┬────────┘
                                       │
                                       │
                              ┌────────▼────────┐
                              │                 │
                              │    Plugins      │
                              │                 │
                              └─────────────────┘
```

### 通信流程

1. **UI 到 Core**: Flutter UI 通过 WebSocket 和 HTTP 与 Go 后端通信
2. **Core 到 Plugin Hosts**: Go 后端通过 WebSocket 与插件宿主通信
3. **Plugin Hosts 到 Plugins**: 插件宿主加载并与插件通信

## 关键组件详解

### wox.core

作为应用程序中心组件的 Go 后端。它处理：

- 用户查询和搜索功能
- 插件管理
- 设置管理
- 与 UI 和插件宿主的通信

关键目录：

- `wox.core/setting`: 包含设置相关的定义
- `wox.core/plugin`: 包含 API 定义和实现

### wox.ui.flutter

基于 Flutter 的用户界面，提供：

- 搜索界面
- 结果显示
- 设置管理
- 主题自定义

### 插件系统

Wox 支持多种语言编写的插件：

- **Python 插件**: 由 `wox.plugin.host.python` 管理
- **NodeJS 插件**: 由 `wox.plugin.host.nodejs` 管理

插件宿主负责：

- 加载插件
- 执行插件代码
- 将结果传回核心

## 开发工作流

Wox 的开发工作流通过 Makefile 管理：

1. `make dev`: 设置开发环境
2. `make test`: 运行测试
3. `make publish`: 构建并发布所有组件
4. `make plugins`: 更新插件商店

## 平台特定注意事项

Wox 设计为跨平台，具体注意事项如下：

- **Windows**: 使用 `make publish` 生成标准构建产物（不再使用 UPX 压缩）
- **macOS**: 使用 create-dmg 对应用进行打包
- **Linux**: 使用 `make publish` 生成标准构建产物（不再使用 UPX 压缩）

## 数据流

1. 用户在 UI 中输入查询
2. 查询通过 WebSocket 发送到核心
3. 核心处理查询并确定要调用的插件
4. 核心向相应的插件宿主发送请求
5. 插件宿主执行插件代码并返回结果
6. 核心聚合结果并将其发送回 UI
7. UI 向用户显示结果

## 配置和数据存储

所有用户数据，包括设置和插件数据，都存储在用户主目录下的 `.wox` 目录中：

- Windows: `C:\Users\<username>\.wox`
- macOS/Linux: `~/.wox`

## 日志

日志存储在 `.wox/log` 目录中，可用于调试目的。
