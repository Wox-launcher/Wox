# 深度链接

深度链接让其他应用或脚本通过 `wox://` URL 调用 Wox，例如填入查询、打开安装界面或调用插件。可用于终端命令、自动化工具和应用内按钮。

| 链接 | 作用 |
| --- | --- |
| `wox://query` | 在主窗口中填入查询 |
| `wox://install` | 打开本地插件包或主题包的安装界面 |
| `wox://toggle` | 显示或隐藏主窗口 |
| `wox://confetti` | 播放撒花动画 |
| `wox://plugin/<plugin-id>` | 调用指定插件的深度链接回调 |

## query：填入查询

### 作用

将指定文本填入主窗口并显示查询结果，不会自动执行某条结果。此链接复用主窗口，不会新建副窗口。

### 格式与参数

```text
wox://query?q=<经过 URL 编码的查询文本>
```

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `q` | 是 | 非空查询文本，与手动输入 Wox 的内容相同。需要对参数值进行 URL 编码。 |

### 示例

搜索名称包含 `invoice` 的文件，查询文本为 `f invoice`：

```text
wox://query?q=f%20invoice
```

计算 `100 + 20`，其中加号编码为 `%2B`：

```text
wox://query?q=100%20%2B%2020
```

打开插件管理器的安装查询，保留 `wpm install ` 末尾的空格：

```text
wox://query?q=wpm%20install%20
```

## install：安装插件或主题

### 作用

打开本地 `.wox` 插件包或 `.wox-theme` 主题包的安装界面，由用户确认安装、升级或重新安装。注册文件关联后，双击这两类文件也会走此入口。

安装界面复用主窗口，并在焦点返回文件管理器时保持可见。

### 格式与参数

```text
wox://install?path=<经过 URL 编码的文件路径>
```

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `path` | 是 | 本地插件包或主题包的绝对路径，也可使用 `file://` URL。整个参数值需要进行 URL 编码。 |

### 示例

安装 Windows 上的 `C:\Users\demo\Downloads\example.wox`：

```text
wox://install?path=C%3A%5CUsers%5Cdemo%5CDownloads%5Cexample.wox
```

安装 macOS 上的 `/Users/demo/Downloads/example.wox-theme`：

```text
wox://install?path=%2FUsers%2Fdemo%2FDownloads%2Fexample.wox-theme
```

请将示例路径替换为实际存在的文件路径。

## toggle：显示或隐藏主窗口

### 作用

切换主窗口的可见状态：隐藏时显示，显示时隐藏。适合绑定到外部快捷键或自动化按钮。

### 格式与参数

```text
wox://toggle
```

无参数。

### 示例

在 PowerShell 中切换 Wox 主窗口：

```powershell
Start-Process "wox://toggle"
```

## confetti：播放撒花动画

### 作用

在鼠标所在屏幕播放撒花动画，不打开启动器或抢占焦点。动画约六秒后结束，重复调用会重新播放。

### 格式与参数

```text
wox://confetti
```

无参数。

### 示例

在 macOS 终端中播放动画：

```bash
open "wox://confetti"
```

## plugin：调用插件

### 作用

将链接中的查询参数传给指定插件注册的深度链接回调。插件必须已安装并支持此回调；具体操作、参数含义以及是否打开界面均由插件决定。

### 格式与参数

```text
wox://plugin/<plugin-id>?<参数名>=<经过 URL 编码的参数值>
```

| 字段 | 说明 |
| --- | --- |
| `<plugin-id>` | 插件元数据中的 `ID`，不是插件显示名称。 |
| 查询参数 | 由插件定义。多个参数用 `&` 分隔，每个参数值分别进行 URL 编码。 |

### 示例

假设某插件的 ID 为 `12345678-1234-1234-1234-123456789abc`，并支持通过 `action=open` 和 `id` 打开条目：

```text
wox://plugin/12345678-1234-1234-1234-123456789abc?action=open&id=123
```

该插件的回调会收到 `action` 为 `open`、`id` 为 `123` 的参数。这里的 ID 和参数仅作示意，使用时应按目标插件的文档替换。

## 在脚本中打开链接

下面三种命令都将 `f invoice` 填入 Wox。将 URL 替换为其他深度链接即可调用对应功能。

### Windows PowerShell

```powershell
Start-Process "wox://query?q=f%20invoice"
```

### macOS

```bash
open "wox://query?q=f%20invoice"
```

### Linux

```bash
xdg-open "wox://query?q=f%20invoice"
```

## 参数编码

对参数值进行 URL 编码，保留链接结构中的 `://`、`?`、`=` 和参数之间的 `&`。

| 参数值中的字符 | 编码 |
| --- | --- |
| 空格 | `%20` |
| `+` | `%2B` |
| `&` | `%26` |
| `#` | `%23` |

例如，查询文本 `A&B` 对应 `wox://query?q=A%26B`。动态生成链接时，应使用语言提供的 URL 编码函数，避免手动拼接未经编码的文本。

不要把密钥放进深度链接；URL 可能被 shell、浏览器或自动化日志记录。
