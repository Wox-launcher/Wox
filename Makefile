.PHONY: build clean host _bundle_mac_app plugins help dev sdk _update_sdk_versions _sync_sdk_versions test test-all test-calculator test-converter test-plugin test-time test-network test-quick test-legacy only_test check_deps release appimage smoke www

SMOKE_FILTER := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
SQLITE_BUILD_TAGS ?= sqlite_fts5
FLUTTER_MASTER_COMMIT_FILE ?= .flutter-master-commit
FLUTTER_MASTER_COMMIT ?= $(strip $(file <$(FLUTTER_MASTER_COMMIT_FILE)))

# GNU Make on Windows may choose Git's sh.exe without exposing Git usr/bin to
# recipes or $(shell ...) calls. The root build relies on sed/rm/uname before
# dependency checks run, so normalize PATH here instead of requiring callers to
# launch from a preconfigured MINGW64 shell.
ifeq ($(OS),Windows_NT)
    GIT_USR_BIN := $(patsubst %/bin/sh.exe,%/usr/bin,$(SHELL))
    ifneq ($(GIT_USR_BIN),$(SHELL))
        export PATH := $(GIT_USR_BIN);$(PATH)
    endif
endif

# The previous build always preferred Corepack when the shim existed, but some
# Node/Corepack installs expose the command while `corepack pnpm` still fails at
# runtime. Prefer a working global pnpm first, then fall back to a working
# Corepack shim so dependency checks and nested builds choose an executable CLI.
PNPM ?= $(shell if command -v pnpm >/dev/null 2>&1 && pnpm --version >/dev/null 2>&1; then echo pnpm; elif command -v corepack >/dev/null 2>&1 && corepack pnpm --version >/dev/null 2>&1; then echo "corepack pnpm"; else echo pnpm; fi)
export PNPM

CURRENT_NODEJS_SDK_VERSION := $(shell node -p "require('./wox.plugin.nodejs/package.json').version")
CURRENT_PYTHON_SDK_VERSION := $(shell sed -n 's/^version = "\(.*\)"/\1/p' wox.plugin.python/pyproject.toml)
NEXT_NODEJS_SDK_VERSION := $(shell node -e "const parts='$(CURRENT_NODEJS_SDK_VERSION)'.split('.').map(Number); if (parts.length !== 3 || parts.some(Number.isNaN)) process.exit(1); parts[2] += 1; console.log(parts.join('.'))")
NEXT_PYTHON_SDK_VERSION := $(shell node -e "const parts='$(CURRENT_PYTHON_SDK_VERSION)'.split('.').map(Number); if (parts.length !== 3 || parts.some(Number.isNaN)) process.exit(1); parts[2] += 1; console.log(parts.join('.'))")
SYNC_NODEJS_SDK_VERSION ?= $(NEXT_NODEJS_SDK_VERSION)
SYNC_PYTHON_SDK_VERSION ?= $(NEXT_PYTHON_SDK_VERSION)

# Determine the current platform
ifeq ($(OS),Windows_NT)
    PLATFORM := windows
    ARCH := amd64
else
    UNAME_S := $(shell uname -s)
    ifeq ($(UNAME_S),Linux)
        PLATFORM := linux
        ARCH := amd64
    endif
    ifeq ($(UNAME_S),Darwin)
        PLATFORM := macos
        UNAME_M := $(shell uname -m)
        ifeq ($(UNAME_M),arm64)
            ARCH := arm64
        else
            ARCH := amd64
        endif
    endif
endif

RELEASE_DIR := release
APPIMAGE_TOOL ?= appimagetool.AppImage
APPIMAGE_DIR := $(RELEASE_DIR)/wox.AppDir
APPIMAGE_NAME := wox-linux-$(ARCH).AppImage
ifeq ($(ARCH),amd64)
	APPIMAGE_ARCH := x86_64
else
	APPIMAGE_ARCH := $(ARCH)
endif

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  help       Show this help message"
	@echo "  dev        Setup development environment"
	@echo "  test       Run tests"
	@echo "  build      Build all components"
	@echo "  smoke      Run the desktop smoke E2E flow"
	@echo "  sdk        Bump SDK patch versions, publish SDKs, sync hosts, then run dev"
	@echo "  appimage   Build Linux AppImage"
	@echo "  plugins    Update plugin store"
	@echo "  www        Run docs dev server"
	@echo "  clean      Clean release directory"
	@echo "  host       Build plugin hosts"
	@echo "  release    Create a new release (reads version from CHANGELOG.md)"

_check_deps:
	@echo "Checking required dependencies..."
	@command -v go >/dev/null 2>&1 || { echo "go is required but not installed. Visit https://golang.org/doc/install" >&2; exit 1; }
	@command -v flutter >/dev/null 2>&1 || { echo "flutter is required but not installed. Visit https://flutter.dev/docs/get-started/install" >&2; exit 1; }
	@command -v node >/dev/null 2>&1 || { echo "nodejs is required but not installed. Visit https://nodejs.org/" >&2; exit 1; }
	@$(PNPM) --version >/dev/null 2>&1 || { echo "pnpm is required but unavailable. Install pnpm globally or enable Corepack for this Node.js installation." >&2; exit 1; }
	@command -v uv >/dev/null 2>&1 || { echo "uv is required but not installed. Visit https://github.com/astral-sh/uv" >&2; exit 1; }
ifeq ($(PLATFORM),linux)
	@if ! command -v $(APPIMAGE_TOOL) >/dev/null 2>&1 && [ ! -x "$(APPIMAGE_TOOL)" ]; then \
		echo "appimagetool is required but not installed. Install from https://github.com/AppImage/AppImageKit/releases or set APPIMAGE_TOOL to its path." >&2; \
		exit 1; \
	fi
	@command -v patchelf >/dev/null 2>&1 || { echo "patchelf is required on Linux to fix bundled shared library rpath." >&2; exit 1; }
endif
ifeq ($(PLATFORM),macos)
	@command -v create-dmg >/dev/null 2>&1 || { echo "create-dmg is required but not installed. Visit https://github.com/create-dmg/create-dmg" >&2; exit 1; }
endif

ifeq ($(PLATFORM),windows)
	@uname -s | grep -q '^MINGW64_NT' || { \
		echo "Please run this command in MINGW64 environment. If you have not installed MINGW64, please install it first. refer to https://www.mingw-w64.org/downloads/ or scoop install mingw" >&2; \
		exit 1; \
	}
endif

# Keep local development on the same Flutter master revision as CI because Wox
# depends on experimental windowing APIs that can change between master commits.
_pin_flutter_master:
	@echo "Ensuring Flutter SDK is pinned to $(FLUTTER_MASTER_COMMIT)..."
	@flutter_root="$$(flutter --version --machine | node -e "let input=''; process.stdin.on('data', chunk => input += chunk); process.stdin.on('end', () => { const data = JSON.parse(input); process.stdout.write(data.flutterRoot || ''); });")"; \
	if [ -z "$$flutter_root" ]; then \
		echo "Unable to determine Flutter SDK root from flutter --version --machine." >&2; \
		exit 1; \
	fi; \
	if ! git -C "$$flutter_root" rev-parse --is-inside-work-tree >/dev/null 2>&1; then \
		echo "Flutter SDK root is not a git checkout: $$flutter_root" >&2; \
		exit 1; \
	fi; \
	current_commit="$$(git -C "$$flutter_root" rev-parse HEAD)"; \
	if [ "$$current_commit" = "$(FLUTTER_MASTER_COMMIT)" ]; then \
		echo "Flutter SDK already at $(FLUTTER_MASTER_COMMIT)."; \
	else \
		echo "Checking out Flutter $(FLUTTER_MASTER_COMMIT) in $$flutter_root"; \
		git -C "$$flutter_root" fetch origin "$(FLUTTER_MASTER_COMMIT)" --depth 1; \
		git -C "$$flutter_root" checkout --detach "$(FLUTTER_MASTER_COMMIT)"; \
	fi; \
	flutter --version
	flutter config --enable-windowing

clean:
	rm -rf $(RELEASE_DIR)

dev: _check_deps _pin_flutter_master ensure-resources
	$(MAKE) -C wox.core woxmr-build
	$(MAKE) host

host:
	$(MAKE) -C wox.plugin.host.nodejs build
	$(MAKE) -C wox.plugin.host.python build

# SDK releases bump both SDK patch versions before publish because both npm and
# PyPI reject already-published versions. The host dependency update still waits
# until both publishes succeed so bundled hosts never point at an SDK release
# that failed partway through the workflow.
sdk: _update_sdk_versions
	$(MAKE) -C wox.plugin.nodejs publish
	$(MAKE) -C wox.plugin.python publish
	$(MAKE) _sync_sdk_versions SYNC_NODEJS_SDK_VERSION=$(NEXT_NODEJS_SDK_VERSION) SYNC_PYTHON_SDK_VERSION=$(NEXT_PYTHON_SDK_VERSION)

_update_sdk_versions:
	@echo "Updating Node.js SDK version from $(CURRENT_NODEJS_SDK_VERSION) to $(NEXT_NODEJS_SDK_VERSION)"
	# Use direct JSON edits here so the release flow only changes the version field instead of letting a package-manager helper normalize unrelated package.json content.
	cd wox.plugin.nodejs && node -e "const fs=require('fs'); const p='package.json'; const data=JSON.parse(fs.readFileSync(p,'utf8')); data.version='$(NEXT_NODEJS_SDK_VERSION)'; fs.writeFileSync(p, JSON.stringify(data, null, 2) + '\n');"
	@echo "Updating Python SDK version from $(CURRENT_PYTHON_SDK_VERSION) to $(NEXT_PYTHON_SDK_VERSION)"
	cd wox.plugin.python && perl -0pi -e 's/^version = "[^"]+"/version = "$(NEXT_PYTHON_SDK_VERSION)"/m' pyproject.toml

_sync_sdk_versions:
	@echo "Hosts use local SDK sources; skip syncing published SDK versions into host dependencies."
	# Hosts intentionally depend on the in-repo SDK packages so protocol changes are compiled and bundled with the matching host before any SDK release is published.

# Ensure required resource directories exist with dummy files for go:embed
ensure-resources:
	@echo "Ensuring required resource directories exist..."
	@mkdir -p wox.core/resource/ui/flutter
	@touch wox.core/resource/ui/flutter/placeholder
	@mkdir -p wox.core/resource/hosts
	@touch wox.core/resource/hosts/placeholder
	@mkdir -p wox.core/resource/others
	@touch wox.core/resource/others/placeholder

# Bug fix: keep the tracked others placeholder because go:embed rejects an
# empty directory, and deleting it after tests makes the next smoke build fail.
clean-resources:
	@rm -f wox.core/resource/ui/flutter/placeholder
	@rm -f wox.core/resource/hosts/placeholder

appimage:
ifeq ($(PLATFORM),linux)
	@echo "Building AppImage..."
	rm -rf $(APPIMAGE_DIR)
	mkdir -p $(APPIMAGE_DIR)/usr/bin
	mkdir -p $(APPIMAGE_DIR)/usr/share/icons/hicolor/256x256/apps
	mkdir -p $(APPIMAGE_DIR)/usr/share/applications
	cp $(RELEASE_DIR)/wox-linux-$(ARCH) $(APPIMAGE_DIR)/usr/bin/wox
	chmod +x $(APPIMAGE_DIR)/usr/bin/wox
	cp assets/linux/wox.desktop $(APPIMAGE_DIR)/wox.desktop
	cp assets/linux/wox.desktop $(APPIMAGE_DIR)/usr/share/applications/wox.desktop
	cp assets/linux/AppRun $(APPIMAGE_DIR)/AppRun
	chmod +x $(APPIMAGE_DIR)/AppRun
	cp assets/app.png $(APPIMAGE_DIR)/wox.png
	cp assets/app.png $(APPIMAGE_DIR)/.DirIcon
	cp assets/app.png $(APPIMAGE_DIR)/usr/share/icons/hicolor/256x256/apps/wox.png
	ARCH=$(APPIMAGE_ARCH) $(APPIMAGE_TOOL) $(APPIMAGE_DIR) $(RELEASE_DIR)/$(APPIMAGE_NAME)
else
	@echo "appimage target is only supported on Linux"
endif

# Test without rebuilding dependencies (fast)
test: ensure-resources
	@trap '$(MAKE) clean-resources' EXIT; $(MAKE) test-isolated

# Test with custom environment
# Bug fix: let the Go test config choose its per-process sandbox instead of
# forcing one shared /tmp directory. The shared directory lets stateful plugin
# tests leak saved settings, favorites, and histories into later make test
# runs, which makes CI and local reruns fail for reasons unrelated to code.
test-isolated:
	cd wox.core && WOX_TEST_CLEANUP=true go test -tags "$(SQLITE_BUILD_TAGS)" ./test -v

# Test without network dependencies
test-offline:
	cd wox.core && WOX_TEST_ENABLE_NETWORK=false go test -tags "$(SQLITE_BUILD_TAGS)" ./test -v

test-verbose:
	cd wox.core && WOX_TEST_VERBOSE=true go test -tags "$(SQLITE_BUILD_TAGS)" ./test -v

# Test with custom directories and no cleanup (for debugging)
test-debug:
	cd wox.core && WOX_TEST_DATA_DIR=/tmp/wox-test-debug WOX_TEST_CLEANUP=false WOX_TEST_VERBOSE=true go test -tags "$(SQLITE_BUILD_TAGS)" ./test -v

smoke: ensure-resources
	@trap '$(MAKE) clean-resources' EXIT; $(MAKE) -C wox.test smoke SMOKE_FILTER="$(SMOKE_FILTER)"

%:
	@:


build: clean dev
	    $(MAKE) -C wox.ui.flutter/wox build
		$(MAKE) -C wox.core build

ifeq ($(PLATFORM),linux)
		$(MAKE) appimage
endif

ifeq ($(PLATFORM),macos)
		# to make sure the working directory is the release directory
		cd $(RELEASE_DIR) && $(MAKE) -f ../Makefile _bundle_mac_app APP_NAME=wox-mac-$(ARCH)
endif

_bundle_mac_app:
	chmod +x $(APP_NAME)
	rm -rf $(APP_NAME).app Wox.app
	mkdir -p $(APP_NAME).app/Contents/MacOS
	mkdir -p $(APP_NAME).app/Contents/Resources
	cp $(APP_NAME) $(APP_NAME).app/Contents/MacOS/wox
	cp ../assets/mac/Info.plist $(APP_NAME).app/Contents/Info.plist
	cp ../assets/mac/app.icns $(APP_NAME).app/Contents/Resources/app.icns
	mv $(APP_NAME).app Wox.app
	@if [ -n "$(MACOS_KEYCHAINPWD)" ]; then \
		security unlock-keychain -p "$(MACOS_KEYCHAINPWD)"; \
	fi
	@if [ -n "$(MACOS_SIGN_IDENTITY)" ]; then \
		codesign --options=runtime --force --deep --sign "$(MACOS_SIGN_IDENTITY)" Wox.app/Contents/MacOS/wox; \
	else \
		echo "MACOS_SIGN_IDENTITY is empty; skip codesign"; \
	fi
	@if [ -n "$(MACOS_SIGN_IDENTITY)" ]; then \
		create-dmg \
			--codesign "$(MACOS_SIGN_IDENTITY)" \
			--notarize "wox" \
			--volname "Wox Installer" \
			--volicon "../assets/mac/app.icns" \
			--window-pos 200 120 \
			--window-size 800 400 \
			--icon-size 100 \
			--icon "Wox.app" 200 190 \
			--hide-extension "Wox.app" \
			--app-drop-link 600 185 \
			Wox.dmg Wox.app; \
	else \
		create-dmg \
			--volname "Wox Installer" \
			--volicon "../assets/mac/app.icns" \
			--window-pos 200 120 \
			--window-size 800 400 \
			--icon-size 100 \
			--icon "Wox.app" 200 190 \
			--hide-extension "Wox.app" \
			--app-drop-link 600 185 \
			Wox.dmg Wox.app; \
	fi
	mv "Wox.dmg" $(APP_NAME).dmg

release:
	cd ci && go run . release

plugins:
	cd ci && go run . plugin

# Keep the docs dev shortcut at the repository root so contributors can discover the website workflow without duplicating the script definition from www/package.json.
www:
	cd www && pnpm docs:dev
