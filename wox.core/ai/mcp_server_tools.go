package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"wox/util"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// parseToolArguments parses the raw JSON arguments from a tool request
func parseToolArguments(req *mcp.CallToolRequest) map[string]any {
	var args map[string]any
	if req.Params.Arguments != nil {
		_ = json.Unmarshal(req.Params.Arguments, &args)
	}
	if args == nil {
		args = make(map[string]any)
	}
	return args
}

func handleGetPluginSDKDocs(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseToolArguments(req)
	runtime, _ := args["runtime"].(string)

	var docs string
	switch runtime {
	case "nodejs":
		docs = getNodeJSSDKDocs()
	case "python":
		docs = getPythonSDKDocs()
	case "script":
		docs = getScriptPluginDocs()
	default:
		docs = "Please specify a runtime: nodejs, python, or script"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: docs},
		},
	}, nil
}

func handleGetPluginJsonSchema(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	schema := getPluginJsonSchema()
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: schema},
		},
	}, nil
}

func handleGeneratePluginScaffold(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseToolArguments(req)
	runtime, _ := args["runtime"].(string)
	name, _ := args["name"].(string)
	triggerKeywordsRaw, _ := args["trigger_keywords"].([]any)
	description, _ := args["description"].(string)

	if description == "" {
		description = fmt.Sprintf("A %s plugin for Wox", name)
	}

	var triggerKeywords []string
	for _, kw := range triggerKeywordsRaw {
		if s, ok := kw.(string); ok {
			triggerKeywords = append(triggerKeywords, s)
		}
	}

	var scaffold string
	switch runtime {
	case "nodejs":
		scaffold = generateNodeJSScaffold(name, description, triggerKeywords)
	case "python":
		scaffold = generatePythonScaffold(name, description, triggerKeywords)
	case "script":
		scaffold = generateScriptScaffold(name, description, triggerKeywords)
	default:
		scaffold = "Please specify a runtime: nodejs, python, or script"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: scaffold},
		},
	}, nil
}

func handleGetWoxDirectories(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	location := util.GetLocation()
	dirs := map[string]string{
		"wox_data_directory":            location.GetWoxDataDirectory(),
		"user_data_directory":           location.GetUserDataDirectory(),
		"plugins_directory":             location.GetPluginDirectory(),
		"user_script_plugins_directory": location.GetUserScriptPluginsDirectory(),
		"log_directory":                 location.GetLogDirectory(),
	}

	jsonBytes, _ := json.MarshalIndent(dirs, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonBytes)},
		},
	}, nil
}

func getPluginJsonSchema() string {
	exampleID := uuid.New().String()
	return fmt.Sprintf(`# plugin.json Schema

## Required Fields
- Id: string (UUID format)
- Name: string (display name)
- Version: string (semantic version)
- MinWoxVersion: string (minimum Wox version)
- Runtime: string (PYTHON, NODEJS, or SCRIPT)
- Entry: string (entry file path)
- Icon: string (WoxImage: emoji:X, base64:X, or relative path)
- TriggerKeywords: array (use "*" for global)
- SupportedOS: array (Windows, Linux, Macos)

## Optional Fields
- Author, Website, Description, Commands, Features, SettingDefinitions

## Feature Flags
- querySelection: Access selected text/files
- queryEnv: Access query environment info
- ai: Use AI/LLM capabilities
- mru: Access MRU list
- debounce: Debounce query input (IntervalMs param)
- ignoreAutoScore: Disable auto-scoring
- deepLink: Handle deep links

## Example
{"Id":"%s","Name":"My Plugin","Version":"1.0.0","MinWoxVersion":"2.0.0","Runtime":"PYTHON","Entry":"main.py","Icon":"emoji:🚀","TriggerKeywords":["myp"],"SupportedOS":["Windows","Linux","Macos"]}`, exampleID)
}

func getNodeJSSDKDocs() string {
	return `# Wox Node.js Plugin SDK

## Installation
pnpm add @wox-launcher/wox-plugin

## Key Types

### Plugin Interface
interface Plugin {
  init(ctx: Context, params: PluginInitParams): Promise<void>
  query(ctx: Context, query: Query): Promise<Result[]>
}

### Query
interface Query {
  Type: "input" | "selection"
  RawQuery: string
  TriggerKeyword: string
  Command: string
  Search: string
}

### Result
interface Result {
  Title: string
  SubTitle?: string
  Icon: WoxImage
  Actions: ResultAction[]
  Score?: number
}

### ResultAction
interface ResultAction {
  Id: string
  Name: string
  IsDefault?: boolean
  Action: (ctx: Context, actionContext: ActionContext) => Promise<void>
}

### WoxImage
type WoxImageType = "absolute" | "relative" | "base64" | "svg" | "url" | "emoji" | "lottie"
interface WoxImage { ImageType: WoxImageType; ImageData: string }

### API Methods
- ChangeQuery(ctx, query): Change the query
- HideApp(ctx): Hide Wox
- ShowApp(ctx): Show Wox
- Notify(ctx, message): Show notification
- Log(ctx, level, msg): Write log
- GetSetting(ctx, key): Get plugin setting
- SaveSetting(ctx, key, value): Save plugin setting
- LLMStream(ctx, conversations, callback): Chat with LLM

## Example
import { Plugin, Query, Result, WoxImage } from "@wox-launcher/wox-plugin"

class MyPlugin implements Plugin {
  async init(ctx, params) { this.api = params.API }
  async query(ctx, query) {
    return [{
      Title: "Hello " + query.Search,
      Icon: { ImageType: "emoji", ImageData: "👋" },
      Actions: [{ Id: "copy", Name: "Copy", Action: async () => {} }]
    }]
  }
}
export const plugin = new MyPlugin()`
}

func getPythonSDKDocs() string {
	return `# Wox Python Plugin SDK

## Installation
uv add wox-plugin

## Key Classes

### Plugin Base Class
from wox_plugin import Plugin, Query, Result, Context, PluginInitParams
from wox_plugin.models.image import WoxImage

class MyPlugin(Plugin):
    async def init(self, ctx: Context, params: PluginInitParams) -> None:
        self.api = params.api

    async def query(self, ctx: Context, query: Query) -> list[Result]:
        return []

### Query Model
class Query:
    type: str  # "input" or "selection"
    raw_query: str
    trigger_keyword: str
    command: str
    search: str

### Result Model
class Result:
    title: str
    icon: WoxImage
    sub_title: str = ""
    actions: List[ResultAction] = []
    score: float = 0.0

### WoxImage
class WoxImage:
    image_type: str  # "emoji", "url", "base64", etc.
    image_data: str

    @classmethod
    def emoji(cls, emoji: str) -> "WoxImage":
        return cls(image_type="emoji", image_data=emoji)

### API Methods
- change_query(ctx, query): Change the query
- hide_app(ctx): Hide Wox
- show_app(ctx): Show Wox
- notify(ctx, message): Show notification
- log(ctx, level, msg): Write log
- get_setting(ctx, key): Get plugin setting
- save_setting(ctx, key, value): Save plugin setting
- llm_stream(ctx, conversations, callback): Chat with LLM

## Example
from wox_plugin import Plugin, Query, Result, Context, PluginInitParams
from wox_plugin.models.image import WoxImage
from wox_plugin.models.result import ResultAction

class HelloPlugin(Plugin):
    async def init(self, ctx, params): self.api = params.api
    async def query(self, ctx, query):
        return [Result(
            title=f"Hello {query.search}",
            icon=WoxImage.emoji("👋"),
            actions=[ResultAction(id="copy", name="Copy", action=lambda c,a: None)]
        )]

plugin = HelloPlugin()`
}

func getScriptPluginDocs() string {
	exampleID := uuid.New().String()
	return fmt.Sprintf(`# Wox Script Plugin

Script plugins are single-file plugins with complete plugin.json metadata embedded in comments. They use JSON-RPC over stdin/stdout.

## Supported Languages
- Python (.py)
- JavaScript/Node.js (.js)
- Shell scripts (.sh, .bat, .ps1)

## Metadata Format
Embed a complete plugin.json structure in comments at the top of the file. The JSON block must be valid JSON wrapped in comment lines.

### Python/Shell Example:
# {
#   "Id": "%s",
#   "Name": "My Script Plugin",
#   "Version": "1.0.0",
#   "MinWoxVersion": "2.0.0",
#   "Runtime": "SCRIPT",
#   "Entry": "my_plugin.py",
#   "TriggerKeywords": ["myp"],
#   "SupportedOS": ["Windows", "Linux", "Macos"],
#   "Icon": "emoji:🔧"
# }

### JavaScript Example:
// {
//   "Id": "uuid-here",
//   "Name": "My Script Plugin",
//   "Runtime": "SCRIPT",
//   ...
// }

## Required Fields in Metadata
- Id: string (UUID format)
- Name: string (display name)
- TriggerKeywords: array of strings
- Version: "1.0.0"
- MinWoxVersion: "2.0.0"
- Author: "Unknown"
- Description: "A script plugin"
- Icon: "emoji:📝"
- SupportedOS: ["Windows", "Linux", "Macos"]

## JSON-RPC Protocol
Input (stdin): {"method":"query","params":{"Search":"user input","RawQuery":"kw user input",...}}
Output (stdout): {"result":[{"Title":"Result","SubTitle":"desc","Icon":{"ImageType":"emoji","ImageData":"✨"}}]}

## Query Params
- RawQuery: The complete query string
- TriggerKeyword: The trigger keyword used
- Command: The command (if any)
- Search: The search term after keyword and command

## Result Fields
- Title: string (required)
- SubTitle: string
- Icon: {"ImageType": "emoji"|"url"|"base64", "ImageData": "..."}
- Actions: [{"Id": "id", "Name": "Action Name", "IsDefault": true}]
- Score: number (0-100)

## Example Python Script
#!/usr/bin/env python3
# {
#   "Id": "example-uuid",
#   "Name": "Hello Script",
#   "Version": "1.0.0",
#   "MinWoxVersion": "2.0.0",
#   "Runtime": "SCRIPT",
#   "Entry": "hello.py",
#   "TriggerKeywords": ["hello"],
#   "SupportedOS": ["Windows", "Linux", "Macos"],
#   "Icon": "emoji:👋"
# }

import sys, json

def query(params):
    search = params.get("Search", "")
    return [{"Title": f"Hello {search}", "SubTitle": "Script plugin",
             "Icon": {"ImageType": "emoji", "ImageData": "👋"},
             "Actions": [{"Id": "copy", "Name": "Copy", "IsDefault": True}]}]

if __name__ == "__main__":
    req = json.loads(sys.stdin.read())
    if req.get("method") == "query":
        print(json.dumps({"result": query(req.get("params", {}))}))

## Script Plugin Location
Place script files in: ~/.wox/user-data/script-plugins/
Script plugins are automatically loaded and reloaded when modified.`, exampleID)
}

func generateNodeJSScaffold(name, description string, triggerKeywords []string) string {
	pluginID := uuid.New().String()
	keywords, _ := json.Marshal(triggerKeywords)
	return fmt.Sprintf(`# %s - Node.js Plugin Scaffold

## plugin.json
{"Id":"%s","Name":"%s","Description":"%s","Version":"1.0.0","MinWoxVersion":"2.0.0","Runtime":"NODEJS","Entry":"dist/index.js","Icon":"emoji:🚀","TriggerKeywords":%s,"SupportedOS":["Windows","Linux","Macos"]}

## src/index.ts
import { Context, Plugin, PluginInitParams, Query, Result, WoxImage, PublicAPI } from "@wox-launcher/wox-plugin"

class %sPlugin implements Plugin {
  private api!: PublicAPI

  async init(ctx: Context, params: PluginInitParams): Promise<void> {
    this.api = params.API
  }

  async query(ctx: Context, query: Query): Promise<Result[]> {
    return [{
      Title: "Hello from %s",
      SubTitle: query.Search || "Type something...",
      Icon: { ImageType: "emoji", ImageData: "🚀" } as WoxImage,
      Actions: [{
        Id: "action",
        Name: "Execute",
        IsDefault: true,
        Action: async (ctx, actionContext) => {
          await this.api.Notify(ctx, "Action executed!")
        }
      }]
    }]
  }
}

export const plugin = new %sPlugin()

## package.json
{"name":"%s","version":"1.0.0","main":"dist/index.js","scripts":{"build":"tsc"},"dependencies":{"@wox-launcher/wox-plugin":"latest"},"devDependencies":{"typescript":"^5.0.0"}}

## tsconfig.json
{"compilerOptions":{"target":"ES2020","module":"commonjs","outDir":"./dist","strict":true,"esModuleInterop":true},"include":["src/**/*"]}

## Build Steps
1. pnpm install
2. pnpm build`, name, pluginID, name, description, string(keywords), toPascalCase(name), name, toPascalCase(name), toKebabCase(name))
}

func generatePythonScaffold(name, description string, triggerKeywords []string) string {
	pluginID := uuid.New().String()
	keywords, _ := json.Marshal(triggerKeywords)
	return fmt.Sprintf(`# %s - Python Plugin Scaffold

## plugin.json
{"Id":"%s","Name":"%s","Description":"%s","Version":"1.0.0","MinWoxVersion":"2.0.0","Runtime":"PYTHON","Entry":"main.py","Icon":"emoji:🚀","TriggerKeywords":%s,"SupportedOS":["Windows","Linux","Macos"]}

## main.py
from wox_plugin import Plugin, Query, Result, Context, PluginInitParams, PublicAPI
from wox_plugin.models.image import WoxImage
from wox_plugin.models.result import ResultAction

class %sPlugin(Plugin):
    api: PublicAPI

    async def init(self, ctx: Context, params: PluginInitParams) -> None:
        self.api = params.api

    async def query(self, ctx: Context, query: Query) -> list[Result]:
        async def execute_action(ctx: Context, action_context) -> None:
            await self.api.notify(ctx, "Action executed!")

        return [
            Result(
                title="Hello from %s",
                sub_title=query.search or "Type something...",
                icon=WoxImage.emoji("🚀"),
                actions=[
                    ResultAction(
                        id="action",
                        name="Execute",
                        is_default=True,
                        action=execute_action,
                    )
                ],
            )
        ]

plugin = %sPlugin()

## pyproject.toml
[project]
name = "%s"
version = "1.0.0"
dependencies = ["wox-plugin"]

[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

## Setup Steps
1. uv venv
2. uv pip install -e .`, name, pluginID, name, description, string(keywords), toPascalCase(name), name, toPascalCase(name), toKebabCase(name))
}

func generateScriptScaffold(name, description string, triggerKeywords []string) string {
	pluginID := uuid.New().String()
	keywords, _ := json.Marshal(triggerKeywords)
	pyFileName := toSnakeCase(name) + ".py"
	jsFileName := toSnakeCase(name) + ".js"

	return fmt.Sprintf(`# %s - Script Plugin

Place the script file in: ~/.wox/user-data/script-plugins/

## Python Version: %s

`+"```python"+`
#!/usr/bin/env python3
# {
#   "Id": "%s",
#   "Name": "%s",
#   "Description": "%s",
#   "Version": "1.0.0",
#   "MinWoxVersion": "2.0.0",
#   "Icon": "emoji:🚀",
#   "TriggerKeywords": %s
# }

"""
Available environment variables:
- WOX_PLUGIN_ID, WOX_PLUGIN_NAME
- WOX_DIRECTORY_USER_SCRIPT_PLUGINS, WOX_DIRECTORY_USER_DATA
- WOX_SETTING_<KEY>: Plugin settings (uppercase key)

Built-in Actions (handled automatically):
- copy-to-clipboard: {"id": "copy-to-clipboard", "text": "..."}
- open-url: {"id": "open-url", "url": "https://..."}
- open-directory: {"id": "open-directory", "path": "/path/..."}
- notify: {"id": "notify", "message": "..."}
"""

import sys
import json
import os


def handle_query(params, request_id):
    results = [
        {
            "title": "Hello from %s",
            "subtitle": "Type something to search...",
            "score": 100,
            "actions": [
                {"name": "Copy", "id": "copy-to-clipboard", "text": "Hello!"}
            ]
        }
    ]
    return {"jsonrpc": "2.0", "result": {"items": results}, "id": request_id}


def handle_action(params, request_id):
    action_id = params.get("id", "")
    if action_id == "custom-action":
        return {"jsonrpc": "2.0", "result": {"action": "notify", "message": "Custom action!"}, "id": request_id}
    return {"jsonrpc": "2.0", "result": {}, "id": request_id}


def main():
    try:
        request = json.loads(sys.argv[1] if len(sys.argv) > 1 else sys.stdin.read())
    except json.JSONDecodeError as e:
        print(json.dumps({"jsonrpc": "2.0", "error": {"code": -32700, "message": str(e)}, "id": None}))
        return 1

    method, params, req_id = request.get("method"), request.get("params", {}), request.get("id")
    if method == "query":
        print(json.dumps(handle_query(params, req_id)))
    elif method == "action":
        print(json.dumps(handle_action(params, req_id)))
    else:
        print(json.dumps({"jsonrpc": "2.0", "error": {"code": -32601, "message": "Method not found"}, "id": req_id}))
    return 0


if __name__ == "__main__":
    sys.exit(main())
`+"```"+`

## JavaScript Version: %s

`+"```javascript"+`
#!/usr/bin/env node
// {
//   "Id": "%s",
//   "Name": "%s",
//   "Description": "%s",
//   "Version": "1.0.0",
//   "MinWoxVersion": "2.0.0",
//   "Icon": "emoji:🚀",
//   "TriggerKeywords": %s
// }

let request;
try {
  request = process.argv.length > 2
    ? JSON.parse(process.argv[2])
    : JSON.parse(require("fs").readFileSync(0, "utf-8"));
} catch (e) {
  console.log(JSON.stringify({jsonrpc: "2.0", error: {code: -32700, message: e.message}, id: null}));
  process.exit(1);
}

switch (request.method) {
  case "query":
    console.log(JSON.stringify({
      jsonrpc: "2.0",
      result: {
        items: [{
          title: "Hello from %s",
          subtitle: "Type something...",
          score: 100,
          actions: [{name: "Copy", id: "copy-to-clipboard", text: "Hello!"}]
        }]
      },
      id: request.id
    }));
    break;
  case "action":
    console.log(JSON.stringify({jsonrpc: "2.0", result: {}, id: request.id}));
    break;
  default:
    console.log(JSON.stringify({jsonrpc: "2.0", error: {code: -32601, message: "Method not found"}, id: request.id}));
}
`+"```"+`

## Notes
- Script plugins are automatically reloaded when modified
- No installation or build steps required
- Just save the file and start using it
`, name, pyFileName, pluginID, name, description, string(keywords), name, jsFileName, pluginID, name, description, string(keywords), name)
}

func toSnakeCase(s string) string {
	result := ""
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				result += "_"
			}
			result += string(c + 32)
		} else if c == ' ' || c == '-' {
			result += "_"
		} else {
			result += string(c)
		}
	}
	return result
}

func toPascalCase(s string) string {
	if s == "" {
		return s
	}
	result := ""
	capitalizeNext := true
	for _, c := range s {
		if c == ' ' || c == '-' || c == '_' {
			capitalizeNext = true
			continue
		}
		if capitalizeNext {
			if c >= 'a' && c <= 'z' {
				result += string(c - 32)
			} else {
				result += string(c)
			}
			capitalizeNext = false
		} else {
			result += string(c)
		}
	}
	return result
}

func toKebabCase(s string) string {
	result := ""
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				result += "-"
			}
			result += string(c + 32)
		} else if c == ' ' || c == '_' {
			result += "-"
		} else {
			result += string(c)
		}
	}
	return result
}
