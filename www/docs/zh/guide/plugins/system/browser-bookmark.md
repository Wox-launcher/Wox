# 浏览器书签插件

浏览器书签是全局插件。输入书签标题或 URL 的一部分，Wox 就可以打开匹配的页面。只想看书签结果时，可以使用 `b`。

![浏览器书签插件](/images/guide/plugin_bookmark.png)

## 快速开始

```text
github
docs
wox launcher
github.com/Wox-launcher
b github
```

插件的匹配比普通文本搜索更严格，避免书签结果刷满每一次查询。

## 支持的浏览器

| 浏览器 | 说明 |
| --- | --- |
| Chrome | 读取常见 profile，例如 `Default`、`Profile 1`、`Profile 2`、`Profile 3`。 |
| Edge | 读取 Windows、macOS 和 Linux 上的常见 profile。 |
| Firefox | 读取 Firefox profile 目录和 `places.sqlite`。 |

目前不会索引 Safari 书签。

Linux 上，Firefox 支持 `~/.mozilla/firefox` 和 `$XDG_CONFIG_HOME/mozilla/firefox`（默认是 `~/.config/mozilla/firefox`），以及 Snap 和 Flatpak 数据目录。Chrome 支持 `$XDG_CONFIG_HOME/google-chrome` 和 Flatpak 的 `~/.var/app/com.google.Chrome/config/google-chrome`。即使浏览器可执行文件不在宿主机的 PATH 中，插件也能通过这些数据目录发现并索引书签。

## 设置

打开 **设置 -> 插件 -> 浏览器书签**，选择 Wox 应该索引哪些浏览器。如果重复书签很多，只保留你真正在用的浏览器。

Wox 启动时或插件设置中的浏览器选择发生变化时会重新加载书签。浏览器书签文件变化后，可以重启 Wox 以读取更新。Firefox 书签通过 profile 数据库及其事务文件的临时副本读取，无需关闭 Firefox。

## 图标和排序

Wox 会在后台预取书签 favicon 并缓存。经常打开的书签会通过 MRU 逐渐靠前。

## 排查

### 书签缺失

- 确认插件设置里启用了对应浏览器。
- 确认书签在受支持的 profile 中。
- 如果浏览器刚同步或重写了书签数据库，重启 Wox。

### 出现重复书签

插件会去掉标题和 URL 完全相同的重复项。来自不同 profile 或不同 URL 的相似书签会保留。

## 相关插件

- [网页搜索](websearch.md) 用于搜索网页
- [应用](application.md) 用于启动浏览器
