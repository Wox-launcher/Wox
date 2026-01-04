# Browser Bookmark 插件

Browser Bookmark 插件提供浏览器书签的快速搜索和访问功能。

## 功能特点

- **多浏览器支持**: 支持 Chrome、Edge 等主流浏览器
- **多配置文件**: 支持多个浏览器配置文件
- **自动同步**: 自动读取浏览器书签
- **模糊搜索**: 支持书签名称和 URL 搜索
- **Favicon 缓存**: 缓存网站图标，提升显示速度
- **MRU 功能**: 记住常用书签，优先显示

## 基本使用

### 搜索书签

1. 打开 Wox
2. 直接输入书签名称或 URL 的一部分
3. 按 Enter 键打开书签

```
Wox Launcher
→ 搜索包含 "Wox Launcher" 的书签

github
→ 搜索所有包含 "github" 的书签

https://github.com
→ 搜索匹配该 URL 的书签
```

### 严格匹配

Browser Bookmark 插件使用较严格的匹配评分（最低 50 分），避免显示过多无关结果：
- 需要书签名称或 URL 与搜索词高度匹配
- URL 匹配必须是精确的部分匹配

## 支持的浏览器

### Windows

- Google Chrome
- Microsoft Edge

**书签位置**:
- Chrome: `%LOCALAPPDATA%\Google\Chrome\User Data\Default\Bookmarks`
- Edge: `%LOCALAPPDATA%\Microsoft\Edge\User Data\Default\Bookmarks`

支持多个配置文件：Default, Profile 1, Profile 2, Profile 3

### macOS

- Google Chrome
- Microsoft Edge

**书签位置**:
- Chrome: `~/Library/Application Support/Google/Chrome/Default/Bookmarks`
- Edge: `~/Library/Application Support/Microsoft Edge/Default/Bookmarks`

支持多个配置文件：Default, Profile 1, Profile 2, Profile 3

### Linux

- Google Chrome
- Microsoft Edge

**书签位置**:
- Chrome: `~/.config/google-chrome/Default/Bookmarks`
- Edge: `~/.config/microsoft-edge/Default/Bookmarks`

支持多个配置文件：Default, Profile 1, Profile 2, Profile 3

### Safari

目前不支持 Safari 书签。如果需要，可以提交 issue 反馈。

## 书签图标

### Favicon 缓存

Browser Bookmark 插件会自动预取和缓存网站图标：
- 启动时在后台预取
- 避免实时加载影响性能
- 使用缓存文件快速显示

### 图标叠加

显示书签时：
- 基础图标为书签图标
- 如果有缓存 favicon，叠加在基础图标上
- 叠加位置：右下角，大小为 60%

## MRU 功能

### 最近使用

Browser Bookmark 插件支持 MRU（最近使用）功能：
- 经常访问的书签会优先显示
- 基于 MRU 数据进行智能排序
- 可以快速找到常用书签

### 使用频率

MRU 会跟踪：
- 书签的访问次数
- 最后访问时间
- 计算推荐分数

## 搜索技巧

### 书签名称搜索

使用书签的完整名称或部分名称：

```
Wox GitHub
→ Wox Launcher GitHub 仓库

Python Docs
→ Python 官方文档
```

### URL 搜索

输入 URL 的一部分来匹配书签：

```
github.com/wox
→ Wox Launcher GitHub 仓库

stackoverflow.com
→ StackOverflow 书签
```

### 模糊匹配

即使不完全准确也能找到书签：
- 支持拼写错误
- 支持部分匹配
- 智能评分系统

## 常见问题

### 为什么搜索不到某个书签？

1. **检查浏览器**: 确认书签在支持的浏览器中
2. **检查配置文件**: 某些书签可能在非默认配置文件中
3. **等待同步**: 浏览器正在同步书签时，Wox 可能读取不到
4. **重启 Wox**: 有时需要重启才能重新加载书签

### 如何添加新书签？

在浏览器中正常添加书签，Wox 会自动读取：
- 确保浏览器已关闭书签文件（某些浏览器需要）
- 重启 Wox 或等待自动重载

### 支持其他浏览器吗？

目前仅支持 Chrome 和 Edge。如需支持其他浏览器：
- 提交 GitHub Issue 反馈
- 或考虑使用第三方插件

### Favicon 不显示？

1. **等待预取**: 首次启动时需要预取，可能需要几秒
2. **检查网络**: Favicon 预取需要网络连接
3. **检查缓存**: 查看 Wox 缓存目录，确认 favicon 文件存在

### 书签重复怎么办？

插件会自动去重：
- 相同名称和 URL 的书签只保留一个
- 去重在加载时自动进行
- 不影响原始书签文件

## 配置选项

Browser Bookmark 插件目前没有用户可配置的选项。所有设置都是自动的。

### 自动重载

插件会在以下情况自动重载书签：
- 浏览器书签文件更改时（通过文件监视）
- 重启 Wox 时

### 手动重载

如果需要立即重载书签：
1. 重启 Wox
2. 或关闭浏览器保存书签后再打开

## 使用场景

### 快速访问常用网站

```
github
reddit
hacker news
```

### 打开工作相关书签

```
company docs
project tracker
jira
```

### 开发者工具

```
stackoverflow
mdn docs
can i use
```

## 相关插件

- [WebSearch](websearch.md) - 网页搜索
- [Application](application.md) - 启动浏览器
- [Clipboard](clipboard.md) - 剪贴板历史，方便粘贴 URL

## 技术说明

Browser Bookmark 插件：

**书签读取**:
- 直接读取浏览器的书签 JSON 文件
- 支持多个浏览器配置文件
- 自动去重

**图标缓存**:
- 启动时后台预取 favicon
- 使用独立缓存文件
- 支持图标叠加显示

**搜索匹配**:
- 书签名称：使用模糊匹配
- URL：使用精确部分匹配
- 最低匹配分数为 50，过滤低分结果

**MRU**:
- 使用全局 MRU 系统
- 按访问频率和时间排序
- 支持快速访问常用书签
