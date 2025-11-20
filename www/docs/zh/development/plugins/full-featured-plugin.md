# 全功能插件开发指南

全功能插件是综合性插件，在专用宿主进程中运行，并通过 WebSocket 与 Wox 通信。它们通过丰富的 API、持久状态和高级功能提供 Wox 插件系统的全部能力。

## 概览

全功能插件专为需要以下功能的复杂应用程序设计：

- 持久状态管理
- 高性能查询处理
- 高级 Wox API 集成（AI、设置、预览）
- 复杂的异步操作
- 专业级插件开发

## 支持语言

### Python 插件

**要求：**

- Python >= 3.8 (推荐 Python 3.12)
- `wox-plugin` SDK

**安装：**

```bash
# Using pip
pip install wox-plugin

# Using uv (recommended)
uv add wox-plugin
```

**基本结构：**

```python
from wox_plugin import Plugin, Query, Result, Context, PluginInitParams

class MyPlugin(Plugin):
    async def init(self, ctx: Context, params: PluginInitParams) -> None:
        self.api = params.api
        self.plugin_dir = params.plugin_directory

    async def query(self, ctx: Context, query: Query) -> list[Result]:
        # Your plugin logic here
        results = []
        results.append(
            Result(
                title="Hello Wox",
                subtitle="This is a sample result",
                icon="path/to/icon.png",
                score=100
            )
        )
        return results

# MUST HAVE! The plugin class will be automatically loaded by Wox
plugin = MyPlugin()
```

### Node.js 插件

**要求：**

- Node.js >= 16
- `@wox-launcher/wox-plugin` SDK

**安装：**

```bash
npm install @wox-launcher/wox-plugin
# or
pnpm add @wox-launcher/wox-plugin
```

**基本结构：**

```javascript
import { Plugin, Query, Result, Context, PluginInitParams } from '@wox-launcher/wox-plugin'

class MyPlugin implements Plugin {
  private api: any
  private pluginDir: string

  async init(ctx: Context, params: PluginInitParams): Promise<void> {
    this.api = params.API
    this.pluginDir = params.PluginDirectory
  }

  async query(ctx: Context, query: Query): Promise<Result[]> {
    // Your plugin logic here
    return [
      {
        Title: "Hello Wox",
        SubTitle: "This is a sample result",
        Icon: "path/to/icon.png",
        Score: 100
      }
    ]
  }
}

// MUST HAVE! Export the plugin instance
export const plugin = new MyPlugin()
```

## 插件架构

### 插件宿主系统

全功能插件在专用宿主进程中运行：

- **Python Host**: `wox.plugin.host.python` 管理 Python 插件
- **Node.js Host**: `wox.plugin.host.nodejs` 管理 Node.js 插件

### 通信流程

1. **Wox Core** ↔ **Plugin Host** (WebSocket)
2. **Plugin Host** ↔ **Plugin Instance** (直接方法调用)

### 插件生命周期

1. **Load**: 插件宿主加载插件模块
2. **Init**: 插件初始化，获取 API 访问权限
3. **Query**: 处理用户查询（多次）
4. **Unload**: 禁用/卸载时清理插件

## 丰富 API 功能

### 核心 API

```python
# Change query
await self.api.change_query(ctx, ChangeQueryParam(query="new query"))

# Show/hide app
await self.api.show_app(ctx)
await self.api.hide_app(ctx)

# Notifications
await self.api.notify(ctx, "Hello from plugin!")

# Logging
await self.api.log(ctx, "info", "Plugin message")

# Settings
value = await self.api.get_setting(ctx, "setting_key")

# Translations
text = await self.api.get_translation(ctx, "i18n_key")
```

### AI 集成

```python
# Chat with AI
conversation = [
    ai_message("You are a helpful assistant"),
    user_message("What is 2+2?")
]

async def stream_callback(data):
    # Handle streaming response
    print(data.content)

response = await self.api.ai_chat(ctx, conversation, stream_callback)
```

### 高级结果功能

```python
# Result with preview
Result(
    title="Document",
    subtitle="Click to preview",
    preview=WoxPreview(
        preview_type=WoxPreviewType.TEXT,
        preview_data="Document content here..."
    )
)

# Result with custom actions
Result(
    title="File",
    subtitle="Multiple actions available",
    actions=[
        ResultAction(name="Open", action=self.open_file),
        ResultAction(name="Delete", action=self.delete_file)
    ]
)
```

## 插件配置

### plugin.json

```json
{
  "Id": "my-awesome-plugin",
  "Name": "My Awesome Plugin",
  "Author": "Your Name",
  "Version": "1.0.0",
  "MinWoxVersion": "2.0.0",
  "Runtime": "Python",
  "Entry": "main.py",
  "TriggerKeywords": ["awesome", "ap"],
  "Commands": [
    {
      "Command": "config",
      "Description": "Configure plugin settings"
    }
  ],
  "Settings": [
    {
      "Type": "textbox",
      "Key": "api_key",
      "Title": "API Key",
      "Description": "Enter your API key"
    }
  ]
}
```

### 设置定义

```python
from wox_plugin import PluginSettingDefinitionItem

settings = [
    PluginSettingDefinitionItem(
        type="textbox",
        key="api_key",
        title="API Key",
        description="Enter your API key",
        value=""
    ),
    PluginSettingDefinitionItem(
        type="checkbox",
        key="enable_cache",
        title="Enable Cache",
        description="Cache results for better performance",
        value="true"
    )
]
```

## 性能优化

### 异步操作

```python
import asyncio
import aiohttp

async def query(self, ctx: Context, query: Query) -> list[Result]:
    # Concurrent API calls
    async with aiohttp.ClientSession() as session:
        tasks = [
            self.fetch_data(session, url1),
            self.fetch_data(session, url2),
            self.fetch_data(session, url3)
        ]
        results = await asyncio.gather(*tasks)

    return self.process_results(results)
```

### 缓存

```python
from functools import lru_cache
import time

class MyPlugin(Plugin):
    def __init__(self):
        self.cache = {}
        self.cache_ttl = 300  # 5 minutes

    async def query(self, ctx: Context, query: Query) -> list[Result]:
        cache_key = f"query:{query.search}"

        # Check cache
        if cache_key in self.cache:
            data, timestamp = self.cache[cache_key]
            if time.time() - timestamp < self.cache_ttl:
                return data

        # Fetch new data
        results = await self.fetch_results(query.search)

        # Update cache
        self.cache[cache_key] = (results, time.time())

        return results
```

## 错误处理

```python
from wox_plugin import WoxPluginError, APIError

async def query(self, ctx: Context, query: Query) -> list[Result]:
    try:
        # Your plugin logic
        return await self.process_query(query)
    except APIError as e:
        await self.api.log(ctx, "error", f"API error: {e}")
        return [Result(
            title="API Error",
            subtitle="Please check your settings",
            score=0
        )]
    except Exception as e:
        await self.api.log(ctx, "error", f"Unexpected error: {e}")
        return []
```

## 测试

### 单元测试

```python
import pytest
from unittest.mock import AsyncMock
from your_plugin import MyPlugin

@pytest.mark.asyncio
async def test_query():
    plugin = MyPlugin()

    # Mock API
    mock_api = AsyncMock()
    await plugin.init(mock_context, PluginInitParams(
        api=mock_api,
        plugin_directory="/test"
    ))

    # Test query
    results = await plugin.query(mock_context, Query(search="test"))

    assert len(results) > 0
    assert results[0].title == "Expected Title"
```

## 最佳实践

1. **使用 Async/Await**：利用异步操作获得更好的性能
2. **优雅地处理错误**：始终捕获并记录异常
3. **实现缓存**：在适当的时候缓存昂贵的操作
4. **提供良好的 UX**：使用有意义的标题、副标题和图标
5. **遵循命名约定**：使用清晰、描述性的名称
6. **记录您的代码**：添加文档字符串和注释
7. **彻底测试**：为您的插件逻辑编写单元测试

## 从脚本插件迁移

如果您的脚本插件需要更多功能：

1. **创建插件结构**：使用 `plugin.json` 设置正确的插件目录
2. **安装 SDK**：添加适当的 SDK 依赖项
3. **转换逻辑**：将您的脚本逻辑移动到插件类
4. **添加状态管理**：如果需要，利用持久状态
5. **增强 API**：添加 AI、设置或其他高级功能
6. **优化性能**：实现缓存和异步操作

## 发布

1. **本地测试**：确保您的插件正常工作
2. **创建包**：遵循插件打包指南
3. **提交到商店**：使用插件提交流程
4. **维护**：保持插件更新并响应用户反馈
