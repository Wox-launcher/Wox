# Chat Plugin

Chat opens an AI conversation inside Wox. Configure an AI provider before using it.

![The Chat plugin](/images/guide/plugin_chat.jpg)

## Quick Start

```text
chat How do I use Wox?
```

You can continue from chat history, switch models, attach selected text, files, or images, and use configured MCP tools. Type `/` in the chat input to select a model or skill.

A conversation can be popped into a dedicated window. The unsent draft stays there after the launcher hides.

## Mention a Plugin

Type `@` in the chat input and select a plugin to make its tools available to the AI. Then describe what you want it to do. Only enabled plugins that provide tools appear in the list. The selected plugin's tools remain available in later turns of the conversation.

For example, mention the Unsplash plugin and ask it to change your desktop wallpaper:

![Using @Unsplash in a chat](/images/guide/ai-chat-at-plugin.png)

## Settings

| Setting | Use |
| --- | --- |
| Fallback Search | Offer an AI Chat result when no non-debounced plugin has returned a result yet. Results from debounced plugins may still appear later. |
| Default Model | Provider and model used for a new conversation. Sending a chat also remembers its model for the next new conversation. |

Configure providers, MCP servers, and skills in **Settings -> AI**, separately from the AI Chat plugin settings. See [AI Settings](../../ai/settings.md) for provider setup. The plugin's trigger keywords can also be changed on its settings page.

## Privacy

Chat messages are sent to the selected model provider. Do not paste secrets, private documents, or selected text into online models unless that is acceptable for your workflow.
