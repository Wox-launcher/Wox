# Single-file SDK Plugin

A single-file SDK plugin is one `.py` or CommonJS `.js` file that loads into Wox's existing Python or Node.js runtime host. It has the full Public API of a packaged SDK plugin, without a `.wox` package and without starting a new process for each query.

## How it compares

```text
Script Plugin
  = one file + a new process per call + stdin/stdout JSON-RPC

Single-file SDK Plugin
  = one file + the shared SDK runtime host + full Public API

SDK Plugin
  = .wox package + the shared SDK runtime host + full Public API
```

| Need | Choose |
|---|---|
| A one-shot shell or command wrapper | Script Plugin |
| One file, and you need the Wox API | Single-file SDK Plugin |
| Dependencies, resources, TypeScript, or multiple files | SDK Plugin |

Script plugins are not version 1 of this feature, and they are not deprecated.

Query and action on a single-file SDK plugin reuse the host process. Wox does not start a plugin-specific Python or Node process. Host crash recovery still uses the existing watchdog.

## Getting started

Use `wpm create <name>` and pick **Python Single-file SDK Plugin** or **Node.js Single-file SDK Plugin**. WPM writes:

```text
~/.wox/wox-user/plugins/single-file/Wox.Plugin.<Name>.py
~/.wox/wox-user/plugins/single-file/Wox.Plugin.<Name>.js
```

Then opens the file. Saving it reloads the plugin.

This is also the direct WPM path for a Python SDK plugin. You do not need to clone the packaged Python template unless you need extra files or pip dependencies.

### Python

Python files use runtime `PYTHON`. You can `import wox_plugin` and use SDK models, helpers, and the full Public API.

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

Node.js files use runtime `NODEJS`. The first version is CommonJS only:

- file extension `.js`
- `module.exports.plugin`
- full Public API through `params.API`
- do not `import` / `require` `@wox-launcher/wox-plugin`
- simple types such as `WoxImage` are object literals

The first version does not support `.mjs`, ESM, TypeScript, npm dependencies, or SDK npm helpers. The Node host compiles dynamic `import()` to `require()`, so CommonJS can reuse the existing loader and `require.cache` reload.

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

Put a JSON object in the leading `#` or `//` comments. A shebang on the first line is allowed.

Required in the header:

- `Id`
- `Name`
- `Version`
- `MinWoxVersion`
- `Runtime`
- `TriggerKeywords`

Also supported: `Author`, `Description`, `Icon`, `Website`, `Commands`, `SupportedOS`, `Features`, `Glances`, `SettingDefinitions`, `QueryRequirements`, `I18n`.

Wox sets `Entry` to the file name and `Directory` to `plugins/single-file`. The header must not declare `Entry` or `Directory`.

Runtime must match the suffix. Wox does not guess or fall back:

| File | Valid Runtime |
|---|---|
| `.py` | `PYTHON` |
| `.js` | `NODEJS` |

Do not put a `PYTHON` or `NODEJS` file in `plugins/scripts/`. Script plugins without `Runtime` still load as `SCRIPT`. An explicit `PYTHON`/`NODEJS` Runtime in the scripts directory is rejected with a message to move the file to `plugins/single-file/`.

## Reload

Saving the file reloads it after a short debounce (about 500ms).

- `init()` runs once after each load or reload.
- Query/action keep in-memory state until the next save.
- Saving resets that state. There is no state migration and no transactional rollback of the old instance.
- Reload calls the existing unload path first, so timers, watchers, and sockets registered with unload callbacks can clean up.
- Invalid metadata is logged and the previous instance stays loaded.
- Deleting the file unloads that plugin. Renaming unloads the old path and waits for a create on the new path.

Register unload callbacks whenever you start background work.

## Shared directory

All single-file plugins live in `plugins/single-file/`. They do not load a shared `lang/` folder. Use inline `I18n` in the header.

Metadata icons cannot be relative paths. Dynamic results should also avoid relative images. Supported image forms:

- emoji
- URL
- SVG
- base64
- absolute path

If you need extra resource files, package a `.wox` plugin. Settings and WPM locate the plugin file instead of opening the mixed shared directory.

## Store delivery

The store does not add a new manifest field. Wox classifies the download from `Runtime` plus the URL path suffix (query strings are ignored):

| Runtime | URL suffix | Type |
|---|---|---|
| `PYTHON` | `.py` | Single-file SDK Plugin |
| `NODEJS` | `.js` | Single-file SDK Plugin |
| `PYTHON` / `NODEJS` | `.wox` | Packaged SDK Plugin |
| `SCRIPT` | script file | Script Plugin |

Unknown suffixes, missing suffixes, and mismatches such as `PYTHON` + `.js` are rejected. A plugin ID cannot change delivery form between `.wox` and single-file during update.

Single-file store rows must declare `MinWoxVersion` at or above the first Wox release that can load them. Header `Id`, `Runtime`, and `Version` must match the store manifest. Uninstall moves only that file to the trash.
