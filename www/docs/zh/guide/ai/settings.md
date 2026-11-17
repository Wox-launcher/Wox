# AI 设置

AI 功能是可选的。只有在你需要 AI 对话、AI 命令、AI 辅助 Emoji 搜索或 AI 生成主题时，才需要配置 provider。

![AI provider 设置](/images/guide/ai_provider.jpg)

## 添加 Provider

1. 打开 **设置 -> AI**。
2. 点击 **添加**。
3. 选择 **API** provider，或 **已安装的 CLI** provider。
4. 填写名称、凭据或 CLI 信息、模型，以及需要时的自定义 host。
5. 保存后，在具体功能中选择这个 provider。

provider 列表会分成 **API** 和 **已安装的 CLI**。API provider 使用密钥和可选 host。CLI provider 使用本机已安装的命令，例如桌面编程助手；Wox 认得出时会显示品牌图标。列表可以搜索。

## 字段含义

| 字段 | 作用 |
| --- | --- |
| Provider 名称 | 在 Wox 设置里识别这个 provider 的名称。 |
| API key | Wox 发给 API provider 的凭据。 |
| Host | 兼容服务、代理或本地服务的可选 API 地址。 |
| Model | 聊天、命令或生成类功能使用的模型。 |
| CLI | 已安装 CLI provider 使用的本地命令。 |

## 使用 MCP 查询公开文档

AI 对话可以通过 MCP 服务器查询技术资料，并在回答中附上原始来源链接。例如，[Parallel MCP setup](https://docs.parallel.ai/integrations/mcp/quickstart) 提供托管搜索服务，无需 Parallel API key 即可在限流范围内使用。你仍需配置支持工具调用的 AI provider。

1. 打开 **设置 -> AI -> MCP 服务器 -> 编辑 JSON**。
2. 将下面的 `Parallel` 条目添加到 `mcpServers` 中，保留已有的服务器条目。保存此编辑器会替换整个服务器列表。
3. 保存后，检查 Parallel 行是否显示 1 个工具。`enabledTools` 列表仅开放此服务器的 `web_search` 工具。

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

新建 AI 对话，选择 provider 和模型，然后输入公开文档查询：

```text
使用 Parallel 查找 Python 官方文档中使用 venv 创建虚拟环境的方法。在回答中包含原始来源 URL。仅搜索公开文档。
```

展开工具活动，确认搜索显示为 `Parallel/web_search`。检查回答是否链接到工具返回的原始文档页面。如果搜索失败或没有返回有用的来源，请让模型如实说明，不要编造引用。

已启用且名为 `web_search` 的 MCP 工具会优先于 Wox 的内置搜索工具。要恢复内置搜索，请编辑 Parallel 服务器行，勾选 **禁用** 并保存，然后新建对话。如果要使用内置 `web_search`，请保持该内置工具启用。

只输入你打算发送给 Parallel 的查询。对话内容也会发送给所选 AI provider；进行此查询时，请避免附加私有文件、剪贴板或选中文本。

## 安全注意事项

- 把 API key 当作密码处理。
- 付费 provider 可能会对每次请求计费，包括 AI 命令和主题生成。
- 只使用可信的自定义 host；Wox 会把 prompt 内容发送到该地址。
- 不要把敏感剪贴板、选中文本或私有文件发送给在线模型，除非你确认这符合自己的工作流。

## 相关功能

- [AI 对话](../plugins/system/chat.md)
- [AI 命令](./commands.md)
- [主题生成](./theme.md)

## 排查

如果 AI 功能没有返回结果，先检查：

1. provider 已启用，并被对应功能选中。
2. API key 有效，或 CLI 命令在 PATH 中。
3. 模型名被 provider 接受。
4. 自定义 host 可以访问。
5. API provider 的网络可用。
