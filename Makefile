.PHONY: build clean host _bundle_mac_app _linux_package_icons plugins help dev sdk _update_sdk_versions _sync_sdk_versions test test-go-ui-unit build-go-ui-smoke clean-go-ui-smoke smoke test-all test-calculator test-converter test-plugin test-time test-network test-quick test-legacy only_test check_deps release release-continue appimage deb rpm www

ifeq ($(firstword $(MAKECMDGOALS)),smoke)
SMOKE_ARGUMENTS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
SMOKE_CASE_TARGET := $(firstword $(filter-out slow,$(SMOKE_ARGUMENTS)))
SMOKE_CASE := $(if $(strip $(CASE)),$(strip $(CASE)),$(SMOKE_CASE_TARGET))
SMOKE_STEP_DELAY ?= $(if $(filter slow,$(SMOKE_ARGUMENTS)),500ms,)
ifneq ($(SMOKE_CASE_TARGET),)
.PHONY: $(SMOKE_CASE_TARGET)
$(SMOKE_CASE_TARGET):
	@:
endif
ifneq ($(filter slow,$(SMOKE_ARGUMENTS)),)
.PHONY: slow
slow:
	@:
endif
endif

SQLITE_BUILD_TAGS ?= sqlite_fts5

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
# Keep package version in sync with the embedded updater constant used by release binaries.
VERSION := $(shell sed -n 's/^const CURRENT_VERSION = "\(.*\)"/\1/p' wox.core/updater/version.go)
APPIMAGE_TOOL ?= appimagetool.AppImage
APPIMAGE_DIR := $(RELEASE_DIR)/wox.AppDir
APPIMAGE_NAME := wox-linux-$(ARCH).AppImage
APPIMAGE_DESKTOP_FILE := io.github.WoxLauncher.Wox.desktop
APPIMAGE_ICON_FILE := io.github.WoxLauncher.Wox.png
ifeq ($(ARCH),amd64)
	APPIMAGE_ARCH := x86_64
else
	APPIMAGE_ARCH := $(ARCH)
endif
# Debian and RPM both sort ~ before the final release, so 2.4.0-beta.2 becomes 2.4.0~beta.2.
DEB_VERSION := $(shell printf '%s' '$(VERSION)' | sed 's/-/~/')
DEB_DIR := $(RELEASE_DIR)/wox.debroot
DEB_NAME := wox-linux-$(ARCH).deb
DEB_ARCH := $(ARCH)
DEB_DESKTOP_FILE := $(APPIMAGE_DESKTOP_FILE)
DEB_ICON_FILE := $(APPIMAGE_ICON_FILE)
RPM_VERSION := $(DEB_VERSION)
RPM_TOPDIR := $(RELEASE_DIR)/wox.rpmbuild
RPM_SPEC := $(RPM_TOPDIR)/SPECS/wox.spec
RPM_NAME := wox-linux-$(ARCH).rpm
RPM_DESKTOP_FILE := $(APPIMAGE_DESKTOP_FILE)
RPM_ICON_FILE := $(APPIMAGE_ICON_FILE)
LINUX_METAINFO_FILE := io.github.WoxLauncher.Wox.metainfo.xml
LINUX_ICON_STAGING := $(RELEASE_DIR)/linux-icons
ifeq ($(ARCH),amd64)
	RPM_ARCH := x86_64
else
ifeq ($(ARCH),arm64)
	RPM_ARCH := aarch64
else
	RPM_ARCH := $(ARCH)
endif
endif

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  help       Show this help message"
	@echo "  dev        Setup development environment"
	@echo "  test       Run tests"
	@echo "  test-go-ui-unit  Run retained-widget, automation-contract, and driver tests"
	@echo "  smoke      Run all native smoke cases, or one with: make smoke launcher/plugin/calculator/001"
	@echo "             Add slow to pause 500ms after visible steps; override with SMOKE_STEP_DELAY=1s"
	@echo "  build      Build all components"
	@echo "  sdk        Bump SDK patch versions, publish SDKs, sync hosts, then run dev"
	@echo "  appimage   Build Linux AppImage"
	@echo "  deb        Build Linux .deb package"
	@echo "  rpm        Build Linux .rpm package"
	@echo "  plugins    Update plugin store"
	@echo "  www        Run docs dev server"
	@echo "  clean      Clean release directory"
	@echo "  host       Build plugin hosts"
	@echo "  release    Create a new release (reads version from CHANGELOG.md)"
	@echo "  release-continue Re-push the existing top CHANGELOG release tag after a failed release run"

_check_deps:
	@echo "Checking required dependencies..."
	@command -v go >/dev/null 2>&1 || { echo "go is required but not installed. Visit https://golang.org/doc/install" >&2; exit 1; }
	@command -v node >/dev/null 2>&1 || { echo "nodejs is required but not installed. Visit https://nodejs.org/" >&2; exit 1; }
	@$(PNPM) --version >/dev/null 2>&1 || { echo "pnpm is required but unavailable. Install pnpm globally or enable Corepack for this Node.js installation." >&2; exit 1; }
	@command -v uv >/dev/null 2>&1 || { echo "uv is required but not installed. Visit https://github.com/astral-sh/uv" >&2; exit 1; }
ifeq ($(PLATFORM),linux)
	@if ! command -v $(APPIMAGE_TOOL) >/dev/null 2>&1 && [ ! -x "$(APPIMAGE_TOOL)" ]; then \
		echo "appimagetool is required but not installed. Install from https://github.com/AppImage/AppImageKit/releases or set APPIMAGE_TOOL to its path." >&2; \
		exit 1; \
	fi
	@command -v dpkg-deb >/dev/null 2>&1 || { echo "dpkg-deb is required on Linux to build .deb packages. Install dpkg-dev." >&2; exit 1; }
	@command -v rpmbuild >/dev/null 2>&1 || { echo "rpmbuild is required on Linux to build .rpm packages. Install rpm." >&2; exit 1; }
	@command -v python3 >/dev/null 2>&1 || { echo "python3 is required on Linux to generate software-center icon sizes." >&2; exit 1; }
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



clean:
	rm -rf $(RELEASE_DIR)

dev: _check_deps ensure-resources
	$(MAKE) -C wox.core woxmr-build
	$(MAKE) -C wox.core window-hook-build
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
	@mkdir -p wox.core/resource/hosts
	@touch wox.core/resource/hosts/placeholder
	@mkdir -p wox.core/resource/others
	@touch wox.core/resource/others/placeholder

# Bug fix: keep the tracked others placeholder because go:embed rejects an
# empty directory, and deleting it after tests makes the next Go build fail.
clean-resources:
	@rm -f wox.core/resource/hosts/placeholder

# GNOME Software and Discover read AppStream metadata plus 64/128/256 hicolor icons from the package itself.
_linux_package_icons:
	@test -f assets/app.png || { echo "Missing assets/app.png" >&2; exit 1; }
	@test -f assets/linux/$(LINUX_METAINFO_FILE) || { echo "Missing assets/linux/$(LINUX_METAINFO_FILE)" >&2; exit 1; }
	rm -rf $(LINUX_ICON_STAGING)
	python3 assets/linux/write_hicolor_icons.py assets/app.png $(LINUX_ICON_STAGING) $(APPIMAGE_ICON_FILE)

appimage:
ifeq ($(PLATFORM),linux)
	@echo "Building AppImage..."
	$(MAKE) _linux_package_icons
	rm -rf $(APPIMAGE_DIR)
	mkdir -p $(APPIMAGE_DIR)/usr/bin
	mkdir -p $(APPIMAGE_DIR)/usr/share/applications
	mkdir -p $(APPIMAGE_DIR)/usr/share/metainfo
	mkdir -p $(APPIMAGE_DIR)/usr/share/icons/hicolor/64x64/apps
	mkdir -p $(APPIMAGE_DIR)/usr/share/icons/hicolor/128x128/apps
	mkdir -p $(APPIMAGE_DIR)/usr/share/icons/hicolor/256x256/apps
	cp $(RELEASE_DIR)/wox-linux-$(ARCH) $(APPIMAGE_DIR)/usr/bin/wox
	chmod +x $(APPIMAGE_DIR)/usr/bin/wox
	cp assets/linux/wox.desktop $(APPIMAGE_DIR)/$(APPIMAGE_DESKTOP_FILE)
	cp assets/linux/wox.desktop $(APPIMAGE_DIR)/usr/share/applications/$(APPIMAGE_DESKTOP_FILE)
	cp assets/linux/$(LINUX_METAINFO_FILE) $(APPIMAGE_DIR)/usr/share/metainfo/$(LINUX_METAINFO_FILE)
	cp assets/linux/AppRun $(APPIMAGE_DIR)/AppRun
	chmod +x $(APPIMAGE_DIR)/AppRun
	cp $(LINUX_ICON_STAGING)/256x256/$(APPIMAGE_ICON_FILE) $(APPIMAGE_DIR)/$(APPIMAGE_ICON_FILE)
	cp $(LINUX_ICON_STAGING)/256x256/$(APPIMAGE_ICON_FILE) $(APPIMAGE_DIR)/.DirIcon
	cp $(LINUX_ICON_STAGING)/64x64/$(APPIMAGE_ICON_FILE) $(APPIMAGE_DIR)/usr/share/icons/hicolor/64x64/apps/$(APPIMAGE_ICON_FILE)
	cp $(LINUX_ICON_STAGING)/128x128/$(APPIMAGE_ICON_FILE) $(APPIMAGE_DIR)/usr/share/icons/hicolor/128x128/apps/$(APPIMAGE_ICON_FILE)
	cp $(LINUX_ICON_STAGING)/256x256/$(APPIMAGE_ICON_FILE) $(APPIMAGE_DIR)/usr/share/icons/hicolor/256x256/apps/$(APPIMAGE_ICON_FILE)
	ARCH=$(APPIMAGE_ARCH) $(APPIMAGE_TOOL) $(APPIMAGE_DIR) $(RELEASE_DIR)/$(APPIMAGE_NAME)
else
	@echo "appimage target is only supported on Linux"
endif

deb:
ifeq ($(PLATFORM),linux)
	@echo "Building .deb package..."
	@test -n "$(VERSION)" || { echo "Unable to read CURRENT_VERSION from wox.core/updater/version.go" >&2; exit 1; }
	@test -f "$(RELEASE_DIR)/wox-linux-$(ARCH)" || { echo "Missing $(RELEASE_DIR)/wox-linux-$(ARCH). Run make build first." >&2; exit 1; }
	$(MAKE) _linux_package_icons
	rm -rf $(DEB_DIR)
	mkdir -p $(DEB_DIR)/DEBIAN
	mkdir -p $(DEB_DIR)/usr/bin
	mkdir -p $(DEB_DIR)/usr/share/applications
	mkdir -p $(DEB_DIR)/usr/share/metainfo
	mkdir -p $(DEB_DIR)/usr/share/pixmaps
	mkdir -p $(DEB_DIR)/usr/share/icons/hicolor/64x64/apps
	mkdir -p $(DEB_DIR)/usr/share/icons/hicolor/128x128/apps
	mkdir -p $(DEB_DIR)/usr/share/icons/hicolor/256x256/apps
	cp $(RELEASE_DIR)/wox-linux-$(ARCH) $(DEB_DIR)/usr/bin/wox
	chmod 755 $(DEB_DIR)/usr/bin/wox
	cp assets/linux/wox.desktop $(DEB_DIR)/usr/share/applications/$(DEB_DESKTOP_FILE)
	cp assets/linux/$(LINUX_METAINFO_FILE) $(DEB_DIR)/usr/share/metainfo/$(LINUX_METAINFO_FILE)
	cp $(LINUX_ICON_STAGING)/64x64/$(DEB_ICON_FILE) $(DEB_DIR)/usr/share/icons/hicolor/64x64/apps/$(DEB_ICON_FILE)
	cp $(LINUX_ICON_STAGING)/128x128/$(DEB_ICON_FILE) $(DEB_DIR)/usr/share/icons/hicolor/128x128/apps/$(DEB_ICON_FILE)
	cp $(LINUX_ICON_STAGING)/256x256/$(DEB_ICON_FILE) $(DEB_DIR)/usr/share/icons/hicolor/256x256/apps/$(DEB_ICON_FILE)
	cp $(LINUX_ICON_STAGING)/128x128/$(DEB_ICON_FILE) $(DEB_DIR)/usr/share/pixmaps/$(DEB_ICON_FILE)
	# Hard-linked GTK/X11 libs are required at process start; optional features stay in Recommends.
	@{ \
		installed_size=$$(du -sk $(DEB_DIR)/usr | awk '{print $$1}'); \
		printf '%s\n' \
			'Package: wox' \
			'Version: $(DEB_VERSION)' \
			'Architecture: $(DEB_ARCH)' \
			'Section: utils' \
			'Priority: optional' \
			'Maintainer: Wox Contributors <wox-launcher@users.noreply.github.com>' \
			'Homepage: https://github.com/Wox-launcher/Wox' \
			"Installed-Size: $$installed_size" \
			'Depends: libgtk-3-0, libepoxy0, libx11-6, libxtst6' \
			'Recommends: libgtk-layer-shell0, libpipewire-0.3-0, libwebkit2gtk-4.1-0 | libwebkit2gtk-4.0-37' \
			'Description: A launcher that stays out of your way' \
			' Wox is a fully native open-source launcher for Linux with GPU rendering,' \
			' local search, keyboard-first actions, and an extensible plugin system.' \
			> $(DEB_DIR)/DEBIAN/control; \
	}
	printf '%s\n' \
		'#!/bin/sh' \
		'set -e' \
		'if command -v update-desktop-database >/dev/null 2>&1; then' \
		'  update-desktop-database -q /usr/share/applications || true' \
		'fi' \
		'if command -v gtk-update-icon-cache >/dev/null 2>&1; then' \
		'  gtk-update-icon-cache -q /usr/share/icons/hicolor || true' \
		'fi' \
		> $(DEB_DIR)/DEBIAN/postinst
	chmod 755 $(DEB_DIR)/DEBIAN/postinst
	dpkg-deb --build --root-owner-group $(DEB_DIR) $(RELEASE_DIR)/$(DEB_NAME)
else
	@echo "deb target is only supported on Linux"
endif

rpm:
ifeq ($(PLATFORM),linux)
	@echo "Building .rpm package..."
	@test -n "$(VERSION)" || { echo "Unable to read CURRENT_VERSION from wox.core/updater/version.go" >&2; exit 1; }
	@test -f "$(RELEASE_DIR)/wox-linux-$(ARCH)" || { echo "Missing $(RELEASE_DIR)/wox-linux-$(ARCH). Run make build first." >&2; exit 1; }
	$(MAKE) _linux_package_icons
	rm -rf $(RPM_TOPDIR)
	mkdir -p $(RPM_TOPDIR)/BUILD $(RPM_TOPDIR)/RPMS $(RPM_TOPDIR)/SOURCES $(RPM_TOPDIR)/SPECS $(RPM_TOPDIR)/SRPMS
	# Keep the binary as-is and list only hard-linked GTK/X11 libs; optional dlopen features stay in Recommends.
	@{ \
		printf '%s\n' \
			'%global debug_package %{nil}' \
			'%global __os_install_post %{nil}' \
			'%global _build_id_links none' \
			'%define _binary_payload w2.xzdio' \
			'' \
			'Name: wox' \
			'Version: $(RPM_VERSION)' \
			'Release: 1' \
			'Summary: A launcher that stays out of your way' \
			'License: GPL-3.0-or-later' \
			'URL: https://github.com/Wox-launcher/Wox' \
			'BuildArch: $(RPM_ARCH)' \
			'AutoReqProv: no' \
			'' \
			'Requires: libgtk-3.so.0()(64bit)' \
			'Requires: libepoxy.so.0()(64bit)' \
			'Requires: libX11.so.6()(64bit)' \
			'Requires: libXtst.so.6()(64bit)' \
			'Recommends: libgtk-layer-shell.so.0()(64bit)' \
			'Recommends: libpipewire-0.3.so.0()(64bit)' \
			'Recommends: libwebkit2gtk-4.1.so.0()(64bit)' \
			'' \
			'%description' \
			'Wox is a fully native open-source launcher for Linux with GPU rendering,' \
			'local search, keyboard-first actions, and an extensible plugin system.' \
			'' \
			'%prep' \
			'%build' \
			'%install' \
			'install -D -m 755 $(abspath $(RELEASE_DIR)/wox-linux-$(ARCH)) %{buildroot}/usr/bin/wox' \
			'install -D -m 644 $(abspath assets/linux/wox.desktop) %{buildroot}/usr/share/applications/$(RPM_DESKTOP_FILE)' \
			'install -D -m 644 $(abspath assets/linux/$(LINUX_METAINFO_FILE)) %{buildroot}/usr/share/metainfo/$(LINUX_METAINFO_FILE)' \
			'install -D -m 644 $(abspath $(LINUX_ICON_STAGING)/64x64/$(RPM_ICON_FILE)) %{buildroot}/usr/share/icons/hicolor/64x64/apps/$(RPM_ICON_FILE)' \
			'install -D -m 644 $(abspath $(LINUX_ICON_STAGING)/128x128/$(RPM_ICON_FILE)) %{buildroot}/usr/share/icons/hicolor/128x128/apps/$(RPM_ICON_FILE)' \
			'install -D -m 644 $(abspath $(LINUX_ICON_STAGING)/256x256/$(RPM_ICON_FILE)) %{buildroot}/usr/share/icons/hicolor/256x256/apps/$(RPM_ICON_FILE)' \
			'install -D -m 644 $(abspath $(LINUX_ICON_STAGING)/128x128/$(RPM_ICON_FILE)) %{buildroot}/usr/share/pixmaps/$(RPM_ICON_FILE)' \
			'' \
			'%files' \
			'%attr(0755,root,root) /usr/bin/wox' \
			'%attr(0644,root,root) /usr/share/applications/$(RPM_DESKTOP_FILE)' \
			'%attr(0644,root,root) /usr/share/metainfo/$(LINUX_METAINFO_FILE)' \
			'%attr(0644,root,root) /usr/share/icons/hicolor/64x64/apps/$(RPM_ICON_FILE)' \
			'%attr(0644,root,root) /usr/share/icons/hicolor/128x128/apps/$(RPM_ICON_FILE)' \
			'%attr(0644,root,root) /usr/share/icons/hicolor/256x256/apps/$(RPM_ICON_FILE)' \
			'%attr(0644,root,root) /usr/share/pixmaps/$(RPM_ICON_FILE)' \
			'' \
			'%post' \
			'if command -v update-desktop-database >/dev/null 2>&1; then' \
			'  update-desktop-database -q /usr/share/applications || true' \
			'fi' \
			'if command -v gtk-update-icon-cache >/dev/null 2>&1; then' \
			'  gtk-update-icon-cache -q /usr/share/icons/hicolor || true' \
			'fi' \
			'' \
			'%postun' \
			'if command -v update-desktop-database >/dev/null 2>&1; then' \
			'  update-desktop-database -q /usr/share/applications || true' \
			'fi' \
			'if command -v gtk-update-icon-cache >/dev/null 2>&1; then' \
			'  gtk-update-icon-cache -q /usr/share/icons/hicolor || true' \
			'fi' \
			> $(RPM_SPEC); \
	}
	rpmbuild -bb --define '_topdir $(abspath $(RPM_TOPDIR))' $(RPM_SPEC)
	@rpm_path=$$(find $(RPM_TOPDIR)/RPMS -name '*.rpm' | head -n 1); \
		test -n "$$rpm_path" || { echo "rpmbuild did not produce an RPM" >&2; exit 1; }; \
		mv "$$rpm_path" $(RELEASE_DIR)/$(RPM_NAME)
else
	@echo "rpm target is only supported on Linux"
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

# The fast Go UI layer runs on every relevant change and never opens a native window.
test-go-ui-unit: ensure-resources
	cd wox.core && go test -tags "wox_automation" ./appcontrol ./ui/automation ./ui/runtime ./ui/widget ./ui/launcher ./test/automationdriver -count=1

GO_UI_SMOKE_BINARY_NAME := wox-go-ui-smoke$(if $(filter windows,$(PLATFORM)),.exe,)
GO_UI_SMOKE_BINARY := $(CURDIR)/wox.core/.tmp/$(GO_UI_SMOKE_BINARY_NAME)
GO_UI_SMOKE_RUNNER ?=

# Keep the smoke binary build reusable by CI, make, and editor launch configurations.
build-go-ui-smoke: ensure-resources
	@mkdir -p wox.core/.tmp
	cd wox.core && go build -tags "$(SQLITE_BUILD_TAGS) wox_automation" -o "$(GO_UI_SMOKE_BINARY)" .

clean-go-ui-smoke:
	@rm -f "$(GO_UI_SMOKE_BINARY)"

# The suite runner owns one Wox process shared by every serial smoke package.
smoke: build-go-ui-smoke
	@trap 'rm -f "$(GO_UI_SMOKE_BINARY)"' EXIT; \
		cd wox.core && \
		WOX_GO_UI_SMOKE_BINARY="$(GO_UI_SMOKE_BINARY)" WOX_GO_UI_SMOKE_STEP_DELAY="$(SMOKE_STEP_DELAY)" $(GO_UI_SMOKE_RUNNER) go run ./test/smokerunner -case "$(SMOKE_CASE)"

# Test without network dependencies
test-offline:
	cd wox.core && WOX_TEST_ENABLE_NETWORK=false go test -tags "$(SQLITE_BUILD_TAGS)" ./test -v

test-verbose:
	cd wox.core && WOX_TEST_VERBOSE=true go test -tags "$(SQLITE_BUILD_TAGS)" ./test -v

# Test with custom directories and no cleanup (for debugging)
test-debug:
	cd wox.core && WOX_TEST_DATA_DIR=/tmp/wox-test-debug WOX_TEST_CLEANUP=false WOX_TEST_VERBOSE=true go test -tags "$(SQLITE_BUILD_TAGS)" ./test -v

%:
	@:


build: clean dev
	$(MAKE) -C wox.core build

ifeq ($(PLATFORM),linux)
		$(MAKE) appimage
		$(MAKE) deb
		$(MAKE) rpm
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
	# Keep local development builds separate from an installed production Wox.
	# The outer development bundle gets a stable designated requirement so TCC grants
	# survive code changes without requiring an interactive signing certificate.
	@if [ -n "$(MACOS_SIGN_IDENTITY)" ]; then \
		codesign --options=runtime --force --deep --sign "$(MACOS_SIGN_IDENTITY)" Wox.app; \
	else \
		/usr/libexec/PlistBuddy -c "Set :CFBundleIdentifier com.github.wox.dev" Wox.app/Contents/Info.plist; \
		codesign --force --deep --sign - Wox.app; \
		codesign --force --sign - --requirements '=designated => identifier "com.github.wox.dev"' Wox.app; \
		echo "MACOS_SIGN_IDENTITY is empty; use the stable com.github.wox.dev development requirement"; \
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

release-continue:
	@tag=$$(awk '/^## v[0-9]/{ print $$2; exit }' CHANGELOG.md); \
	if [ -z "$$tag" ]; then \
		echo "Unable to read release tag from the top CHANGELOG.md version header." >&2; \
		exit 1; \
	fi; \
	if [ -n "$$(git status --porcelain)" ]; then \
		echo "Please commit/stash your changes before continuing release $$tag." >&2; \
		git status --short; \
		exit 1; \
	fi; \
	if ! git ls-remote --exit-code --tags origin "refs/tags/$$tag" >/dev/null 2>&1; then \
		echo "Remote release tag $$tag does not exist. Run make release first." >&2; \
		exit 1; \
	fi; \
	remote_tag=$$(git ls-remote --tags origin "refs/tags/$$tag" | awk '{ print $$1; exit }'); \
	current_head=$$(git rev-parse --short HEAD); \
	echo ""; \
	echo "============================================================"; \
	echo "Release Continue Review"; \
	echo "============================================================"; \
	echo "Tag:             $$tag"; \
	echo "Current HEAD:    $$current_head"; \
	echo "Remote tag SHA:  $$remote_tag"; \
	echo ""; \
	echo "This will:"; \
	echo "  1. Recreate local annotated tag $$tag at current HEAD"; \
	echo "  2. Force-push $$tag to origin"; \
	echo "  3. Trigger the GitHub Release workflow again"; \
	echo "============================================================"; \
	printf "\nProceed with release continue? (yes/no): "; \
	read input; \
	input=$$(printf "%s" "$$input" | tr '[:upper:]' '[:lower:]'); \
	if [ "$$input" != "yes" ] && [ "$$input" != "y" ]; then \
		echo "Release continue cancelled."; \
		exit 0; \
	fi; \
	echo "Continuing release $$tag from current HEAD $$current_head"; \
	git tag -f -a "$$tag" -m "Release $$tag"; \
	git push origin "refs/tags/$$tag" --force; \
	echo "Re-pushed $$tag."; \

plugins:
	cd ci && go run . plugin

# Keep the docs dev shortcut at the repository root so contributors can discover the website workflow without duplicating the script definition from www/package.json.
www:
	cd www && pnpm docs:dev
