# 单文件 SDK 插件

单文件 SDK 插件是一个 `.py` 或 CommonJS `.js` 文件，加载到 Wox 现有的 Python / Node.js runtime host 中。它拥有普通 SDK 插件的全部 Public API，但不需要 `.wox` 包，也不会为每次 query 启动新进程。

## 和其它类型的区别

```text
Script Plugin
  = 单文件 + 每次调用启动进程 + stdin/stdout JSON-RPC

Single-file SDK Plugin
  = 单文件 + 常驻 SDK runtime host + 完整 Public API

SDK Plugin
  = .wox 包 + 常驻 SDK runtime host + 完整 Public API
```

| 需求 | 选择 |
|---|---|
| 一次性 shell / 命令包装 | 脚本插件 |
| 只要一个文件，但需要 Wox API | 单文件 SDK 插件 |
| 需要依赖、资源、TypeScript 或多文件 | SDK 插件 |

脚本插件不是这个功能的 v1，也不会被废弃。

单文件 SDK 插件的 query/action 复用 host 进程，Wox 不会为插件单独创建 Python 或 Node 进程。host 崩溃恢复继续使用现有 watchdog。

## 快速开始

使用 `wpm create <name>`，选择 **Python 单文件 SDK 插件** 或 **Node.js 单文件 SDK 插件**。WPM 会写入：

```text
~/.wox/wox-user/plugins/single-file/Wox.Plugin.<Name>.py
~/.wox/wox-user/plugins/single-file/Wox.Plugin.<Name>.js
```

然后打开该文件。保存后会自动 reload。

这也是 WPM 直接创建 Python SDK 插件的路径。除非你需要额外文件或 pip 依赖，否则不必去克隆打包版 Python 模板。

### Python

Python 文件的 Runtime 是 `PYTHON`。可以直接 `import wox_plugin`，使用 SDK 模型、helper 和全部 Public API。

```python
# {
#   "Id": "com.example.weather",
#   "Name": "Weather",
#   "Author": "Example",
#   "Version": "1.0.0",
#   "MinWoxVersion": "2.4.2",
#   "Runtime": "PYTHON",
#   "Description": "Show current weather",
#   "Icon": "emoji:🌤️",
#   "TriggerKeywords": ["weather"],
#   "SupportedOS": ["Windows", "Linux", "Macos"]
# }

from wox_plugin import PluginInitParams, Query, QueryResponse, Result, WoxImage

class WeatherPlugin:
    async def init(self, ctx, params: PluginInitParams):
        self.api = params.api

    async def query(self, ctx, query: Query):
        return QueryResponse(results=[
            Result(
                title="Weather",
                icon=WoxImage.new_emoji("🌤️"),
            )
        ])

plugin = WeatherPlugin()
```

### Node.js

Node.js 文件的 Runtime 是 `NODEJS`。第一版固定为 CommonJS：

- 扩展名 `.js`
- 使用 `module.exports.plugin`
- 通过 `params.API` 使用全部 Public API
- 不要 `import` / `require` `@wox-launcher/wox-plugin`
- `WoxImage` 等简单类型使用对象字面量

第一版不支持 `.mjs`、ESM、TypeScript、npm 依赖和 SDK npm helper。当前 Node host 会把动态 `import()` 编译成 `require()`，保持 CommonJS 才能复用现有加载和 `require.cache` 热重载。

```js
// {
//   "Id": "com.example.weather",
//   "Name": "Weather",
//   "Author": "Example",
//   "Version": "1.0.0",
//   "MinWoxVersion": "2.4.2",
//   "Runtime": "NODEJS",
//   "Description": "Show current weather",
//   "Icon": "emoji:🌤️",
//   "TriggerKeywords": ["weather"],
//   "SupportedOS": ["Windows", "Linux", "Macos"]
// }

class WeatherPlugin {
  async init(ctx, params) {
    this.api = params.API
  }

  async query(ctx, query) {
    return {
      Results: [{
        Title: "Weather",
        Icon: {
          ImageType: "emoji",
          ImageData: "🌤️"
        },
        Actions: []
      }]
    }
  }
}

module.exports.plugin = new WeatherPlugin()
```

## Metadata

在文件开头的 `#` 或 `//` 注释里放一个 JSON 对象。允许第一行 shebang。

必须显式提供：

- `Id`
- `Name`
- `Version`
- `MinWoxVersion`
- `Runtime`
- `TriggerKeywords`

也支持：`Author`、`Description`、`Icon`、`Website`、`Commands`、`SupportedOS`、`Features`、`Glances`、`SettingDefinitions`、`QueryRequirements`、`I18n`。

Wox 会把 `Entry` 设为当前文件名，把 `Directory` 设为 `plugins/single-file`。文件头不允许声明 `Entry` 或 `Directory`。

Runtime 必须和后缀匹配，不会猜测或降级：

| 文件 | 合法 Runtime |
|---|---|
| `.py` | `PYTHON` |
| `.js` | `NODEJS` |

不要把 `PYTHON` / `NODEJS` 文件放进 `plugins/scripts/`。未声明 Runtime 的脚本插件仍按 `SCRIPT` 加载。如果在 scripts 目录里显式声明 `PYTHON`/`NODEJS`，会被拒绝，并提示移到 `plugins/single-file/`。

## 保存后 reload

保存文件后大约 500ms 防抖，然后 reload。

- 每次加载或 reload 都会调用一次 `init()`。
- query/action 之间会保留插件对象状态。
- 保存后内存状态重置。不提供状态迁移，也不对旧实例做事务式回退。
- reload 会先走现有 unload 路径，因此 timer、watcher、socket 等可以通过 unload callback 清理。
- metadata 永久无效时记录错误并保留旧实例。
- 删除文件会卸载对应插件。重命名会卸载旧路径，并等待新路径的 Create。

只要创建了后台工作，就应该注册 unload callback。

## 共享目录

所有单文件插件都在 `plugins/single-file/`。不会加载共享的 `lang/` 目录，只支持文件头里的内联 `I18n`。

metadata 图标不能使用相对路径。动态结果也不要使用 relative image。支持：

- emoji
- URL
- SVG
- base64
- 绝对路径

需要自带资源文件时，应升级为普通 `.wox` 插件。设置页和 WPM 中的“打开插件目录”会改为定位插件文件，避免只打开混有多个插件的共享目录。

## 商店发布

商店不增加新字段。Wox 根据 `Runtime` 和 URL path 后缀分类（query 不参与判断）：

| Runtime | URL 后缀 | 类型 |
|---|---|---|
| `PYTHON` | `.py` | 单文件 SDK 插件 |
| `NODEJS` | `.js` | 单文件 SDK 插件 |
| `PYTHON` / `NODEJS` | `.wox` | 普通 SDK 插件 |
| `SCRIPT` | 脚本文件 | 脚本插件 |

未知后缀、无后缀，以及 `PYTHON` + `.js` 这类组合都会被拒绝。更新时不允许同一个插件 ID 在 `.wox` 和单文件之间改变交付形态。

单文件商店条目必须显式声明 `MinWoxVersion`，且不低于该功能的首发版本。文件头的 `Id`、`Runtime`、`Version` 必须和商店 manifest 一致。卸载只会把对应单文件移到回收站。
