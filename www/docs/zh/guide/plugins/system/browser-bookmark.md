# 浏览器书签插件

浏览器书签是全局插件。输入书签标题或 URL 的一部分，Wox 就可以打开匹配页面。

## 快速开始

```text
github
docs
wox launcher
github.com/Wox-launcher
```

该插件的匹配比普通文本搜索更严格，避免书签结果淹没每一次查询。

![浏览器书签插件结果列表](/images/system-plugin-bookmark.png)

## 支持的浏览器

| 浏览器 | 说明 |
| --- | --- |
| Chrome | 读取 `Default`、`Profile 1`、`Profile 2`、`Profile 3` 等常见 profile。 |
| Edge | 在 Windows、macOS、Linux 上读取常见 profile。 |
| Firefox | 读取 Firefox profile 目录和 `places.sqlite`。 |

目前不会索引 Safari 书签。

## 设置

打开 **设置 -> 插件 -> 浏览器书签**，选择要索引的浏览器。如果你有很多重复书签，只保留实际使用的浏览器即可。

## 图标和排序

Wox 会在后台预取书签 favicon 并缓存。经常打开的书签会通过 MRU 逐渐靠前。

## 排查

### 某个书签搜不到

- 确认该浏览器已在插件设置中启用。
- 确认书签在受支持的 profile 中。
- 如果浏览器刚同步或改写书签数据库，重启 Wox。
- Firefox 如果 profile 数据库被锁，先关闭一次 Firefox。

### 出现重复书签

插件会去掉标题和 URL 都相同的精确重复项。来自不同 profile 或 URL 不同的相似书签会保留，因为 Wox 无法判断你要保留哪一个。

### Favicon 不显示

Favicon 在后台加载，需要网络访问。图标缓存还没完成时，书签仍然可以正常打开。
