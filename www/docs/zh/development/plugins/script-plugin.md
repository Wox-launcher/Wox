# 脚本插件开发指南

脚本插件是轻量级的单文件插件，提供了一种扩展 Wox 功能的简单方法。它们非常适合快速自动化任务、个人实用程序和学习插件开发。

## 概览

脚本插件使用 JSON-RPC 通过标准输入/输出 (stdin/stdout) 与 Wox 通信。每个脚本在进行查询时按需执行，这使得它们非常适合简单的无状态操作。

想要快速上手可以参考这个示例脚本：https://gist.github.com/qianlifeng/82a2f748177ce47a900b4c4da3abfd28

## 快速开始

### 创建脚本插件

使用 `wpm` 插件创建一个新的脚本插件：

```
wpm create <name>
```

可用模板：

- `python` - Python 脚本模板
- `javascript` - JavaScript/Node.js 脚本模板
- `bash` - Bash 脚本模板

### 脚本插件结构

脚本插件由单个可执行文件组成，元数据在注释中定义为 JSON 对象：

```python
#!/usr/bin/env python3
# {
#   "Id": "my-calculator",
#   "Name": "My Calculator",
#   "Author": "Your Name",
#   "Version": "1.0.0",
#   "MinWoxVersion": "2.0.0",
#   "Description": "A simple calculator plugin",
#   "Icon": "emoji:🧮",
#   "TriggerKeywords": ["calc"],
#   "SettingDefinitions": [
#     {
#       "Type": "textbox",
#       "Value": {
#         "Key": "precision",
#         "Label": "Decimal Precision",
#         "Tooltip": "Number of decimal places to show",
#         "DefaultValue": "2",
#         "Style": {
#           "Width": 100
#         }
#       }
#     }
#   ],
#   "Features": [
#     {
#       "Name": "debounce",
#       "Params": {
#         "intervalMs": "300"
#       }
#     }
#   ]
# }

# Your plugin code here...
```

JSON 元数据块必须：

- 放置在文件开头的注释中（在 shebang 行之后）
- Python/Bash 使用 `#`，JavaScript 使用 `//`
- 包含具有所有元数据字段的完整 JSON 对象

## 元数据字段

### 必填字段

- `Id` - 唯一插件标识符（建议使用 UUID 格式）
- `Name` - 插件显示名称
- `TriggerKeywords` - 触发关键字数组

### 可选字段

- `Icon` - 插件图标（emoji:🧮，relative:path/to/icon.png，或绝对路径）
- `Version` - 插件版本（默认："1.0.0"）
- `Author` - 插件作者（默认："Unknown"）
- `Description` - 插件描述（默认："A script plugin"）
- `MinWoxVersion` - 最低要求的 Wox 版本（默认："2.0.0"）
- `SettingDefinitions` - 设置定义数组（见下文设置部分）
- `Features` - 插件功能数组（debounce, querySelection 等）
- `Commands` - 插件命令数组
- `SupportedOS` - 支持的操作系统数组（默认：所有平台）

## 插件设置

您可以定义用户可以在 Wox 设置 UI 中配置的设置。设置在 `SettingDefinitions` 数组中定义：

```python
#!/usr/bin/env python3
# {
#   "Id": "weather-plugin",
#   "Name": "Weather",
#   "TriggerKeywords": ["weather"],
#   "SettingDefinitions": [
#     {
#       "Type": "textbox",
#       "Value": {
#         "Key": "api_key",
#         "Label": "API Key",
#         "Tooltip": "Your weather API key",
#         "DefaultValue": "",
#         "Style": {
#           "Width": 400
#         }
#       }
#     },
#     {
#       "Type": "select",
#       "Value": {
#         "Key": "units",
#         "Label": "Temperature Units",
#         "DefaultValue": "celsius",
#         "Options": [
#           {"Label": "Celsius", "Value": "celsius"},
#           {"Label": "Fahrenheit", "Value": "fahrenheit"}
#         ]
#       }
#     },
#     {
#       "Type": "checkbox",
#       "Value": {
#         "Key": "show_forecast",
#         "Label": "Show 7-day forecast",
#         "DefaultValue": "true"
#       }
#     }
#   ]
# }
```

### 支持的设置类型

- **textbox** - 单行或多行文本输入
- **checkbox** - 布尔复选框
- **select** - 下拉选择
- **label** - 仅显示文本标签
- **head** - 章节标题
- **newline** - 换行符
- **table** - 带有可编辑行的表格

### 在脚本中访问设置

设置会自动作为环境变量传递给脚本插件。每个设置都以 `WOX_SETTING_` 为前缀，并且键转换为大写。

例如，如果您定义了一个键为 `api_key` 的设置，它将作为环境变量 `WOX_SETTING_API_KEY` 可用。

**Python 示例：**

```python
import os

# Get setting value
api_key = os.getenv('WOX_SETTING_API_KEY', '')
enable_feature = os.getenv('WOX_SETTING_ENABLE_FEATURE', 'false')
output_format = os.getenv('WOX_SETTING_OUTPUT_FORMAT', 'json')

# Use the settings
if api_key:
    print(f"Using API key: {api_key[:4]}...")
```

**JavaScript 示例：**

```javascript
// Get setting value
const apiKey = process.env.WOX_SETTING_API_KEY || "";
const enableFeature = process.env.WOX_SETTING_ENABLE_FEATURE === "true";
const outputFormat = process.env.WOX_SETTING_OUTPUT_FORMAT || "json";

// Use the settings
if (apiKey) {
  console.log(`Using API key: ${apiKey.substring(0, 4)}...`);
}
```

**Bash 示例：**

```bash
# Get setting value
API_KEY="${WOX_SETTING_API_KEY:-}"
ENABLE_FEATURE="${WOX_SETTING_ENABLE_FEATURE:-false}"
OUTPUT_FORMAT="${WOX_SETTING_OUTPUT_FORMAT:-json}"

# Use the settings
if [ -n "$API_KEY" ]; then
    echo "Using API key: ${API_KEY:0:4}..."
fi
```

**其他环境变量：**

脚本插件还可以访问这些环境变量：

- `WOX_PLUGIN_ID` - 插件的唯一 ID
- `WOX_PLUGIN_NAME` - 插件的显示名称
- `WOX_DIRECTORY_USER_SCRIPT_PLUGINS` - 脚本插件存储目录
- `WOX_DIRECTORY_USER_DATA` - 用户数据目录
- `WOX_DIRECTORY_WOX_DATA` - Wox 应用程序数据目录
- `WOX_DIRECTORY_PLUGINS` - 插件目录
- `WOX_DIRECTORY_THEMES` - 主题目录

## JSON-RPC 通信

脚本插件使用 JSON-RPC 2.0 协议与 Wox 通信。

### 请求格式

Wox 通过 stdin 向您的脚本发送请求：

```json
{
  "jsonrpc": "2.0",
  "method": "query",
  "params": {
    "search": "user search term",
    "trigger_keyword": "calc",
    "command": "",
    "raw_query": "calc 2+2"
  },
  "id": "request-id"
}
```

### 响应格式

您的脚本应通过 stdout 响应：

```json
{
  "jsonrpc": "2.0",
  "result": {
    "items": [
      {
        "title": "Result: 4",
        "subtitle": "2 + 2 = 4",
        "score": 100,
        "actions": [
          {
            "id": "copy-result",
            "data": "4"
          }
        ]
      }
    ]
  },
  "id": "request-id"
}
```

## 可用方法

### query 方法

处理用户查询并返回搜索结果。

**参数：**

- `search` - 用户输入的搜索词
- `trigger_keyword` - 触发此插件的关键字
- `command` - 如果使用插件命令，则为命令
- `raw_query` - 完整的原始查询字符串

### action 方法

处理用户对结果项的选择。

**参数：**

- `id` - 结果项中的操作 ID
- `data` - 结果项中的操作数据

## 能力与限制

- `query` 仅收到 `search`、`trigger_keyword`、`command`、`raw_query`，不会包含 selection 或查询环境数据。
- 每次调用都会启动全新进程，超时 10 秒；如需复用请自行落盘缓存。
- 预览、tails、MRU 恢复、结果动态更新等功能仅在全功能插件中提供。

## 环境变量

脚本插件可以访问这些环境变量：

- `WOX_DIRECTORY_USER_SCRIPT_PLUGINS` - 脚本插件目录
- `WOX_DIRECTORY_USER_DATA` - 用户数据目录
- `WOX_DIRECTORY_WOX_DATA` - Wox 应用程序数据目录
- `WOX_DIRECTORY_PLUGINS` - 插件目录
- `WOX_DIRECTORY_THEMES` - 主题目录

## 操作 (Actions)

脚本插件可以使用两种类型的操作：

### 操作格式

每个结果必须有一个 `actions` 字段，其中包含操作对象数组（即使只有一个操作）。

每个操作对象可以有：

- `id` (必填): 操作标识符
- `name` (可选): UI 中的显示名称（默认为 "Execute"）
- 其他字段取决于操作类型（例如，`text` 用于剪贴板，`url` 用于打开 URL）

**示例 - 单个操作**:

```python
{
    "title": "Copy text",
    "actions": [
        {
            "name": "Copy to Clipboard",
            "id": "copy-to-clipboard",
            "text": "Hello World"
        }
    ]
}
```

**示例 - 多个操作**:

```python
{
    "title": "Multiple options",
    "actions": [
        {
            "name": "Copy",
            "id": "copy-to-clipboard",
            "text": "Hello World"
        },
        {
            "name": "Open URL",
            "id": "open-url",
            "url": "https://example.com"
        }
    ]
}
```

### 内置操作

内置操作由 Wox 自动处理。您可以直接在查询结果中使用它们，而无需在脚本中实现 `action` 方法。

**重要**：使用内置操作时，您不需要在脚本中实现 `handle_action()`。Wox 会自动处理该操作。`action` 方法仍会作为钩子被调用，但您可以简单地返回一个空结果。

#### copy-to-clipboard

将文本复制到剪贴板：

```python
{
    "title": "Copy this text",
    "subtitle": "Click to copy",
    "actions": [
        {
            "name": "Copy",
            "id": "copy-to-clipboard",
            "text": "Text to copy"
        }
    ]
}
```

#### open-url

在默认浏览器中打开 URL：

```python
{
    "title": "Open website",
    "subtitle": "Click to open",
    "actions": [
        {
            "name": "Open in Browser",
            "id": "open-url",
            "url": "https://example.com"
        }
    ]
}
```

#### open-directory

在文件管理器中打开目录：

```python
{
    "title": "Open folder",
    "subtitle": "Click to open",
    "actions": [
        {
            "name": "Open Folder",
            "id": "open-directory",
            "path": "/path/to/directory"
        }
    ]
}
```

#### notify

显示通知消息：

```python
{
    "title": "Show notification",
    "subtitle": "Click to notify",
    "actions": [
        {
            "name": "Notify",
            "id": "notify",
            "message": "Notification message"
        }
    ]
}
```

### 自定义操作

对于自定义操作，您需要在脚本中实现 `action` 方法：

```python
def handle_action(params, request_id):
    action_id = params.get("id", "")
    action_data = params.get("data", "")

    if action_id == "my-custom-action":
        # Handle your custom action
        return {
            "jsonrpc": "2.0",
            "result": {},
            "id": request_id
        }

    # For built-in actions or unknown actions, return empty result
    return {
        "jsonrpc": "2.0",
        "result": {},
        "id": request_id
    }
```

**注意**：`action` 方法会针对所有操作（内置和自定义）作为钩子被调用。这允许您在需要时甚至为内置操作添加额外的逻辑。但是，对于内置操作，您可以简单地返回一个空结果，Wox 将自动处理它们。

## 示例：简单计算器

```python
#!/usr/bin/env python3
# @wox.id simple-calculator
# @wox.name Simple Calculator
# @wox.keywords calc

import json
import sys
import re

def handle_query(params, request_id):
    search = params.get('search', '').strip()

    if not search:
        return {
            "jsonrpc": "2.0",
            "result": {"items": []},
            "id": request_id
        }

    try:
        # Simple math evaluation (be careful with eval in real plugins!)
        if re.match(r'^[0-9+\-*/().\s]+$', search):
            result = eval(search)
            return {
                "jsonrpc": "2.0",
                "result": {
                    "items": [{
                        "title": f"Result: {result}",
                        "subtitle": f"{search} = {result}",
                        "score": 100,
                        "actions": [
                            {
                                "id": "copy-result",
                                "data": str(result)
                            }
                        ]
                    }]
                },
                "id": request_id
            }
    except:
        pass

    return {
        "jsonrpc": "2.0",
        "result": {"items": []},
        "id": request_id
    }

def handle_action(params, request_id):
    # Handle copy action
    return {
        "jsonrpc": "2.0",
        "result": {},
        "id": request_id
    }

if __name__ == "__main__":
    request = json.loads(sys.stdin.read())
    method = request.get("method")
    params = request.get("params", {})
    request_id = request.get("id")

    if method == "query":
        response = handle_query(params, request_id)
    elif method == "action":
        response = handle_action(params, request_id)
    else:
        response = {
            "jsonrpc": "2.0",
            "error": {"code": -32601, "message": "Method not found"},
            "id": request_id
        }

    print(json.dumps(response))
```

## 更多示例

### 文件搜索插件

```bash
#!/bin/bash
# @wox.id file-search
# @wox.name File Search
# @wox.keywords fs

# Read JSON-RPC request
read -r request

# Parse request
search=$(echo "$request" | jq -r '.params.search // ""')
id=$(echo "$request" | jq -r '.id')

if [ -z "$search" ]; then
    echo '{"jsonrpc":"2.0","result":{"items":[]},"id":"'$id'"}'
    exit 0
fi

# Search files
results=()
while IFS= read -r -d '' file; do
    basename=$(basename "$file")
    results+=("{\"title\":\"$basename\",\"subtitle\":\"$file\",\"score\":90,\"action\":{\"id\":\"open-file\",\"data\":\"$file\"}}")
done < <(find "$HOME" -name "*$search*" -type f -print0 2>/dev/null | head -z -10)

# Build JSON response
items=$(IFS=,; echo "${results[*]}")
echo '{"jsonrpc":"2.0","result":{"items":['$items']},"id":"'$id'"}'
```

### 天气插件 (JavaScript)

```javascript
#!/usr/bin/env node
// @wox.id weather-plugin
// @wox.name Weather
// @wox.keywords weather

const https = require("https");

function handleQuery(params, requestId) {
  const search = params.search || "";

  if (!search) {
    return {
      jsonrpc: "2.0",
      result: { items: [] },
      id: requestId,
    };
  }

  // Mock weather data (replace with real API)
  const weatherData = {
    temperature: "22°C",
    condition: "Sunny",
    location: search,
  };

  return {
    jsonrpc: "2.0",
    result: {
      items: [
        {
          title: `${weatherData.temperature} - ${weatherData.condition}`,
          subtitle: `Weather in ${weatherData.location}`,
          score: 100,
          action: {
            id: "show-details",
            data: JSON.stringify(weatherData),
          },
        },
      ],
    },
    id: requestId,
  };
}

// Main execution
const input = process.stdin.read();
if (input) {
  const request = JSON.parse(input.toString());
  const response = handleQuery(request.params || {}, request.id);
  console.log(JSON.stringify(response));
}
```

## 最佳实践

1. **保持简单**：脚本插件最适合简单的无状态操作
2. **处理错误**：始终处理异常并返回正确的 JSON-RPC 响应
3. **性能**：记住脚本是为每个查询执行的
4. **安全**：小心用户输入，尤其是在使用 `eval()` 或执行命令时
5. **测试**：在 Wox 中使用之前，使用 JSON 输入手动测试您的脚本
6. **使用环境变量**：利用提供的 WOX*DIRECTORY*\* 变量
7. **验证输入**：始终验证和清理用户输入
8. **提供有意义的结果**：使用描述性的标题和副标题

## 调试技巧

### 手动测试

手动测试您的脚本：

```bash
# Create test input
echo '{"jsonrpc":"2.0","method":"query","params":{"search":"test"},"id":"1"}' | ./your-script.py

# Expected output format
{"jsonrpc":"2.0","result":{"items":[...]},"id":"1"}
```

### 常见问题

1. **脚本不可执行**：运行 `chmod +x your-script.py`
2. **JSON 解析错误**：验证您的 JSON 输出
3. **超时问题**：优化您的脚本速度
4. **缺少 shebang**：始终包含 `#!/usr/bin/env python3` 或类似内容

## 局限性

- **执行超时**：脚本必须在 10 秒内完成
- **无持久状态**：脚本为每个查询重新执行
- **API 有限**：无法访问高级 Wox API，如 AI 集成
- **性能**：不适合高频查询或复杂操作
- **设置访问**：虽然您可以定义设置 UI，但访问设置值需要额外的实现（存储在文件中或使用环境变量）

## 迁移到全功能插件

如果您的脚本插件变得复杂，请考虑迁移到全功能插件：

- 使用 Python SDK: `wox-plugin`
- 使用 Node.js SDK: `@wox-launcher/wox-plugin`
- 访问完整的 Wox API
- 持久状态和更好的性能
- 支持设置 UI 和高级功能
- AI 集成能力
- 自定义预览支持
