.PHONY: clean build help woxmr-build

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

# Extract version string from updater/version.go
# Expected format: const CURRENT_VERSION = "x.y.z[-prerelease]"
VERSION := $(shell sed -n 's/^const CURRENT_VERSION = "\(.*\)"/\1/p' updater/version.go)

help:
	@echo "Available commands:"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make build         - Build Flutter UI for current platform ($(PLATFORM))"
	@echo "  make woxmr-build   - Build and install WoxMR.bundle for current platform ($(PLATFORM))"

clean:
	rm -rf $(RELEASE_DIR)
	rm -f resource_windows.syso rsrc_windows.syso

build: clean
	$(MAKE) woxmr-build
ifeq ($(PLATFORM),windows)
	# Generate Windows VERSIONINFO using the version from updater.CURRENT_VERSION
	cd . && go run github.com/josephspurrier/goversioninfo/cmd/goversioninfo \
		-o resource_windows.syso -64 -icon resource/app.ico \
		-product-name "Wox" -description "Wox" -internal-name "Wox" -original-name "wox-windows-$(GOARCH).exe" \
		-product-version "$(VERSION)" -file-version "$(VERSION)" \
		-propagate-ver-strings
	CGO_ENABLED=1 GOOS=windows GOARCH=$(GOARCH) go build -ldflags "-H windowsgui -s -w -X 'wox/util.ProdEnv=true'" -o $(RELEASE_DIR)/wox-windows-$(GOARCH).exe
endif
ifeq ($(PLATFORM),linux)
	CGO_ENABLED=1 GOOS=linux GOARCH=$(GOARCH) go build -ldflags "-s -w -X 'wox/util.ProdEnv=true'" -o $(RELEASE_DIR)/wox-linux-$(GOARCH)
endif
ifeq ($(PLATFORM),macos)
	CGO_ENABLED=1 GOOS=darwin GOARCH=$(GOARCH) CGO_CFLAGS="-mmacosx-version-min=10.15" CGO_LDFLAGS="-mmacosx-version-min=10.15" go build -ldflags "-s -w -X 'wox/util.ProdEnv=true'" -o $(RELEASE_DIR)/wox-mac-$(GOARCH)
endif

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
