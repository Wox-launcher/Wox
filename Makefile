.PHONY: build clean host _bundle_mac_app plugins help dev test test-all test-calculator test-converter test-plugin test-time test-network test-quick test-legacy only_test check_deps release appimage

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
	@echo "  appimage   Build Linux AppImage"
	@echo "  plugins    Update plugin store"
	@echo "  clean      Clean release directory"
	@echo "  host       Build plugin hosts"
	@echo "  release    Create a new release (reads version from CHANGELOG.md)"

_check_deps:
	@echo "Checking required dependencies..."
	@command -v go >/dev/null 2>&1 || { echo "go is required but not installed. Visit https://golang.org/doc/install" >&2; exit 1; }
	@command -v flutter >/dev/null 2>&1 || { echo "flutter is required but not installed. Visit https://flutter.dev/docs/get-started/install" >&2; exit 1; }
	@command -v node >/dev/null 2>&1 || { echo "nodejs is required but not installed. Visit https://nodejs.org/" >&2; exit 1; }
	@command -v pnpm >/dev/null 2>&1 || { echo "pnpm is required but not installed. Run: npm install -g pnpm" >&2; exit 1; }
	@command -v uv >/dev/null 2>&1 || { echo "uv is required but not installed. Visit https://github.com/astral-sh/uv" >&2; exit 1; }
ifeq ($(PLATFORM),linux)
	@if ! command -v $(APPIMAGE_TOOL) >/dev/null 2>&1 && [ ! -x "$(APPIMAGE_TOOL)" ]; then \
		echo "appimagetool is required but not installed. Install from https://github.com/AppImage/AppImageKit/releases or set APPIMAGE_TOOL to its path." >&2; \
		exit 1; \
	fi
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

clean:
	rm -rf $(RELEASE_DIR)

dev: _check_deps ensure-resources
	$(MAKE) -C wox.core woxmr-build
	$(MAKE) host

host:
	$(MAKE) -C wox.plugin.host.nodejs build
	$(MAKE) -C wox.plugin.host.python build

# Ensure required resource directories exist with dummy files for go:embed
ensure-resources:
	@echo "Ensuring required resource directories exist..."
	@mkdir -p wox.core/resource/ui/flutter
	@mkdir -p wox.core/resource/ui/macos
	@touch wox.core/resource/ui/flutter/placeholder
	@mkdir -p wox.core/resource/hosts
	@touch wox.core/resource/hosts/placeholder
	@mkdir -p wox.core/resource/others
	@touch wox.core/resource/others/placeholder

clean-resources:
	@rm -f wox.core/resource/ui/flutter/placeholder
	@rm -f wox.core/resource/hosts/placeholder
	@rm -f wox.core/resource/others/placeholder

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
test-isolated:
	cd wox.core && WOX_TEST_DATA_DIR=/tmp/wox-test-isolated WOX_TEST_CLEANUP=true go test ./test -v

# Test without network dependencies
test-offline:
	cd wox.core && WOX_TEST_ENABLE_NETWORK=false go test ./test -v

# Test with verbose logging
test-verbose:
	cd wox.core && WOX_TEST_VERBOSE=true go test ./test -v

# Test with custom directories and no cleanup (for debugging)
test-debug:
	cd wox.core && WOX_TEST_DATA_DIR=/tmp/wox-test-debug WOX_TEST_CLEANUP=false WOX_TEST_VERBOSE=true go test ./test -v


build: clean dev
ifeq ($(PLATFORM),macos)
	cd wox.ui.macos && swift build -c release
	mkdir -p wox.core/resource/ui/macos
	cp wox.ui.macos/.build/release/wox.ui.macos wox.core/resource/ui/macos/wox.ui.macos
else
	$(MAKE) -C wox.ui.flutter/wox build
endif
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
	security unlock-keychain -p $(KEYCHAINPWD) login.keychain
	codesign --options=runtime --force --deep --sign "Developer ID Application: jiajuan mao (AGYCFD2ZGN)" Wox.app/Contents/MacOS/wox
	create-dmg \
		--codesign "Developer ID Application: jiajuan mao (AGYCFD2ZGN)" \
		--notarize "wox" \
		--volname "Wox Installer" \
		--volicon "../assets/mac/app.icns" \
		--window-pos 200 120 \
		--window-size 800 400 \
		--icon-size 100 \
		--icon "Wox.app" 200 190 \
		--hide-extension "Wox.app" \
		--app-drop-link 600 185 \
		Wox.dmg Wox.app
	mv "Wox.dmg" $(APP_NAME).dmg

release:
	cd ci && go run . release

plugins:
	cd ci && go run . plugin
