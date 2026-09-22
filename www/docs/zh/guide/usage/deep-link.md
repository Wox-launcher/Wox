# 深度链接

深度链接允许其他应用或脚本用预设查询打开 Wox。它适合终端 alias、浏览器快捷入口、自动化工具和应用内按钮。

## 格式

```text
wox://query?q=<url-encoded-query>
wox://install?path=<url-encoded-file>
wox://confetti
```

`q` 的值就是 Wox 要放入查询框的文本。空格和特殊符号需要 URL 编码。

`wox://install?path=` 会打开本地 `.wox` 插件安装界面。双击 `.wox` 文件时，Wox 在注册文件关联后走同一条路径。

## 示例

在鼠标所在屏幕播放撒花动画，不打开启动器或抢占焦点。动画约六秒后结束，重复调用会重新播放：

<WoxRunQuery href="wox://confetti" />

打开插件管理器安装查询：

<WoxRunQuery query="wpm install " />

打开文件搜索：

<WoxRunQuery query="f invoice" />

执行计算：

<WoxRunQuery query="100 + 20" />

启动 AI 对话：

<WoxRunQuery query="chat summarize this" />

新建笔记：

<WoxRunQuery query="note new " />

打开已下载的插件包安装界面：

<WoxRunQuery href="wox://install?path=/Users/demo/Downloads/wox.plugin.example.wox" />

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
