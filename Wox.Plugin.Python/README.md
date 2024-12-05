# Wox Plugin Python

This package provides type definitions for developing Wox plugins in Python.

## Installation

```bash
pip install wox-plugin
```

## Usage

```python
from wox_plugin import Plugin, Query, Result, Context, PluginInitParams

class MyPlugin(Plugin):
    async def init(self, ctx: Context, params: PluginInitParams) -> None:
        self.api = params.API
        
    async def query(self, ctx: Context, query: Query) -> list[Result]:
        # Your plugin logic here
        return []
```

## License

MIT 