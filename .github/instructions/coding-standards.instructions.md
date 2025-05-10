---
applyTo: '**'
---

# Wox Project Structure

Wox is a cross-platform quick launcher application, consisting of the following main components:

## Core Components

- [wox.core](mdc:wox.core/main.go): Wox backend implemented in Go, communicating with the frontend via websocket and http
  - [wox.core/setting](mdc:wox.core/setting): Settings-related definitions
  - [wox.core/plugin](mdc:wox.core/plugin): Plugin API definitions and implementations

## Plugin System

- [wox.plugin.python](mdc:wox.plugin.python/src/wox_plugin/__init__.py): Library required for Python plugins
- [wox.plugin.host.python](mdc:): Host for Python plugins, communicating with wox.core via websocket, responsible for loading Python plugins
- [wox.plugin.nodejs](mdc:): Library required for NodeJS plugins
- [wox.plugin.host.nodejs](mdc:): Host for NodeJS plugins, communicating with wox.core via websocket, responsible for loading NodeJS plugins

## Frontend Interface

- [wox.ui.flutter](mdc:wox/wox/wox.ui.flutter/lib/main.dart): Wox frontend implemented in Flutter, communicating with wox.core via websocket


# Wox Project General Coding Standards

## General Guidelines

* Avoid redundant comments, add necessary explanations only for complex logic
* Variables, functions, and class names should be descriptive, clearly expressing their purpose
* Keep code concise, avoid unnecessary complexity

## Naming Conventions

* **Go code**: Use camelCase, e.g., `pluginManager`, public methods start with an uppercase letter
* **Python code**: Use snake_case, e.g., `plugin_manager`, class names use CamelCase with the first letter capitalized
* **Dart code**: Use camelCase, class names start with an uppercase letter
* **File names**: Use lowercase letters and underscores, e.g., `plugin_manager.py`

## Error Handling

* All potential error locations must have appropriate error handling
* Error messages should be clear and understandable, helping to understand the problem
* Avoid swallowing exceptions, ensure errors are properly logged or handled

## Version Control

* Run relevant tests before committing to ensure code works properly
* Commit messages should clearly express the content of changes
* Each commit should focus on a single feature or fix
* Follow the project's branch management strategy

## Internationalization

* All user-facing text should support internationalization
* Use the project's provided internationalization tools, don't hardcode text
* Test appearance in different language environments
