# Wox Plugin Python

This package provides type definitions for developing Wox plugins in Python.

## Requirements

- Python >= 3.8 (defined in `pyproject.toml`)
- Python 3.12 recommended for development (defined in `.python-version`)

## Installation

```bash
# Using pip
pip install wox-plugin

# Using uv (recommended)
uv add wox-plugin
```

## Usage

```python
from wox_plugin import BasePlugin, Query, Result, Context, PluginInitParams

class MyPlugin(BasePlugin):
    async def init(self, ctx: Context, params: PluginInitParams) -> None:
        self.api = params.API
        
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

## License

MIT 