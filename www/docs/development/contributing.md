# Contributing to Wox

Thank you for your interest in contributing to Wox! This document provides guidelines and instructions for contributing to the project.

## Getting Started

1. **Fork the Repository**: Start by forking the [Wox repository](https://github.com/Wox-launcher/Wox) on GitHub.

2. **Clone Your Fork**: Clone your fork to your local machine.

   ```bash
   git clone https://github.com/YOUR-USERNAME/Wox.git
   cd Wox
   ```

3. **Set Up Development Environment**: Follow the instructions in the [Development Setup](./setup.md) document to set up your development environment.
   ```bash
   make dev
   ```

## Development Workflow

### Branching Strategy

- `master`: The main branch that contains the latest stable code
- `feature/*`: Feature branches for new features
- `bugfix/*`: Bugfix branches for bug fixes

### Making Changes

1. **Create a Branch**: Create a new branch for your changes.

   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make Your Changes**: Implement your changes, following the coding standards and guidelines.

3. **Test Your Changes**: Run tests to ensure your changes don't break existing functionality.

   ```bash
   make test
   ```

4. **Commit Your Changes**: Commit your changes with a clear and descriptive commit message.

   ```bash
   git commit -m "feat: add new feature"
   ```

   Please follow the [Conventional Commits](https://www.conventionalcommits.org/) specification for your commit messages:

   - `feat`: A new feature
   - `fix`: A bug fix
   - `docs`: Documentation only changes
   - `style`: Changes that do not affect the meaning of the code
   - `refactor`: A code change that neither fixes a bug nor adds a feature
   - `perf`: A code change that improves performance
   - `test`: Adding missing tests or correcting existing tests
   - `chore`: Changes to the build process or auxiliary tools

5. **Push Your Changes**: Push your changes to your fork.

   ```bash
   git push origin feature/your-feature-name
   ```

6. **Create a Pull Request**: Create a pull request from your branch to the main Wox repository.

## Pull Request Guidelines

When creating a pull request, please:

1. **Provide a Clear Description**: Describe what your changes do and why they should be included.
2. **Reference Related Issues**: If your PR fixes an issue, reference it using the GitHub issue number.
3. **Include Tests**: If your changes include new functionality, include tests that cover the new code.
4. **Update Documentation**: If your changes require documentation updates, include those in your PR.

## Code Style Guidelines

### Go Code

- Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `gofmt` to format your code
- Write meaningful comments and documentation

### Flutter/Dart Code

- Follow the [Dart Style Guide](https://dart.dev/guides/language/effective-dart/style)
- Use `dart format` to format your code
- Write meaningful comments and documentation

### JavaScript/TypeScript Code

- Follow the [Airbnb JavaScript Style Guide](https://github.com/airbnb/javascript)
- Use ESLint to lint your code
- Write meaningful comments and documentation

### Python Code

- Follow [PEP 8](https://www.python.org/dev/peps/pep-0008/)
- Use `black` to format your code
- Write meaningful comments and documentation

## Testing

- Write unit tests for your code
- Run existing tests to ensure your changes don't break existing functionality
- Consider adding integration tests for complex features

## Documentation

- Update documentation for any changes to existing features
- Add documentation for new features
- Use clear and concise language

## Community

- Join the [Wox Discussions](https://github.com/Wox-launcher/Wox/discussions) to ask questions and get help
- Be respectful and considerate of others

## Reporting Issues

If you find a bug or have a feature request, please:

1. Check if the issue already exists in the [GitHub Issues](https://github.com/Wox-launcher/Wox/issues)
2. If not, create a new issue with a clear description and steps to reproduce

## License

By contributing to Wox, you agree that your contributions will be licensed under the project's license.
