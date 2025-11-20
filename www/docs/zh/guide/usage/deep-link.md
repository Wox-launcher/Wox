# Deep Link (深度链接)

Wox 支持深度链接，允许您从外部应用程序或脚本触发 Wox 查询。

## URL Scheme

Wox 的 URL Scheme 是 `wox://`。

## 格式

```
wox://query?q=<query>
```

- `q`：您想要执行的查询字符串。它应该是 URL 编码的。

## 示例

### 使用预填充查询打开 Wox

```
wox://query?q=wpm%20install
```

这将打开 Wox 并在搜索框中输入 `wpm install`。

### 触发特定插件

```
wox://query?q=calc%201%2B1
```

这将打开 Wox 并使用计算器插件计算 `1+1`（假设 `calc` 是触发关键字）。

## 在脚本中使用

您可以在 Shell 脚本或其他自动化工具中使用深度链接。

**macOS:**

```bash
open "wox://query?q=test"
```

**Windows:**

```powershell
start "wox://query?q=test"
```

**Linux:**

```bash
xdg-open "wox://query?q=test"
```
