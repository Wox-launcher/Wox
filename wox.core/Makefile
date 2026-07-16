.PHONY: clean build help ui-resources window-hook-build woxmr-build filesearch-real-index filesearch-real-index-release filesearch-content-real-index filesearch-content-real-index-release

SQLITE_BUILD_TAGS ?= sqlite_fts5
FILESEARCH_REAL_INDEX_ROOT ?= ~/
FILESEARCH_REAL_INDEX_KEYWORD ?= Wox
FILESEARCH_REAL_INDEX_ARTIFACT ?=
# Default to the same no-optimization flags used by the debugger-launched Wox
# core. That makes the local benchmark match the development app's real full
# index latency; use filesearch-real-index-release for an optimized baseline.
FILESEARCH_REAL_INDEX_GCFLAGS ?= all=-N -l
FILESEARCH_REAL_INDEX_TAGS := $(strip $(SQLITE_BUILD_TAGS) filesearch_real_index)
FILESEARCH_REAL_INDEX_ARGS := -filesearch-real-index -filesearch-real-index-root "$(FILESEARCH_REAL_INDEX_ROOT)" -filesearch-real-index-keyword "$(FILESEARCH_REAL_INDEX_KEYWORD)"
ifneq ($(strip $(FILESEARCH_REAL_INDEX_ARTIFACT)),)
FILESEARCH_REAL_INDEX_ARGS += -filesearch-real-index-artifact "$(FILESEARCH_REAL_INDEX_ARTIFACT)"
endif
FILESEARCH_REAL_INDEX_GOFLAGS :=
ifneq ($(strip $(FILESEARCH_REAL_INDEX_GCFLAGS)),)
FILESEARCH_REAL_INDEX_GOFLAGS += -gcflags="$(FILESEARCH_REAL_INDEX_GCFLAGS)"
endif
FILESEARCH_CONTENT_REAL_INDEX_ROOT ?= $(FILESEARCH_REAL_INDEX_ROOT)
FILESEARCH_CONTENT_REAL_INDEX_KEYWORD ?= $(FILESEARCH_REAL_INDEX_KEYWORD)
FILESEARCH_CONTENT_REAL_INDEX_ARTIFACT ?=
FILESEARCH_CONTENT_REAL_INDEX_MAX_READ_BYTES ?= 262144
FILESEARCH_CONTENT_REAL_INDEX_ARGS := -filesearch-content-real-index -filesearch-content-real-index-root "$(FILESEARCH_CONTENT_REAL_INDEX_ROOT)" -filesearch-content-real-index-keyword "$(FILESEARCH_CONTENT_REAL_INDEX_KEYWORD)" -filesearch-content-real-index-max-read-bytes $(FILESEARCH_CONTENT_REAL_INDEX_MAX_READ_BYTES)
ifneq ($(strip $(FILESEARCH_CONTENT_REAL_INDEX_ARTIFACT)),)
FILESEARCH_CONTENT_REAL_INDEX_ARGS += -filesearch-content-real-index-artifact "$(FILESEARCH_CONTENT_REAL_INDEX_ARTIFACT)"
endif

# Determine the current platform
ifeq ($(OS),Windows_NT)
    PLATFORM := windows
    ARCH := amd64
    GOARCH := amd64
else
    UNAME_S := $(shell uname -s)
    ifeq ($(UNAME_S),Linux)
        PLATFORM := linux
        ARCH := amd64
        GOARCH := amd64
    endif
    ifeq ($(UNAME_S),Darwin)
        PLATFORM := macos
        ARCH := $(shell uname -m)
        ifeq ($(ARCH),x86_64)
            GOARCH := amd64
        else
            GOARCH := $(ARCH)
        endif
    endif
endif

RELEASE_DIR := ../release

# GNU Make on Windows can detect Git's sh.exe while still leaving Git's
# usr/bin tools out of PATH. The build uses sed/rm and POSIX env assignments, so
# add the sibling usr/bin directory when that shell is in use instead of
# requiring every PowerShell session to configure the same PATH manually.
ifeq ($(OS),Windows_NT)
    GIT_USR_BIN := $(patsubst %/bin/sh.exe,%/usr/bin,$(SHELL))
    ifneq ($(GIT_USR_BIN),$(SHELL))
        export PATH := $(GIT_USR_BIN);$(PATH)
    endif
endif

# Extract version string from updater/version.go
# Expected format: const CURRENT_VERSION = "x.y.z[-prerelease]"
VERSION := $(shell sed -n 's/^const CURRENT_VERSION = "\(.*\)"/\1/p' updater/version.go)

help:
	@echo "Available commands:"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make build         - Build Wox with the embedded Go UI for current platform ($(PLATFORM))"
	@echo "  make ui-resources  - Prepare platform resources used by the embedded Go UI"
	@echo "  make filesearch-real-index - Capture debugger-aligned filesearch/fd/rg comparison for $(FILESEARCH_REAL_INDEX_ROOT), keyword $(FILESEARCH_REAL_INDEX_KEYWORD)"
	@echo "  make filesearch-real-index-release - Capture optimized filesearch/fd/rg comparison for $(FILESEARCH_REAL_INDEX_ROOT), keyword $(FILESEARCH_REAL_INDEX_KEYWORD)"
	@echo "  make filesearch-content-real-index - Capture debugger-aligned content indexing and rg comparison for $(FILESEARCH_CONTENT_REAL_INDEX_ROOT), keyword $(FILESEARCH_CONTENT_REAL_INDEX_KEYWORD)"
	@echo "  make filesearch-content-real-index-release - Capture optimized content indexing and rg comparison for $(FILESEARCH_CONTENT_REAL_INDEX_ROOT), keyword $(FILESEARCH_CONTENT_REAL_INDEX_KEYWORD)"
	@echo "  make woxmr-build   - Build and install WoxMR.bundle for current platform ($(PLATFORM))"

clean:
	rm -rf $(RELEASE_DIR)
	rm -f resource_windows.syso rsrc_windows.syso

ui-resources:
ifeq ($(PLATFORM),windows)
	@echo "WebView2Loader.dll is managed under resource/ui/go for Windows builds"
else
	@echo "The embedded Go UI does not require staged resources on $(PLATFORM)"
endif

build: clean
	$(MAKE) woxmr-build
	$(MAKE) window-hook-build
ifeq ($(PLATFORM),windows)
	# Generate Windows VERSIONINFO using the version from updater.CURRENT_VERSION
	cd . && go run github.com/josephspurrier/goversioninfo/cmd/goversioninfo@v1.5.0 \
		-o resource_windows.syso -64 -icon resource/app.ico \
		-manifest resource/wox.exe.manifest \
		-product-name "Wox" -description "Wox" -internal-name "Wox" -original-name "wox-windows-$(GOARCH).exe" \
		-product-version "$(VERSION)" -file-version "$(VERSION)" \
		-propagate-ver-strings
	CGO_ENABLED=1 GOOS=windows GOARCH=$(GOARCH) go build -tags "$(SQLITE_BUILD_TAGS)" -ldflags "-H windowsgui -s -w -X 'wox/util.ProdEnv=true'" -o $(RELEASE_DIR)/wox-windows-$(GOARCH).exe
endif

ifeq ($(PLATFORM),linux)
	CGO_ENABLED=1 GOOS=linux GOARCH=$(GOARCH) go build -tags "$(SQLITE_BUILD_TAGS)" -ldflags "-s -w -X 'wox/util.ProdEnv=true'" -o $(RELEASE_DIR)/wox-linux-$(GOARCH)
endif
ifeq ($(PLATFORM),macos)
	CGO_ENABLED=1 GOOS=darwin GOARCH=$(GOARCH) CGO_CFLAGS="-mmacosx-version-min=10.15" CGO_LDFLAGS="-mmacosx-version-min=10.15 -Wl,-no_warn_duplicate_libraries" go build -tags "$(SQLITE_BUILD_TAGS)" -ldflags "-s -w -X 'wox/util.ProdEnv=true'" -o $(RELEASE_DIR)/wox-mac-$(GOARCH)
endif

WINDOW_HOOK_SOURCE := util/windowhook/hook/window_hook.c
WINDOW_HOOK_DIR := resource/others/window_hook
WINDOW_HOOK_DLL := $(WINDOW_HOOK_DIR)/WoxWindowHook64.dll

# Build the native window hook only for Windows so other platform binaries never embed it.
window-hook-build:
	rm -rf $(WINDOW_HOOK_DIR)
ifeq ($(PLATFORM),windows)
	mkdir -p $(WINDOW_HOOK_DIR)
	gcc -shared -O2 -s -o $(WINDOW_HOOK_DLL) $(WINDOW_HOOK_SOURCE) -lole32 -lshell32 -luuid -lcomctl32
else
	@echo "Window hook is only supported on Windows"
endif

# Feature addition: real-root filesearch comparisons are intentionally exposed
# through a local Make target instead of the normal test surface, so CI keeps
# running deterministic tests while developers can opt into the expensive
# workstation-sized scan with one command and a Makefile-controlled root. The
# default target mirrors debugger-launched Wox because that is the app runtime
# developers are comparing against during local profiling.
filesearch-real-index:
	@echo "Capturing filesearch real-index comparison for $(FILESEARCH_REAL_INDEX_ROOT) with keyword $(FILESEARCH_REAL_INDEX_KEYWORD), gcflags=$(if $(strip $(FILESEARCH_REAL_INDEX_GCFLAGS)),$(FILESEARCH_REAL_INDEX_GCFLAGS),<default>)"
	WOX_FILESEARCH_REAL_INDEX_GCFLAGS="$(FILESEARCH_REAL_INDEX_GCFLAGS)" go test $(FILESEARCH_REAL_INDEX_GOFLAGS) -tags "$(FILESEARCH_REAL_INDEX_TAGS)" ./util/filesearch -run TestCaptureFileSearchRealIndexForProjectsRoot -count=1 -v -args $(FILESEARCH_REAL_INDEX_ARGS)

# Diagnostic addition: keep the optimized baseline explicit so performance work
# can separate debugger overhead from release-shaped scanner behavior.
filesearch-real-index-release:
	$(MAKE) filesearch-real-index FILESEARCH_REAL_INDEX_GCFLAGS=

# Content indexing uses the same explicit real-root build tag and debugger/release
# split as name indexing. Keeping it outside normal tests prevents CI from reading
# workstation files while giving optimization work a repeatable cold/warm baseline.
filesearch-content-real-index:
	@echo "Capturing filesearch content-index comparison for $(FILESEARCH_CONTENT_REAL_INDEX_ROOT) with keyword $(FILESEARCH_CONTENT_REAL_INDEX_KEYWORD), max_read_bytes=$(FILESEARCH_CONTENT_REAL_INDEX_MAX_READ_BYTES), gcflags=$(if $(strip $(FILESEARCH_REAL_INDEX_GCFLAGS)),$(FILESEARCH_REAL_INDEX_GCFLAGS),<default>)"
	WOX_FILESEARCH_REAL_INDEX_GCFLAGS="$(FILESEARCH_REAL_INDEX_GCFLAGS)" go test $(FILESEARCH_REAL_INDEX_GOFLAGS) -tags "$(FILESEARCH_REAL_INDEX_TAGS)" ./util/filesearch -run TestCaptureFileSearchContentRealIndex -count=1 -v -args $(FILESEARCH_CONTENT_REAL_INDEX_ARGS)

filesearch-content-real-index-release:
	$(MAKE) filesearch-content-real-index FILESEARCH_REAL_INDEX_GCFLAGS=

# -----------------------------
# MediaRemote XS (Perl) builder
# -----------------------------
# Build and install WoxMR.pm + WoxMR.bundle into resource/others so that
# resource/others/mediaremote_adapter.pl can `use WoxMR` at runtime.
# Only meaningful on macOS.

WOXMR_DIR := plugin/system/mediaplayer/woxmr
WOXMR_INSTALL_DIR := resource/others/woxmr
WOXMR_OUT_BUNDLE := $(WOXMR_INSTALL_DIR)/woxmr.bundle
WOXMR_OUT_PM := $(WOXMR_INSTALL_DIR)/WoxMR.pm

woxmr-clean:
	@echo "Cleaning WoxMR build artifacts..."
	rm -rf $(WOXMR_DIR)/build_arm64 $(WOXMR_DIR)/build_x86_64
	rm -rf $(WOXMR_INSTALL_DIR)

# Auto-detect current machine arch and build single-arch bundle
woxmr-build:
ifeq ($(PLATFORM),macos)
	$(MAKE) woxmr-clean
	$(MAKE) woxmr-$(ARCH)
else
	@echo "woxmr-build is only supported on macOS" && exit 0
endif

# Build arm64 bundle (Apple Silicon)
woxmr-arm64:
ifeq ($(PLATFORM),macos)
	@echo "Building WoxMR (arm64)..."
	rm -rf $(WOXMR_DIR)/build_arm64
	mkdir -p $(WOXMR_DIR)/build_arm64
	cp $(WOXMR_DIR)/WoxMR.pm $(WOXMR_DIR)/WoxMR.xs $(WOXMR_DIR)/wox_mr.m $(WOXMR_DIR)/Makefile.PL $(WOXMR_DIR)/build_arm64/
	cd $(WOXMR_DIR)/build_arm64 && ARCHFLAGS="-arch arm64" perl Makefile.PL
	cd $(WOXMR_DIR)/build_arm64 && ARCHFLAGS="-arch arm64" make
	mkdir -p $(WOXMR_INSTALL_DIR)
	cp -f $(WOXMR_DIR)/WoxMR.pm $(WOXMR_OUT_PM)
	cp -f $(WOXMR_DIR)/adapter.pl $(WOXMR_INSTALL_DIR)/adapter.pl
	cp -f $(WOXMR_DIR)/build_arm64/blib/arch/auto/WoxMR/WoxMR.bundle $(WOXMR_OUT_BUNDLE)
	$(MAKE) woxmr-sign
	rm -rf $(WOXMR_DIR)/build_arm64
else
	@echo "woxmr-arm64 is only supported on macOS" && exit 0
endif

# Build x86_64 bundle (Intel Macs)
woxmr-x86_64:
ifeq ($(PLATFORM),macos)
	@echo "Building WoxMR (x86_64)..."
	rm -rf $(WOXMR_DIR)/build_x86_64
	mkdir -p $(WOXMR_DIR)/build_x86_64
	cp $(WOXMR_DIR)/WoxMR.pm $(WOXMR_DIR)/WoxMR.xs $(WOXMR_DIR)/wox_mr.m $(WOXMR_DIR)/Makefile.PL $(WOXMR_DIR)/build_x86_64/
	cd $(WOXMR_DIR)/build_x86_64 && ARCHFLAGS="-arch x86_64" perl Makefile.PL
	cd $(WOXMR_DIR)/build_x86_64 && ARCHFLAGS="-arch x86_64" make
	mkdir -p $(WOXMR_INSTALL_DIR)
	cp -f $(WOXMR_DIR)/WoxMR.pm $(WOXMR_OUT_PM)
	cp -f $(WOXMR_DIR)/adapter.pl $(WOXMR_INSTALL_DIR)/adapter.pl
	cp -f $(WOXMR_DIR)/build_x86_64/blib/arch/auto/WoxMR/WoxMR.bundle $(WOXMR_OUT_BUNDLE)
	$(MAKE) woxmr-sign
	rm -rf $(WOXMR_DIR)/build_x86_64
else
	@echo "woxmr-x86_64 is only supported on macOS" && exit 0
endif

# Codesign WoxMR.bundle (always, with default identity if not overridden)
woxmr-sign:
ifeq ($(PLATFORM),macos)
	@if [ -z "$(MACOS_SIGN_IDENTITY)" ]; then \
		echo "MACOS_SIGN_IDENTITY is empty; skip signing"; \
	elif [ -f "$(WOXMR_OUT_BUNDLE)" ]; then \
		echo "Signing with identity: $(MACOS_SIGN_IDENTITY)"; \
		codesign --force --sign "$(MACOS_SIGN_IDENTITY)" --timestamp=none "$(WOXMR_OUT_BUNDLE)"; \
	else \
		echo "$(WOXMR_OUT_BUNDLE) not found, skip signing"; \
		exit 1; \
	fi
else
	@echo "woxmr-sign is only meaningful on macOS" && exit 0
endif
