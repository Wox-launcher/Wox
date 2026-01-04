# File 插件

File 插件提供快速文件搜索功能，支持跨平台的高效文件索引。

## 功能特点

- **全局搜索**: 搜索系统中的所有文件
- **快速索引**: 使用操作系统提供的快速索引服务
- **防抖搜索**: 避免频繁搜索，提升性能
- **文件操作**: 打开、删除、显示右键菜单等

## 基本使用

### 搜索文件

1. 打开 Wox
2. 直接输入文件名或部分文件名
3. 查看搜索结果

```
report.pdf
→ 搜索所有包含 "report.pdf" 的文件

README
→ 搜索所有包含 "README" 的文件
```

### 使用触发关键词

使用 `f` 触发文件搜索：

```
f project
→ 搜索包含 "project" 的文件
```

## 文件操作

找到文件后，可以执行以下操作：

### 打开文件

按 `Enter` 键或选择"打开"操作，使用默认程序打开文件。

### 打开所在文件夹

按 `Ctrl+Enter` 键或选择"打开所在文件夹"操作，在文件管理器中打开文件所在的目录。

### 删除文件

选择"删除"操作，将文件移动到回收站（而不是永久删除）。

### 显示右键菜单

按 `Ctrl+M` 键或选择"显示右键菜单"操作，显示系统右键菜单，可以执行更多操作。

## 平台特性

### Windows

Windows 使用 Everything 作为文件索引引擎：

- **速度快**: Everything 是 Windows 上最快的文件搜索工具
- **实时更新**: 文件更改实时反映在搜索结果中
- **低资源占用**: 几乎不占用系统资源

**安装 Everything**:
1. 访问 [Everything 官网](https://www.voidtools.com/)
2. 下载并安装 Everything
3. 启动 Everything（确保服务运行）
4. Wox 会自动连接到 Everything

**常见问题**:

如果看到 "Everything 未运行" 提示：
1. 确认 Everything 已安装并运行
2. 检查 Everything 设置，确保启用了 HTTP 服务
3. 重启 Wox

### macOS

macOS 使用 Spotlight 索引：
- **系统集成**: 使用 macOS 自带的 Spotlight 索引
- **无需额外软件**: 不需要安装其他工具
- **自动更新**: 文件更改自动同步到索引

**注意事项**:
- 某些系统文件夹可能需要权限才能访问
- 外部驱动器的文件可能不会立即出现在搜索中
- 可以在系统设置中配置 Spotlight 索引范围

### Linux

Linux 支持多种文件索引引擎：
- 使用系统默认的文件索引（如 `locate`）
- 可能需要额外配置才能获得最佳性能

## 搜索技巧

### 模糊匹配

File 插件支持模糊匹配，不需要输入完整的文件名：

```
rep.pdf
→ 可能匹配: report.pdf, representation.pdf

readme
→ 可能匹配: README.md, readme.txt
```

### 按扩展名搜索

输入文件扩展名可以搜索特定类型的文件：

```
.pdf
→ 搜索所有 PDF 文件

.jpg
→ 搜索所有 JPEG 图片
```

### 路径搜索

如果知道文件的大致位置，可以输入路径的一部分：

```
Documents/report
→ 搜索 Documents 文件夹中包含 "report" 的文件
```

## 配置选项

File 插件目前没有用户可配置的选项。所有设置都是自动的。

## 常见问题

### 为什么搜索不到某个文件？

1. **检查索引**:
   - Windows: 确保 Everything 正在运行
   - macOS: 检查 Spotlight 设置，确认文件被索引
   - Linux: 确认文件索引服务正在运行

2. **等待索引更新**: 新创建的文件可能需要几秒钟才能被索引

3. **检查文件权限**: 确认你有权限访问该文件

4. **重启 Wox**: 有时需要重启才能重新连接到索引服务

### 搜索速度很慢怎么办？

Windows 用户：
- 确保使用最新版本的 Everything
- 在 Everything 设置中禁用不必要的索引

macOS/Linux 用户：
- 检查磁盘性能
- 清理系统缓存

### 可以搜索网络驱动器吗？

Windows:
- Everything 可以索引网络驱动器，但需要在设置中启用
- 网络驱动器的搜索速度可能较慢

macOS:
- Spotlight 默认不索引网络驱动器
- 需要手动挂载并等待索引

### 搜索结果很多，如何筛选？

1. **输入更具体的文件名**
2. **使用文件扩展名**
3. **组合关键词**

## 使用场景

### 快速打开文档

```
contract.pdf
project-plan.docx
presentation.pptx
```

### 查找代码文件

```
main.go
app.tsx
utils.py
```

### 打开配置文件

```
config.json
.env
settings.yaml
```

## 相关插件

- [Explorer](explorer.md) - 文件夹导航
- [Application](application.md) - 应用启动
- [Clipboard](clipboard.md) - 剪贴板历史，方便复制文件路径

## 技术说明

File 插件使用平台特定的搜索后端：

| 平台 | 搜索后端 | 说明 |
|-----|----------|------|
| Windows | Everything SDK | 最快的文件搜索工具 |
| macOS | Spotlight/Meta | 系统内置索引 |
| Linux | locate/mlocate | 传统文件索引 |

防抖设置为 500ms，避免频繁搜索导致性能问题。
