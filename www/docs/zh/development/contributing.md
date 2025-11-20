# 贡献指南

感谢您有兴趣为 Wox 做贡献！本文档提供了为项目做贡献的指南和说明。

## 快速开始

1. **Fork 仓库**: 首先在 GitHub 上 Fork [Wox 仓库](https://github.com/Wox-launcher/Wox)。

2. **克隆您的 Fork**: 将您的 Fork 克隆到本地机器。

   ```bash
   git clone https://github.com/YOUR-USERNAME/Wox.git
   cd Wox
   ```

3. **设置开发环境**: 按照 [开发环境搭建](./setup.md) 文档中的说明设置您的开发环境。
   ```bash
   make dev
   ```

## 开发工作流

### 分支策略

- `master`: 包含最新稳定代码的主分支
- `feature/*`: 新功能的功能分支
- `bugfix/*`: 错误修复的修复分支

### 进行更改

1. **创建分支**: 为您的更改创建一个新分支。

   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **进行更改**: 实现您的更改，遵循编码标准和指南。

3. **测试您的更改**: 运行测试以确保您的更改不会破坏现有功能。

   ```bash
   make test
   ```

4. **提交您的更改**: 使用清晰且描述性的提交信息提交您的更改。

   ```bash
   git commit -m "feat: add new feature"
   ```

   请遵循 [Conventional Commits](https://www.conventionalcommits.org/) 规范编写提交信息：

   - `feat`: 新功能
   - `fix`: 错误修复
   - `docs`: 仅文档更改
   - `style`: 不影响代码含义的更改
   - `refactor`: 既不修复错误也不添加功能的代码更改
   - `perf`: 提高性能的代码更改
   - `test`: 添加缺失的测试或更正现有的测试
   - `chore`: 对构建过程或辅助工具的更改

5. **推送您的更改**: 将您的更改推送到您的 Fork。

   ```bash
   git push origin feature/your-feature-name
   ```

6. **创建 Pull Request**: 从您的分支创建一个 Pull Request 到主 Wox 仓库。

## Pull Request 指南

创建 Pull Request 时，请：

1. **提供清晰的描述**: 描述您的更改内容以及为什么要包含这些更改。
2. **引用相关 Issue**: 如果您的 PR 修复了一个 Issue，请使用 GitHub Issue 编号引用它。
3. **包含测试**: 如果您的更改包含新功能，请包含覆盖新代码的测试。
4. **更新文档**: 如果您的更改需要更新文档，请在 PR 中包含这些更新。

## 代码风格指南

### Go 代码

- 遵循 [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- 使用 `gofmt` 格式化您的代码
- 编写有意义的注释和文档

### Flutter/Dart 代码

- 遵循 [Dart Style Guide](https://dart.dev/guides/language/effective-dart/style)
- 使用 `dart format` 格式化您的代码
- 编写有意义的注释和文档

### JavaScript/TypeScript 代码

- 遵循 [Airbnb JavaScript Style Guide](https://github.com/airbnb/javascript)
- 使用 ESLint 检查您的代码
- 编写有意义的注释和文档

### Python 代码

- 遵循 [PEP 8](https://www.python.org/dev/peps/pep-0008/)
- 使用 `black` 格式化您的代码
- 编写有意义的注释和文档

## 测试

- 为您的代码编写单元测试
- 运行现有测试以确保您的更改不会破坏现有功能
- 考虑为复杂功能添加集成测试

## 文档

- 更新现有功能更改的文档
- 为新功能添加文档
- 使用清晰简洁的语言

## 社区

- 加入 [Wox Discussions](https://github.com/Wox-launcher/Wox/discussions) 提问和获取帮助
- 尊重并体谅他人

## 报告问题

如果您发现错误或有功能请求，请：

1. 检查 [GitHub Issues](https://github.com/Wox-launcher/Wox/issues) 中是否已存在该问题
2. 如果不存在，请创建一个新 Issue，并提供清晰的描述和重现步骤

## 许可证

通过为 Wox 做贡献，您同意您的贡献将根据项目的许可证进行许可。
