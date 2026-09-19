# AI 设置

AI 功能是可选的。只有在你需要 AI 对话、AI 命令、AI 辅助 Emoji 搜索、听写润色或 AI 生成主题时，才需要配置 provider。

![AI provider 设置](/images/ai_provider.jpg)

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

## 安全注意事项

- 把 API key 当作密码处理。
- 付费 provider 可能会对每次请求计费，包括 AI 命令和主题生成。
- 只使用可信的自定义 host；Wox 会把 prompt 内容发送到该地址。
- 不要把敏感剪贴板、选中文本或私有文件发送给在线模型，除非你确认这符合自己的工作流。

## 相关功能

- [AI 对话](../plugins/system/chat.md)
- [AI 命令](./commands.md)
- [主题生成](./theme.md)
- [听写](/zh/features/dictation)

## 排查

如果 AI 功能没有返回结果，先检查：

1. provider 已启用，并被对应功能选中。
2. API key 有效，或 CLI 命令在 PATH 中。
3. 模型名被 provider 接受。
4. 自定义 host 可以访问。
5. API provider 的网络可用。
