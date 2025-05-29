# Full-featured Plugin Development Guide

Full-featured plugins are comprehensive plugins that run in dedicated host processes and communicate with Wox via WebSocket. They provide the full power of the Wox plugin system with rich APIs, persistent state, and advanced features.

## Overview

Full-featured plugins are designed for complex applications that require:
- Persistent state management
- High-performance query processing
- Advanced Wox API integration (AI, settings, previews)
- Complex async operations
- Professional-grade plugin development

## Supported Languages

### Python Plugins

**Requirements:**
- Python >= 3.8 (Python 3.12 recommended)
- `wox-plugin` SDK

**Installation:**
```bash
# Using pip
pip install wox-plugin

# Using uv (recommended)
uv add wox-plugin
```

**Basic Structure:**
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

### Node.js Plugins

**Requirements:**
- Node.js >= 16
- `@wox-launcher/wox-plugin` SDK

**Installation:**
```bash
npm install @wox-launcher/wox-plugin
# or
pnpm add @wox-launcher/wox-plugin
```

**Basic Structure:**
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

## Plugin Architecture

### Plugin Host System

Full-featured plugins run in dedicated host processes:
- **Python Host**: `wox.plugin.host.python` manages Python plugins
- **Node.js Host**: `wox.plugin.host.nodejs` manages Node.js plugins

### Communication Flow

1. **Wox Core** ↔ **Plugin Host** (WebSocket)
2. **Plugin Host** ↔ **Plugin Instance** (Direct method calls)

### Plugin Lifecycle

1. **Load**: Plugin host loads the plugin module
2. **Init**: Plugin initialization with API access
3. **Query**: Handle user queries (multiple times)
4. **Unload**: Plugin cleanup when disabled/uninstalled

## Rich API Features

### Core APIs

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

### AI Integration

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

### Advanced Result Features

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

# Refreshable result
Result(
    title="Live Data",
    subtitle="Updates automatically",
    on_refresh=self.refresh_data
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

## Plugin Configuration

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

### Settings Definition

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

## Performance Optimization

### Async Operations

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

### Caching

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

## Error Handling

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

## Testing

### Unit Testing

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

## Best Practices

1. **Use Async/Await**: Leverage async operations for better performance
2. **Handle Errors Gracefully**: Always catch and log exceptions
3. **Implement Caching**: Cache expensive operations when appropriate
4. **Provide Good UX**: Use meaningful titles, subtitles, and icons
5. **Follow Naming Conventions**: Use clear, descriptive names
6. **Document Your Code**: Add docstrings and comments
7. **Test Thoroughly**: Write unit tests for your plugin logic

## Migration from Script Plugin

If you have a script plugin that needs more features:

1. **Create Plugin Structure**: Set up proper plugin directory with `plugin.json`
2. **Install SDK**: Add the appropriate SDK dependency
3. **Convert Logic**: Move your script logic to the plugin class
4. **Add State Management**: Utilize persistent state if needed
5. **Enhance with APIs**: Add AI, settings, or other advanced features
6. **Optimize Performance**: Implement caching and async operations

## Publishing

1. **Test Locally**: Ensure your plugin works correctly
2. **Create Package**: Follow the plugin packaging guidelines
3. **Submit to Store**: Use the plugin submission process
4. **Maintain**: Keep your plugin updated and respond to user feedback
