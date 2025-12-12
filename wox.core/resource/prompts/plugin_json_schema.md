# plugin.json Schema

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
{"Id":"{{.ExampleID}}","Name":"My Plugin","Version":"1.0.0","MinWoxVersion":"2.0.0","Runtime":"PYTHON","Entry":"main.py","Icon":"emoji:🚀","TriggerKeywords":["myp"],"SupportedOS":["Windows","Linux","Macos"]}
