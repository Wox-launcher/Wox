---
applyTo: '**/*.py'
---

# Wox Python Plugin Development Guide

## Basic Standards

* Use Python type annotations to ensure type safety
* Maintain consistent code style, follow PEP 8 standards
* Use `async/await` for asynchronous programming

## Plugin Structure

* Plugin entry class must inherit from [wox_plugin.plugin.WoxPlugin](mdc:wox.plugin.python/src/wox_plugin/plugin.py)
* Plugins must implement the `query` method to respond to user searches
* Optionally implement the `query_fallback` method to provide alternatives when no results are found

## Data Models

* Use Python dataclasses for data model definitions, refer to [query.py](mdc:wox.plugin.python/src/wox_plugin/models/query.py)
* Data models must align with models defined in wox.core

## API Usage

* Plugins can use various methods provided by [WoxAPI](mdc:wox.plugin.python/src/wox_plugin/api.py)
* API functionality must align with definitions in [wox.core/plugin/api.go](mdc:wox.core/plugin/api.go)
* All API calls must include the Context object
* Example of using the API:

```python
async def query(self, ctx: Context, query: Query) -> List[QueryResult]:
    # Use API to log
    await self.api.log(ctx, "info", f"Query: {query.search}")
    
    # Return query results
    return [
        QueryResult(
            title="Example Result",
            subtitle="This is an example result",
            icon_path=os.path.join(self.plugin_dir, "icon.png"),
            score=100,
            action=Action(
                action_type=ActionType.DO_NOTHING,
                action_name="do_nothing"
            )
        )
    ]
```

