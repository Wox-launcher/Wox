---
applyTo: '**/*.go'
---

# wox.core Go Code Standards

## General Guidelines

* All Go code must pass `go fmt` and `go vet` validation
* Use proper error handling, avoid using `panic`
* Code should have appropriate unit test coverage

## Logging Standards

* Use the project-defined logging utilities for recording logs, reference: [log.go](mdc:wox.core/util/log.go)
* Example usage reference: [manager.go](mdc:wox.core/plugin/manager.go)
* Methods in the wox.core project typically have `context.Context` as their first parameter, using traceId in the context to implement log tracing
* When writing unit tests, be sure to initialize logging, otherwise logs may not be printed to the correct location

## Plugin System

* Plugin API is defined in [plugin/plugin.go](mdc:wox.core/plugin/plugin.go)
* Plugin manager implementation is in [plugin/manager.go](mdc:wox.core/plugin/manager.go)
* All system plugins must implement the `SystemPlugin` interface
* All external plugins must implement the `Plugin` interface
* Externally exposed APIs must be defined in [plugin/api.go](mdc:wox.core/plugin/api.go)

## Settings System

* Settings-related definitions should be placed in the [setting](mdc:wox.core/setting) package
* All settings should have reasonable default values
* Setting changes must trigger appropriate callback notifications

