# Plugin Screenshot API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a narrowed screenshot API for third-party plugins.

**Architecture:** Keep the existing Go-to-Flutter screenshot protocol private. Add a plugin API adapter in core, route it through the websocket plugin host, and expose typed helpers in the Node and Python SDKs.

**Tech Stack:** Go core, websocket JSON-RPC plugin host, TypeScript Node SDK, Python SDK.

---

### Task 1: Core Plugin API

**Files:**
- Modify: `wox.core/plugin/api.go`

- [ ] Add `ScreenshotOption` and `ScreenshotResult` structs near `CopyParams`.
- [ ] Add `Screenshot(ctx context.Context, option ScreenshotOption) ScreenshotResult` to `API`.
- [ ] Implement `APIImpl.Screenshot` by calling `GetPluginManager().GetUI().CaptureScreenshot`.
- [ ] Map internal completed/cancelled/failed statuses into the public result.

### Task 2: Websocket Host Bridge

**Files:**
- Modify: `wox.core/plugin/host/host_websocket.go`

- [ ] Add a `Screenshot` request case.
- [ ] Decode the optional `option` JSON param into `plugin.ScreenshotOption`.
- [ ] Return `pluginInstance.API.Screenshot(ctx, option)` with `sendResponseToHost`.

### Task 3: TypeScript SDK

**Files:**
- Modify: `wox.plugin.nodejs/types/index.d.ts`
- Modify: `wox.plugin.host.nodejs/src/pluginAPI.ts`

- [ ] Add `ScreenshotOption` and `ScreenshotResult` interfaces.
- [ ] Add `Screenshot(ctx, option)` to `PublicAPI`.
- [ ] Implement `PluginAPI.Screenshot` by JSON serializing the option and returning the narrowed result.

### Task 4: Python SDK

**Files:**
- Modify: `wox.plugin.python/src/wox_plugin/api.py`
- Modify: `wox.plugin.host.python/src/wox_plugin_host/plugin_api.py`

- [ ] Add public dataclasses for `ScreenshotOption` and `ScreenshotResult`.
- [ ] Add `screenshot(ctx, option)` to `PublicAPI`.
- [ ] Implement `PluginAPI.screenshot` by sending the new `Screenshot` method and parsing dict responses into `ScreenshotResult`.

### Task 5: Verification

**Files:**
- Modify only formatting results if needed.

- [ ] Format touched Go files with `gofmt`.
- [ ] Run `make build` in `wox.core`.
- [ ] Report any skipped broader UI smoke tests clearly.
