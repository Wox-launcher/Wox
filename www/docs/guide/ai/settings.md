# AI Settings

AI features are optional. Configure a provider only if you want AI Chat, AI Commands, AI-assisted emoji search, or AI theme generation.

![AI provider settings](/images/guide/ai_provider.jpg)

## Add a Provider

1. Open **Settings -> AI**.
2. Click **Add**.
3. Choose an **API** provider or an **Installed CLI** provider.
4. Enter the provider name, credential or CLI details, model, and a custom host if needed.
5. Save the provider and select it in the feature that should use it.

The provider list is grouped into **API** and **Installed CLI**. API providers use a key and optional host. CLI providers use a local binary that is already installed, such as a desktop coding agent, and show a brand icon when Wox recognizes them. The list is searchable.

## What the Settings Mean

| Field | Use |
| --- | --- |
| Provider name | A label you recognize in Wox settings. |
| API key | The credential Wox sends to an API provider. |
| Host | Optional API endpoint for compatible services, proxies, or local servers. |
| Model | The model used by chat, commands, or generation features. |
| CLI | The installed local command for an Installed CLI provider. |

## Search Public Documentation with MCP

AI Chat can use an MCP server for a technical lookup and include links to the original sources. For example, [Parallel MCP setup](https://docs.parallel.ai/integrations/mcp/quickstart) provides a hosted search server with rate-limited access without a Parallel API key. You still need a configured AI provider that supports tool calls.

1. Open **Settings -> AI -> MCP Servers -> Edit JSON**.
2. Add the `Parallel` entry below inside `mcpServers`, keeping any existing server entries. Saving this editor replaces the full server list.
3. Save, then check that the Parallel row shows `1 tools`. The `enabledTools` list makes only `web_search` available from this server.

```json
{
  "mcpServers": {
    "Parallel": {
      "type": "http",
      "url": "https://mcp.parallel.ai/mcp",
      "headers": {
        "User-Agent": "Wox"
      },
      "enabledTools": ["web_search"]
    }
  }
}
```

Open a new AI Chat, select your provider and model, and enter a public documentation query:

```text
Use Parallel to find the official Python documentation for creating a virtual environment with venv. Include the original source URL in your answer. Search public documentation only.
```

Expand the tool activity to check that the search ran as `Parallel/web_search`. Check that the answer links to an original documentation page returned by the tool. If the search fails or returns no useful sources, ask the model to say so instead of inventing a citation.

An enabled MCP tool named `web_search` takes precedence over Wox's built-in search tool. To return to built-in search, edit the Parallel server row, select **Disabled**, and save, then start a new chat. Leave the built-in `web_search` tool enabled if you want to use it.

Only enter queries you intend to send to Parallel. Chat content also goes to your selected AI provider; avoid attaching private files, clipboard text, or selected text to this lookup.

## Security Notes

- Treat API keys like passwords.
- Paid providers may bill each request, including AI Commands and theme generation.
- Custom hosts should be trusted; Wox sends your prompt content to that endpoint.
- Avoid sending sensitive clipboard text, selected text, or private files to online models unless you understand the provider policy.

## Related Features

- [AI Chat](../plugins/system/chat.md)
- [AI Commands](./commands.md)
- [Theme generation](./theme.md)

## Troubleshooting

If an AI feature does not return anything, check these first:

1. The provider is enabled and selected by the feature.
2. The API key is valid, or the CLI binary is on your PATH.
3. The model name is accepted by the provider.
4. The custom host URL is reachable.
5. Network access is available for API providers.
