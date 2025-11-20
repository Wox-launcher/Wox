# Script Plugin Development Guide

Script plugins are lightweight, single-file plugins that provide a simple way to extend Wox functionality. They are perfect for quick automation tasks, personal utilities, and learning plugin development.

## Overview

Script plugins communicate with Wox using JSON-RPC over stdin/stdout. Each script is executed on-demand when a query is made, making them ideal for simple, stateless operations.

For a ready-to-use example, see this gist: https://gist.github.com/qianlifeng/82a2f748177ce47a900b4c4da3abfd28

## Getting Started

### Creating a Script Plugin

Use the `wpm` plugin to create a new script plugin:

```
wpm create <name>
```

Available templates:

- `python` - Python script template
- `javascript` - JavaScript/Node.js script template
- `bash` - Bash script template

### Script Plugin Structure

A script plugin consists of a single executable file with metadata defined as a JSON object in comments:

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

The JSON metadata block must:

- Be placed in comments at the beginning of the file (after the shebang line)
- Use `#` for Python/Bash or `//` for JavaScript
- Contain a complete JSON object with all metadata fields

## Metadata Fields

### Required Fields

- `Id` - Unique plugin identifier (use UUID format recommended)
- `Name` - Display name of the plugin
- `TriggerKeywords` - Array of trigger keywords

### Optional Fields

- `Icon` - Plugin icon (emoji:🧮, relative:path/to/icon.png, or absolute path)
- `Version` - Plugin version (default: "1.0.0")
- `Author` - Plugin author (default: "Unknown")
- `Description` - Plugin description (default: "A script plugin")
- `MinWoxVersion` - Minimum required Wox version (default: "2.0.0")
- `SettingDefinitions` - Array of setting definitions (see Settings section below)
- `Features` - Array of plugin features (debounce, querySelection, etc.)
- `Commands` - Array of plugin commands
- `SupportedOS` - Array of supported operating systems (default: all platforms)

## Plugin Settings

You can define settings that users can configure in the Wox settings UI. Settings are defined in the `SettingDefinitions` array:

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

### Supported Setting Types

- **textbox** - Single or multi-line text input
- **checkbox** - Boolean checkbox
- **select** - Dropdown selection
- **label** - Display-only text label
- **head** - Section header
- **newline** - Line break
- **table** - Table with editable rows

### Accessing Settings in Your Script

Settings are automatically passed to script plugins as environment variables. Each setting is prefixed with `WOX_SETTING_` and the key is converted to uppercase.

For example, if you define a setting with key `api_key`, it will be available as the environment variable `WOX_SETTING_API_KEY`.

**Python Example:**

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

**JavaScript Example:**

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

**Bash Example:**

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

**Additional Environment Variables:**

Script plugins also have access to these environment variables:

- `WOX_PLUGIN_ID` - The plugin's unique ID
- `WOX_PLUGIN_NAME` - The plugin's display name
- `WOX_DIRECTORY_USER_SCRIPT_PLUGINS` - Directory where script plugins are stored
- `WOX_DIRECTORY_USER_DATA` - User data directory
- `WOX_DIRECTORY_WOX_DATA` - Wox application data directory
- `WOX_DIRECTORY_PLUGINS` - Plugin directory
- `WOX_DIRECTORY_THEMES` - Theme directory

## JSON-RPC Communication

Script plugins communicate with Wox using JSON-RPC 2.0 protocol.

### Request Format

Wox sends requests to your script via stdin:

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

### Response Format

Your script should respond via stdout:

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

## Available Methods

### query Method

Handles user queries and returns search results.

**Parameters:**

- `search` - The search term entered by user
- `trigger_keyword` - The keyword that triggered this plugin
- `command` - Command if using plugin commands
- `raw_query` - The complete raw query string

### action Method

Handles user selection of a result item.

**Parameters:**

- `id` - The action ID from the result item
- `data` - The action data from the result item

## Capabilities and limitations

- Script plugins receive only `search`, `trigger_keyword`, `command`, and `raw_query`. Selection payloads and query environment data are not passed to scripts.
- Each invocation is a fresh process with a 10s timeout; cache to disk if you need reuse.
- Previews, tails, MRU restoration, and result updates are reserved for full-featured plugins.

## Environment Variables

Script plugins have access to these environment variables:

- `WOX_DIRECTORY_USER_SCRIPT_PLUGINS` - Script plugins directory
- `WOX_DIRECTORY_USER_DATA` - User data directory
- `WOX_DIRECTORY_WOX_DATA` - Wox application data directory
- `WOX_DIRECTORY_PLUGINS` - Plugin directory
- `WOX_DIRECTORY_THEMES` - Theme directory

## Actions

Script plugins can use two types of actions:

### Action Format

Each result must have an `actions` field with an array of action objects (even for a single action).

Each action object can have:

- `id` (required): The action identifier
- `name` (optional): Display name in UI (defaults to "Execute")
- Other fields depending on the action type (e.g., `text` for clipboard, `url` for open-url)

**Example - Single Action**:

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

**Example - Multiple Actions**:

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

### Built-in Actions

Built-in actions are handled automatically by Wox. You can use them directly in your query results without implementing the `action` method in your script.

**Important**: When using built-in actions, you don't need to implement `handle_action()` in your script. Wox will handle the action automatically. The `action` method is still called as a hook, but you can simply return an empty result.

#### copy-to-clipboard

Copies text to the clipboard:

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

Opens a URL in the default browser:

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

Opens a directory in the file manager:

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

Shows a notification message:

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

### Custom Actions

For custom actions, you need to implement the `action` method in your script:

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

**Note**: The `action` method is called for ALL actions (built-in and custom) as a hook. This allows you to add additional logic even for built-in actions if needed. However, for built-in actions, you can simply return an empty result and Wox will handle them automatically.

## Example: Simple Calculator

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

## More Examples

### File Search Plugin

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

### Weather Plugin (JavaScript)

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

## Best Practices

1. **Keep it Simple**: Script plugins are best for simple, stateless operations
2. **Handle Errors**: Always handle exceptions and return proper JSON-RPC responses
3. **Performance**: Remember that scripts are executed for each query
4. **Security**: Be careful with user input, especially when using `eval()` or executing commands
5. **Testing**: Test your script manually with JSON input before using in Wox
6. **Use Environment Variables**: Leverage the provided WOX*DIRECTORY*\* variables
7. **Validate Input**: Always validate and sanitize user input
8. **Provide Meaningful Results**: Use descriptive titles and subtitles

## Debugging Tips

### Manual Testing

Test your script manually:

```bash
# Create test input
echo '{"jsonrpc":"2.0","method":"query","params":{"search":"test"},"id":"1"}' | ./your-script.py

# Expected output format
{"jsonrpc":"2.0","result":{"items":[...]},"id":"1"}
```

### Common Issues

1. **Script not executable**: Run `chmod +x your-script.py`
2. **JSON parsing errors**: Validate your JSON output
3. **Timeout issues**: Optimize your script for speed
4. **Missing shebang**: Always include `#!/usr/bin/env python3` or similar

## Limitations

- **Execution Timeout**: Scripts must complete within 10 seconds
- **No Persistent State**: Scripts are executed fresh for each query
- **Limited API**: No access to advanced Wox APIs like AI integration
- **Performance**: Not suitable for high-frequency queries or complex operations
- **Settings Access**: While you can define settings UI, accessing settings values requires additional implementation (store in files or use environment variables)

## Migration to Full-featured Plugin

If your script plugin grows complex, consider migrating to a full-featured plugin:

- Use Python SDK: `wox-plugin`
- Use Node.js SDK: `@wox-launcher/wox-plugin`
- Access to full Wox API
- Persistent state and better performance
- Support for settings UI and advanced features
- AI integration capabilities
- Custom preview support
