# 插件管理器

插件管理器负责安装、更新和查看插件。触发关键字是 `wpm`、`store` 和 `pm`。

![插件管理器](/images/guide/plugin_wpm.jpg)

## 快速开始

```text
wpm
wpm install
wpm install everything
```

| 查询 | 用途 |
| --- | --- |
| `wpm` | 列出安装、更新、卸载和创建命令 |
| `wpm install` | 浏览商店插件 |
| `wpm install <name>` | 搜索商店并安装 |
| `wpm update` | 列出可更新插件，可单独更新或全部更新 |
| `wpm uninstall` | 列出已安装插件以供卸载 |
| `wpm create` | 从模板创建插件 |
| 双击 `.wox` 文件 | 打开同一个本地安装器 |

动作可以安装、更新、卸载、打开插件网站，或复制用于创建插件的 AI prompt。也可以从模板创建单文件插件或全功能插件。

Node.js 和 Python 插件需要对应运行时。如果刚安装了解释器，可以打开 **设置 -> 插件 -> 运行时设置** 刷新状态，或配置其可执行文件路径。

见 [插件商店](/zh/store/plugins) 和 [如何创建插件](/zh/development/)。
