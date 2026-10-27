# AI Chat

This package owns the AI Chat system plugin. It registers through `init`, with
an explicit import in `main.go`, like the other standalone system plugins.

- `chat.go`: plugin entry points, conversation storage and summaries, streaming,
  compaction, attachments and runtime prompts.
- `chat_plugin_mention.go`: plugin mentions and the tools made available by them.
- `chat_project.go`: project metadata, local directory bindings, chat membership
  and per-turn project context. See [project behavior](chat_project.md).
- The adjacent tests cover these responsibilities within this package.

Other features invoke Chat through the plugin API or the internal capabilities
in `common`; they do not depend on its concrete state. Screenshot uses the
exported `ToolOpenChatWithAttachments` name with the plugin tool invocation API.
Keep plugin IDs, tool names and persisted setting keys stable when reorganizing
the implementation.

Chat views and their controller remain under `ui/launcher`; provider execution
and reusable built-in tools remain under `ai`. This package must not import its
parent `plugin/system` package.
