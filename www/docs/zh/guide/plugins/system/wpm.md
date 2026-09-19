# 插件管理器

插件管理器负责安装、更新和查看插件。触发关键字是 `wpm`、`store` 和 `pm`。

![插件管理器](/images/plugin_wpm.jpg)

## 快速开始

```text
wpm
wpm install
wpm install everything
```

| 查询 | 用途 |
| --- | --- |
| `wpm` | 浏览已安装插件和商店插件 |
| `wpm install <name>` | 搜索商店并安装 |
| 双击 `.wox` 文件 | 打开同一个本地安装器 |

动作可以安装、更新、卸载、打开插件网站，或复制用于创建插件的 AI prompt。也可以从模板创建单文件插件或全功能插件。

Node.js 和 Python 插件需要对应运行时。如果刚安装了 host，可以在插件管理器设置里刷新运行时。

见 [插件商店](/zh/store/plugins) 和 [如何创建插件](/zh/development/)。
