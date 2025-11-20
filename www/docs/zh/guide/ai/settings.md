# AI 设置

要使用 Wox 的 AI 功能（如 AI 主题生成和自动 Git 提交信息生成），您需要先配置 AI 提供商设置。

## 配置步骤

1. 打开 Wox 设置
2. 从左侧边栏选择 "AI"
3. 点击右上角的 "+ Add" 按钮
4. 在弹出对话框中填写以下信息：

   - **Provider Name (提供商名称)**：选择您的 AI 提供商（例如 OpenAI）
   - **API Key (API 密钥)**：输入您的 API 密钥
   - **Host (主机)**：（可选）如果您使用的是 Ollama，请输入您的自定义 API 端点

5. 点击 "Confirm" 保存设置

![AI Settings](../../../data/images/ai_setting.png)

## 获取 API 密钥

### OpenAI

1. 访问 [OpenAI API Keys](https://platform.openai.com/account/api-keys)
2. 登录您的 OpenAI 账户
3. 点击 "Create new secret key"
4. 复制生成的密钥并将其粘贴到 Wox 的 API Key 字段中

## 重要提示

- 妥善保管您的 API 密钥，切勿分享
- 如果您使用的是付费服务，请留意 API 使用成本
- 某些功能可能需要特定的 API 访问级别 - 确保您的 API 密钥具有必要的权限

## 相关功能

配置 AI 设置后，您可以使用以下功能：

- [使用 AI 创建主题](./theme.md)
- [自动生成 Git 提交信息](./commands.md#auto-git-commit-message)

## 故障排除

如果遇到问题：

1. 验证您的 API 密钥是否正确且处于激活状态
2. 检查您的网络连接
3. 确保您的 API 密钥具有足够的权限
4. 如果使用自定义主机，请验证端点 URL 是否正确
