# 贡献指南

这篇文档介绍在 Wox 仓库里更实用的协作流程。

## 开始之前

1. 在 GitHub 上 Fork [Wox-launcher/Wox](https://github.com/Wox-launcher/Wox)
2. 克隆你的 Fork
3. 按照 [开发环境搭建](./setup.md) 完成初始化

```bash
git clone https://github.com/YOUR-USERNAME/Wox.git
cd Wox
make dev
```

## 在这个仓库里工作的方式

Wox 是一个多项目仓库。一个看起来很小的改动，也可能因为共享协议漂移而影响其他层。所以做改动时尽量遵循这三个原则：

- 一次只改一个明确行为
- 在改动影响到的最高层做验证
- 用户可见行为、API、工作流变了就同步更新文档

## 典型工作流

1. 从 `master` 创建分支

```bash
git checkout -b feature/your-change
```

2. 在正确的层里修改代码

- `wox.core/`：后端逻辑、内置插件、设置、共享契约
- `wox.ui.flutter/wox/`：启动器 UI、设置页、截图 UI、平台展示逻辑
- `wox.plugin.host.*`：插件宿主桥接行为
- `wox.plugin.*`：对外插件 SDK
- `www/docs/`：文档站

3. 开发过程中做针对性验证

例如：

```bash
make -C wox.core build
make -C wox.plugin.host.nodejs build
make -C wox.ui.flutter/wox build
```

4. 提交 PR 前做更广的验证

```bash
make build
```

如果你改的是用户可见的桌面流程，再加上：

```bash
make smoke
```

## 测试建议

先用最小验证证明改动正确，再补上与你改动层级匹配的更高层验证。

常用命令：

```bash
make test
make smoke
make build
```

一般可以这样理解：

- `make test`：默认的后端回归检查
- `make smoke`：适合启动器、截图、设置等真实 UI 流程
- `make build`：适合共享契约或跨项目改动的最终守门

## 提交信息

使用 [Conventional Commits](https://www.conventionalcommits.org/)：

- `feat`
- `fix`
- `docs`
- `refactor`
- `perf`
- `test`
- `chore`

例如：

```bash
git commit -m "feat(plugin): add screenshot API"
git commit -m "fix(webview): restore open in browser action"
git commit -m "docs(development): refresh contributor setup guide"
```

## Pull Request 建议

一个好评审的 PR，至少应该做到：

- 解释清楚行为变化，而不是只列文件
- 有关联 issue 或 discussion 时附上链接
- 写明你怎么验证的
- UI 改动附上截图或录屏
- 工作流、API、可见行为变化时同步更新文档

## 代码风格

优先遵循仓库现有约定：

- Go：`gofmt`
- Dart：`dart format`
- TypeScript / JavaScript：使用仓库现有规则
- Python：使用仓库现有格式化和风格约定

尽量让控制流直接、职责归位，不要把行为放到不拥有它的层里。

## 文档修改

文档源文件在 `www/docs`。

本地预览方式：

```bash
cd www
pnpm install
pnpm docs:dev
```

如果你改了命令、API、安装/开发流程或插件行为，最好在同一个 PR 里把对应文档一起改掉。
