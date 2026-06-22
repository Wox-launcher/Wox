# 深度链接

深度链接允许其他应用或脚本用预设查询打开 Wox。它适合终端 alias、浏览器快捷入口、自动化工具和应用内按钮。

## 格式

```text
wox://query?q=<url-encoded-query>
```

`q` 的值就是 Wox 要放入查询框的文本。空格和特殊符号需要 URL 编码。

## 示例

打开插件管理器安装查询：

```text
wox://query?q=wpm%20install
```

打开文件搜索：

```text
wox://query?q=f%20invoice
```

执行计算：

```text
wox://query?q=100%20%2B%2020
```

启动 AI 对话：

```text
wox://query?q=chat%20summarize%20this
```

## 在脚本中使用

macOS：

```bash
open "wox://query?q=f%20invoice"
```

Windows PowerShell：

```powershell
Start-Process "wox://query?q=f%20invoice"
```

Linux：

```bash
xdg-open "wox://query?q=f%20invoice"
```

## 编码注意事项

- 空格编码为 `%20`。
- 计算表达式里的 `+` 需要编码为 `%2B`。
- 不要把密钥放进 deep link；URL 可能被 shell、浏览器或自动化日志记录。
